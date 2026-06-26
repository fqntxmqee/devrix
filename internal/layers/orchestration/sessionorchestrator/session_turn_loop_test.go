package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestRunSessionTurnLoop_SingleGoal_Completes(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&recordingExecutor{},
		WithTaskManager(tm),
		WithItemPipelineRunner(runner),
		WithLearner(runner.Learner),
	)

	sessionID := "sess-turn-loop"
	_, _ = tm.EnsureGoal(sessionID, "implement feature X")
	_ = tm.Tree().SetUncertainty(sessionID, mustGoalID(t, tm, sessionID), 0.2)

	ch, err := orch.RunSessionTurnLoop(context.Background(), orchtypes.ProcessRequest{
		SessionID: sessionID,
		Message:   "implement feature X",
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate})
	if err != nil {
		t.Fatalf("RunSessionTurnLoop: %v", err)
	}
	events := drainEvents(ch)
	if !hasEventType(events, "complete") {
		t.Fatalf("expected complete, got %v", loopEventTypes(events))
	}
	if !hasEventType(events, "pipeline_round") {
		t.Fatal("expected pipeline_round event")
	}
	goal, _ := tm.GetWorkItem(sessionID, mustGoalID(t, tm, sessionID))
	if goal.Status != workmodel.TaskStatusCompleted {
		t.Fatalf("goal status = %q, want completed", goal.Status)
	}
}

func TestRunSessionTurnLoop_DecomposeRecursive_CompletesChildren(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	runner.Verify = func(_ *wavescheduler.Artifact) workmodel.Verdict {
		return workmodel.Verdict{
			Kind: types.VerdictPartial, SourceID: "v_partial", Confidence: 0.4,
			Reason: "explore more",
		}
	}

	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&recordingExecutor{},
		WithTaskManager(tm),
		WithItemPipelineRunner(runner),
		WithLearner(runner.Learner),
	)

	sessionID := "sess-recursive"
	goal, _ := tm.EnsureGoal(sessionID, "compare cache strategies")
	_ = tm.Tree().SetUncertainty(sessionID, goal.ID, 0.9)

	ch, err := orch.RunSessionTurnLoop(context.Background(), orchtypes.ProcessRequest{
		SessionID: sessionID,
		Message:   "compare cache strategies",
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate})
	if err != nil {
		t.Fatalf("RunSessionTurnLoop: %v", err)
	}
	events := drainEvents(ch)
	if !hasEventType(events, "complete") {
		t.Fatalf("expected complete, got %v", loopEventTypes(events))
	}

	children := tm.Tree().ListChildren(sessionID, goal.ID)
	if len(children) < 2 {
		t.Fatalf("expected decomposed children, got %d", len(children))
	}
	for _, c := range children {
		if c.Status != workmodel.TaskStatusCompleted {
			t.Fatalf("child %s status = %q, want completed", c.ID, c.Status)
		}
	}
	goal, _ = tm.GetWorkItem(sessionID, goal.ID)
	if goal.Status != workmodel.TaskStatusCompleted {
		t.Fatalf("parent status = %q, want completed after children", goal.Status)
	}
}

func TestProcessMessage_WorkItemPipelineFeatureFlag(t *testing.T) {
	t.Setenv(workmodel.FeatureWorkItemPipelineEnv, "1")

	runner, tm, _ := newItemPipelineTestRunner(t)
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&recordingExecutor{},
		WithTaskManager(tm),
		WithItemPipelineRunner(runner),
		WithLearner(runner.Learner),
	)

	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-pm-flag",
		Message:   "build login module",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	events := drainEvents(ch)
	if !hasEventType(events, "complete") {
		t.Fatalf("expected complete via turn loop, got %v", loopEventTypes(events))
	}
}

func mustGoalID(t *testing.T, tm *workmodel.TaskManager, sessionID string) string {
	t.Helper()
	for _, item := range tm.Tree().List(sessionID) {
		if item != nil && item.Kind == workmodel.WorkKindGoal && item.ParentID == "" {
			return item.ID
		}
	}
	t.Fatal("goal not found")
	return ""
}

func drainEvents(ch <-chan *contracts.EngineEvent) []*contracts.EngineEvent {
	var out []*contracts.EngineEvent
	for ev := range ch {
		out = append(out, ev)
	}
	return out
}

func hasEventType(events []*contracts.EngineEvent, typ string) bool {
	for _, ev := range events {
		if ev != nil && ev.Type == typ {
			return true
		}
	}
	return false
}

func loopEventTypes(events []*contracts.EngineEvent) []string {
	var types []string
	for _, ev := range events {
		if ev != nil {
			types = append(types, ev.Type)
		}
	}
	return types
}
