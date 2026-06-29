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

// --- DM-20260629-009 PR-C: 3 new metric counters (AC13/14/15) ---
//
// All 3 counters are atomic.Int64 with the same threading contract as
// InterruptMetrics. Snapshot helpers expose JSON-friendly views for the
// Prometheus scrape exporter.

// TaskContractMetrics counts PR-C gate activity. Disabled-by-default
// feature flags mean counters stay at 0 until the flags flip on (so PR-C
// ships with 0 behavior change).
type TaskContractMetrics struct {
	HardEvidenceRejects          atomic.Int64 // verifier rejected Pass (insufficient evidence)
	VersionChainAppends          atomic.Int64 // workmodel.VersionChainRegistry.Append calls
	SimilarityCheckInterceptions atomic.Int64 // similarity Jaccard > InterceptThreshold
}

// Snapshot returns a point-in-time view; safe under concurrent use.
func (m *TaskContractMetrics) Snapshot() TaskContractMetricsSnapshot {
	if m == nil {
		return TaskContractMetricsSnapshot{}
	}
	return TaskContractMetricsSnapshot{
		HardEvidenceRejects:          m.HardEvidenceRejects.Load(),
		VersionChainAppends:          m.VersionChainAppends.Load(),
		SimilarityCheckInterceptions: m.SimilarityCheckInterceptions.Load(),
	}
}

// TaskContractMetricsSnapshot is the JSON-friendly view of TaskContractMetrics.
// Schema-stable: field names are part of the D5 observability contract.
type TaskContractMetricsSnapshot struct {
	HardEvidenceRejects          int64 `json:"hard_evidence_rejects"`
	VersionChainAppends          int64 `json:"versionchain_appends"`
	SimilarityCheckInterceptions int64 `json:"similarity_interceptions"`
}

// TaskContract returns the package-level TaskContractMetrics singleton.
// Counters increment via RecordHardEvidenceReject / RecordVersionChainAppend /
// RecordSimilarityCheckIntercept. nil-safe on every method.
var TaskContract = &TaskContractMetrics{}

// RecordHardEvidenceReject bumps the HardEvidenceRejects counter. PR-C AC15.
func RecordHardEvidenceReject() {
	if TaskContract == nil {
		return
	}
	TaskContract.HardEvidenceRejects.Add(1)
}

// RecordVersionChainAppend bumps the VersionChainAppends counter. PR-C AC13.
func RecordVersionChainAppend() {
	if TaskContract == nil {
		return
	}
	TaskContract.VersionChainAppends.Add(1)
}

// RecordSimilarityCheckIntercept bumps the SimilarityCheckInterceptions
// counter. PR-C AC14.
func RecordSimilarityCheckIntercept() {
	if TaskContract == nil {
		return
	}
	TaskContract.SimilarityCheckInterceptions.Add(1)
}