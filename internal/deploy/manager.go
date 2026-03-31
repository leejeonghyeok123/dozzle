package deploy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/amir20/dozzle/internal/container"
	"github.com/google/uuid"
)

const (
	labelEnabled = "dev.dozzle.deploy.enabled"
	labelPath    = "dev.dozzle.deploy.path"
	labelRepo    = "dev.dozzle.deploy.repo"
	labelBranch  = "dev.dozzle.deploy.branch"
	labelCompose = "dev.dozzle.deploy.compose"
	labelService = "dev.dozzle.deploy.service"
)

type run struct {
	status Status
	lines  []string
}

type Manager struct {
	mu           sync.RWMutex
	runs         map[string]*run
	projectLocks map[string]*sync.Mutex
}

func NewManager() *Manager {
	return &Manager{
		runs:         make(map[string]*run),
		projectLocks: make(map[string]*sync.Mutex),
	}
}

func (m *Manager) Start(ctx context.Context, c container.Container, req Request) (string, error) {
	resolved, err := resolveRequest(c, req)
	if err != nil {
		return "", err
	}

	runID := uuid.NewString()
	m.mu.Lock()
	m.runs[runID] = &run{
		status: Status{
			RunID:       runID,
			ContainerID: c.ID,
			State:       StatePending,
			StartedAt:   time.Now(),
			ExitCode:    -1,
		},
		lines: make([]string, 0, 128),
	}
	m.mu.Unlock()

	bg := context.WithoutCancel(ctx)
	runCtx, cancel := context.WithTimeout(bg, 20*time.Minute)
	go func() {
		defer cancel()
		m.execute(runCtx, c, resolved, runID)
	}()
	return runID, nil
}

func (m *Manager) Status(runID string) (Status, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.runs[runID]
	if !ok {
		return Status{}, fmt.Errorf("run not found: %s", runID)
	}
	return r.status, nil
}

func (m *Manager) Logs(runID string, offset int) (LogChunk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.runs[runID]
	if !ok {
		return LogChunk{}, fmt.Errorf("run not found: %s", runID)
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(r.lines) {
		offset = len(r.lines)
	}
	lines := append([]string(nil), r.lines[offset:]...)
	return LogChunk{
		RunID:   runID,
		Offset:  offset,
		Lines:   lines,
		Next:    len(r.lines),
		HasMore: false,
		Done:    r.status.State == StateFailed || r.status.State == StateSuccess,
	}, nil
}

func (m *Manager) Recent(containerID string, limit int) []Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]Status, 0, len(m.runs))
	for _, r := range m.runs {
		if containerID != "" && r.status.ContainerID != containerID {
			continue
		}
		items = append(items, r.status)
	}

	slices.SortFunc(items, func(a, b Status) int {
		if a.StartedAt.After(b.StartedAt) {
			return -1
		}
		if a.StartedAt.Before(b.StartedAt) {
			return 1
		}
		return 0
	})

	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	return append([]Status(nil), items[:limit]...)
}

func resolveRequest(c container.Container, req Request) (Request, error) {
	labels := c.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	if req.ProjectPath == "" {
		req.ProjectPath = labels[labelPath]
	}
	if req.RepoURL == "" {
		req.RepoURL = labels[labelRepo]
	}
	if req.Branch == "" {
		req.Branch = labels[labelBranch]
	}
	if req.ComposeFile == "" {
		req.ComposeFile = labels[labelCompose]
	}
	if req.Service == "" {
		req.Service = labels[labelService]
	}

	if req.Branch == "" {
		req.Branch = "main"
	}
	if req.ComposeFile == "" {
		req.ComposeFile = "docker-compose.yml"
	}

	if !req.AllowDisabled {
		if labels[labelEnabled] != "true" {
			return Request{}, errors.New("deploy is not enabled for this container (label dev.dozzle.deploy.enabled=true required)")
		}
	}
	if req.ProjectPath == "" {
		return Request{}, errors.New("projectPath is required")
	}

	return req, nil
}

