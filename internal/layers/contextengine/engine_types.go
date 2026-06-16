package contextengine

import (
	"sync"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/attachments"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/layers/contextengine/persist/transcript"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
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
	QueryLLMCaller      contracts.LLMCaller
	Summarizer          contracts.Summarizer
	SessionCommandQueue contracts.SessionCommandQueue
}

// ContextEngine implements contracts.IEngine.
//
// DSAFT: D2-S1-A01 (ExecuteQuery)
type ContextEngine struct {
	memory       *memory.Manager
	counter      contracts.ITokenCounter
	queryLoop    *query.Loop
	tools        IToolRunner
	toolsReg     IToolRegistry
	permission   contracts.IPermissionGate
	prompt       *prompt.Loader
	cfg          *config.ContextEngineConfig
	observer     IObserver
	compObserver ICompressionObserver
	obsBridge    *observability.Bridge
	asyncCompact *compression.AsyncAutocompacter
	assembler    *prompt.SystemPromptAssembler
	mainTranscript *transcript.MainThreadStore
	attachReg      *attachments.Registry
	sessionQueue   contracts.SessionCommandQueue
	defaultModel string
	tierResolver contracts.TierResolver
	agentRoleToolFilter AgentRoleToolFilter
	queryCaller contracts.LLMCaller
	summarizer  contracts.Summarizer

	metricsOnce      sync.Once
	compressionRatio metrics.Histogram
}
