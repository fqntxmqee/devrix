package contextengine

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// LLMChunk is a streaming LLM response fragment.
type LLMChunk struct {
	Content   string
	Thinking  string
	ToolCalls []ToolCall
	Done      bool
	Usage     TokenUsage
}

// TokenUsage reports token consumption.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	CacheReadTokens  int
	ReasoningTokens  int
}

// LLMRequest is the input to ChatStream.
type LLMRequest struct {
	Model        string
	SystemPrompt string
	Messages     []types.Message
	Tools        []ToolSchema
}

// ToolSchema describes a tool for the LLM.
type ToolSchema struct {
	Name        string
	Description string
	Parameters  string
}

// ToolCall is an LLM-requested tool invocation.
type ToolCall struct {
	ID       string
	Name     string
	Input    string
	RiskLevel types.RiskLevel
}

// ToolResult is the outcome of tool execution.
type ToolResult struct {
	Output string
	Error  string
}

// ILLMGateway provides streaming chat completion.
type ILLMGateway interface {
	ChatStream(ctx context.Context, req *LLMRequest) (<-chan LLMChunk, error)
}

// IToolRunner executes tool calls.
type IToolRunner interface {
	Execute(ctx context.Context, call ToolCall) (*ToolResult, error)
}

// IToolRegistry lists available tools and risk levels.
type IToolRegistry interface {
	ListTools(ctx context.Context, workDir string) ([]ToolSchema, error)
	RiskLevel(toolName string) types.RiskLevel
}

// IPermissionGate approves tool execution before running.
type IPermissionGate interface {
	Request(ctx context.Context, sessionID, toolName, input string, risk types.RiskLevel) bool
}

// AutocompactMeta describes autocompact observability metadata.
type AutocompactMeta struct {
	Degraded      bool
	SummaryTokens int
	Model         string
}

// ICompressionObserver emits compression pipeline events.
type ICompressionObserver interface {
	EmitCompressionStep(sessionID, step string, before, after int)
	EmitAutocompact(sessionID string, meta AutocompactMeta)
	EmitAutocompactComplete(sessionID string, summary types.Message, asyncToken string)
}

// IPEVObserver emits PEV phase events.
type IPEVObserver interface {
	EmitVerifyCommand(sessionID, cmd string, result VerifyCommandResult)
	EmitPlanCompleted(sessionID string, milestoneCount int)
	EmitMilestoneProgress(sessionID, milestoneID string, progress float64)
}

// NoOpCompressionObserver discards compression observer events.
type NoOpCompressionObserver struct{}

func (NoOpCompressionObserver) EmitCompressionStep(string, string, int, int)              {}
func (NoOpCompressionObserver) EmitAutocompact(string, AutocompactMeta)                   {}
func (NoOpCompressionObserver) EmitAutocompactComplete(string, types.Message, string)     {}

// NoOpPEVObserver discards PEV observer events.
type NoOpPEVObserver struct{}

func (NoOpPEVObserver) EmitVerifyCommand(string, string, VerifyCommandResult)                 {}
func (NoOpPEVObserver) EmitPlanCompleted(string, int)                                           {}
func (NoOpPEVObserver) EmitMilestoneProgress(string, string, float64)                           {}

// IObserver emits context engine observability events.
type IObserver interface {
	EmitContextCompressed(report types.CompressionReport)
	EmitPEVPhase(sessionID string, phase types.PEVPhase, iteration int)
	EmitSnapshotRestored(sessionID string, fromBackup bool)
	EmitErrorOccurred(sessionID string, code string, err error)
	EmitPEVIteration(sessionID string, iteration int, phase types.PEVPhase)
}

// NoOpObserver discards observer events.
type NoOpObserver struct{}

func (NoOpObserver) EmitContextCompressed(types.CompressionReport)              {}
func (NoOpObserver) EmitPEVPhase(string, types.PEVPhase, int)                 {}
func (NoOpObserver) EmitSnapshotRestored(string, bool)                       {}
func (NoOpObserver) EmitErrorOccurred(string, string, error)                   {}
func (NoOpObserver) EmitPEVIteration(string, int, types.PEVPhase)              {}
