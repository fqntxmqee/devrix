package enforce

import (
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/permission"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/types"
)

// FilterToolsByPermissionMode returns tools visible under the current permission mode.
//
// DSAFT: D2-S3-A02 (FilterTools)
func FilterToolsByPermissionMode(mode types.PermissionMode, ts []tools.ToolSchema, planFilePath string) []tools.ToolSchema {
	if mode != types.PermissionPlan {
		return ts
	}
	out := make([]tools.ToolSchema, 0, len(ts))
	for _, t := range ts {
		if permission.IsPlanModeAllowedToolName(t.Name, planFilePath) {
			out = append(out, t)
		}
	}
	return out
}
