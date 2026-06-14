package contextengine

import (
	"github.com/devrix/devrix/internal/shared/types"
)

// AgentRoleToolFilter applies leader/worker tool visibility (D7 policy, wired in bootstrap).
type AgentRoleToolFilter interface {
	Filter(sc *types.SessionContext, tools []ToolSchema) []ToolSchema
}
