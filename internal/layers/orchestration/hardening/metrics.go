// Package hardening metrics counters (moved from sessionorchestrator/ in v6.0.0).
package hardening

import "sync/atomic"

// InterruptMetrics counts cancel step failures across all 3 cancelers
// (Wave / D4 / Process). Designed for observability — does NOT block
// caller. The Handle method returns errors.Join aggregated error if
// any step fails.
//
// Threading: counters are atomic; safe for concurrent Handle calls.
// Snapshot uses atomic.Load for each field; values may drift slightly
// under heavy concurrency, but are useful for SRE dashboard counters.
//
// DM-20260621-010 PR-A: replaces the previous "all warn + nil" anti-pattern
// with structured counters + propagated errors.
type InterruptMetrics struct {
	WaveCancelFailed    atomic.Int64
	D4CancelFailed      atomic.Int64
	ProcessCancelFailed atomic.Int64

	HandleCompleted atomic.Int64 // Handle returned nil
	HandleErrored   atomic.Int64 // Handle returned non-nil (≥1 step failed)
}

// Snapshot returns a point-in-time view of all counters.
//
// Safe to call concurrently with Handle; values may be slightly stale
// across fields but the snapshot is internally consistent per field.
func (m *InterruptMetrics) Snapshot() InterruptMetricsSnapshot {
	if m == nil {
		return InterruptMetricsSnapshot{}
	}
	return InterruptMetricsSnapshot{
		WaveCancelFailed:    m.WaveCancelFailed.Load(),
		D4CancelFailed:      m.D4CancelFailed.Load(),
		ProcessCancelFailed: m.ProcessCancelFailed.Load(),
		HandleCompleted:     m.HandleCompleted.Load(),
		HandleErrored:       m.HandleErrored.Load(),
	}
}

// InterruptMetricsSnapshot is the JSON-friendly view of InterruptMetrics.
//
// Schema-stable: field names are part of the D5 observability contract.
// Adding fields is backward-compatible; removing or renaming is breaking.
type InterruptMetricsSnapshot struct {
	WaveCancelFailed    int64 `json:"wave_cancel_failed"`
	D4CancelFailed      int64 `json:"d4_cancel_failed"`
	ProcessCancelFailed int64 `json:"process_cancel_failed"`
	HandleCompleted     int64 `json:"handle_completed"`
	HandleErrored       int64 `json:"handle_errored"`
}

// TotalCancelFailures sums the 3 canceler failure counters. Returns 0
// when m is nil. Used by SLO queries: "p99 cancel success rate".
func (m *InterruptMetrics) TotalCancelFailures() int64 {
	if m == nil {
		return 0
	}
	return m.WaveCancelFailed.Load() + m.D4CancelFailed.Load() + m.ProcessCancelFailed.Load()
}