// Package toolpolicy — D7 worker/leader tool visibility (DM-20260614-015).
//
// DSAFT: D7-S5 F — hides delegate_* from workers and constrains explore/plan read-only sets.
package toolpolicy

import (
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/types"
)

var delegateToolNames = map[string]bool{
	"delegate_explore":   true,
	"delegate_plan":      true,
	"delegate_implement": true,
	"delegate_status":    true,
	"task_spawn":         true,
}

// Filter implements contextengine.AgentRoleToolFilter.
type Filter struct{}

// NewFilter returns the default agent-role tool filter.
func NewFilter() *Filter {
	return &Filter{}
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
func (f *Filter) Filter(sc *types.SessionContext, ts []tools.ToolSchema) []tools.ToolSchema {
	if f == nil {
		return ts
	}
	return FilterToolsForAgentRole(sc, ts)
}

func filterReadOnlyWorkerTools(ts []tools.ToolSchema) []tools.ToolSchema {
	allowed := map[string]bool{
		"read_file": true, "glob": true, "grep": true, "list_dir": true, "bash": true,
		"enter_plan_mode": true, "exit_plan_mode": true,
		"todo_write":      true,
		"task_write":      true,
		"task_list":       true,
		"task_await":      true,
		"edit_file":       true,
		"task_create":     true, "task_get": true, "task_update": true,
	}
	out := make([]tools.ToolSchema, 0, len(ts))
	for _, t := range ts {
		name := strings.ToLower(t.Name)
		if allowed[name] {
			out = append(out, t)
		}
	}
	return out
}
