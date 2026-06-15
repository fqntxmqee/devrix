package hubspoke

import (
	"context"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// FlowReporter publishes SubQuery lifecycle events through D7 ExecutionFlowHub.
type FlowReporter struct {
	hub contracts.ExecutionFlowHub
}

// NewFlowReporter creates a SubQuery flow reporter backed by hub.
func NewFlowReporter(hub contracts.ExecutionFlowHub) *FlowReporter {
	return &FlowReporter{hub: hub}
}

func (r *FlowReporter) publish(ctx context.Context, params contracts.SubQueryFlowParams, kind contracts.FlowEventKind, summary string, meta map[string]string) {
	if r == nil || r.hub == nil {
		return
	}
	flowID := params.AgentID
	if flowID == "" {
		flowID = params.AgentName
	}
	if meta == nil {
		meta = map[string]string{}
	}
	r.hub.Publish(ctx, contracts.FlowEvent{
		SessionID: params.SessionID,
		FlowID:    flowID,
		TaskID:    params.TaskID,
		WorkerID:  params.AgentID,
		Source:    contracts.ExecutionSourceSubQuery,
		Role:      subQueryRole(params),
		Kind:      kind,
		Summary:   summary,
		At:        time.Now(),
		Metadata:  meta,
	})
}

func subQueryRole(params contracts.SubQueryFlowParams) string {
	if params.Role != "" {
		return params.Role
	}
	return params.AgentName
}

// OnStarted implements contracts.SubQueryFlowReporter.
func (r *FlowReporter) OnStarted(ctx context.Context, params contracts.SubQueryFlowParams, summary string) {
	r.publish(ctx, params, contracts.FlowStarted, summary, nil)
}

// OnToolCall implements contracts.SubQueryFlowReporter.
func (r *FlowReporter) OnToolCall(ctx context.Context, params contracts.SubQueryFlowParams, toolName, input string) {
	r.publish(ctx, params, contracts.FlowToolCall, toolName, map[string]string{
		"tool_name": toolName,
		"input":     input,
	})
}

// OnCompleted implements contracts.SubQueryFlowReporter.
func (r *FlowReporter) OnCompleted(ctx context.Context, params contracts.SubQueryFlowParams, summary string) {
	r.publish(ctx, params, contracts.FlowCompleted, summary, nil)
}

// OnFailed implements contracts.SubQueryFlowReporter.
func (r *FlowReporter) OnFailed(ctx context.Context, params contracts.SubQueryFlowParams, errMsg string) {
	r.publish(ctx, params, contracts.FlowFailed, errMsg, nil)
}

// WrapEmit implements contracts.SubQueryFlowReporter.
func (r *FlowReporter) WrapEmit(ctx context.Context, params contracts.SubQueryFlowParams, inner contracts.EngineEmitFunc) contracts.EngineEmitFunc {
	if r == nil || r.hub == nil {
		return inner
	}
	return func(ev *contracts.EngineEvent) {
		if inner != nil {
			inner(ev)
		}
		if ev == nil {
			return
		}
		if ev.Type != "tool_call" {
			return
		}
		tool := ev.ToolName
		if tool == "" {
			tool = ev.Metadata["tool_name"]
		}
		r.OnToolCall(ctx, params, tool, ev.Metadata["input"])
	}
}

// FlowParamsFromSubQuery maps nested SubQuery identity to flow params.
func FlowParamsFromSubQuery(sessionID, agentID, agentName, taskID, role string) contracts.SubQueryFlowParams {
	return contracts.SubQueryFlowParams{
		SessionID: sessionID,
		AgentID:   agentID,
		AgentName: agentName,
		TaskID:    taskID,
		Role:      role,
	}
}
