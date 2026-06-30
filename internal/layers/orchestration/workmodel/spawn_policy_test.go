package workmodel

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

func baseRound(verdict types.VerdictKind, planKind plan.PlanKind, uncertainty float64) *WorkItemPipelineRound {
	return &WorkItemPipelineRound{
		WorkItemID:      "wi_test",
		PlanID:          "plan_1",
		VerdictID:       "verdict_1",
		ObservationIDs:  []string{"obs_1"},
		VerdictKind:     verdict,
		PlanKind:        planKind,
		UncertaintyMean: uncertainty,
	}
}

func baseCtx() TreeEvalContext {
	return TreeEvalContext{
		Depth:                   0,
		MaxDepth:                3,
		Threshold:               0.6,
		MaxIndeterminateRetries: 3,
	}
}

func TestSpawnPolicyEvaluator_R0_RunningChildren(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
	ctx := baseCtx()
	ctx.RunningChildren = 2
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnAwait {
		t.Fatalf("R0: got %q, want await", got)
	}
}

func TestSpawnPolicyEvaluator_R1_MaxDepth(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
	ctx := baseCtx()
	ctx.Depth = 3
	ctx.MaxDepth = 3
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnInline {
		t.Fatalf("R1: got %q, want inline", got)
	}
}

func TestSpawnPolicyEvaluator_R2_DailyLimit(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
	ctx := baseCtx()
	ctx.DailyLimitExceeded = true
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnEscalateHuman {
		t.Fatalf("R2: got %q, want escalate_human", got)
	}
}

func TestSpawnPolicyEvaluator_R3_CommitmentPass(t *testing.T) {
	round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
	if got := SpawnPolicyEvaluator(round, baseCtx()); got != SpawnNone {
		t.Fatalf("R3: got %q, want none", got)
	}
}

func TestSpawnPolicyEvaluator_R4_ExplorationPass(t *testing.T) {
	round := baseRound(types.VerdictPass, plan.ExplorationPlan, 0.2)
	if got := SpawnPolicyEvaluator(round, baseCtx()); got != SpawnNone {
		t.Fatalf("R4: got %q, want none", got)
	}
}

func TestSpawnPolicyEvaluator_R5_PartialHighUncertainty(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ProtocolPlan, 0.75)
	if got := SpawnPolicyEvaluator(round, baseCtx()); got != SpawnDecompose {
		t.Fatalf("R5: got %q, want decompose", got)
	}
}

func TestSpawnPolicyEvaluator_R5_PartialAtThreshold(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ProtocolPlan, 0.6)
	if got := SpawnPolicyEvaluator(round, baseCtx()); got != SpawnDecompose {
		t.Fatalf("R5 at threshold: got %q, want decompose", got)
	}
}

func TestSpawnPolicyEvaluator_R5_ExplorationPartialLowUncertainty_Decomposable(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.2)
	ctx := baseCtx()
	ctx.CanDecompose = true
	ctx.ChildTotal = 0
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnDecompose {
		t.Fatalf("R5 exploration partial decomposable: got %q, want decompose", got)
	}
}

func TestSpawnPolicyEvaluator_R5_ExploreLeafPartialInlines(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.2)
	ctx := baseCtx()
	ctx.CanDecompose = false
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnInline {
		t.Fatalf("R5 explore leaf partial: got %q, want inline", got)
	}
}

func TestSpawnPolicyEvaluator_R5_RollupPartialInlines(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.2)
	ctx := baseCtx()
	ctx.RollupRound = true
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnInline {
		t.Fatalf("rollup partial: got %q, want inline", got)
	}
}

func TestSpawnPolicyEvaluator_R5_PartialLowUncertainty(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ProtocolPlan, 0.5)
	if got := SpawnPolicyEvaluator(round, baseCtx()); got != SpawnNone {
		t.Fatalf("R5 low uncertainty: got %q, want none", got)
	}
}

func TestSpawnPolicyEvaluator_R6_ScenarioFail(t *testing.T) {
	round := baseRound(types.VerdictFail, plan.ScenarioPlan, 0.8)
	if got := SpawnPolicyEvaluator(round, baseCtx()); got != SpawnParallelExplore {
		t.Fatalf("R6 scenario: got %q, want parallel_explore", got)
	}
}

