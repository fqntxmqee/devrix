// PendingResolutionStore — HumanArbitrator T2 ResumeSession 续跑状态 (DM-20260625-003, PR-V5.3)
//
// 关键设计 (doc 38 §21.3.4, design.md §5.3):
//   - Save: 持久化 EscapeDecision (ProcessMessage 同步返回前)
//   - Load: T2 续跑入口检查 (ProcessMessage 开头)
//   - Delete: 消费后立即删除 (防重复续跑)
//   - InMemoryPendingResolutionStore 是 dev 默认实现 (sync.RWMutex)
//   - 生产可换 DB / Redis 实现 (Phase V5.5+ 接入 D5 observability)
package escape

import (
	"errors"
	"fmt"
	"sync"
)

// ErrPendingResolutionNotFound is returned by Load when no pending decision
// exists for the session (正常 T2 路径: 走完整 5 节点).
var ErrPendingResolutionNotFound = errors.New("escape: pending resolution not found")

// PendingResolutionStore is the persistence interface for human-pending
// decisions awaiting user response. The EscapeEngine.ResumeSession
// entry point uses Load at the start of each ProcessMessage to detect
// a prior pending decision; if found, the consumer runs the recorded
// decision instead of going through the full 5-node pipeline.
type PendingResolutionStore interface {
	// Save persists a decision for a session. Overwrites any existing
	// decision for the same sessionID (the prior session's pending
	// is implicitly resolved by the new write).
	Save(sessionID string, decision EscapeDecision) error

	// Load retrieves a saved decision. Returns the decision, a "found"
	// flag, and an error. found=false + err=nil means no pending
	// decision (caller should run the normal pipeline).
	Load(sessionID string) (EscapeDecision, bool, error)

	// Delete removes the saved decision. Idempotent — deleting a
	// non-existent sessionID returns nil.
	Delete(sessionID string) error
}

// InMemoryPendingResolutionStore is the dev default implementation.
// Safe for concurrent use; backed by a sync.RWMutex-protected map.
//
// State is process-local; restarting the orchestrator loses all
// pending decisions (caller will fall through to normal pipeline).
// Production deployments should swap in a DB-backed or Redis-backed
// implementation.
type InMemoryPendingResolutionStore struct {
	mu   sync.RWMutex
	data map[string]EscapeDecision
}

// NewInMemoryPendingResolutionStore constructs an empty store.
func NewInMemoryPendingResolutionStore() *InMemoryPendingResolutionStore {
	return &InMemoryPendingResolutionStore{
		data: make(map[string]EscapeDecision),
	}
}

// Save implements PendingResolutionStore.
func (s *InMemoryPendingResolutionStore) Save(sessionID string, decision EscapeDecision) error {
	if sessionID == "" {
		return fmt.Errorf("escape: PendingResolutionStore.Save: sessionID is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[sessionID] = decision
	return nil
}

// Load implements PendingResolutionStore.
func (s *InMemoryPendingResolutionStore) Load(sessionID string) (EscapeDecision, bool, error) {
	if sessionID == "" {
		return EscapeDecision{}, false, fmt.Errorf("escape: PendingResolutionStore.Load: sessionID is empty")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.data[sessionID]
	if !ok {
		return EscapeDecision{}, false, nil
	}
	return d, true, nil
}

// Delete implements PendingResolutionStore.
func (s *InMemoryPendingResolutionStore) Delete(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("escape: PendingResolutionStore.Delete: sessionID is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, sessionID)
	return nil
}

// Len returns the number of pending decisions (debug aid).
func (s *InMemoryPendingResolutionStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}