package workmodel

import (
	"errors"
	"fmt"

	"github.com/devrix/devrix/internal/shared/types"
)

var (
	errSpawnRoundRequired      = errors.New("pipeline round required for spawn")
	errSpawnPolicyNotDecompose = errors.New("spawn policy is not decompose")
	errSpawnRoundIncomplete    = errors.New("pipeline round missing required LP-5 fields")
)

// SpawnPolicyEvaluator applies the 4-sub-decision algebra
// (checkBudget → checkResolutionReport → checkRollupGuard →
// checkVerdictDirection). LLM MUST NOT set SpawnPolicy directly; only
// this function may assign it (goal G3).
//
// DM-20260704-006 inserts checkResolutionReport after checkBudget:
// checkBudget hard caps win (depth/children/daily limits), then the
// ResolutionReport's AnySubWorktreePending / MaxUnresolvedStrength gates
// override the verdict direction (RC-4a/b). See spawn_decide_resolution.go
// for the override semantics.
func SpawnPolicyEvaluator(round *WorkItemPipelineRound, ctx TreeEvalContext) SpawnPolicy {
	if round == nil {
		return SpawnNone
	}
	ctx = normalizeCtx(ctx)
	if policy, fired := checkBudget(round, ctx); fired {
		return policy
	}
	if policy, fired := checkResolutionReport(round, ctx); fired {
		return policy
	}
	if policy, fired := checkRollupGuard(round, ctx); fired {
		return policy
	}
	return checkVerdictDirection(round, ctx)
}
// EvaluateSpawnPolicy fills round.SpawnPolicy and SpawnRationale in place.
func EvaluateSpawnPolicy(round *WorkItemPipelineRound, ctx TreeEvalContext) {
	if round == nil {
		return
	}
	policy := SpawnPolicyEvaluator(round, ctx)
	round.SpawnPolicy = policy
	round.RollupSynthRequested = policy == SpawnInline && RollupSynthEligible(round, ctx)
	round.SpawnRationale = spawnRationale(policy, round, ctx)
}

