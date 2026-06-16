package enforce

import (
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/shared/types"
)

// AgentRoleToolFilter applies leader/worker tool visibility (D7 policy, wired in bootstrap).
//
// DSAFT: D2-S3-A02 (FilterTools)
type AgentRoleToolFilter interface {
	Filter(sc *types.SessionContext, tools []toolrunner.ToolSchema) []toolrunner.ToolSchema
}
