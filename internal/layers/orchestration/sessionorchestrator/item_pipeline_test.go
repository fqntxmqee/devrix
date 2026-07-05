package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
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

type execContextCapturingExecutor struct {
	priorReasons []string
}

func (e *execContextCapturingExecutor) ExecuteWorkItem(ctx context.Context, _, _, _ string) (*WorkItemResult, error) {
	ec, _ := WorkItemExecContextFrom(ctx)
	e.priorReasons = append(e.priorReasons, ec.PriorVerifyReason)
	return &WorkItemResult{
		Content:    "ok",
		Done:       true,
		Iterations: 1,
		StopReason: "final_answer",
	}, nil
}

type rejectingStrategicPlanProposer struct{}

func (rejectingStrategicPlanProposer) ProposeStrategicPlan(context.Context, StrategicPlanInput) (*StrategicPlanProposal, error) {
	return nil, &StrategicPlanReject{
		Reason:     BudgetFieldChildren,
		Field:      BudgetFieldChildren,
		Requested:  5,
		MaxAllowed: 2,
	}
}

// commitmentOnlyPlanner is a stub Planner that always returns CommitmentPlan.
// Used by integration tests that need a deterministic plan kind (so the
// M3 行为增量 for ExplorationPlan+VerdictPass does not fire). The M3
// (DM-20260705-008) override for ExplorationPlan+Pass changes
// SpawnNone→SpawnDecompose; tests that need SpawnNone (e.g. "goal
// completes on Pass") must use CommitmentPlan, not Goal (Goal becomes
// ExplorationPlan via intent_orchestrate in planQuantizedKind).
type commitmentOnlyPlanner struct{}

func (commitmentOnlyPlanner) Plan(in plan.PlanInput) (*plan.Plan, error) {
	steps := in.Steps
	if steps == nil {
		steps = []plan.Step{}
	}
	pl := plan.NewPlan(
		plan.NewPlanID(in.SessionID, in.ObservationIDs),
		in.SessionID,
		plan.CommitmentPlan, // force commitment for deterministic test
		in.ObservationIDs,
		steps,
		1.0,
	).WithFailureCriteria(in.FailureCriteria).WithBlastRadius(in.BlastRadius)
	return &pl, nil
}

// decomposeAwarePlanner returns ExplorationPlan on the first call (parent)
// and CommitmentPlan on subsequent calls (children). Used by
// TestRunSessionTurnLoop_DecomposeRecursive_CompletesChildren to satisfy
// M3 (DM-20260705-008) constraints: parent must be decomposable
// (ExplorationPlan + Partial + high U → SpawnDecompose), children must
// be completable (CommitmentPlan + Pass → SpawnNone terminal, no
// Exploration+Pass→SpawnDecompose override).
type decomposeAwarePlanner struct {
	callCount int
}

func (d *decomposeAwarePlanner) Plan(in plan.PlanInput) (*plan.Plan, error) {
	d.callCount++
	kind := plan.CommitmentPlan
	if d.callCount == 1 {
		kind = plan.ExplorationPlan // first call: parent (decomposable)
	}
	steps := in.Steps
	if steps == nil {
		steps = []plan.Step{}
	}
	pl := plan.NewPlan(
		plan.NewPlanID(in.SessionID, in.ObservationIDs),
		in.SessionID,
		kind,
		in.ObservationIDs,
		steps,
		1.0,
	).WithFailureCriteria(in.FailureCriteria).WithBlastRadius(in.BlastRadius)
	return &pl, nil
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

// T: D7-SN-T05 (DM-20260701-002)
func TestRunItemPipeline_StrategicPlanRejectFeedsNextPrompt(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	exec := &execContextCapturingExecutor{}
	runner.Executor = exec
	runner.StrategicPlanProposer = rejectingStrategicPlanProposer{}
	runner.Verify = func(_ *wavescheduler.Artifact) workmodel.Verdict {
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     "first pass failed",
			SourceID:   "v_fail",
			Confidence: 0.4,
		}
	}

	sessionID := "sess-strategic-reject-feedback"
	goal, err := tm.EnsureGoal(sessionID, "review D7 architecture")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}

	first, err := runner.Run(context.Background(), sessionID, goal, "", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if !strings.Contains(first.SpawnRationale, "strategic_plan_rejected") {
		t.Fatalf("first round SpawnRationale = %q, want strategic rejection", first.SpawnRationale)
	}

	goal, _ = tm.GetWorkItem(sessionID, goal.ID)
	if _, err := runner.Run(context.Background(), sessionID, goal, "", ItemPipelineRunOpts{}); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if len(exec.priorReasons) < 2 {
		t.Fatalf("ExecuteWorkItem calls = %d, want at least 2", len(exec.priorReasons))
	}
	if got := exec.priorReasons[1]; !strings.Contains(got, "strategic_plan_rejected") {
		t.Fatalf("second prior reason = %q, want strategic rejection feedback", got)
	}
}

