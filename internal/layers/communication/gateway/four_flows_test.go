package gateway

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// TestFourFlowEngine_EventFlow tests that the engine emits all event types
func TestFourFlowEngine_EventFlow(t *testing.T) {
	engine := NewFourFlowEngine()
	session := &types.Session{
		SessionID: "test_session_1",
		ChatID:   "feishu_oc_123_ou_456",
		State:    types.SessionStateIdle,
	}

	events := make([]*EngineEvent, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventChan := engine.Process(ctx, session, "test message")

	for {
		select {
		case <-ctx.Done():
			t.Fatal("timeout waiting for events")
		case event, ok := <-eventChan:
			if !ok {
				goto done
			}
			events = append(events, event)
			t.Logf("event: type=%s, content=%s", event.Type, event.Content)
		}
	}

done:
	// Verify all 4 flows are represented
	eventTypes := make(map[string]bool)
	for _, e := range events {
		eventTypes[e.Type] = true
	}

	// Event flow: thinking, text, tool_call, tool_result, complete
	if !eventTypes["thinking"] {
		t.Error("missing thinking event (event flow)")
	}
	if !eventTypes["text"] {
		t.Error("missing text event (event flow)")
	}
	if !eventTypes["tool_call"] {
		t.Error("missing tool_call event (event flow)")
	}
	if !eventTypes["tool_result"] {
		t.Error("missing tool_result event (event flow)")
	}

	// Task flow: milestone_progress
	if !eventTypes["milestone_progress"] {
		t.Error("missing milestone_progress event (task flow)")
	}

	// Info flow: info
	if !eventTypes["info"] {
		t.Error("missing info event (info flow)")
	}

	// Complete event
	if !eventTypes["complete"] {
		t.Error("missing complete event")
	}
}

// TestFourFlowGateway_AllEvents tests that gateway routes all 4 flows to adapter
func TestFourFlowGateway_AllEvents(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := &testEventHandler{
		messages: make([]*types.OutboundMessage, 0),
		statuses: make(map[string]types.SessionState),
	}
	cfg := config.DefaultConfig()

	// Use FourFlowEngine instead of stub
	engine := NewFourFlowEngine()

	// Create permission manager with YOLO mode for auto-approve
	permMgr := NewPermissionManager(nil)
	userCfg := config.DefaultUserConfig()
	userCfg.YOLO.Enabled = true
	userCfg.YOLO.AutoApproveTools = true
	permMgr.SetUserConfig(userCfg)

	gw := NewCommunicationGateway(store, handler, engine, permMgr, cfg)

	session, _ := gw.CreateSession("feishu_oc_123_ou_456", "/tmp")

	msg := &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "feishu_oc_123_ou_456",
		Content:   "test",
	}

	// Create a cancellable context to prevent race
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = gw.RouteInbound(ctx, msg)
	if err != nil {
		t.Fatalf("RouteInbound() error = %v", err)
	}

	// Wait for all events to be processed (with polling)
	maxWait := 5 * time.Second
	pollInterval := 100 * time.Millisecond
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		handler.mu.Lock()
		count := len(handler.messages)
		handler.mu.Unlock()
		if count >= 7 { // We expect: thinking, tool_call, tool_result, milestone_progress, text, info, complete
			break
		}
		time.Sleep(pollInterval)
	}

	// Verify messages received
	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.messages) == 0 {
		t.Fatal("no messages received")
	}

	// Categorize messages by event_type
	eventTypes := make(map[string][]string)
	for _, m := range handler.messages {
		et := "unknown"
		if m.Metadata != nil {
			et = m.Metadata["event_type"]
		}
		eventTypes[et] = append(eventTypes[et], m.Content)
	}

	t.Log("Received event types:")
	for et, contents := range eventTypes {
		t.Logf("  %s: %d messages", et, len(contents))
	}

	// Verify 事件流: thinking, tool_call, tool_result
	if len(eventTypes["thinking"]) == 0 {
		t.Error("missing thinking events (event flow)")
	}
	if len(eventTypes["tool_call"]) == 0 {
		t.Error("missing tool_call events (event flow)")
	}
	if len(eventTypes["tool_result"]) == 0 {
		t.Error("missing tool_result events (event flow)")
	}

	// Verify 任务流: milestone_progress
	if len(eventTypes["milestone_progress"]) == 0 {
		t.Error("missing milestone_progress events (task flow)")
	}

	// Verify 信息流: info
	if len(eventTypes["info"]) == 0 {
		t.Error("missing info events (info flow)")
	}

	// Verify complete
	if len(eventTypes["complete"]) == 0 {
		t.Error("missing complete events")
	}
}

// testEventHandler implements EventHandler for testing
type testEventHandler struct {
	mu       sync.Mutex
	messages []*types.OutboundMessage
	statuses map[string]types.SessionState
	errors   []error
}

func (h *testEventHandler) OnMessage(msg *types.OutboundMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, msg)
}

func (h *testEventHandler) OnPermissionRequest(req *types.PermissionRequest) bool {
	return true
}

func (h *testEventHandler) OnError(err error, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.errors = append(h.errors, err)
}

func (h *testEventHandler) OnStatus(sessionID string, state types.SessionState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.statuses[sessionID] = state
}
