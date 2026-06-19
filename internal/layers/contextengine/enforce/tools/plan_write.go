package tools

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/permission"
)

// EnforcePlanModeWrite blocks writes outside the plan file when plan mode is active.
func EnforcePlanModeWrite(ctx context.Context, targetPath string) *ToolResult {
	sc := ToolSessionContextFromContext(ctx)
	if sc == nil || !sc.PermissionMode.IsPlanMode() {
		return nil
	}
	workDir := ToolWorkDirFromContext(ctx)
	resolved := targetPath
	if workDir != "" && !filepath.IsAbs(resolved) {
		resolved = filepath.Join(workDir, resolved)
	}
	if !permission.CanWritePath(
		sc.PermissionMode, sc.PlanFilePath, resolved, workDir, FilesAutoApprovedFromContext(ctx),
	) {
		return &ToolResult{
			Error: fmt.Sprintf(
				"plan mode: write denied for %s (allowed plan file: %s). %s",
				targetPath, sc.PlanFilePath, permission.PlanModeWriteHint(),
			),
		}
	}
	return nil
}
