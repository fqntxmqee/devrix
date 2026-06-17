package bootstrap

import (
	"context"
	"log/slog"
	"time"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/external"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
	"github.com/devrix/devrix/internal/layers/orchestration/delegatetools"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionqueue"
	"github.com/devrix/devrix/internal/layers/orchestration/toolpolicy"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
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
	if err := enforce.RegisterQueryLoopTools(toolReg, b.ctxCfg); err != nil {
		slog.Error("register query loop tools", "error", err)
	}
	if err := workmodel.RegisterTaskTools(toolReg, b.ctxCfg, workmodel.GlobalTaskManager); err != nil {
		slog.Error("register task tools", "error", err)
	}
	if err := enforce.RegisterBackgroundTaskTools(toolReg); err != nil {
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

	// G1 LSP tool (default disabled; 启用需 lsp.enabled=true + servers 配置)
	if err := toolrunner.RegisterLSPTool(toolReg, nil); err != nil {
		slog.Error("register lsp tool", "error", err)
	}

	// DM-20260617-002 W6 (AC4): G4 verify_plan_execution tool — 暴露给 LLM。
	if err := toolrunner.RegisterVerifyTool(toolReg); err != nil {
		slog.Error("register verify_plan_execution tool", "error", err)
	}

	// DM-20260617-002 W7 (AC5): G5 free_fork tool — 通过 freefork.GlobalForker 注入。
	if err := toolrunner.RegisterFreeForkTool(toolReg); err != nil {
		slog.Error("register free_fork tool", "error", err)
	}

	// DM-20260617-002 W8 (AC6): G6 query_diagnostics tool — 通过 tracker.GlobalTracker 注入。
	// 同一 buildWithGate 中创建 tracker 实例 + SetGlobalTracker + 启动 1s 间隔 tick goroutine。
	diagTracker := tracker.New(0)
	tracker.SetGlobalTracker(diagTracker)
	startTrackerTick(diagTracker, 1*time.Second)
	if err := toolrunner.RegisterTrackerTool(toolReg); err != nil {
		slog.Error("register query_diagnostics tool", "error", err)
	}

	tools := contextengine.NewLimitedToolRunner(
		toolReg,
		contextengine.NewToolLimiter(b.toolCfg.ConcurrentMax),
	)
	if perm == nil {
		perm = capture.NewPermissionGateAdapter(nil)
	}
	queryCaller := turn.NewQueryLLMCaller(turn.QueryLLMCallerDeps{
		Gateway:      b.stack.RawGateway,
		TierResolver: b.stack.TierResolver,
		DefaultTier:  b.stack.DefaultModel,
	})
	summarizer := turn.NewCompressionSummarizer(turn.CompressionSummarizerDeps{
		Gateway:      b.stack.RawGateway,
		TierResolver: b.stack.TierResolver,
		DefaultTier:  b.stack.DefaultModel,
		Timeout:      b.ctxCfg.Compression.Autocompact.Timeout,
	})

	return contextengine.NewContextEngine(contextengine.EngineDeps{
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
		AgentRoleToolFilter: toolpolicy.NewFilter(),
		QueryLLMCaller:      queryCaller,
		Summarizer:          summarizer,
		SessionCommandQueue: sessionqueue.GlobalSessionQueue,
	})
}

// startTrackerTick 在后台 goroutine 中按 interval 调 TickOnce,跟进程同生命周期。
// DM-20260617-002 W8: query_diagnostics tool 的"异步 tick"实现。linter 不可用时
// TickOnce 内部静默返回 0,不输出日志（避免噪音）。
func startTrackerTick(tr *tracker.Tracker, interval time.Duration) {
	if tr == nil || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			tr.TickOnce(context.Background())
		}
	}()
}
