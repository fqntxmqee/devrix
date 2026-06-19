package contracts

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// SubTurnScope classifies nested turn execution (D7-owned loop).
type SubTurnScope string

const (
	SubTurnScopeSubQuery    SubTurnScope = "subquery"
	SubTurnScopeBackground  SubTurnScope = "background"
	SubTurnScopeWaveWorker  SubTurnScope = "wave_worker"
)

// SubTurnRequest is the input to SubTurnExecutor.RunSubTurn.
// D2 enforce builds messages and delegates the loop to D7 via this contract.
type SubTurnRequest struct {
	SessionID      string
	AgentID        string
	AgentName      string
	SystemPrompt   string
	Messages       []types.Message
	Tools          []ToolSchema
	MaxTurns       int
	Scope          SubTurnScope
	ChildContext   *types.SessionContext
	FlowParams     SubQueryFlowParams
	FlowReporter   SubQueryFlowReporter
	Emit           EngineEmitFunc
}

// SubTurnResult is the synchronous outcome of a nested turn.
type SubTurnResult struct {
	AssistantText   string
	Messages        []types.Message
	TurnCount       int
	ToolCallHistory []types.ToolCallRecord
	Usage           TokenUsage
}

// SubTurnExecutor runs a nested LLM↔Tool loop in D7 (not D2).
type SubTurnExecutor interface {
	RunSubTurn(ctx context.Context, req SubTurnRequest) (*SubTurnResult, error)
}
