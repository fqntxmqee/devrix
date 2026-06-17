package turn

import (
	"context"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// TurnScope classifies the turn execution context.
type TurnScope string

const (
	TurnScopeMain     TurnScope = "main"
	TurnScopeSubQuery TurnScope = "subquery"
	TurnScopeCompress TurnScope = "compress"
)

// TurnRequest is the input to RunTurn.
type TurnRequest struct {
	SessionID    string
	UserMessage  types.Message
	SystemPrompt string
	MaxTurns     int
	Scope        TurnScope
	// Mode is the agent mode for this turn ("plan_mode" | "build_mode" | ...).
	// Empty means no mode-specific constraint.
	Mode string
}

// CompressHint signals that D2 detected a token budget overrun and D7 should
// invoke D3 for summarization before the next turn.
type CompressHint struct {
	MessagesToSummarize []types.Message
	TargetTokenBudget   int
}

// PreparedContext is the output of D2-S15 PrepareExecutionContext, ready for
// LLM consumption.
type PreparedContext struct {
	SystemPrompt     string
	Messages         []types.Message
	Tools            []ToolSchema
	CompressHint     *CompressHint
	Model            string
	MaxContextTokens int
}

// ToolRoundRequest is the input to D2-S18 ExecuteToolRound.
type ToolRoundRequest struct {
	SessionID string
	ToolCalls []llmgateway.ToolCall
}

// ToolRoundResult is the output of D2-S18 ExecuteToolRound.
type ToolRoundResult struct {
	Results []ToolResult
}

// ToolResult is a single tool execution outcome.
type ToolResult struct {
	ToolCallID string
	Output     string
	Error      string
}

// PersistRequest is the input to D2-S17 PersistSessionState.
type PersistRequest struct {
	SessionID string
	Messages  []types.Message
	TurnCount int
	Usage     llmgateway.TokenUsage
	FinalText string
}

// ToolSchema mirrors query.ToolSchema for the D7→D2 contract boundary.
type ToolSchema struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// LLMInvokeRequest is the input to D7-S2-A07 InvokeLLM.
type LLMInvokeRequest struct {
	SessionID    string
	Tier         string
	SystemPrompt string
	Messages     []types.Message
	Tools        []ToolSchema
}

// TurnOrchestrator owns the LLM↔Tool turn loop (D7-S2-A06).
type TurnOrchestrator interface {
	RunTurn(ctx context.Context, req TurnRequest) (<-chan *contracts.EngineEvent, error)
}

// LLMInvoker performs one D3 streaming call (D7-S2-A07).
type LLMInvoker interface {
	InvokeStream(ctx context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error)
}

// ContextPreparer assembles legal context for one iteration (D2-S15).
type ContextPreparer interface {
	Prepare(ctx context.Context, req PrepareRequest) (PreparedContext, error)
}

// PrepareRequest is the input to ContextPreparer.
type PrepareRequest struct {
	SessionID string
	Message   types.Message
	// Mode is the agent mode for the upcoming turn ("plan_mode" | "build_mode" | ...).
	// ToolFilters that key off the mode (e.g. PlanModeOpenWorldPolicy.ShouldDefer)
	// consult this field. Empty means "no mode constraint".
	Mode string
}

// ToolRoundExecutor runs policy-gated tool batch (D2-S18).
type ToolRoundExecutor interface {
	ExecuteRound(ctx context.Context, req ToolRoundRequest) (ToolRoundResult, error)
}

// SessionPersister commits turn outcome (D2-S17).
type SessionPersister interface {
	PersistTurn(ctx context.Context, req PersistRequest) error
}
