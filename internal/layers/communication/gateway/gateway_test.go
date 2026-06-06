package gateway

import (
	"context"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// mockEventHandler implements EventHandler for testing
type mockEventHandler struct {
	mu              sync.Mutex
	messages        []*types.OutboundMessage
	errors          []error
	statuses        map[string]types.SessionState
	permissionResult bool
}

func newMockEventHandler() *mockEventHandler {
	return &mockEventHandler{
		messages: make([]*types.OutboundMessage, 0),
		statuses: make(map[string]types.SessionState),
		permissionResult: true,
	}
}

func (h *mockEventHandler) OnMessage(msg *types.OutboundMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, msg)
}

func (h *mockEventHandler) OnPermissionRequest(req *types.PermissionRequest) bool {
	return h.permissionResult
}

func (h *mockEventHandler) OnError(err error, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.errors = append(h.errors, err)
}

func (h *mockEventHandler) OnStatus(sessionID string, state types.SessionState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.statuses[sessionID] = state
}

// mockContextEngine implements IContextEngine for testing
type mockContextEngine struct {
	events []*EngineEvent
}

func (m *mockContextEngine) Process(ctx context.Context, session *types.Session, message string) <-chan *EngineEvent {
	ch := make(chan *EngineEvent, len(m.events))
	for _, e := range m.events {
		ch <- e
	}
	close(ch)
	return ch
}

func TestCommunicationGateway_CreateSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, nil, cfg)

	session, err := gw.CreateSession("chat_123", "/tmp")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if session.SessionID == "" {
		t.Error("expected non-empty session ID")
	}

	if session.WorkDir != "/tmp" {
		t.Errorf("expected workDir '/tmp', got '%s'", session.WorkDir)
	}

	if session.State != types.SessionStateIdle {
		t.Errorf("expected state 'idle', got '%s'", session.State)
	}
}

func TestCommunicationGateway_GetSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, nil, cfg)

	created, _ := gw.CreateSession("chat_123", "/tmp")

	got, err := gw.GetSession(created.SessionID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if got.SessionID != created.SessionID {
		t.Errorf("expected session ID '%s', got '%s'", created.SessionID, got.SessionID)
	}
}

func TestCommunicationGateway_ExpireSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, nil, cfg)

	session, _ := gw.CreateSession("chat_123", "/tmp")

	if err := gw.ExpireSession(session.SessionID); err != nil {
		t.Fatalf("failed to expire session: %v", err)
	}

	// Verify session is expired in store
	expired, _ := store.Get(session.SessionID)
	if expired.State != types.SessionStateFailed {
		t.Errorf("expected state 'failed', got '%s'", expired.State)
	}
}

func TestCommunicationGateway_RouteInbound_EmptyMessage(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, nil, cfg)

	msg := &types.InboundMessage{
		Content: "",
	}

	err = gw.RouteInbound(context.Background(), msg)
	if err == nil {
		t.Error("expected error for empty message")
	}
}
