package sessionorchestrator

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

func TestParseStrategicPlanJSON_should_accept_single_mode(t *testing.T) {
	raw := `{"execution_mode":"single","scope_in":["internal/foo/"],"child_specs":[],"deliverable_schema":"p0_p1_file_line","react_iters_hint":3,"rationale":"one pass"}`
	prop, err := parseStrategicPlanJSON(raw, "review internal/foo/")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if prop.ExecutionMode != "single" {
		t.Fatalf("mode = %q", prop.ExecutionMode)
	}
	if prop.QuantizedKind != "intent_command" {
		t.Fatalf("quantized = %q", prop.QuantizedKind)
	}
	if len(prop.ChildSpecs) != 0 {
		t.Fatalf("expected no child specs for single, got %d", len(prop.ChildSpecs))
	}
}

func TestParseStrategicPlanJSON_should_map_decompose_child_specs(t *testing.T) {
	raw := `{"execution_mode":"decompose","child_specs":[{"title":"slice A","directive_suffix":"focus A","expected_return":"P0 list"}],"deliverable_schema":"p0_p1_file_line","react_iters_hint":2}`
	prop, err := parseStrategicPlanJSON(raw, "review kernel")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prop.ChildSpecs) != 1 {
		t.Fatalf("child specs = %d", len(prop.ChildSpecs))
	}
	if !strings.Contains(prop.ChildSpecs[0].Directive, "focus A") {
		t.Fatalf("directive = %q", prop.ChildSpecs[0].Directive)
	}
}

func TestValidateStrategicPlan_should_reject_decompose_without_children(t *testing.T) {
	_, err := parseStrategicPlanJSON(`{"execution_mode":"decompose","child_specs":[]}`, "review")
	if err == nil {
		t.Fatal("expected error for decompose without child_specs")
	}
}

func TestAppendDeliverableExecuteHint_should_add_schema_tag_for_review_schema(t *testing.T) {
	got := AppendDeliverableExecuteHint("review code", workmodel.DeliverableSchemaP0P1FileLine)
	if !strings.Contains(got, "<deliverable_schema>p0_p1_file_line</deliverable_schema>") {
		t.Fatalf("missing schema tag: %q", got)
	}
}

func TestAppendDeliverableExecuteHint_should_skip_when_not_applicable(t *testing.T) {
	base := "implement feature"
	if got := AppendDeliverableExecuteHint(base, workmodel.DeliverableSchemaNotApplicable); got != base {
		t.Fatalf("unexpected mutation: %q", got)
	}
}
