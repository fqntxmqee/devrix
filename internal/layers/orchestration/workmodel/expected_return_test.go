package workmodel

import "testing"

func TestDeliverableSchemaTag(t *testing.T) {
	schema := FirstRegisteredDeliverableSchema()
	got := DeliverableSchemaTag(schema)
	want := DeliverableSchemaTag(schema)
	if got != want {
		t.Fatalf("tag = %q, want %q", got, want)
	}
}

func TestDefaultChildExpectedReturn_PropagatesParentContract(t *testing.T) {
	contract := DefaultTestDeliverableContract()
	parent := &WorkItem{
		Kind: WorkKindGoal,
		LastRound: &WorkItemPipelineRound{
			DeliverableContract: contract,
		},
	}
	got := DefaultChildExpectedReturn(parent, "any directive without tag")
	want := DeliverableContractTag(contract)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDefaultChildExpectedReturn_NoSchemaUsesStructuralFallback(t *testing.T) {
	parent := &WorkItem{Kind: WorkKindGoal}
	got := DefaultChildExpectedReturn(parent, "review internal/kernel")
	if got != DefaultStructuralExpectedReturn {
		t.Fatalf("got %q, want structural fallback %q", got, DefaultStructuralExpectedReturn)
	}
}

func TestParseDeliverableSchemaTag(t *testing.T) {
	schema := FirstRegisteredDeliverableSchema()
	got := ParseDeliverableSchemaTag("<deliverable_schema>" + string(schema) + "</deliverable_schema>")
	if got != schema {
		t.Fatalf("parsed = %q", got)
	}
}
