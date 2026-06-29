package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestScopeContract_HasOpenQuestions(t *testing.T) {
	sc := &ScopeContract{OpenQuestions: []string{"  ", "need API shape?"}}
	if !sc.HasOpenQuestions() {
		t.Fatal("expected open questions")
	}
}

func TestApplyScopeContractSpawnGate_BlocksDecompose(t *testing.T) {
	item := &WorkItem{Kind: WorkKindGoal, ScopeContract: &ScopeContract{OpenQuestions: []string{"?"}}}
	round := &WorkItemPipelineRound{
		PlanKind:    plan.ExplorationPlan,
		VerdictKind: types.VerdictPartial,
		SpawnPolicy: SpawnDecompose,
	}
	ApplyScopeContractSpawnGate(item, round)
	if round.SpawnPolicy != SpawnInline {
		t.Fatalf("got %q, want inline", round.SpawnPolicy)
	}
}

func TestDecomposeChildren_RequiresExpectedReturn(t *testing.T) {
	ResetDecomposeLimits()
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	_, err := tm.DecomposeChildren("s1", goal.ID, []ChildSpec{{Title: "c", Directive: "d"}})
	if err == nil {
		t.Fatal("expected error for missing expected_return")
	}
}

func TestInferScopeInFromDirective_SingleFile(t *testing.T) {
	got := InferScopeInFromDirective("fix bug in internal/layers/foo/bar.go handler")
	if len(got) == 0 {
		t.Fatal("expected scope in paths")
	}
	found := false
	for _, p := range got {
		if p == "internal/layers/foo/bar.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("paths = %v", got)
	}
}
