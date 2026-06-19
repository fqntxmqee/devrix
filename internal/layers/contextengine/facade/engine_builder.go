package facade

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/layers/contextengine/persist/snapshot"
	"github.com/devrix/devrix/internal/layers/contextengine/persist/transcript"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/adapters"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/attachments"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/shared/config"
)

// NewContextEngine creates the Layer 2 context engine.
func NewContextEngine(deps EngineDeps) *ContextEngine {
	cfg := deps.Config
	if cfg == nil {
		cfg = config.DefaultContextEngineConfig()
	}
	observer := deps.Observer
	if observer == nil {
		observer = kernel.NoOpObserver{}
	}
	compObserver := deps.CompressionObserver
	if compObserver == nil {
		compObserver = kernel.NoOpCompressionObserver{}
	}
	counter := token.ResolveCounter(cfg, deps.TokenCounter)
	store := snapshot.NewStore(&cfg.Snapshot)
	summarizer := deps.Summarizer
	if summarizer == nil {
		panic("contextengine: Summarizer is required (inject D7 turn.CompressionSummarizer)")
	}
	var asyncCompact *compression.AsyncAutocompacter
	if cfg.Compression.Autocompact.Enabled {
		asyncCompact = compression.NewAsyncAutocompacter(summarizer)
	}
	toolsReg := deps.ToolsReg

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

	memMgr := memory.NewManager(cfg, store, deps.LongTermRecaller, deps.LongTermStore)
	assembler := prompt.NewSystemPromptAssembler(cfg.Workspace)

	return &ContextEngine{
		memory:              memMgr,
		counter:             counter,
		preparedTurnRunner:  deps.PreparedTurnRunner,
		prompt:              prompt.NewLoader(&cfg.SystemPrompt),
		cfg:                 cfg,
		observer:            observer,
		compObserver:        compObserver,
		obsBridge:           deps.ObsBridge,
		asyncCompact:        asyncCompact,
		tools:               deps.Tools,
		toolsReg:            toolsReg,
		permission:          deps.Permission,
		assembler:           assembler,
		mainTranscript:      mainTranscript,
		attachReg:           attachments.NewRegistry(cfg.Attachments),
		sessionQueue:        deps.SessionCommandQueue,
		defaultModel:        deps.DefaultModel,
		tierResolver:        deps.TierResolver,
		agentRoleToolFilter: deps.AgentRoleToolFilter,
		summarizer:          summarizer,
		surfaces:            deps.Surfaces,
		filters:             deps.Filters,
		prepareOrchestrator: nil, // wired in wirePrepareOrchestrator after construction
		persistOrchestrator: nil, // wired in wirePersistOrchestrator after construction
	}
}

// wirePrepareOrchestrator builds the prepare.PrepareOrchestrator with the
// four concrete adapters (P1-b) and registers facade lifecycle hooks
// (worker fork, permission init, tier resolution, prompt → CompressedView wrap).
//
// Called lazily on first Process() to defer allocation until actually needed.
func (e *ContextEngine) wirePrepareOrchestrator() {
	if e.prepareOrchestrator != nil {
		return
	}
	hooks := []adapters.HooksOption{
		adapters.WithSpanStarter(e.startSpan),
		// Emit is set per-call in runProcess via the emit closure.
	}

	sessionLoader := adapters.NewSessionLoaderAdapter(e.memory, hooks...)
	recaller := adapters.NewMemoryRecallerAdapter(e.memory, hooks...)
	compressor := adapters.NewCompressorAdapter(e.compressionPipeline, hooks...).
		WithCompressPerTurnSkip(func() bool { return !e.cfg.TurnRuntime.CompressPerTurn })
	assemblerAdapter := adapters.NewAssemblerAdapter(e.assembler, hooks...)

	e.prepareOrchestrator = prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader:   sessionLoader,
		MemoryRecaller:  recaller,
		Compressor:      compressor,
		PromptAssembler: assemblerAdapter,
	})
}

// wirePersistOrchestrator builds the persist.PersistOrchestrator with facade
// adapters for snapshot / transcript / long-term / commit-window (P1-e).
//
// Called lazily on first Process() to defer allocation until actually needed.
func (e *ContextEngine) wirePersistOrchestrator() {
	if e.persistOrchestrator != nil {
		return
	}
	e.persistOrchestrator = persist.NewPersistOrchestrator(persist.PersistDeps{
		SnapshotPersister: newSnapshotAdapter(e),
		TranscriptWriter:  newTranscriptAdapter(e),
		LongTermStorer:    newLongTermAdapter(e),
		CommitWindow:      newCommitWindowAdapter(e),
	})
}
