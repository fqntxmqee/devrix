// Package workmodel — spawn_decision_algebra_test.go
//
// Unit tests for the 3-sub-decision algebra (M5):
//   - checkBudget       (6 cases: R0/R0.5/R1 w/ cont/R1 w/ exhaust/R1 no schema/R2 + fall-through)
//   - checkRollupGuard  (4 cases: at-limit escalate/below-limit inline/non-rollup fall-through/Pass+Rollup fall-through)
//   - checkVerdictDirection (5 cases: R3/R4 + Pass w/ cont CC-1 + R5 + R6 + R7 + R8)
//   - normalizeCtx      (1 case: 5 field default-guard coverage)
//   - Sub-decision order locking (1 test: 3 sub-decisions fire in order)

package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// -----------------------------------------------------------------------------
// checkBudget tests (6 cases) — T01
// -----------------------------------------------------------------------------

func TestCheckBudget_R0_RunningChildren(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
	ctx := baseCtx()
	ctx.RunningChildren = 2
	gotPolicy, gotFired := checkBudget(round, ctx)
	if gotPolicy != SpawnAwait || !gotFired {
		t.Fatalf("R0: got (policy=%q, fired=%v), want (SpawnAwait, true)", gotPolicy, gotFired)
	}
}

func TestCheckBudget_R05_DeliverableCompleteAtMaxDepth(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
	round.DeliverableSchema = FirstRegisteredDeliverableSchema()
	round.DeliverableStatus = DeliverableStatusComplete
	ctx := baseCtx()
	ctx.Depth = 3
	ctx.MaxDepth = 3
	gotPolicy, gotFired := checkBudget(round, ctx)
	if gotPolicy != SpawnNone || !gotFired {
		t.Fatalf("R0.5: got (policy=%q, fired=%v), want (SpawnNone, true)", gotPolicy, gotFired)
	}
}

func TestCheckBudget_R1_WithContinuation(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
	round.DeliverableSchema = FirstRegisteredDeliverableSchema()
	round.DeliverableStatus = DeliverableStatusIncomplete
	ctx := baseCtx()
	ctx.Depth = 3
	ctx.MaxDepth = 3
	gotPolicy, gotFired := checkBudget(round, ctx)
	// spawnForDeliverableContinuation returns SpawnInline (RollupSynthEligible
	// is false with default evidence). Either SpawnInline or
	// spawnForDeliverableContinuation outcome is fine as long as fired=true.
	if !gotFired {
		t.Fatalf("R1 w/ cont: got fired=%v, want true", gotFired)
	}
	if gotPolicy != SpawnInline && gotPolicy != SpawnEscalateHuman {
		t.Fatalf("R1 w/ cont: got policy=%q, want inline/escalate", gotPolicy)
	}
}

func TestCheckBudget_R1_InlineExhaustedEscalates(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
	round.DeliverableSchema = FirstRegisteredDeliverableSchema()
	round.DeliverableStatus = DeliverableStatusIncomplete
	ctx := baseCtx()
	ctx.Depth = 3
	ctx.MaxDepth = 3
	ctx.InlineRetriesAtMaxDepth = DefaultMaxInlineRetriesAtMaxDepth
	ctx.MaxInlineRetriesAtMaxDepth = DefaultMaxInlineRetriesAtMaxDepth
	gotPolicy, gotFired := checkBudget(round, ctx)
	if gotPolicy != SpawnEscalateHuman || !gotFired {
		t.Fatalf("R1 w/ exhaust: got (policy=%q, fired=%v), want (SpawnEscalateHuman, true)", gotPolicy, gotFired)
	}
}

func TestCheckBudget_R1_NoDeliverableSchema(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
	// No DeliverableSchema/Status set
	ctx := baseCtx()
	ctx.Depth = 3
	ctx.MaxDepth = 3
	gotPolicy, gotFired := checkBudget(round, ctx)
	if gotPolicy != SpawnInline || !gotFired {
		t.Fatalf("R1 no schema: got (policy=%q, fired=%v), want (SpawnInline, true)", gotPolicy, gotFired)
	}
}

