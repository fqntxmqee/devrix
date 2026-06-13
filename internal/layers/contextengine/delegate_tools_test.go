package contextengine

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent/delegate"
)

func TestDelegateToolDescription_explore_should_discourage_trivial_use(t *testing.T) {
	desc := delegateToolDescription(delegate.WorkerRoleExplore)
	for _, kw := range []string{"read-only", "3+ files", "Do NOT use for trivial", "async=true"} {
		if !strings.Contains(desc, kw) {
			t.Fatalf("explore description missing %q: %s", kw, desc)
		}
	}
}

func TestDelegateToolDescription_plan_should_require_read_only_planning(t *testing.T) {
	desc := delegateToolDescription(delegate.WorkerRolePlan)
	for _, kw := range []string{"read-only", "todo_write", "delegate_implement"} {
		if !strings.Contains(desc, kw) {
			t.Fatalf("plan description missing %q: %s", kw, desc)
		}
	}
}

func TestDelegateToolDescription_implement_should_scope_single_task(t *testing.T) {
	desc := delegateToolDescription(delegate.WorkerRoleImplement)
	for _, kw := range []string{"one scoped task", "task_id", "Do NOT bundle"} {
		if !strings.Contains(desc, kw) {
			t.Fatalf("implement description missing %q: %s", kw, desc)
		}
	}
}

func TestDelegateToolRunner_schema_should_use_rich_description(t *testing.T) {
	runner := &delegateToolRunner{name: "delegate_explore", role: delegate.WorkerRoleExplore}
	schema := runner.Schema()
	if schema.Name != "delegate_explore" {
		t.Fatalf("name = %q", schema.Name)
	}
	if !strings.Contains(schema.Description, "Explore worker") {
		t.Fatalf("unexpected description: %s", schema.Description)
	}
	if !strings.Contains(schema.Parameters, "async") {
		t.Fatalf("parameters should document async: %s", schema.Parameters)
	}
}

func TestDelegateStatusRunner_schema_should_mention_workplan(t *testing.T) {
	runner := newDelegateStatusRunner()
	schema := runner.Schema()
	if !strings.Contains(schema.Description, "WorkPlan") {
		t.Fatalf("unexpected description: %s", schema.Description)
	}
}
