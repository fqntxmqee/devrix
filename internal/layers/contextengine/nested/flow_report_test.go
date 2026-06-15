package nested_test

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/nested"
	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

type captureHub struct {
	events []contracts.FlowEvent
}

func (c *captureHub) Publish(_ context.Context, ev contracts.FlowEvent) {
	c.events = append(c.events, ev)
}

func (c *captureHub) Snapshot(string) contracts.WorkPlanSnapshot {
	return contracts.WorkPlanSnapshot{}
}

type testFlowReporter struct {
	hub *captureHub
}

func (r *testFlowReporter) OnStarted(ctx context.Context, params contracts.SubQueryFlowParams, summary string) {
	r.publish(ctx, params, contracts.FlowStarted, summary, nil)
}

func (r *testFlowReporter) OnToolCall(ctx context.Context, params contracts.SubQueryFlowParams, toolName, input string) {
	r.publish(ctx, params, contracts.FlowToolCall, toolName, map[string]string{
		"tool_name": toolName,
		"input":     input,
	})
}

func (r *testFlowReporter) OnCompleted(ctx context.Context, params contracts.SubQueryFlowParams, summary string) {
	r.publish(ctx, params, contracts.FlowCompleted, summary, nil)
}

func (r *testFlowReporter) OnFailed(ctx context.Context, params contracts.SubQueryFlowParams, errMsg string) {
	r.publish(ctx, params, contracts.FlowFailed, errMsg, nil)
}

func (r *testFlowReporter) WrapEmit(ctx context.Context, params contracts.SubQueryFlowParams, inner contracts.EngineEmitFunc) contracts.EngineEmitFunc {
	return inner
}

func (r *testFlowReporter) publish(ctx context.Context, params contracts.SubQueryFlowParams, kind contracts.FlowEventKind, summary string, meta map[string]string) {
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
		Role:      params.Role,
		Kind:      kind,
		Summary:   summary,
		At:        time.Now(),
		Metadata:  meta,
	})
}

func TestSubQuery_should_publish_flow_lifecycle(t *testing.T) {
	hub := &captureHub{}
	reporter := &testFlowReporter{hub: hub}

	llm := &query.SequentialLLM{Responses: []query.LLMScript{{Content: "done"}}}
	loop := &query.Loop{LLM: llm}
	parent := &types.SessionContext{SessionID: "sess_flow", Model: "test"}

	_, err := nested.Run(context.Background(), nested.LoopDeps{
		Loop:         loop,
		FlowReporter: reporter,
	}, nested.SubQueryParams{
		ParentSC:     parent,
		AgentID:      "explore_flow",
		AgentName:    "Explore",
		Role:         "explore",
		SystemPrompt: "explore",
		MaxTurns:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hub.events) < 2 {
		t.Fatalf("expected started+completed, got %d events", len(hub.events))
	}
	if hub.events[0].Kind != contracts.FlowStarted || hub.events[len(hub.events)-1].Kind != contracts.FlowCompleted {
		t.Fatalf("events = %+v", hub.events)
	}
	_ = config.DefaultExecutionFlowConfig
}
