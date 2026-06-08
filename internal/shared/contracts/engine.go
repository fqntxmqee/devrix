package contracts

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// IEngine is the cross-layer context processing contract (L1 Gateway ↔ L2 Context Engine ↔ L4 Agent).
type IEngine interface {
	Process(ctx context.Context, session *types.Session, message string) <-chan *EngineEvent
}

// EngineEvent is emitted by the context engine during Process.
type EngineEvent struct {
	Type      string // thinking | text | tool_call | tool_result | permission | status | complete | error
	Content   string
	ToolName  string
	ToolInput string
	ToolRisk  types.RiskLevel
	SessionID string
	Metadata  map[string]string
}
