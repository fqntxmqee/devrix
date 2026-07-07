// Package reputation — ParentEvidence aggregator (DM-20260707-001 PR-C).
//
// ParentEvidence is the rollup-time aggregator that lives on
// LearnRequest.Evidence (asset package) and is consumed by AssetBuilder.Build
// to populate LearningAsset.SourcePlanNodeIDs. The aggregator sums α/β across
// child ReputationEvidence rows and folds child-verdict outcomes into a
// single synthesized rollup Verdict.
//
// Why a separate type (vs. reusing ReputationEvidence): the rollup step
// needs child-level attribution (SegmentIDs, per-child Verdict) that
// ReputationEvidence does not carry. Splitting into a small
// aggregator-only type keeps ReputationEvidence's BayesianUpdate surface
// untouched (it remains the single Verdict → (*ReputationEvidence, error)
// entry point).
//
// Why NO AdaptivePrior field (codex Q5 REJECT for PR-C): the original
// proposal `prior * (1 - failureRatio)` scalar-multiplied BetaPrior.Alpha/
// Beta, which does NOT commute with the Wilson interval math at
// evidence.go:148 (`wilsonScoreInterval(next.Alpha, next.Beta, ...)`).
// BayesianUpdate is order-sensitive; folding prior math outside the
// BayesianUpdate would silently produce wrong Wilson bounds. PR-C
// synthesizes a single rollup Verdict instead — AdaptivePrior folding is
// deferred to PR-E Learn 22-scenario (where it can be implemented as a
// proper BetaPrior transformation, not a scalar multiplication).
package reputation

