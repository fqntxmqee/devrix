package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/escape"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestEvaluateSessionLoopExitAfterRound_EscalateHuman(t *testing.T) {
	round := &workmodel.WorkItemPipelineRound{
		SpawnPolicy: workmodel.SpawnEscalateHuman,
		SpawnRationale: "rollup retries exhausted",
	}
	got := evaluateSessionLoopExitAfterRound(context.Background(), "s1", nil, round, escape.EscapeDecision{})
	if got.Kind != SessionLoopExitEscalate {
		t.Fatalf("kind=%q want escalate_human", got.Kind)
	}
}

func TestEvaluateSessionLoopExitAfterRound_EscapeForceExit(t *testing.T) {
	got := evaluateSessionLoopExitAfterRound(context.Background(), "s1", nil, nil, escape.EscapeDecision{
		Action: escape.EscapeForceExit,
		Reason: "loop_depth_exceeded",
	})
	if got.Kind != SessionLoopExitEscape {
		t.Fatalf("kind=%q want escape", got.Kind)
	}
}

func TestEvaluateSessionLoopExitAfterRound_PlanningTextAnomaly(t *testing.T) {
	tm := workmodel.NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "review code")
	round := &workmodel.WorkItemPipelineRound{
		WorkItemID:        goal.ID,
		VerdictKind:       types.VerdictPartial,
		SpawnPolicy:       workmodel.SpawnInline,
		DeliverableSchema: workmodel.FirstRegisteredDeliverableSchema(),
		DeliverableStatus: workmodel.DeliverableStatusIncomplete,
		ArtifactSummary:   "Let me continue reading the file.",
	}
	got := evaluateSessionLoopExitAfterRound(context.Background(), "s1", tm, round, escape.EscapeDecision{})
	if got.ShouldExit() {
		t.Fatalf("inline retry should continue loop, got exit kind=%q reason=%q", got.Kind, got.Reason)
	}
	round.SpawnPolicy = workmodel.SpawnNone
	_ = tm.Tree().ApplyPipelineRound("s1", goal.ID, round, workmodel.RoundPhaseIdle)
	got = evaluateSessionLoopExitAfterRound(context.Background(), "s1", tm, round, escape.EscapeDecision{})
	if got.Kind != SessionLoopExitAnomaly {
		t.Fatalf("stagnation should exit with anomaly, got kind=%q reason=%q", got.Kind, got.Reason)
	}
}

func TestBuildEscapeLoopContextFromRound_DecomposeSkipsDeliverableHash(t *testing.T) {
	round := &workmodel.WorkItemPipelineRound{
		SpawnPolicy:       workmodel.SpawnDecompose,
		DeliverableSchema: workmodel.FirstRegisteredDeliverableSchema(),
		DeliverableStatus: workmodel.DeliverableStatusIncomplete,
		PlanKind:          plan.CommitmentPlan,
		ExitReason:        "partial_verified",
	}
	ctx := buildEscapeLoopContextFromRound("s1", round)
	if strings.Contains(ctx.FailureCriterion, "deliverable_incomplete") {
		t.Fatalf("decompose forward progress must not use deliverable_incomplete hash, got %q", ctx.FailureCriterion)
	}
	if ctx.FailureCriterion != "partial_verified" {
		t.Fatalf("failure = %q, want exit_reason", ctx.FailureCriterion)
	}
}

func TestDeliverableIncompleteEscapeCriterion_InlineUsesHash(t *testing.T) {
	contract := workmodel.DefaultTestDeliverableContract()
	round := &workmodel.WorkItemPipelineRound{
		SpawnPolicy:         workmodel.SpawnInline,
		DeliverableContract: contract,
		DeliverableStatus:   workmodel.DeliverableStatusIncomplete,
	}
	got := deliverableIncompleteEscapeCriterion(round)
	if got != "deliverable_incomplete:"+contract.CacheKey() {
		t.Fatalf("got %q", got)
	}
}

func TestIsExplorationPlanningText(t *testing.T) {
	if !isExplorationPlanningText("Let me first locate the kernel directory.") {
		t.Fatal("expected planning marker match")
	}
	if isExplorationPlanningText("P0: internal/foo.go:42 — nil deref") {
		t.Fatal("review finding should not match planning marker")
	}
}
