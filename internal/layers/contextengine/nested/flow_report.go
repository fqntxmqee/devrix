package nested

import (
	"context"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/shared/contracts"
)

func resolveFlowHub(params SubQueryParams, deps LoopDeps) contracts.ExecutionFlowHub {
	if params.FlowHub != nil {
		return params.FlowHub
	}
	return deps.FlowHub
}

func subQueryRole(params SubQueryParams) string {
	if params.Role != "" {
		return params.Role
	}
	return params.AgentName
}

func publishSubQueryFlow(
	ctx context.Context,
	hub contracts.ExecutionFlowHub,
	params SubQueryParams,
	kind contracts.FlowEventKind,
	summary string,
	meta map[string]string,
) {
	if hub == nil {
		return
	}
	flowID := params.AgentID
	if flowID == "" {
		flowID = params.AgentName
	}
	if meta == nil {
		meta = map[string]string{}
	}
	hub.Publish(ctx, contracts.FlowEvent{
		SessionID: params.ParentSC.SessionID,
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

func subQueryFlowEmit(
	ctx context.Context,
	hub contracts.ExecutionFlowHub,
	params SubQueryParams,
	inner query.EmitFunc,
) query.EmitFunc {
	if hub == nil {
		return inner
	}
	return func(ev *contracts.EngineEvent) {
		if inner != nil {
			inner(ev)
		}
		if ev == nil {
			return
		}
		switch ev.Type {
		case "tool_call":
			tool := ev.ToolName
			if tool == "" {
				tool = ev.Metadata["tool_name"]
			}
			publishSubQueryFlow(ctx, hub, params, contracts.FlowToolCall, tool, map[string]string{
				"tool_name": tool,
				"input":     ev.Metadata["input"],
			})
		}
	}
}
