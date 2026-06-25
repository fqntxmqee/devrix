// Package learn: ReputationStore interface + InMemoryReputationStore
// (PR-E5 E5.3).
//
// ReputationStore persists ReputationEvidence across Learn calls so the
// Bayesian Update on Verdict #N can use the prior accumulated from
// Verdicts #1..N-1. The Learn node owns Get/Update/List semantics; concrete
// stores (in-memory, file-backed, D2 ContextEngine-backed) implement the
// interface.
package learn

import (
	"context"
	"sync"
)

// ReputationStore is the LP-3 + LP-5 跨会话可追溯 persistence contract.
type ReputationStore interface {
	// Get returns the ReputationEvidence for sessionID, or (nil, nil) if
	// absent (caller treats absent as cold start). Returns
	// ErrReputationStoreUnavailable for IO / serialization failures.
	Get(ctx context.Context, sessionID string) (*ReputationEvidence, error)

	// Update persists evidence. Nil evidence returns
	// ErrReputationStoreUnavailable (fail-fast). The store is allowed to
	// re-initialize sessionID rows when the prior is absent.
	Update(ctx context.Context, evidence *ReputationEvidence) error

	// List returns up to `limit` ReputationEvidence rows whose TrackMode
	// matches. Pass limit=0 to use a default cap (256). Empty trackMode
	// returns rows of any track mode.
	List(ctx context.Context, trackMode TrackMode, limit int) ([]*ReputationEvidence, error)
}

// defaultListLimit is the cap applied when List is called with limit ≤ 0.
const defaultListLimit = 256

// InMemoryReputationStore is the test/dev-default implementation. It holds
// all rows in a map[string]*ReputationEvidence guarded by sync.RWMutex.
//
// Production deployments should swap in a D2 ContextEngine-backed
// implementation that survives process restarts; the InMemory variant is
// explicitly scoped to dev/test/single-session runtime.
type InMemoryReputationStore struct {
	store map[string]*ReputationEvidence
	mu    sync.RWMutex
}

// NewInMemoryReputationStore constructs an empty InMemoryReputationStore.
func NewInMemoryReputationStore() *InMemoryReputationStore {
	return &InMemoryReputationStore{
		store: make(map[string]*ReputationEvidence),
	}
}

// Get returns the row for sessionID, or (nil, nil) if absent. Returns
// ErrReputationStoreUnavailable only if the store is in an unrecoverable
// state — currently never (in-memory is always recoverable).
func (s *InMemoryReputationStore) Get(ctx context.Context, sessionID string) (*ReputationEvidence, error) {
	if sessionID == "" {
		return nil, ErrReputationStoreUnavailable
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store[sessionID], nil
}

// Update persists evidence. Returns ErrReputationStoreUnavailable when
// evidence is nil.
func (s *InMemoryReputationStore) Update(ctx context.Context, evidence *ReputationEvidence) error {
	if evidence == nil {
		return ErrReputationStoreUnavailable
	}
	if evidence.SessionID == "" {
		return ErrReputationStoreUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Defensive copy so external mutation of the caller's struct cannot
	// contaminate the store (LP-3 immutability).
	clone := *evidence
	s.store[evidence.SessionID] = &clone
	return nil
}

// List returns up to `limit` rows whose TrackMode matches. Empty trackMode
// returns all rows. Limit ≤ 0 → defaultListLimit (256).
func (s *InMemoryReputationStore) List(ctx context.Context, trackMode TrackMode, limit int) ([]*ReputationEvidence, error) {
	if limit <= 0 {
		limit = defaultListLimit
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*ReputationEvidence, 0, len(s.store))
	for _, ev := range s.store {
		if trackMode != "" && ev.TrackMode != trackMode {
			continue
		}
		out = append(out, ev)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}