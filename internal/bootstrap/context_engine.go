package bootstrap

import (
	"log/slog"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionqueue"
	"github.com/devrix/devrix/internal/layers/orchestration/toolpolicy"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"

	// tool package is imported via the agentToolReg parameter; the bridge plugin
	// is created here at the composition root.
	"github.com/devrix/devrix/internal/layers/multiagent/tool"
)

// NewContextEngine wires Layer 2 to a pre-built LLM stack (L3 bridge).
// agentToolReg is nil when agent tools are disabled.
func NewContextEngine(
	stack llmbridge.ContextLLMStack,
	permMgr *capture.PermissionManager,
	ctxCfg *config.ContextEngineConfig,
	toolCfg *config.ToolConfig,
	obsBridge *observability.Bridge,
	agentToolReg *tool.Registry,
) *contextengine.ContextEngine {
	longTerm := WireContextV3(ctxCfg)
	workmodel.InitGlobalTaskManager(ctxCfg.Tasks, obsBridge)
	toolReg, err := contextengine.NewBuiltinToolRegistry(toolCfg)
	if err != nil {
		slog.Error("create builtin tool registry", "error", err)
		toolReg = contextengine.NewToolRegistry()
	}
	if err := contextengine.RegisterQueryLoopTools(toolReg, ctxCfg); err != nil {
		slog.Error("register query loop tools", "error", err)
	}
	if err := workmodel.RegisterTaskTools(toolReg, ctxCfg, workmodel.GlobalTaskManager); err != nil {
		slog.Error("register task tools", "error", err)
	}
	if err := contextengine.RegisterBackgroundTaskTools(toolReg); err != nil {
		slog.Error("register background task tools", "error", err)
	}

	// Register per-agent call_<name> plugins if agent tools are enabled.
	if agentToolReg != nil {
		plugins := newAgentToolPlugins(agentToolReg, obsBridge)
		for _, plugin := range plugins {
			if err := toolReg.Register(plugin); err != nil {
				slog.Error("register agent plugin", "tool", plugin.Name(), "error", err)
			} else {
				slog.Info("agent tool registered", "tool", plugin.Name())
			}
		}
	}

	tools := contextengine.NewLimitedToolRunner(
		toolReg,
		contextengine.NewToolLimiter(toolCfg.ConcurrentMax),
	)
	return contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:                 stack.Gateway,
		TokenCounter:        stack.TokenCounter,
		Tools:               tools,
		ToolsReg:            toolReg,
		Permission:          capture.NewPermissionGateAdapter(permMgr),
		Observer:            contextengine.NoOpObserver{},
		LongTerm:            longTerm,
		Config:              ctxCfg,
		ObsBridge:           obsBridge,
		DefaultModel:        stack.DefaultModel,
		TierResolver:        stack.TierResolver,
		SessionCommandQueue: sessionqueue.GlobalSessionQueue,
		AgentRoleToolFilter: toolpolicy.NewFilter(),
	})
}
