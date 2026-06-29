package sessionorchestrator

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

func TestPlanQuantizedKind_GoalForcesOrchestrateDespiteLoopFirst(t *testing.T) {
	item := &workmodel.WorkItem{Kind: workmodel.WorkKindGoal}
	report := orchtypes.UncertaintyReport{
		QuantizedIntent: &orchtypes.QuantizedIntent{
			Kind: orchtypes.IntentFast, // loop_first default ingress
		},
	}
	if qKind := planQuantizedKind(item, report); qKind != "intent_orchestrate" {
		t.Fatalf("goal qKind = %q, want intent_orchestrate", qKind)
	}
	kind := plan.MatchKind("intent_orchestrate", 1, 0)
	if kind != plan.ExplorationPlan {
		t.Fatalf("MatchKind = %v, want ExplorationPlan", kind)
	}
}

func TestPlanQuantizedKind_GoalRollupUsesCommandIntent(t *testing.T) {
	item := &workmodel.WorkItem{Kind: workmodel.WorkKindGoal, NeedsRollup: true}
	report := orchtypes.UncertaintyReport{
		QuantizedIntent: &orchtypes.QuantizedIntent{
			Kind: orchtypes.IntentFast,
		},
	}
	if got := planQuantizedKind(item, report); got != "intent_command" {
		t.Fatalf("rollup qKind = %q, want intent_command", got)
	}
}

func TestPlanQuantizedKind_ExploreUsesOrchestrate(t *testing.T) {
	item := &workmodel.WorkItem{Kind: workmodel.WorkKindExplore}
	report := orchtypes.UncertaintyReport{
		QuantizedIntent: &orchtypes.QuantizedIntent{Kind: orchtypes.IntentFast},
	}
	if got := planQuantizedKind(item, report); got != "intent_orchestrate" {
		t.Fatalf("explore qKind = %q, want intent_orchestrate", got)
	}
}

func TestEnsureGoal_SeedsExplorationUncertainty(t *testing.T) {
	tm := workmodel.NewTaskManager()
	goal, err := tm.EnsureGoal("s1", "investigate module X")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	if goal.Uncertainty < workmodel.DefaultUncertaintyDecomposeThreshold {
		t.Fatalf("goal uncertainty = %v, want >= %v", goal.Uncertainty, workmodel.DefaultUncertaintyDecomposeThreshold)
	}
}