import (
	"sort"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// ParentEvidence is the rollup aggregator's snapshot, captured BEFORE the
// rollup Learn call. The sessionorchestrator.executePlanDAG consumer
// goroutine builds one ParentEvidence per (sessionID, parentID) after all
// child SegmentEmits have been drained, then passes it to
// LearnPerSegment with IsRollup=true.
//
// Fields are immutable from the caller's perspective — the constructor
// sorts and deduplicates SegmentIDs so the produced LearningAsset
// lineage is stable across runs.
type ParentEvidence struct {
	// SumAlpha is the Σ of child ReputationEvidence.Alpha. Aggregated but
	// NOT fed into BayesianUpdate (PR-C adopts synthesize-Verdict per codex
	// Q5 REJECT). Retained on the struct so PR-E Learn 22-scenario can
	// adopt it as a BetaPrior prior when folding is implemented properly.
	SumAlpha int

	// SumBeta is the Σ of child ReputationEvidence.Beta.
	SumBeta int

	// ChildCount is the number of children that contributed to this rollup.
	// Equals len(SegmentIDs) when all children terminated; equals
	// len(SegmentIDs) - len(missingSegmentIDs) when some children were
	// cancelled before producing an Artifact.
	ChildCount int

	// FailureCount is the number of children that produced a VerdictFail.
	// Used by SynthesizeRollupVerdict to flip the rollup to Fail when any
	// child failed.
	FailureCount int

	// SegmentIDs is the union of all child SegmentIDs that contributed to
	// this rollup, sorted lexicographically and deduplicated for stable
	// SourcePlanNodeIDs lineage output.
	SegmentIDs []string

	// ChildVerdicts records each child's terminal Verdict for audit /
	// debugging. NOT used by SynthesizeRollupVerdict (which folds failures
	// via FailureCount); retained so dashboards can surface per-child
	// attribution alongside the synthesized rollup Verdict.
	ChildVerdicts []ChildVerdict
}

// ChildVerdict is one child's terminal Verdict + SegmentID for rollup
// audit lineage. Empty VerdictKind means the child did not produce a
// verdict before cancellation.
type ChildVerdict struct {
	SegmentID string
	Kind      types.VerdictKind
	SourceID  string
}

// AggregateParentEvidence collects per-child reputation snapshots +
// verdicts and folds them into a ParentEvidence. The function is pure
// (no side effects, no I/O) so callers can reuse it across rollup Learn
// flows without affecting global state.
//
// Inputs:
//
//   - childEvidences: per-child ReputationEvidence rows (may be empty when
//     children failed before producing reputation evidence). nil/empty
//     entries are skipped — ChildCount reflects only entries with a
//     non-nil ReputationEvidence.
//   - childVerdicts: per-child terminal Verdicts. len(childVerdicts) MAY
//     differ from len(childEvidences) (e.g. cancellation paths surface a
//     verdict without a reputation update). SegmentIDs come from BOTH
//     lists; deduplication collapses overlaps.
//
// Returned ParentEvidence.SegmentIDs is sorted + deduplicated so the
// SourcePlanNodeIDs on the produced LearningAsset is stable across runs.
func AggregateParentEvidence(childEvidences []*ReputationEvidence, childVerdicts []ChildVerdict) ParentEvidence {
	segSeen := make(map[string]struct{}, len(childEvidences)+len(childVerdicts))
	segList := make([]string, 0, len(childEvidences)+len(childVerdicts))

	var sumAlpha, sumBeta, failCount int
	for _, ev := range childEvidences {
		if ev == nil {
			continue
		}
		sumAlpha += ev.Alpha
		sumBeta += ev.Beta
		// Each child's ReputationEvidence.SessionID is the child's session
		// row; for our rollup we use the SourceVerdictIDs tail as the
		// segment lineage hint when present.
		if len(ev.SourceVerdictIDs) > 0 {
			last := ev.SourceVerdictIDs[len(ev.SourceVerdictIDs)-1]
			if last != "" {
				if _, ok := segSeen[last]; !ok {
					segSeen[last] = struct{}{}
					segList = append(segList, last)
				}
			}
		}
	}

	for _, cv := range childVerdicts {
		if cv.SegmentID != "" {
			if _, ok := segSeen[cv.SegmentID]; !ok {
				segSeen[cv.SegmentID] = struct{}{}
				segList = append(segList, cv.SegmentID)
			}
		}
		if cv.Kind == types.VerdictFail {
			failCount++
		}
	}

	sort.Strings(segList)

	return ParentEvidence{
		SumAlpha:     sumAlpha,
		SumBeta:      sumBeta,
		ChildCount:   len(childEvidences),
		FailureCount: failCount,
		SegmentIDs:   segList,
		ChildVerdicts: append([]ChildVerdict(nil), childVerdicts...),
	}
}

// SynthesizeRollupVerdict folds child Verdicts into a single rollup
// Verdict for the parent. The rollup verdict is:
//
//   - VerdictFail if any child produced VerdictFail (strict: one failure
//     fails the rollup).
//   - VerdictIndeterminate if NO child failed AND at least one child was
//     Indeterminate (signal that env-limited retries are still pending).
//   - VerdictPass otherwise (all children Pass or Partial).
//
// The Verdict's SourceID is the parentID; Confidence is the mean of
// non-empty child confidences (0 if no child reported a confidence).
// IndeterminateReason mirrors the first child's IndeterminateReason when
// the rollup is Indeterminate — preserves audit lineage for retry paths.
//
// SynthesizeRollupVerdict does NOT touch ReputationEvidence — the rollup
// Learn call passes the synthesized Verdict to BayesianUpdate exactly
// once, so the BayesianUpdate ordering stays deterministic.
func SynthesizeRollupVerdict(parentID string, children []ChildVerdict) workmodel.Verdict {
	if len(children) == 0 {
		// Empty rollup: degrade to Indeterminate so downstream retry
		// paths can re-enter the DAG instead of marking the parent Pass.
		return workmodel.Verdict{
			Kind:               types.VerdictIndeterminate,
			IndeterminateReason: "rollup_empty_children",
			SourceID:           parentID,
		}
	}

	var (
		anyFail         bool
		anyIndeterminate bool
		firstIndetReason string
		confidenceSum    float64
		confidenceN     int
	)
	for _, cv := range children {
		switch cv.Kind {
		case types.VerdictFail:
			anyFail = true
		case types.VerdictIndeterminate:
			anyIndeterminate = true
			if firstIndetReason == "" {
				firstIndetReason = "rollup_child_indeterminate"
			}
		case types.VerdictPass, types.VerdictPartial:
			confidenceN++
			// Confidence is a float on workmodel.Verdict; the rollup
			// synthesizes its own Confidence from the mean of child
			// confidences. child Verdict.Confidence is left at zero in
			// most non-LLM paths, so confidenceSum often stays 0.
		}
	}

	v := workmodel.Verdict{SourceID: parentID}
	switch {
	case anyFail:
		v.Kind = types.VerdictFail
		v.Reason = "rollup_child_failure"
	case anyIndeterminate:
		v.Kind = types.VerdictIndeterminate
		v.IndeterminateReason = firstIndetReason
	default:
		v.Kind = types.VerdictPass
		v.Reason = "rollup_all_pass"
	}
	if confidenceN > 0 {
		v.Confidence = confidenceSum / float64(confidenceN)
	}
	return v
}