func spawnRationale(policy SpawnPolicy, round *WorkItemPipelineRound, ctx TreeEvalContext) string {
	switch policy {
	case SpawnAwait:
		return fmt.Sprintf("R0: %d running children", ctx.RunningChildren)
	case SpawnInline:
		if round.RollupSynthRequested {
			return "CC-U3: rollup synth (evidence sufficient, format deliverable incomplete)"
		}
		if deliverableContinuationRequired(round) {
			if ctx.Depth >= ctx.MaxDepth {
				return fmt.Sprintf("R1: max depth inline retry %d/%d (schema=%s status=%s)",
					ctx.InlineRetriesAtMaxDepth+1, ctx.MaxInlineRetriesAtMaxDepth,
					round.DeliverableSchema, round.DeliverableStatus)
			}
			return fmt.Sprintf("deliverable incomplete (schema=%s status=%s): inline retry",
				round.DeliverableSchema, round.DeliverableStatus)
		}
		if ctx.Depth >= ctx.MaxDepth {
			return fmt.Sprintf("R1: depth %d >= max %d", ctx.Depth, ctx.MaxDepth)
		}
		if ctx.RollupRound {
			return fmt.Sprintf("R5/R6/R7-rollup: retry %d/%d", ctx.RollupRetries, ctx.MaxRollupRetries)
		}
		return fmt.Sprintf("R7: indeterminate retry %d/%d", ctx.IndeterminateRetries, ctx.MaxIndeterminateRetries)
	case SpawnEscalateHuman:
		if ctx.DailyLimitExceeded {
			return "R2: daily decompose limit exceeded"
		}
		if deliverableContinuationRequired(round) &&
			ctx.InlineRetriesAtMaxDepth >= ctx.MaxInlineRetriesAtMaxDepth &&
			!RollupSynthEligible(round, ctx) {
			return fmt.Sprintf("CC-1.2: deliverable inline retries exhausted (%d/%d)",
				ctx.InlineRetriesAtMaxDepth, ctx.MaxInlineRetriesAtMaxDepth)
		}
		if ctx.Depth >= ctx.MaxDepth && deliverableContinuationRequired(round) &&
			ctx.InlineRetriesAtMaxDepth >= ctx.MaxInlineRetriesAtMaxDepth {
			return fmt.Sprintf("R1: inline retries exhausted at max depth (%d/%d)",
				ctx.InlineRetriesAtMaxDepth, ctx.MaxInlineRetriesAtMaxDepth)
		}
		if ctx.RollupRound {
			return fmt.Sprintf("rollup retries exhausted (%d/%d)", ctx.RollupRetries, ctx.MaxRollupRetries)
		}
		if round.VerdictKind == types.VerdictIndeterminate {
			return fmt.Sprintf("R7: indeterminate retries exhausted (%d)", ctx.MaxIndeterminateRetries)
		}
		return fmt.Sprintf("CC-1.2: deliverable inline retries exhausted (%d/%d)",
			ctx.InlineRetriesAtMaxDepth, ctx.MaxInlineRetriesAtMaxDepth)
	case SpawnDecompose:
		if round.VerdictKind == types.VerdictIndeterminate {
			return fmt.Sprintf("R7: indeterminate retries exhausted → decompose (plan=%d uncertainty=%.2f)",
				round.PlanKind, round.UncertaintyMean)
		}
		// DM-20260704-006 RC-4a: distinguish resolution-report driven
		// decompose from R5/R6 (uncertainty-driven) so dashboards can
		// tell "plan emitted sub_worktree" apart from "plan crossed
		// uncertainty threshold". The RC-4a case wins precedence in the
		// sub-decision chain even when VerdictKind=Pass.
		if round.ResolutionReport != nil && round.ResolutionReport.AnySubWorktreePending() {
			return fmt.Sprintf("RC-4a: ResolutionReport.AnySubWorktreePending → decompose (n=%d)",
				len(round.ResolutionReport.UnresolvedObs))
		}
		return fmt.Sprintf("R5/R6: verdict=%s uncertainty=%.2f threshold=%.2f plan=%d",
			round.VerdictKind, round.UncertaintyMean, ctx.Threshold, round.PlanKind)
	case SpawnParallelExplore:
		return fmt.Sprintf("R6: scenario fail plan=%d → parallel probe", round.PlanKind)
	case SpawnUserGate:
		// DM-20260704-006 RC-4b: surface the unresolved ObsIDs by
		// strength so operators can see why a user gate fired.
		if round.ResolutionReport == nil {
			return "RC-4b: user gate (no resolution report)"
		}
		report := *round.ResolutionReport
		return fmt.Sprintf("RC-4b: user gate (max_unresolved_strength=%.3f >= %.3f, n=%d)",
			report.MaxUnresolvedStrength(), DefaultUnresolvedStrengthThreshold,
			len(report.UnresolvedObs))
	case SpawnNone:
		if applicableDeliverableSchema(round) && !deliverableContinuationRequired(round) {
			return fmt.Sprintf("R0.5: deliverable complete (schema=%s) → terminal",
				round.DeliverableSchema)
		}
		return fmt.Sprintf("R3/R4/R8: verdict=%s plan=%d converged or terminal fail", round.VerdictKind, round.PlanKind)
	default:
		return ""
	}
}

// DeliverableContinuationRequired reports whether a round still owes a
// registered deliverable schema (RH-MUPS-12). Exported for session loop
// stagnation checks in sessionorchestrator.
func DeliverableContinuationRequired(round *WorkItemPipelineRound) bool {
	return deliverableContinuationRequired(round)
}

// deliverableContinuationRequired reports whether SpawnNone would leave an
// applicable deliverable schema unsatisfied — the WorkItem must inline-retry
// (or decompose) instead of stopping. RH-MUPS-12 (2026-07-03).
func deliverableContinuationRequired(round *WorkItemPipelineRound) bool {
	if round == nil {
		return false
	}
	if !round.DeliverableContract.ContractApplicable() && !IsRegisteredDeliverableSchema(round.DeliverableSchema) {
		return false
	}
	status := round.DeliverableStatus
	if status == DeliverableStatusNotApplicable {
		return false
	}
	return status != DeliverableStatusComplete
}

func applicableDeliverableSchema(round *WorkItemPipelineRound) bool {
	if round == nil {
		return false
	}
	if round.DeliverableContract.ContractApplicable() {
		return true
	}
	return IsRegisteredDeliverableSchema(round.DeliverableSchema)
}

func IsDeliverableInlineBudgetExhaustedFromCtx(ctx TreeEvalContext) bool {
	return ctx.InlineRetriesAtMaxDepth >= ctx.MaxInlineRetriesAtMaxDepth
}

// deliverableInlineWouldExhaust reports whether the next inline retry would
// hit the CC-1.2 budget (TouchInlineRetry runs after EvaluateSpawnPolicy).
func deliverableInlineWouldExhaust(ctx TreeEvalContext) bool {
	max := ctx.MaxInlineRetriesAtMaxDepth
	if max <= 0 {
		max = DefaultMaxInlineRetriesAtMaxDepth
	}
	return ctx.InlineRetriesAtMaxDepth+1 >= max
}
