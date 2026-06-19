package facade

import (
	"sync"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
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
	IToolRunner          = toolrunner.IToolRunner
	IToolRegistry        = toolrunner.IToolRegistry
	IObserver            = kernel.IObserver
	ICompressionObserver = kernel.ICompressionObserver
	AgentRoleToolFilter  = enforce.AgentRoleToolFilter
)

// EngineDeps holds dependencies for ContextEngine.
type EngineDeps struct {
	TokenCounter        contracts.ITokenCounter
	Tools               IToolRunner
	ToolsReg            IToolRegistry
	Permission          contracts.IPermissionGate
	Observer            IObserver
	CompressionObserver ICompressionObserver
	LongTerm            memory.ILongTermMemory
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
