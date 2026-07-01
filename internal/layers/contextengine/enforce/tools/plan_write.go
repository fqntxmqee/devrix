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

// EnforcePlanModeBash blocks arbitrary shell execution while plan mode is
// active. Unlike write_file/edit_file, bash can hide writes behind redirects,
// subprocesses, or interpreter snippets; without a complete write-target
// extractor it must fail closed unless the session explicitly enables YOLO
// workspace writes.
func EnforcePlanModeBash(ctx context.Context) *ToolResult {
	sc := ToolSessionContextFromContext(ctx)
	if sc == nil || !sc.PermissionMode.IsPlanMode() || FilesAutoApprovedFromContext(ctx) {
		return nil
	}
	return &ToolResult{
		Error: fmt.Sprintf("plan mode: bash denied. %s", permission.PlanModeWriteHint()),
	}
}
