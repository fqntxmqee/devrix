package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

// SessionStore defines the interface for session storage
type SessionStore interface {
	Create(session *types.Session) error
	Get(sessionID string) (*types.Session, error)
	Update(session *types.Session) error
	Delete(sessionID string) error
	List() ([]*types.Session, error)
	GetIdleSessions(timeout time.Duration) ([]*types.Session, error)
}

// FileSessionStore implements SessionStore using JSON files
type FileSessionStore struct {
	mu  sync.RWMutex
	dir string
}

// NewFileSessionStore creates a new FileSessionStore
func NewFileSessionStore(dir string) (*FileSessionStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}
	return &FileSessionStore{
		dir: dir,
	}, nil
}

// sessionFilePath returns the file path for a session
func (s *FileSessionStore) sessionFilePath(sessionID string) string {
	return filepath.Join(s.dir, fmt.Sprintf("%s.json", sessionID))
}

// Create persists a new session to disk
func (s *FileSessionStore) Create(session *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := s.atomicWrite(s.sessionFilePath(session.SessionID), data); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// Get loads a session from disk
func (s *FileSessionStore) Get(sessionID string) (*types.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.sessionFilePath(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session types.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// Update re-persists a session to disk
func (s *FileSessionStore) Update(session *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := s.atomicWrite(s.sessionFilePath(session.SessionID), data); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// Delete removes a session file from disk
func (s *FileSessionStore) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.sessionFilePath(sessionID)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	return nil
}

// List returns all sessions from disk
func (s *FileSessionStore) List() ([]*types.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}

	var sessions []*types.Session
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionID := entry.Name()[:len(entry.Name())-5] // Remove .json
		session, err := s.loadSession(sessionID)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetIdleSessions returns sessions that have been idle longer than the timeout
func (s *FileSessionStore) GetIdleSessions(timeout time.Duration) ([]*types.Session, error) {
	sessions, err := s.List()
	if err != nil {
		return nil, err
	}

	var idle []*types.Session
	for _, session := range sessions {
		if session.IsIdle(timeout) {
			idle = append(idle, session)
		}
	}

	return idle, nil
}

// loadSession loads a session without locking (caller must hold lock)
func (s *FileSessionStore) loadSession(sessionID string) (*types.Session, error) {
	data, err := os.ReadFile(s.sessionFilePath(sessionID))
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session types.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// atomicWrite writes data to a temp file then renames it (atomic operation)
func (s *FileSessionStore) atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "session-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmp.Close()

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
