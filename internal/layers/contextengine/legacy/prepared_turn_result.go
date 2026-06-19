package legacy

import (
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// preparedTurnLoopResult mirrors the legacy query loop result fields used by finalizeTurn.
type preparedTurnLoopResult struct {
	AssistantText   string
	Usage           contracts.TokenUsage
	TurnCount       int
	ToolCallHistory []types.ToolCallRecord
}
