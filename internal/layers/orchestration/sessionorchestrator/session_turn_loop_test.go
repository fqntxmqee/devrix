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

// T: D7-SX-AXX-TXX (DM-20260705-008 M3) — uses commitmentOnlyPlanner
// to isolate from M3 行为增量. See item_pipeline_test.go for rationale.
func TestRunSessionTurnLoop_SingleGoal_Completes(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	runner.Planner = commitmentOnlyPlanner{} // isolate from M3 行为增量
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
		Message:   "implement cache layer feature",
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
	if goal.Directive != "implement cache layer feature" {
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

// T: D7-SX-AXX-TXX (DM-20260705-008 M3) — uses decomposeAwarePlanner
// to isolate from M3 行为增量. M3 changes:
//   - CommitmentPlan + VerdictPartial → SpawnNone (terminal, no decompose on high U)
//   - ExplorationPlan + VerdictPass   → SpawnDecompose (no complete on pass)
// For this test, the parent must be decomposable (Partial + high U) and the
// children must be completable (Pass). Use ExplorationPlan for the first
// call (parent) and CommitmentPlan for subsequent calls (children).
func TestRunSessionTurnLoop_DecomposeRecursive_CompletesChildren(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	runner.Planner = &decomposeAwarePlanner{} // context-aware: explore→commit
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

// T: D7-S2-A86-T03 (DM-20260703-001 L5-D7-CC-07) — session loop delegates to pipeline retry.
func TestRunSessionTurnLoop_RetriesWhenDeliverableIncomplete(t *testing.T) {
	exec := &multiRoundReviewExecutor{}
	runner, tm, _ := newItemPipelineTestRunner(t)
	runner.Executor = exec
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&recordingExecutor{},
		WithTaskManager(tm),
		WithItemPipelineRunner(runner),
		WithLearner(runner.Learner),
	)

	sessionID := "sess-incomplete-retry"
	directive := directiveWithDeliverableSchema("review d2 domain kernel directory code")
	goal, _ := tm.EnsureGoal(sessionID, directive)
	ph, _ := tm.CreateWorkItem(sessionID, workmodel.CreateWorkItemInput{
		ParentID: goal.ID, Kind: workmodel.WorkKindImplement, Title: "done",
	})
	_ = tm.Tree().UpdateStatus(sessionID, ph.ID, workmodel.TaskStatusInProgress)
	_ = tm.Tree().UpdateStatus(sessionID, ph.ID, workmodel.TaskStatusCompleted)

	ch, err := orch.RunSessionTurnLoop(context.Background(), orchtypes.ProcessRequest{
		SessionID: sessionID,
		Message:   directive,
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate})
	if err != nil {
		t.Fatalf("RunSessionTurnLoop: %v", err)
	}
	events := drainEvents(ch)
	if exec.calls < 2 {
		// Pipeline-level retry is covered by TestRunItemPipeline_IncompleteDeliverable_InlineRetry.
		goal, _ = tm.GetWorkItem(sessionID, goal.ID)
		t.Fatalf("executor calls = %d, want >= 2; last spawn=%q status=%q deliverable=%s",
			exec.calls, goal.LastRound.SpawnPolicy, goal.Status, goal.LastRound.DeliverableStatus)
	}
	pipelineRounds := 0
	var complete *contracts.EngineEvent
	for _, ev := range events {
		if ev == nil {
			continue
		}
		if ev.Type == "pipeline_round" {
			pipelineRounds++
		}
		if ev.Type == "complete" {
			complete = ev
		}
	}
	if pipelineRounds < 2 {
		t.Fatalf("pipeline_round events = %d, want >= 2", pipelineRounds)
	}
	if complete == nil {
		t.Fatal("missing complete event")
	}
	if complete.Metadata["task_incomplete"] == "true" {
		t.Fatalf("complete flagged task_incomplete after successful retry: %q", complete.Content)
	}
	goal, _ = tm.GetWorkItem(sessionID, mustGoalID(t, tm, sessionID))
	if goal.Status != workmodel.TaskStatusCompleted {
		t.Fatalf("goal status = %q, want completed after retry", goal.Status)
	}
	if !strings.Contains(complete.Content, "kernel/foo.go:42") {
		t.Fatalf("complete should carry deliverable from retry round, got %q", complete.Content)
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
