package bootstrap

import (
	"context"
	"log/slog"
	"time"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
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
//
// DM-20260617-005: register the same diagnostic tool surface that
// buildWithGate exposes to per-agent engines. The main engine used by
// devrix binary goes through SelectContextEngine → NewContextEngine;
// without these registrations the leader LLM cannot see free_fork,
// query_diagnostics, verify_plan_execution, lsp, or delegate_*, so it
// responds with "unknown tool" when the user asks for them.
func NewContextEngine(
	stack llmbridge.ContextLLMStack,
	permMgr *capture.PermissionManager,
	ctxCfg *config.ContextEngineConfig,
	toolCfg *config.ToolConfig,
	maCfg *config.MultiAgentConfig,
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
	if err := enforce.RegisterQueryLoopTools(toolReg, ctxCfg); err != nil {
		slog.Error("register query loop tools", "error", err)
	}
	if err := workmodel.RegisterTaskTools(toolReg, ctxCfg, workmodel.GlobalTaskManager); err != nil {
		slog.Error("register task tools", "error", err)
	}
	if err := enforce.RegisterBackgroundTaskTools(toolReg); err != nil {
		slog.Error("register background task tools", "error", err)
	}
	if ctxCfg.TodoWrite.Enabled {
		if err := toolReg.Register(contextengine.NewTodoWriteRunner()); err != nil {
			slog.Error("register todo_write", "error", err)
		}
	}
	// delegate_* tools are NOT registered here on the main engine —
	// WireDelegate owns that registration (see internal/bootstrap/delegate.go)
	// so the leader LLM sees the same set the leader is allowed to dispatch.
	// Per-agent engines (ContextEngineBuilder.buildWithGate) register them
	// directly because they don't go through WireDelegate.

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

	// Diagnostic tool surface (kept in sync with ContextEngineBuilder.buildWithGate
	// so the leader LLM sees the same tool list as per-agent engines).
	//
	// W11 phase 2c: lsp / verify_plan_execution / free_fork are now exposed
	// exclusively via the surface list built in BuildSurfaces below. The
	// legacy toolrunner.RegisterLSPTool / RegisterVerifyTool / RegisterFreeForkTool
	// helpers are removed; turn_adapter.Prepare aggregates tool specs from
	// the engine's surface list (TOOL-SURFACE-1 SoT) and the per-tool
	// dispatch goes through surface.Execute (W9). The schema + execution
	// surface is no longer dependent on a package-level global singleton.
	wireTaskNotifDrainer()

	diagCfg := ctxCfg.Diagnostics.Normalized()
	diagTracker := tracker.New(diagCfg.TrackerLRUCapacity)
	// W11 phase 2: query_diagnostics is exposed via surface.TrackerSurface
	// (built in BuildSurfaces below). The legacy toolrunner.RegisterTrackerTool
	// + tracker.SetGlobalTracker path is removed; the surface holds the
	// tracker instance explicitly so production code no longer relies on the
	// process-wide singleton.
	startTrackerTick(context.Background(), diagTracker, time.Duration(diagCfg.TrackerTickIntervalMs)*time.Millisecond)

	// DM-20260617-008 W1: transcript writer creation moved to bootstrap.NewTranscriptWriter.
	// Caller (cmd/devrix/main.go) invokes NewTranscriptWriter and passes the result to
	// capture.NewCommunicationGateway as the `writer` arg; this function no longer
	// touches the process-wide singleton.

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

	// TOOL-SURFACE-1 (W8): assemble the surface list from the same
	// dependencies the legacy registry uses. The surfaces are stored
	// on the engine for the W9 turn_adapter surface dispatch path.
	// For now, the legacy Tools/ToolsReg path is still in use and
	// produces the same tool set the leader LLM sees.
	//
	// TOOL-SURFACE-1 (W11 partial): pass the same forker closure the
	// legacy wireFreeForkerInjection installs. The surface path is
	// primary; the global injection is the legacy back-compat. When
	// the surface set covers all callers, the global injection can
	// be removed.
	surfaces := BuildSurfaces(SurfaceBuildOpts{
		ToolReg:   toolReg,
		LSPConfig: nil, // LSP wired via legacy RegisterLSPTool above
		Tracker:   diagTracker,
		Forker:    freeforkGlobalFunc,
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
		AgentRoleToolFilter: toolpolicy.NewFilter(),
		QueryLLMCaller:      queryCaller,
		Summarizer:          summarizer,
		SessionCommandQueue: sessionqueue.GlobalSessionQueue,
		// TOOL-SURFACE-1 (W8): surface list (no filter on main engine).
		Surfaces: surfaces,
		Filters:  nil,
	})
}

// Compile-time assertion that the adapters implement the D2拆面 contracts.
var (
	_ contracts.LLMCaller       = (*turn.QueryLLMCaller)(nil)
	_ contracts.Summarizer      = (*turn.CompressionSummarizer)(nil)
	_ contracts.IEngine         = (*contextengine.ContextEngine)(nil)
	_ contracts.IPermissionGate = (*capture.PermissionGateAdapter)(nil)
)
