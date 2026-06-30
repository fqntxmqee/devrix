package sessionorchestrator

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

func validRollupSummary() string {
	var b strings.Builder
	b.WriteString("Executive summary of d2 review.\nP0 issues:\n")
	b.WriteString(strings.Repeat("critical finding detail. ", 40))
	b.WriteString("\nP1 issues:\n")
	b.WriteString(strings.Repeat("minor finding detail. ", 20))
	return b.String()
}

func TestVerifyRollupArtifact_Pass(t *testing.T) {
	art := &wavescheduler.Artifact{
		TaskID:   "wi_parent",
		Summary:  validRollupSummary(),
		ExitCode: 0,
	}
	v := verifyRollupArtifact(art, workmodel.ChildOutcomeStats{Total: 3, Completed: 3})
	if v.Kind != types.VerdictPass {
		t.Fatalf("kind=%v reason=%s", v.Kind, v.Reason)
	}
}

func TestVerifyRollupArtifact_TooShort(t *testing.T) {
	art := &wavescheduler.Artifact{
		TaskID:   "wi_parent",
		Summary:  "P0: short P1: short",
		ExitCode: 0,
	}
	v := verifyRollupArtifact(art, workmodel.ChildOutcomeStats{Total: 3, Completed: 3})
	if v.Kind != types.VerdictFail {
		t.Fatalf("kind=%v, want fail", v.Kind)
	}
}

func TestVerifyRollupArtifact_PlanningDenylist(t *testing.T) {
	art := &wavescheduler.Artifact{
		TaskID:   "wi_parent",
		Summary:  validRollupSummary() + "\n我将要 parallel explore",
		ExitCode: 0,
	}
	v := verifyRollupArtifact(art, workmodel.ChildOutcomeStats{Total: 3, Completed: 3})
	if v.Kind != types.VerdictFail {
		t.Fatalf("kind=%v, want fail for planning meta", v.Kind)
	}
}

// T: D7-S15-A90-T01 (DM-20260701-001 RH-MUPS-04 RollupOutcomeAggregation)
//
// All children failed → rollup MUST refuse Pass even if the synthesized
// summary is well-formed. Without this guard, the failure is washed into
// apparent success and the parent gets marked Completed.
func TestVerifyRollupArtifact_AllChildrenFailed_RefusesPass(t *testing.T) {
	art := &wavescheduler.Artifact{
		TaskID:   "wi_parent",
		Summary:  validRollupSummary(),
		ExitCode: 0,
	}
	stats := workmodel.ChildOutcomeStats{Total: 3, Failed: 3}
	v := verifyRollupArtifact(art, stats)
	if v.Kind != types.VerdictFail {
		t.Fatalf("all-failed rollup: kind=%v reason=%s, want fail", v.Kind, v.Reason)
	}
	if !strings.Contains(v.Reason, "3") {
		t.Errorf("reason should mention failed count, got %q", v.Reason)
	}
}

// T: D7-S15-A90-T02 (DM-20260701-001 RH-MUPS-04 RollupOutcomeAggregation)
//
// Some failed + some still running → Partial (premature rollup).
func TestVerifyRollupArtifact_FailedAndRunning_Partial(t *testing.T) {
	art := &wavescheduler.Artifact{
		TaskID:   "wi_parent",
		Summary:  validRollupSummary(),
		ExitCode: 0,
	}
	stats := workmodel.ChildOutcomeStats{Total: 4, Completed: 1, Failed: 2, Running: 1}
	v := verifyRollupArtifact(art, stats)
	if v.Kind != types.VerdictPartial {
		t.Fatalf("failed+running: kind=%v reason=%s, want partial", v.Kind, v.Reason)
	}
}

// T: D7-S15-A90-T03 (DM-20260701-001 RH-MUPS-04 RollupOutcomeAggregation)
//
// Some failed + some completed (no running) → still Pass. The failure
// info is in the artifact summary; we don't refuse Pass here because at
// least some children succeeded and the rollup synthesizes a partial-
// success picture. (The aggregate-gate refusal is for all-failed only.)
func TestVerifyRollupArtifact_MixedFailure_Passes(t *testing.T) {
	art := &wavescheduler.Artifact{
		TaskID:   "wi_parent",
		Summary:  validRollupSummary(),
		ExitCode: 0,
	}
	stats := workmodel.ChildOutcomeStats{Total: 4, Completed: 2, Failed: 2}
	v := verifyRollupArtifact(art, stats)
	if v.Kind != types.VerdictPass {
		t.Fatalf("mixed failure: kind=%v reason=%s, want pass", v.Kind, v.Reason)
	}
}

// T: D7-S15-A90-T04 (DM-20260701-001 RH-MUPS-04 RollupOutcomeAggregation)
//
// Zero children (empty stats) → behaviour matches legacy: rollup verify
// runs on summary shape alone. Tests the "rollup with no children" path
// used by the root-rollup fallback.
func TestVerifyRollupArtifact_NoChildren_LegacyShapeCheck(t *testing.T) {
	art := &wavescheduler.Artifact{
		TaskID:   "wi_parent",
		Summary:  validRollupSummary(),
		ExitCode: 0,
	}
	v := verifyRollupArtifact(art, workmodel.ChildOutcomeStats{})
	if v.Kind != types.VerdictPass {
		t.Fatalf("no-children rollup: kind=%v reason=%s, want pass (legacy shape check)", v.Kind, v.Reason)
	}
}

