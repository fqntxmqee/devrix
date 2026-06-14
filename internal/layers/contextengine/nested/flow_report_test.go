package nested_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/layers/contextengine/nested"
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

func TestSubQuery_should_publish_flow_lifecycle(t *testing.T) {
	hub := &captureHub{}

	llm := &query.SequentialLLM{Responses: []query.LLMScript{{Content: "done"}}}
	loop := &query.Loop{LLM: llm}
	parent := &types.SessionContext{SessionID: "sess_flow", Model: "test"}

	_, err := nested.Run(context.Background(), nested.LoopDeps{Loop: loop, FlowHub: hub}, nested.SubQueryParams{
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
