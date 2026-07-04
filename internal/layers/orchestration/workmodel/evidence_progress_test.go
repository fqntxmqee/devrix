package workmodel

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestEvidenceProgress_toolCallsAndScope(t *testing.T) {
	got := EvidenceProgress(EvidenceInput{ToolCalls: 3, HasScopeIn: true})
	if got < EvidenceSufficientThreshold {
		t.Fatalf("progress=%v want >= %v", got, EvidenceSufficientThreshold)
	}
}

func TestEvidenceProgress_insufficient(t *testing.T) {
	got := EvidenceProgress(EvidenceInput{ToolCalls: 0, HasScopeIn: false})
	if got >= EvidenceSufficientThreshold {
		t.Fatalf("progress=%v want < %v", got, EvidenceSufficientThreshold)
	}
}

func TestRollupSynthEligible_requiresEvidence(t *testing.T) {
	round := baseRoundForEvidence(0.3)
	round.DeliverableContract = DefaultTestDeliverableContract()
	round.DeliverableStatus = DeliverableStatusIncomplete
	round.ExecuteToolCalls = 0
	ctx := baseCtx()
	if RollupSynthEligible(round, ctx) {
		t.Fatal("expected false without tool evidence")
	}
	round.ExecuteToolCalls = 3
	round.ScopeInPresent = true
	if !RollupSynthEligible(round, ctx) {
		t.Fatal("expected true with evidence and low U")
	}
}

func TestSpawnPolicyEvaluator_CCU1_inlineNotEscalateWithEvidence(t *testing.T) {
	round := baseRoundForEvidence(0.3)
	round.DeliverableContract = DefaultTestDeliverableContract()
	round.DeliverableStatus = DeliverableStatusIncomplete
	round.ExecuteToolCalls = 4
	round.ScopeInPresent = true
	ctx := baseCtx()
	ctx.InlineRetriesAtMaxDepth = 2
	ctx.MaxInlineRetriesAtMaxDepth = 3
	got := SpawnPolicyEvaluator(round, ctx)
	if got != SpawnInline {
		t.Fatalf("got %q, want inline (rollup synth)", got)
	}
	EvaluateSpawnPolicy(round, ctx)
	if !round.RollupSynthRequested {
		t.Fatal("expected RollupSynthRequested")
	}
}

func TestSpawnPolicyEvaluator_CCU1_escalateWithoutEvidence(t *testing.T) {
	round := baseRoundForEvidence(0.3)
	round.DeliverableContract = DefaultTestDeliverableContract()
	round.DeliverableStatus = DeliverableStatusIncomplete
	ctx := baseCtx()
	ctx.InlineRetriesAtMaxDepth = 2
	ctx.MaxInlineRetriesAtMaxDepth = 3
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnEscalateHuman {
		t.Fatalf("got %q, want escalate_human", got)
	}
}

func TestSpawnRationale_CC12_notR7(t *testing.T) {
	round := baseRoundForEvidence(0.3)
	round.DeliverableContract = DefaultTestDeliverableContract()
	round.DeliverableStatus = DeliverableStatusIncomplete
	ctx := baseCtx()
	ctx.InlineRetriesAtMaxDepth = 3
	ctx.MaxInlineRetriesAtMaxDepth = 3
	rationale := spawnRationale(SpawnEscalateHuman, round, ctx)
	if rationale == "" {
		t.Fatal("empty rationale")
	}
	if !strings.Contains(rationale, "CC-1.2") && !strings.Contains(rationale, "inline retries exhausted") {
		t.Fatalf("rationale=%q want CC-1.2 or inline exhausted", rationale)
	}
	if strings.Contains(rationale, "R7: indeterminate") {
		t.Fatalf("rationale=%q should not be R7 indeterminate", rationale)
	}
}

func baseRoundForEvidence(u float64) *WorkItemPipelineRound {
	return &WorkItemPipelineRound{
		WorkItemID:      "wi_ev",
		PlanID:          "plan_1",
		VerdictID:       "v_1",
		ObservationIDs:  []string{"obs_1"},
		VerdictKind:     types.VerdictPartial,
		PlanKind:        plan.CommitmentPlan,
		UncertaintyMean: u,
	}
}
