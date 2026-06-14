package capture

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/delivery/eventbus"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// mockEventHandler implements EventHandler for testing
type mockEventHandler struct {
	mu               sync.Mutex
	messages         []*types.OutboundMessage
	errors           []error
	statuses         map[string]types.SessionState
	permissionResult bool
}

func newMockEventHandler() *mockEventHandler {
	return &mockEventHandler{
		messages:         make([]*types.OutboundMessage, 0),
		statuses:         make(map[string]types.SessionState),
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
		ev := *e
		ev.SessionID = session.SessionID
		ch <- &ev
	}
	close(ch)
	return ch
}

type engineOrchestrationEntry struct {
	engine *mockContextEngine
}

func (e *engineOrchestrationEntry) ProcessMessage(ctx context.Context, sessionID, message string) (<-chan *EngineEvent, error) {
	session := types.NewSession(sessionID, "test", "")
	return e.engine.Process(ctx, session, message), nil
}

func (e *engineOrchestrationEntry) Cancel(context.Context, string) error { return nil }

func wireMockEngine(gw *CommunicationGateway, engine *mockContextEngine) {
	gw.SetOrchestrationEntry(&engineOrchestrationEntry{engine: engine})
}

func TestCommunicationGateway_CreateSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, cfg)

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
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, cfg)

	created, _ := gw.CreateSession("chat_123", "/tmp")

	got, err := gw.GetSession(created.SessionID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if got.SessionID != created.SessionID {
		t.Errorf("expected session ID '%s', got '%s'", created.SessionID, got.SessionID)
	}
}

func TestCommunicationGateway_ResolveSessionByChatID(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, cfg)

	chatKey := "feishu_oc_123_ou_456"
	older, err := gw.CreateSession(chatKey, "/tmp")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	older.LastMessageAt = time.Now().Add(-10 * time.Minute)
	if err := store.Update(older); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	newer, err := gw.CreateSession(chatKey, "/tmp")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	newer.LastMessageAt = time.Now().Add(-1 * time.Minute)
	if err := store.Update(newer); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := gw.ResolveSessionByChatID(chatKey)
	if err != nil {
		t.Fatalf("ResolveSessionByChatID() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected restored session, got nil")
	}
	if got.SessionID != newer.SessionID {
		t.Errorf("sessionID = %s, want %s", got.SessionID, newer.SessionID)
	}
}

func TestCommunicationGateway_ResolveSessionByChatID_should_restore_idle_with_snapshot(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Session.IdleTimeout = 5 * time.Minute
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, cfg)

	chatKey := "feishu_oc_idle_ou_456"
	stale, err := gw.CreateSession(chatKey, "/tmp")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	stale.LastMessageAt = time.Now().Add(-30 * time.Minute)
	stale.ContextSnapshot = []byte("ctx-v1-snapshot")
	if err := store.Update(stale); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := gw.ResolveSessionByChatID(chatKey)
	if err != nil {
		t.Fatalf("ResolveSessionByChatID() error = %v", err)
	}
	if got == nil || got.SessionID != stale.SessionID {
		t.Fatalf("expected idle session with snapshot restored, got %v", got)
	}
}

func TestCommunicationGateway_ResolveSessionByChatID_should_prefer_recent_over_stale_snapshot(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, cfg)

	chatKey := "feishu_oc_ctx_ou_456"
	rich, err := gw.CreateSession(chatKey, "/tmp")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	rich.LastMessageAt = time.Now().Add(-2 * time.Hour)
	rich.ContextSnapshot = []byte("large-context-snapshot")

	empty, err := gw.CreateSession(chatKey, "/tmp")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	empty.LastMessageAt = time.Now().Add(-1 * time.Minute)

	if err := store.Update(rich); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := store.Update(empty); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := gw.ResolveSessionByChatID(chatKey)
	if err != nil {
		t.Fatalf("ResolveSessionByChatID() error = %v", err)
	}
	if got == nil || got.SessionID != empty.SessionID {
		t.Fatalf("sessionID = %v, want %s (recent session)", got, empty.SessionID)
	}
}

func TestCommunicationGateway_ResolveSessionByChatID_should_prefer_snapshot_on_same_timestamp(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, cfg)

	chatKey := "feishu_oc_ctx_ou_456"
	ts := time.Now().Add(-1 * time.Minute)

	rich, err := gw.CreateSession(chatKey, "/tmp")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	rich.LastMessageAt = ts
	rich.ContextSnapshot = []byte("large-context-snapshot")

	empty, err := gw.CreateSession(chatKey, "/tmp")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	empty.LastMessageAt = ts

	if err := store.Update(rich); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := store.Update(empty); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := gw.ResolveSessionByChatID(chatKey)
	if err != nil {
		t.Fatalf("ResolveSessionByChatID() error = %v", err)
	}
	if got == nil || got.SessionID != rich.SessionID {
		t.Fatalf("sessionID = %v, want %s (snapshot tiebreaker)", got, rich.SessionID)
	}
}

