package hubspoke

import (
	"github.com/devrix/devrix/internal/shared/contracts"
)

// SubQueryBridge publishes D2 SubQuery flow events through the single D7 export.
//
// Unlike AgentBridge (which wires into AgentObserver), SubQueryBridge is a
// lightweight wrapper that D7 SpokeDispatcher injects into SubQuery execution
// so that D2 nested/builtin code never calls hub.Publish directly.
//
// DSAFT: D7-S4-A05 (SubQueryBridge) — v2.0-c target
type SubQueryBridge struct {
	hub        contracts.ExecutionFlowHub
	sessionID  string
	subQueryID string
	taskID     string
}

// NewSubQueryBridge creates a flow publisher for a SubQuery run.
func NewSubQueryBridge(hub contracts.ExecutionFlowHub, sessionID, subQueryID, taskID string) *SubQueryBridge {
	return &SubQueryBridge{
		hub:        hub,
		sessionID:  sessionID,
		subQueryID: subQueryID,
		taskID:     taskID,
	}
}

// PublishStarted emits a FlowStarted event for a SubQuery.
func (b *SubQueryBridge) PublishStarted(summary string) {
	if b == nil || b.hub == nil {
		return
	}
	b.hub.Publish(nil, contracts.FlowEvent{
		SessionID: b.sessionID,
		FlowID:    b.subQueryID,
		TaskID:    b.taskID,
		WorkerID:  b.subQueryID,
		Source:    contracts.ExecutionSourceSubQuery,
		Kind:      contracts.FlowStarted,
		Summary:   summary,
	})
}

// PublishCompleted emits a FlowCompleted event for a SubQuery.
func (b *SubQueryBridge) PublishCompleted(summary string) {
	if b == nil || b.hub == nil {
		return
	}
	b.hub.Publish(nil, contracts.FlowEvent{
		SessionID: b.sessionID,
		FlowID:    b.subQueryID,
		TaskID:    b.taskID,
		WorkerID:  b.subQueryID,
		Source:    contracts.ExecutionSourceSubQuery,
		Kind:      contracts.FlowCompleted,
		Summary:   summary,
	})
}

// PublishFailed emits a FlowFailed event for a SubQuery.
func (b *SubQueryBridge) PublishFailed(summary string) {
	if b == nil || b.hub == nil {
		return
	}
	b.hub.Publish(nil, contracts.FlowEvent{
		SessionID: b.sessionID,
		FlowID:    b.subQueryID,
		TaskID:    b.taskID,
		WorkerID:  b.subQueryID,
		Source:    contracts.ExecutionSourceSubQuery,
		Kind:      contracts.FlowFailed,
		Summary:   summary,
	})
}

// Hub returns the underlying ExecutionFlowHub for direct snapshot access.
func (b *SubQueryBridge) Hub() contracts.ExecutionFlowHub {
	if b == nil {
		return nil
	}
	return b.hub
}
