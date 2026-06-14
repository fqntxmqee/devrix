package capture

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D7-MIG-T01 — D7-only ingress matrix (plan_mode hints are orchestrator concerns).
// Legacy d7_enabled=false combinations removed in DM-20260614-007.

type matrixEntry struct {
	calls            int32
	lastSystemPrompt string
	planModeHint     string
}

func (m *matrixEntry) ProcessMessage(_ context.Context, sessionID, message string) (<-chan *contracts.EngineEvent, error) {
	atomic.AddInt32(&m.calls, 1)
	_ = message
	_ = m.planModeHint
	ch := make(chan *contracts.EngineEvent, 2)
	ch <- &contracts.EngineEvent{Type: "text", Content: "ok", SessionID: sessionID}
	ch <- &contracts.EngineEvent{Type: "complete", SessionID: sessionID}
	close(ch)
	return ch, nil
}

func (m *matrixEntry) Cancel(_ context.Context, _ string) error {
	return nil
}

// T: D1-S13-A03-T01
func TestMatrix_D7True_PlanModeFalse_D7Path(t *testing.T) {
	gw := newTestGateway(t)
	entry := &matrixEntry{}
	gw.SetOrchestrationEntry(entry)

	msg := &types.InboundMessage{
		SessionID: "sess-m3", ChatID: "c3", MessageID: "m3",
		Content: "hello", UserID: "u1",
	}
	if err := gw.RouteInbound(context.Background(), msg); err != nil {
		t.Fatalf("RouteInbound err: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&entry.calls) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&entry.calls); got != 1 {
		t.Fatalf("orchestrationEntry must be called once; got %d", got)
	}
}

// T: D1-S13-A03-T01
func TestMatrix_D7True_PlanModeTrue_D7Path(t *testing.T) {
	gw := newTestGateway(t)
	entry := &matrixEntry{planModeHint: "[plan_mode]"}
	gw.SetOrchestrationEntry(entry)

	msg := &types.InboundMessage{
		SessionID: "sess-m4", ChatID: "c4", MessageID: "m4",
		Content: "/plan add auth", UserID: "u1",
	}
	if err := gw.RouteInbound(context.Background(), msg); err != nil {
		t.Fatalf("RouteInbound err: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&entry.calls) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&entry.calls); got != 1 {
		t.Fatalf("orchestrationEntry must be called once; got %d", got)
	}
}

func TestMatrix_StopProcess_D7True(t *testing.T) {
	gw := newTestGateway(t)
	var cancels int32
	rec := &cancelRecorder{calls: &cancels}
	gw.SetOrchestrationEntry(rec)
	if err := gw.StopProcess("sess-stop-mat"); err != nil {
		t.Fatalf("StopProcess err: %v", err)
	}
	if atomic.LoadInt32(&cancels) != 1 {
		t.Fatalf("Cancel not invoked")
	}
}

type cancelRecorder struct {
	calls *int32
}

func (c *cancelRecorder) ProcessMessage(_ context.Context, sessionID, message string) (<-chan *contracts.EngineEvent, error) {
	ch := make(chan *contracts.EngineEvent, 1)
	ch <- &contracts.EngineEvent{Type: "text", SessionID: sessionID, Content: message}
	close(ch)
	return ch, nil
}

func (c *cancelRecorder) Cancel(_ context.Context, _ string) error {
	atomic.AddInt32(c.calls, 1)
	return nil
}
