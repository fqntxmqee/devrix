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

// SubAgentMode classifies how a sub-agent inherits parent history
// (Phase B AC6 — context-budget-and-isolation).
type SubAgentMode string

const (
	// SubAgentModeBrief: child starts with a fresh history; only the child's
	// own user message is in the prompt. Use for single-shot sub-agents
	// (single file query / single grep) where parent context adds no value.
	SubAgentModeBrief SubAgentMode = "brief"
	// SubAgentModeFork: child sees the parent's last assistant message (with
	// all tool_use) plus a placeholder tool_result for each tool_use. Sibling
	// fork children share byte-level prefix (prompt cache friendly). Use for
	// multi-step research where the LLM benefits from the parent's tool
	// sequence.
	SubAgentModeFork SubAgentMode = "fork"
	// SubAgentModeFull: child sees the full parent history minus the last
	// user message. Use for D5-style evaluation where the child must see the
	// complete conversation flow. Equivalent to pre-Phase-B behavior.
	SubAgentModeFull SubAgentMode = "full"
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
	// Mode selects how the sub-agent inherits parent history (Phase B AC6).
	// Empty defers to SubTurnRunner.Cfg.DefaultMode (typically "brief").
	Mode SubAgentMode
	// Depth is the recursion depth (0 = root turn, 1 = first-level sub-agent).
	// SubTurnRunner rejects requests with Depth >= MaxSubagentDepth
	// (Phase B AC9).
	Depth int
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