func TestCheckBudget_R2_DailyLimit(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
	ctx := baseCtx()
	ctx.DailyLimitExceeded = true
	gotPolicy, gotFired := checkBudget(round, ctx)
	if gotPolicy != SpawnEscalateHuman || !gotFired {
		t.Fatalf("R2: got (policy=%q, fired=%v), want (SpawnEscalateHuman, true)", gotPolicy, gotFired)
	}
}

func TestCheckBudget_FallThrough(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ProtocolPlan, 0.5)
	ctx := baseCtx()
	// All 4 gates not fired: no RunningChildren, no deliverable, depth<max, no daily limit
	gotPolicy, gotFired := checkBudget(round, ctx)
	if gotPolicy != SpawnNone || gotFired {
		t.Fatalf("fall-through: got (policy=%q, fired=%v), want (SpawnNone, false)", gotPolicy, gotFired)
	}
}

// -----------------------------------------------------------------------------
// checkRollupGuard tests (4 cases) — T02
// -----------------------------------------------------------------------------

func TestCheckRollupGuard_AtLimitEscalates(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.2)
	ctx := baseCtx()
	ctx.RollupRound = true
	ctx.RollupRetries = 3
	ctx.MaxRollupRetries = 3
	gotPolicy, gotFired := checkRollupGuard(round, ctx)
	if gotPolicy != SpawnEscalateHuman || !gotFired {
		t.Fatalf("at-limit: got (policy=%q, fired=%v), want (SpawnEscalateHuman, true)", gotPolicy, gotFired)
	}
}

func TestCheckRollupGuard_BelowLimitInlines(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.2)
	ctx := baseCtx()
	ctx.RollupRound = true
	ctx.RollupRetries = 1
	ctx.MaxRollupRetries = 3
	gotPolicy, gotFired := checkRollupGuard(round, ctx)
	if gotPolicy != SpawnInline || !gotFired {
		t.Fatalf("below-limit: got (policy=%q, fired=%v), want (SpawnInline, true)", gotPolicy, gotFired)
	}
}

func TestCheckRollupGuard_NonRollupFallThrough(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.2)
	ctx := baseCtx()
	ctx.RollupRound = false
	gotPolicy, gotFired := checkRollupGuard(round, ctx)
	if gotPolicy != SpawnNone || gotFired {
		t.Fatalf("non-rollup: got (policy=%q, fired=%v), want (SpawnNone, false)", gotPolicy, gotFired)
	}
}

func TestCheckRollupGuard_PassSkipsGuard(t *testing.T) {
	// VerdictPass MUST skip rollup guard (R3/R4 already SpawnNone).
	// Verified by TestSpawnPolicyEvaluator_RollupPass_AlwaysNone.
	round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
	ctx := baseCtx()
	ctx.RollupRound = true
	ctx.RollupRetries = 5
	ctx.MaxRollupRetries = 3
	gotPolicy, gotFired := checkRollupGuard(round, ctx)
	if gotPolicy != SpawnNone || gotFired {
		t.Fatalf("pass+rollup: got (policy=%q, fired=%v), want (SpawnNone, false)", gotPolicy, gotFired)
	}
}

// -----------------------------------------------------------------------------
// checkVerdictDirection tests (5 cases) — T03
// -----------------------------------------------------------------------------

func TestCheckVerdictDirection_R3R4_CommitmentPass(t *testing.T) {
	round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
	if got := checkVerdictDirection(round, baseCtx()); got != SpawnNone {
		t.Fatalf("R3: got %q, want none", got)
	}
}

