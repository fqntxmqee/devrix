package kernel

import (
	"sync"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/layers/contextengine/persist/transcript"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/attachments"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

type (
	IToolRunner         = tools.IToolRunner
	IToolRegistry       = tools.IToolRegistry
	AgentRoleToolFilter = enforce.AgentRoleToolFilter
)

// EngineDeps holds dependencies for ContextEngine.
//
// P4 split (AC-P4-3): LongTerm is now two independent ports — a
// read-side LongTermRecaller and a write-side LongTermStore. The
// production wiring passes a single shared *SQLiteLongTerm as both
// arguments; tests can inject narrower doubles.
type EngineDeps struct {
	TokenCounter        contracts.ITokenCounter
	Tools               IToolRunner
	ToolsReg            IToolRegistry
	Permission          contracts.IPermissionGate
	Observer            IObserver
	CompressionObserver ICompressionObserver
	LongTermRecaller    contracts.LongTermRecaller
	LongTermStore       contracts.LongTermStore
	Config              *config.ContextEngineConfig
	ObsBridge           *observability.Bridge
	DefaultModel        string
	TierResolver        contracts.TierResolver
	AgentRoleToolFilter AgentRoleToolFilter
	Summarizer          contracts.Summarizer
	SessionCommandQueue contracts.SessionCommandQueue
	PreparedTurnRunner  contracts.PreparedTurnRunner
	// Surfaces and Filters are the new TOOL-SURFACE-1 inputs (W8).
	// When non-nil, the engine stores them for use by the surface
	// dispatch path (W9 replaces turn_adapter IToolRunner.Execute).
	// When nil, the engine still works via the legacy Tools/ToolsReg
	// path. Both can be set simultaneously during phase 1.
	//
	// DSAFT: TOOL-SURFACE-1-A03 (DM-20260617-007 devrix-tool-surface-contract)
	Surfaces []contracts.ToolSurface
	Filters  []contracts.ToolFilter
}

// ContextEngine implements contracts.IEngine.
//
// DSAFT: D2-S1-A01 (ExecuteQuery)
type ContextEngine struct {
	memory              *memory.Manager
	counter             contracts.ITokenCounter
	preparedTurnRunner  contracts.PreparedTurnRunner
	tools               IToolRunner
	toolsReg            IToolRegistry
	permission          contracts.IPermissionGate
	prompt              *prompt.Loader
	cfg                 *config.ContextEngineConfig
	observer            IObserver
	compObserver        ICompressionObserver
	obsBridge           *observability.Bridge
	asyncCompact        *compression.AsyncAutocompacter
	assembler           *prompt.SystemPromptAssembler
	mainTranscript      *transcript.MainThreadStore
	attachReg           *attachments.Registry
	sessionQueue        contracts.SessionCommandQueue
	defaultModel        string
	tierResolver        contracts.TierResolver
	agentRoleToolFilter AgentRoleToolFilter
	summarizer          contracts.Summarizer
	// prepareOrchestrator is the D2-S15 scenario orchestrator wired in P1-d
	// to replace the legacy inline engine_prepare.go helpers. Built lazily
	// on first Process() via wirePrepareOrchestrator().
	prepareOrchestrator *prepare.PrepareOrchestrator
	// persistOrchestrator is the D2-S17 scenario orchestrator wired in P1-e
	// to replace the legacy finalizeTurn helpers in engine_persist.go. Built
	// lazily on first Process() via wirePersistOrchestrator().
	persistOrchestrator *persist.PersistOrchestrator
	// surfaces and filters are the new TOOL-SURFACE-1 fields (W8).
	// nil in phase 1 means the legacy Tools/ToolsReg path is still in
	// use; non-nil enables the surface dispatch path in W9.
	surfaces []contracts.ToolSurface
	filters  []contracts.ToolFilter

	metricsOnce      sync.Once
	compressionRatio metrics.Histogram
}
