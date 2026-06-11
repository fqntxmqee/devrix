package contextengine

import (
	"strings"

	"github.com/devrix/devrix/internal/shared/types"
)

var delegateToolNames = map[string]bool{
	"delegate_explore":   true,
	"delegate_plan":      true,
	"delegate_implement": true,
	"delegate_status":    true,
}

// FilterToolsForAgentRole hides delegate tools from workers and sub-agents.
func FilterToolsForAgentRole(sc *types.SessionContext, tools []ToolSchema) []ToolSchema {
	if sc == nil || len(tools) == 0 {
		return tools
	}
	isLeaderMain := sc.AgentID == "" && !sc.IsWorker
	if isLeaderMain {
		return tools
	}
	out := make([]ToolSchema, 0, len(tools))
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

func filterReadOnlyWorkerTools(tools []ToolSchema) []ToolSchema {
	allowed := map[string]bool{
		"read_file": true, "glob": true, "grep": true, "list_dir": true, "bash": true,
		"enter_plan_mode": true, "exit_plan_mode": true,
		"task_create": true, "task_get": true, "task_list": true, "task_update": true,
	}
	out := make([]ToolSchema, 0, len(tools))
	for _, t := range tools {
		name := strings.ToLower(t.Name)
		if allowed[name] {
			out = append(out, t)
		}
	}
	return out
}
