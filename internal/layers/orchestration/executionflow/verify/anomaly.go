// Package verify is the v6.0.0 6 S 精简 S4 角色（Costly Signaler +
// Certifier）的 Span 发射层。它把现有 workmodel.SystemAnomalyAggregator
// 的布尔聚合结果升级为带 anomaly_kind / severity / threshold /
// evidence_id 的 DetectSystemAnomaly，配套发射 system.anomaly_detect
// Span（S4-A47 P0）。
//
// 关系：
//
//	executionflow/verify/anomaly.go (NEW) → orchtypes/system_anomaly_wiring.go
//	  → workmodel/system_anomaly.go (existing bool aggregator)
//
// 不重写 workmodel 的现有 Evaluate —— 6 S 精简保持 LP-1 兼容（Phase 4
// PR-D4 的 UncertaintyCoord Value=0.95 路径 0 变化）。
package verify

import (
	"context"
	"strconv"

	"github.com/devrix/devrix/internal/layers/orchestration/d7spans"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// AnomalyKind enumerates the 4 system-level anomaly types the Certifier
// role can surface. v6.0.0 currently emits cat_system_aggregate as the
// forward-compatible default — the 4 specific detectors (rate_spike /
// quota_exceeded / schema_violation / verifier_abstain_loop) are slated
// for the v6.1 work-item and will override this default by passing a
// non-empty overrideKind to DetectSystemAnomaly.
type AnomalyKind string

const (
	// AnomalyKindCatSystemAggregate — current default (Phase 4 PR-D4 bool
	// aggregator). Captures any signal that crosses the CatSystem ratio
	// threshold without distinguishing the underlying cause.
	AnomalyKindCatSystemAggregate AnomalyKind = "cat_system_aggregate"

	// AnomalyKindRateSpike — reserved for v6.1 rate-based detectors.
	AnomalyKindRateSpike AnomalyKind = "rate_spike"

	// AnomalyKindQuotaExceeded — reserved for v6.1 quota detectors.
	AnomalyKindQuotaExceeded AnomalyKind = "quota_exceeded"

	// AnomalyKindSchemaViolation — reserved for v6.1 schema validators.
	AnomalyKindSchemaViolation AnomalyKind = "schema_violation"

	// AnomalyKindVerifierAbstainLoop — reserved for v6.1 verifier
	// abstention-loop detectors.
	AnomalyKindVerifierAbstain AnomalyKind = "verifier_abstain_loop"
)

// Severity bands the anomaly detection by how concentrated the
// CatSystem signal is. The bands are computed from the ratio of
// CatSystem anomalies vs total anomalies, so a "high" reading means a
// strong systemic signal and a "low" reading means borderline (mostly
// CatBusiness) noise.
type Severity string

const (
	SeverityNone   Severity = "none"
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

// DetectionResult carries the output of DetectSystemAnomaly. It is
// returned alongside the bool so callers (ObserveNode, UncertaintyCoord
// builder, dashboards) can surface the kind / severity / evidence_id
// without re-running the aggregator.
type DetectionResult struct {
	Triggered  bool
	Kind       AnomalyKind
	Severity   Severity
	Threshold  int    // total anomaly count observed (after the threshold was applied)
	EvidenceID string // stable id derived from sessionID + anomaly count
}

// DefaultDetectionThresholds — convenience tuple matching the
// workmodel.SystemAnomalyAggregator defaults (Phase 4 PR-D4).
var DefaultDetectionThresholds = struct {
	Count int
	Ratio float64
}{Count: workmodel.DefaultAnomalyThreshold, Ratio: workmodel.DefaultMinCatSystemRatio}

// DetectSystemAnomaly is the v6.0.0 6 S 精简 Span-emitting entry point
// for the S4 Certifier role. It wraps orchtypes.EvaluateSystemAnomaly so
// the existing wiring (Phase 4 PR-D4 → UncertaintyCoord Value=0.95
// propagation) is preserved, and emits a system.anomaly_detect Span with
// anomaly_kind / severity / threshold / evidence_id attributes for
// Jaeger.
//
// Parameters:
//
//	sessionID     — owning session (Span attribute)
//	report        — UncertaintyReport from ObserveNode
//	threshold     — min total anomaly count (≤ 0 → default 3)
//	minCatRatio   — min CatSystem/total ratio (≤ 0 → default 0.5)
//	overrideKind  — optional explicit AnomalyKind (empty → cat_system_aggregate)
//
// The function emits the Span unconditionally (no-op when the bridge is
// unset) so even "false" detections are observable in Jaeger.
func DetectSystemAnomaly(ctx context.Context, sessionID string, report orchtypes.UncertaintyReport, threshold int, minCatRatio float64, overrideKind AnomalyKind) DetectionResult {
	if threshold <= 0 {
		threshold = workmodel.DefaultAnomalyThreshold
	}
	if minCatRatio <= 0 {
		minCatRatio = workmodel.DefaultMinCatSystemRatio
	}

	triggered := orchtypes.EvaluateSystemAnomaly(report)

	catSystemCount := 0
	for _, o := range report.Anomalies {
		if o.Category == orchtypes.CatSystem {
			catSystemCount++
		}
	}
	total := len(report.Anomalies)
	ratio := 0.0
	if total > 0 {
		ratio = float64(catSystemCount) / float64(total)
	}

	kind := AnomalyKindCatSystemAggregate
	if overrideKind != "" {
		kind = overrideKind
	}

	severity := computeSeverity(triggered, ratio)

	evidenceID := buildEvidenceID(sessionID, total, catSystemCount)

	res := DetectionResult{
		Triggered:  triggered,
		Kind:       kind,
		Severity:   severity,
		Threshold:  total,
		EvidenceID: evidenceID,
	}

	end := d7spans.EmitSystemAnomalyDetect(
		ctx,
		sessionID,
		string(kind),
		string(severity),
		strconv.Itoa(threshold),
		evidenceID,
	)
	end(nil)
	return res
}

// computeSeverity maps a triggered flag + ratio into the 4-band scale.
// Non-triggered detections always return SeverityNone so dashboards can
// filter "interesting" anomalies via != none.
func computeSeverity(triggered bool, ratio float64) Severity {
	if !triggered {
		return SeverityNone
	}
	switch {
	case ratio >= 1.0:
		return SeverityHigh
	case ratio >= 0.75:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// buildEvidenceID produces a stable evidence identifier from sessionID +
// total + catSystem counts. The format is "sessionID:total:catSystem"
// which is unique per detection run and human-readable in the Jaeger UI.
func buildEvidenceID(sessionID string, total, catSystem int) string {
	if sessionID == "" {
		sessionID = "unknown"
	}
	return sessionID + ":" + strconv.Itoa(total) + ":" + strconv.Itoa(catSystem)
}