func TestSessionRestoreScore_should_prefer_recency_over_large_snapshot(t *testing.T) {
	staleLarge := &types.Session{
		LastMessageAt:   time.Now().Add(-2 * time.Hour),
		ContextSnapshot: make([]byte, 531_678),
	}
	recent := &types.Session{
		LastMessageAt: time.Now().Add(-1 * time.Minute),
	}

	if sessionRestoreScore(staleLarge) >= sessionRestoreScore(recent) {
		t.Fatalf("stale large score %d should lose to recent score %d",
			sessionRestoreScore(staleLarge), sessionRestoreScore(recent))
	}
}

func TestCommunicationGateway_ExpireSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, cfg)

	session, _ := gw.CreateSession("chat_123", "/tmp")

	if err := gw.ExpireSession(session.SessionID); err != nil {
		t.Fatalf("failed to expire session: %v", err)
	}

	// ExpireSession removes the session from persistent store
	expired, _ := store.Get(session.SessionID)
	if expired != nil {
		t.Errorf("expected session to be removed from store, got session %s", expired.SessionID)
	}
}

func TestCommunicationGateway_RouteInbound_EmptyMessage(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, cfg)

	msg := &types.InboundMessage{
		Content: "",
	}

	err = gw.RouteInbound(context.Background(), msg)
	if err == nil {
		t.Error("expected error for empty message")
	}
}

// mockContextEngineWithEvents creates a mock context engine that returns specified events
func mockContextEngineWithEvents(events ...*EngineEvent) *mockContextEngine {
	return &mockContextEngine{events: events}
}

func TestCommunicationGateway_RouteInbound_ResumesIdleSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := newMockEventHandler()
	cfg := config.DefaultConfig()
	cfg.Session.IdleTimeout = 5 * time.Minute

	mockEngine := &mockContextEngine{
		events: []*EngineEvent{{Type: "complete"}},
	}
	gw := NewCommunicationGateway(store, handler, nil, cfg)
	wireMockEngine(gw, mockEngine)

	session, _ := gw.CreateSession("feishu_chat", "/tmp")
	session.LastMessageAt = time.Now().Add(-2 * time.Hour)
	if err := store.Update(session); err != nil {
		t.Fatalf("update session: %v", err)
	}

	msg := &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "feishu_chat",
		Content:   "wake up",
		MessageID: "msg-idle-resume",
	}
	if err := gw.RouteInbound(context.Background(), msg); err != nil {
		t.Fatalf("RouteInbound() error = %v", err)
	}
	gw.WaitForProcesses()

	got, err := store.Get(session.SessionID)
	if err != nil || got == nil {
		t.Fatalf("session missing after resume: %v", err)
	}
	if got.IsIdle(cfg.Session.IdleTimeout) {
		t.Fatal("session should not be idle after inbound message")
	}
}

func TestCommunicationGateway_RouteInbound_NormalMessage(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := newMockEventHandler()
	cfg := config.DefaultConfig()

	mockEngine := &mockContextEngine{
		events: []*EngineEvent{
			{Type: "text", Content: "Hello!"},
			{Type: "complete", Content: ""},
		},
	}

	gw := NewCommunicationGateway(store, handler, nil, cfg)
	wireMockEngine(gw, mockEngine)

	session, _ := gw.CreateSession("feishu_oc_123_ou_456", "/tmp")

	msg := &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "feishu_oc_123_ou_456",
		Content:   "hello",
	}

	err = gw.RouteInbound(context.Background(), msg)
	if err != nil {
		t.Fatalf("RouteInbound() error = %v", err)
	}

	// Wait for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Check that message was received
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.messages) == 0 {
		t.Error("expected at least one message, got none")
	}
}

// Note: Tool call tests require complex permission manager setup with goroutines
// to resolve pending requests. For now, we test with text events only.
// Full tool_call flow should be tested in integration tests.

func TestCommunicationGateway_RouteOutbound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := newMockEventHandler()
	cfg := config.DefaultConfig()

	gw := NewCommunicationGateway(store, handler, nil, cfg)

	msg := &types.OutboundMessage{
		SessionID: "sess_123",
		Content:   "Hello from gateway!",
	}

	err = gw.RouteOutbound(msg)
	if err != nil {
		t.Fatalf("RouteOutbound() error = %v", err)
	}

	// Check that message was received
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(handler.messages))
	}
	if handler.messages[0].Content != "Hello from gateway!" {
		t.Errorf("expected content 'Hello from gateway!', got '%s'", handler.messages[0].Content)
	}
}

func TestCommunicationGateway_RouteError(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := newMockEventHandler()
	cfg := config.DefaultConfig()

	gw := NewCommunicationGateway(store, handler, nil, cfg)

	testErr := context.DeadlineExceeded
	gw.RouteError(testErr, "sess_123")

	// Check that error was received
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(handler.errors))
	}
}

