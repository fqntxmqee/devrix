package bootstrap

import (
	"log/slog"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionqueue"
	"github.com/devrix/devrix/internal/layers/orchestration/toolpolicy"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"

	// tool package is imported via the agentToolReg parameter; the bridge plugin
	// is created here at the composition root.
	"github.com/devrix/devrix/internal/layers/multiagent/external"
)

// NewContextEngine wires Layer 2 to a pre-built LLM stack (L3 bridge).
// agentToolReg is nil when agent tools are disabled.
//
// DM-020: D2→D3 拆面 is enforced by constructing D7 turn adapters from llmStack
// and injecting them via EngineDeps.QueryLLMCaller / EngineDeps.Summarizer.
// EngineDeps.LLM is left nil so production wiring cannot fall through to the
// deprecated D2→D3 direct path.
func NewContextEngine(
	stack llmbridge.ContextLLMStack,
	permMgr *capture.PermissionManager,
	ctxCfg *config.ContextEngineConfig,
	toolCfg *config.ToolConfig,
	obsBridge *observability.Bridge,
	agentToolReg *external.Registry,
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

	// DM-020: D7-supplied adapters for the two D2 LLM拆面 contracts.
	queryCaller := turn.NewQueryLLMCaller(turn.QueryLLMCallerDeps{
		Gateway:      stack.RawGateway,
		TierResolver: stack.TierResolver,
		DefaultTier:  stack.DefaultModel,
	})
	summarizer := turn.NewCompressionSummarizer(turn.CompressionSummarizerDeps{
		Gateway:      stack.RawGateway,
		TierResolver: stack.TierResolver,
		DefaultTier:  stack.DefaultModel,
		Timeout:      ctxCfg.Compression.Autocompact.Timeout,
	})

	return contextengine.NewContextEngine(contextengine.EngineDeps{
		// LLM deliberately omitted — production must go through DM-020 拆面.
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
		QueryLLMCaller:      queryCaller,
		Summarizer:          summarizer,
	})
}

// Compile-time assertion that the adapters implement the D2拆面 contracts.
var (
	_ contracts.LLMCaller = (*turn.QueryLLMCaller)(nil)
	_ contracts.Summarizer = (*turn.CompressionSummarizer)(nil)
)
