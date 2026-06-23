package learn

import "fmt"

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

// DefaultPriors — from doc 25 §四.
var (
	// DefaultDeveloperPrior — slightly positive prior for developers.
	// Used when TrackMode == "developer" (or empty / unknown — fail-safe).
	DefaultDeveloperPrior = BetaPrior{Alpha: 5, Beta: 3}

	// DefaultOperatorPrior — strongly positive prior for operators.
	// Used when TrackMode == "operator".
	DefaultOperatorPrior = BetaPrior{Alpha: 8, Beta: 1}
)

// InjectTarget identifies where to inject the AdaptivePrior in the next
// Observe.All() call. The 3 standard targets are LP-1 衍生 (Phase 5 PR-E5).
type InjectTarget int

const (
	// InjectIntentQuantizer — inject into IntentQuantizer.Quantize.
	InjectIntentQuantizer InjectTarget = iota

	// InjectHistoricalDetector — inject into AnomalyDetector.HistoricalDetector.
	InjectHistoricalDetector

	// InjectRuleClassifier — inject into RuleClassifier.Classify.
	InjectRuleClassifier
)

// String returns the wire format name.
func (t InjectTarget) String() string {
	switch t {
	case InjectIntentQuantizer:
		return "intent_quantizer"
	case InjectHistoricalDetector:
		return "historical_detector"
	case InjectRuleClassifier:
		return "rule_classifier"
	default:
		return fmt.Sprintf("InjectTarget(%d)", int(t))
	}
}

// DefaultInjectTargets — the 3 standard injection points.
var DefaultInjectTargets = []InjectTarget{
	InjectIntentQuantizer,
	InjectHistoricalDetector,
	InjectRuleClassifier,
}

// AdaptivePrior is the immutable output of BuildAdaptivePrior. Carries the
// merged Beta prior + Reputation + InjectTargets list (LP-1 衍生).
type AdaptivePrior struct {
	// Reputation — current ReputationEvidence (nullable for cold start).
	Reputation *ReputationEvidence

	// PriorBeta — Bayesian-merged Beta prior (DefaultPrior + Reputation).
	PriorBeta BetaPrior

	// InjectTargets — the list of injection targets.
	InjectTargets []InjectTarget
}

// BuildAdaptivePrior constructs an AdaptivePrior from a ReputationEvidence
// and track mode.
//
// Behavior:
//
//   - rep == nil → uses DefaultPrior only; Reputation field is nil.
//   - trackMode == TrackModeDeveloper (or "") → DefaultDeveloperPrior (fail-safe).
//   - trackMode == TrackModeOperator → DefaultOperatorPrior.
//   - other trackMode → DefaultDeveloperPrior (fail-safe, matches phase 5
//     PR-E3 design).
//
// The merged prior is computed via Bayesian combination:
//
//	mergedAlpha = prior.Alpha + rep.Alpha
//	mergedBeta  = prior.Beta + rep.Beta
//
// When rep == nil, the merged prior is just DefaultPrior (no combination).
func BuildAdaptivePrior(rep *ReputationEvidence, trackMode TrackMode) *AdaptivePrior {
	prior := defaultPriorForTrackMode(trackMode)

	if rep != nil {
		prior.Alpha += rep.Alpha
		prior.Beta += rep.Beta
	}

	return &AdaptivePrior{
		Reputation:    rep,
		PriorBeta:     prior,
		InjectTargets: DefaultInjectTargets,
	}
}

func defaultPriorForTrackMode(trackMode TrackMode) BetaPrior {
	switch trackMode {
	case TrackModeOperator:
		return DefaultOperatorPrior
	case TrackModeDeveloper, "":
		return DefaultDeveloperPrior
	default:
		// Unknown / non-empty → fail-safe to Developer prior (doc 25 §四
		// classifies Developer as the broader default).
		return DefaultDeveloperPrior
	}
}