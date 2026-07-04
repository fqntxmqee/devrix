package prompttags

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

func TestExecuteOutputTagDoc_ContainsEnvelopeTags(t *testing.T) {
	got := ExecuteOutputTagDoc()
	for _, want := range []string{
		"<scope_contract>",
		"<deliverable_schema>",
		"<deliverable_contract>",
		"<open_questions>",
		"<prior_verify_reason>",
		"<conclusion>",
		ScopeContractJSONShape,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ExecuteOutputTagDoc missing %q:\n%s", want, got)
		}
	}
}

func TestDocBlock_ExecutePhase(t *testing.T) {
	got := DocBlock(contracts.MUPSPhaseExecute)
	if !strings.Contains(got, "<scope_contract>") {
		t.Fatalf("execute DocBlock = %q", got)
	}
}

func TestDocBlockObserveSchema(t *testing.T) {
	got := DocBlockObserveSchema()
	if !strings.Contains(got, `"kind":"obs_fact`) {
		t.Fatalf("observe schema = %q", got)
	}
}

func TestDocBlockPlanSchema(t *testing.T) {
	got := DocBlockPlanSchema(`{"citation":"file_line"}`)
	if !strings.Contains(got, `"execution_mode":"single`) {
		t.Fatalf("plan schema = %q", got)
	}
	if !strings.Contains(got, `"citation":"file_line"`) {
		t.Fatalf("plan schema missing contract example: %q", got)
	}
}
