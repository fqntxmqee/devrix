// Package toolpolicy — D7 worker/leader tool visibility (DM-20260614-015).
//
// DSAFT: D7-S5 F — hides delegate_* from workers and constrains explore/plan read-only sets.
package toolpolicy

import (
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/shared/types"
)

var delegateToolNames = map[string]bool{
	"delegate_explore":   true,
	"delegate_plan":      true,
	"delegate_implement": true,
	"delegate_status":    true,
}

// Filter implements contextengine.AgentRoleToolFilter.
type Filter struct{}

// NewFilter returns the default agent-role tool filter.
func NewFilter() *Filter {
	return &Filter{}
}

// FilterToolsForAgentRole hides delegate tools from workers and sub-agents.
func FilterToolsForAgentRole(sc *types.SessionContext, tools []toolrunner.ToolSchema) []toolrunner.ToolSchema {
	if sc == nil || len(tools) == 0 {
		return tools
	}
	isLeaderMain := sc.AgentID == "" && !sc.IsWorker
	if isLeaderMain {
		return tools
	}
	out := make([]toolrunner.ToolSchema, 0, len(tools))
	for _, t := range tools {
		if delegateToolNames[t.Name] {
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

// Filter satisfies contextengine.AgentRoleToolFilter using D2 ToolSchema aliases.
func (f *Filter) Filter(sc *types.SessionContext, tools []toolrunner.ToolSchema) []toolrunner.ToolSchema {
	if f == nil {
		return tools
	}
	return FilterToolsForAgentRole(sc, tools)
}

func filterReadOnlyWorkerTools(tools []toolrunner.ToolSchema) []toolrunner.ToolSchema {
	allowed := map[string]bool{
		"read_file": true, "glob": true, "grep": true, "list_dir": true, "bash": true,
		"enter_plan_mode": true, "exit_plan_mode": true,
		"todo_write":      true,
		"edit_file":       true,
		"task_create":     true, "task_get": true, "task_list": true, "task_update": true,
	}
	out := make([]toolrunner.ToolSchema, 0, len(tools))
	for _, t := range tools {
		name := strings.ToLower(t.Name)
		if allowed[name] {
			out = append(out, t)
		}
	}
	return out
}
