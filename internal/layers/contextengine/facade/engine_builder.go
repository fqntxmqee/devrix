package facade

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/contextengine/persist/snapshot"
	"github.com/devrix/devrix/internal/layers/contextengine/persist/transcript"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/attachments"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
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

	return &ContextEngine{
		memory:              memory.NewManager(cfg, store, deps.LongTerm),
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
		assembler:           prompt.NewSystemPromptAssembler(cfg.Workspace),
		mainTranscript:      mainTranscript,
		attachReg:           attachments.NewRegistry(cfg.Attachments),
		sessionQueue:        deps.SessionCommandQueue,
		defaultModel:        deps.DefaultModel,
		tierResolver:        deps.TierResolver,
		agentRoleToolFilter: deps.AgentRoleToolFilter,
		summarizer:          summarizer,
		surfaces:            deps.Surfaces,
		filters:             deps.Filters,
	}
}