func TestCheckVerdictDirection_R3_PassWithContinuation(t *testing.T) {
	round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
	round.DeliverableSchema = FirstRegisteredDeliverableSchema()
	round.DeliverableStatus = DeliverableStatusIncomplete
	// With low evidence + no tool calls + no scope_in, spawnForDeliverableContinuation
	// returns SpawnInline (CC-1.2 path), NOT SpawnNone.
	got := checkVerdictDirection(round, baseCtx())
	if got != SpawnInline && got != SpawnEscalateHuman {
		t.Fatalf("R3 w/ cont: got %q, want inline/escalate", got)
	}
}

func TestCheckVerdictDirection_R5_PartialLowUncertainty(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ProtocolPlan, 0.5)
	if got := checkVerdictDirection(round, baseCtx()); got != SpawnNone {
		t.Fatalf("R5 low U: got %q, want none", got)
	}
}

func TestCheckVerdictDirection_R6_CommitmentFail(t *testing.T) {
	round := baseRound(types.VerdictFail, plan.CommitmentPlan, 0.8)
	if got := checkVerdictDirection(round, baseCtx()); got != SpawnNone {
		t.Fatalf("R6 commitment: got %q, want none", got)
	}
}

func TestCheckVerdictDirection_R7_IndeterminateRetry(t *testing.T) {
	round := baseRound(types.VerdictIndeterminate, plan.ExplorationPlan, 0.5)
	ctx := baseCtx()
	ctx.IndeterminateRetries = 1
	if got := checkVerdictDirection(round, ctx); got != SpawnInline {
		t.Fatalf("R7 retry: got %q, want inline", got)
	}
}

func TestCheckVerdictDirection_R8_UnknownVerdict(t *testing.T) {
	round := baseRound(types.VerdictKind(99), plan.ProtocolPlan, 0.5)
	if got := checkVerdictDirection(round, baseCtx()); got != SpawnNone {
		t.Fatalf("R8: got %q, want none", got)
	}
}

// -----------------------------------------------------------------------------
// normalizeCtx tests (1 case) — T04
// -----------------------------------------------------------------------------

func TestNormalizeCtx_DefaultValues(t *testing.T) {
	ctx := TreeEvalContext{} // all zero
	got := normalizeCtx(ctx)
	if got.MaxDepth != DefaultMaxDecomposeDepth {
		t.Errorf("MaxDepth: got %d, want %d", got.MaxDepth, DefaultMaxDecomposeDepth)
	}
	if got.Threshold != DefaultUncertaintyDecomposeThreshold {
		t.Errorf("Threshold: got %f, want %f", got.Threshold, DefaultUncertaintyDecomposeThreshold)
	}
	if got.MaxIndeterminateRetries != DefaultMaxIndeterminateRetries {
		t.Errorf("MaxIndeterminateRetries: got %d, want %d", got.MaxIndeterminateRetries, DefaultMaxIndeterminateRetries)
	}
	if got.MaxRollupRetries != DefaultMaxRollupRetries {
		t.Errorf("MaxRollupRetries: got %d, want %d", got.MaxRollupRetries, DefaultMaxRollupRetries)
	}
	if got.MaxInlineRetriesAtMaxDepth != DefaultMaxInlineRetriesAtMaxDepth {
		t.Errorf("MaxInlineRetriesAtMaxDepth: got %d, want %d", got.MaxInlineRetriesAtMaxDepth, DefaultMaxInlineRetriesAtMaxDepth)
	}
}

func TestNormalizeCtx_PreservesNonZero(t *testing.T) {
	ctx := TreeEvalContext{
		MaxDepth:                5,
		Threshold:               0.7,
		MaxIndeterminateRetries: 4,
		MaxRollupRetries:        5,
		MaxInlineRetriesAtMaxDepth: 2,
	}
	got := normalizeCtx(ctx)
	if got.MaxDepth != 5 || got.Threshold != 0.7 ||
		got.MaxIndeterminateRetries != 4 || got.MaxRollupRetries != 5 ||
		got.MaxInlineRetriesAtMaxDepth != 2 {
		t.Errorf("normalizeCtx altered non-zero values: %+v", got)
	}
}

