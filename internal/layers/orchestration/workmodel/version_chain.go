package workmodel

import (
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// VersionChainTTL is the GC TTL for inactive CoW chain entries. Aligned with
// the PR-C IV-3 head-protection requirement: head entries are never GC'd
// regardless of age; only non-head entries older than this TTL are pruned.
const VersionChainTTL = 24 * time.Hour

// VersionChainRegistry holds one VersionChain per session. It is the workmodel-
// side owner of CoW chains; the underlying Hash / Append / RollbackTo live in
// the interfaces package (IV-1: pure types, no D7 sub-package imports).
//
// Concurrency: safe for concurrent use via the embedded mutex. All public
// methods acquire the lock; the returned *VersionChain is a per-call snapshot
// and is safe to use outside the registry.
type VersionChainRegistry struct {
	mu      sync.RWMutex
	chains  map[string]*interfaces.VersionChain
	gcEvery time.Duration
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewVersionChainRegistry returns a registry with the default 24h TTL.
func NewVersionChainRegistry() *VersionChainRegistry {
	return &VersionChainRegistry{
		chains:  make(map[string]*interfaces.VersionChain),
		gcEvery: VersionChainTTL,
		stopCh:  make(chan struct{}),
	}
}

// ChainFor returns the chain for sessionID, creating an empty one on first
// access. The returned chain is a snapshot; subsequent Append/RollbackTo
// calls on it do NOT affect this registry (use ChainFor fresh each time).
func (r *VersionChainRegistry) ChainFor(sessionID string) *interfaces.VersionChain {
	r.mu.RLock()
	c, ok := r.chains[sessionID]
	r.mu.RUnlock()
	if ok {
		return c
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.chains[sessionID]; ok {
		return c
	}
	c = interfaces.NewVersionChain()
	r.chains[sessionID] = c
	return c
}

// Append appends content to the chain for sessionID and persists the new
// chain back into the registry. Returns the new head hash.
//
// Concurrency: the read-modify-write is wrapped in a write lock to avoid the
// lost-update race where two concurrent callers both read the same base
// chain, each produce their own next snapshot, and the second writer
// overwrites the first's append (PR-C IV-2 immutability invariant).
func (r *VersionChainRegistry) Append(sessionID string, content []byte, reason string) (interfaces.Hash, *interfaces.VersionChain, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.chains[sessionID]
	if !ok {
		cur = interfaces.NewVersionChain()
		r.chains[sessionID] = cur
	}
	h, next, err := cur.Append(content, reason)
	if err != nil {
		return interfaces.EmptyHash, nil, err
	}
	r.chains[sessionID] = next
	return h, next, nil
}

// Rollback rewinds the chain for sessionID to the given hash. Wrapped in the
// registry's lock to avoid the same RMW race as Append.
func (r *VersionChainRegistry) Rollback(sessionID string, h interfaces.Hash) (*interfaces.VersionChain, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.chains[sessionID]
	if !ok {
		return nil, interfaces.NewCoWVersionChainBrokenError()
	}
	next, err := cur.RollbackTo(h)
	if err != nil {
		return nil, err
	}
	r.chains[sessionID] = next
	return next, nil
}

// GCAll prunes non-head entries older than TTL across all sessions. Returns
// the total number of entries deleted and the number of sessions touched.
//
// This is exposed for tests and for the periodic worker below; production
// callers normally just call Start to run a 24h background worker.
func (r *VersionChainRegistry) GCAll(ttl time.Duration) (int, int, error) {
	if ttl <= 0 {
		ttl = VersionChainTTL
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	totalDeleted := 0
	touched := 0
	for sid, chain := range r.chains {
		deleted, next, err := chain.GC(ttl)
		if err != nil {
			slog.Warn("version_chain_registry.gc.failed",
				"session", sid,
				"err", err.Error())
			continue
		}
		if deleted > 0 {
			r.chains[sid] = next
			totalDeleted += deleted
			touched++
		}
	}
	return totalDeleted, touched, nil
}

// SessionCount returns the number of tracked sessions (for tests/metrics).
func (r *VersionChainRegistry) SessionCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.chains)
}

// Start launches the 24h GC worker in a goroutine. Returns immediately;
// call Stop to terminate. The worker runs GC at the registry's gcEvery
// interval (default 24h) and logs deletion counts via slog.
func (r *VersionChainRegistry) Start() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(r.gcEvery)
		defer ticker.Stop()
		for {
			select {
			case <-r.stopCh:
				return
			case <-ticker.C:
				deleted, touched, err := r.GCAll(VersionChainTTL)
				if err != nil {
					slog.Warn("version_chain_registry.gc_worker.error", "err", err.Error())
					continue
				}
				if deleted > 0 {
					slog.Info("version_chain_registry.gc_worker.pruned",
						"deleted", deleted,
						"sessions_touched", touched)
				}
			}
		}
	}()
}

// Stop terminates the GC worker. Blocks until the goroutine exits or the
// configured timeout elapses.
func (r *VersionChainRegistry) Stop() {
	select {
	case <-r.stopCh:
		// already closed
	default:
		close(r.stopCh)
	}
	r.wg.Wait()
}
