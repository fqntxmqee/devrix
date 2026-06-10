package tool

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

func TestCLIAgentTool_Execute_StreamJSON(t *testing.T) {
	tool := NewCLIAgentTool(CLIConfig{
		Name:         "echo-test",
		DisplayName:  "Echo Test",
		Description:  "Test tool that echoes stream-json",
		Command:      "bash",
		Args:         []string{"-c", `echo '{"type":"text","content":"hello"}'; echo '{"type":"complete","content":""}'`},
		Timeout:      5 * time.Second,
	})

	ctx := context.Background()
	ch, err := tool.Execute(ctx, "sess_test", Request{Task: "test"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var events []Event
	for evt := range ch {
		events = append(events, evt)
	}

	if len(events) < 2 {
		t.Fatalf("got %d events, want at least 2", len(events))
	}
	if events[0].Type != "text" || events[0].Content != "hello" {
		t.Errorf("first event = %+v, want {text hello}", events[0])
	}
	if events[len(events)-1].Type != "complete" {
		t.Errorf("last event type = %q, want 'complete'", events[len(events)-1].Type)
	}

	// Cleanup
	tool.Stop()
}

func TestCLIAgentTool_Execute_NonJSONLine(t *testing.T) {
	tool := NewCLIAgentTool(CLIConfig{
		Name:    "plain-test",
		Command: "bash",
		Args:    []string{"-c", `echo "plain text"; echo '{"type":"complete","content":""}'`},
		Timeout: 5 * time.Second,
	})

	ctx := context.Background()
	ch, err := tool.Execute(ctx, "sess_nonjson", Request{Task: "test"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var events []Event
	for evt := range ch {
		events = append(events, evt)
	}

	if len(events) < 2 {
		t.Fatalf("got %d events, want >= 2", len(events))
	}
	if events[0].Type != "text" || !strings.Contains(events[0].Content, "plain text") {
		t.Errorf("first event = %+v, want {text ...plain text...}", events[0])
	}

	tool.Stop()
}

func TestCLIAgentTool_Execute_Timeout(t *testing.T) {
	tool := NewCLIAgentTool(CLIConfig{
		Name:    "sleep-test",
		Command: "bash",
		Args:    []string{"-c", `echo '{"type":"text","content":"start"}'; sleep 10; echo '{"type":"complete","content":""}'`},
		Timeout: 1 * time.Minute,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	ch, err := tool.Execute(ctx, "sess_timeout", Request{Task: "test"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var events []Event
	for evt := range ch {
		events = append(events, evt)
	}

	// Should at least get the "start" text before timeout
	if len(events) == 0 {
		t.Fatal("expected at least one event before timeout")
	}

	tool.Stop()
}

func TestCLIAgentTool_SessionCreateAndReuse(t *testing.T) {
	tool := NewCLIAgentTool(CLIConfig{
		Name:    "session-test",
		Command: "bash",
		Args:    []string{"-c", `while read line; do echo '{"type":"text","content":"ok"}'; echo '{"type":"complete","content":""}'; done`},
		Timeout: 10 * time.Second,
	})
	defer tool.Stop()

	ctx := context.Background()

	// First call — creates session
	ch1, err := tool.Execute(ctx, "sess_reuse", Request{Task: "first"})
	if err != nil {
		t.Fatalf("first Execute failed: %v", err)
	}
	for evt := range ch1 {
		if evt.Type == "complete" {
			break
		}
	}

	// Second call — reuses session
	ch2, err := tool.Execute(ctx, "sess_reuse", Request{Task: "second"})
	if err != nil {
		t.Fatalf("second Execute failed: %v", err)
	}
	for evt := range ch2 {
		if evt.Type == "complete" {
			break
		}
	}

	// Verify only one session was created
	tool.mu.RLock()
	count := len(tool.sessions)
	tool.mu.RUnlock()
	if count != 1 {
		t.Errorf("expected 1 session, got %d", count)
	}
}

func TestCLIAgentTool_SessionIsolation(t *testing.T) {
	tool := NewCLIAgentTool(CLIConfig{
		Name:    "isolate-test",
		Command: "bash",
		Args:    []string{"-c", `while read line; do echo '{"type":"text","content":"ok"}'; echo '{"type":"complete","content":""}'; done`},
		Timeout: 10 * time.Second,
	})
	defer tool.Stop()

	ctx := context.Background()

	ch1, err := tool.Execute(ctx, "sess_a", Request{Task: "a"})
	if err != nil {
		t.Fatalf("sess_a Execute failed: %v", err)
	}
	for evt := range ch1 {
		if evt.Type == "complete" {
			break
		}
	}

	ch2, err := tool.Execute(ctx, "sess_b", Request{Task: "b"})
	if err != nil {
		t.Fatalf("sess_b Execute failed: %v", err)
	}
	for evt := range ch2 {
		if evt.Type == "complete" {
			break
		}
	}

	tool.mu.RLock()
	count := len(tool.sessions)
	tool.mu.RUnlock()
	if count != 2 {
		t.Errorf("expected 2 sessions, got %d", count)
	}
}

func TestCLIAgentTool_CleanupBySessionID(t *testing.T) {
	tool := NewCLIAgentTool(CLIConfig{
		Name:    "cleanup-test",
		Command: "bash",
		Args:    []string{"-c", `while read line; do echo '{"type":"text","content":"ok"}'; echo '{"type":"complete","content":""}'; done`},
		Timeout: 10 * time.Second,
	})
	defer tool.Stop()

	ctx := context.Background()

	// Create two sessions
	ch1, _ := tool.Execute(ctx, "sess_1", Request{Task: "a"})
	for evt := range ch1 {
		if evt.Type == "complete" {
			break
		}
	}
	ch2, _ := tool.Execute(ctx, "sess_2", Request{Task: "b"})
	for evt := range ch2 {
		if evt.Type == "complete" {
			break
		}
	}

	// Cleanup sess_1
	tool.CleanupBySessionID("sess_1")

	tool.mu.RLock()
	count := len(tool.sessions)
	tool.mu.RUnlock()
	if count != 1 {
		t.Errorf("expected 1 session after cleanup, got %d", count)
	}

	// sess_2 should still work
	ch3, err := tool.Execute(ctx, "sess_2", Request{Task: "c"})
	if err != nil {
		t.Fatalf("reuse sess_2 after cleanup failed: %v", err)
	}
	for evt := range ch3 {
		if evt.Type == "complete" {
			break
		}
	}
}

func TestCLIAgentTool_ConcurrentSessionAccess(t *testing.T) {
	tool := NewCLIAgentTool(CLIConfig{
		Name:    "concurrent-test",
		Command: "bash",
		Args:    []string{"-c", `while read line; do echo '{"type":"text","content":"ok"}'; echo '{"type":"complete","content":""}'; done`},
		Timeout: 30 * time.Second,
	})
	defer tool.Stop()

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sessionID := "sess_" + string(rune('A'+id))
			ch, err := tool.Execute(ctx, sessionID, Request{Task: "test"})
			if err != nil {
				t.Errorf("Execute(%s) failed: %v", sessionID, err)
				return
			}
			for evt := range ch {
				if evt.Type == "complete" {
					break
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestCLIAgentTool_IdleTimeout(t *testing.T) {
	tool := NewCLIAgentTool(CLIConfig{
		Name:        "idle-test",
		Command:     "bash",
		Args:        []string{"-c", `while read line; do echo '{"type":"text","content":"ok"}'; echo '{"type":"complete","content":""}'; done`},
		Timeout:     10 * time.Second,
		IdleTimeout: 200 * time.Millisecond,
	})
	defer tool.Stop()

	ctx := context.Background()
	ch, _ := tool.Execute(ctx, "sess_idle", Request{Task: "test"})
	for evt := range ch {
		if evt.Type == "complete" {
			break
		}
	}

	// Manually trigger idle sweep (sweeper runs at 30s intervals; we call directly in tests)
	time.Sleep(300 * time.Millisecond) // must exceed IdleTimeout (200ms)
	tool.reapIdle()

	tool.mu.RLock()
	count := len(tool.sessions)
	tool.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 sessions after idle timeout, got %d", count)
	}
}

func TestCLIAgentTool_Stop(t *testing.T) {
	tool := NewCLIAgentTool(CLIConfig{
		Name:    "stop-test",
		Command: "bash",
		Args:    []string{"-c", `while read line; do echo '{"type":"text","content":"ok"}'; echo '{"type":"complete","content":""}'; done`},
		Timeout: 10 * time.Second,
	})

	ctx := context.Background()
	ch, _ := tool.Execute(ctx, "sess_stop", Request{Task: "test"})
	for evt := range ch {
		if evt.Type == "complete" {
			break
		}
	}

	tool.Stop()

	tool.mu.RLock()
	count := len(tool.sessions)
	tool.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 sessions after Stop, got %d", count)
	}
}

func TestCLIAgentTool_Execute_should_propagate_trace_env(t *testing.T) {
	traceID, err := tracer.ParseTraceID("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("trace id: %v", err)
	}
	spanID, err := tracer.ParseSpanID("00f067aa0ba902b7")
	if err != nil {
		t.Fatalf("span id: %v", err)
	}
	ctx := tracer.ContextWithSpan(context.Background(), tracer.SpanContext{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: tracer.FlagSampled,
	})
	ctx = tracer.DefaultBaggageManager.Set(ctx, "session.id", "sess_env")

	tool := NewCLIAgentTool(CLIConfig{
		Name:    "env-test",
		Command: "bash",
		Args: []string{"-c", `test -n "$TRACEPARENT" && test -n "$BAGGAGE" && echo '{"type":"complete","content":""}'`},
		Timeout: 5 * time.Second,
	})

	ch, err := tool.Execute(ctx, "sess_env", Request{Task: "check env"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	for evt := range ch {
		if evt.Type == "error" {
			t.Fatalf("unexpected error event: %s", evt.Content)
		}
	}
	tool.Stop()
}
