package bridge_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/bridge"
)

// recordingHub captures FlowEvents published during tests. Moved from
// internal/layers/orchestration/hubspoke/hubspoke_test.go when hubspoke/ was
// retired as a package boundary (the directory only held tests after
// PR-C2 migrated all hubspoke.* production code to sessionorchestrator/ +
// bridge/).
type recordingHub struct {
	mu     sync.Mutex
	events []contracts.FlowEvent
}

func (h *recordingHub) Publish(ctx context.Context, ev contracts.FlowEvent) {
	h.mu.Lock()
	h.events = append(h.events, ev)
	h.mu.Unlock()
}
func (h *recordingHub) Snapshot(string) contracts.WorkPlanSnapshot { return contracts.WorkPlanSnapshot{} }

type recordingObserver struct {
	forkCalls     int
	completeCalls int
}

func (r *recordingObserver) OnWorkerForked(string, string, multiagent.Agent) { r.forkCalls++ }
func (r *recordingObserver) OnWorkerCompleted(string, string, string, error) { r.completeCalls++ }

// testAgent is a minimal multiagent.Agent stub. Moved from hubspoke_test.go.
type testAgent struct {
	id       string
	cfg      multiagent.AgentConfig
	observer multiagent.AgentObserver
	sink     func(*contracts.EngineEvent)
}

func (a *testAgent) ID() string                                      { return a.id }
func (a *testAgent) State() multiagent.AgentState                    { return multiagent.AgentStateCreated }
func (a *testAgent) Config() multiagent.AgentConfig                  { return a.cfg }
func (a *testAgent) GetMessages() []types.Message                   { return nil }
func (a *testAgent) ResolvePermission(string, bool)                  {}
func (a *testAgent) Terminate(context.Context) error                 { return nil }
func (a *testAgent) SetAgentObserver(o multiagent.AgentObserver)     { a.observer = o }
func (a *testAgent) SetEngineEventSink(f func(*contracts.EngineEvent)) { a.sink = f }
func (a *testAgent) Run(ctx context.Context) (*multiagent.AgentResult, error) {
	return &multiagent.AgentResult{ExitCode: 0}, nil
}
func (a *testAgent) Wait(ctx context.Context) (*multiagent.AgentResult, error) {
	return nil, nil
}
func (a *testAgent) Fork(ctx context.Context, cfg multiagent.AgentConfig) (multiagent.Agent, error) {
	return &testAgent{id: "child", cfg: cfg}, nil
}
func (a *testAgent) Join(ctx context.Context, child multiagent.Agent) error { return nil }

func TestNewAgentBridge(t *testing.T) {
	b := bridge.NewAgentBridge(nil, "sess-1", "flow-1", "w-1", "task-1", "plan", nil)
	if b == nil {
		t.Fatal("NewAgentBridge returned nil")
	}
}

func TestAgentBridge_OnWorkerForked_wiresObserver(t *testing.T) {
	rec := &recordingObserver{}
	ag := &testAgent{id: "worker-1"}
	b := bridge.NewAgentBridge(nil, "sess-1", "", "", "task-1", "plan", nil)

	b.OnWorkerForked(ag.ID(), "sess-1", ag)

	if ag.observer != b {
		t.Fatal("agent observer was not wired to bridge")
	}
	if ag.sink == nil {
		t.Fatal("engine event sink was not wired")
	}
	_ = rec
}

func TestAgentBridge_OnWorkerCompleted_success(t *testing.T) {
	hub := &recordingHub{}
	b := bridge.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan", nil)

	b.OnWorkerCompleted("w-1", "sess-1", "all done", nil)

	if len(hub.events) != 2 {
		t.Fatalf("expected 2 events (completed + joined), got %d", len(hub.events))
	}
	if hub.events[0].Kind != contracts.FlowCompleted {
		t.Errorf("first event kind = %s, want completed", hub.events[0].Kind)
	}
	if hub.events[1].Kind != contracts.FlowJoined {
		t.Errorf("second event kind = %s, want joined", hub.events[1].Kind)
	}
}

func TestAgentBridge_OnWorkerCompleted_error(t *testing.T) {
	hub := &recordingHub{}
	b := bridge.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan", nil)

	b.OnWorkerCompleted("w-1", "sess-1", "", errors.New("boom"))

	if len(hub.events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(hub.events))
	}
	if hub.events[0].Kind != contracts.FlowFailed {
		t.Errorf("first event kind = %s, want failed", hub.events[0].Kind)
	}
}

func TestAgentBridge_OnWorkerCompleted_nilBridge(t *testing.T) {
	var b *bridge.AgentBridge
	// Should not panic
	b.OnWorkerCompleted("w", "s", "summary", nil)
}

func TestAgentBridge_OnWorkerCompleted_nilHub(t *testing.T) {
	b := bridge.NewAgentBridge(nil, "sess-1", "flow-1", "w-1", "task-1", "plan", nil)
	// Should not panic
	b.OnWorkerCompleted("w-1", "sess-1", "done", nil)
}

func TestAgentBridge_EmitAgentEvent(t *testing.T) {
	hub := &recordingHub{}
	b := bridge.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan", nil)

	b.EmitAgentEvent(multiagent.AgentEvent{EventType: "agent.started"})

	if len(hub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(hub.events))
	}
	if hub.events[0].Kind != contracts.FlowStarted {
		t.Errorf("kind = %s, want started", hub.events[0].Kind)
	}
}

func TestAgentBridge_EmitAgentEvent_permission(t *testing.T) {
	hub := &recordingHub{}
	b := bridge.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan", nil)

	b.EmitAgentEvent(multiagent.AgentEvent{EventType: "permission_required"})

	if len(hub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(hub.events))
	}
	if hub.events[0].Kind != contracts.FlowWaitingPermission {
		t.Errorf("kind = %s, want waiting_permission", hub.events[0].Kind)
	}
}

func TestAgentBridge_EmitAgentEvent_nilBridge(t *testing.T) {
	var b *bridge.AgentBridge
	// Should not panic
	b.EmitAgentEvent(multiagent.AgentEvent{EventType: "agent.started"})
}

func TestAgentBridge_EmitAgentEvent_unknownType(t *testing.T) {
	hub := &recordingHub{}
	b := bridge.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan", nil)

	b.EmitAgentEvent(multiagent.AgentEvent{EventType: "unknown.event"})

	if len(hub.events) != 0 {
		t.Fatalf("expected 0 events for unknown type, got %d", len(hub.events))
	}
}

func TestAgentBridge_EngineEventSink_toolCall(t *testing.T) {
	hub := &recordingHub{}
	b := bridge.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan", nil)
	sink := b.EngineEventSink()

	sink(&contracts.EngineEvent{Type: "tool_call", ToolName: "read_file"})

	if len(hub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(hub.events))
	}
	if hub.events[0].Kind != contracts.FlowToolCall {
		t.Errorf("kind = %s, want tool_call", hub.events[0].Kind)
	}
}

func TestAgentBridge_EngineEventSink_nonToolCall(t *testing.T) {
	hub := &recordingHub{}
	b := bridge.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan", nil)
	sink := b.EngineEventSink()

	sink(&contracts.EngineEvent{Type: "text", Content: "hello"})

	if len(hub.events) != 0 {
		t.Fatalf("expected 0 events for non-tool_call, got %d", len(hub.events))
	}
}

func TestAgentBridge_EngineEventSink_nilEvent(t *testing.T) {
	hub := &recordingHub{}
	b := bridge.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan", nil)
	sink := b.EngineEventSink()

	// Should not panic
	sink(nil)
}