func TestSpawnPolicyEvaluator_R6_ExplorationFail(t *testing.T) {
	round := baseRound(types.VerdictFail, plan.ExplorationPlan, 0.8)
	ctx := baseCtx()
	ctx.CanDecompose = true
	ctx.ChildTotal = 0
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnDecompose {
		t.Fatalf("R6 exploration decomposable: got %q, want decompose", got)
	}
}

// T: D7-S15-A89-T01 (DM-20260701-001 RH-MUPS-03 SpawnPolicyEvaluator rollup guard)
//
// Rollup partial verdict at retry < limit → still SpawnInline.
func TestSpawnPolicyEvaluator_RollupPartial_BelowLimitInlines(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.2)
	ctx := baseCtx()
	ctx.RollupRound = true
	ctx.RollupRetries = 1
	ctx.MaxRollupRetries = DefaultMaxRollupRetries
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnInline {
		t.Fatalf("rollup partial retry 1/3: got %q, want inline", got)
	}
}

// T: D7-S15-A89-T02 (DM-20260701-001 RH-MUPS-03 SpawnPolicyEvaluator rollup guard)
//
// Rollup partial verdict at retry == limit → SpawnEscalateHuman (was SpawnInline).
func TestSpawnPolicyEvaluator_RollupPartial_AtLimitEscalates(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.2)
	ctx := baseCtx()
	ctx.RollupRound = true
	ctx.RollupRetries = DefaultMaxRollupRetries
	ctx.MaxRollupRetries = DefaultMaxRollupRetries
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnEscalateHuman {
		t.Fatalf("rollup partial retry %d/%d: got %q, want escalate_human",
			ctx.RollupRetries, ctx.MaxRollupRetries, got)
	}
}

// T: D7-S15-A89-T03 (DM-20260701-001 RH-MUPS-03 SpawnPolicyEvaluator rollup guard)
//
// Rollup fail verdict at retry == limit → SpawnEscalateHuman (was SpawnInline).
func TestSpawnPolicyEvaluator_RollupFail_AtLimitEscalates(t *testing.T) {
	round := baseRound(types.VerdictFail, plan.CommitmentPlan, 0.2)
	ctx := baseCtx()
	ctx.RollupRound = true
	ctx.RollupRetries = DefaultMaxRollupRetries
	ctx.MaxRollupRetries = DefaultMaxRollupRetries
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnEscalateHuman {
		t.Fatalf("rollup fail retry %d/%d: got %q, want escalate_human",
			ctx.RollupRetries, ctx.MaxRollupRetries, got)
	}
}

// T: D7-S15-A89-T04 (DM-20260701-001 RH-MUPS-03 SpawnPolicyEvaluator rollup guard)
//
// Rollup indeterminate at retry == limit → SpawnEscalateHuman.
func TestSpawnPolicyEvaluator_RollupIndeterminate_AtLimitEscalates(t *testing.T) {
	round := baseRound(types.VerdictIndeterminate, plan.CommitmentPlan, 0.2)
	ctx := baseCtx()
	ctx.RollupRound = true
	ctx.RollupRetries = DefaultMaxRollupRetries
	ctx.MaxRollupRetries = DefaultMaxRollupRetries
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnEscalateHuman {
		t.Fatalf("rollup indeterminate retry %d/%d: got %q, want escalate_human",
			ctx.RollupRetries, ctx.MaxRollupRetries, got)
	}
}

// T: D7-S15-A89-T05 (DM-20260701-001 RH-MUPS-03 SpawnPolicyEvaluator rollup guard)
//
// Rollup pass verdict → SpawnNone regardless of retry count (success resets).
func TestSpawnPolicyEvaluator_RollupPass_AlwaysNone(t *testing.T) {
	round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
	ctx := baseCtx()
	ctx.RollupRound = true
	ctx.RollupRetries = 5 // any prior retry count
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnNone {
		t.Fatalf("rollup pass: got %q, want none", got)
	}
}

func TestSpawnPolicyEvaluator_R6_ExploreLeafFailInlines(t *testing.T) {
	round := baseRound(types.VerdictFail, plan.ExplorationPlan, 0.8)
	ctx := baseCtx()
	ctx.CanDecompose = false
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnInline {
		t.Fatalf("R6 explore leaf fail: got %q, want inline", got)
	}
}

func TestSpawnPolicyEvaluator_R6_CommitmentFail(t *testing.T) {
	round := baseRound(types.VerdictFail, plan.CommitmentPlan, 0.8)
	if got := SpawnPolicyEvaluator(round, baseCtx()); got != SpawnNone {
		t.Fatalf("R6 commitment fail: got %q, want none", got)
	}
}

