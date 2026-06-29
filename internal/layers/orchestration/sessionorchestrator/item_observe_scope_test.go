package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

func TestMapScopeContractToObservations_OpenQuestions(t *testing.T) {
	item := &workmodel.WorkItem{
		ID:   "goal_1",
		Kind: workmodel.WorkKindGoal,
		ScopeContract: &workmodel.ScopeContract{
			OpenQuestions: []string{"Which API version?"},
		},
	}
	obs, err := mapScopeContractToObservations("s1", item)
	if err != nil {
		t.Fatalf("mapScopeContractToObservations: %v", err)
	}
	if len(obs) != 1 || obs[0].Kind != orchtypes.ObsUncertainty {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestObserveWorkItem_ScopeContractObserve(t *testing.T) {
	tm := workmodel.NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "build auth")
	goal.ScopeContract = &workmodel.ScopeContract{
		OpenQuestions: []string{"OAuth or JWT?"},
	}
	_, obsIDs, err := observeWorkItem(context.Background(), "s1", goal, nil, nil, "", tm, nil)
	if err != nil {
		t.Fatalf("observeWorkItem: %v", err)
	}
	if len(obsIDs) < 2 {
		t.Fatalf("expected base + scope observations, got %d", len(obsIDs))
	}
}
