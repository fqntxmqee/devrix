package contracts

import "context"

// ToolSchema describes a tool for the LLM facade contract.
//
// DSAFT: D2-S18-A03 ToolRegistry (D2 owner) + D7 delegation surface (D7 consumer).
// Used by D2 subquery (enforce/subquery.go) and D7 delegatetools/builtin_agents.go.
type ToolSchema struct {
	Name        string
	Description string
	Parameters  string
}

// TokenUsage reports per-call token consumption.
//
// DSAFT: cross-domain type. D2 prepared_turn_result.go + D7 turn/subturn.go consume.
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
