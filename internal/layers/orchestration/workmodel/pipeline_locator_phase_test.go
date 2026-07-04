package workmodel

import (
	"context"
	"strings"
	"testing"
)

// Regression: locator phase must be set before worktree set_round_phase emits
// so Jaeger breadcrumbs match the active MUPS node (not the previous phase).
func TestWithLocatorPhase_alignsWorktreeLocator(t *testing.T) {
	frame := LocatorFrame{
		SessionID:  "sess_x",
		SemanticID: "wi_d0_s0_goal",
		LoopTick:   1,
		RoundNo:    1,
		Trigger:    MUPSTriggerInitial,
	}
	ctx := WithLocatorFrame(context.Background(), frame)
	ctx = WithLocatorPhase(ctx, string(RoundPhaseObserve))
	f, ok := LocatorFrameFrom(ctx)
	if !ok {
		t.Fatal("frame missing")
	}
	loc := BuildLocator(f)
	if !strings.HasSuffix(loc, "/observe") {
		t.Fatalf("locator = %q, want suffix /observe", loc)
	}
	if f.Phase != string(RoundPhaseObserve) {
		t.Fatalf("phase = %q", f.Phase)
	}
}
