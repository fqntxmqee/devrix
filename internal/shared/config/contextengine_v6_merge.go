package config

// mergeContextEngineV6 merges turn-runtime v1/v2 fields from file config over defaults.
func mergeContextEngineV6(base, file ContextEngineConfig) ContextEngineConfig {
	out := base
	out.TurnRuntime = mergeTurnRuntimeConfig(out.TurnRuntime, file.TurnRuntime)
	out.TurnRuntime = mergeTurnRuntimeConfig(out.TurnRuntime, file.QueryLoop)
	out.UserContext = mergeUserContextConfig(out.UserContext, file.UserContext)
	out.Attachments = mergeAttachmentsConfig(out.Attachments, file.Attachments)
	out.Permission = mergeContextPermissionConfig(out.Permission, file.Permission)
	out.Tasks = mergeTasksConfig(out.Tasks, file.Tasks)
	out.SubQuery = mergeSubQueryConfig(out.SubQuery, file.SubQuery)
	out.ExecutionFlow = mergeExecutionFlowConfig(out.ExecutionFlow, file.ExecutionFlow)
	out.Sandbox = mergeSandboxConfig(out.Sandbox, file.Sandbox)
	out.Sandbox = mergeSandboxConfig(out.Sandbox, file.Worktree)
	return out
}

func mergeTurnRuntimeConfig(base, override TurnRuntimeConfig) TurnRuntimeConfig {
	out := base
	if override.MaxTurns != 0 {
		out.MaxTurns = override.MaxTurns
	}
	if override.CompressPerTurn {
		out.CompressPerTurn = true
	}
	if override.StreamingTools {
		out.StreamingTools = true
	}
	return out
}

func mergeUserContextConfig(base, override UserContextConfig) UserContextConfig {
	out := base
	if override.Mode != "" {
		out.Mode = override.Mode
	}
	return out
}

func mergeAttachmentsConfig(base, override AttachmentsConfig) AttachmentsConfig {
	out := base
	if override.Enabled {
		out.Enabled = true
	}
	if !override.Enabled && override.PlanModeFullEvery != 0 {
		out.Enabled = false
	}
	if override.PlanModeFullEvery != 0 {
		out.PlanModeFullEvery = override.PlanModeFullEvery
	}
	return out
}

func mergeContextPermissionConfig(base, override ContextPermissionConfig) ContextPermissionConfig {
	out := base
	if override.DefaultMode != "" {
		out.DefaultMode = override.DefaultMode
	}
	if override.Plan.ExploreAgentCount != 0 {
		out.Plan.ExploreAgentCount = override.Plan.ExploreAgentCount
	}
	if override.Plan.PlanAgentCount != 0 {
		out.Plan.PlanAgentCount = override.Plan.PlanAgentCount
	}
	if override.Plan.PlanFileDir != "" {
		out.Plan.PlanFileDir = override.Plan.PlanFileDir
	}
	return out
}

func mergeTasksConfig(base, override TasksConfig) TasksConfig {
	out := base
	if override.Mode != "" {
		out.Mode = override.Mode
	}
	if override.StoreDir != "" {
		out.StoreDir = override.StoreDir
	}
	return out
}

func mergeSubQueryConfig(base, override SubQueryConfig) SubQueryConfig {
	out := base
	if override.ForkSubagentEnabled {
		out.ForkSubagentEnabled = true
	}
	if override.SidechainTranscript {
		out.SidechainTranscript = true
	}
	if !override.SidechainTranscript && (override.ForkSubagentEnabled || override.DefaultSubagentMaxTurns != 0) {
		out.SidechainTranscript = override.SidechainTranscript
	}
	if override.DefaultSubagentMaxTurns != 0 {
		out.DefaultSubagentMaxTurns = override.DefaultSubagentMaxTurns
	}
	return out
}

func mergeExecutionFlowConfig(base, override ExecutionFlowConfig) ExecutionFlowConfig {
	out := base
	if override.Enabled {
		out.Enabled = true
	}
	if override.LinkTasks {
		out.LinkTasks = true
	}
	if !override.LinkTasks && override.Enabled {
		out.LinkTasks = override.LinkTasks
	}
	if override.IMProgress {
		out.IMProgress = true
	}
	if !override.IMProgress && override.Enabled {
		out.IMProgress = override.IMProgress
	}
	if override.ToolSummaryThrottleMs != 0 {
		out.ToolSummaryThrottleMs = override.ToolSummaryThrottleMs
	}
	if override.EventBufferSize != 0 {
		out.EventBufferSize = override.EventBufferSize
	}
	return out
}

func mergeSandboxConfig(base, override SandboxConfig) SandboxConfig {
	out := base
	if override.Enabled {
		out.Enabled = true
	}
	if override.BaseDir != "" {
		out.BaseDir = override.BaseDir
	}
	return out
}
