package bootstrap

import (
	"context"
	"log/slog"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/external"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/delegatetools"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionqueue"
	"github.com/devrix/devrix/internal/layers/orchestration/toolpolicy"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
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
	maCfg        *config.MultiAgentConfig
	obsBridge    *observability.Bridge
	agentToolReg *external.Registry
}

// NewContextEngineBuilder creates a reusable engine builder.
func NewContextEngineBuilder(
	stack llmbridge.ContextLLMStack,
	ctxCfg *config.ContextEngineConfig,
	toolCfg *config.ToolConfig,
	obsBridge *observability.Bridge,
	agentToolReg *external.Registry,
) *ContextEngineBuilder {
	return &ContextEngineBuilder{
		stack:        stack,
		ctxCfg:       ctxCfg,
		toolCfg:      toolCfg,
		obsBridge:    obsBridge,
		agentToolReg: agentToolReg,
	}
}

// WithMultiAgentConfig enables delegate tool registration on per-agent engines.
func (b *ContextEngineBuilder) WithMultiAgentConfig(maCfg *config.MultiAgentConfig) *ContextEngineBuilder {
	if b != nil {
		b.maCfg = maCfg
	}
	return b
}

// Build returns a context engine using the agent permission gate.
func (b *ContextEngineBuilder) Build(perm multiagent.PermissionGate) contracts.IEngine {
	var gate contracts.IPermissionGate
	if perm != nil {
		gate = permissionGateFunc(perm.Request)
	}
	return b.buildWithGate(gate)
}

func (b *ContextEngineBuilder) buildWithGate(perm contracts.IPermissionGate) contracts.IEngine {
	if b == nil {
		return nil
	}
	longTerm := WireContextV3(b.ctxCfg)
	workmodel.InitGlobalTaskManager(b.ctxCfg.Tasks, b.obsBridge)
	toolReg, err := contextengine.NewBuiltinToolRegistry(b.toolCfg)
	if err != nil {
		slog.Error("create builtin tool registry", "error", err)
		toolReg = contextengine.NewToolRegistry()
	}
	if err := contextengine.RegisterQueryLoopTools(toolReg, b.ctxCfg); err != nil {
		slog.Error("register query loop tools", "error", err)
	}
	if err := workmodel.RegisterTaskTools(toolReg, b.ctxCfg, workmodel.GlobalTaskManager); err != nil {
		slog.Error("register task tools", "error", err)
	}
	if err := contextengine.RegisterBackgroundTaskTools(toolReg); err != nil {
		slog.Error("register background task tools", "error", err)
	}
	if b.ctxCfg.TodoWrite.Enabled {
		if err := toolReg.Register(contextengine.NewTodoWriteRunner()); err != nil {
			slog.Error("register todo_write", "error", err)
		}
	}
	if b.maCfg != nil && b.maCfg.Delegate.Enabled {
		if err := delegatetools.RegisterTools(toolReg, b.maCfg); err != nil {
			slog.Error("register delegate tools", "error", err)
		}
	}

	if b.agentToolReg != nil {
		plugins := newAgentToolPlugins(b.agentToolReg, b.obsBridge)
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
		contextengine.NewToolLimiter(b.toolCfg.ConcurrentMax),
	)
	if perm == nil {
		perm = capture.NewPermissionGateAdapter(nil)
	}
	return contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:                 b.stack.Gateway,
		TokenCounter:        b.stack.TokenCounter,
		Tools:               tools,
		ToolsReg:            toolReg,
		Permission:          perm,
		Observer:            contextengine.NoOpObserver{},
		LongTerm:            longTerm,
		Config:              b.ctxCfg,
		ObsBridge:           b.obsBridge,
		DefaultModel:        b.stack.DefaultModel,
		TierResolver:        b.stack.TierResolver,
		SessionCommandQueue: sessionqueue.GlobalSessionQueue,
		AgentRoleToolFilter: toolpolicy.NewFilter(),
	})
}
