package enforce_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S18-A02 (PermissionMode-driven tool filtering)
// Migrated from internal/layers/contextengine/legacy/engine_helpers_test.go
// during the 2026-06-19 D2 legacy test cleanup. The legacy file tested
// real production code (`enforce.FilterToolsByPermissionMode`) that belongs
// next to its source in enforce/, not in the legacy/ deprecation home.
func TestFilterToolsByPermissionMode_NonPlanMode(t *testing.T) {
	tools := []tools.ToolSchema{{Name: "bash"}, {Name: "read"}}
	got := enforce.FilterToolsByPermissionMode(types.PermissionDefault, tools, "")
	if len(got) != 2 {
		t.Errorf("non-plan mode should not filter, got %d", len(got))
	}
}

func TestFilterToolsByPermissionMode_PlanMode(t *testing.T) {
	tools := []tools.ToolSchema{
		{Name: "bash"},
		{Name: "read"},
		{Name: "write"},
	}
	got := enforce.FilterToolsByPermissionMode(types.PermissionPlan, tools, "/tmp/plan.md")
	if len(got) == len(tools) {
		t.Error("plan mode should filter tools")
	}
}