func TestRunItemPipeline_RollupRound(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)

	sessionID := "sess-rollup"
	parent, _ := tm.EnsureGoal(sessionID, "review d2 domain code")
	childPass, _ := tm.CreateWorkItem(sessionID, workmodel.CreateWorkItemInput{
		ParentID: parent.ID, Kind: workmodel.WorkKindImplement, Title: "review prepare",
	})
	childFail, _ := tm.CreateWorkItem(sessionID, workmodel.CreateWorkItemInput{
		ParentID: parent.ID, Kind: workmodel.WorkKindImplement, Title: "review gateway",
	})
	_ = tm.Tree().ApplyPipelineRound(sessionID, childPass.ID, &workmodel.WorkItemPipelineRound{
		WorkItemID:        childPass.ID,
		VerdictKind:       types.VerdictPass,
		VerdictID:         "v_pass",
		PlanID:            "plan_pass",
		ArtifactSummary:   "child pass summary with findings",
		ContextBubbleKind: workmodel.BubbleStructured,
	}, workmodel.RoundPhaseIdle)
	_ = tm.UpdateStatus(sessionID, childPass.ID, workmodel.TaskStatusInProgress)
	_ = tm.UpdateStatus(sessionID, childPass.ID, workmodel.TaskStatusCompleted)
	_ = tm.Tree().ApplyPipelineRound(sessionID, childFail.ID, &workmodel.WorkItemPipelineRound{
		WorkItemID:        childFail.ID,
		VerdictKind:       types.VerdictFail,
		VerdictID:         "v_fail",
		PlanID:            "plan_fail",
		ArtifactSummary:   "child fail summary with blockers",
		ContextBubbleKind: workmodel.BubbleStructured,
	}, workmodel.RoundPhaseIdle)
	_ = tm.UpdateStatus(sessionID, childFail.ID, workmodel.TaskStatusInProgress)
	_ = tm.UpdateStatus(sessionID, childFail.ID, workmodel.TaskStatusFailed)

	_ = tm.Tree().ApplyPipelineRound(sessionID, parent.ID, &workmodel.WorkItemPipelineRound{
		WorkItemID:  parent.ID,
		SpawnPolicy: workmodel.SpawnDecompose,
		VerdictKind: types.VerdictPartial,
		PlanID:      "plan_parent",
	}, workmodel.RoundPhaseAwaitChild)
	_ = tm.Tree().SetNeedsRollup(sessionID, parent.ID, true)
	_ = tm.Tree().ReopenForRollup(sessionID, parent.ID)
	parent, _ = tm.GetWorkItem(sessionID, parent.ID)

	capture := &capturingWorkItemExecutor{}
	runner.Executor = capture
	runner.Executor = &rollupContentExecutor{summary: validRollupSummary(), capture: capture}

	round, err := runner.Run(context.Background(), sessionID, parent, "", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.PlanKind != plan.CommitmentPlan {
		t.Fatalf("PlanKind=%v, want CommitmentPlan", round.PlanKind)
	}
	if round.VerdictKind != types.VerdictPass {
		t.Fatalf("VerdictKind=%v, want pass", round.VerdictKind)
	}
	if utf8.RuneCountInString(round.ArtifactSummary) < 500 {
		t.Fatalf("artifact summary too short: %d runes", utf8.RuneCountInString(round.ArtifactSummary))
	}
	if len(capture.calls) != 1 {
		t.Fatalf("executor calls=%d, want 1", len(capture.calls))
	}
	dir := capture.calls[0].Directive
	if !strings.Contains(dir, childPass.ID) || !strings.Contains(dir, childFail.ID) {
		t.Fatalf("rollup directive missing child ids: %s", dir)
	}
	if !strings.Contains(dir, "verdict=pass") || !strings.Contains(dir, "verdict=fail") {
		t.Fatalf("rollup directive missing verdicts: %s", dir)
	}

	got, _ := tm.GetWorkItem(sessionID, parent.ID)
	if got.NeedsRollup {
		t.Fatal("NeedsRollup should be cleared after pass")
	}
	if got.Status != workmodel.TaskStatusCompleted {
		t.Fatalf("status=%s, want completed", got.Status)
	}
}

type rollupContentExecutor struct {
	summary string
	capture *capturingWorkItemExecutor
}

func (e *rollupContentExecutor) ExecuteWorkItem(ctx context.Context, sessionID, itemID, directive string) (*WorkItemResult, error) {
	if e.capture != nil {
		e.capture.ExecuteWorkItem(ctx, sessionID, itemID, directive)
	}
	return &WorkItemResult{
		Content:    e.summary,
		Done:       true,
		Iterations: 1,
		StopReason: "final_answer",
	}, nil
}