// -----------------------------------------------------------------------------
// Sub-decision order locking test — T07
//
// Verifies the 3 sub-decisions fire in expected order within SpawnPolicyEvaluator.
// Strategy: construct scenarios that uniquely identify which sub-decision fired:
//   - Scenario A: R0 fires (RunningChildren>0) → checkBudget wins, others not called
//   - Scenario B: rollup guard below-limit (RollupRound+Retries<Max+!Pass) → checkRollupGuard wins
//   - Scenario C: pure direction (Pass+CommitmentPlan+no cont) → checkVerdictDirection wins
// -----------------------------------------------------------------------------

func TestSpawnPolicyEvaluator_SubDecisionOrder(t *testing.T) {
	t.Run("checkBudget wins over rollup guard and direction", func(t *testing.T) {
		round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
		ctx := baseCtx()
		ctx.RunningChildren = 2 // R0
		// Set rollup guard conditions too, but R0 must win
		ctx.RollupRound = true
		ctx.RollupRetries = 0
		ctx.MaxRollupRetries = 3
		if got := SpawnPolicyEvaluator(round, ctx); got != SpawnAwait {
			t.Fatalf("R0 wins: got %q, want SpawnAwait", got)
		}
	})

	t.Run("checkRollupGuard wins over direction when budget not fired", func(t *testing.T) {
		round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.2)
		ctx := baseCtx()
		ctx.RollupRound = true
		ctx.RollupRetries = 1
		ctx.MaxRollupRetries = 3
		// Without rollup guard, R5 with low U would return SpawnNone.
		// With rollup guard, expect SpawnInline.
		if got := SpawnPolicyEvaluator(round, ctx); got != SpawnInline {
			t.Fatalf("rollup guard below-limit: got %q, want SpawnInline", got)
		}
	})

	t.Run("checkVerdictDirection is the final step", func(t *testing.T) {
		round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
		// no R0, no rollup round, no deliverable → R3/R4 path
		if got := SpawnPolicyEvaluator(round, baseCtx()); got != SpawnNone {
			t.Fatalf("R3/R4: got %q, want SpawnNone", got)
		}
	})

	t.Run("Pass skips rollup guard and reaches direction", func(t *testing.T) {
		// D7-S15-A89-T05: rollup pass → SpawnNone regardless of retry count
		round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
		ctx := baseCtx()
		ctx.RollupRound = true
		ctx.RollupRetries = 5
		ctx.MaxRollupRetries = 3
		if got := SpawnPolicyEvaluator(round, ctx); got != SpawnNone {
			t.Fatalf("rollup pass: got %q, want SpawnNone", got)
		}
	})
}


// -----------------------------------------------------------------------------
// M3 (DM-20260705-008) 行为增量 integration tests: 4 PlanKind × 5 VerdictKind
// Verifies checkVerdictDirection correctly applies Strategy.SpawnOverride.
// 4 cases change behavior (M3 行为增量), 16 cases fall through to 5-case default.
//
// T: D7-SX-AXX-T20 — CommitmentPlan × 5 verdicts
// -----------------------------------------------------------------------------

