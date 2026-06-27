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
	v := verifyRollupArtifact(art)
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
	v := verifyRollupArtifact(art)
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
	v := verifyRollupArtifact(art)
	if v.Kind != types.VerdictFail {
		t.Fatalf("kind=%v, want fail for planning meta", v.Kind)
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

	round, err := runner.Run(context.Background(), sessionID, parent, "")
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
