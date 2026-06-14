package coordinator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// TaskStore defines the interface for task persistence.
type TaskStore interface {
	// Load reads tasks for a session from storage.
	Load(sessionID string) ([]*Task, error)
	// Save writes tasks for a session to storage.
	Save(sessionID string, tasks []*Task) error
}

// DiskTaskStore persists session task lists as JSON files.
type DiskTaskStore struct {
	mu  sync.RWMutex
	dir string
}

// NewDiskTaskStore creates a disk-backed task store.
func NewDiskTaskStore(dir string) (*DiskTaskStore, error) {
	dir = expandStorePath(dir)
	if dir == "" {
		return nil, fmt.Errorf("task store dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &DiskTaskStore{dir: dir}, nil
}

func (s *DiskTaskStore) path(sessionID string) string {
	return filepath.Join(s.dir, sessionID+".json")
}

// Load reads tasks for a session from disk.
func (s *DiskTaskStore) Load(sessionID string) ([]*Task, error) {
	if s == nil || sessionID == "" {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var file sessionTasksFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	return file.Tasks, nil
}

// Save writes tasks for a session to disk.
func (s *DiskTaskStore) Save(sessionID string, tasks []*Task) error {
	if s == nil || sessionID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	file := sessionTasksFile{SessionID: sessionID, Tasks: tasks}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	target := s.path(sessionID)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

type sessionTasksFile struct {
	SessionID string  `json:"session_id"`
	Tasks     []*Task `json:"tasks"`
}

func expandStorePath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}
	return os.ExpandEnv(path)
}
