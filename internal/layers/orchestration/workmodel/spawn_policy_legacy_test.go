//go:build legacy_spawn
// +build legacy_spawn

// Package workmodel — spawn_policy_legacy_test.go (build tag: legacy_spawn)
//
// Preserves the original 50+ line SpawnPolicyEvaluator as
// SpawnPolicyEvaluatorLegacy for byte-equivalent validation against the
// 3-sub-decision algebra refactor (M5). This file is ONLY compiled when
// `-tags legacy_spawn` is passed to `go test`. It is removed in the
// follow-on `mups-cleanup-legacy` change.

package workmodel

import (
	"fmt"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// SpawnPolicyEvaluatorLegacy is the verbatim pre-M5 implementation of
// SpawnPolicyEvaluator. Only present under -tags legacy_spawn.
func SpawnPolicyEvaluatorLegacy(round *WorkItemPipelineRound, ctx TreeEvalContext) SpawnPolicy {
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
	// R1 — max depth with continuation: bounded inline, then escalate (CC-U1 may prefer rollup).
	if ctx.Depth >= ctx.MaxDepth {
		if deliverableContinuationRequired(round) {
			return spawnForDeliverableContinuation(round, ctx)
		}
		if deliverableInlineWouldExhaust(ctx) {
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
			return spawnForDeliverableContinuation(round, ctx)
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
			return spawnForDeliverableContinuation(round, ctx)
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

// -----------------------------------------------------------------------------
// Byte-equivalent validation: 22 cases
// -----------------------------------------------------------------------------

// TestSpawnPolicyEvaluatorRefactor_ByteEquivalent_OldVsNew exercises 22
// representative round+ctx combinations and asserts that the new 3-sub-decision
// algebra SpawnPolicyEvaluator produces byte-equal SpawnPolicy values to the
// pre-M5 SpawnPolicyEvaluatorLegacy.
//
// Run with: go test -tags legacy_spawn ./internal/layers/orchestration/workmodel/ -run ByteEquivalent
func TestSpawnPolicyEvaluatorRefactor_ByteEquivalent_OldVsNew(t *testing.T) {
	type tc struct {
		name  string
		round *WorkItemPipelineRound
		ctx   TreeEvalContext
	}

	mkRound := func(kind types.VerdictKind, p plan.PlanKind, u float64) *WorkItemPipelineRound {
		return &WorkItemPipelineRound{
			WorkItemID:      "wi_be",
			PlanID:          "plan_be",
			VerdictID:       "verdict_be",
			ObservationIDs:  []string{"obs_be"},
			VerdictKind:     kind,
			PlanKind:        p,
			UncertaintyMean: u,
		}
	}

	mkCtx := func() TreeEvalContext {
		return TreeEvalContext{
			Depth: 0, MaxDepth: 3, Threshold: 0.6, MaxIndeterminateRetries: 3,
		}
	}

	cases := []tc{
		// 1. nil round
		{"nil-round", nil, mkCtx()},

		// 2. R0
		{"R0-running-children", mkRound(types.VerdictPartial, plan.ExplorationPlan, 0.9),
			func() (c TreeEvalContext) { c = mkCtx(); c.RunningChildren = 2; return }()},

		// 3. R0.5 at depth 0
		{"R05-deliverable-complete-depth0", func() *WorkItemPipelineRound {
			r := mkRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
			r.DeliverableSchema = FirstRegisteredDeliverableSchema()
			r.DeliverableStatus = DeliverableStatusComplete
			return r
		}(), mkCtx()},

		// 4. R0.5 at max depth
		{"R05-deliverable-complete-maxdepth", func() *WorkItemPipelineRound {
			r := mkRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
			r.DeliverableSchema = FirstRegisteredDeliverableSchema()
			r.DeliverableStatus = DeliverableStatusComplete
			return r
		}(), func() (c TreeEvalContext) { c = mkCtx(); c.Depth = 3; c.MaxDepth = 3; return }()},

		// 5. R1 w/ cont
		{"R1-max-depth-with-cont", func() *WorkItemPipelineRound {
			r := mkRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
			r.DeliverableSchema = FirstRegisteredDeliverableSchema()
			r.DeliverableStatus = DeliverableStatusIncomplete
			return r
		}(), func() (c TreeEvalContext) { c = mkCtx(); c.Depth = 3; c.MaxDepth = 3; return }()},

		// 6. R1 w/ exhaust
		{"R1-inline-exhausted", func() *WorkItemPipelineRound {
			r := mkRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
			r.DeliverableSchema = FirstRegisteredDeliverableSchema()
			r.DeliverableStatus = DeliverableStatusIncomplete
			return r
		}(), func() (c TreeEvalContext) {
			c = mkCtx(); c.Depth = 3; c.MaxDepth = 3
			c.InlineRetriesAtMaxDepth = DefaultMaxInlineRetriesAtMaxDepth
			c.MaxInlineRetriesAtMaxDepth = DefaultMaxInlineRetriesAtMaxDepth
			return
		}()},

		// 7. R1 no schema
		{"R1-no-schema", mkRound(types.VerdictPartial, plan.ExplorationPlan, 0.9),
			func() (c TreeEvalContext) { c = mkCtx(); c.Depth = 3; c.MaxDepth = 3; return }()},

		// 8. R2
		{"R2-daily-limit", mkRound(types.VerdictPartial, plan.ExplorationPlan, 0.9),
			func() (c TreeEvalContext) { c = mkCtx(); c.DailyLimitExceeded = true; return }()},

		// 9. R3 Pass+CommitmentPlan
		{"R3-pass-commitment", mkRound(types.VerdictPass, plan.CommitmentPlan, 0.2), mkCtx()},

		// 10. R4 Pass+ExplorationPlan
		{"R4-pass-exploration", mkRound(types.VerdictPass, plan.ExplorationPlan, 0.2), mkCtx()},

		// 11. R3 Pass w/ cont (CC-1 §8.1)
		{"R3-pass-with-continuation", func() *WorkItemPipelineRound {
			r := mkRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
			r.DeliverableSchema = FirstRegisteredDeliverableSchema()
			r.DeliverableStatus = DeliverableStatusIncomplete
			return r
		}(), mkCtx()},

		// 12. R5 Partial+CommitmentPlan+low U
		{"R5-partial-low-U", mkRound(types.VerdictPartial, plan.ProtocolPlan, 0.5), mkCtx()},

		// 13. R5 exploratory decomposable
		{"R5-exploratory-decomposable", mkRound(types.VerdictPartial, plan.ExplorationPlan, 0.2),
			func() (c TreeEvalContext) { c = mkCtx(); c.CanDecompose = true; c.ChildTotal = 0; return }()},

		// 14. R5 explore leaf
		{"R5-explore-leaf", mkRound(types.VerdictPartial, plan.ExplorationPlan, 0.2),
			func() (c TreeEvalContext) { c = mkCtx(); c.CanDecompose = false; return }()},

		// 15. R5 high U
		{"R5-high-U", mkRound(types.VerdictPartial, plan.ProtocolPlan, 0.75), mkCtx()},

		// 16. R5 w/ cont
		{"R5-with-continuation", func() *WorkItemPipelineRound {
			r := mkRound(types.VerdictPartial, plan.ProtocolPlan, 0.5)
			r.DeliverableSchema = FirstRegisteredDeliverableSchema()
			r.DeliverableStatus = DeliverableStatusIncomplete
			return r
		}(), mkCtx()},

		// 17. R6 scenario
		{"R6-scenario-fail", mkRound(types.VerdictFail, plan.ScenarioPlan, 0.8), mkCtx()},

		// 18. R6 exploration decomposable
		{"R6-exploration-decomposable", mkRound(types.VerdictFail, plan.ExplorationPlan, 0.8),
			func() (c TreeEvalContext) { c = mkCtx(); c.CanDecompose = true; c.ChildTotal = 0; return }()},

		// 19. R6 explore leaf
		{"R6-explore-leaf-fail", mkRound(types.VerdictFail, plan.ExplorationPlan, 0.8),
			func() (c TreeEvalContext) { c = mkCtx(); c.CanDecompose = false; return }()},

		// 20. R6 commitment
		{"R6-commitment-fail", mkRound(types.VerdictFail, plan.CommitmentPlan, 0.8), mkCtx()},

		// 21. R7 retry
		{"R7-indeterminate-retry", mkRound(types.VerdictIndeterminate, plan.ExplorationPlan, 0.5),
			func() (c TreeEvalContext) { c = mkCtx(); c.IndeterminateRetries = 1; return }()},

		// 22. R8 unknown
		{"R8-unknown-verdict", mkRound(types.VerdictKind(99), plan.ProtocolPlan, 0.5), mkCtx()},
	}

	// Rollup guard dedicated cases (3 of 4; #4 is Pass+Rollup which is in
	// TestSpawnPolicyEvaluator_RollupPass_AlwaysNone and is verified by
	// checkRollupGuard_PassSkipsGuard).
	rollupCases := []tc{
		// 23. R5 rollup at-limit escalate
		{"R5-rollup-at-limit-escalate", mkRound(types.VerdictPartial, plan.CommitmentPlan, 0.2),
			func() (c TreeEvalContext) {
				c = mkCtx(); c.RollupRound = true
				c.RollupRetries = DefaultMaxRollupRetries
				c.MaxRollupRetries = DefaultMaxRollupRetries
				return
			}()},

		// 24. R6 rollup at-limit escalate
		{"R6-rollup-at-limit-escalate", mkRound(types.VerdictFail, plan.CommitmentPlan, 0.2),
			func() (c TreeEvalContext) {
				c = mkCtx(); c.RollupRound = true
				c.RollupRetries = DefaultMaxRollupRetries
				c.MaxRollupRetries = DefaultMaxRollupRetries
				return
			}()},

		// 25. R7 rollup at-limit escalate
		{"R7-rollup-at-limit-escalate", mkRound(types.VerdictIndeterminate, plan.CommitmentPlan, 0.2),
			func() (c TreeEvalContext) {
				c = mkCtx(); c.RollupRound = true
				c.RollupRetries = DefaultMaxRollupRetries
				c.MaxRollupRetries = DefaultMaxRollupRetries
				return
			}()},

		// 26. R5 rollup below-limit inline
		{"R5-rollup-below-limit-inline", mkRound(types.VerdictPartial, plan.CommitmentPlan, 0.2),
			func() (c TreeEvalContext) {
				c = mkCtx(); c.RollupRound = true
				c.RollupRetries = 1
				c.MaxRollupRetries = DefaultMaxRollupRetries
				return
			}()},

		// 27. Pass+Rollup fall-through (R3/R4 → None)
		{"R3-rollup-pass-fallthrough", mkRound(types.VerdictPass, plan.CommitmentPlan, 0.2),
			func() (c TreeEvalContext) {
				c = mkCtx(); c.RollupRound = true
				c.RollupRetries = 5
				c.MaxRollupRetries = 3
				return
			}()},
	}

	all := append(cases, rollupCases...)

	for i, c := range all {
		t.Run(fmt.Sprintf("case-%02d-%s", i+1, c.name), func(t *testing.T) {
			oldV := SpawnPolicyEvaluatorLegacy(c.round, c.ctx)
			newV := SpawnPolicyEvaluator(c.round, c.ctx)
			if oldV != newV {
				t.Errorf("byte mismatch: old=%q new=%q (case %d: %s)", oldV, newV, i+1, c.name)
			}
		})
	}
}