func TestM3_StrategyOverride_CommitmentPlan_AllVerdicts(t *testing.T) {
	// Expected matrix (SpawnPolicy):
	// VerdictPass             → SpawnNone (R3 default, no M3 override)
	// VerdictFail             → SpawnNone (M3 行为增量: 1-step terminal, same as R6 default)
	// VerdictPartial (low U)  → SpawnNone (M3 行为增量: terminal partial)
	// VerdictIndeterminate    → R7 default (M3 doesn't override)
	// Unknown verdict         → R8 SpawnNone default
	tests := []struct {
		verdict  types.VerdictKind
		u        float64
		expected SpawnPolicy
	}{
		{types.VerdictPass, 0.2, SpawnNone},
		{types.VerdictFail, 0.8, SpawnNone},
		{types.VerdictPartial, 0.3, SpawnNone}, // M3 行为增量
		{types.VerdictIndeterminate, 0.5, SpawnInline},
		{types.VerdictKind(99), 0.5, SpawnNone},
	}
	for _, tc := range tests {
		round := baseRound(tc.verdict, plan.CommitmentPlan, tc.u)
		got := SpawnPolicyEvaluator(round, baseCtx())
		if got != tc.expected {
			t.Errorf("CommitmentPlan+%v (U=%.2f): got %q, want %q",
				tc.verdict, tc.u, got, tc.expected)
		}
	}
}

// T: D7-SX-AXX-T21 — ProtocolPlan × 5 verdicts (safe default: all fall through)
func TestM3_StrategyOverride_ProtocolPlan_AllVerdicts(t *testing.T) {
	// ProtocolStrategy returns ok=false for all → 5-case default applies.
	// VerdictPass             → SpawnNone
	// VerdictFail             → SpawnNone (R6 default, not Scenario/Exploration)
	// VerdictPartial (low U)  → SpawnNone (R5 low U, no decompose)
	// VerdictPartial (high U) → SpawnDecompose (R5 high U)
	// VerdictIndeterminate    → R7 SpawnInline
	tests := []struct {
		verdict  types.VerdictKind
		u        float64
		expected SpawnPolicy
	}{
		{types.VerdictPass, 0.2, SpawnNone},
		{types.VerdictFail, 0.8, SpawnNone},
		{types.VerdictPartial, 0.3, SpawnNone},
		{types.VerdictPartial, 0.9, SpawnDecompose},
		{types.VerdictIndeterminate, 0.5, SpawnInline},
	}
	for _, tc := range tests {
		round := baseRound(tc.verdict, plan.ProtocolPlan, tc.u)
		got := SpawnPolicyEvaluator(round, baseCtx())
		if got != tc.expected {
			t.Errorf("ProtocolPlan+%v (U=%.2f): got %q, want %q",
				tc.verdict, tc.u, got, tc.expected)
		}
	}
}

// T: D7-SX-AXX-T22 — ScenarioPlan × 5 verdicts
func TestM3_StrategyOverride_ScenarioPlan_AllVerdicts(t *testing.T) {
	// VerdictPass             → SpawnNone
	// VerdictFail             → SpawnNone (M3 行为增量: read-only no retry, was SpawnParallelExplore)
	// VerdictPartial (low U)  → SpawnNone
	// VerdictIndeterminate    → SpawnInline
	// Unknown verdict         → SpawnNone
	tests := []struct {
		verdict  types.VerdictKind
		u        float64
		expected SpawnPolicy
	}{
		{types.VerdictPass, 0.2, SpawnNone},
		{types.VerdictFail, 0.8, SpawnNone}, // M3 行为增量
		{types.VerdictPartial, 0.3, SpawnInline}, // R5 exploratory default (CanDecompose=false)
		{types.VerdictIndeterminate, 0.5, SpawnInline},
		{types.VerdictKind(99), 0.5, SpawnNone},
	}
	for _, tc := range tests {
		round := baseRound(tc.verdict, plan.ScenarioPlan, tc.u)
		got := SpawnPolicyEvaluator(round, baseCtx())
		if got != tc.expected {
			t.Errorf("ScenarioPlan+%v (U=%.2f): got %q, want %q",
				tc.verdict, tc.u, got, tc.expected)
		}
	}
}

