package workmodel

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestDefaultDecomposeProposer_NoHypothesisLabels(t *testing.T) {
	item := &WorkItem{
		Kind:      WorkKindGoal,
		Directive: "review d2 kernel code",
	}
	round := &WorkItemPipelineRound{PlanKind: plan.ExplorationPlan}
	specs := DefaultDecomposeProposer(item, round)
	if len(specs) < 2 {
		t.Fatalf("specs = %d, want >= 2", len(specs))
	}
	for _, s := range specs {
		if strings.Contains(strings.ToLower(s.Directive), "hypothesis") {
			t.Fatalf("directive must not use hypothesis labels: %q", s.Directive)
		}
		if s.ExpectedReturn == "" {
			t.Fatal("expected_return required")
		}
	}
}

func TestDefaultDecomposeProposer_SplitsScopePaths(t *testing.T) {
	item := &WorkItem{
		Kind:      WorkKindGoal,
		Directive: "review kernel",
		ScopeContract: &ScopeContract{
			InScope: []string{
				"internal/layers/contextengine/kernel/contracts.go",
				"internal/layers/contextengine/kernel/spans.go",
				"internal/layers/contextengine/kernel/observer_test.go",
			},
		},
	}
	round := &WorkItemPipelineRound{PlanKind: plan.ExplorationPlan}
	specs := DefaultDecomposeProposer(item, round)
	if len(specs) != 2 {
		t.Fatalf("specs = %d, want 2 scope slices", len(specs))
	}
	if len(specs[0].ScopeIn) == 0 {
		t.Fatal("expected scope_in on child spec")
	}
}

func TestHasOpenWork_PendingRollupOnGoal(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	_ = tm.Tree().UpdateStatus("s1", goal.ID, TaskStatusCompleted)
	_ = tm.Tree().SetNeedsRollup("s1", goal.ID, true)
	if !tm.Tree().HasOpenWork("s1") {
		t.Fatal("expected open work while root needs_rollup")
	}
}

func TestSpawnPolicy_RollupFailInlines(t *testing.T) {
	round := &WorkItemPipelineRound{
		VerdictKind: types.VerdictFail,
		PlanKind:    plan.CommitmentPlan,
	}
	ctx := TreeEvalContext{RollupRound: true}
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnInline {
		t.Fatalf("got %q, want inline", got)
	}
}
