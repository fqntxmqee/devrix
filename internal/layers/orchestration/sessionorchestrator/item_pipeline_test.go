package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// stubWorkItemExecutor is the production-shape WorkItemExecutor used by
// ItemPipelineRunner tests that don't care about executor internals.
// Always returns a passing result so the round completes successfully.
type stubWorkItemExecutor struct{}

func (stubWorkItemExecutor) ExecuteWorkItem(_ context.Context, _, _, directive string) (*WorkItemResult, error) {
	return &WorkItemResult{
		Content:    "ok: " + directive,
		Done:       true,
		Iterations: 1,
		ToolCalls:  0,
		StopReason: "final_answer",
	}, nil
}

// capturingWorkItemExecutor records the directive of each ExecuteWorkItem
// call. Used by the regression test for DM-20260626-009: ItemPipelineRunner
// must pass the WorkItem's directive straight to the executor (no shim, no
// ToolArgs JSON, no synthetic tool name).
type capturingWorkItemExecutor struct {
	calls []capturedWorkItemCall
}

type capturedWorkItemCall struct {
	SessionID string
	ItemID    string
	Directive string
}

func (s *capturingWorkItemExecutor) ExecuteWorkItem(_ context.Context, sessionID, itemID, directive string) (*WorkItemResult, error) {
	s.calls = append(s.calls, capturedWorkItemCall{
		SessionID: sessionID,
		ItemID:    itemID,
		Directive: directive,
	})
	return &WorkItemResult{
		Content:    "ok",
		Done:       true,
		Iterations: 1,
		StopReason: "final_answer",
	}, nil
}

// contentWorkItemExecutor is a stub that returns a fixed Content. Used by
// the regression test for Artifact truncation (DM-20260626-009 follow-up):
// the ItemPipelineRunner must propagate the full Content into round.ArtifactSummary
// so feishu shows the entire LLM response, not just a 200-char prefix.
type contentWorkItemExecutor struct {
	content string
}

func (c *contentWorkItemExecutor) ExecuteWorkItem(_ context.Context, _, _, _ string) (*WorkItemResult, error) {
	return &WorkItemResult{
		Content:    c.content,
		Done:       true,
		Iterations: 1,
		StopReason: "final_answer",
	}, nil
}

func newItemPipelineTestRunner(t *testing.T) (*ItemPipelineRunner, *workmodel.TaskManager, *learn.InMemoryReputationStore) {
	t.Helper()
	tm := workmodel.NewTaskManager()
	skill := learn.NewSkillMemory()
	feedback := learn.NewFeedbackMemory()
	scheduled := learn.NewScheduledMemory()
	rep := learn.NewInMemoryReputationStore()
	learner := learn.NewDefaultLearner(skill, feedback, scheduled, rep, learn.NewAssetBuilder())
	runner, err := NewItemPipelineRunner(ItemPipelineDeps{
		Executor: stubWorkItemExecutor{},
		Learner:  learner,
		Tasks:    tm,
	})
	if err != nil {
		t.Fatalf("NewItemPipelineRunner: %v", err)
	}
	return runner, tm, rep
}

