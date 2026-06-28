package capture

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/bootstrap/sessionagents"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/provision"
	"github.com/devrix/devrix/internal/layers/multiagent/run"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/types"
)

// fakeEntry is a minimal IOrchestrationEntry for testing the D7 routing
// branch. It records ProcessMessage and Cancel calls and returns a
// pre-canned event channel.
type fakeEntry struct {
	mu          sync.Mutex
	processes   int
	cancels     int
	lastSession string
	lastMessage string
}

func (f *fakeEntry) ProcessMessage(_ context.Context, sessionID, message string) (<-chan *contracts.EngineEvent, error) {
	f.mu.Lock()
	f.processes++
	f.lastSession = sessionID
	f.lastMessage = message
	f.mu.Unlock()
	ch := make(chan *contracts.EngineEvent, 2)
	ch <- &contracts.EngineEvent{Type: "text", Content: "d7:" + message, SessionID: sessionID}
	ch <- &contracts.EngineEvent{Type: "complete", SessionID: sessionID}
	close(ch)
	return ch, nil
}

func (f *fakeEntry) Cancel(_ context.Context, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancels++
	f.lastSession = sessionID
	return nil
}

// T: D7-D1-T01 — routes to orchestrationEntry.ProcessMessage.
func TestGateway_D7Enabled_RoutesToEntry(t *testing.T) {
	gw := newTestGateway(t)
	entry := &fakeEntry{}
	gw.SetOrchestrationEntry(entry)

	msg := &types.InboundMessage{
		SessionID: "sess-d7",
		ChatID:    "chat-d7",
		MessageID: "m1",
		Content:   "hello",
		UserID:    "user1",
	}
	if err := gw.RouteInbound(context.Background(), msg); err != nil {
		t.Fatalf("RouteInbound err: %v", err)
	}
	// Wait for the goroutine to call ProcessMessage.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entry.mu.Lock()
		c := entry.processes
		entry.mu.Unlock()
		if c == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.processes != 1 {
		t.Fatalf("orchestrationEntry.ProcessMessage calls = %d, want 1", entry.processes)
	}
	if entry.lastSession != "sess-d7" {
		t.Fatalf("last session = %q, want sess-d7", entry.lastSession)
	}
	if entry.lastMessage != "hello" {
		t.Fatalf("last message = %q, want hello", entry.lastMessage)
	}
}

// T: D7-D1-T02 — multi_agent factory must not hijack ingress when D7 entry is wired.
func TestGateway_AgentFactoryWithD7_PrefersD7Path(t *testing.T) {
	gw := newTestGateway(t)
	entry := &fakeEntry{}
	gw.SetOrchestrationEntry(entry)
	factory := provision.NewAgentFactory(multiagent.AgentDeps{
		Engine: &run.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}},
	}, config.DefaultMultiAgentConfig())
	mgr := sessionagents.NewManager(factory)
	gw.SetBeforeDispatch(mgr.EnsureSessionLeader)

	session, err := gw.CreateSession("cli", t.TempDir())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	msg := &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "chat-d7-ma",
		MessageID: "m1",
		Content:   "hello",
		UserID:    "user1",
	}
	if err := gw.RouteInbound(context.Background(), msg); err != nil {
		t.Fatalf("RouteInbound err: %v", err)
	}
	gw.WaitForProcesses()

	entry.mu.Lock()
	processes := entry.processes
	entry.mu.Unlock()
	if processes != 1 {
		t.Fatalf("orchestrationEntry.ProcessMessage calls = %d, want 1", processes)
	}

	ag := mgr.SessionAgent(session.SessionID)
	if ag == nil {
		t.Fatal("expected session leader to be provisioned")
	}
	if ag.State() != multiagent.AgentStateCreated {
		t.Fatalf("leader state = %v, want Created (must not Run on ingress)", ag.State())
	}
}

