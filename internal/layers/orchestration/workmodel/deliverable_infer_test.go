package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D7-S9-A93-T16 (DM-20260703-001) — schema from explicit tags only, no NL keywords.
func TestInferDeliverableSchema_ExplicitTagOnly(t *testing.T) {
	schema := FirstRegisteredDeliverableSchema()
	tag := DeliverableSchemaTag(schema)
	if got := InferDeliverableSchema(nil, "review arbitrary text", tag); got != schema {
		t.Fatalf("expectedReturn tag: got %q", got)
	}
	if got := InferDeliverableSchema(nil, tag, ""); got != schema {
		t.Fatalf("directive tag: got %q", got)
	}
	if got := InferDeliverableSchema(nil, "review d7 plan目录下代码", ""); got != DeliverableSchemaNotApplicable {
		t.Fatalf("NL review without tag: got %q, want not_applicable", got)
	}
}

// T: D7-S5-A93-T04 (DM-20260703-001 CC-1.4) — incomplete deliverable inlines, never auto-decompose.
func TestSpawnPolicyEvaluator_PartialIncompleteDeliverable_Inlines(t *testing.T) {
	schema := FirstRegisteredDeliverableSchema()
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.3)
	round.DeliverableSchema = schema
	round.DeliverableStatus = DeliverableStatusIncomplete
	ctx := baseCtx()
	ctx.CanDecompose = true
	ctx.ChildTotal = 0
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnInline {
		t.Fatalf("got %q, want inline (no deliverable→decompose shortcut)", got)
	}
}
