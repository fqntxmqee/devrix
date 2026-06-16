package contextengine

import (
	"context"
	"log/slog"

	"github.com/devrix/devrix/internal/layers/contextengine/fallback"
	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/attachments"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/usercontext"
	"github.com/devrix/devrix/internal/layers/contextengine/persist/snapshot"
	"github.com/devrix/devrix/internal/layers/contextengine/persist/transcript"
	"github.com/devrix/devrix/internal/layers/contextengine/token"
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
	harnessBoot := fallback.NewBootstrap(fallback.BootstrapDeps{
		Config:    cfg.Harness,
		ToolsReg:  toolRegistryAdapter{reg: toolsReg},
		ObsBridge: deps.ObsBridge,
	})
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
		harnessBoot:  harnessBoot,
		assembler:    prompt.NewSystemPromptAssembler(cfg.Workspace),
		preflight:    fallback.NewPreflightEvaluator(cfg.Preflight, fallback.NewToolPoolFilter(cfg.Harness.ToolPool)),
		router:       fallback.NewPromptRouter(cfg.Harness.Routing),
		transcript:   fallback.NewTranscriptManager(cfg.Harness.Transcript),
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
