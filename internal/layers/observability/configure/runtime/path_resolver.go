// Package runtime exposes lightweight, in-process runtime counters used by
// the harness-unification effort to track which path a request took through
// the LLM↔Tool loop. The counters are also surfaced through the D5 metric
// `runtime.path_resolved_total{path="query_loop|legacy_harness"}`.
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
	// PathQueryLoop labels the primary LLM↔Tool path counter. After
	// DM-20260618-010 the loop runs in D7; D2 Process() still records
	// this label when delegating to PreparedTurnRunner.
	PathQueryLoop PathKind = "query_loop"
	// PathLegacyHarness is the retired harness bootstrap path (removed v6.5.0).
	// PathRegressionProbe (D6) will fail the build if any production
	// request takes this path.
	PathLegacyHarness PathKind = "legacy_harness"
)

// pathCounters holds the cumulative count per PathKind.
type pathCounters struct {
	queryLoop     atomic.Int64
	legacyHarness atomic.Int64
}

var (
	globalOnce sync.Once
	global     *pathCounters
)

// Global returns the process-wide PathCounters singleton. The returned
// pointer is safe to use from any goroutine; the struct itself is never
// re-assigned.
func Global() *pathCounters {
	globalOnce.Do(func() {
		global = &pathCounters{}
	})
	return global
}

// Reset zeroes all path counters. Test-only convenience.
func Reset() {
	g := Global()
	g.queryLoop.Store(0)
	g.legacyHarness.Store(0)
}

// PathCounters tracks how many requests have been resolved via each
// LLM↔Tool path. Use the package-level helpers (Inc, Snapshot) unless
// you need direct access to Global() for testing.
type PathCounters struct {
	// QueryLoop is the count of requests resolved via the QueryLoop path.
	QueryLoop int64
	// LegacyHarness is the count of requests resolved via the legacy
	// harness path. Any value > 0 is a regression that fails the
	// PathRegressionProbe.
	LegacyHarness int64
}

// Snapshot returns a stable, point-in-time view of the global counters.
func Snapshot() PathCounters {
	g := Global()
	return PathCounters{
		QueryLoop:     g.queryLoop.Load(),
		LegacyHarness: g.legacyHarness.Load(),
	}
}

// Record increments the counter for the given path. Safe to call from any
// goroutine. Unknown path kinds are silently ignored (we never want a
// rogue caller to crash the runtime).
func Record(p PathKind) {
	switch p {
	case PathQueryLoop:
		Global().queryLoop.Add(1)
	case PathLegacyHarness:
		Global().legacyHarness.Add(1)
	}
}
