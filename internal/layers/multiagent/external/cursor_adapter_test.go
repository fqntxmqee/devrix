package external

import (
	"context"
	"sync"
	"testing"
	"time"
)

// bashCursorScript returns a bash -c script that outputs cursor-style stream-json events.
func bashCursorScript(sessionID, text string, isError bool) string {
	errField := "false"
	if isError {
		errField = "true"
	}
	return `echo '{"type":"system","session_id":"` + sessionID + `"}'; echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"` + text + `"}]}}'; echo '{"type":"result","subtype":"success","result":"` + text + `","session_id":"` + sessionID + `","is_error":` + errField + `}'`
}

func TestCursorAgentTool_Execute_BasicText(t *testing.T) {
	agt := NewCursorAgentTool(CursorConfig{
		Name:    "cursor-test",
		Command: "bash",
		Args:    []string{"-c", bashCursorScript("chat_s1", "hello from cursor", false)},
		WorkDir: ".",
		Timeout: 5 * time.Second,
	})
	defer agt.Stop()

	ctx := context.Background()
	ch, err := agt.Execute(ctx, "sess_basic", Request{Task: "ignored"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var events []Event
	for evt := range ch {
		events = append(events, evt)
	}

	if len(events) < 2 {
		t.Fatalf("got %d events, want at least 2 (text + complete)", len(events))
	}
	if events[0].Type != "text" || events[0].Content != "hello from cursor" {
		t.Errorf("first event = %+v, want {text hello from cursor}", events[0])
	}
	if events[len(events)-1].Type != "complete" {
		t.Errorf("last event type = %q, want 'complete'", events[len(events)-1].Type)
	}
}

func TestCursorAgentTool_SessionResume(t *testing.T) {
	agt := NewCursorAgentTool(CursorConfig{
		Name:    "cursor-resume",
		Command: "bash",
		Args:    []string{"-c", bashCursorScript("chat_abc", "first", false)},
		WorkDir: ".",
		Timeout: 5 * time.Second,
	})
	defer agt.Stop()

	ctx := context.Background()

	// First call — stores chatID
	ch1, err := agt.Execute(ctx, "sess_resume", Request{Task: "ignored"})
	if err != nil {
		t.Fatalf("first Execute failed: %v", err)
	}
	for evt := range ch1 {
		if evt.Type == "complete" {
			break
		}
	}

	// Verify chatID was stored
	func() {
		agt.mu.RLock()
		defer agt.mu.RUnlock()
		if agt.chatIDs["sess_resume"] != "chat_abc" {
			t.Errorf("stored chatID = %q, want %q", agt.chatIDs["sess_resume"], "chat_abc")
		}
	}()

	// Second call with different script to verify Args mode still works
	agt2 := NewCursorAgentTool(CursorConfig{
		Name:    "cursor-resume",
		Command: "bash",
		Args:    []string{"-c", bashCursorScript("chat_abc", "second", false)},
		WorkDir: ".",
		Timeout: 5 * time.Second,
	})
	ch2, err := agt2.Execute(ctx, "sess_other", Request{Task: "ignored"})
	if err != nil {
		t.Fatalf("second Execute failed: %v", err)
	}
	var events []Event
	for evt := range ch2 {
		events = append(events, evt)
	}
	if len(events) < 2 || events[0].Content != "second" {
		t.Errorf("event content = %q, want 'second'", events[0].Content)
	}
	agt2.Stop()
}

func TestCursorAgentTool_Execute_Error(t *testing.T) {
	agt := NewCursorAgentTool(CursorConfig{
		Name:    "cursor-error",
		Command: "bash",
		Args:    []string{"-c", bashCursorScript("chat_err", "API error", true)},
		WorkDir: ".",
		Timeout: 5 * time.Second,
	})
	defer agt.Stop()

	ctx := context.Background()
	ch, err := agt.Execute(ctx, "sess_error", Request{Task: "ignored"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var events []Event
	for evt := range ch {
		events = append(events, evt)
	}

	hasError, hasComplete := false, false
	for _, e := range events {
		if e.Type == "error" && e.Content == "API error" {
			hasError = true
		}
		if e.Type == "complete" {
			hasComplete = true
		}
	}
	if !hasError {
		t.Error("expected an error event with 'API error'")
	}
	if !hasComplete {
		t.Error("expected a complete event after error")
	}
}

func TestCursorAgentTool_Execute_Timeout(t *testing.T) {
	agt := NewCursorAgentTool(CursorConfig{
		Name:    "cursor-slow",
		Command: "bash",
		Args:    []string{"-c", `echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"start"}]}}'; sleep 10; echo '{"type":"result","subtype":"success","result":"done","session_id":"chat_to","is_error":false}'`},
		WorkDir: ".",
	})
	defer agt.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	ch, err := agt.Execute(ctx, "sess_timeout", Request{Task: "ignored"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var events []Event
	for evt := range ch {
		events = append(events, evt)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one event before timeout")
	}
}

func TestCursorAgentTool_ConcurrentSessions(t *testing.T) {
	agt := NewCursorAgentTool(CursorConfig{
		Name:    "cursor-concurrent",
		Command: "bash",
		WorkDir: ".",
		Timeout: 30 * time.Second,
	})
	defer agt.Stop()

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sid := "sess_c" + string(rune('A'+id))
			agt := NewCursorAgentTool(CursorConfig{
				Name:    "cursor-concurrent",
				Command: "bash",
				Args:    []string{"-c", bashCursorScript("chat_"+sid, "ok", false)},
				WorkDir: ".",
				Timeout: 10 * time.Second,
			})
			defer agt.Stop()

			ch, err := agt.Execute(ctx, sid, Request{Task: "ignored"})
			if err != nil {
				t.Errorf("Execute(%s) failed: %v", sid, err)
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

func TestCursorAgentTool_CloseSession(t *testing.T) {
	agt := NewCursorAgentTool(CursorConfig{
		Name:    "cursor-close",
		Command: "bash",
		Args:    []string{"-c", bashCursorScript("chat_close", "ok", false)},
		WorkDir: ".",
		Timeout: 5 * time.Second,
	})
	defer agt.Stop()

	ctx := context.Background()
	ch, _ := agt.Execute(ctx, "sess_close", Request{Task: "ignored"})
	for evt := range ch {
		if evt.Type == "complete" {
			break
		}
	}

	agt.CloseSession("sess_close")

	func() {
		agt.mu.RLock()
		defer agt.mu.RUnlock()
		if _, exists := agt.chatIDs["sess_close"]; exists {
			t.Error("chatID should be removed after CloseSession")
		}
	}()
}

func TestCursorAgentTool_CleanupBySessionID(t *testing.T) {
	agt := NewCursorAgentTool(CursorConfig{
		Name:    "cursor-cleanup",
		Command: "bash",
		Args:    []string{"-c", bashCursorScript("chat_a", "a", false)},
		WorkDir: ".",
		Timeout: 5 * time.Second,
	})
	defer agt.Stop()

	ctx := context.Background()

	ch1, _ := agt.Execute(ctx, "sess_a", Request{Task: "ignored"})
	for evt := range ch1 {
		if evt.Type == "complete" {
			break
		}
	}

	ch2, _ := agt.Execute(ctx, "sess_b", Request{Task: "ignored"})
	for evt := range ch2 {
		if evt.Type == "complete" {
			break
		}
	}

	agt.CleanupBySessionID("sess_a")

	func() {
		agt.mu.RLock()
		defer agt.mu.RUnlock()
		if _, exists := agt.chatIDs["sess_a"]; exists {
			t.Error("sess_a should be removed after CleanupBySessionID")
		}
		if _, exists := agt.chatIDs["sess_b"]; !exists {
			t.Error("sess_b should still exist after cleanup")
		}
	}()
}

func TestCursorAgentTool_Execute_ToolCallAndThinking(t *testing.T) {
	script := `echo '{"type":"thinking","subtype":"delta","text":"planning review"}'; ` +
		`echo '{"type":"tool_call","subtype":"started","tool_call":{"readToolCall":{"args":{"path":"/tmp/foo.go"}}}}'; ` +
		`echo '{"type":"tool_call","subtype":"completed","tool_call":{"readToolCall":{"result":{"success":{}}}}}'; ` +
		`echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}'; ` +
		`echo '{"type":"result","subtype":"success","result":"done","is_error":false}'`

	agt := NewCursorAgentTool(CursorConfig{
		Name:    "cursor-stream",
		Command: "bash",
		Args:    []string{"-c", script},
		WorkDir: ".",
		Timeout: 5 * time.Second,
	})
	defer agt.Stop()

	ch, err := agt.Execute(context.Background(), "sess_stream", Request{Task: "ignored"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var types []string
	for evt := range ch {
		types = append(types, evt.Type)
	}

	want := []string{"thinking", "tool_use", "text", "complete"}
	if len(types) != len(want) {
		t.Fatalf("event types = %v, want %v", types, want)
	}
	for i, wt := range want {
		if types[i] != wt {
			t.Errorf("event[%d] type = %q, want %q", i, types[i], wt)
		}
	}
}

func TestFormatCursorToolCallLabel(t *testing.T) {
	toolCall := map[string]any{
		"shellToolCall": map[string]any{
			"description": "List bootstrap files",
			"args": map[string]any{
				"command": "ls internal/bootstrap",
			},
		},
	}
	got := formatCursorToolCallLabel(toolCall)
	if got != "🔧 shell: List bootstrap files" {
		t.Errorf("label = %q", got)
	}
}

func TestCursorAgentTool_ExecutionTimeout(t *testing.T) {
	agt := NewCursorAgentTool(CursorConfig{Timeout: 12 * time.Minute})
	if got := agt.ExecutionTimeout(); got != 12*time.Minute {
		t.Errorf("ExecutionTimeout() = %v, want 12m", got)
	}
}

func TestCursorAgentTool_StopClearsAll(t *testing.T) {
	agt := NewCursorAgentTool(CursorConfig{
		Name:    "cursor-stop",
		Command: "bash",
		Args:    []string{"-c", bashCursorScript("chat_stop", "ok", false)},
		WorkDir: ".",
		Timeout: 5 * time.Second,
	})

	ctx := context.Background()
	ch, _ := agt.Execute(ctx, "sess_stop", Request{Task: "ignored"})
	for evt := range ch {
		if evt.Type == "complete" {
			break
		}
	}

	agt.Stop()

	func() {
		agt.mu.RLock()
		defer agt.mu.RUnlock()
		if len(agt.chatIDs) != 0 {
			t.Errorf("expected empty chatIDs after Stop, got %d", len(agt.chatIDs))
		}
	}()
}
