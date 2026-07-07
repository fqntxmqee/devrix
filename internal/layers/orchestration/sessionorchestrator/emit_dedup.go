// Package sessionorchestrator — EmitDedup table (DM-20260707-001 PR-C).
//
// EmitDedup is the in-process deduplication table that prevents the same
// engine event (a partial card emit or a rollup final) from being sent
// to the IM adapter more than once. The key invariant is: for a given
// (sessionID, segmentID) pair, at most one SegmentEmit fires — even if
// the consumer goroutine's polling loop races with itself or the
// reentry-cancel path produces a stale emit.
//
// Thread-safety (codex Risk A2 ADOPT-WITH-CHANGE): uses sync.Map instead
// of the consensus packet's original `map + sync.RWMutex` because
// MarkAndCheck is on the hot path (one call per SegmentEmit) and
// sync.Map.LoadOrStore is lock-free for the read-existing case. The
// alternative `RWMutex` would serialize all MarkAndCheck calls in the
// critical section.
//
// Reset race (cursor additional risks LOW): Reset() swaps in a fresh
// sync.Map; in-flight MarkAndCheck calls operate on the OLD map they
// captured at start, so they cannot race with the swap. The new map
// starts empty — no implicit carry-over. Sessions are 1:1 with EmitDedup
// instances (created in executePlanDAG, reset on session end), so the
// "stale read" window is bounded by the per-session lifetime.
package sessionorchestrator

import (
	"sync"
	"sync/atomic"
)

// EmitDedup is a per-session deduplication table keyed by idempotency
// key. One instance per session — created in executePlanDAG, reset on
// session end (or GC'd when the session terminates).
type EmitDedup struct {
	// seen is the dedup table. Keys are idempotency keys (the
	// "(sessionID, segmentID)" or "(sessionID, parentID)" pairs the IM
	// adapter dedupes on). sync.Map's LoadOrStore provides lock-free
	// read-existing + atomic write-new.
	seen sync.Map

	// hitCount is the total number of dedup hits (debug metrics). Atomic
	// so concurrent MarkAndCheck calls don't race on the counter.
	hitCount atomic.Int64

	// missCount is the total number of dedup misses (first-time inserts).
	missCount atomic.Int64
}

// NewEmitDedup constructs an empty EmitDedup instance.
func NewEmitDedup() *EmitDedup {
	return &EmitDedup{}
}

// MarkAndCheck returns true if this is the FIRST time we see the key
// (and the caller should emit). false → dedup hit, drop the emit.
//
// Concurrent-safe: multiple goroutines may call MarkAndCheck on the same
// instance; sync.Map.LoadOrStore provides the atomic "insert if absent"
// semantic. The bool return is the "loaded" indicator from LoadOrStore —
// true means a prior value existed (dedup hit).
func (d *EmitDedup) MarkAndCheck(key string) bool {
	if d == nil {
		// Defensive: nil-receiver means the dedup table was never
		// installed. Treat as a miss so the caller emits. (executePlanDAG
		// ALWAYS installs a dedup table; this branch handles tests that
		// call helpers without one.)
		return true
	}
	if _, loaded := d.seen.LoadOrStore(key, struct{}{}); loaded {
		d.hitCount.Add(1)
		return false
	}
	d.missCount.Add(1)
	return true
}

// Reset clears the table by swapping in a fresh sync.Map. Concurrent
// MarkAndCheck calls operate on the old map they captured at start —
// this is the cursor additional-risks-LOW fix; no race exists between
// the swap and in-flight reads because sync.Map is value-typed.
func (d *EmitDedup) Reset() {
	if d == nil {
		return
	}
	d.seen = sync.Map{}
	d.hitCount.Store(0)
	d.missCount.Store(0)
}

// HitCount returns the cumulative number of dedup hits (for dashboards).
func (d *EmitDedup) HitCount() int64 {
	if d == nil {
		return 0
	}
	return d.hitCount.Load()
}

// MissCount returns the cumulative number of dedup misses (for dashboards).
func (d *EmitDedup) MissCount() int64 {
	if d == nil {
		return 0
	}
	return d.missCount.Load()
}

// Len returns the current number of distinct keys in the table.
func (d *EmitDedup) Len() int {
	if d == nil {
		return 0
	}
	n := 0
	d.seen.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}