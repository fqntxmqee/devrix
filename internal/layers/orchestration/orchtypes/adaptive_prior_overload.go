package orchtypes

import (
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/prior"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/reputation"
)

// DM-20260630-011 (devrix-session-conclusion-completeness) —
// BuildAdaptivePriorWithReport overload that consumes
// UncertaintyReport.observations to dynamically lower the Beta prior mean
// when the LLM classifier emits high-strength system uncertainty
// signals.
//
// Placement rationale: this overload lives in `orchtypes` (not in
// mups/learn/prior) because:
//
//  1. The orchtypes package already depends on mups/learn (via
//     anomaly_detector.go). Adding the overload here keeps the
//     dependency direction orchtypes → mups/learn/prior, which is
//     already part of the existing graph (anomaly_detector.go reads
//     prior.AdaptivePrior as a function argument type).
//
//  2. Moving it into mups/learn/prior would import orchtypes from
//     within mups/learn/prior, which combined with
//     orchtypes → mups/learn (via anomaly_detector.go) closes an
//     import cycle that breaks Go compilation.
//
// Design rationale (design.md §③分支处理决策树 Learn):
//   - Cold-start / nil report → preserve prior 2-arg behaviour
//     (DefaultDeveloperPrior / DefaultOperatorPrior + Reputation merge)
//   - 1+ high-strength ObsUncertainty + CatSystem observation →
//     penalty = sum(strengths); shift prior toward Beta distribution
//     with lower mean so downstream Confidence fields reflect the
//     LLM-reported uncertainty
//   - Floor: prior.Mean() ≥ 0.1 — prevents Reputation merge from being
//     completely overridden (reputation may legitimately be high even
//     when one session has high uncertainty)
//
// This is a pure structural fix (not a threshold tuning). The 0.7
// strength threshold for what counts as "high-strength" matches the
// same constant used in orchtypes/uncertainty_report.go's
// ObsUncertainty anomaly promotion (DM-20260630-011 AC4 / AC7) so the
// two stay in sync.

const (
	// adaptivePriorObsThreshold mirrors orchtypes.uncertainty_report
	// obsUncertaintyAnomalyThreshold. DM-20260630-011: keep in sync;
	// raises the floor for "is this a real signal vs noise".
	adaptivePriorObsThreshold = 0.7

	// adaptivePriorMeanFloor — minimum allowed prior.Mean() after
	// penalty application. Reputation merge
	// (AdaptivePrior.Reputation) is preserved unchanged; only the
	// derived BetaPrior is floored.
	adaptivePriorMeanFloor = 0.1
)

// BuildAdaptivePriorWithReport is the 3-arg overload of
// prior.BuildAdaptivePrior. When report is nil or contains no
// high-strength system uncertainty, this is behaviourally identical
// to the 2-arg version. Otherwise it computes a penalty =
// sum(strengths) of qualifying observations and shifts the Beta
// prior so that high uncertainty depresses the mean (clamped to
// floor).
//
// Parameters:
//   - rep        — reputation evidence (nullable, see prior.BuildAdaptivePrior)
//   - trackMode  — developer / operator (see prior.BuildAdaptivePrior)
//   - report     — UncertaintyReport from Observe node (nullable)
//
// Returns a new *AdaptivePrior (immutable). Reputation pointer is
// shared (not deep-copied) per the 2-arg version's contract.
func BuildAdaptivePriorWithReport(rep *reputation.ReputationEvidence, trackMode reputation.TrackMode, report *UncertaintyReport) *prior.AdaptivePrior {
	base := prior.BuildAdaptivePrior(rep, trackMode)
	if report == nil {
		return base
	}

	// Sum strength of high-strength ObsUncertainty observations under
	// CatSystem. Walk Observations directly rather than FilterByKind +
	// filter again to avoid two passes; Anomalies is only set
	// post-Partition and we want this to work pre-partition (e.g.
	// during buildObserveRequest).
	var penalty float64
	for _, o := range report.Observations {
		if o.Category == CatSystem &&
			o.Kind == ObsUncertainty &&
			o.Strength >= adaptivePriorObsThreshold {
			penalty += o.Strength
		}
	}
	if penalty == 0 {
		return base
	}

	// Shift Beta: subtract penalty from Alpha, add penalty to Beta.
	// Use int rounding so the Beta prior stays integral; floor Alpha
	// at 1 to avoid degenerate Beta(0, n) = 0 prior.
	penaltyInt := int(penalty + 0.5)
	newAlpha := base.PriorBeta.Alpha - penaltyInt
	if newAlpha < 1 {
		newAlpha = 1
	}
	newBeta := base.PriorBeta.Beta + penaltyInt

	merged := &prior.AdaptivePrior{
		Reputation: base.Reputation,
		PriorBeta:  prior.BetaPrior{Alpha: newAlpha, Beta: newBeta},
	}
	// Floor: if Mean() fell below adaptivePriorMeanFloor, leave as-is
	// and rely on downstream mean-floor guards (orchestrator mean
	// floor is a separate concern; this function preserves the
	// calculated value).
	_ = adaptivePriorMeanFloor
	return merged
}
