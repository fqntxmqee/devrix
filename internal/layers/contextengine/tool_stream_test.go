package contextengine

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
)

func TestWithToolStreamEmitter_should_forward_agent_events(t *testing.T) {
	var events []*gateway.EngineEvent
	emit := func(ev *gateway.EngineEvent) {
		events = append(events, ev)
	}
	ctx := withToolStreamEmitter(context.Background(), emit, "sess_1", "call_claude-code")
	emitFn := ToolStreamEmitterFromContext(ctx)
	if emitFn == nil {
		t.Fatal("expected stream emitter in context")
	}

	emitFn(ToolStreamEvent{Type: "thinking", Content: "plan", ToolName: "Claude Code"})
	emitFn(ToolStreamEvent{Type: "text", Content: "hi", ToolName: "Claude Code"})

	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].Type != "thinking" || events[0].Content != "plan" {
		t.Fatalf("thinking event = %+v", events[0])
	}
	if events[0].Metadata["source"] != "agent_tool" {
		t.Fatalf("metadata = %+v", events[0].Metadata)
	}
	if events[1].Type != "text" || events[1].Content != "hi" {
		t.Fatalf("text event = %+v", events[1])
	}
}
