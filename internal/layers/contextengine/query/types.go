package query

import (
	"context"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

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

// ToolSchema re-exported from shared/contracts.ToolSchema.
type ToolSchema = contracts.ToolSchema

// Result is the outcome of a QueryLoop run.
type Result struct {
	Messages        []types.Message
	AssistantText   string
	Usage           TokenUsage
	TurnCount       int
	ToolCallHistory []types.ToolCallRecord
}

// TokenUsage token counts from LLM responses.
type TokenUsage = contracts.TokenUsage

// EmitFunc streams engine events to the capture.
type EmitFunc func(*contracts.EngineEvent)

// CompressFunc runs messages-only compression for one iteration.
type CompressFunc func(ctx context.Context, msgs []types.Message) ([]types.Message, error)

// LLMCaller performs one streaming LLM call with optional user context prepend.
//
// Re-exported from shared/contracts (DM-020拆面契约). Implemented by
// D7 GatewayInvoker; may also be implemented by D2-internal fakes for tests.
type LLMCaller = contracts.LLMCaller

// LLMRequest is the per-iteration LLM input.
type LLMRequest = contracts.LLMRequest

// LLMChunk streaming fragment.
type LLMChunk = contracts.LLMChunk

// ToolCall requested tool invocation.
type ToolCall = contracts.ToolCall

// ToolExecutor runs a tool call.
type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (output, errMsg string, execErr error)
}

// PermissionChecker approves tool execution.
type PermissionChecker interface {
	Request(ctx context.Context, sessionID, toolName, input string) bool
}
