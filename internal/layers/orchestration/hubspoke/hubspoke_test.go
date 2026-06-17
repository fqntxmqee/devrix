package hubspoke_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/execute"
	"github.com/devrix/devrix/internal/layers/orchestration/hubspoke"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// --- AgentBridge tests ---

func TestNewAgentBridge(t *testing.T) {
	b := hubspoke.NewAgentBridge(nil, "sess-1", "flow-1", "w-1", "task-1", "plan")
	if b == nil {
		t.Fatal("NewAgentBridge returned nil")
	}
}

func TestAgentBridge_OnWorkerForked_wiresObserver(t *testing.T) {
	rec := &recordingObserver{}
	ag := &testAgent{id: "worker-1"}
	b := hubspoke.NewAgentBridge(nil, "sess-1", "", "", "task-1", "plan")

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
	b := hubspoke.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan")

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
	b := hubspoke.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan")

	b.OnWorkerCompleted("w-1", "sess-1", "", errors.New("boom"))

	if len(hub.events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(hub.events))
	}
	if hub.events[0].Kind != contracts.FlowFailed {
		t.Errorf("first event kind = %s, want failed", hub.events[0].Kind)
	}
}

func TestAgentBridge_OnWorkerCompleted_nilBridge(t *testing.T) {
	var b *hubspoke.AgentBridge
	// Should not panic
	b.OnWorkerCompleted("w", "s", "summary", nil)
}

func TestAgentBridge_OnWorkerCompleted_nilHub(t *testing.T) {
	b := hubspoke.NewAgentBridge(nil, "sess-1", "flow-1", "w-1", "task-1", "plan")
	// Should not panic
	b.OnWorkerCompleted("w-1", "sess-1", "done", nil)
}

func TestAgentBridge_EmitAgentEvent(t *testing.T) {
	hub := &recordingHub{}
	b := hubspoke.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan")

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
	b := hubspoke.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan")

	b.EmitAgentEvent(multiagent.AgentEvent{EventType: "permission_required"})

	if len(hub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(hub.events))
	}
	if hub.events[0].Kind != contracts.FlowWaitingPermission {
		t.Errorf("kind = %s, want waiting_permission", hub.events[0].Kind)
	}
}

func TestAgentBridge_EmitAgentEvent_nilBridge(t *testing.T) {
	var b *hubspoke.AgentBridge
	// Should not panic
	b.EmitAgentEvent(multiagent.AgentEvent{EventType: "agent.started"})
}

func TestAgentBridge_EmitAgentEvent_unknownType(t *testing.T) {
	hub := &recordingHub{}
	b := hubspoke.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan")

	b.EmitAgentEvent(multiagent.AgentEvent{EventType: "unknown.event"})

	if len(hub.events) != 0 {
		t.Fatalf("expected 0 events for unknown type, got %d", len(hub.events))
	}
}

func TestAgentBridge_EngineEventSink_toolCall(t *testing.T) {
	hub := &recordingHub{}
	b := hubspoke.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan")
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
	b := hubspoke.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan")
	sink := b.EngineEventSink()

	sink(&contracts.EngineEvent{Type: "text", Content: "hello"})

	if len(hub.events) != 0 {
		t.Fatalf("expected 0 events for non-tool_call, got %d", len(hub.events))
	}
}

func TestAgentBridge_EngineEventSink_nilEvent(t *testing.T) {
	hub := &recordingHub{}
	b := hubspoke.NewAgentBridge(hub, "sess-1", "flow-1", "w-1", "task-1", "plan")
	sink := b.EngineEventSink()

	// Should not panic
	sink(nil)
}

// --- SubQueryBridge tests ---

func TestNewSubQueryBridge(t *testing.T) {
	hub := &recordingHub{}
	b := hubspoke.NewSubQueryBridge(hub, "sess-1", "sq-1", "task-1")
	if b == nil {
		t.Fatal("NewSubQueryBridge returned nil")
	}
}

func TestSubQueryBridge_PublishStarted(t *testing.T) {
	hub := &recordingHub{}
	b := hubspoke.NewSubQueryBridge(hub, "sess-1", "sq-1", "task-1")

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
	b := hubspoke.NewSubQueryBridge(hub, "sess-1", "sq-1", "task-1")

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
	b := hubspoke.NewSubQueryBridge(hub, "sess-1", "sq-1", "task-1")

	b.PublishFailed("error occurred")

	if len(hub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(hub.events))
	}
	if hub.events[0].Kind != contracts.FlowFailed {
		t.Errorf("kind = %s, want failed", hub.events[0].Kind)
	}
}

func TestSubQueryBridge_nilBridge(t *testing.T) {
	var b *hubspoke.SubQueryBridge
	// Should not panic
	b.PublishStarted("")
	b.PublishCompleted("")
	b.PublishFailed("")
}

func TestSubQueryBridge_nilHub(t *testing.T) {
	b := hubspoke.NewSubQueryBridge(nil, "sess-1", "sq-1", "task-1")
	// Should not panic
	b.PublishStarted("start")
	b.PublishCompleted("done")
}

// --- Dispatcher tests ---

func TestNewDispatcher(t *testing.T) {
	d := hubspoke.NewDispatcher(
		config.DelegateConfig{Enabled: true},
		nil, // executor
		nil, // subQuery
		nil, // hub → defaults to NoOp (flow.GlobalHub removed DM-20260617-008 W2)
		nil, // leaderRes
	)
	if d == nil {
		t.Fatal("NewDispatcher returned nil")
	}
}

