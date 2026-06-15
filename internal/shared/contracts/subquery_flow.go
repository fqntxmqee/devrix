package contracts

import "context"

// SubQueryFlowParams identifies a nested SubQuery run for Hub-Spoke flow publishing.
type SubQueryFlowParams struct {
	SessionID string
	AgentID   string
	AgentName string
	TaskID    string
	Role      string
}

// EngineEmitFunc streams engine events from a nested query loop.
type EngineEmitFunc func(*EngineEvent)

// SubQueryFlowReporter publishes execution flow events for SubQuery runs.
// Implemented by D7 hubspoke; injected by D7 delegatetools into D2 nested runs.
type SubQueryFlowReporter interface {
	OnStarted(ctx context.Context, params SubQueryFlowParams, summary string)
	OnToolCall(ctx context.Context, params SubQueryFlowParams, toolName, input string)
	OnCompleted(ctx context.Context, params SubQueryFlowParams, summary string)
	OnFailed(ctx context.Context, params SubQueryFlowParams, errMsg string)
	WrapEmit(ctx context.Context, params SubQueryFlowParams, inner EngineEmitFunc) EngineEmitFunc
}
