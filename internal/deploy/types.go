package deploy

import "time"

type State string

const (
	StatePending State = "pending"
	StateRunning State = "running"
	StateSuccess State = "success"
	StateFailed  State = "failed"
)

type Request struct {
	ContainerID   string `json:"containerId"`
	ComposeProject string `json:"composeProject"`
	ProjectPath   string `json:"projectPath"`
	RepoURL       string `json:"repoUrl"`
	Branch        string `json:"branch"`
	ComposeFile   string `json:"composeFile"`
	Service       string `json:"service"`
	Services      []string `json:"services"`
	GitUsername   string `json:"gitUsername"`
	GitToken      string `json:"gitToken"`
	Bootstrap     bool   `json:"bootstrap"`
	RequestedBy   string `json:"requestedBy"`
	AllowDisabled bool   `json:"allowDisabled"`
}

type Status struct {
	RunID       string     `json:"runId"`
	ContainerID string     `json:"containerId"`
	State       State      `json:"state"`
	Message     string     `json:"message,omitempty"`
	StartedAt   time.Time  `json:"startedAt"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	ExitCode    int        `json:"exitCode"`
}

type LogChunk struct {
	RunID   string   `json:"runId"`
	Offset  int      `json:"offset"`
	Lines   []string `json:"lines"`
	Next    int      `json:"next"`
	HasMore bool     `json:"hasMore"`
	Done    bool     `json:"done"`
}

