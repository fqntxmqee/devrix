package communication

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// mockEventHandler implements gateway.EventHandler for integration tests
type mockEventHandler struct {
	mu              sync.Mutex
	messages        []*types.OutboundMessage
	errors          []error
	statuses        map[string]types.SessionState
	permissionCalls int
}

func newMockEventHandler() *mockEventHandler {
	return &mockEventHandler{
		messages: make([]*types.OutboundMessage, 0),
		statuses: make(map[string]types.SessionState),
	}
}

func (h *mockEventHandler) OnMessage(msg *types.OutboundMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, msg)
}

func (h *mockEventHandler) OnPermissionRequest(req *types.PermissionRequest) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.permissionCalls++
	return true
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

// mockContextEngine implements gateway.IContextEngine for integration tests
type mockContextEngine struct {
	events []*gateway.EngineEvent
}

func (m *mockContextEngine) Process(ctx context.Context, session *types.Session, message string) <-chan *gateway.EngineEvent {
	ch := make(chan *gateway.EngineEvent, len(m.events))
	for _, e := range m.events {
		e.SessionID = session.SessionID
		ch <- e
	}
	close(ch)
	return ch
}

func TestIntegration_CLIToGatewayToSession(t *testing.T) {
	dir := t.TempDir()
	store, err := gateway.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	eventHandler := newMockEventHandler()

	gw := gateway.NewCommunicationGateway(
		store,
		eventHandler,
		nil, // no context engine
		nil, // no permission manager
		cfg,
	)

	// Test 1: Create session
	session, err := gw.CreateSession("cli", "/tmp")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if session.SessionID == "" {
		t.Error("expected non-empty session ID")
	}

	// Test 2: Get session
	got, err := gw.GetSession(session.SessionID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if got.SessionID != session.SessionID {
		t.Errorf("expected session ID '%s', got '%s'", session.SessionID, got.SessionID)
	}

	// Test 3: Session persists in store
	got, err = store.Get(session.SessionID)
	if err != nil {
		t.Fatalf("failed to get session from store: %v", err)
	}

	if got == nil {
		t.Fatal("expected session in store")
	}

	// Test 4: List sessions
	sessions, err := store.List()
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}
}

func TestIntegration_SessionExpiration(t *testing.T) {
	dir := t.TempDir()
	store, err := gateway.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Session.IdleTimeout = 100 * time.Millisecond

	gw := gateway.NewCommunicationGateway(
		store,
		nil,
		nil,
		nil,
		cfg,
	)

	session, _ := gw.CreateSession("cli", "/tmp")

	// Make session idle
	session.LastMessageAt = time.Now().Add(-1 * time.Hour)
	store.Update(session)

	// Try to use expired session
	msg := &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "cli",
		Content:   "hello",
	}

	err = gw.RouteInbound(context.Background(), msg)
	if err == nil {
		t.Error("expected error for expired session")
	}
}

func TestIntegration_PermissionRequest(t *testing.T) {
	dir := t.TempDir()
	store, err := gateway.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	permMgr := gateway.NewPermissionManager(&cfg.Permission)

	gw := gateway.NewCommunicationGateway(
		store,
		nil,
		nil,
		permMgr,
		cfg,
	)

	session, _ := gw.CreateSession("cli", "/tmp")

	// Test permission request
	approved := permMgr.Request(session.SessionID, "bash", "ls -la", types.RiskLevelMedium)

	// Without mock input, this will timeout
	if approved {
		t.Error("expected permission to be denied (timeout)")
	}
}

func TestIntegration_CommandParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected types.CommandType
	}{
		{"new command", "/new", types.CommandNew},
		{"stop command", "/stop", types.CommandStop},
		{"help command", "/help", types.CommandHelp},
		{"unknown command", "/unknown", types.CommandUnknown},
		{"regular message", "hello", types.CommandUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := types.ParseCommand(tt.input, "/")
			if cmd.Type != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, cmd.Type)
			}
		})
	}
}
