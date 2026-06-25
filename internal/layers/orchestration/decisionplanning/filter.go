// Package decisionplanning — D7 worker/leader tool visibility (DM-20260614-015).
//
// DSAFT: D7-S5 F — hides delegate_* from workers and constrains explore/plan read-only sets.
package decisionplanning

import (
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/types"
)

// DelegateToolNames is the set of tool names hidden from workers/sub-agents.
// Exported so filter_adapter.go can reuse the same set for contracts.ToolSpec.
var DelegateToolNames = map[string]bool{
	"delegate_explore":   true,
	"delegate_plan":      true,
	"delegate_implement": true,
	"delegate_status":    true,
	"task_spawn":         true,
}

// readOnlyWorkerTools is the allowed tool set for explore/plan workers.
// Lowercased; matched case-insensitively against tool.Name.
var readOnlyWorkerTools = map[string]bool{
	"read_file":       true,
	"glob":            true,
	"grep":            true,
	"list_dir":        true,
	"bash":            true,
	"enter_plan_mode": true,
	"exit_plan_mode":  true,
	"todo_write":      true,
	"task_write":      true,
	"task_list":       true,
	"task_await":      true,
	"edit_file":       true,
	"task_create":     true,
	"task_get":        true,
	"task_update":     true,
}

// AsAgentRoleToolFilter adapts FilterToolsForAgentRole to the
// enforce.AgentRoleToolFilter interface so bootstrap can inject the policy
// without an empty Filter wrapper struct.
func AsAgentRoleToolFilter() enforce.AgentRoleToolFilter {
	return agentRoleFilterFunc(FilterToolsForAgentRole)
}

type agentRoleFilterFunc func(sc *types.SessionContext, ts []tools.ToolSchema) []tools.ToolSchema

// Filter implements enforce.AgentRoleToolFilter.
func (f agentRoleFilterFunc) Filter(sc *types.SessionContext, ts []tools.ToolSchema) []tools.ToolSchema {
	return f(sc, ts)
}

// FilterToolsForAgentRole hides delegate tools from workers and sub-agents.
func FilterToolsForAgentRole(sc *types.SessionContext, ts []tools.ToolSchema) []tools.ToolSchema {
	if sc == nil || len(ts) == 0 {
		return ts
	}
	isLeaderMain := sc.AgentID == "" && !sc.IsWorker
	if isLeaderMain {
		return ts
	}
	out := make([]tools.ToolSchema, 0, len(ts))
	for _, t := range ts {
		if DelegateToolNames[t.Name] {
			continue
		}
		out = append(out, t)
	}
	if !sc.IsWorker {
		return out
	}
	if sc.WorkerRole == "explore" || sc.WorkerRole == "plan" {
		return filterReadOnlyWorkerTools(out)
	}
	return out
}

func filterReadOnlyWorkerTools(ts []tools.ToolSchema) []tools.ToolSchema {
	out := make([]tools.ToolSchema, 0, len(ts))
	for _, t := range ts {
		name := strings.ToLower(t.Name)
		if readOnlyWorkerTools[name] {
			out = append(out, t)
		}
	}
	return out
}
