package bootstrap

import (
	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
)

// NewContextEngine wires Layer 2 to a pre-built LLM stack (L3 bridge).
func NewContextEngine(
	stack llmbridge.ContextLLMStack,
	permMgr *gateway.PermissionManager,
	ctxCfg *config.ContextEngineConfig,
	toolCfg *config.ToolConfig,
	obsBridge *observability.Bridge,
	milestoneSvc milestone.IMilestoneService,
) *contextengine.ContextEngine {
	planner, longTerm := WireContextV3(ctxCfg, milestoneSvc)
	toolReg := contextengine.NewBuiltinToolRegistry(toolCfg)
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
