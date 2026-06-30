package workmodel

import (
	"os"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// Regression: decompose fallback must stay structural — no tactical NL in Go source.
func TestDefaultDecomposeProposer_NoTacticalHardcoding(t *testing.T) {
	raw, err := os.ReadFile("decompose_proposer.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	src := string(raw)
	for _, forbidden := range []string{
		"聚焦",
		"只 read_file",
		"禁止探索",
		"P0/P1 清单",
		"contracts and API surface",
		"implementation and observability",
		"hypothesis",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("decompose_proposer.go contains forbidden tactical string %q", forbidden)
		}
	}
}

func TestDefaultDecomposeProposer_NoHypothesisLabels(t *testing.T) {
	item := &WorkItem{
		Kind:      WorkKindGoal,
		Directive: "review d2 kernel code",
	}
	round := &WorkItemPipelineRound{PlanKind: plan.ExplorationPlan}
	specs := DefaultDecomposeProposer(item, round)
	if len(specs) != 1 {
		t.Fatalf("specs = %d, want 1 pass-through without scope paths", len(specs))
	}
	if specs[0].Directive != item.Directive {
		t.Fatalf("directive = %q, want pass-through %q", specs[0].Directive, item.Directive)
	}
	if specs[0].ExpectedReturn == "" {
		t.Fatal("expected_return required")
	}
	if !strings.Contains(specs[0].ExpectedReturn, "deliverable_schema") {
		t.Fatalf("expected machine schema tag, got %q", specs[0].ExpectedReturn)
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
	for _, s := range specs {
		if len(s.ScopeIn) == 0 {
			t.Fatal("expected scope_in on child spec")
		}
		if s.Directive != item.Directive {
			t.Fatalf("directive mutated tactically: %q", s.Directive)
		}
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
