package contextengine

import (
	"github.com/devrix/devrix/internal/layers/contextengine/policy/permission"
	"github.com/devrix/devrix/internal/shared/types"
)

// FilterToolsByPermissionMode returns tools visible under the current permission mode.
func FilterToolsByPermissionMode(mode types.PermissionMode, tools []ToolSchema, planFilePath string) []ToolSchema {
	if mode != types.PermissionPlan {
		return tools
	}
	out := make([]ToolSchema, 0, len(tools))
	for _, t := range tools {
		if permission.IsPlanModeAllowedToolName(t.Name, planFilePath) {
			out = append(out, t)
		}
	}
	return out
}
