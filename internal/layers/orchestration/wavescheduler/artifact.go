package wavescheduler

import "sync"

// ArtifactStore records the output of each completed Task so downstream tasks
// with context_policy=upstream can consume it. It is intentionally simple:
// in-memory map keyed by task id, with optional session scope to keep tests
// isolated. A future v1.2 change may add persistence (T1 of design §11).
type ArtifactStore struct {
	mu     sync.RWMutex
	byTask map[string]Artifact
	bySess map[string]map[string]Artifact // sessionID -> taskID -> Artifact
}

// NewArtifactStore creates an empty store.
func NewArtifactStore() *ArtifactStore {
	return &ArtifactStore{
		byTask: make(map[string]Artifact),
		bySess: make(map[string]map[string]Artifact),
	}
}

// Put records an artifact globally and scoped to its session (when non-empty).
func (s *ArtifactStore) Put(art Artifact) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byTask[art.TaskID] = art
	if art.SessionID != "" {
		s.putSessionLocked(art.SessionID, art)
	}
}

// PutForSession records an artifact and explicitly tags it with a session id.
func (s *ArtifactStore) PutForSession(sessionID string, art Artifact) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	art.SessionID = sessionID
	s.byTask[art.TaskID] = art
	if sessionID != "" {
		s.putSessionLocked(sessionID, art)
	}
}

// Get returns a previously-stored artifact by task id (global lookup).
func (s *ArtifactStore) Get(taskID string) (Artifact, bool) {
	if s == nil {
		return Artifact{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	art, ok := s.byTask[taskID]
	return art, ok
}

// GetForSession returns a previously-stored artifact scoped to a session.
func (s *ArtifactStore) GetForSession(sessionID, taskID string) (Artifact, bool) {
	if s == nil {
		return Artifact{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.bySess[sessionID]
	if !ok {
		return Artifact{}, false
	}
	art, ok := m[taskID]
	return art, ok
}

// List returns a snapshot of all stored artifacts.
func (s *ArtifactStore) List() []Artifact {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Artifact, 0, len(s.byTask))
	for _, a := range s.byTask {
		out = append(out, a)
	}
	return out
}

// ListForSession returns a snapshot of artifacts for one session only.
func (s *ArtifactStore) ListForSession(sessionID string) []Artifact {
	if s == nil || sessionID == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.bySess[sessionID]
	out := make([]Artifact, 0, len(m))
	for _, a := range m {
		out = append(out, a)
	}
	return out
}

// putSessionLocked inserts an artifact into the per-session map. Caller holds
// the write lock.
func (s *ArtifactStore) putSessionLocked(sessionID string, art Artifact) {
	if _, ok := s.bySess[sessionID]; !ok {
		s.bySess[sessionID] = make(map[string]Artifact)
	}
	s.bySess[sessionID][art.TaskID] = art
}
