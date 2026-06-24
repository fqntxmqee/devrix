package bridge_test

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/bridge"
)

// SubQueryBridge tests. Moved from internal/layers/orchestration/hubspoke/
// when hubspoke/ was retired as a package boundary. recordingHub is defined
// in agent_bridge_test.go (same package).

func TestNewSubQueryBridge(t *testing.T) {
	hub := &recordingHub{}
	b := bridge.NewSubQueryBridge(hub, "sess-1", "sq-1", "task-1")
	if b == nil {
		t.Fatal("NewSubQueryBridge returned nil")
	}
}

func TestSubQueryBridge_PublishStarted(t *testing.T) {
	hub := &recordingHub{}
	b := bridge.NewSubQueryBridge(hub, "sess-1", "sq-1", "task-1")

	b.PublishStarted("subquery began")

	if len(hub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(hub.events))
	}
	ev := hub.events[0]
	if ev.Kind != contracts.FlowStarted {
		t.Errorf("kind = %s, want started", ev.Kind)
	}
	if ev.Source != contracts.ExecutionSourceSubQuery {
		t.Errorf("source = %s, want subquery", ev.Source)
	}
	if ev.Summary != "subquery began" {
		t.Errorf("summary = %s, want 'subquery began'", ev.Summary)
	}
}

func TestSubQueryBridge_PublishCompleted(t *testing.T) {
	hub := &recordingHub{}
	b := bridge.NewSubQueryBridge(hub, "sess-1", "sq-1", "task-1")

	b.PublishCompleted("done")

	if len(hub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(hub.events))
	}
	if hub.events[0].Kind != contracts.FlowCompleted {
		t.Errorf("kind = %s, want completed", hub.events[0].Kind)
	}
}

func TestSubQueryBridge_PublishFailed(t *testing.T) {
	hub := &recordingHub{}
	b := bridge.NewSubQueryBridge(hub, "sess-1", "sq-1", "task-1")

	b.PublishFailed("error occurred")

	if len(hub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(hub.events))
	}
	if hub.events[0].Kind != contracts.FlowFailed {
		t.Errorf("kind = %s, want failed", hub.events[0].Kind)
	}
}

func TestSubQueryBridge_nilBridge(t *testing.T) {
	var b *bridge.SubQueryBridge
	// Should not panic
	b.PublishStarted("")
	b.PublishCompleted("")
	b.PublishFailed("")
}

func TestSubQueryBridge_nilHub(t *testing.T) {
	b := bridge.NewSubQueryBridge(nil, "sess-1", "sq-1", "task-1")
	// Should not panic
	b.PublishStarted("start")
	b.PublishCompleted("done")
}
