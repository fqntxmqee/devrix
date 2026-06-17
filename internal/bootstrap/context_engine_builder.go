package bootstrap

import (
	"context"
	"log/slog"
	"time"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/external"
	"github.com/devrix/devrix/internal/layers/multiagent/provision/freefork"
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

// CheckPermission is a Risk-only fallback for the legacy
// permissionGateFunc adapter. Plan-mode OpenWorld denial is composed
// at the turn_adapter layer (DM-20260618-002), not here.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
func (f permissionGateFunc) CheckPermission(_ context.Context, spec contracts.ToolSpec) contracts.Decision {
	switch spec.Risk {
	case types.RiskLevelLow:
		return contracts.DecisionAllow
	default:
		return contracts.DecisionAsk
	}
}

// ContextEngineBuilder builds Layer 2 engines with a custom permission gate (per Agent).
type ContextEngineBuilder struct {
	stack        llmbridge.ContextLLMStack
	ctxCfg       *config.ContextEngineConfig
	toolCfg      *config.ToolConfig
	maCfg        *config.MultiAgentConfig
	obsBridge    *observability.Bridge
	agentToolReg *external.Registry
	// forker is the free_fork tool injection. nil → free_fork tool returns
	// "forker not initialized" error (legacy behaviour). DM-20260617-008 W5
	// replaces the previous freefork.GlobalForker() lookup with this explicit
	// field; callers (main.go) construct the forker via WireDefaultForker
	// before calling NewContextEngineBuilder.
	forker freefork.Forker
	// ctx 用于 startTrackerTick 等后台 goroutine 的生命周期管理;
	// nil 时回退到 context.Background (不会主动退出, 适用于单例启动场景).
	ctx context.Context
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
		ctx:          context.Background(),
	}
}

// WithMultiAgentConfig enables delegate tool registration on per-agent engines.
func (b *ContextEngineBuilder) WithMultiAgentConfig(maCfg *config.MultiAgentConfig) *ContextEngineBuilder {
	if b != nil {
		b.maCfg = maCfg
	}
	return b
}

// WithContext binds a parent context whose cancellation stops background
// goroutines (e.g. tracker tick) spawned during Build. Recommended for
// long-lived processes so shutdown is clean.
func (b *ContextEngineBuilder) WithContext(ctx context.Context) *ContextEngineBuilder {
	if b != nil && ctx != nil {
		b.ctx = ctx
	}
	return b
}

