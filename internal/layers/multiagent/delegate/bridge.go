package delegate

import (
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/orchestration/flow"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// FlowBridge publishes D4 worker lifecycle and engine events to ExecutionFlowHub.
//
// Deprecated: migrated to D7 hubspoke.AgentBridge (v2.0-b).
// New code should wire AgentBridge via hubspoke.Dispatcher instead of
// constructing FlowBridge directly.
// Existing callers in delegate.Service are preserved for backward compat
// and will be removed in the re-export cleanup cycle (v2.0-e).
type FlowBridge struct {
	hub      contracts.ExecutionFlowHub
	session  string
	flowID   string
	taskID   string
	workerID string
	role     WorkerRole
}

// NewFlowBridge creates an observer for a delegated worker.
func NewFlowBridge(
	hub contracts.ExecutionFlowHub,
	sessionID, flowID, workerID, taskID string,
	role WorkerRole,
) *FlowBridge {
	if hub == nil {
		hub = flow.GlobalHub
	}
	return &FlowBridge{
		hub:      hub,
		session:  sessionID,
		flowID:   flowID,
		workerID: workerID,
		taskID:   taskID,
		role:     role,
	}
}

// EmitAgentEvent implements multiagent.AgentObserver.
func (b *FlowBridge) EmitAgentEvent(ev multiagent.AgentEvent) {
	if b == nil || b.hub == nil {
		return
	}
	kind := mapAgentEventKind(ev.EventType)
	if kind == "" {
		return
	}
	b.hub.Publish(nil, contracts.FlowEvent{
		SessionID: b.session,
		FlowID:    b.flowID,
		TaskID:    b.taskID,
		WorkerID:  b.workerID,
		Source:    contracts.ExecutionSourceD4Worker,
		Role:      string(b.role),
		Kind:      kind,
		Summary:   ev.EventType,
		At:        time.Now(),
	})
}

// EngineEventSink returns a callback for worker engine streaming events.
func (b *FlowBridge) EngineEventSink() func(*contracts.EngineEvent) {
	return func(ev *contracts.EngineEvent) {
		if b == nil || b.hub == nil || ev == nil {
			return
		}
		switch ev.Type {
		case "tool_call":
			tool := ev.ToolName
			if tool == "" {
				tool = ev.Metadata["tool_name"]
			}
			b.hub.Publish(nil, contracts.FlowEvent{
				SessionID: b.session,
				FlowID:    b.flowID,
				TaskID:    b.taskID,
				WorkerID:  b.workerID,
				Source:    contracts.ExecutionSourceD4Worker,
				Role:      string(b.role),
				Kind:      contracts.FlowToolCall,
				Summary:   tool,
				At:        time.Now(),
				Metadata:  map[string]string{"tool_name": tool},
			})
		}
	}
}

func mapAgentEventKind(eventType string) contracts.FlowEventKind {
	switch eventType {
	case "agent.forked":
		return contracts.FlowForked
	case "agent.started":
		return contracts.FlowStarted
	case "agent.iterating":
		return contracts.FlowIterating
	case "agent.terminated":
		return contracts.FlowCompleted
	case "agent.error":
		return contracts.FlowFailed
	case "agent.joined":
		return contracts.FlowJoined
	default:
		if strings.Contains(eventType, "permission") {
			return contracts.FlowWaitingPermission
		}
		return ""
	}
}
