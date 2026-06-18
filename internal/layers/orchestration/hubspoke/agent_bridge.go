package hubspoke

import (
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/orchestration/runregistry"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// AgentBridge publishes D4 worker lifecycle and engine events to ExecutionFlowHub.
// It implements both multiagent.AgentObserver and execute.WorkerObserver,
// and is wired by SpokeDispatcher before calling WorkerExecutor.
//
// DSAFT: D7-S4 (SpokeBridge) — migrated from D4 delegate/bridge.go
type AgentBridge struct {
	hub      contracts.ExecutionFlowHub
	session  string
	flowID   string
	taskID   string
	workerID string
	role     string
}

// NewAgentBridge creates a FlowBridge for a D4 delegated worker.
func NewAgentBridge(
	hub contracts.ExecutionFlowHub,
	sessionID, flowID, workerID, taskID, role string,
) *AgentBridge {
	return &AgentBridge{
		hub:      hub,
		session:  sessionID,
		flowID:   flowID,
		workerID: workerID,
		taskID:   taskID,
		role:     role,
	}
}

// OnWorkerForked implements execute.WorkerObserver — wires this bridge
// as the agent's lifecycle observer and engine event sink, and patches
// the workerID/flowID from the actual forked agent.
func (b *AgentBridge) OnWorkerForked(workerID, sessionID string, agent multiagent.Agent) {
	b.workerID = workerID
	b.flowID = workerID
	if b.session == "" {
		b.session = sessionID
	}
	if agent != nil {
		agent.SetAgentObserver(b)
		agent.SetEngineEventSink(b.EngineEventSink())
	}
}

// OnWorkerCompleted implements execute.WorkerObserver — publishes terminal flow events.
func (b *AgentBridge) OnWorkerCompleted(workerID, sessionID string, summary string, runErr error) {
	if b == nil || b.hub == nil {
		return
	}
	runregistry.CompleteByWorkItem(sessionID, b.taskID, summary, runErr)
	kind := contracts.FlowCompleted
	if runErr != nil {
		kind = contracts.FlowFailed
	}
	b.hub.Publish(nil, contracts.FlowEvent{
		SessionID: sessionID,
		FlowID:    workerID,
		TaskID:    b.taskID,
		WorkerID:  workerID,
		Source:    contracts.ExecutionSourceD4Worker,
		Role:      b.role,
		Kind:      kind,
		Summary:   summary,
	})
	b.hub.Publish(nil, contracts.FlowEvent{
		SessionID: sessionID,
		FlowID:    workerID,
		TaskID:    b.taskID,
		WorkerID:  workerID,
		Source:    contracts.ExecutionSourceD4Worker,
		Role:      b.role,
		Kind:      contracts.FlowJoined,
		Summary:   summary,
	})
}

// EmitAgentEvent implements multiagent.AgentObserver.
func (b *AgentBridge) EmitAgentEvent(ev multiagent.AgentEvent) {
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
		Role:      b.role,
		Kind:      kind,
		Summary:   ev.EventType,
		At:        time.Now(),
	})
}

// EngineEventSink returns a callback for worker engine streaming events.
func (b *AgentBridge) EngineEventSink() func(*contracts.EngineEvent) {
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
				Role:      b.role,
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