// WithForker wires the free_fork tool injection. Replaces the legacy
// freefork.SetGlobalForker process-wide write (DM-20260617-008 W5).
func (b *ContextEngineBuilder) WithForker(f freefork.Forker) *ContextEngineBuilder {
	if b != nil {
		b.forker = f
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
	// DM-20260617-008 W4: TaskManager constructed locally and passed to
	// RegisterTaskTools. Replaces workmodel.GlobalTaskManager singleton.
	tm := workmodel.NewTaskManagerFromConfig(b.ctxCfg.Tasks, b.obsBridge)
	toolReg, err := contextengine.NewBuiltinToolRegistry(b.toolCfg)
	if err != nil {
		slog.Error("create builtin tool registry", "error", err)
		toolReg = contextengine.NewToolRegistry()
	}
	if err := enforce.RegisterQueryLoopTools(toolReg, b.ctxCfg); err != nil {
		slog.Error("register query loop tools", "error", err)
	}
	if err := workmodel.RegisterTaskTools(toolReg, b.ctxCfg, tm); err != nil {
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
	// W11 phase 2c: legacy RegisterLSPTool is removed; the lsp tool is now
	// exposed via surface.LSPToolSurface (built in BuildSurfaces below).
	// The lspRunner itself is kept in toolrunner (LSPToolSurface wraps it
	// via NewLSPRunnerForSurface) but the Register*Tool registration helper
	// is gone. turn_adapter.Prepare now reads the schema from the surface
	// list, not from toolReg.

	// DM-20260617-002 W6 (AC4): G4 verify_plan_execution tool — 暴露给 LLM。
	// W11 phase 2c: now exposed exclusively via surface.VerifySurface below.
	// The legacy verifyRunner is removed.

	// DM-20260617-002 W7 (AC5): G5 free_fork tool — 通过 toolrunner.SetFreeForker 注入。
	// S4-Gate H-3 fix: function-based DI, toolrunner 不直接 import freefork.
	// W11 phase 2c: now exposed exclusively via surface.FreeForkSurface below.
	// The legacy freeforkRunner is removed; the toolrunner.globalFreeForker
	// package-level singleton is gone (the FreeForkerFunc is held in the
	// surface, not in a global var).
	// DM-20260617-002 W12 (AC11) + S4-Gate H-3: G3 notify drainer 通过 prompt
	// 注入点接入, 避免 prompt 包 import orchestration/workmodel/notify.
	wireTaskNotifDrainer()

	// DM-20260617-002 W8 (AC6): G6 query_diagnostics tool — 通过 surface 注入。
	// 同一 buildWithGate 中创建 tracker 实例 + 启动 tick goroutine。tracker 实例
	// 通过 BuildSurfaces(SurfaceBuildOpts.Tracker) 显式传给 surface.TrackerSurface,
	// 不再走 tracker.SetGlobalTracker + toolrunner.RegisterTrackerTool 旧路径。
	// W13 (AC14): tracker cap / tick interval 走 DiagnosticsConfig.
	// S4-Gate H-1 fix: 用 builder 持有的 ctx 控制 tick goroutine 生命周期, 避免多次 build 泄漏.
	diagCfg := b.ctxCfg.Diagnostics.Normalized()
	diagTracker := tracker.New(diagCfg.TrackerLRUCapacity)
	startTrackerTick(b.ctx, diagTracker, time.Duration(diagCfg.TrackerTickIntervalMs)*time.Millisecond)

	// DM-20260617-008 W1: transcript writer creation moved to bootstrap.NewTranscriptWriter.
	// Caller (cmd/devrix/main.go) invokes NewTranscriptWriter and passes the result to
	// capture.NewCommunicationGateway as the `writer` arg; this function no longer
	// touches the process-wide singleton.

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

	// TOOL-SURFACE-1 (W11 phase 2b): assemble the surface list for the
	// per-agent engine so turn_adapter.findSurface dispatches through the
	// surface path (the same as the main engine). The diagTracker created
	// above is the canonical instance; b.forker is the explicit free_fork
	// dependency (DM-20260617-008 W5). When the surface set covers all
	// callers, the global injection can be removed.
	surfaces := BuildSurfaces(SurfaceBuildOpts{
		ToolReg:   toolReg,
		LSPConfig: nil, // LSP wired via legacy RegisterLSPTool above
		Tracker:   diagTracker,
		Forker:    freeforkGlobalFunc(b.forker),
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
		SessionCommandQueue: sessionqueue.NewSessionQueue(),
		Surfaces:            surfaces,
		Filters:             DefaultFilters(),
	})
}

// startTrackerTick 在后台 goroutine 中按 interval 调 TickOnce。
// DM-20260617-002 W8: query_diagnostics tool 的"异步 tick"实现。linter 不可用时
// TickOnce 内部静默返回 0,不输出日志（避免噪音）。
//
// S4-Gate H-1 fix: parent ctx cancel → goroutine 干净退出, 避免多次 build 泄漏。
// tr/interval 不合法时立即返回, ctx 为 nil 时回退 Background (单例场景, 跟进程同生命周期)。
func startTrackerTick(parent context.Context, tr *tracker.Tracker, interval time.Duration) {
	if tr == nil || interval <= 0 {
		return
	}
	if parent == nil {
		parent = context.Background()
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-parent.Done():
				return
			case <-ticker.C:
				tr.TickOnce(parent)
			}
		}
	}()
}
