package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestBuildSessionCompleteEvent_should_prefer_rollup_deliverable(t *testing.T) {
	tm := workmodel.NewTaskManager()
	sessionID := "sess-complete-gate"
	goal, _ := tm.EnsureGoal(sessionID, "review kernel")
	child, err := tm.DecomposeChildren(sessionID, goal.ID, []workmodel.ChildSpec{{
		Title: "slice", Directive: "review slice", ExpectedReturn: "P0/P1 file:line",
	}})
	if err != nil || len(child) == 0 {
		t.Fatalf("decompose: %v", err)
	}
	rollupSummary := validRollupSummary()
	_ = tm.Tree().ApplyPipelineRound(sessionID, goal.ID, &workmodel.WorkItemPipelineRound{
		ArtifactSummary: rollupSummary,
		VerdictKind:     types.VerdictPass,
	}, workmodel.RoundPhaseIdle)

	ev := buildSessionCompleteEvent(context.Background(), sessionID, tm, "Let me continue exploring...")
	if ev == nil {
		t.Fatal("nil event")
	}
	if !strings.Contains(ev.Content, "Executive summary") {
		t.Fatalf("complete content should prefer rollup deliverable, got %q", ev.Content)
	}
	if ev.Metadata["summary_quality"] == "" {
		t.Fatal("expected summary_quality metadata")
	}
	if ev.Metadata["final_quality"] == "" {
		t.Fatal("expected final_quality metadata")
	}
}

func TestBuildSessionCompleteEvent_should_emit_task_incomplete_when_both_bad(t *testing.T) {
	tm := workmodel.NewTaskManager()
	sessionID := "sess-both-bad"
	goal, _ := tm.EnsureGoal(sessionID, "review")
	_ = tm.Tree().ApplyPipelineRound(sessionID, goal.ID, &workmodel.WorkItemPipelineRound{
		ArtifactSummary: "Let me continue exploring.",
	}, workmodel.RoundPhaseIdle)

	ev := buildSessionCompleteEvent(context.Background(), sessionID, tm, "Let me continue exploring.")
	if ev.Content != taskIncompleteUserMessage {
		t.Fatalf("content = %q, want task incomplete message", ev.Content)
	}
	if ev.Metadata["task_incomplete"] != "true" {
		t.Fatal("expected task_incomplete meta")
	}
}

func TestRunSessionTurnLoop_CompletePrefersRollupDeliverable(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	runner.Executor = &rollupContentExecutor{summary: validRollupSummary(), capture: nil}
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&recordingExecutor{},
		WithTaskManager(tm),
		WithItemPipelineRunner(runner),
		WithLearner(runner.Learner),
	)

	sessionID := "sess-rollup-complete"
	goal, _ := tm.EnsureGoal(sessionID, "compare cache strategies")
	_ = tm.Tree().SetUncertainty(sessionID, goal.ID, 0.9)
	goalID := goal.ID

	runner.Verify = func(art *wavescheduler.Artifact) workmodel.Verdict {
		if art != nil && art.TaskID != goalID {
			return workmodel.Verdict{Kind: types.VerdictPass, SourceID: art.TaskID, Confidence: 0.9}
		}
		return workmodel.Verdict{Kind: types.VerdictPartial, SourceID: "v_partial", Confidence: 0.4}
	}

	ch, err := orch.RunSessionTurnLoop(context.Background(), orchtypes.ProcessRequest{
		SessionID: sessionID,
		Message:   "compare cache strategies",
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate})
	if err != nil {
		t.Fatalf("RunSessionTurnLoop: %v", err)
	}
	events := drainEvents(ch)
	var complete *contracts.EngineEvent
	for _, ev := range events {
		if ev != nil && ev.Type == "complete" {
			complete = ev
		}
	}
	if complete == nil {
		t.Fatal("missing complete event")
	}
	if strings.Contains(strings.ToLower(complete.Content), "let me continue") {
		t.Fatalf("complete leaked transition text: %q", complete.Content)
	}
	if complete.Metadata["summary_quality"] == "" {
		t.Fatal("expected summary_quality on complete")
	}
}
