package contextengine

import (
	"context"
	"log/slog"

	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/attachments"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/usercontext"
	"github.com/devrix/devrix/internal/layers/contextengine/persist/snapshot"
	"github.com/devrix/devrix/internal/layers/contextengine/persist/transcript"
	"github.com/devrix/devrix/internal/layers/contextengine/token"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// NewContextEngine creates the Layer 2 context engine.
func NewContextEngine(deps EngineDeps) *ContextEngine {
	cfg := deps.Config
	if cfg == nil {
		cfg = config.DefaultContextEngineConfig()
	}
	observer := deps.Observer
	if observer == nil {
		observer = NoOpObserver{}
	}
	compObserver := deps.CompressionObserver
	if compObserver == nil {
		compObserver = NoOpCompressionObserver{}
	}
	counter := token.ResolveCounter(cfg, deps.TokenCounter)
	store := snapshot.NewStore(&cfg.Snapshot)
	queryCaller := deps.QueryLLMCaller
	if queryCaller == nil {
		panic("contextengine: QueryLLMCaller is required (inject D7 turn.QueryLLMCaller)")
	}
	summarizer := deps.Summarizer
	if summarizer == nil {
		panic("contextengine: Summarizer is required (inject D7 turn.CompressionSummarizer)")
	}
	var asyncCompact *compression.AsyncAutocompacter
	if cfg.Compression.Autocompact.Enabled {
		asyncCompact = compression.NewAsyncAutocompacter(summarizer)
	}
	toolsReg := deps.ToolsReg
	ucProvider := usercontext.NewProvider(prompt.NewLoader(&cfg.SystemPrompt), cfg.UserContext)

	loop := &query.Loop{
		LLM:             queryCaller,
		Tools:           query.NewToolExecutor(deps.Tools, toolsReg, deps.ObsBridge),
		Permission:      query.NewPermChecker(deps.Permission, toolsReg, deps.ObsBridge),
		UserContext:     ucProvider,
		WrapToolContext: func(ctx context.Context, sc *types.SessionContext) context.Context {
			return ToolContextWithGate(ctx, sc, deps.Permission)
		},
		WrapToolStreamContext: func(ctx context.Context, emit query.EmitFunc, sessionID, toolName string) context.Context {
			return withToolStreamEmitter(ctx, emit, sessionID, toolName)
		},
		StreamingTools:    cfg.QueryLoop.StreamingTools,
		Observability:     deps.ObsBridge,
	}
	// DM-20260617-001: wire the legacy-path counter so any
	// loopFirst=false invocation bumps
	// d2_query_loop_legacy_invocations_total.
	loop.LegacyCounter = resolveLegacyQueryLoopCounter(deps.ObsBridge)
	if cfg.QueryLoop.CompressPerTurn {
		loop.CompressFactory = compression.NewQueryLoopCompressFactory(
			cfg.QueryLoop.CompressPerTurn,
			cfg,
			counter,
			summarizer,
			asyncCompact,
			compObserver,
			deps.ObsBridge,
		)
	}

	var mainTranscript *transcript.MainThreadStore
	if cfg.MainTranscript.Enabled {
		baseDir := cfg.MainTranscript.BaseDir
		if baseDir == "" {
			baseDir = config.DefaultMainTranscriptConfig().BaseDir
		}
		if store, err := transcript.NewMainThreadStore(baseDir); err != nil {
			slog.Warn("contextengine: main transcript disabled", "error", err)
		} else {
			mainTranscript = store
		}
	}

	return &ContextEngine{
		memory:       memory.NewManager(cfg, store, deps.LongTerm),
		counter:      counter,
		queryLoop:    loop,
		prompt:       prompt.NewLoader(&cfg.SystemPrompt),
		cfg:          cfg,
		observer:     observer,
		compObserver: compObserver,
		obsBridge:    deps.ObsBridge,
		asyncCompact: asyncCompact,
		tools:        deps.Tools,
		toolsReg:     toolsReg,
		permission:   deps.Permission,
		assembler:    prompt.NewSystemPromptAssembler(cfg.Workspace),
		mainTranscript: mainTranscript,
		attachReg:      attachments.NewRegistry(cfg.Attachments),
		sessionQueue:   deps.SessionCommandQueue,
		defaultModel: deps.DefaultModel,
		tierResolver: deps.TierResolver,
		agentRoleToolFilter: deps.AgentRoleToolFilter,
		queryCaller: queryCaller,
		summarizer:  summarizer,
	}
}

// resolveLegacyQueryLoopCounter returns the
// d2_query_loop_legacy_invocations_total counter for the QueryLoop
// to bump on every Run() invocation. Returns nil if observability is
// not wired (test contexts, dev mode with bridge disabled) so the
// counter becomes a soft dependency rather than a hard boot
// requirement.
//
// The counter is registered with its canonical name in the
// observability registry directly (not via the meter's auto-prefix
// machinery) so the name on /metrics scraping is exactly
// `d2_query_loop_legacy_invocations_total` with no meter prefix.
// See openspec/specs/d7-orchestration/spec.md "D2 QueryLoop Legacy
// Path Decommission" and the t-registry entry D5-S24-A02-T04.
//
// DM-20260617-001.
func resolveLegacyQueryLoopCounter(bridge *observability.Bridge) metrics.Counter {
	if bridge == nil {
		return nil
	}
	meter := bridge.Meter()
	if meter == nil {
		return nil
	}
	registry := meter.Registry()
	if registry == nil {
		return nil
	}
	const name = "d2_query_loop_legacy_invocations_total"
	// Register directly into the underlying registry so the metric
	// name is exactly `d2_query_loop_legacy_invocations_total` (no
	// `devrix_` prefix from the meter's fullMetricName helper).
	if existing, ok := registry.GetCounter(name, nil); ok && existing != nil {
		return existing
	}
	c := metrics.NewCounter(name, nil)
	_ = registry.RegisterCounter(name, nil, c)
	return c
}
