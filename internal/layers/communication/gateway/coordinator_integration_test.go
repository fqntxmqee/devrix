package gateway

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/coordinator"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
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

// T: D7-D1-T01 — orchestrationEnabled=true routes to orchestrationEntry.ProcessMessage.
func TestGateway_D7Enabled_RoutesToEntry(t *testing.T) {
	gw := newTestGateway(t)
	entry := &fakeEntry{}
	gw.SetOrchestrationEntry(entry, true)

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

// T: D7-D1-T01 (negative) — orchestrationEnabled=false keeps legacy D1→D2 path.
func TestGateway_D7Disabled_LegacyPath(t *testing.T) {
	gw := newTestGateway(t)
	entry := &fakeEntry{}
	gw.SetOrchestrationEntry(entry, false) // enabled=false

	msg := &types.InboundMessage{
		SessionID: "sess-legacy",
		ChatID:    "chat-legacy",
		MessageID: "m1",
		Content:   "hello",
		UserID:    "user1",
	}
	// legacy path requires contextEngine. We don't wire one; the gateway
	// should NOT call orchestrationEntry.
	if err := gw.RouteInbound(context.Background(), msg); err == nil {
		t.Fatalf("expected error when context engine is nil and d7 disabled")
	}
	// Wait a moment to make sure no goroutine calls orchestrationEntry.
	time.Sleep(100 * time.Millisecond)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.processes != 0 {
		t.Fatalf("orchestrationEntry.ProcessMessage calls = %d, want 0 (legacy path)", entry.processes)
	}
}

// T: D1 StopProcess with D7 enabled also calls orchestrationEntry.Cancel.
func TestGateway_StopProcess_D7Cancel(t *testing.T) {
	gw := newTestGateway(t)
	entry := &fakeEntry{}
	gw.SetOrchestrationEntry(entry, true)

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
var _ contracts.IOrchestrationEntry = (*coordinator.Entry)(nil)

// newTestGateway is a minimal helper that builds a gateway without a
// real context engine. The d7-enabled tests don't need one; the
// d7-disabled test expects an error from the legacy nil-engine path.
func newTestGateway(t *testing.T) *CommunicationGateway {
	t.Helper()
	store := &fakeSessionStore{}
	handler := &fakeEventHandler{}
	cfg := &config.CommunicationConfig{}
	gw := NewCommunicationGateway(store, handler, nil, nil, cfg)
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
