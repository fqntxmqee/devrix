package contracts

import (
	"context"
	"time"
)

// ExecutionSource identifies where a flow event originated.
type ExecutionSource string

const (
	ExecutionSourceSubQuery ExecutionSource = "subquery"
	ExecutionSourceD4Worker ExecutionSource = "d4_worker"
)

// FlowEventKind classifies execution flow lifecycle events.
type FlowEventKind string

const (
	FlowForked            FlowEventKind = "forked"
	FlowStarted           FlowEventKind = "started"
	FlowIterating         FlowEventKind = "iterating"
	FlowToolCall          FlowEventKind = "tool_call"
	FlowWaitingPermission FlowEventKind = "waiting_permission"
	FlowProgress          FlowEventKind = "progress"
	FlowCompleted         FlowEventKind = "completed"
	FlowFailed            FlowEventKind = "failed"
	FlowJoined            FlowEventKind = "joined"
)

// FlowEvent is a structured sub-agent progress event (Hub-Spoke v2).
type FlowEvent struct {
	SessionID string
	FlowID    string
	TaskID    string
	WorkerID  string
	Source    ExecutionSource
	Role      string
	Kind      FlowEventKind
	Summary   string
	At        time.Time
	Metadata  map[string]string
}

// ExecutionFlowSnapshot is a point-in-time view of one worker flow.
type ExecutionFlowSnapshot struct {
	FlowID       string
	WorkerID     string
	TaskID       string
	Source       ExecutionSource
	Role         string
	Status       string
	Directive    string
	LastEvent    FlowEvent
	RecentEvents []FlowEvent
}

// TaskSnapshot is a read-model projection of a D2 task node.
type TaskSnapshot struct {
	ID      string
	Subject string
	Status  string
	Owner   string
}

// WorkPlanSnapshot aggregates planning and execution state for a session.
type WorkPlanSnapshot struct {
	SessionID      string
	Tasks          []TaskSnapshot
	ExecutionFlows []ExecutionFlowSnapshot
	UpdatedAt      time.Time
}

// ExecutionFlowHub publishes sub-agent progress to Leader cognition and IM.
type ExecutionFlowHub interface {
	Publish(ctx context.Context, ev FlowEvent)
	Snapshot(sessionID string) WorkPlanSnapshot
}

// NoOpExecutionFlowHub discards flow events.
type NoOpExecutionFlowHub struct{}

func (NoOpExecutionFlowHub) Publish(context.Context, FlowEvent) {}

func (NoOpExecutionFlowHub) Snapshot(string) WorkPlanSnapshot {
	return WorkPlanSnapshot{}
}
