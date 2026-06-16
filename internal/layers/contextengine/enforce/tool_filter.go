package enforce

import (
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/permission"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/shared/types"
)

// FilterToolsByPermissionMode returns tools visible under the current permission mode.
//
// DSAFT: D2-S3-A02 (FilterTools)
func FilterToolsByPermissionMode(mode types.PermissionMode, tools []toolrunner.ToolSchema, planFilePath string) []toolrunner.ToolSchema {
	if mode != types.PermissionPlan {
		return tools
	}
	out := make([]toolrunner.ToolSchema, 0, len(tools))
	for _, t := range tools {
		if permission.IsPlanModeAllowedToolName(t.Name, planFilePath) {
			out = append(out, t)
		}
	}
	return out
}
