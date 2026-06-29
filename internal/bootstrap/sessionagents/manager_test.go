package sessionagents_test

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/bootstrap/sessionagents"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubPermissionRouter struct {
	approved bool
	calls    int
}

func (s *stubPermissionRouter) RoutePermission(_ *types.PermissionRequest) (bool, error) {
	s.calls++
	return s.approved, nil
}

func (s *stubPermissionRouter) PermissionDefaultTimeout() time.Duration {
	return time.Minute
}

type mockAgent struct {
	id         string
	resolveCh  chan resolveCall
	engineSink func(*contracts.EngineEvent)
}

type resolveCall struct {
	tool    string
	granted bool
}

func (m *mockAgent) ID() string   { return m.id }
func (m *mockAgent) State() multiagent.AgentState {
	return multiagent.AgentStateCreated
}
func (m *mockAgent) Config() multiagent.AgentConfig { return multiagent.AgentConfig{} }
func (m *mockAgent) Run(context.Context) (*multiagent.AgentResult, error) {
	return nil, nil
}
func (m *mockAgent) Fork(context.Context, multiagent.AgentConfig) (multiagent.Agent, error) {
	return nil, nil
}
func (m *mockAgent) Join(context.Context, multiagent.Agent) error { return nil }
func (m *mockAgent) Terminate(context.Context) error              { return nil }
func (m *mockAgent) Wait(context.Context) (*multiagent.AgentResult, error) {
	return nil, nil
}
func (m *mockAgent) ResolvePermission(tool string, granted bool) {
	if m.resolveCh != nil {
		m.resolveCh <- resolveCall{tool: tool, granted: granted}
	}
}
func (m *mockAgent) GetMessages() []types.Message { return nil }
func (m *mockAgent) SetAgentObserver(obs multiagent.AgentObserver) {
	if obs == nil {
		return
	}
	obs.EmitAgentEvent(multiagent.AgentEvent{
		EventType: "permission_required",
		Metadata:  map[string]any{"tool": "bash"},
	})
}
func (m *mockAgent) SetEngineEventSink(sink func(*contracts.EngineEvent)) {
	m.engineSink = sink
}

type mockFactory struct {
	agent *mockAgent
}

func (f *mockFactory) Create(_ context.Context, cfg multiagent.AgentConfig, _ *types.Session) (multiagent.Agent, error) {
	if f.agent != nil {
		f.agent.id = cfg.SessionID + "-leader"
		return f.agent, nil
	}
	return &mockAgent{id: cfg.SessionID + "-leader"}, nil
}

// T: D1-RF-T03 — permission_required observer chain resolves via manager.
func TestManager_PermissionObserverChain(t *testing.T) {
	resolveCh := make(chan resolveCall, 1)
	ag := &mockAgent{resolveCh: resolveCh}
	factory := &mockFactory{agent: ag}
	router := &stubPermissionRouter{approved: true}

	mgr := sessionagents.NewManager(factory)
	mgr.SetPermissionRouter(router)

	session := types.NewSession("sess-perm", "cli", t.TempDir())
	if err := mgr.EnsureSessionLeader(context.Background(), session); err != nil {
		t.Fatalf("EnsureSessionLeader: %v", err)
	}

	select {
	case call := <-resolveCh:
		if call.tool != "bash" || !call.granted {
			t.Fatalf("resolve=%+v", call)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ResolvePermission")
	}
	if router.calls != 1 {
		t.Fatalf("RoutePermission calls=%d want 1", router.calls)
	}
}

// T: D1-RF-T04 — orphan engine events delivered when no active D7 turn.
func TestManager_OrphanEngineEventSink(t *testing.T) {
	ag := &mockAgent{}
	factory := &mockFactory{agent: ag}
	got := make(chan *contracts.EngineEvent, 1)

	mgr := sessionagents.NewManager(factory)
	mgr.SetActiveProcessChecker(activeChecker{active: false})
	mgr.SetOrphanEngineEventSink(func(_ context.Context, _ *types.Session, ev *contracts.EngineEvent) {
		got <- ev
	})

	session := types.NewSession("sess-orphan", "cli", t.TempDir())
	if err := mgr.EnsureSessionLeader(context.Background(), session); err != nil {
		t.Fatalf("EnsureSessionLeader: %v", err)
	}
	if ag.engineSink == nil {
		t.Fatal("expected engine sink wired")
	}
	ag.engineSink(&contracts.EngineEvent{Type: "text", Content: "orphan"})

	select {
	case ev := <-got:
		if ev.Content != "orphan" {
			t.Fatalf("content=%q", ev.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for orphan sink")
	}
}

type activeChecker struct {
	active bool
}

func (a activeChecker) HasActiveProcess(string) bool { return a.active }
