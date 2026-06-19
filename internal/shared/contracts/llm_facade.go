package contracts

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// LLMCaller is a mock/test-only streaming LLM facade.
//
// Deprecated: production LLM calls use D7 GatewayInvoker. Retained for mockctx.StaticLLMCaller.
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

// LLMChunk is a streaming LLM response fragment.
type LLMChunk struct {
	Content   string
	Thinking  string
	ToolCalls []ToolCall
	Done      bool
	Usage     TokenUsage
}

// ToolCall is an LLM-requested tool invocation.
type ToolCall struct {
	ID    string
	Name  string
	Input string
}

// ToolSchema describes a tool for the LLM facade contract.
type ToolSchema struct {
	Name        string
	Description string
	Parameters  string
}

// TokenUsage reports per-call token consumption.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
}

// Summarizer generates a text summary for autocompact.
//
// DSAFT: D7-S2-A07 (InvokeLLM) → D2 compression pipeline 拆面出口
// Implemented by D7 turn.CompressionSummarizer; consumed by D2 compression.Pipeline via EngineDeps.
type Summarizer interface {
	Summarize(ctx context.Context, model, prompt string, maxTokens int) (string, error)
}
