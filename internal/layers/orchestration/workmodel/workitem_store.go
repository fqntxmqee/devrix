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

// Load reads work items; migrates v1 tasks; tolerates empty/corrupt files.
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

	if len(file.Items) > 0 {
		return file.Items, nil
	}
	if len(file.Tasks) > 0 {
		return WorkItemsFromTasks(file.Tasks), nil
	}
	return nil, nil
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
	Tasks         []*Task     `json:"tasks,omitempty"`
}

type taskStoreAdapter struct {
	store TaskStore
}

func (a *taskStoreAdapter) Load(sessionID string) ([]*WorkItem, error) {
	tasks, err := a.store.Load(sessionID)
	if err != nil {
		return nil, err
	}
	return WorkItemsFromTasks(tasks), nil
}

func (a *taskStoreAdapter) Save(sessionID string, items []*WorkItem) error {
	return a.store.Save(sessionID, TasksFromWorkItems(items))
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
