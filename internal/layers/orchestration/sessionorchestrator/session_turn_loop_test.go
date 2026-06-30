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

// Regression (2026-06-26): when RunSessionTurnLoop is invoked with a fresh
// session (no prior WorkItem tree) and the user message as the only input,
// the loop used to break on GetPipelineFocus=nil and emit a 50-byte stub.
// RunSessionTurnLoop now seeds an EnsureGoal from req.Message so a single
// intent_orchestrate request lands on a real WorkItem.
func TestRunSessionTurnLoop_FreshSession_SeedsGoalFromMessage(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&recordingExecutor{},
		WithTaskManager(tm),
		WithItemPipelineRunner(runner),
		WithLearner(runner.Learner),
	)

	sessionID := "sess-turn-loop-fresh"
	if focus, _ := tm.Tree().GetPipelineFocus(sessionID); focus != nil {
		t.Fatalf("precondition: expected no focus, got %+v", focus)
	}

	ch, err := orch.RunSessionTurnLoop(context.Background(), orchtypes.ProcessRequest{
		SessionID: sessionID,
		Message:   "review d2 domain code",
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate})
	if err != nil {
		t.Fatalf("RunSessionTurnLoop: %v", err)
	}
	events := drainEvents(ch)
	if !hasEventType(events, "pipeline_round") {
		t.Fatalf("expected pipeline_round, got %v", loopEventTypes(events))
	}
	if !hasEventType(events, "complete") {
		t.Fatalf("expected complete, got %v", loopEventTypes(events))
	}
	for _, ev := range events {
		if ev.Type == "text" && ev.Content == "session turn loop: no work items processed" {
			t.Fatalf("regression: emitted empty-tree stub; events=%v", loopEventTypes(events))
		}
	}
	goal, ok := tm.GetWorkItem(sessionID, mustGoalID(t, tm, sessionID))
	if !ok || goal == nil {
		t.Fatalf("expected goal WorkItem to be seeded from req.Message")
	}
	if goal.Directive != "review d2 domain code" {
		t.Fatalf("goal.Directive = %q, want seeded from req.Message", goal.Directive)
	}
}

// Regression (2026-06-26): RunSessionTurnLoop used to emit a `text` event
// carrying the D7 internal pipeline summary ([Goal] title → VerdictKind
// (spawn=...)) at loop end, which the feishu reply card treated as
// user-facing content. The LLM's streaming path already delivers the
// real answer; the D7 metadata must stay internal.
func TestRunSessionTurnLoop_NoSummaryTextEventAtLoopEnd(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&recordingExecutor{},
		WithTaskManager(tm),
		WithItemPipelineRunner(runner),
		WithLearner(runner.Learner),
	)

	sessionID := "sess-no-summary"
	_, _ = tm.EnsureGoal(sessionID, "draft release notes")
	_ = tm.Tree().SetUncertainty(sessionID, mustGoalID(t, tm, sessionID), 0.2)

	ch, err := orch.RunSessionTurnLoop(context.Background(), orchtypes.ProcessRequest{
		SessionID: sessionID,
		Message:   "draft release notes",
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate})
	if err != nil {
		t.Fatalf("RunSessionTurnLoop: %v", err)
	}
	events := drainEvents(ch)
	for _, ev := range events {
		if ev.Type == "text" && strings.Contains(ev.Content, "draft release notes →") {
			t.Fatalf("regression: pipeline summary leaked as text event: %q", ev.Content)
		}
	}
	last := events[len(events)-1]
	if last.Type != "complete" {
		t.Fatalf("last event type = %q, want complete", last.Type)
	}
	// DM-20260627-001: complete carries root deliverable (reverses empty-Content hotfix).
	if last.Content == "" {
		t.Fatalf("complete event Content empty, want session deliverable summary")
	}
}

func TestRunSessionTurnLoop_DecomposeRecursive_CompletesChildren(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	runner.Executor = &rollupContentExecutor{summary: validRollupSummary(), capture: nil}

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
	goalID := goal.ID

	runner.Verify = func(art *wavescheduler.Artifact) workmodel.Verdict {
		// Partial on root triggers decompose; children must pass to reach rollup.
		if art != nil && art.TaskID != goalID {
			return workmodel.Verdict{
				Kind: types.VerdictPass, SourceID: art.TaskID, Confidence: 0.9,
			}
		}
		return workmodel.Verdict{
			Kind: types.VerdictPartial, SourceID: "v_partial", Confidence: 0.4,
			Reason: "explore more",
		}
	}

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
	if len(children) < 1 {
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

func TestProcessMessage_WorkItemPipelineTurnLoop(t *testing.T) {
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
