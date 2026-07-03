package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestSpawnPolicyEvaluator_DeliverableInlineWouldExhaustEscalatesAtDepth0(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.3)
	round.DeliverableContract = DefaultTestDeliverableContract()
	round.DeliverableStatus = DeliverableStatusIncomplete
	ctx := baseCtx()
	ctx.InlineRetriesAtMaxDepth = 2
	ctx.MaxInlineRetriesAtMaxDepth = 3
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnEscalateHuman {
		t.Fatalf("got %q, want escalate_human", got)
	}
}
