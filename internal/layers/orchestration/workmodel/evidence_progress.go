package workmodel

import "strings"

// CC-U1 / DM-20260704-001: evidence progress drives rollup synth vs deliverable inline.

const (
	EvidenceSufficientThreshold = 0.5
	RollupSynthThreshold          = 0.50
	// SingleModeUncertaintyThreshold caps U for strategic execution_mode=single (CC-U4).
	SingleModeUncertaintyThreshold = 0.45
)

// EvidenceInput carries deterministic execute signals for spawn continuation.
type EvidenceInput struct {
	ToolCalls  int
	HasScopeIn bool
	ChildStats ChildOutcomeStats
}

// EvidenceProgress scores how much exploration evidence exists (0..1).
func EvidenceProgress(in EvidenceInput) float64 {
	var score float64
	switch {
	case in.ToolCalls >= 2:
		score += 0.4
	case in.ToolCalls >= 1:
		score += 0.2
	}
	if in.HasScopeIn {
		score += 0.2
	}
	if in.ChildStats.Total > 0 && in.ChildStats.Running == 0 {
		score += 0.3
	}
	if score > 1 {
		return 1
	}
	if score < 0 {
		return 0
	}
	return score
}

// EvidenceProgressFromRound derives progress from a pipeline round snapshot.
func EvidenceProgressFromRound(round *WorkItemPipelineRound, childStats ChildOutcomeStats) float64 {
	if round == nil {
		return 0
	}
	return EvidenceProgress(EvidenceInput{
		ToolCalls:  round.ExecuteToolCalls,
		HasScopeIn: round.ScopeInPresent,
		ChildStats: childStats,
	})
}

// RollupSynthEligible reports CC-U1: sufficient evidence and low U → convergence phase.
func RollupSynthEligible(round *WorkItemPipelineRound, ctx TreeEvalContext) bool {
	if round == nil || !deliverableContinuationRequired(round) {
		return false
	}
	if round.UncertaintyMean >= RollupSynthThreshold {
		return false
	}
	if ctx.RunningChildren > 0 {
		return false
	}
	if ctx.RollupRound && ctx.RollupRetries >= ctx.MaxRollupRetries {
		return false
	}
	stats := ChildOutcomeStats{Total: ctx.ChildTotal, Running: ctx.RunningChildren}
	return EvidenceProgressFromRound(round, stats) >= EvidenceSufficientThreshold
}

// ConvergenceFormatFailureConfFactor scales the confidence penalty when deliverable
// format failed but exploration evidence is already sufficient (CC-U5 / T-P1-3).
const ConvergenceFormatFailureConfFactor = 0.5

// DeliverableIncompleteObsStrength returns ObsSignal strength for CC-U5 observe wiring.
// When EvidenceProgress is sufficient, damp the signal so format failure is not
// treated as high exploration need (structural — uses EvidenceProgress, not reason strings).
func DeliverableIncompleteObsStrength(round *WorkItemPipelineRound) float64 {
	const defaultStrength = 0.7
	const convergenceStrength = 0.35
	if round == nil {
		return defaultStrength
	}
	if EvidenceProgressFromRound(round, ChildOutcomeStats{}) >= EvidenceSufficientThreshold {
		return convergenceStrength
	}
	return defaultStrength
}
func spawnForDeliverableContinuation(round *WorkItemPipelineRound, ctx TreeEvalContext) SpawnPolicy {
	if RollupSynthEligible(round, ctx) {
		if ctx.RollupRound && ctx.RollupRetries >= ctx.MaxRollupRetries {
			return SpawnEscalateHuman
		}
		return SpawnInline
	}
	if deliverableInlineWouldExhaust(ctx) {
		return SpawnEscalateHuman
	}
	return SpawnInline
}

// DeliverableReasonFromRound returns a machine reason when deliverable verify failed.
func DeliverableReasonFromRound(round *WorkItemPipelineRound) string {
	if round == nil {
		return ""
	}
	if s := strings.TrimSpace(round.DeliverableReason); s != "" {
		return s
	}
	return ""
}
