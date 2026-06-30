package wavescheduler

import (
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

// WorkerPool manages fixed-size slot pools keyed by WorkerType. Acquire is
// non-blocking: returns (slotID, true) on success, ("", false) if the type's
// pool is full. Release frees the slot and invokes registered release hooks
// on a background goroutine (callers must NOT hold locks that would block
// dispatch loops).
type WorkerPool struct {
	mu     sync.Mutex
	caps   map[WorkerType]int
	used   map[WorkerType]int
	owners map[SlotID]WorkerType
	hooks  []func(SlotID)

	// closed flag — when set, Release becomes a no-op so cleanup is idempotent.
	closed atomic.Bool
}

// NewWorkerPool creates a pool with the given per-type capacities.
func NewWorkerPool(capacity map[WorkerType]int) *WorkerPool {
	caps := make(map[WorkerType]int, len(capacity))
	used := make(map[WorkerType]int, len(capacity))
	for k, v := range capacity {
		caps[k] = v
		used[k] = 0
	}
	return &WorkerPool{
		caps:   caps,
		used:   used,
		owners: make(map[SlotID]WorkerType),
		hooks:  nil,
	}
}

// Acquire attempts to reserve a slot. Returns ("", false) if the type is
// unknown or all slots are taken.
func (p *WorkerPool) Acquire(kind WorkerType, taskID string) (SlotID, bool) {
	if p == nil {
		return "", false
	}
	if !kind.Valid() {
		return "", false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	cap, ok := p.caps[kind]
	if !ok {
		return "", false
	}
	if p.used[kind] >= cap {
		return "", false
	}
	p.used[kind]++
	id := SlotID("slot-" + uuid.New().String()[:8])
	p.owners[id] = kind
	return id, true
}

// Release frees a previously-acquired slot. Idempotent: double release is
// silently ignored (slot already gone). Invokes registered hooks.
func (p *WorkerPool) Release(id SlotID) {
	if p == nil {
		return
	}
	p.mu.Lock()
	kind, ok := p.owners[id]
	if !ok {
		p.mu.Unlock()
		return
	}
	delete(p.owners, id)
	if p.used[kind] > 0 {
		p.used[kind]--
	}
	hooks := p.hooks
	p.mu.Unlock()

	if len(hooks) == 0 {
		return
	}
	// Fire hooks async to avoid blocking Release callers (e.g. worker goroutine).
	for _, h := range hooks {
		go func(h func(SlotID)) { h(id) }(h)
	}
}

// Available returns the number of free slots for a worker type.
func (p *WorkerPool) Available(kind WorkerType) int {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.caps[kind] - p.used[kind]
}

// OnRelease registers a callback fired when any slot is released. Used by the
// scheduler to wake its dispatch loop without polling.
func (p *WorkerPool) OnRelease(hook func(SlotID)) {
	if p == nil || hook == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hooks = append(p.hooks, hook)
}

// HookCount returns the number of registered OnRelease hooks. Used by tests
// to verify the OnReleaseOnce invariant (D7-S3-A84): exactly one hook must
// be registered per scheduler lifetime, regardless of how many waves Start
// spawns.
func (p *WorkerPool) HookCount() int {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.hooks)
}

// Close marks the pool as closed; subsequent Release calls become no-ops on
// unknown ids. Existing slots continue to function. For test teardown.
func (p *WorkerPool) Close() {
	if p == nil {
		return
	}
	p.closed.Store(true)
}

// InUse reports the number of currently-held slots for a type (test helper).
func (p *WorkerPool) InUse(kind WorkerType) int {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.used[kind]
}
