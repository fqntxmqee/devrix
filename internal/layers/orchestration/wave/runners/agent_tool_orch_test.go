package runners

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent/tool"
	"github.com/devrix/devrix/internal/layers/orchestration/wave"
)

// ORCH-S2-T21 (stub): the runner bridge half is fully covered. The
// process-kill half (OS-level SIGTERM on the actual cursor / claude-code
// subprocess) is verified by cursor_adapter_test.go and cli_adapter_test.go
// in the multiagent package. This test pins the contract that the
// WorkerEvent channel sees a 'cancelled' event with the right content.
func TestAgentToolRunner_L5_ORCH_21_CancelledEmitted(t *testing.T) {
	fake := &fakeAgentTool{
		events:           []tool.Event{{Type: "thinking", Content: "start"}},
		blockUntilCancel: true,
	}
	reg := tool.NewRegistry()
	_ = reg.Register(fake)

	r := NewAgentToolRunner(wave.WorkerCursor, AgentToolDeps{Registry: reg})

	var cancelledCount atomic.Int32
	var lastCancel string
	emit := func(ev wave.WorkerEvent) {
		if ev.Type == "cancelled" {
			cancelledCount.Add(1)
			lastCancel = ev.Content
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	spec := wave.WorkerRunSpec{
		SessionID: "s1",
		TaskID:    "t1",
		WorkDir:   "/tmp",
		Directive: "do x",
		Emit:      emit,
	}
	err := r.Run(ctx, spec)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if cancelledCount.Load() != 1 {
		t.Fatalf("expected exactly 1 cancelled event, got %d", cancelledCount.Load())
	}
	if lastCancel == "" {
		t.Fatal("expected non-empty cancellation reason")
	}
}
