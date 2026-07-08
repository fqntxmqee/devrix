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

	ev := buildSessionCompleteEvent(context.Background(), sessionID, tm, "Let me continue exploring...", "")
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

	ev := buildSessionCompleteEvent(context.Background(), sessionID, tm, "Let me continue exploring.", "")
	if ev.Content != taskIncompleteUserMessage {
		t.Fatalf("content = %q, want task incomplete message", ev.Content)
	}
	if ev.Metadata["task_incomplete"] != "true" {
		t.Fatal("expected task_incomplete meta")
	}
}

func TestBuildSessionCompleteEvent_should_emit_task_incomplete_when_open_incomplete_deliverable(t *testing.T) {
	tm := workmodel.NewTaskManager()
	sessionID := "sess-open-incomplete"
	goal, _ := tm.EnsureGoal(sessionID, "review kernel")
	_ = tm.Tree().UpdateStatus(sessionID, goal.ID, workmodel.TaskStatusInProgress)
	_ = tm.Tree().ApplyPipelineRound(sessionID, goal.ID, &workmodel.WorkItemPipelineRound{
		SpawnPolicy:       workmodel.SpawnInline,
		DeliverableSchema: workmodel.FirstRegisteredDeliverableSchema(),
		DeliverableStatus: workmodel.DeliverableStatusIncomplete,
		ArtifactSummary:   "P0: issue in internal/foo.go:1",
	}, workmodel.RoundPhaseIdle)

	ev := buildSessionCompleteEvent(context.Background(), sessionID, tm, "P0: issue in internal/foo.go:1", "")
	if ev.Metadata["task_incomplete"] != "true" {
		t.Fatalf("expected task_incomplete when open WI owes deliverable, meta=%v", ev.Metadata)
	}
}

// DM-20260708-002 (devrix hotfix for "2×3=6" → "❌ 任务未完成" screenshot):
// the observational_answer fast-path produces a short CatBusiness ObsFact
// answer (e.g. "2×3=6", "巴黎是法国首都") that legitimately falls under
// the 100-rune too_short threshold. buildSessionCompleteEvent must NOT
// replace it with taskIncompleteUserMessage — the answer is structurally
// pre-validated by pickHighStrengthBusinessFact (strength ≥ 0.9, no
// ObsUncertainty) and the round carries VerdictPass. The fast-path
// caller passes `source = observational_answer_fastpath` to signal this
// so the quality-gate override is suppressed.
//
// Regression guard: removing the source bypass (or removing the
// `source != completeEventSourceObservationalAnswerFastPath` clause
// in buildSessionCompleteEvent) makes this test fail.
func TestBuildSessionCompleteEvent_preserves_fastpath_short_answer(t *testing.T) {
	tm := workmodel.NewTaskManager()
	sessionID := "sess-fastpath-short"
	_, _ = tm.EnsureGoal(sessionID, "2x3=几?")

	// "2×3=6" is 4 runes — well under the 100-rune too_short threshold.
	const fastPathAnswer = "2×3=6"
	ev := buildSessionCompleteEvent(context.Background(), sessionID, tm, fastPathAnswer, completeEventSourceObservationalAnswerFastPath)
	if ev == nil {
		t.Fatal("nil event")
	}
	if ev.Content != fastPathAnswer {
		t.Fatalf("content = %q, want %q (fast-path answer must not be replaced by taskIncompleteUserMessage)",
			ev.Content, fastPathAnswer)
	}
	if ev.Metadata["task_incomplete"] == "true" {
		t.Fatalf("task_incomplete must be false for fast-path source; meta=%v", ev.Metadata)
	}
	// Quality meta is still recorded for observability (Jaeger / dashboards).
	if ev.Metadata["summary_quality"] != string(SummaryQualityTooShort) {
		t.Errorf("summary_quality = %q, want %q (gate still runs, just doesn't override)",
			ev.Metadata["summary_quality"], SummaryQualityTooShort)
	}
	if ev.Metadata["final_quality"] != string(SummaryQualityTooShort) {
		t.Errorf("final_quality = %q, want %q", ev.Metadata["final_quality"], SummaryQualityTooShort)
	}
	if ev.Metadata["source"] != completeEventSourceObservationalAnswerFastPath {
		t.Errorf("source meta = %q, want %q", ev.Metadata["source"], completeEventSourceObservationalAnswerFastPath)
	}
}

// DM-20260708-002: same content, but source="" — must still trigger
// task_incomplete (the gate's override is per-source, not per-content).
// This pins the asymmetric behavior: the fast-path bypass is opt-in
// via source, not implicit.
func TestBuildSessionCompleteEvent_task_incomplete_still_triggers_when_source_unknown(t *testing.T) {
	tm := workmodel.NewTaskManager()
	sessionID := "sess-short-unknown-source"
	_, _ = tm.EnsureGoal(sessionID, "2x3=几?")

	ev := buildSessionCompleteEvent(context.Background(), sessionID, tm, "2×3=6", "")
	if ev.Content != taskIncompleteUserMessage {
		t.Fatalf("content = %q, want %q (unknown source must not bypass task_incomplete override)",
			ev.Content, taskIncompleteUserMessage)
	}
	if ev.Metadata["task_incomplete"] != "true" {
		t.Fatal("expected task_incomplete meta when source is empty")
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