func TestRunItemPipeline_SingleWorkItem_Completed(t *testing.T) {
	runner, tm, rep := newItemPipelineTestRunner(t)
	sessionID := "sess-item-pipeline"
	goal, err := tm.EnsureGoal(sessionID, "implement cache layer")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	_ = tm.Tree().SetUncertainty(sessionID, goal.ID, 0.2)
	goal, _ = tm.GetWorkItem(sessionID, goal.ID)

	round, err := runner.Run(context.Background(), sessionID, goal, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.SpawnPolicy != workmodel.SpawnNone {
		t.Fatalf("SpawnPolicy = %q, want none", round.SpawnPolicy)
	}
	if len(round.ObservationIDs) == 0 || round.PlanID == "" || round.VerdictID == "" {
		t.Fatalf("LP-5 incomplete round: %+v", round)
	}
	if round.VerdictKind != types.VerdictPass {
		t.Fatalf("VerdictKind = %v, want Pass", round.VerdictKind)
	}

	got, _ := tm.GetWorkItem(sessionID, goal.ID)
	if got.Status != workmodel.TaskStatusCompleted {
		t.Fatalf("status = %q, want completed", got.Status)
	}
	if got.LastRound == nil || got.LastRound.PlanID != round.PlanID {
		t.Fatal("LastRound not persisted on WorkItem")
	}

	ev, err := rep.Get(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("rep.Get: %v", err)
	}
	if ev == nil || ev.Alpha < 1 {
		t.Fatalf("reputation after learn = %+v, want Alpha>=1", ev)
	}
}

func TestRunItemPipeline_PartialHighUncertainty_SpawnDecompose(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	runner.Verify = func(_ *wavescheduler.Artifact) workmodel.Verdict {
		return workmodel.Verdict{
			Kind:       types.VerdictPartial,
			Reason:     "needs more exploration",
			SourceID:   "v_partial",
			Confidence: 0.5,
		}
	}

	sessionID := "sess-decompose"
	goal, _ := tm.EnsureGoal(sessionID, "compare three cache strategies")
	_ = tm.Tree().SetUncertainty(sessionID, goal.ID, 0.85)
	goal, _ = tm.GetWorkItem(sessionID, goal.ID)

	round, err := runner.Run(context.Background(), sessionID, goal, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.SpawnPolicy != workmodel.SpawnDecompose {
		t.Fatalf("SpawnPolicy = %q, want decompose (G1 partial+uncertainty)", round.SpawnPolicy)
	}
	if round.UncertaintyMean <= workmodel.DefaultUncertaintyDecomposeThreshold {
		t.Fatalf("UncertaintyMean = %.2f, expected > threshold", round.UncertaintyMean)
	}

	got, _ := tm.GetWorkItem(sessionID, goal.ID)
	if got.Status == workmodel.TaskStatusCompleted {
		t.Fatal("partial decompose path should not mark item completed yet")
	}
	if got.RoundPhase != workmodel.RoundPhaseAwaitChild {
		t.Fatalf("RoundPhase = %q, want await_child", got.RoundPhase)
	}
}

func TestRunItemPipeline_LP5_LineageFields(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	sessionID := "sess-lp5"
	goal, _ := tm.EnsureGoal(sessionID, "verify login flow")
	round, err := runner.Run(context.Background(), sessionID, goal, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(round.ObservationIDs) < 1 {
		t.Fatal("expected observation IDs")
	}
	if round.PlanID == "" || round.ArtifactID == "" || round.VerdictID == "" {
		t.Fatalf("LP-5 chain broken: %+v", round)
	}
	if round.ExitReason == "" {
		t.Fatal("ExitReason required")
	}
}

// TestRunItemPipeline_WorkItemExecutorReceivesDirective is the regression
// test for DM-20260626-009: ItemPipelineRunner.Run must pass the WorkItem's
// directive straight to WorkItemExecutor.ExecuteWorkItem (no CommitChannel
// shim, no ToolArgs JSON, no synthetic work_item_execute tool name).
//
// Pre-DM-20260626-009 (PR #249+#250) the directive flowed via
// Plan.Step.ToolArgs → CommitChannel → ItemToolRunner → LLM. The 2026-06-26
// hotfix wired the LLM call but the synthetic-tool plumbing was the wrong
// shape: a directive is a first-class WorkItem parameter, not a tool
// argument. ItemPipelineRunner now calls the Executor directly with the
// directive as a parameter; the capturing executor below asserts the
// parameter reaches the executor intact.
func TestRunItemPipeline_WorkItemExecutorReceivesDirective(t *testing.T) {
	tm := workmodel.NewTaskManager()
	exec := &capturingWorkItemExecutor{}
	r, err := NewItemPipelineRunner(ItemPipelineDeps{
		Executor: exec,
		Tasks:    tm,
	})
	if err != nil {
		t.Fatalf("NewItemPipelineRunner: %v", err)
	}

	sessionID := "sess-item-pipeline-directive"
	const directive = "review d2领域代码"
	item, err := tm.CreateWorkItem(sessionID, workmodel.CreateWorkItemInput{
		Kind: workmodel.WorkKindPlan, Title: directive, Directive: directive,
	})
	if err != nil {
		t.Fatalf("CreateWorkItem: %v", err)
	}
	_ = tm.Tree().SetUncertainty(sessionID, item.ID, 0.1)

	if _, err := r.Run(context.Background(), sessionID, item, ""); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(exec.calls) == 0 {
		t.Fatal("WorkItemExecutor.ExecuteWorkItem was never called (no pipeline round reached execute)")
	}
	got := exec.calls[0]
	if got.SessionID != sessionID {
		t.Fatalf("SessionID = %q, want %q", got.SessionID, sessionID)
	}
	if got.ItemID != item.ID {
		t.Fatalf("ItemID = %q, want %q", got.ItemID, item.ID)
	}
	if got.Directive != directive {
		t.Fatalf("Directive = %q, want %q (regression: ItemPipelineRunner.Run must inject the WorkItem's directive straight into WorkItemExecutor.ExecuteWorkItem)",
			got.Directive, directive)
	}
}

// TestRunItemPipeline_LongLLMResponseSurvivesArtifact is the regression
// test for the post-PR-#251 truncation bug: buildArtifactFromWorkItemResult
// used truncateForArtifact(content, 200), which cut long LLM reviews down
// to 200 chars + ellipsis. The user then saw only the truncated prefix in
// the feishu reply card (sess_1782472901145 — LLM emitted a 700-char review
// after 8 bash tool calls; the user received only the first 203 bytes).
//
// DM-20260626-009 follow-up: WorkerWorkItem artifacts hold the user's
// answer verbatim. Skip truncation for WorkerWorkItem; downstream Learn
// truncates evidence further (asset_builder.go:272).
func TestRunItemPipeline_LongLLMResponseSurvivesArtifact(t *testing.T) {
	tm := workmodel.NewTaskManager()
	longContent := strings.Repeat("Devrix D2 上下文引擎层. ", 50) // ~1400 chars
	if len(longContent) < 500 {
		t.Fatalf("test fixture too short: %d chars", len(longContent))
	}
	exec := &contentWorkItemExecutor{content: longContent}
	r, err := NewItemPipelineRunner(ItemPipelineDeps{
		Executor: exec,
		Tasks:    tm,
	})
	if err != nil {
		t.Fatalf("NewItemPipelineRunner: %v", err)
	}
	sessionID := "sess-long-content"
	goal, err := tm.EnsureGoal(sessionID, "review d2领域代码")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	_ = tm.Tree().SetUncertainty(sessionID, goal.ID, 0.1)

	round, err := r.Run(context.Background(), sessionID, goal, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.ArtifactSummary != longContent {
		t.Fatalf("ArtifactSummary truncated: len=%d, want=%d (regression: WorkerWorkItem artifact must hold the full LLM response)",
			len(round.ArtifactSummary), len(longContent))
	}
}