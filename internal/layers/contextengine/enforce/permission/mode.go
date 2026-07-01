package permission

import (
	"path/filepath"
	"strings"

	"github.com/devrix/devrix/internal/shared/types"
)

// EnterPlan sets permission mode to plan and records previous mode.
func EnterPlan(sc *types.SessionContext, planFilePath string) {
	if sc == nil {
		return
	}
	if sc.PermissionMode != types.PermissionPlan {
		sc.PrePlanMode = sc.PermissionMode
	}
	sc.PermissionMode = types.PermissionPlan
	if planFilePath != "" {
		sc.PlanFilePath = planFilePath
	}
}

// ExitPlan restores pre-plan mode.
func ExitPlan(sc *types.SessionContext) types.PermissionMode {
	if sc == nil {
		return types.PermissionDefault
	}
	prev := sc.PrePlanMode
	if prev == "" {
		prev = types.PermissionDefault
	}
	sc.PermissionMode = prev
	sc.PrePlanMode = ""
	return prev
}

// PlanModeWriteHint explains plan-mode write restrictions vs YOLO.
func PlanModeWriteHint() string {
	return "Plan mode restricts writes to the plan file unless YOLO file auto-approve is enabled."
}

// CanWritePath reports whether plan mode allows writing to path.
func CanWritePath(mode types.PermissionMode, planFilePath, targetPath, workDir string, filesAutoApproved bool) bool {
	if mode != types.PermissionPlan {
		return true
	}
	if filesAutoApproved && isPathUnderWorkDir(workDir, targetPath) {
		return true
	}
	if planFilePath == "" || targetPath == "" {
		return false
	}
	absPlan, err1 := filepath.Abs(planFilePath)
	absTarget, err2 := filepath.Abs(targetPath)
	if err1 != nil || err2 != nil {
		return filepath.Clean(planFilePath) == filepath.Clean(targetPath)
	}
	return absPlan == absTarget
}

func isPathUnderWorkDir(workDir, targetPath string) bool {
	workDir = filepath.Clean(workDir)
	targetPath = filepath.Clean(targetPath)
	if workDir == "" || targetPath == "" {
		return false
	}
	absWork, err1 := filepath.Abs(workDir)
	absTarget, err2 := filepath.Abs(targetPath)
	if err1 != nil || err2 != nil {
		rel, err := filepath.Rel(workDir, targetPath)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	rel, err := filepath.Rel(absWork, absTarget)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// IsPlanModeAllowedToolName reports whether a tool name is allowed in plan mode.
func IsPlanModeAllowedToolName(name, planFilePath string) bool {
	switch name {
	case "read_file", "glob", "grep", "list_dir", "task_list", "task_get",
		"enter_plan_mode", "exit_plan_mode", "todo_write", "task_create", "task_update",
		"edit_file", "agent", "call_agent":
		return true
	case "write_file":
		return planFilePath != ""
	default:
		return len(name) > 5 && (name[:5] == "task_" || name[:5] == "read_")
	}
}
