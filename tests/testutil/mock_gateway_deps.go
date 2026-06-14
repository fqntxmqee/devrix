package testutil

import (
	"context"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/shared/types"
)

// MockEventHandler implements capture.EventHandler for integration and acceptance tests.
type MockEventHandler struct {
	mu              sync.Mutex
	Messages        []*types.OutboundMessage
	Errors          []error
	Statuses        map[string]types.SessionState
	PermissionCalls int
}

func NewMockEventHandler() *MockEventHandler {
	return &MockEventHandler{
		Messages: make([]*types.OutboundMessage, 0),
		Statuses: make(map[string]types.SessionState),
	}
}

func (h *MockEventHandler) OnMessage(msg *types.OutboundMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Messages = append(h.Messages, msg)
}

func (h *MockEventHandler) OnPermissionRequest(req *types.PermissionRequest) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.PermissionCalls++
	return true
}

func (h *MockEventHandler) OnError(err error, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Errors = append(h.Errors, err)
}

func (h *MockEventHandler) OnStatus(sessionID string, state types.SessionState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Statuses[sessionID] = state
}

// PermissionRequestCount returns how many permission prompts were handled.
func (h *MockEventHandler) PermissionRequestCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.PermissionCalls
}

// MessageCount returns the number of outbound messages received so far.
func (h *MockEventHandler) MessageCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.Messages)
}

// OutboundMessages returns a snapshot of received messages.
func (h *MockEventHandler) OutboundMessages() []*types.OutboundMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*types.OutboundMessage, len(h.Messages))
	copy(out, h.Messages)
	return out
}

// WaitForMessages blocks until at least n messages arrive or timeout elapses.
func (h *MockEventHandler) WaitForMessages(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if h.MessageCount() >= n {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return h.MessageCount() >= n
}

// MockContextEngine implements capture.IContextEngine for integration and acceptance tests.
type MockContextEngine struct {
	Events []*capture.EngineEvent
}

func (m *MockContextEngine) Process(ctx context.Context, session *types.Session, message string) <-chan *capture.EngineEvent {
	ch := make(chan *capture.EngineEvent, len(m.Events))
	for _, e := range m.Events {
		event := *e
		event.SessionID = session.SessionID
		ch <- &event
	}
	close(ch)
	return ch
}
