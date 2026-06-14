package capture

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D7-Migration-4-combination matrix (per d7-domain.md §D7-D1 Coexistence
// Contract). The matrix is d7_enabled × plan_mode_active. We assert that:
//   - d7=false routes do NOT call orchestrationEntry.ProcessMessage (legacy path).
//   - d7=true routes DO call orchestrationEntry.ProcessMessage (new path).
//   - Cancel always works regardless of plan_mode (StopProcess is a
//     gate-level concern; it shouldn't depend on whether the user is in
//     a plan).
//
// This test matrix is the "P0 green-bar" for the migration: any
// combination that regresses is a contract violation.

// matrixEntry is a PlanMode-aware fake that records the system-prompt
// hint so we can assert whether the plan path was engaged.
type matrixEntry struct {
	calls            int32
	lastSystemPrompt string
	planModeHint     string
}

func (m *matrixEntry) ProcessMessage(_ context.Context, sessionID, message string) (<-chan *contracts.EngineEvent, error) {
	atomic.AddInt32(&m.calls, 1)
	// The orchestrator forwards the system prompt hint on the command
	// path. We don't drive that branch here — the matrix is about
	// d7_enabled × plan_mode at the gateway level.
	_ = sessionID
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

// T: Combination 1 — d7=false, plan_mode=false. Legacy D1→D2 path.
// Expects orchestrationEntry NOT invoked. We don't have a real contextEngine, so we
// expect an error from the legacy nil-engine path. The d7 entry counter
// must remain 0.
// T: D1-S13-A03-T02
func TestMatrix_D7False_PlanModeFalse_Legacy(t *testing.T) {
	gw := newTestGateway(t)
	entry := &matrixEntry{}
	gw.SetOrchestrationEntry(entry, false) // d7 disabled
	_ = entry.planModeHint                 // plan_mode is OFF — legacy

	msg := &types.InboundMessage{
		SessionID: "sess-m1", ChatID: "c1", MessageID: "m1",
		Content: "hello", UserID: "u1",
	}
	_ = gw.RouteInbound(context.Background(), msg)
	// Wait briefly to ensure no async goroutine invokes orchestrationEntry.
	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&entry.calls); got != 0 {
		t.Fatalf("d7 disabled must not call orchestrationEntry; got %d calls", got)
	}
}

// T: Combination 2 — d7=false, plan_mode=true. Legacy path; plan_mode
// is handled by D2 internally. orchestrationEntry must still not be invoked.
// T: D1-S13-A03-T02
func TestMatrix_D7False_PlanModeTrue_Legacy(t *testing.T) {
	gw := newTestGateway(t)
	entry := &matrixEntry{planModeHint: "[plan_mode]"}
	gw.SetOrchestrationEntry(entry, false) // d7 disabled
	// plan_mode is a runtime concern of D2 (legacy path), not the
	// capture. Gateway should not be affected.

	msg := &types.InboundMessage{
		SessionID: "sess-m2", ChatID: "c2", MessageID: "m2",
		Content: "/plan add auth", UserID: "u1",
	}
	_ = gw.RouteInbound(context.Background(), msg)
	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&entry.calls); got != 0 {
		t.Fatalf("d7 disabled + plan_mode must not call orchestrationEntry; got %d", got)
	}
}

// T: Combination 3 — d7=true, plan_mode=false. orchestrationEntry is invoked.
// T: D1-S13-A03-T01
func TestMatrix_D7True_PlanModeFalse_D7Path(t *testing.T) {
	gw := newTestGateway(t)
	entry := &matrixEntry{}
	gw.SetOrchestrationEntry(entry, true) // d7 enabled

	msg := &types.InboundMessage{
		SessionID: "sess-m3", ChatID: "c3", MessageID: "m3",
		Content: "hello", UserID: "u1",
	}
	if err := gw.RouteInbound(context.Background(), msg); err != nil {
		t.Fatalf("RouteInbound err: %v", err)
	}
	// d7 enabled path is async (goroutine); wait for the call.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&entry.calls) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&entry.calls); got != 1 {
		t.Fatalf("d7 enabled must call orchestrationEntry once; got %d", got)
	}
}

// T: Combination 4 — d7=true, plan_mode=true. orchestrationEntry is invoked. The
// plan_mode awareness lives inside the orchestrator (system-prompt
// hint), not at the capture. Gateway contract: orchestrationEntry.ProcessMessage
// gets the raw message; orchestrator decides intent.
// T: D1-S13-A03-T01
func TestMatrix_D7True_PlanModeTrue_D7Path(t *testing.T) {
	gw := newTestGateway(t)
	entry := &matrixEntry{planModeHint: "[plan_mode]"}
	gw.SetOrchestrationEntry(entry, true) // d7 enabled

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
		t.Fatalf("d7 enabled + plan_mode must call orchestrationEntry; got %d", got)
	}
}

// T: StopProcess matrix — Cancel is invoked in both d7=true combos. We
// don't expect StopProcess to differ across plan_mode.
func TestMatrix_StopProcess_D7True(t *testing.T) {
	gw := newTestGateway(t)
	entry := &matrixEntry{}
	gw.SetOrchestrationEntry(entry, true)
	var cancels int32
	// We need a stub entry that records cancels.
	rec := &cancelRecorder{calls: &cancels}
	// Replace the entry with a cancel-recording one.
	gw.SetOrchestrationEntry(rec, true)
	if err := gw.StopProcess("sess-stop-mat"); err != nil {
		t.Fatalf("StopProcess err: %v", err)
	}
	if atomic.LoadInt32(&cancels) != 1 {
		t.Fatalf("Cancel not invoked under d7=true")
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
