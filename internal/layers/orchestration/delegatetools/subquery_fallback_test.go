package delegatetools

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/nested"
	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/layers/multiagent/delegate"
	"github.com/devrix/devrix/internal/layers/orchestration/flow"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

type captureFlowHub struct {
	events []contracts.FlowEvent
}

func (h *captureFlowHub) Publish(_ context.Context, ev contracts.FlowEvent) {
	h.events = append(h.events, ev)
}

func (h *captureFlowHub) Snapshot(string) contracts.WorkPlanSnapshot {
	return contracts.WorkPlanSnapshot{}
}

// T: D4-S10-A01-T08
func TestSubQueryFallback_should_publish_flow_events_when_d4_disabled(t *testing.T) {
	hub := &captureFlowHub{}
	prev := flow.GlobalHub
	flow.SetGlobalHub(hub)
	defer flow.SetGlobalHub(prev)

	loop := &query.Loop{
		LLM:        &query.SequentialLLM{Responses: []query.LLMScript{{Content: "fallback summary"}}},
		Permission: query.AllowPermission{},
	}
	adapter := &SubQueryFallback{LoopDeps: nested.LoopDeps{Loop: loop}}
	parent := &types.SessionContext{SessionID: "sess_fb", WorkDir: t.TempDir(), Model: "test"}

	_, err := adapter.RunSubQuery(context.Background(), parent, delegate.WorkerSpec{
		Role:      delegate.WorkerRoleExplore,
		Directive: "scan repo",
		TaskID:    "task_fb",
	})
	if err != nil {
		t.Fatalf("RunSubQuery: %v", err)
	}
	if len(hub.events) == 0 {
		t.Fatal("expected subquery flow events on fallback path")
	}
	foundStarted := false
	for _, ev := range hub.events {
		if ev.Kind == contracts.FlowStarted {
			foundStarted = true
		}
		if ev.Source != contracts.ExecutionSourceSubQuery {
			t.Fatalf("source = %q, want subquery", ev.Source)
		}
	}
	if !foundStarted {
		t.Fatalf("events = %+v", hub.events)
	}
}
