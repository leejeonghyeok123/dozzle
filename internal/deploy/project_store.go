package deploy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type ProjectConfig struct {
	Host          string   `json:"host"`
	ComposeProject string  `json:"composeProject"`
	ProjectPath   string   `json:"projectPath"`
	RepoURL       string   `json:"repoUrl"`
	Branch        string   `json:"branch"`
	ComposeFile   string   `json:"composeFile"`
	Services      []string `json:"services,omitempty"`
}

type ProjectStore struct {
	path string
	mu   sync.Mutex
}

func NewProjectStore(path string) *ProjectStore {
	return &ProjectStore{path: path}
}

func (s *ProjectStore) key(host, composeProject string) string {
	return host + "::" + composeProject
}

func (s *ProjectStore) Save(config ProjectConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.readAll()
	if err != nil {
		return err
	}
	all[s.key(config.Host, config.ComposeProject)] = config
	return s.writeAll(all)
}

func (s *ProjectStore) Get(host, composeProject string) (ProjectConfig, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.readAll()
	if err != nil {
		return ProjectConfig{}, false
	}
	value, ok := all[s.key(host, composeProject)]
	return value, ok
}

func (s *ProjectStore) readAll() (map[string]ProjectConfig, error) {
	values := map[string]ProjectConfig{}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return values, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return values, nil
	}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func (s *ProjectStore) writeAll(values map[string]ProjectConfig) error {
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0600)
}

