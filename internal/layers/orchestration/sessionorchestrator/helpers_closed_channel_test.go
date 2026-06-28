package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// TestEmit_ClosedChannel_RecoversInsteadOfPanics verifies the 2026-06-28 hotfix
// for "send on closed channel" panic (DM-20260628-002). The previous
// implementation panicked the entire orchestrator process; the fixed
// implementation logs a warning and silently drops the late event.
//
// Repro scenario from production: sess_1782638991113_5000
//
//	session turn 1  → RunSessionTurnLoop creates out_1, executor.Emit = emitFn_1
//	timer / shutdown → defer close(out_1)
//	session turn 2  → RunSessionTurnLoop creates out_2, but executor.Emit
//	                   (without architectural fix) still holds emitFn_1
//	LLM stream chunk → e.Emit(ev) → emitFn_1(ev) → emit(... out_1 ...) → PANIC
//
// The emit() defensive recover is the backstop; item_pipeline.go's overwrite
// (vs "set if nil") is the architectural fix that prevents the bug from
// reaching emit() in the first place. This test exercises the recover path
// directly because it's the load-bearing safety net.
func TestEmit_ClosedChannel_RecoversInsteadOfPanics(t *testing.T) {
	out := make(chan *contracts.EngineEvent, 1)
	close(out) // simulate the owning goroutine having already finished

	ctx := context.Background()
	ev := &contracts.EngineEvent{Type: "text", Content: "late", SessionID: "sess_x"}

	// Must not panic. Without the recover() in helpers.go this line
	// crashes the goroutine with "send on closed channel".
	emit(ctx, nil, out, ev)
}

// TestEmit_OpenChannel_DeliversEvent verifies the recovery path doesn't
// disturb the normal happy-path (event still lands on the buffered channel).
func TestEmit_OpenChannel_DeliversEvent(t *testing.T) {
	out := make(chan *contracts.EngineEvent, 4)
	ctx := context.Background()
	ev := &contracts.EngineEvent{Type: "text", Content: "hello", SessionID: "sess_y"}

	emit(ctx, nil, out, ev)

	select {
	case got := <-out:
		if got.Type != "text" || got.Content != "hello" {
			t.Fatalf("got %+v, want text/hello", got)
		}
	default:
		t.Fatal("expected event delivered to buffered channel")
	}
}
