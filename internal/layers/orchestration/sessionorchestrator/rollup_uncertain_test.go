package sessionorchestrator

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D7-S8-A95-T22 (DM-20260701-001 T-P2-3 ChildUncertaintyBubble)
//
// buildRollupDirective MUST surface any high-uncertainty child in a
// dedicated "UncertainChildren:" section so the LLM cannot accidentally
// drop the signal. Without this, a single high-u child gets washed
// into the aggregated mean and the parent never learns the per-child
// uncertainty distribution.
func TestBuildRollupDirective_SurfacesUncertainChildren(t *testing.T) {
	tm := workmodel.NewTaskManager()
	sessionID := "sess_uncert"
	parent, _ := tm.EnsureGoal(sessionID, "review d2 code")
	low, _ := tm.CreateWorkItem(sessionID, workmodel.CreateWorkItemInput{
		ParentID: parent.ID, Kind: workmodel.WorkKindImplement, Title: "low-u child",
	})
	high, _ := tm.CreateWorkItem(sessionID, workmodel.CreateWorkItemInput{
		ParentID: parent.ID, Kind: workmodel.WorkKindImplement, Title: "high-u child",
	})

	// Low-u child: terminal pass with u=0.1 (below threshold 0.6).
	_ = tm.Tree().ApplyPipelineRound(sessionID, low.ID, &workmodel.WorkItemPipelineRound{
		WorkItemID:      low.ID,
		VerdictKind:     types.VerdictPass,
		UncertaintyMean: 0.1,
		ArtifactSummary: "all good",
	}, workmodel.RoundPhaseIdle)
	_ = tm.UpdateStatus(sessionID, low.ID, workmodel.TaskStatusInProgress)
	_ = tm.UpdateStatus(sessionID, low.ID, workmodel.TaskStatusCompleted)

	// High-u child: terminal partial with u=0.8 (above threshold).
	_ = tm.Tree().ApplyPipelineRound(sessionID, high.ID, &workmodel.WorkItemPipelineRound{
		WorkItemID:      high.ID,
		VerdictKind:     types.VerdictPartial,
		UncertaintyMean: 0.8,
		ArtifactSummary: "some uncertainty",
		ExitReason:      "missing p0 evidence",
	}, workmodel.RoundPhaseIdle)
	_ = tm.UpdateStatus(sessionID, high.ID, workmodel.TaskStatusInProgress)
	_ = tm.UpdateStatus(sessionID, high.ID, workmodel.TaskStatusCompleted)

	dir := buildRollupDirective(sessionID, parent, tm)
	if !strings.Contains(dir, "UncertainChildren: 1") {
		t.Errorf("rollup directive must include UncertainChildren section, got:\n%s", dir)
	}
	if !strings.Contains(dir, high.ID) {
		t.Errorf("rollup directive must reference high-u child %s, got:\n%s", high.ID, dir)
	}
	// The low-u child should NOT be in the UncertainChildren section
	// (it's in the regular ChildOutcomes section).
	uncertIdx := strings.Index(dir, "UncertainChildren:")
	childOutIdx := strings.Index(dir, "ChildOutcomes:")
	if uncertIdx < 0 || childOutIdx < 0 {
		t.Fatalf("expected both section markers, got:\n%s", dir)
	}
	if uncertIdx < childOutIdx {
		t.Errorf("UncertainChildren should come after ChildOutcomes")
	}
	// Low child appears in ChildOutcomes but not in UncertainChildren
	between := dir[uncertIdx:]
	if strings.Contains(between, low.ID) {
		t.Errorf("low-u child should NOT be in UncertainChildren section, got:\n%s", between)
	}
}
