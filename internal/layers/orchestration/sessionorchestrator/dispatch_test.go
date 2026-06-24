package sessionorchestrator_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/execute"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
)

// Dispatcher tests. Moved from internal/layers/orchestration/hubspoke/
// when hubspoke/ was retired as a package boundary (the directory only held
// tests after PR-C2 migrated all hubspoke.* production code to
// sessionorchestrator/ + bridge/).

// recordingHub is intentionally duplicated here rather than imported from
// bridge_test, because bridge_test uses *_test package suffix and the dispatcher
// tests run in sessionorchestrator_test — cross-package reuse would require
// exporting the helper.
type recordingHub struct {
	events []contracts.FlowEvent
}

func (h *recordingHub) Publish(ctx context.Context, ev contracts.FlowEvent) {
	h.events = append(h.events, ev)
}
func (h *recordingHub) Snapshot(string) contracts.WorkPlanSnapshot {
	return contracts.WorkPlanSnapshot{}
}

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

func (s *stubSubQueryRunner) RunSubQuery(ctx context.Context, parent *types.SessionContext, role, directive, taskID string, maxTurns int, mode contracts.SubAgentMode) (string, error) {
	return s.summary, s.err
}

func TestNewDispatcher(t *testing.T) {
	d := sessionorchestrator.NewDispatcher(
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
	leader := &stubAgent{id: "leader-1"}
	res := &staticLeaderResolver{leader: leader, ok: true}
	exec := &stubExecutor{result: execute.WorkerResult{WorkerID: "w-1", Summary: "done"}}
	d := sessionorchestrator.NewDispatcher(config.DelegateConfig{Enabled: true}, exec, nil, nil, res)

	result, err := d.Dispatch(context.Background(), sessionorchestrator.DispatchRequest{
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
	leader := &stubAgent{id: "leader-1"}
	res := &staticLeaderResolver{leader: leader, ok: true}
	exec := &stubExecutor{workerID: "w-async"}
	d := sessionorchestrator.NewDispatcher(config.DelegateConfig{Enabled: true}, exec, nil, nil, res)

	result, err := d.Dispatch(context.Background(), sessionorchestrator.DispatchRequest{
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
	d := sessionorchestrator.NewDispatcher(config.DelegateConfig{Enabled: false}, nil, sub, nil, res)

	result, err := d.Dispatch(context.Background(), sessionorchestrator.DispatchRequest{
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
	d := sessionorchestrator.NewDispatcher(config.DelegateConfig{Enabled: true}, nil, sub, nil, res)

	result, err := d.Dispatch(context.Background(), sessionorchestrator.DispatchRequest{
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
	d := sessionorchestrator.NewDispatcher(config.DelegateConfig{Enabled: false}, nil, nil, nil, res)

	_, err := d.Dispatch(context.Background(), sessionorchestrator.DispatchRequest{
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
	d := sessionorchestrator.NewDispatcher(config.DelegateConfig{}, nil, nil, hub, nil)
	if got := d.Hub(); got != hub {
		t.Fatalf("Hub() = %v, want %v", got, hub)
	}
}

// stubAgent is a minimal multiagent.Agent stub for dispatcher tests.
type stubAgent struct {
	id string
}

func (a *stubAgent) ID() string                                 { return a.id }
func (a *stubAgent) State() multiagent.AgentState               { return multiagent.AgentStateCreated }
func (a *stubAgent) Config() multiagent.AgentConfig             { return multiagent.AgentConfig{SessionID: "sess-1", WorkDir: "/tmp"} }
func (a *stubAgent) GetMessages() []types.Message               { return nil }
func (a *stubAgent) ResolvePermission(string, bool)             {}
func (a *stubAgent) Terminate(context.Context) error            { return nil }
func (a *stubAgent) SetAgentObserver(o multiagent.AgentObserver) {}
func (a *stubAgent) SetEngineEventSink(f func(*contracts.EngineEvent)) {}
func (a *stubAgent) Run(ctx context.Context) (*multiagent.AgentResult, error) {
	return &multiagent.AgentResult{ExitCode: 0}, nil
}
func (a *stubAgent) Wait(ctx context.Context) (*multiagent.AgentResult, error) {
	return nil, nil
}
func (a *stubAgent) Fork(ctx context.Context, cfg multiagent.AgentConfig) (multiagent.Agent, error) {
	return &stubAgent{id: "child"}, nil
}
func (a *stubAgent) Join(ctx context.Context, child multiagent.Agent) error { return nil }
