// Package execute metrics counters.
package execute

import "sync/atomic"

// ExecutorMetrics counts worker-side sandbox cleanup failures.
//
// DM-20260621-010 PR-B: replaces the previous `_ = e.sandbox.Exit(...)`
// silent-swallow pattern at worker.go:53,106,149 with structured counters
// + slog.Warn, paralleling the ForkerMetrics treatment in
// multiagent/provision/freefork.
//
// Threading: atomic.Int64; safe for concurrent ExecuteSync / ExecuteAsync.
type ExecutorMetrics struct {
	SandboxExitFailed atomic.Int64
}

// Snapshot returns a point-in-time view. Safe on a nil receiver.
func (m *ExecutorMetrics) Snapshot() ExecutorMetricsSnapshot {
	if m == nil {
		return ExecutorMetricsSnapshot{}
	}
	return ExecutorMetricsSnapshot{
		SandboxExitFailed: m.SandboxExitFailed.Load(),
	}
}

// ExecutorMetricsSnapshot is the JSON-friendly view of ExecutorMetrics.
type ExecutorMetricsSnapshot struct {
	SandboxExitFailed int64 `json:"sandbox_exit_failed"`
}