func TestSpawnPolicyEvaluator_R7_IndeterminateRetry(t *testing.T) {
	round := baseRound(types.VerdictIndeterminate, plan.ExplorationPlan, 0.5)
	ctx := baseCtx()
	ctx.IndeterminateRetries = 1
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnInline {
		t.Fatalf("R7 retry: got %q, want inline", got)
	}
}

func TestSpawnPolicyEvaluator_R7_IndeterminateExhausted_ExploratoryDecomposes(t *testing.T) {
	round := baseRound(types.VerdictIndeterminate, plan.ExplorationPlan, 0.5)
	ctx := baseCtx()
	ctx.CanDecompose = true
	ctx.ChildTotal = 0
	ctx.IndeterminateRetries = 3
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnDecompose {
		t.Fatalf("R7 exhausted exploratory: got %q, want decompose", got)
	}
}

func TestSpawnPolicyEvaluator_R7_ExploreLeafIndeterminateExhaustedEscalates(t *testing.T) {
	round := baseRound(types.VerdictIndeterminate, plan.ExplorationPlan, 0.5)
	ctx := baseCtx()
	ctx.CanDecompose = false
	ctx.IndeterminateRetries = 3
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnEscalateHuman {
		t.Fatalf("R7 explore leaf exhausted: got %q, want escalate_human", got)
	}
}

func TestSpawnPolicyEvaluator_R7_IndeterminateExhausted_CommitmentEscalatesHuman(t *testing.T) {
	round := baseRound(types.VerdictIndeterminate, plan.CommitmentPlan, 0.5)
	ctx := baseCtx()
	ctx.IndeterminateRetries = 3
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnEscalateHuman {
		t.Fatalf("R7 exhausted commitment: got %q, want escalate_human", got)
	}
}

func TestSpawnPolicyEvaluator_R8_UnknownVerdict(t *testing.T) {
	round := baseRound(types.VerdictKind(99), plan.ProtocolPlan, 0.5)
	if got := SpawnPolicyEvaluator(round, baseCtx()); got != SpawnNone {
		t.Fatalf("R8: got %q, want none", got)
	}
}

func TestSpawnPolicyEvaluator_NilRound(t *testing.T) {
	if got := SpawnPolicyEvaluator(nil, baseCtx()); got != SpawnNone {
		t.Fatalf("nil round: got %q, want none", got)
	}
}

func TestEvaluateSpawnPolicy_SetsRationale(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
	ctx := baseCtx()
	ctx.CanDecompose = true
	ctx.ChildTotal = 0
	EvaluateSpawnPolicy(round, ctx)
	if round.SpawnPolicy != SpawnDecompose {
		t.Fatalf("policy = %q, want decompose", round.SpawnPolicy)
	}
	if round.SpawnRationale == "" {
		t.Fatal("expected non-empty spawn rationale")
	}
}

func TestValidateSpawnDecompose(t *testing.T) {
	valid := baseRound(types.VerdictPartial, plan.ExplorationPlan, 0.9)
	valid.SpawnPolicy = SpawnDecompose
	if err := ValidateSpawnDecompose(valid); err != nil {
		t.Fatalf("valid round: %v", err)
	}

	wrongPolicy := *valid
	wrongPolicy.SpawnPolicy = SpawnNone
	if err := ValidateSpawnDecompose(&wrongPolicy); err != errSpawnPolicyNotDecompose {
		t.Fatalf("wrong policy err = %v", err)
	}

	incomplete := *valid
	incomplete.PlanID = ""
	if err := ValidateSpawnDecompose(&incomplete); err != errSpawnRoundIncomplete {
		t.Fatalf("incomplete err = %v", err)
	}
}

func TestCapChildSpecs(t *testing.T) {
	specs := make([]ChildSpec, 10)
	for i := range specs {
		specs[i] = ChildSpec{Title: "c"}
	}
	capped := CapChildSpecs(specs)
	if len(capped) != DefaultMaxChildren {
		t.Fatalf("cap len = %d, want %d", len(capped), DefaultMaxChildren)
	}
}

func TestResolveHint_FromLastRound(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	goal.LastRound = &WorkItemPipelineRound{SpawnPolicy: SpawnAwait}
	hint := ResolveHint("s1", tm, goal)
	if !strings.Contains(hint, "await") {
		t.Fatalf("hint = %q, want await guidance from LastRound", hint)
	}
}