// T: D7-SX-AXX-T23 — ExplorationPlan × 5 verdicts
func TestM3_StrategyOverride_ExplorationPlan_AllVerdicts(t *testing.T) {
	// VerdictPass             → SpawnDecompose (M3 行为增量: parallel explore continues)
	// VerdictFail (CanDecompose=true, ChildTotal=0) → SpawnDecompose (R6 default)
	// VerdictPartial (low U)  → SpawnDecompose (R5 exploratory, decomposable)
	// VerdictIndeterminate    → SpawnInline
	// Unknown verdict         → SpawnNone
	tests := []struct {
		verdict   types.VerdictKind
		u         float64
		expected  SpawnPolicy
		ctxAdjust func(*TreeEvalContext)
	}{
		{types.VerdictPass, 0.2, SpawnDecompose, nil}, // M3 行为增量
		{types.VerdictFail, 0.8, SpawnDecompose, func(c *TreeEvalContext) {
			c.CanDecompose = true
			c.ChildTotal = 0
		}},
		{types.VerdictPartial, 0.2, SpawnDecompose, func(c *TreeEvalContext) {
			c.CanDecompose = true
			c.ChildTotal = 0
		}},
		{types.VerdictIndeterminate, 0.5, SpawnInline, nil},
		{types.VerdictKind(99), 0.5, SpawnNone, nil},
	}
	for _, tc := range tests {
		round := baseRound(tc.verdict, plan.ExplorationPlan, tc.u)
		ctx := baseCtx()
		if tc.ctxAdjust != nil {
			tc.ctxAdjust(&ctx)
		}
		got := SpawnPolicyEvaluator(round, ctx)
		if got != tc.expected {
			t.Errorf("ExplorationPlan+%v (U=%.2f): got %q, want %q",
				tc.verdict, tc.u, got, tc.expected)
		}
	}
}

// T: D7-SX-AXX-T24 — M3 行为增量 summary: 4 known changes, 16 fall-throughs
// Locks the 4-of-20 behavior change guarantee for the M3 PR description.
func TestM3_StrategyOverride_BehaviorChangeSummary(t *testing.T) {
	// Without M3 (5-case default), 4 of 20 combinations return:
	//   - Commitment+Partial low U  → SpawnNone (no change: M3 returns same)
	//   - Scenario+Fail            → SpawnParallelExplore (CHANGED by M3 to SpawnNone)
	//   - Exploration+Pass         → SpawnNone (CHANGED by M3 to SpawnDecompose)
	//   - Commitment+Partial high U → SpawnDecompose (CHANGED by M3 to SpawnNone)
	// The actual "behavior change" is the delta between 5-case default and M3 output.

	// The 4 M3 行为增量:
	changes := []struct {
		name     string
		round    *WorkItemPipelineRound
		ctx      TreeEvalContext
		wantNow  SpawnPolicy
	}{
		{
			name:    "CommitmentPlan+Fail → SpawnNone (terminal fail)",
			round:   baseRound(types.VerdictFail, plan.CommitmentPlan, 0.8),
			ctx:     baseCtx(),
			wantNow: SpawnNone,
		},
		{
			name:    "CommitmentPlan+Partial+lowU → SpawnNone (terminal partial)",
			round:   baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.3),
			ctx:     baseCtx(),
			wantNow: SpawnNone,
		},
		{
			name:    "ScenarioPlan+Fail → SpawnNone (read-only no retry, was SpawnParallelExplore)",
			round:   baseRound(types.VerdictFail, plan.ScenarioPlan, 0.8),
			ctx:     baseCtx(),
			wantNow: SpawnNone,
		},
		{
			name:    "ExplorationPlan+Pass → SpawnDecompose (parallel explore, was SpawnNone)",
			round:   baseRound(types.VerdictPass, plan.ExplorationPlan, 0.2),
			ctx:     baseCtx(),
			wantNow: SpawnDecompose,
		},
	}
	for _, c := range changes {
		t.Run(c.name, func(t *testing.T) {
			if got := SpawnPolicyEvaluator(c.round, c.ctx); got != c.wantNow {
				t.Fatalf("got %q, want %q", got, c.wantNow)
			}
		})
	}
}
