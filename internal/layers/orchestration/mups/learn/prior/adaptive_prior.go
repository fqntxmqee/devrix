package prior

import (
	"errors"
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/reputation"
)

// ErrAdaptivePriorNotReady — AdaptivePrior not ready (cold start).
// Moved from mups/learn/asset/learning_asset.go in v6.0.0 subpackage split
// (logically belongs to AdaptivePrior, not Asset).
var ErrAdaptivePriorNotReady = errors.New("learn: adaptive prior not ready (cold start)")

// BetaPrior is a Beta distribution prior (α, β). Used as the default prior
// for new sessions (doc 25 §四: Developer Beta(5,3) / Operator Beta(8,1)).
type BetaPrior struct {
	Alpha int
	Beta  int
}

// String returns the wire format "Beta(α,β)".
func (p BetaPrior) String() string {
	return fmt.Sprintf("Beta(%d,%d)", p.Alpha, p.Beta)
}

// Mean returns the Beta distribution's expected value α/(α+β). When
// α+β == 0 (cold start), returns 0 (fail-safe: cold-start prior carries
// no information, so the multiplier is a no-op).
//
// This is the canonical Phase 6 PR-F1 hook for downstream Observer
// submodules (IntentQuantizer / AnomalyDetector / RuleClassifier) to
// read the prior as a confidence / threshold multiplier.
func (p BetaPrior) Mean() float64 {
	total := p.Alpha + p.Beta
	if total == 0 {
		return 0
	}
	return float64(p.Alpha) / float64(total)
}

// DefaultPriors — from doc 25 §四.
var (
	// DefaultDeveloperPrior — slightly positive prior for developers.
	// Used when reputation.TrackMode == "developer" (or empty / unknown — fail-safe).
	DefaultDeveloperPrior = BetaPrior{Alpha: 5, Beta: 3}

	// DefaultOperatorPrior — strongly positive prior for operators.
	// Used when reputation.TrackMode == "operator".
	DefaultOperatorPrior = BetaPrior{Alpha: 8, Beta: 1}
)

// AdaptivePrior is the immutable output of BuildAdaptivePrior. Carries the
// merged Beta prior + Reputation (LP-1 衍生).
type AdaptivePrior struct {
	// Reputation — current reputation.ReputationEvidence (nullable for cold start).
	Reputation *reputation.ReputationEvidence

	// PriorBeta — Bayesian-merged Beta prior (DefaultPrior + Reputation).
	PriorBeta BetaPrior
}

// BuildAdaptivePrior constructs an AdaptivePrior from a reputation.ReputationEvidence
// and track mode.
//
// Behavior:
//
//   - rep == nil → uses DefaultPrior only; Reputation field is nil.
//   - trackMode == reputation.TrackModeDeveloper (or "") → DefaultDeveloperPrior (fail-safe).
//   - trackMode == reputation.TrackModeOperator → DefaultOperatorPrior.
//   - other trackMode → DefaultDeveloperPrior (fail-safe, matches phase 5
//     PR-E3 design).
//
// The merged prior is computed via Bayesian combination:
//
//	mergedAlpha = prior.Alpha + rep.Alpha
//	mergedBeta  = prior.Beta + rep.Beta
//
// When rep == nil, the merged prior is just DefaultPrior (no combination).
func BuildAdaptivePrior(rep *reputation.ReputationEvidence, trackMode reputation.TrackMode) *AdaptivePrior {
	prior := defaultPriorForTrackMode(trackMode)

	if rep != nil {
		prior.Alpha += rep.Alpha
		prior.Beta += rep.Beta
	}

	return &AdaptivePrior{
		Reputation: rep,
		PriorBeta:  prior,
	}
}

func defaultPriorForTrackMode(trackMode reputation.TrackMode) BetaPrior {
	switch trackMode {
	case reputation.TrackModeOperator:
		return DefaultOperatorPrior
	case reputation.TrackModeDeveloper, "":
		return DefaultDeveloperPrior
	default:
		// Unknown / non-empty → fail-safe to Developer prior (doc 25 §四
		// classifies Developer as the broader default).
		return DefaultDeveloperPrior
	}
}