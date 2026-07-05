// Package workmodel — spawn_decision_algebra.go
//
// 3-sub-decision algebra for SpawnPolicyEvaluator (MUPS Decide node, M5).
// Decomposes the original 50+ line if/switch chain into:
//   - normalizeCtx      — 5 default-value guards (value copy)
//   - checkBudget       — R0/R0.5/R1/R2 early-exit budget gates
//   - checkRollupGuard  — RH-MUPS-03 (DM-20260701-001) cross-verdict rollup retry exhausted guard
//   - checkVerdictDirection — R3..R8 switch on round.VerdictKind
//
// Order of sub-decisions is explicit in SpawnPolicyEvaluator's 3-step
// composition. Adding a new spawn rule = add a case in the appropriate
// sub-decision function. Modifying rollup behavior = edit checkRollupGuard
// only (no more 3-place duplication as in R5/R6/R7 of the legacy switch).
//
// 0-behavior-change refactor. See openspec/changes/d7-spawn-decision-algebra/
// for design rationale and byte-equivalent validation.

package workmodel

import (
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// normalizeCtx applies 5 default-value guards to ctx. Returns a new ctx
// (TreeEvalContext is a value type, so this is pure / non-mutating).
// Hoisted out of SpawnPolicyEvaluator so the 3 sub-decisions see a fully
// defaulted context and the main function reads as a 3-step composition.
func normalizeCtx(ctx TreeEvalContext) TreeEvalContext {
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
	return ctx
}

// checkBudget applies R0 (running children), R0.5 (deliverable complete
// terminal), R1 (max depth with continuation), R2 (daily limit) — early
// budget gates independent of round.VerdictKind. Returns (SpawnNone, false)
// to fall through to the next sub-decision.
//
// 0-behavior-change refactor of SpawnPolicyEvaluator R0–R2 block.
func checkBudget(round *WorkItemPipelineRound, ctx TreeEvalContext) (SpawnPolicy, bool) {
	// R0
	if ctx.RunningChildren > 0 {
		return SpawnAwait, true
	}
	// R0.5 — CC-1.1: applicable deliverable satisfied → terminal before R1.
	if applicableDeliverableSchema(round) && !deliverableContinuationRequired(round) {
		return SpawnNone, true
	}
	// R1 — max depth with continuation: bounded inline, then escalate (CC-U1 may prefer rollup).
	if ctx.Depth >= ctx.MaxDepth {
		if deliverableContinuationRequired(round) {
			return spawnForDeliverableContinuation(round, ctx), true
		}
		if deliverableInlineWouldExhaust(ctx) {
			return SpawnEscalateHuman, true
		}
		return SpawnInline, true
	}
	// R2
	if ctx.DailyLimitExceeded {
		return SpawnEscalateHuman, true
	}
	return SpawnNone, false
}

// checkRollupGuard applies the RH-MUPS-03 (DM-20260701-001) rollup retry
// exhausted guard. Single source of truth for the rollup termination logic
// that used to be duplicated in R5/R6/R7 blocks of the legacy switch.
//
// VerdictPass is excluded: the legacy switch never applied rollup guard to
// R3/R4 (Pass → SpawnNone regardless of RollupRetries — verified by
// TestSpawnPolicyEvaluator_RollupPass_AlwaysNone, D7-S15-A89-T05).
//
// Returns (SpawnNone, false) when ctx.RollupRound is false OR
// round.VerdictKind == VerdictPass so the caller falls through to
// checkVerdictDirection.
func checkRollupGuard(round *WorkItemPipelineRound, ctx TreeEvalContext) (SpawnPolicy, bool) {
	if !ctx.RollupRound {
		return SpawnNone, false
	}
	// R3/R4 (Pass) skip rollup guard; the legacy switch applied rollup
	// guard only inside R5/R6/R7 verdict blocks.
	if round.VerdictKind == types.VerdictPass {
		return SpawnNone, false
	}
	if ctx.RollupRetries >= ctx.MaxRollupRetries {
		return SpawnEscalateHuman, true
	}
	return SpawnInline, true
}

// checkVerdictDirection applies R3..R8 by verdict kind. Rollup retry
// exhausted guard is hoisted to checkRollupGuard so this switch only
// handles the post-rollup verdict-direction routing.
func checkVerdictDirection(round *WorkItemPipelineRound, ctx TreeEvalContext) SpawnPolicy {
	switch round.VerdictKind {
	case types.VerdictPass:
		// CC-1 / §8.1: Pass with applicable schema MUST NOT SpawnNone while
		// deliverable is still owed — inline (or R1 budget) instead.
		if deliverableContinuationRequired(round) {
			return spawnForDeliverableContinuation(round, ctx)
		}
		return SpawnNone

	case types.VerdictPartial:
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
			return spawnForDeliverableContinuation(round, ctx)
		}
		return SpawnNone

	case types.VerdictFail:
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
