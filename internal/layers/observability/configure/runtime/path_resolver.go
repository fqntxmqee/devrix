// Package runtime exposes lightweight, in-process runtime counters used to
// track which path a request took through the LLM↔Tool loop. The counters
// are also surfaced through the D5 metric
// `runtime.path_resolved_total{path="d7_turn|legacy_harness"}`.
package runtime

import (
	"sync"
	"sync/atomic"
)

// PathKind identifies which LLM↔Tool path was selected for a request.
//
// The string values are part of the D5 contract: they are emitted verbatim
// as the `path` label on `runtime.path_resolved_total`.
type PathKind string

const (
	// PathD7Turn is the primary LLM↔Tool path (D7 RunTurn / PreparedTurnRunner).
	PathD7Turn PathKind = "d7_turn"
	// PathLegacyHarness is the retired harness bootstrap path (removed v6.5.0).
	PathLegacyHarness PathKind = "legacy_harness"
)

type pathCounters struct {
	d7Turn        atomic.Int64
	legacyHarness atomic.Int64
}

var (
	globalOnce sync.Once
	global     *pathCounters
)

func Global() *pathCounters {
	globalOnce.Do(func() {
		global = &pathCounters{}
	})
	return global
}

// Reset zeroes all path counters. Test-only convenience.
func Reset() {
	g := Global()
	g.d7Turn.Store(0)
	g.legacyHarness.Store(0)
}

// PathCounters tracks how many requests have been resolved via each path.
type PathCounters struct {
	D7Turn        int64
	LegacyHarness int64
}

// Snapshot returns a stable, point-in-time view of the global counters.
func Snapshot() PathCounters {
	g := Global()
	return PathCounters{
		D7Turn:        g.d7Turn.Load(),
		LegacyHarness: g.legacyHarness.Load(),
	}
}

// Record increments the counter for the given path.
func Record(p PathKind) {
	switch p {
	case PathD7Turn:
		Global().d7Turn.Add(1)
	case PathLegacyHarness:
		Global().legacyHarness.Add(1)
	}
}
