//go:build integration && d7

package d7integration

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// TestIntegration_RollupTraceReplay_Stub verifies Path B rollup gate wiring.
// Full TurnLoop 2× MUPS + complete.Content E2E remains stubbed until CI stub LLM lands.
func TestIntegration_RollupTraceReplay_Stub(t *testing.T) {
	tm := workmodel.NewTaskManager()
	sessionID := "rollup-trace-stub"
	goal, _ := tm.EnsureGoal(sessionID, "review d2 domain code")
	_ = tm.Tree().ApplyPipelineRound(sessionID, goal.ID, &workmodel.WorkItemPipelineRound{
		SpawnPolicy: workmodel.SpawnNone,
		VerdictKind: types.VerdictFail,
	}, workmodel.RoundPhaseIdle)
	_ = tm.Tree().UpsertChecklist(sessionID, goal.ID, []workmodel.ChecklistEntry{
		{Content: "review prepare/", Status: workmodel.TaskStatusPending},
	})
	_ = tm.UpdateStatus(sessionID, goal.ID, workmodel.TaskStatusInProgress)
	_ = tm.UpdateStatus(sessionID, goal.ID, workmodel.TaskStatusFailed)

	wi, ok := workmodel.MaybeRootRollupFallback(sessionID, tm)
	if !ok || wi == nil {
		t.Fatal("expected root rollup fallback for spawn=none + checklist fixture")
	}
	if !wi.NeedsRollup {
		t.Fatal("expected NeedsRollup after fallback")
	}
	if wi.Status != workmodel.TaskStatusPending {
		t.Fatalf("status=%s, want pending after reopen", wi.Status)
	}
}
