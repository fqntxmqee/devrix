package delegatetools

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/orchestration/hubspoke"
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

type stubSubTurn struct {
	text string
}

func (s *stubSubTurn) RunSubTurn(_ context.Context, _ contracts.SubTurnRequest) (*contracts.SubTurnResult, error) {
	return &contracts.SubTurnResult{AssistantText: s.text}, nil
}

// T: D4-S10-A01-T08 (legacy; canonical → D7-S4)
func TestSubQueryRunner_should_publish_flow_events_when_d4_disabled(t *testing.T) {
	hub := &captureFlowHub{}

	adapter := &SubQueryRunner{LoopDeps: enforce.SubQueryDeps{
		SubTurn:      &stubSubTurn{text: "fallback summary"},
		FlowReporter: hubspoke.NewFlowReporter(hub),
	}}
	parent := &types.SessionContext{SessionID: "sess_fb", WorkDir: t.TempDir(), Model: "test"}

	_, err := adapter.RunSubQuery(context.Background(), parent, "explore", "scan repo", "task_fb", 0, "")
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
