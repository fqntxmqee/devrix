package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestShouldRollupAfterChildren(t *testing.T) {
	parent := &WorkItem{
		ID: "p1",
		LastRound: &WorkItemPipelineRound{
			SpawnPolicy: SpawnDecompose,
		},
	}
	stats := ChildOutcomeStats{Total: 2, Completed: 1, Failed: 1}
	if !ShouldRollupAfterChildren(parent, RollupGateBestEffort, stats) {
		t.Fatal("expected rollup when all non-checklist children terminal")
	}
	stats.Running = 1
	if ShouldRollupAfterChildren(parent, RollupGateBestEffort, stats) {
		t.Fatal("expected no rollup while child running")
	}
}

func TestShouldRollupAfterChildren_AllPassBlocksOnFail(t *testing.T) {
	parent := &WorkItem{
		ID: "p1",
		LastRound: &WorkItemPipelineRound{SpawnPolicy: SpawnDecompose},
	}
	stats := ChildOutcomeStats{Total: 2, Completed: 1, Failed: 1}
	if ShouldRollupAfterChildren(parent, RollupGateAllPass, stats) {
		t.Fatal("all_pass should not rollup when any child failed")
	}
	stats = ChildOutcomeStats{Total: 2, Completed: 2}
	if !ShouldRollupAfterChildren(parent, RollupGateAllPass, stats) {
		t.Fatal("all_pass should rollup when all children completed")
	}
}

func TestRollupGatePolicyFor_Phase1BestEffort(t *testing.T) {
	if got := RollupGatePolicyFor(&WorkItem{ID: "p1"}); got != RollupGateBestEffort {
		t.Fatalf("policy=%q, want best_effort", got)
	}
}

func TestReopenForRollup(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "review d2")
	_ = tm.Tree().ApplyPipelineRound("s1", goal.ID, &WorkItemPipelineRound{
		SpawnPolicy: SpawnNone,
		VerdictKind: types.VerdictFail,
	}, RoundPhaseIdle)
	_ = tm.Tree().UpdateStatus("s1", goal.ID, TaskStatusInProgress)
	_ = tm.Tree().UpdateStatus("s1", goal.ID, TaskStatusFailed)
	if err := tm.Tree().SetNeedsRollup("s1", goal.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := tm.Tree().ReopenForRollup("s1", goal.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := tm.GetWorkItem("s1", goal.ID)
	if got.Status != TaskStatusPending {
		t.Fatalf("status = %s, want pending", got.Status)
	}
	if got.Locked {
		t.Fatal("expected unlocked for rollup")
	}
}

func TestGetReadyItems_SkipsEphemeralChecklist(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "task")
	_ = tm.Tree().UpsertChecklist("s1", goal.ID, []ChecklistEntry{
		{Content: "step 1", Status: TaskStatusPending},
	})
	ready := tm.Tree().GetReadyItems("s1")
	for _, item := range ready {
		if item.Kind == WorkKindChecklist && item.Ephemeral {
			t.Fatalf("ephemeral checklist %s should not be ready", item.ID)
		}
	}
}

func TestGetReadyItems_NeedsRollupInProgress(t *testing.T) {
	tm := NewTaskManager()
	parent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindPlan, Title: "parent"})
	_ = tm.Tree().UpdateStatus("s1", parent.ID, TaskStatusInProgress)
	if err := tm.Tree().SetNeedsRollup("s1", parent.ID, true); err != nil {
		t.Fatal(err)
	}
	ready := tm.Tree().GetReadyItems("s1")
	if len(ready) != 1 || ready[0].ID != parent.ID {
		t.Fatalf("ready=%v, want parent with NeedsRollup in_progress", ready)
	}
}

func TestMaybeRootRollupFallback(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "review d2")
	_ = tm.Tree().ApplyPipelineRound("s1", goal.ID, &WorkItemPipelineRound{
		SpawnPolicy: SpawnNone,
		VerdictKind: types.VerdictFail,
	}, RoundPhaseIdle)
	_ = tm.Tree().UpsertChecklist("s1", goal.ID, []ChecklistEntry{
		{Content: "review prepare/", Status: TaskStatusPending},
	})
	_ = tm.Tree().UpdateStatus("s1", goal.ID, TaskStatusInProgress)
	_ = tm.Tree().UpdateStatus("s1", goal.ID, TaskStatusFailed)
	wi, ok := MaybeRootRollupFallback("s1", tm)
	if !ok || wi == nil {
		t.Fatal("expected root rollup fallback")
	}
	if !wi.NeedsRollup {
		t.Fatal("expected NeedsRollup")
	}
	if wi.Status != TaskStatusPending {
		t.Fatalf("status = %s, want pending after reopen", wi.Status)
	}
}
