package learn

import (
	"fmt"
	"math"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// TrackMode is the developer / operator split (doc 25 §四 + doc 46 §4.3).
type TrackMode string

const (
	// TrackModeDeveloper — slightly positive prior (Beta(5,3)).
	TrackModeDeveloper TrackMode = "developer"

	// TrackModeOperator — strongly positive prior (Beta(8,1)).
	TrackModeOperator TrackMode = "operator"
)

// ParseTrackMode parses a wire-format track mode string.
func ParseTrackMode(s string) (TrackMode, error) {
	switch s {
	case string(TrackModeDeveloper):
		return TrackModeDeveloper, nil
	case string(TrackModeOperator):
		return TrackModeOperator, nil
	default:
		return "", fmt.Errorf("learn: unknown track mode %q", s)
	}
}

// ReputationEvidence captures cross-session reputation (Bayesian Beta).
// Immutable: BayesianUpdate returns a NEW ReputationEvidence, leaving the
// prior unchanged (LP-3 衍生).
type ReputationEvidence struct {
	// SessionID — owner session (必填).
	SessionID string

	// TrackMode — developer / operator (必填).
	TrackMode TrackMode

	// Beta distribution parameters (LP-3).
	Alpha int
	Beta  int

	// Derived metrics.
	Mean           float64
	Variance       float64
	ConfidenceLow  float64
	ConfidenceHigh float64

	// Metadata.
	LastUpdated      time.Time
	UpdateCount      int
	SourceVerdictIDs []string

	// VerifierFailureCount — ⭐G8-1 fix: Verifier自身失败的次数（不污染 α/β）.
	// Incremented only when IndeterminateReason == "verifier_parse_failure".
	VerifierFailureCount int

	// IndeterminateCount — env-limited INDETERMINATE 计数（含 verifier failure,
	// 区别于 VerifierFailureCount 单独追踪）. Used for G5-3 INDETERMINATE ratio.
	IndeterminateCount int
}

// NewReputationEvidence creates a fresh ReputationEvidence with cold-start
// defaults (Alpha=Beta=0, Mean=0, Variance=0, ConfidenceLow=0,
// ConfidenceHigh=1).
func NewReputationEvidence(sessionID string, trackMode TrackMode) (*ReputationEvidence, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("%w: sessionID is empty", ErrReputationStoreUnavailable)
	}
	if _, err := ParseTrackMode(string(trackMode)); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReputationStoreUnavailable, err)
	}
	now := time.Now()
	return &ReputationEvidence{
		SessionID:        sessionID,
		TrackMode:        trackMode,
		Alpha:            0,
		Beta:             0,
		Mean:             0,
		Variance:         0,
		ConfidenceLow:    0,
		ConfidenceHigh:   1,
		LastUpdated:      now,
		UpdateCount:      0,
		SourceVerdictIDs: []string{},
	}, nil
}

// BayesianUpdate applies one Verdict to a prior ReputationEvidence and
// returns a NEW ReputationEvidence (prior unchanged).
//
// ⭐G8-1 fix (Phase 5 PR-E2): when Kind == VerdictIndeterminate and
// IndeterminateReason == "verifier_parse_failure", only
// VerifierFailureCount is incremented and α/β are NOT touched. This prevents
// verifier LLM output format issues from polluting user reputation.
//
// ⭐G8-1 defensive: when Alpha+Beta == 0 (cold start with no updates yet),
// Mean is preserved from the prior (cold-start default = 0 from a fresh
// NewReputationEvidence, or Developer Beta(5,3) → 0.625 after merging in
// BuildAdaptivePrior).
func BayesianUpdate(prior *ReputationEvidence, verdict workmodel.Verdict) *ReputationEvidence {
	next := *prior // immutable copy
	next.UpdateCount++
	next.LastUpdated = time.Now()
	next.SourceVerdictIDs = append(next.SourceVerdictIDs, verdict.SourceID)

	switch verdict.Kind {
	case types.VerdictPass, types.VerdictPartial:
		next.Alpha++
	case types.VerdictFail:
		next.Beta++
	case types.VerdictIndeterminate:
		// ⭐G8-1 fix: distinguish verifier_parse_failure from env-limited
		if verdict.IndeterminateReason == "verifier_parse_failure" {
			next.VerifierFailureCount++
			// DO NOT update α/β — verifier output issue is not user's fault
		} else {
			next.IndeterminateCount++
			// Other INDETERMINATE: keep prior behavior (no α/β update)
		}
	}

	// Derived metrics.
	total := next.Alpha + next.Beta
	if total == 0 {
		// ⭐G8-1 defensive: cold start α=β=0 → keep prior Mean
		next.Mean = prior.Mean
		next.Variance = 0
		next.ConfidenceLow = 0
		next.ConfidenceHigh = 1
	} else {
		totalF := float64(total)
		next.Mean = float64(next.Alpha) / totalF
		next.Variance = float64(next.Alpha*next.Beta) / (totalF * totalF * (totalF + 1))
		next.ConfidenceLow, next.ConfidenceHigh = wilsonScoreInterval(next.Alpha, next.Beta, 0.95)
	}

	return &next
}

// wilsonScoreInterval computes the Wilson Score confidence interval.
// confidence=0.95 → z=1.96.
//
// Formula:
//
//	p̂ = α/(α+β)
//	z² = z*z
//	center = p̂ + z²/(2n)
//	margin = z * sqrt(p̂(1-p̂)/n + z²/(4n²))
//	denominator = 1 + z²/n
//	return (center - margin) / denominator, (center + margin) / denominator
func wilsonScoreInterval(alpha, beta int, confidence float64) (float64, float64) {
	n := float64(alpha + beta)
	if n == 0 {
		return 0, 1
	}
	var z float64
	switch confidence {
	case 0.95:
		z = 1.96
	case 0.99:
		z = 2.576
	case 0.90:
		z = 1.645
	default:
		z = 1.96
	}
	z2 := z * z
	pHat := float64(alpha) / n
	center := pHat + z2/(2*n)
	margin := z * math.Sqrt((pHat*(1-pHat))/n+z2/(4*n*n))
	denom := 1 + z2/n
	return (center - margin) / denom, (center + margin) / denom
}