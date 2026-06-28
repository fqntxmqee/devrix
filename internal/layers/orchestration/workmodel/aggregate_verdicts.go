// Package workmodel — Verdict value object + With* builders.
//
// v2.6.0 (DM-20260629-001): the legacy AggregationStrategy enum +
// AggregateVerdicts fold + 4 serialization helpers (String /
// ParseAggregationStrategy / MarshalJSON / UnmarshalJSON) have been
// removed. Multi-verifier folding is not yet wired in production
// (v6.0.0 always uses a single verifier sub-agent); the API surface
// can be reintroduced in a follow-up Change when the v2.7 MUPS Learn
// node actually drives an aggregation strategy from the Bayesian
// state. Until then, the enum is an attractive nuisance and a source
// of unused test maintenance.
package workmodel

import (
	"math"

	"github.com/devrix/devrix/internal/shared/types"
)

// Verdict is a single Verifier sub-agent output. It is intentionally
// immutable: any field modification should be done via a With* method or
// by constructing a new Verdict (mirrors Plan/Step immutability from
// Phase 2 PR-B1).
type Verdict struct {
	Kind          types.VerdictKind `json:"kind"`
	Confidence    float64           `json:"confidence,omitempty"`
	Reason        string            `json:"reason,omitempty"`
	SourceID      string            `json:"source_id,omitempty"`
	SystemAnomaly bool              `json:"system_anomaly,omitempty"`

	// IndeterminateReason — Phase 5 G8-1 fix extension. When Kind ==
	// VerdictIndeterminate, this field distinguishes the cause so that the
	// BayesianUpdate in the Learn node does NOT pollute α/β on
	// verifier_parse_failure (verifier LLM output format issue ≠ user fault).
	// Possible values (defined as learn-package constants in
	// PendingAssetContent.IndeterminateReason):
	//   ""                       — not INDETERMINATE / no specific cause
	//   "verifier_parse_failure" — Phase 5 G8-1 fix path
	//   "env_limited"            — env-level transient failure (network / IO)
	//   "user_decision_pending"  — MVE checkpoint pending (Phase 5 PR-E5)
	IndeterminateReason string `json:"indeterminate_reason,omitempty"`
}

// WithKind returns a copy with the new Kind.
func (v Verdict) WithKind(k types.VerdictKind) Verdict {
	v.Kind = k
	return v
}

// WithConfidence returns a copy with the new Confidence (clamped to [0,1]).
func (v Verdict) WithConfidence(c float64) Verdict {
	v.Confidence = clamp01OrFallback(c, 0.5)
	return v
}

// clamp01OrFallback returns v clamped to [0,1]; NaN or out-of-range
// inputs return fallback. Shared by Verdict.WithConfidence and
// Evidence.NewEvidence/WithConfidence.
func clamp01OrFallback(v, fallback float64) float64 {
	if math.IsNaN(v) || v < 0 || v > 1 {
		return fallback
	}
	return v
}

// WithReason returns a copy with the new Reason.
func (v Verdict) WithReason(r string) Verdict {
	v.Reason = r
	return v
}

// WithSourceID returns a copy with the new SourceID.
func (v Verdict) WithSourceID(id string) Verdict {
	v.SourceID = id
	return v
}

// WithSystemAnomaly returns a copy with the SystemAnomaly flag set.
// Phase 4 PR-D4 (SystemAnomaly aggregation + ObserveNode wiring) populates
// this flag from SystemAnomalyAggregator.Evaluate; PR-D2 (VerdictToExitReason)
// consumes it as an override that forces ExitReasonSystemAnomaly.
func (v Verdict) WithSystemAnomaly(sa bool) Verdict {
	v.SystemAnomaly = sa
	return v
}

// WithIndeterminateReason returns a copy with the IndeterminateReason set.
// Used by Phase 5 Learn node's BayesianUpdate to distinguish
// verifier_parse_failure (G8-1 fix) from env-limited INDETERMINATE.
func (v Verdict) WithIndeterminateReason(reason string) Verdict {
	v.IndeterminateReason = reason
	return v
}
