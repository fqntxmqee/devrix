package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestApplySpawnPolicy_RollupSynthSetsNeedsRollup(t *testing.T) {
	tm := NewTaskManager()
	sessionID := "s1"
	goal, err := tm.EnsureGoal(sessionID, "explore kernel")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.3)
	round.DeliverableContract = DefaultTestDeliverableContract()
	round.DeliverableStatus = DeliverableStatusIncomplete
	round.ExecuteToolCalls = 3
	round.ScopeInPresent = true
	EvaluateSpawnPolicy(round, DefaultTreeEvalContext(sessionID, goal.ID, "", tm))
	if !round.RollupSynthRequested {
		t.Fatal("expected rollup synth requested")
	}
	if err := ApplySpawnPolicy(sessionID, goal, round, tm); err != nil {
		t.Fatalf("ApplySpawnPolicy: %v", err)
	}
	got, ok := tm.GetWorkItem(sessionID, goal.ID)
	if !ok || got == nil || !got.NeedsRollup {
		t.Fatal("expected NeedsRollup after rollup synth spawn apply")
	}
}