func (m *Manager) execute(ctx context.Context, c container.Container, req Request, runID string) {
	m.update(runID, func(r *run) {
		r.status.State = StateRunning
		r.status.Message = "deployment started"
	})

	lock := m.projectLock(req.ProjectPath)
	lock.Lock()
	defer lock.Unlock()

	m.appendLine(runID, "Starting deploy run for container: "+c.Name)
	m.appendLine(runID, "Project path: "+req.ProjectPath)

	exitCode, err := m.runPipeline(ctx, runID, req)
	finished := time.Now()
	m.update(runID, func(r *run) {
		r.status.ExitCode = exitCode
		r.status.FinishedAt = &finished
		if err != nil {
			r.status.State = StateFailed
			r.status.Message = err.Error()
		} else {
			r.status.State = StateSuccess
			r.status.Message = "deployment completed"
		}
	})
}

func (m *Manager) runPipeline(ctx context.Context, runID string, req Request) (int, error) {
	projectPath := req.ProjectPath
	if st, err := os.Stat(projectPath); err != nil || !st.IsDir() {
		if !req.Bootstrap {
			return 1, fmt.Errorf("project path does not exist: %s", projectPath)
		}
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			return 1, fmt.Errorf("failed to create project path: %w", err)
		}
	}

	gitDir := filepath.Join(projectPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if !req.Bootstrap {
			return 1, fmt.Errorf("git repository not found at %s", projectPath)
		}
		if req.RepoURL == "" {
			return 1, errors.New("repoUrl is required for bootstrap")
		}
		if err := m.runCommand(ctx, runID, "", "git", "clone", authenticatedRepoURL(req.RepoURL, req.GitUsername, req.GitToken), projectPath); err != nil {
			return 1, err
		}
	}

	if err := m.runCommand(ctx, runID, projectPath, "git", "fetch", "--all", "--prune"); err != nil {
		return 1, err
	}
	if err := m.runCommand(ctx, runID, projectPath, "git", "checkout", req.Branch); err != nil {
		return 1, err
	}
	if err := m.runCommand(ctx, runID, projectPath, "git", "pull", "origin", req.Branch); err != nil {
		return 1, err
	}

	composeArgs := []string{}
	if req.ComposeFile != "" {
		composeArgs = append(composeArgs, "-f", req.ComposeFile)
	}
	composeArgs = append(composeArgs, "up", "-d", "--build")
	if req.Service != "" {
		composeArgs = append(composeArgs, req.Service)
	}

	// Prefer docker compose plugin, fallback to docker-compose binary.
	if err := m.runCommand(ctx, runID, projectPath, "docker", append([]string{"compose"}, composeArgs...)...); err != nil {
		m.appendLine(runID, "docker compose failed, trying docker-compose fallback")
		if err2 := m.runCommand(ctx, runID, projectPath, "docker-compose", composeArgs...); err2 != nil {
			return 1, err
		}
	}
	return 0, nil
}

func authenticatedRepoURL(repoURL, username, token string) string {
	if token == "" || !strings.HasPrefix(repoURL, "https://") {
		return repoURL
	}
	user := username
	if user == "" {
		user = "x-access-token"
	}
	return strings.Replace(repoURL, "https://", "https://"+user+":"+token+"@", 1)
}

func (m *Manager) runCommand(ctx context.Context, runID, dir, name string, args ...string) error {
	m.appendLine(runID, "$ "+name+" "+strings.Join(redactArgs(args), " "))
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	copyLines := func(reader io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			m.appendLine(runID, scanner.Text())
		}
	}
	wg.Add(2)
	go copyLines(stdout)
	go copyLines(stderr)
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		m.appendLine(runID, "Command failed: "+err.Error())
		return err
	}
	return nil
}

func redactArgs(args []string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = redactURLCredentials(arg)
	}
	return out
}

func redactURLCredentials(input string) string {
	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		return input
	}
	schemeEnd := strings.Index(input, "://")
	if schemeEnd == -1 {
		return input
	}
	rest := input[schemeEnd+3:]
	at := strings.Index(rest, "@")
	if at == -1 {
		return input
	}
	return input[:schemeEnd+3] + "***@" + rest[at+1:]
}

func (m *Manager) projectLock(path string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	if lock, ok := m.projectLocks[path]; ok {
		return lock
	}
	lock := &sync.Mutex{}
	m.projectLocks[path] = lock
	return lock
}

func (m *Manager) update(runID string, fn func(r *run)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.runs[runID]; ok {
		fn(r)
	}
}

func (m *Manager) appendLine(runID, line string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.runs[runID]; ok {
		r.lines = append(r.lines, line)
		if len(r.lines) > 2000 {
			r.lines = r.lines[len(r.lines)-2000:]
		}
	}
}