func TestDispatcher_Dispatch_D4_enabled_withLeader(t *testing.T) {
	leader := &testAgent{id: "leader-1", cfg: multiagent.AgentConfig{SessionID: "sess-1", WorkDir: "/tmp"}}
	res := &staticLeaderResolver{leader: leader, ok: true}
	exec := &stubExecutor{result: execute.WorkerResult{WorkerID: "w-1", Summary: "done"}}
	hub := &recordingHub{}
	d := hubspoke.NewDispatcher(config.DelegateConfig{Enabled: true}, exec, nil, hub, res)

	result, err := d.Dispatch(context.Background(), hubspoke.DispatchRequest{
		SessionID: "sess-1",
		Role:      "plan",
		Directive: "plan it",
		TaskID:    "task-1",
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.WorkerID != "w-1" {
		t.Errorf("WorkerID = %s, want w-1", result.WorkerID)
	}
}

func TestDispatcher_Dispatch_D4_async(t *testing.T) {
	leader := &testAgent{id: "leader-1", cfg: multiagent.AgentConfig{SessionID: "sess-1", WorkDir: "/tmp"}}
	res := &staticLeaderResolver{leader: leader, ok: true}
	exec := &stubExecutor{workerID: "w-async"}
	hub := &recordingHub{}
	d := hubspoke.NewDispatcher(config.DelegateConfig{Enabled: true}, exec, nil, hub, res)

	result, err := d.Dispatch(context.Background(), hubspoke.DispatchRequest{
		SessionID: "sess-1",
		Role:      "plan",
		Directive: "async plan",
		TaskID:    "task-1",
		Async:     true,
	})
	if err != nil {
		t.Fatalf("Dispatch async: %v", err)
	}
	if result.WorkerID != "w-async" {
		t.Errorf("WorkerID = %s, want w-async", result.WorkerID)
	}
}

func TestDispatcher_Dispatch_D4_disabled_fallsToD2(t *testing.T) {
	res := &staticLeaderResolver{leader: nil, ok: true}
	sub := &stubSubQueryRunner{summary: "subquery result"}
	hub := &recordingHub{}
	d := hubspoke.NewDispatcher(config.DelegateConfig{Enabled: false}, nil, sub, hub, res)

	result, err := d.Dispatch(context.Background(), hubspoke.DispatchRequest{
		SessionID: "sess-1",
		ParentSC:  &types.SessionContext{SessionID: "sess-1"},
		Role:      "plan",
		Directive: "plan via subquery",
		TaskID:    "task-1",
	})
	if err != nil {
		t.Fatalf("Dispatch D2: %v", err)
	}
	if result.Summary != "subquery result" {
		t.Errorf("summary = %s, want 'subquery result'", result.Summary)
	}
}

func TestDispatcher_Dispatch_D4_enabled_noLeader_fallsToD2(t *testing.T) {
	res := &staticLeaderResolver{leader: nil, ok: false}
	sub := &stubSubQueryRunner{summary: "d2 fallback"}
	hub := &recordingHub{}
	d := hubspoke.NewDispatcher(config.DelegateConfig{Enabled: true}, nil, sub, hub, res)

	result, err := d.Dispatch(context.Background(), hubspoke.DispatchRequest{
		SessionID: "sess-1",
		ParentSC:  &types.SessionContext{SessionID: "sess-1"},
		Role:      "implement",
		Directive: "code it",
		TaskID:    "task-2",
	})
	if err != nil {
		t.Fatalf("Dispatch D2 fallback: %v", err)
	}
	if result.Summary != "d2 fallback" {
		t.Errorf("summary = %s, want 'd2 fallback'", result.Summary)
	}
}

func TestDispatcher_Dispatch_noFallback(t *testing.T) {
	res := &staticLeaderResolver{leader: nil, ok: false}
	hub := &recordingHub{}
	d := hubspoke.NewDispatcher(config.DelegateConfig{Enabled: false}, nil, nil, hub, res)

	_, err := d.Dispatch(context.Background(), hubspoke.DispatchRequest{
		SessionID: "sess-1",
		Role:      "plan",
		TaskID:    "task-1",
	})
	if err == nil {
		t.Fatal("expected error with no fallback available")
	}
}

func TestDispatcher_Hub(t *testing.T) {
	hub := &recordingHub{}
	d := hubspoke.NewDispatcher(config.DelegateConfig{}, nil, nil, hub, nil)
	if d.Hub() != hub {
		t.Fatal("Hub() returned wrong hub")
	}
}

// --- Helpers ---

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

type testAgent struct {
	id       string
	cfg      multiagent.AgentConfig
	observer multiagent.AgentObserver
	sink     func(*contracts.EngineEvent)
}

func (a *testAgent) ID() string                                      { return a.id }
func (a *testAgent) State() multiagent.AgentState                    { return multiagent.AgentStateCreated }
func (a *testAgent) Config() multiagent.AgentConfig                  { return a.cfg }
func (a *testAgent) GetMessages() []types.Message                    { return nil }
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

type staticLeaderResolver struct {
	leader multiagent.Agent
	ok     bool
}

func (s *staticLeaderResolver) Leader(string) (multiagent.Agent, bool) { return s.leader, s.ok }

type stubExecutor struct {
	result   execute.WorkerResult
	err      error
	workerID string
}

func (s *stubExecutor) ExecuteSync(ctx context.Context, leader multiagent.Agent, spec execute.WorkerRunSpec) (execute.WorkerResult, error) {
	return s.result, s.err
}
func (s *stubExecutor) ExecuteAsync(ctx context.Context, leader multiagent.Agent, spec execute.WorkerRunSpec) (string, error) {
	return s.workerID, nil
}

type stubSubQueryRunner struct {
	summary string
	err     error
}

func (s *stubSubQueryRunner) RunSubQuery(ctx context.Context, parent *types.SessionContext, role, directive, taskID string, maxTurns int) (string, error) {
	return s.summary, s.err
}
