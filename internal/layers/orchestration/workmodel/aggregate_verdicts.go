package workmodel

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/devrix/devrix/internal/shared/types"
)

// AggregationStrategy selects how a list of Verdicts is folded into a
// single Verdict by AggregateVerdicts.
//
// Strategy semantics:
//
//	WeakConjunction   — any PASS → PASS (OR, most permissive)
//	StrongConjunction — all PASS → PASS (AND, strictest)
//	Majority          — PASS > len/2 → PASS (plurality)
//	ThresholdByPass   — PASS ≥ threshold → PASS (configurable)
//
// Phase 4 PR-D1 (DM-20260623-002) introduces this enum so the Verify node
// can fold multi-aspect Verification (Compliance/Timeliness/RootCause/
// Statistical per doc 45 §四) into a single Verdict that drives
// UncertaintyCoord.FromVerifier. Prior to PR-D1 the verifier path only
// supported a single sub-agent call, with no aggregation entry point.
type AggregationStrategy uint8

const (
	// WeakConjunction — any single PASS yields PASS (OR semantics).
	WeakConjunction AggregationStrategy = iota
	// StrongConjunction — all PASS required for PASS (AND semantics).
	StrongConjunction
	// Majority — PASS count strictly greater than len/2 wins.
	Majority
	// ThresholdByPass — PASS count ≥ threshold (default 1) yields PASS.
	ThresholdByPass
)

// String returns the wire format name (snake_case). Unknown strategies
// return a debug-formatted integer so logs stay grep-able.
func (s AggregationStrategy) String() string {
	switch s {
	case WeakConjunction:
		return "weak_conjunction"
	case StrongConjunction:
		return "strong_conjunction"
	case Majority:
		return "majority"
	case ThresholdByPass:
		return "threshold_by_pass"
	default:
		return fmt.Sprintf("AggregationStrategy(%d)", uint8(s))
	}
}

// ParseAggregationStrategy reverses String() to recover the enum value from
// a wire payload. Returns an error on unknown input.
func ParseAggregationStrategy(s string) (AggregationStrategy, error) {
	switch s {
	case "weak_conjunction":
		return WeakConjunction, nil
	case "strong_conjunction":
		return StrongConjunction, nil
	case "majority":
		return Majority, nil
	case "threshold_by_pass":
		return ThresholdByPass, nil
	default:
		return 0, fmt.Errorf("workmodel: unknown AggregationStrategy %q", s)
	}
}

// MarshalJSON encodes the strategy as its String() form.
func (s AggregationStrategy) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON accepts the wire format produced by MarshalJSON. An empty
// string decodes to the zero value (WeakConjunction) for v2 backward
// compatibility with prior single-verifier callers.
func (s *AggregationStrategy) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if str == "" {
		*s = WeakConjunction
		return nil
	}
	parsed, err := ParseAggregationStrategy(str)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

