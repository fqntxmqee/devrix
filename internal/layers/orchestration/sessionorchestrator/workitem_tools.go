package sessionorchestrator

// pipelineBlockedTools are unavailable during automated WorkItem pipeline
// execution (RunSessionTurnLoop → ItemPipelineRunner). Interactive tools
// belong to the user-facing Turn loop, not autonomous MUPS rounds.
var pipelineBlockedTools = map[string]struct{}{
	"ask_user_question": {},
}

func filterPipelineTools(tools []ToolSchema) []ToolSchema {
	if len(tools) == 0 {
		return tools
	}
	out := make([]ToolSchema, 0, len(tools))
	for _, t := range tools {
		if _, blocked := pipelineBlockedTools[t.Name]; blocked {
			continue
		}
		out = append(out, t)
	}
	return out
}
