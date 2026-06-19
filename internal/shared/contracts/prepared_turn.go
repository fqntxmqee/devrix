package contracts

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// PreparedTurnRequest is the D2-assembled context handed to D7 for Process().
type PreparedTurnRequest struct {
	SessionID    string
	SystemPrompt string
	Messages     []types.Message
	Tools        []ToolSchema
	MaxTurns     int
	Emit         EngineEmitFunc
}

// PreparedTurnResult summarizes a prepared turn run (legacy Process path).
type PreparedTurnResult struct {
	AssistantText   string
	Usage           TokenUsage
	TurnCount       int
	ToolCallHistory []types.ToolCallRecord
}

// PreparedTurnRunner executes the LLM↔Tool loop for engine.Process after D2 prep.
type PreparedTurnRunner interface {
	RunPreparedTurn(ctx context.Context, req PreparedTurnRequest) (*PreparedTurnResult, error)
}