// Verdict is a single Verifier sub-agent output. It is intentionally
// immutable: any field modification should be done via a With* method or
// by constructing a new Verdict (mirrors Plan/Step immutability from
// Phase 2 PR-B1).
type Verdict struct {
	Kind         types.VerdictKind `json:"kind"`
	Confidence   float64           `json:"confidence,omitempty"`
	Reason       string            `json:"reason,omitempty"`
	SourceID     string            `json:"source_id,omitempty"`
	SystemAnomaly bool             `json:"system_anomaly,omitempty"`

	// IndeterminateReason — Phase 5 G8-1 fix extension. When Kind ==
	// VerdictIndeterminate, this field distinguishes the cause so that the
	// BayesianUpdate in the Learn node does NOT pollute α/β on
	// verifier_parse_failure (verifier LLM output format issue ≠ user fault).
	// Possible values (defined as learn-package constants in
	// PendingAssetContent.IndeterminateReason):
	//   ""                   — not INDETERMINATE / no specific cause
	//   "verifier_parse_failure" — Phase 5 G8-1 fix path
	//   "env_limited"        — env-level transient failure (network / IO)
	//   "user_decision_pending" — MVE checkpoint pending (Phase 5 PR-E5)
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

// AggregateVerdicts folds a slice of Verdicts into a single Verdict
// according to strategy. Empty input returns INDETERMINATE; single input
// is returned directly; homogeneous input is returned with averaged
// confidence.
//
// All four strategies share the following invariants:
//
//   - Empty input → Verdict{Kind: VerdictIndeterminate, Reason: "empty_verdict_set"}
//   - Single input → returned verbatim (no aggregation cost)
//   - Homogeneous input → returned with averaged Confidence and the
//     longest Reason (most specific explanation)
//
// Confidence aggregation is arithmetic mean; Reason aggregation is
// "longest wins" so the most informative verdict drives the aggregated
// explanation.
func AggregateVerdicts(verdicts []Verdict, strategy AggregationStrategy) Verdict {
	if len(verdicts) == 0 {
		return Verdict{
			Kind:   types.VerdictIndeterminate,
			Reason: "empty_verdict_set",
		}
	}
	if len(verdicts) == 1 {
		return verdicts[0]
	}

	// Homogeneous shortcut: if all verdicts share the same Kind, no
	// strategy dispatch is needed.
	allSame := true
	for i := 1; i < len(verdicts); i++ {
		if verdicts[i].Kind != verdicts[0].Kind {
			allSame = false
			break
		}
	}
	if allSame {
		return aggregateMeta(verdicts)
	}

	switch strategy {
	case WeakConjunction:
		return aggregateWeakConjunction(verdicts)
	case StrongConjunction:
		return aggregateStrongConjunction(verdicts)
	case Majority:
		return aggregateMajority(verdicts)
	case ThresholdByPass:
		return aggregateThresholdByPass(verdicts, 1)
	default:
		return Verdict{
			Kind:   types.VerdictIndeterminate,
			Reason: "unknown_strategy",
		}
	}
}

// aggregateMeta computes averaged Confidence + longest Reason for a
// homogeneous-kind verdict slice.
//
// Metadata preservation (homogeneous shortcut only):
//
//	SourceID            — comma-joined deduplicated union of all
//	                      non-empty SourceIDs (the original "first
//	                      wins" silently dropped cross-source provenance).
//	IndeterminateReason — longest non-empty value across the slice
//	                      (matches the Reason aggregation policy).
//	SystemAnomaly       — OR-aggregate (any true → true) so a single
//	                      anomaly in the set propagates to the result.
//	Kind                 — preserved from verdicts[0] (homogeneous
//	                      invariant guarantees all entries match).
func aggregateMeta(verdicts []Verdict) Verdict {
	if len(verdicts) == 0 {
		return Verdict{Kind: types.VerdictIndeterminate}
	}
	out := verdicts[0]
	confSum := out.Confidence
	longestReason := out.Reason
	seenSource := map[string]struct{}{}
	if out.SourceID != "" {
		seenSource[out.SourceID] = struct{}{}
	}
	systemAnomaly := out.SystemAnomaly
	longestIndet := out.IndeterminateReason
	for i := 1; i < len(verdicts); i++ {
		confSum += verdicts[i].Confidence
		if len(verdicts[i].Reason) > len(longestReason) {
			longestReason = verdicts[i].Reason
		}
		if verdicts[i].SystemAnomaly {
			systemAnomaly = true
		}
		if len(verdicts[i].IndeterminateReason) > len(longestIndet) {
			longestIndet = verdicts[i].IndeterminateReason
		}
		if verdicts[i].SourceID != "" {
			if _, dup := seenSource[verdicts[i].SourceID]; !dup {
				seenSource[verdicts[i].SourceID] = struct{}{}
			}
		}
	}
	out.Confidence = confSum / float64(len(verdicts))
	out.Reason = longestReason
	out.SystemAnomaly = systemAnomaly
	out.IndeterminateReason = longestIndet
	// Build the deduplicated SourceID union (preserve first-seen order).
	if len(seenSource) == 0 {
		out.SourceID = ""
	} else if len(seenSource) == 1 {
		for id := range seenSource {
			out.SourceID = id
		}
	} else {
		ids := make([]string, 0, len(seenSource))
		// First-seen order: re-walk verdicts so the joined SourceID
		// matches the input order, not the map iteration order.
		ordered := make(map[string]struct{}, len(seenSource))
		for _, v := range verdicts {
			if v.SourceID == "" {
				continue
			}
			if _, dup := ordered[v.SourceID]; dup {
				continue
			}
			if _, present := seenSource[v.SourceID]; !present {
				continue
			}
			ordered[v.SourceID] = struct{}{}
			ids = append(ids, v.SourceID)
		}
		out.SourceID = strings.Join(ids, ",")
	}
	return out
}

// aggregateWeakConjunction: OR semantics. Any PASS → PASS; any FAIL →
// FAIL; otherwise INDETERMINATE.
func aggregateWeakConjunction(verdicts []Verdict) Verdict {
	var hasPass, hasFail bool
	confSum := 0.0
	longestReason := ""
	for _, v := range verdicts {
		switch v.Kind {
		case types.VerdictPass:
			hasPass = true
		case types.VerdictFail:
			hasFail = true
		}
		confSum += v.Confidence
		if len(v.Reason) > len(longestReason) {
			longestReason = v.Reason
		}
	}
	avgConf := confSum / float64(len(verdicts))
	out := Verdict{Confidence: avgConf, Reason: longestReason}
	switch {
	case hasPass:
		out.Kind = types.VerdictPass
	case hasFail:
		out.Kind = types.VerdictFail
	default:
		out.Kind = types.VerdictIndeterminate
	}
	return out
}

// aggregateStrongConjunction: AND semantics. All PASS → PASS; any FAIL →
// FAIL; any INDETERMINATE (with no FAIL) → INDETERMINATE.
func aggregateStrongConjunction(verdicts []Verdict) Verdict {
	var hasFail, hasIndeterminate bool
	confSum := 0.0
	longestReason := ""
	for _, v := range verdicts {
		switch v.Kind {
		case types.VerdictFail:
			hasFail = true
		case types.VerdictIndeterminate:
			hasIndeterminate = true
		}
		confSum += v.Confidence
		if len(v.Reason) > len(longestReason) {
			longestReason = v.Reason
		}
	}
	avgConf := confSum / float64(len(verdicts))
	out := Verdict{Confidence: avgConf, Reason: longestReason}
	switch {
	case hasFail:
		out.Kind = types.VerdictFail
	case hasIndeterminate:
		out.Kind = types.VerdictIndeterminate
	default:
		out.Kind = types.VerdictPass
	}
	return out
}

// aggregateMajority: plurality with strict greater-than-half. PASS >
// len/2 → PASS; FAIL > len/2 → FAIL; otherwise INDETERMINATE.
func aggregateMajority(verdicts []Verdict) Verdict {
	passCount, failCount := 0, 0
	confSum := 0.0
	longestReason := ""
	for _, v := range verdicts {
		switch v.Kind {
		case types.VerdictPass:
			passCount++
		case types.VerdictFail:
			failCount++
		}
		confSum += v.Confidence
		if len(v.Reason) > len(longestReason) {
			longestReason = v.Reason
		}
	}
	avgConf := confSum / float64(len(verdicts))
	out := Verdict{Confidence: avgConf, Reason: longestReason}
	half := len(verdicts) / 2
	switch {
	case passCount > half:
		out.Kind = types.VerdictPass
	case failCount > half:
		out.Kind = types.VerdictFail
	default:
		out.Kind = types.VerdictIndeterminate
	}
	return out
}

// aggregateThresholdByPass: PASS count ≥ threshold → PASS; otherwise
// INDETERMINATE. Threshold default is 1 (any single PASS wins); callers
// can pass a higher threshold for stricter aggregation.
func aggregateThresholdByPass(verdicts []Verdict, threshold int) Verdict {
	passCount := 0
	confSum := 0.0
	longestReason := ""
	for _, v := range verdicts {
		if v.Kind == types.VerdictPass {
			passCount++
		}
		confSum += v.Confidence
		if len(v.Reason) > len(longestReason) {
			longestReason = v.Reason
		}
	}
	avgConf := confSum / float64(len(verdicts))
	out := Verdict{Confidence: avgConf, Reason: longestReason}
	if passCount >= threshold {
		out.Kind = types.VerdictPass
	} else {
		out.Kind = types.VerdictIndeterminate
	}
	return out
}

// clamp01OrFallback clamps v into [0,1]. If the value is NaN or out of
// range, the fallback is used. Mirrors the helper from PR-A1
// uncertainty.go so the behaviour is consistent across packages.
//
// NaN handling: NaN comparisons always return false (including <, >, ==
// and !=), so without an explicit IsNaN check NaN would leak through
// and pollute downstream aggregates (sum-of-confidence, Bayesian
// inputs, etc.) with NaN, propagating the failure to all subsequent
// computations. Catching NaN at the boundary is cheaper than the
// alternative — defensive checks in every consumer.
func clamp01OrFallback(v, fallback float64) float64 {
	if math.IsNaN(v) {
		return fallback
	}
	if v < 0 || v > 1 {
		return fallback
	}
	return v
}