func TestCommunicationGateway_CleanupExpiredSessions_SkipsActiveProcess(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Session.IdleTimeout = time.Millisecond
	gw := NewCommunicationGateway(store, newMockEventHandler(), nil, cfg)

	session, err := gw.CreateSession("chat_active", "/tmp")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	session.LastMessageAt = time.Now().Add(-time.Hour)
	gw.mu.Lock()
	gw.sessions[session.SessionID] = session
	gw.activeProcesses[session.SessionID] = func() {}
	gw.mu.Unlock()

	time.Sleep(5 * time.Millisecond)
	gw.cleanupExpiredSessions()

	gw.mu.RLock()
	_, ok := gw.sessions[session.SessionID]
	gw.mu.RUnlock()
	if !ok {
		t.Fatal("active session should not be cleaned from in-memory cache")
	}
}

func TestCommunicationGateway_StartCleanupRoutine(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := newMockEventHandler()
	cfg := config.DefaultConfig()
	cfg.Session.IdleTimeout = 100 * time.Millisecond

	gw := NewCommunicationGateway(store, handler, nil, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a session
	_, _ = gw.CreateSession("chat_123", "/tmp")

	// Wait for session to become idle
	time.Sleep(200 * time.Millisecond)

	// Start cleanup routine
	gw.StartCleanupRoutine(ctx, 50*time.Millisecond)

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Session should be cleaned up from in-memory cache
	// Note: File store still has it, but in-memory map should be cleared
}

func TestCommunicationGateway_GetOrCreateSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := newMockEventHandler()
	cfg := config.DefaultConfig()

	gw := NewCommunicationGateway(store, handler, nil, cfg)

	// Create a session first
	existingSession, _ := gw.CreateSession("feishu_oc_123_ou_456", "/tmp")

	msg := &types.InboundMessage{
		SessionID: existingSession.SessionID,
		ChatID:    "feishu_oc_123_ou_456",
		Content:   "hello",
	}

	// getOrCreateSession with existing SessionID should return that session
	session, err := gw.getOrCreateSession(context.Background(), msg)
	if err != nil {
		t.Fatalf("getOrCreateSession() error = %v", err)
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.SessionID != existingSession.SessionID {
		t.Errorf("expected session ID %s, got %s", existingSession.SessionID, session.SessionID)
	}
}

func TestOutboundMetadata_should_merge_engine_metadata(t *testing.T) {
	meta := outboundMetadata("text", &EngineEvent{
		Metadata: map[string]string{
			"source":      "agent_tool",
			"agent":       "Claude Code",
			"is_complete": "false",
		},
	})
	if meta["event_type"] != "text" {
		t.Fatalf("event_type = %q", meta["event_type"])
	}
	if meta["source"] != "agent_tool" || meta["agent"] != "Claude Code" {
		t.Fatalf("metadata = %#v", meta)
	}
}

// DM-20260611-003 (devrix-event-channel): integration smoke — when a
// BackpressureEventBus is wired into the gateway, engine events are
// routed through the bus and still reach the EventHandler with the
// original wire-level OutboundMessage shape. This is the wire-compat
// invariant the S4 change must preserve.
func TestCommunicationGateway_WithEventBus_RoutesThroughBus(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	handler := newMockEventHandler()
	mockEngine := &mockContextEngine{
		events: []*EngineEvent{
			{Type: "text", Content: "hi"},
			{Type: "complete", Content: ""},
		},
	}
	gw := NewCommunicationGateway(store, handler, nil, cfg)
	wireMockEngine(gw, mockEngine)

	// Wire a real BackpressureEventBus.
	busCfg := config.DefaultEventBusConfig()
	bus, err := eventbus.NewBus(busCfg)
	if err != nil {
		t.Fatalf("NewBus: %v", err)
	}
	defer bus.Close()
	gw.SetEventBus(bus)

	if !gw.EventBusEnabled() {
		t.Fatalf("EventBusEnabled() = false after SetEventBus")
	}

	session, _ := gw.CreateSession("chat_bus", "/tmp")
	msg := &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "chat_bus",
		Content:   "hello",
	}
	if err := gw.RouteInbound(context.Background(), msg); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}
	gw.WaitForProcesses()

	// Drain residual events from the bus.
	ctx, cc := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cc()
	if _, err := bus.Drain(ctx, session.SessionID, 200*time.Millisecond); err != nil &&
		err != eventbus.ErrDrainTimeout {
		t.Logf("post-test drain: %v", err)
	}

	// The handler must have received both text and complete messages
	// through the bus round-trip.
	handler.mu.Lock()
	defer handler.mu.Unlock()
	slog.Info("test: handler.messages after wait", "count", len(handler.messages), "sessionID", session.SessionID)
	for i, m := range handler.messages {
		if m == nil {
			continue
		}
		slog.Info("test: msg", "i", i, "type", m.Metadata["event_type"], "content", m.Content)
	}
	var sawText, sawComplete bool
	for _, m := range handler.messages {
		if m == nil {
			continue
		}
		switch m.Metadata["event_type"] {
		case "text":
			sawText = true
		case "complete":
			sawComplete = true
		}
	}
	if !sawText {
		t.Error("text event did not reach handler through bus")
	}
	if !sawComplete {
		t.Error("complete event did not reach handler through bus")
	}
}
