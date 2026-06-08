package bootstrap

import (
	"log/slog"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"

	// tool package is imported via the agentToolReg parameter; the bridge plugin
	// is created here at the composition root.
	"github.com/devrix/devrix/internal/layers/multiagent/tool"
)

// NewContextEngine wires Layer 2 to a pre-built LLM stack (L3 bridge).
// agentToolReg is nil when agent tools are disabled.
func NewContextEngine(
	stack llmbridge.ContextLLMStack,
	permMgr *gateway.PermissionManager,
	ctxCfg *config.ContextEngineConfig,
	toolCfg *config.ToolConfig,
	obsBridge *observability.Bridge,
	milestoneSvc milestone.IMilestoneService,
	agentToolReg *tool.Registry,
) *contextengine.ContextEngine {
	planner, longTerm := WireContextV3(ctxCfg, milestoneSvc)
	toolReg := contextengine.NewBuiltinToolRegistry(toolCfg)

	// Register call_agent bridge if agent tools are enabled.
	if agentToolReg != nil {
		plugin := newAgentToolPlugin(agentToolReg)
		if err := toolReg.Register(plugin); err != nil {
			slog.Error("register call_agent plugin", "error", err)
		} else {
			slog.Info("call_agent tool registered", "tools", len(agentToolReg.List()))
		}
	}

	tools := contextengine.NewLimitedToolRunner(
		toolReg,
		contextengine.NewToolLimiter(toolCfg.ConcurrentMax),
	)
	return contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:          stack.Gateway,
		TokenCounter: stack.TokenCounter,
		Tools:        tools,
		ToolsReg:     toolReg,
		Permission:   gateway.NewPermissionGateAdapter(permMgr),
		Observer:     contextengine.NoOpObserver{},
		Planner:      planner,
		LongTerm:     longTerm,
		Config:       ctxCfg,
		ObsBridge:    obsBridge,
	})
}
