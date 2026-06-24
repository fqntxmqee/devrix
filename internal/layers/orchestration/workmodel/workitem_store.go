package workmodel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const workItemSchemaVersion = 2

// WorkItemStore defines persistence for work items.
type WorkItemStore interface {
	Load(sessionID string) ([]*WorkItem, error)
	Save(sessionID string, items []*WorkItem) error
}

// DiskWorkItemStore persists session work trees as JSON files.
type DiskWorkItemStore struct {
	mu  sync.RWMutex
	dir string
}

// NewDiskWorkItemStore creates a disk-backed work item store.
func NewDiskWorkItemStore(dir string) (*DiskWorkItemStore, error) {
	dir = expandStorePath(dir)
	if dir == "" {
		return nil, fmt.Errorf("work item store dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &DiskWorkItemStore{dir: dir}, nil
}

func (s *DiskWorkItemStore) path(sessionID string) string {
	return filepath.Join(s.dir, sessionID+".json")
}

// Load reads work items; tolerates empty/corrupt files.
func (s *DiskWorkItemStore) Load(sessionID string) ([]*WorkItem, error) {
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
	if len(data) == 0 {
		return nil, nil
	}

	var file sessionWorkFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("work item store: corrupt json for %s: %w", sessionID, err)
	}

	if file.SchemaVersion > workItemSchemaVersion {
		return nil, fmt.Errorf("work item store: unsupported schema version %d", file.SchemaVersion)
	}

	return file.Items, nil
}

// Save writes work items atomically (tmp + rename).
func (s *DiskWorkItemStore) Save(sessionID string, items []*WorkItem) error {
	if s == nil || sessionID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	file := sessionWorkFile{
		SessionID:     sessionID,
		SchemaVersion: workItemSchemaVersion,
		Items:         items,
	}
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

type sessionWorkFile struct {
	SessionID     string      `json:"session_id"`
	SchemaVersion int         `json:"schema_version,omitempty"`
	Items         []*WorkItem `json:"items,omitempty"`
}

// FindByItemID scans all session files in the store directory.
func (s *DiskWorkItemStore) FindByItemID(itemID string) (*WorkItem, string, bool) {
	if s == nil || itemID == "" {
		return nil, "", false
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, "", false
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		sessionID := strings.TrimSuffix(ent.Name(), ".json")
		items, err := s.Load(sessionID)
		if err != nil {
			continue
		}
		for _, item := range items {
			if item != nil && item.ID == itemID {
				return item, sessionID, true
			}
		}
	}
	return nil, "", false
}

func expandStorePath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}
	return os.ExpandEnv(path)
}
