// Package observability owns the multi-agent local metrics registry.
//
// It exposes a tiny, no-allocation counter for the D5 metric
// runtime.fork_session_view_total{policy=cow|snapshot|shared}.
//
// The counter is backed by an atomic int per label value, which keeps
// the -race path in the hot Fork path allocation-free.
package observability

import (
	"sync"
	"sync/atomic"
)

// D5Sink is a typed callback that mirrors a Policy's counter value to a
// D5 metric registry. Implementations should be allocation-free; the
// sink is invoked on every counter bump.
//
// v1.0 ships without a sink wired in (the local atomic counter is the
// source of truth for the in-process probe). The signature is part of
// the public API so external wiring (e.g. the D5 observability init
// path) can plug in without changing Fork's call shape.
type D5Sink func(p Policy, delta int64)

// Policy is the SessionView fork policy label. v1.0 ships cow only;
// snapshot/shared are reserved for v2.0 (DM-20260611-005).
type Policy string

const (
	PolicyCow      Policy = "cow"
	PolicySnapshot Policy = "snapshot"
	PolicyShared   Policy = "shared"
)

var (
	policyMu sync.RWMutex
	counters = map[Policy]*int64{}
	d5Sink   D5Sink
)

// IncForkSessionView bumps the counter for the given policy label.
// Safe under -race; uses an RWMutex to lazily allocate a slot.
func IncForkSessionView(policy string) {
	IncForkSessionViewPolicy(Policy(policy))
}

// IncForkSessionViewPolicy is the typed variant.
func IncForkSessionViewPolicy(p Policy) {
	ctr := counterFor(p)
	atomic.AddInt64(ctr, 1)
	if sink := currentSink(); sink != nil {
		sink(p, 1)
	}
}

// SetD5Sink installs (or clears, when nil) the D5 sink used to mirror
// counter bumps to the global D5 metric registry. Idempotent.
func SetD5Sink(s D5Sink) {
	policyMu.Lock()
	d5Sink = s
	policyMu.Unlock()
}

func currentSink() D5Sink {
	policyMu.RLock()
	s := d5Sink
	policyMu.RUnlock()
	return s
}

// ForkSessionViewValue returns the current count for the given policy.
// Returns 0 when the counter has never been bumped.
func ForkSessionViewValue(p Policy) int64 {
	ctr := counterFor(p)
	return atomic.LoadInt64(ctr)
}

// Reset clears all counters. Test-only; do not call from production code.
func Reset() {
	policyMu.Lock()
	defer policyMu.Unlock()
	for _, c := range counters {
		atomic.StoreInt64(c, 0)
	}
}

// Snapshot returns a map of policy -> count for the probes to read.
func Snapshot() map[Policy]int64 {
	policyMu.RLock()
	defer policyMu.RUnlock()
	out := make(map[Policy]int64, len(counters))
	for p, c := range counters {
		out[p] = atomic.LoadInt64(c)
	}
	return out
}

func counterFor(p Policy) *int64 {
	policyMu.RLock()
	c, ok := counters[p]
	policyMu.RUnlock()
	if ok {
		return c
	}
	policyMu.Lock()
	defer policyMu.Unlock()
	if c, ok := counters[p]; ok {
		return c
	}
	var v int64
	counters[p] = &v
	return &v
}
