package runners

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent/external"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

// ORCH-S2-T21 (D7-S3-T11): the runner bridge half (cancelled event emission) is
// covered here. The OS-level SIGTERM/SIGKILL path is verified by
// TestCLIAgentTool_Stop_kills_process_that_survives_stdin_close in
// multiagent/external/cli_adapter_test.go.
func TestAgentToolRunner_L5_ORCH_21_CancelledEmitted(t *testing.T) {
	fake := &fakeAgentTool{
		events:           []external.Event{{Type: "thinking", Content: "start"}},
		blockUntilCancel: true,
	}
	reg := external.NewRegistry()
	_ = reg.Register(fake)

	r := NewAgentToolRunner(wavescheduler.WorkerCursor, AgentToolDeps{Registry: reg})

	var cancelledCount atomic.Int32
	var lastCancel string
	emit := func(ev wavescheduler.WorkerEvent) {
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
	spec := wavescheduler.WorkerRunSpec{
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
