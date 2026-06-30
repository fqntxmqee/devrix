package sessionorchestrator

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D7-S15-A89-T06 (DM-20260701-001 RH-MUPS-03 findExhaustedRollupParent)
//
// When a rollup parent's LastRound.RollupRetries hits DefaultMaxRollupRetries
// and the parent is non-terminal, findExhaustedRollupParent MUST surface
// it so the session loop can emit human_review instead of exiting silently.
func TestFindExhaustedRollupParent_AtLimit_Surfaces(t *testing.T) {
	sessionID := "sess-rollup-exhausted"
	tm := workmodel.NewTaskManager()

	parent, err := tm.EnsureGoal(sessionID, "review d2 code")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	if err := tm.Tree().SetNeedsRollup(sessionID, parent.ID, true); err != nil {
		t.Fatalf("SetNeedsRollup: %v", err)
	}
	// Persist a real round with RollupRetries at the limit so the tree sees it.
	if err := tm.Tree().ApplyPipelineRound(sessionID, parent.ID, &workmodel.WorkItemPipelineRound{
		WorkItemID:    parent.ID,
		VerdictKind:   types.VerdictFail,
		RollupRetries: workmodel.DefaultMaxRollupRetries,
		ExitReason:    "rollup summary missing P0/P1 sections",
	}, workmodel.RoundPhaseIdle); err != nil {
		t.Fatalf("ApplyPipelineRound: %v", err)
	}

	got, reason := findExhaustedRollupParent(sessionID, tm)
	if got == nil {
		t.Fatalf("findExhaustedRollupParent returned nil; want the exhausted rollup parent")
	}
	if got.ID != parent.ID {
		t.Errorf("item.ID = %s, want %s", got.ID, parent.ID)
	}
	if reason == "" {
		t.Errorf("reason = empty, want non-empty verdict description")
	}
}

// T: D7-S15-A89-T07 (DM-20260701-001 RH-MUPS-03 findExhaustedRollupParent)
//
// Below limit → not surfaced. Without this check, healthy rollups with
// retry=0 would also trigger human_review on every session close.
func TestFindExhaustedRollupParent_BelowLimit_NotSurfaced(t *testing.T) {
	sessionID := "sess-rollup-below"
	tm := workmodel.NewTaskManager()

	parent, _ := tm.EnsureGoal(sessionID, "review d2")
	_ = tm.Tree().SetNeedsRollup(sessionID, parent.ID, true)
	_ = tm.Tree().ApplyPipelineRound(sessionID, parent.ID, &workmodel.WorkItemPipelineRound{
		WorkItemID:    parent.ID,
		VerdictKind:   types.VerdictPartial,
		RollupRetries: 1,
	}, workmodel.RoundPhaseIdle)

	got, reason := findExhaustedRollupParent(sessionID, tm)
	if got != nil || reason != "" {
		t.Errorf("unexpectedly surfaced: item=%v reason=%q", got, reason)
	}
}

// T: D7-S15-A89-T08 (DM-20260701-001 RH-MUPS-03 findExhaustedRollupParent)
//
// Terminal rollup parents (Completed/Failed/Cancelled) MUST NOT be
// surfaced even at the retry limit — they're done, just unsuccessfully.
// Only non-terminal items (InProgress/Pending) are unresolved.
func TestFindExhaustedRollupParent_TerminalNotSurfaced(t *testing.T) {
	sessionID := "sess-rollup-terminal"
	tm := workmodel.NewTaskManager()

	parent, _ := tm.EnsureGoal(sessionID, "review d2")
	_ = tm.Tree().SetNeedsRollup(sessionID, parent.ID, true)
	_ = tm.Tree().ApplyPipelineRound(sessionID, parent.ID, &workmodel.WorkItemPipelineRound{
		WorkItemID:    parent.ID,
		VerdictKind:   types.VerdictFail,
		RollupRetries: workmodel.DefaultMaxRollupRetries,
	}, workmodel.RoundPhaseIdle)
	if err := tm.UpdateStatus(sessionID, parent.ID, workmodel.TaskStatusInProgress); err != nil {
		t.Fatalf("UpdateStatus InProgress: %v", err)
	}
	if err := tm.UpdateStatus(sessionID, parent.ID, workmodel.TaskStatusFailed); err != nil {
		t.Fatalf("UpdateStatus Failed: %v", err)
	}

	got, _ := findExhaustedRollupParent(sessionID, tm)
	if got != nil {
		t.Errorf("terminal rollup parent surfaced; should be skipped")
	}
}

// T: D7-S15-A89-T09 (DM-20260701-001 RH-MUPS-03 findExhaustedRollupParent)
//
// Items without NeedsRollup (the common leaf case) MUST NOT be surfaced,
// even if their LastRound.RollupRetries somehow reaches the limit.
func TestFindExhaustedRollupParent_NonRollupNotSurfaced(t *testing.T) {
	sessionID := "sess-non-rollup"
	tm := workmodel.NewTaskManager()

	parent, _ := tm.EnsureGoal(sessionID, "review d2")
	// NeedsRollup stays false (default).
	_ = tm.Tree().ApplyPipelineRound(sessionID, parent.ID, &workmodel.WorkItemPipelineRound{
		WorkItemID:    parent.ID,
		VerdictKind:   types.VerdictFail,
		RollupRetries: workmodel.DefaultMaxRollupRetries,
	}, workmodel.RoundPhaseIdle)

	got, _ := findExhaustedRollupParent(sessionID, tm)
	if got != nil {
		t.Errorf("non-rollup item surfaced; should be skipped (NeedsRollup=false)")
	}
}

// T: D7-S15-A89-T10 (DM-20260701-001 RH-MUPS-03 findExhaustedRollupParent)
//
// Nil TaskManager MUST NOT panic.
func TestFindExhaustedRollupParent_NilTaskManager(t *testing.T) {
	got, reason := findExhaustedRollupParent("sess", nil)
	if got != nil || reason != "" {
		t.Errorf("nil tm: got=%v reason=%q, want zero values", got, reason)
	}
}