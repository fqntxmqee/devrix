package bootstrap

import (
	"context"
	"log/slog"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/tool"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

type permissionGateFunc func(ctx context.Context, sessionID, toolName, input string, risk types.RiskLevel) bool

func (f permissionGateFunc) Request(ctx context.Context, sessionID, toolName, input string, risk types.RiskLevel) bool {
	return f(ctx, sessionID, toolName, input, risk)
}

// ContextEngineBuilder builds Layer 2 engines with a custom permission gate (per Agent).
type ContextEngineBuilder struct {
	stack        llmbridge.ContextLLMStack
	ctxCfg       *config.ContextEngineConfig
	toolCfg      *config.ToolConfig
	obsBridge    *observability.Bridge
	milestoneSvc milestone.IMilestoneService
	agentToolReg *tool.Registry
}

// NewContextEngineBuilder creates a reusable engine builder.
func NewContextEngineBuilder(
	stack llmbridge.ContextLLMStack,
	ctxCfg *config.ContextEngineConfig,
	toolCfg *config.ToolConfig,
	obsBridge *observability.Bridge,
	milestoneSvc milestone.IMilestoneService,
	agentToolReg *tool.Registry,
) *ContextEngineBuilder {
	return &ContextEngineBuilder{
		stack:        stack,
		ctxCfg:       ctxCfg,
		toolCfg:      toolCfg,
		obsBridge:    obsBridge,
		milestoneSvc: milestoneSvc,
		agentToolReg: agentToolReg,
	}
}

// Build returns a context engine using the agent permission gate.
func (b *ContextEngineBuilder) Build(perm multiagent.PermissionGate) contracts.IEngine {
	var gate contextengine.IPermissionGate
	if perm != nil {
		gate = permissionGateFunc(perm.Request)
	}
	return b.buildWithGate(gate)
}

func (b *ContextEngineBuilder) buildWithGate(perm contextengine.IPermissionGate) contracts.IEngine {
	if b == nil {
		return nil
	}
	planner, longTerm := WireContextV3(b.ctxCfg, b.milestoneSvc)
	toolReg := contextengine.NewBuiltinToolRegistry(b.toolCfg)

	if b.agentToolReg != nil {
		plugin := newAgentToolPlugin(b.agentToolReg)
		if err := toolReg.Register(plugin); err != nil {
			slog.Error("register call_agent plugin", "error", err)
		} else {
			slog.Info("call_agent tool registered", "tools", len(b.agentToolReg.List()))
		}
	}

	tools := contextengine.NewLimitedToolRunner(
		toolReg,
		contextengine.NewToolLimiter(b.toolCfg.ConcurrentMax),
	)
	if perm == nil {
		perm = gateway.NewPermissionGateAdapter(nil)
	}
	return contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:          b.stack.Gateway,
		TokenCounter: b.stack.TokenCounter,
		Tools:        tools,
		ToolsReg:     toolReg,
		Permission:   perm,
		Observer:     contextengine.NoOpObserver{},
		Planner:      planner,
		LongTerm:     longTerm,
		Config:       b.ctxCfg,
		ObsBridge:    b.obsBridge,
	})
}