// T: D7-D1-T01 — missing orchestration entry fails fast (no agent-factory bypass).
func TestGateway_MissingOrchestrationEntry(t *testing.T) {
	gw := newTestGateway(t)

	msg := &types.InboundMessage{
		SessionID: "sess-no-d7",
		ChatID:    "chat-no-d7",
		MessageID: "m1",
		Content:   "hello",
		UserID:    "user1",
	}
	if err := gw.RouteInbound(context.Background(), msg); err == nil {
		t.Fatal("expected error when orchestration entry is nil")
	}
}

// T: D7-D1-T03 — agent factory hook without D7 entry still fails (no bypass).
func TestGateway_AgentFactoryWithoutD7_Fails(t *testing.T) {
	gw := newTestGateway(t)
	factory := provision.NewAgentFactory(multiagent.AgentDeps{
		Engine: &run.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}},
	}, config.DefaultMultiAgentConfig())
	mgr := sessionagents.NewManager(factory)
	gw.SetBeforeDispatch(mgr.EnsureSessionLeader)

	session, err := gw.CreateSession("cli", t.TempDir())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	msg := &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "chat-no-d7",
		MessageID: "m1",
		Content:   "hello",
		UserID:    "user1",
	}
	if err := gw.RouteInbound(context.Background(), msg); err == nil {
		t.Fatal("expected error when orchestration entry is nil even with agent factory")
	}
}

// T: D1 StopProcess with D7 enabled also calls orchestrationEntry.Cancel.
func TestGateway_StopProcess_D7Cancel(t *testing.T) {
	gw := newTestGateway(t)
	entry := &fakeEntry{}
	gw.SetOrchestrationEntry(entry)

	if err := gw.StopProcess("sess-stop"); err != nil {
		t.Fatalf("StopProcess err: %v", err)
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.cancels != 1 {
		t.Fatalf("orchestrationEntry.Cancel calls = %d, want 1", entry.cancels)
	}
	if entry.lastSession != "sess-stop" {
		t.Fatalf("last session = %q, want sess-stop", entry.lastSession)
	}
}

// Compile-time check: coordinator.Entry satisfies contracts.IOrchestrationEntry.
var _ contracts.IOrchestrationEntry = (*sessionorchestrator.Entry)(nil)

// newTestGateway is a minimal helper that builds a gateway without a
// real context engine. The d7-enabled tests don't need one; the
// d7-disabled test expects an error from the legacy nil-engine path.
func newTestGateway(t *testing.T) *CommunicationGateway {
	t.Helper()
	store := &fakeSessionStore{}
	handler := &fakeEventHandler{}
	cfg := &config.CommunicationConfig{}
	gw := NewCommunicationGateway(store, handler, nil, cfg, nil)
	return gw
}

// fakeSessionStore is a minimal in-memory store implementing SessionStore.
type fakeSessionStore struct {
	mu       sync.Mutex
	sessions map[string]*types.Session
}

func (f *fakeSessionStore) Create(s *types.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessions == nil {
		f.sessions = make(map[string]*types.Session)
	}
	f.sessions[s.SessionID] = s
	return nil
}
func (f *fakeSessionStore) Get(sessionID string) (*types.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessions == nil {
		f.sessions = make(map[string]*types.Session)
	}
	if s, ok := f.sessions[sessionID]; ok {
		return s, nil
	}
	s := &types.Session{SessionID: sessionID}
	f.sessions[sessionID] = s
	return s, nil
}
func (f *fakeSessionStore) Update(s *types.Session) error { return nil }
func (f *fakeSessionStore) Delete(_ string) error         { return nil }
func (f *fakeSessionStore) List() ([]*types.Session, error) {
	return nil, nil
}
func (f *fakeSessionStore) GetIdleSessions(_ time.Duration) ([]*types.Session, error) {
	return nil, nil
}

// fakeEventHandler implements EventHandler with no-op methods.
type fakeEventHandler struct{}

func (f *fakeEventHandler) OnMessage(_ *types.OutboundMessage) {}
func (f *fakeEventHandler) OnPermissionRequest(_ *types.PermissionRequest) bool {
	return true
}
func (f *fakeEventHandler) OnError(_ error, _ string)               {}
func (f *fakeEventHandler) OnStatus(_ string, _ types.SessionState) {}
