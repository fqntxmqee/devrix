package workmodel

import (
	"errors"
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

var (
	errSpawnRoundRequired      = errors.New("pipeline round required for spawn")
	errSpawnPolicyNotDecompose = errors.New("spawn policy is not decompose")
	errSpawnRoundIncomplete    = errors.New("pipeline round missing required LP-5 fields")
)

// SpawnPolicyEvaluator applies rules R0–R8 (design §4). LLM MUST NOT set
// SpawnPolicy directly; only this function may assign it (goal G3).
func SpawnPolicyEvaluator(round *WorkItemPipelineRound, ctx TreeEvalContext) SpawnPolicy {
	if round == nil {
		return SpawnNone
	}
	if ctx.MaxDepth <= 0 {
		ctx.MaxDepth = DefaultMaxDecomposeDepth
	}
	if ctx.Threshold <= 0 {
		ctx.Threshold = DefaultUncertaintyDecomposeThreshold
	}
	if ctx.MaxIndeterminateRetries <= 0 {
		ctx.MaxIndeterminateRetries = DefaultMaxIndeterminateRetries
	}
	if ctx.MaxRollupRetries <= 0 {
		ctx.MaxRollupRetries = DefaultMaxRollupRetries
	}
	if ctx.MaxInlineRetriesAtMaxDepth <= 0 {
		ctx.MaxInlineRetriesAtMaxDepth = DefaultMaxInlineRetriesAtMaxDepth
	}

	// R0
	if ctx.RunningChildren > 0 {
		return SpawnAwait
	}
	// R0.5 — CC-1.1: applicable deliverable satisfied → terminal before R1.
	if applicableDeliverableSchema(round) && !deliverableContinuationRequired(round) {
		return SpawnNone
	}
	// R1 — max depth with continuation: bounded inline, then escalate.
	if ctx.Depth >= ctx.MaxDepth {
		if ctx.InlineRetriesAtMaxDepth >= ctx.MaxInlineRetriesAtMaxDepth {
			return SpawnEscalateHuman
		}
		return SpawnInline
	}
	// R2
	if ctx.DailyLimitExceeded {
		return SpawnEscalateHuman
	}

	switch round.VerdictKind {
	case types.VerdictPass:
		// CC-1 / §8.1: Pass with applicable schema MUST NOT SpawnNone while
		// deliverable is still owed — inline (or R1 budget) instead.
		if deliverableContinuationRequired(round) {
			if IsDeliverableInlineBudgetExhaustedFromCtx(ctx) {
				return SpawnEscalateHuman
			}
			return SpawnInline
		}
		return SpawnNone

	case types.VerdictPartial:
		// R5 — rollup synthesis retries inline until verifyRollup passes.
		// RH-MUPS-03 (DM-20260701-001): after MaxRollupRetries consecutive
		// non-Pass rollup rounds escalate to human review rather than
		// silently looping until the session loop max=16 backstops us.
		if ctx.RollupRound {
			if ctx.RollupRetries >= ctx.MaxRollupRetries {
				return SpawnEscalateHuman
			}
			return SpawnInline
		}
		// Exploratory partial on decomposable parents (Goal/Plan/Implement)
		// triggers the first split; leaf explore items retry inline.
		if IsExploratoryPlanKind(round.PlanKind) {
			if ctx.CanDecompose && ctx.ChildTotal == 0 {
				return SpawnDecompose
			}
			return SpawnInline
		}
		if round.UncertaintyMean >= ctx.Threshold {
			return SpawnDecompose
		}
		if deliverableContinuationRequired(round) {
			if IsDeliverableInlineBudgetExhaustedFromCtx(ctx) {
				return SpawnEscalateHuman
			}
			return SpawnInline
		}
		return SpawnNone

	case types.VerdictFail:
		// RH-MUPS-03 (DM-20260701-001): same termination guard as R5.
		if ctx.RollupRound {
			if ctx.RollupRetries >= ctx.MaxRollupRetries {
				return SpawnEscalateHuman
			}
			return SpawnInline
		}
		// R6
		if round.PlanKind == plan.ScenarioPlan {
			return SpawnParallelExplore
		}
		// Leaf explore items cannot decompose; retry inline instead.
		if round.PlanKind == plan.ExplorationPlan {
			if ctx.CanDecompose && ctx.ChildTotal == 0 {
				return SpawnDecompose
			}
			return SpawnInline
		}
		return SpawnNone

	case types.VerdictIndeterminate:
		// RH-MUPS-03 (DM-20260701-001): same termination guard as R5/R6.
		if ctx.RollupRound {
			if ctx.RollupRetries >= ctx.MaxRollupRetries {
				return SpawnEscalateHuman
			}
			return SpawnInline
		}
		// R7 — exploratory plans decompose when verifier abstains (uncertainty path),
		// instead of blocking on human gate.
		if ctx.IndeterminateRetries >= ctx.MaxIndeterminateRetries {
			if (IsExploratoryPlanKind(round.PlanKind) || round.UncertaintyMean >= ctx.Threshold) &&
				ctx.CanDecompose && ctx.ChildTotal == 0 {
				return SpawnDecompose
			}
			return SpawnEscalateHuman
		}
		return SpawnInline

	default:
		// R8
		return SpawnNone
	}
}

// EvaluateSpawnPolicy fills round.SpawnPolicy and SpawnRationale in place.
func EvaluateSpawnPolicy(round *WorkItemPipelineRound, ctx TreeEvalContext) {
	if round == nil {
		return
	}
	policy := SpawnPolicyEvaluator(round, ctx)
	round.SpawnPolicy = policy
	round.SpawnRationale = spawnRationale(policy, round, ctx)
}

func spawnRationale(policy SpawnPolicy, round *WorkItemPipelineRound, ctx TreeEvalContext) string {
	switch policy {
	case SpawnAwait:
		return fmt.Sprintf("R0: %d running children", ctx.RunningChildren)
	case SpawnInline:
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
		if ctx.Depth >= ctx.MaxDepth && deliverableContinuationRequired(round) &&
			ctx.InlineRetriesAtMaxDepth >= ctx.MaxInlineRetriesAtMaxDepth {
			return fmt.Sprintf("R1: inline retries exhausted at max depth (%d/%d)",
				ctx.InlineRetriesAtMaxDepth, ctx.MaxInlineRetriesAtMaxDepth)
		}
		if ctx.RollupRound {
			return fmt.Sprintf("rollup retries exhausted (%d/%d)", ctx.RollupRetries, ctx.MaxRollupRetries)
		}
		return fmt.Sprintf("R7: indeterminate retries exhausted (%d)", ctx.MaxIndeterminateRetries)
	case SpawnDecompose:
		if round.VerdictKind == types.VerdictIndeterminate {
			return fmt.Sprintf("R7: indeterminate retries exhausted → decompose (plan=%d uncertainty=%.2f)",
				round.PlanKind, round.UncertaintyMean)
		}
		return fmt.Sprintf("R5/R6: verdict=%s uncertainty=%.2f threshold=%.2f plan=%d",
			round.VerdictKind, round.UncertaintyMean, ctx.Threshold, round.PlanKind)
	case SpawnParallelExplore:
		return fmt.Sprintf("R6: scenario fail plan=%d → parallel probe", round.PlanKind)
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
