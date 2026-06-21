// Package freefork metrics counters.
package freefork

import "sync/atomic"

// ForkerMetrics counts fork-related failures. Designed for observability;
// does NOT block caller. Forker returns aggregated errors.Join when
// any counter increments during a Fork call.
//
// Threading: counters are atomic; safe for concurrent Fork calls.
// Snapshot uses atomic.Load for each field.
//
// DM-20260621-010 PR-A: replaces the previous "_ = Sandbox.Exit(...)"
// silent-swallow pattern with structured counters + slog.Warn.
type ForkerMetrics struct {
	Spawned             atomic.Int64
	SpawnFailed         atomic.Int64
	SandboxEnterFailed  atomic.Int64
	SandboxExitFailed   atomic.Int64
	FactoryCreateFailed atomic.Int64
	RollbackTriggered   atomic.Int64
}

// Snapshot returns a point-in-time view of all counters.
//
// Safe to call concurrently with Fork.
func (m *ForkerMetrics) Snapshot() ForkerMetricsSnapshot {
	if m == nil {
		return ForkerMetricsSnapshot{}
	}
	return ForkerMetricsSnapshot{
		Spawned:             m.Spawned.Load(),
		SpawnFailed:         m.SpawnFailed.Load(),
		SandboxEnterFailed:  m.SandboxEnterFailed.Load(),
		SandboxExitFailed:   m.SandboxExitFailed.Load(),
		FactoryCreateFailed: m.FactoryCreateFailed.Load(),
		RollbackTriggered:   m.RollbackTriggered.Load(),
	}
}

// ForkerMetricsSnapshot is the JSON-friendly view of ForkerMetrics.
//
// Schema-stable: field names are part of the D5 observability contract.
type ForkerMetricsSnapshot struct {
	Spawned             int64 `json:"spawned"`
	SpawnFailed         int64 `json:"spawn_failed"`
	SandboxEnterFailed  int64 `json:"sandbox_enter_failed"`
	SandboxExitFailed   int64 `json:"sandbox_exit_failed"`
	FactoryCreateFailed int64 `json:"factory_create_failed"`
	RollbackTriggered   int64 `json:"rollback_triggered"`
}