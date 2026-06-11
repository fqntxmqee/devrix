package permission_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/permission"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-CTX-37
func TestIsPlanModeAllowedToolName_should_filter_unknown_tools(t *testing.T) {
	if !permission.IsPlanModeAllowedToolName("read_file", "/tmp/plan.md") {
		t.Fatal("read_file should be allowed")
	}
	if permission.IsPlanModeAllowedToolName("unknown_tool", "/tmp/plan.md") {
		t.Fatal("unknown_tool should be filtered")
	}
}

func TestCanWritePath_plan_mode_only_plan_file(t *testing.T) {
	if !permission.CanWritePath(types.PermissionPlan, "/tmp/plan.md", "/tmp/plan.md", "/tmp", false) {
		t.Fatal("plan file should be writable")
	}
	if permission.CanWritePath(types.PermissionPlan, "/tmp/plan.md", "/etc/passwd", "/tmp", false) {
		t.Fatal("non-plan path should be denied")
	}
}

func TestCanWritePath_yolo_allows_workspace_writes_in_plan_mode(t *testing.T) {
	workDir := "/tmp/proj"
	if !permission.CanWritePath(types.PermissionPlan, "/tmp/proj/plan.md", "/tmp/proj/other.md", workDir, true) {
		t.Fatal("YOLO should allow workspace writes in plan mode")
	}
	if permission.CanWritePath(types.PermissionPlan, "/tmp/proj/plan.md", "/etc/passwd", workDir, true) {
		t.Fatal("YOLO must not allow writes outside workspace")
	}
}
