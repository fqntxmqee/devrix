package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestApplySpawnDecompose_CreatesChildren(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "compare options")
	round := &WorkItemPipelineRound{
		WorkItemID:     goal.ID,
		SpawnPolicy:    SpawnDecompose,
		PlanID:         "plan_1",
		VerdictID:      "v_1",
		ObservationIDs: []string{"obs_1"},
		PlanKind:       plan.ExplorationPlan,
		VerdictKind:    types.VerdictPartial,
	}
	if err := ApplySpawnPolicy("s1", goal, round, tm); err != nil {
		t.Fatalf("ApplySpawnPolicy: %v", err)
	}
	children := tm.Tree().ListChildren("s1", goal.ID)
	if len(children) != 2 {
		t.Fatalf("children = %d, want 2", len(children))
	}
}

func TestGetPipelineFocus_InlineRetry(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	_ = tm.Tree().UpdateStatus("s1", goal.ID, TaskStatusInProgress)
	goal.LastRound = &WorkItemPipelineRound{SpawnPolicy: SpawnInline}
	tm.Tree().ApplyPipelineRound("s1", goal.ID, goal.LastRound, RoundPhaseIdle)

	focus, err := tm.Tree().GetPipelineFocus("s1")
	if err != nil {
		t.Fatalf("GetPipelineFocus: %v", err)
	}
	if focus == nil || focus.ID != goal.ID {
		t.Fatalf("focus = %v, want goal %s", focus, goal.ID)
	}
}

func TestApplySpawnEscalateHuman_CreatesReviewChild(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "risky change")
	round := &WorkItemPipelineRound{
		WorkItemID:     goal.ID,
		SpawnPolicy:    SpawnEscalateHuman,
		PlanID:         "plan_1",
		VerdictID:      "v_1",
		ObservationIDs: []string{"obs_1"},
		SpawnRationale: "R2: daily decompose limit exceeded",
	}
	if err := ApplySpawnPolicy("s1", goal, round, tm); err != nil {
		t.Fatalf("ApplySpawnPolicy: %v", err)
	}
	children := tm.Tree().ListChildren("s1", goal.ID)
	if len(children) != 1 {
		t.Fatalf("children = %d, want 1 review item", len(children))
	}
	if children[0].Kind != WorkKindVerify {
		t.Fatalf("review kind = %q, want verify", children[0].Kind)
	}
	got, _ := tm.GetWorkItem("s1", goal.ID)
	if got.RoundPhase != RoundPhaseAwaitChild {
		t.Fatalf("parent phase = %q, want await_child", got.RoundPhase)
	}
}

func TestDefaultTreeEvalContext_AdaptiveThreshold(t *testing.T) {
	tm := NewTaskManager()
	tm.SetAdaptiveThreshold(&AdaptiveThreshold{
		GlobalDefault: 0.75,
		PerUser:       map[string]float64{"user_a": 0.55},
	})
	ctx := DefaultTreeEvalContext("s1", "wi_1", "", tm)
	if ctx.Threshold != 0.75 {
		t.Fatalf("threshold = %.2f, want 0.75", ctx.Threshold)
	}
	ctxUser := DefaultTreeEvalContext("s1", "wi_1", "user_a", tm)
	if ctxUser.Threshold != 0.55 {
		t.Fatalf("per-user threshold = %.2f, want 0.55", ctxUser.Threshold)
	}
}
