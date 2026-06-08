package contextengine

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type slowToolRunner struct {
	delay time.Duration
	calls atomic.Int32
}

func (s *slowToolRunner) Execute(ctx context.Context, call ToolCall) (*ToolResult, error) {
	s.calls.Add(1)
	select {
	case <-ctx.Done():
		return &ToolResult{Error: ctx.Err().Error()}, nil
	case <-time.After(s.delay):
		return &ToolResult{Output: "ok"}, nil
	}
}

// Covers: L5-TOOL-04
func TestToolLimiter_should_queue_excess_concurrent_calls(t *testing.T) {
	inner := &slowToolRunner{delay: 200 * time.Millisecond}
	limiter := NewToolLimiter(2)
	runner := NewLimitedToolRunner(inner, limiter)

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = runner.Execute(context.Background(), ToolCall{Name: "bash", Input: "sleep 0"})
		}()
	}
	wg.Wait()

	if inner.calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", inner.calls.Load())
	}
	if elapsed := time.Since(start); elapsed < 300*time.Millisecond {
		t.Fatalf("expected queued execution, elapsed=%s", elapsed)
	}
}

// Covers: L5-TOOL-04
func TestToolLimiter_should_respect_context_cancellation(t *testing.T) {
	inner := &slowToolRunner{delay: 2 * time.Second}
	limiter := NewToolLimiter(1)
	runner := NewLimitedToolRunner(inner, limiter)

	blocker, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = runner.Execute(blocker, ToolCall{Name: "bash"})
	}()

	time.Sleep(30 * time.Millisecond)

	ctx, cancelWait := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelWait()
	result, err := runner.Execute(ctx, ToolCall{Name: "bash"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected acquire timeout error")
	}
	cancel()
	wg.Wait()
}
