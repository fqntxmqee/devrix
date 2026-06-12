package query

import (
	"context"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// LoopHooks optional extension points (PEV Verify, stop hooks).
type LoopHooks struct {
	AfterToolRound func(ctx context.Context, sc *types.SessionContext, toolResults []ToolRoundResult) (stopLoop bool, err error)
	BeforeComplete func(ctx context.Context, sc *types.SessionContext) (preventContinue bool, err error)
}

// ToolRoundResult summarizes one tool execution in a batch.
type ToolRoundResult struct {
	Name   string
	Output string
	Error  string
}

// Params immutable per Run invocation (Claude Code QueryParams aligned).
type Params struct {
	SystemPrompt string
	UserContext  map[string]string
	Messages     []types.Message
	Tools        []ToolSchema
	MaxTurns     int
}

// ToolSchema mirrors contextengine.ToolSchema to avoid import cycles in tests.
type ToolSchema struct {
	Name        string
	Description string
	Parameters  string
}

// Result is the outcome of a QueryLoop run.
type Result struct {
	Messages        []types.Message
	AssistantText   string
	Usage           TokenUsage
	TurnCount       int
	ToolCallHistory []types.ToolCallRecord
}

// TokenUsage token counts from LLM responses.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
}

// EmitFunc streams engine events to the gateway.
type EmitFunc func(*contracts.EngineEvent)

// CompressFunc runs messages-only compression for one iteration.
type CompressFunc func(ctx context.Context, msgs []types.Message) ([]types.Message, error)

// LLMCaller performs one streaming LLM call with optional user context prepend.
type LLMCaller interface {
	Call(ctx context.Context, req LLMRequest) (<-chan LLMChunk, error)
}

// LLMRequest is the per-iteration LLM input.
type LLMRequest struct {
	Model        string
	SystemPrompt string
	Messages     []types.Message
	Tools        []ToolSchema
}

// LLMChunk streaming fragment.
type LLMChunk struct {
	Content   string
	Thinking  string
	ToolCalls []ToolCall
	Done      bool
	Usage     TokenUsage
}

// ToolCall requested tool invocation.
type ToolCall struct {
	ID    string
	Name  string
	Input string
}

// ToolExecutor runs a tool call.
type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (output, errMsg string, execErr error)
}

// PermissionChecker approves tool execution.
type PermissionChecker interface {
	Request(ctx context.Context, sessionID, toolName, input string) bool
}
