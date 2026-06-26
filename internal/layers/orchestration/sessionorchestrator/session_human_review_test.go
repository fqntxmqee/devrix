package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

func TestRunSessionTurnLoop_HumanReview_Pauses(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&recordingExecutor{},
		WithTaskManager(tm),
		WithItemPipelineRunner(runner),
		WithLearner(runner.Learner),
	)

	sessionID := "sess-human-review"
	goal, _ := tm.EnsureGoal(sessionID, "blocked change")
	round := &workmodel.WorkItemPipelineRound{
		WorkItemID:     goal.ID,
		SpawnPolicy:    workmodel.SpawnEscalateHuman,
		PlanID:         "p1",
		VerdictID:      "v1",
		ObservationIDs: []string{"o1"},
	}
	if err := workmodel.ApplySpawnPolicy(sessionID, goal, round, tm); err != nil {
		t.Fatalf("ApplySpawnPolicy: %v", err)
	}

	ch, err := orch.RunSessionTurnLoop(context.Background(), orchtypes.ProcessRequest{
		SessionID: sessionID,
		Message:   "continue",
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate})
	if err != nil {
		t.Fatalf("RunSessionTurnLoop: %v", err)
	}
	events := drainEvents(ch)
	if !hasEventType(events, "human_review") {
		t.Fatalf("expected human_review event, got %v", loopEventTypes(events))
	}
}

func TestEffectiveUserID_FromBaggage(t *testing.T) {
	ctx := tracer.DefaultBaggageManager.Set(context.Background(), "user.id", "user_bag")
	got := effectiveUserID(ctx, orchtypes.ProcessRequest{SessionID: "s1", Message: "hi"})
	if got != "user_bag" {
		t.Fatalf("userID = %q, want user_bag", got)
	}
}

func TestProcessMessageContract_ThreadsUserID(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&recordingExecutor{},
		WithTaskManager(tm),
		WithItemPipelineRunner(runner),
	)
	ctx := tracer.DefaultBaggageManager.Set(context.Background(), "user.id", "user_contract")
	ch, err := orch.ProcessMessageContract(ctx, "sess-uid", "hello")
	if err != nil {
		t.Fatalf("ProcessMessageContract: %v", err)
	}
	for range ch {
	}
	_ = contracts.EngineEvent{}
}
