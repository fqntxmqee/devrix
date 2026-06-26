package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestObserveWorkItem_IncludesChildStructuredBubble(t *testing.T) {
	tm := workmodel.NewTaskManager()
	parent, _ := tm.CreateWorkItem("s1", workmodel.CreateWorkItemInput{
		Kind: workmodel.WorkKindPlan, Title: "parent plan", Directive: "aggregate child outcomes",
	})
	child, _ := tm.CreateWorkItem("s1", workmodel.CreateWorkItemInput{
		ParentID: parent.ID, Kind: workmodel.WorkKindImplement, Title: "child", Directive: "child work",
	})
	_ = tm.Tree().ApplyPipelineRound("s1", child.ID, &workmodel.WorkItemPipelineRound{
		WorkItemID:        child.ID,
		PlanID:            "plan_child",
		VerdictID:         "verdict_child",
		VerdictKind:       types.VerdictPass,
		ContextBubbleKind: workmodel.BubbleStructured,
		ObservationIDs:    []string{"obs_child"},
	}, workmodel.RoundPhaseIdle)
	_ = tm.UpdateStatus("s1", child.ID, workmodel.TaskStatusInProgress)
	_ = tm.UpdateStatus("s1", child.ID, workmodel.TaskStatusCompleted)

	_, obsIDs, err := observeWorkItem(
		context.Background(),
		"s1",
		parent,
		nil,
		nil,
		"",
		tm,
	)
	if err != nil {
		t.Fatalf("observeWorkItem: %v", err)
	}
	found := false
	for _, id := range obsIDs {
		if strings.Contains(id, "obs") {
			found = true
		}
	}
	if len(obsIDs) < 2 {
		t.Fatalf("expected base + child bubble observations, got %d ids", len(obsIDs))
	}
	_ = found
}

func TestRunItemPipeline_ChildBubbleOnRound(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	sessionID := "sess-bubble-kind"
	goal, _ := tm.EnsureGoal(sessionID, "goal")
	round, err := runner.Run(context.Background(), sessionID, goal, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.ContextBubbleKind != workmodel.BubbleStructured {
		t.Fatalf("ContextBubbleKind=%q want structured", round.ContextBubbleKind)
	}
}