func directiveWithDeliverableSchema(directive string) string {
	return directive + "\n" + workmodel.DeliverableContractTag(workmodel.DefaultTestDeliverableContract())
}

func TestRunItemPipeline_IncompleteDeliverable_InlineRetry(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	exec := &multiRoundReviewExecutor{}
	runner.Executor = exec

	sessionID := "sess-pipeline-retry"
	goal, _ := tm.EnsureGoal(sessionID, directiveWithDeliverableSchema("review d2 domain kernel directory code"))
	ph, _ := tm.CreateWorkItem(sessionID, workmodel.CreateWorkItemInput{
		ParentID: goal.ID, Kind: workmodel.WorkKindImplement, Title: "done",
	})
	_ = tm.Tree().UpdateStatus(sessionID, ph.ID, workmodel.TaskStatusInProgress)
	_ = tm.Tree().UpdateStatus(sessionID, ph.ID, workmodel.TaskStatusCompleted)

	round1, err := runner.Run(context.Background(), sessionID, goal, "", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if round1.SpawnPolicy != workmodel.SpawnInline {
		t.Fatalf("first SpawnPolicy = %q, want inline (got rationale %q)", round1.SpawnPolicy, round1.SpawnRationale)
	}
	if exec.calls != 1 {
		t.Fatalf("calls after first = %d, want 1", exec.calls)
	}

	goal, _ = tm.GetWorkItem(sessionID, goal.ID)
	round2, err := runner.Run(context.Background(), sessionID, goal, "", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if exec.calls != 2 {
		t.Fatalf("calls after second = %d, want 2", exec.calls)
	}
	if round2.SpawnPolicy != workmodel.SpawnNone {
		t.Fatalf("second SpawnPolicy = %q, want none", round2.SpawnPolicy)
	}
	got, _ := tm.GetWorkItem(sessionID, goal.ID)
	if got.Status != workmodel.TaskStatusCompleted {
		t.Fatalf("status = %q, want completed", got.Status)
	}
}

type multiRoundReviewExecutor struct {
	calls int
}

func (e *multiRoundReviewExecutor) ExecuteWorkItem(_ context.Context, _, _, _ string) (*WorkItemResult, error) {
	e.calls++
	if e.calls == 1 {
		return &WorkItemResult{
			Content:    "I'll review the D2 kernel directory.\nLet me locate internal/layers/contextengine/kernel/ first.",
			Done:       true,
			Iterations: 1,
			ToolCalls:  0,
			StopReason: "final_answer",
		}, nil
	}
	return &WorkItemResult{
		Content:    "P0: nil deref in internal/layers/contextengine/kernel/foo.go:42 — missing guard\nExecutive summary: one P0 issue.",
		Done:       true,
		Iterations: 1,
		ToolCalls:  3,
		StopReason: "final_answer",
	}, nil
}

// T: D7-SX-AXX-TXX (DM-20260705-008 M3) — uses commitmentOnlyPlanner
// to isolate from M3 行为增量 (ExplorationPlan+Pass→SpawnDecompose).
// Goal→intent_orchestrate→ExplorationPlan would trigger the M3 override;
// CommitmentPlan does not, so Pass returns SpawnNone (terminal).
func TestRunItemPipeline_SingleWorkItem_Completed(t *testing.T) {
	runner, tm, rep := newItemPipelineTestRunner(t)
	runner.Planner = commitmentOnlyPlanner{} // isolate from M3 行为增量
	sessionID := "sess-item-pipeline"
	goal, err := tm.EnsureGoal(sessionID, "implement cache layer")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	_ = tm.Tree().SetUncertainty(sessionID, goal.ID, 0.2)
	goal, _ = tm.GetWorkItem(sessionID, goal.ID)

	round, err := runner.Run(context.Background(), sessionID, goal, "", ItemPipelineRunOpts{})
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

	round, err := runner.Run(context.Background(), sessionID, goal, "", ItemPipelineRunOpts{})
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
	round, err := runner.Run(context.Background(), sessionID, goal, "", ItemPipelineRunOpts{})
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

	if _, err := r.Run(context.Background(), sessionID, item, "", ItemPipelineRunOpts{}); err != nil {
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

	round, err := r.Run(context.Background(), sessionID, goal, "", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.ArtifactSummary != longContent {
		t.Fatalf("ArtifactSummary truncated: len=%d, want=%d (regression: WorkerWorkItem artifact must hold the full LLM response)",
			len(round.ArtifactSummary), len(longContent))
	}
}
