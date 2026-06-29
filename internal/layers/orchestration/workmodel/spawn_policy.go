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

	// R0
	if ctx.RunningChildren > 0 {
		return SpawnAwait
	}
	// R1
	if ctx.Depth >= ctx.MaxDepth {
		return SpawnInline
	}
	// R2
	if ctx.DailyLimitExceeded {
		return SpawnEscalateHuman
	}

	switch round.VerdictKind {
	case types.VerdictPass:
		// R3 / R4 — converged success for commitment and exploratory plans.
		return SpawnNone

	case types.VerdictPartial:
		// R5 — rollup synthesis retries inline until verifyRollup passes.
		if ctx.RollupRound {
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
		return SpawnNone

	case types.VerdictFail:
		if ctx.RollupRound {
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
		if ctx.RollupRound {
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
		if ctx.Depth >= ctx.MaxDepth {
			return fmt.Sprintf("R1: depth %d >= max %d", ctx.Depth, ctx.MaxDepth)
		}
		return fmt.Sprintf("R7: indeterminate retry %d/%d", ctx.IndeterminateRetries, ctx.MaxIndeterminateRetries)
	case SpawnEscalateHuman:
		if ctx.DailyLimitExceeded {
			return "R2: daily decompose limit exceeded"
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
		return fmt.Sprintf("R3/R4/R8: verdict=%s plan=%d converged or terminal fail", round.VerdictKind, round.PlanKind)
	default:
		return ""
	}
}
