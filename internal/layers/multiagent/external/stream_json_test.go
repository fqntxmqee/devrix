package external

import (
	"context"
	"strings"
	"testing"
)

func TestParseStreamJSONLine_should_support_devrix_format(t *testing.T) {
	got := ParseStreamJSONLine(`{"type":"text","content":"hello"}`)
	if len(got.Events) != 1 || got.Events[0].Type != "text" || got.Events[0].Content != "hello" {
		t.Fatalf("text event = %+v", got.Events)
	}
	if got.Done {
		t.Fatal("text line should not end the stream")
	}

	done := ParseStreamJSONLine(`{"type":"complete","content":"done"}`)
	if !done.Done || len(done.Events) != 1 || done.Events[0].Type != "complete" {
		t.Fatalf("complete = %+v", done)
	}
}

func TestParseStreamJSONLine_should_parse_claude_assistant_text(t *testing.T) {
	line := `{"type":"assistant","message":{"id":"m1","type":"message","role":"assistant","content":[{"type":"thinking","thinking":"plan"},{"type":"text","text":"你好"}]}}`
	got := ParseStreamJSONLine(line)
	if len(got.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(got.Events))
	}
	if got.Events[0].Type != "thinking" || !strings.Contains(got.Events[0].Content, "plan") {
		t.Fatalf("thinking block = %+v", got.Events[0])
	}
	if got.Events[1].Type != "text" || got.Events[1].Content != "你好" {
		t.Fatalf("text block = %+v", got.Events[1])
	}
	if got.Done {
		t.Fatal("assistant line should not end stream")
	}
}

func TestParseStreamJSONLine_should_parse_claude_result_success(t *testing.T) {
	line := `{"type":"result","subtype":"success","is_error":false,"result":"你好","session_id":"sess"}`
	got := ParseStreamJSONLine(line)
	if !got.Done {
		t.Fatal("result line should end stream")
	}
	if len(got.Events) != 1 {
		t.Fatalf("events = %+v", got.Events)
	}
	if got.Events[0].Type != "complete" || got.Events[0].Content != "你好" {
		t.Fatalf("complete event = %+v", got.Events[0])
	}
}

func TestParseStreamJSONLine_should_parse_claude_result_error(t *testing.T) {
	line := `{"type":"result","subtype":"error","is_error":true,"result":"boom"}`
	got := ParseStreamJSONLine(line)
	if !got.Done {
		t.Fatal("error result should end stream")
	}
	if len(got.Events) != 1 || got.Events[0].Type != "error" || got.Events[0].Content != "boom" {
		t.Fatalf("error event = %+v", got.Events)
	}
}

func TestParseStreamJSONLine_should_ignore_claude_system_events(t *testing.T) {
	line := `{"type":"system","subtype":"init","session_id":"sess"}`
	got := ParseStreamJSONLine(line)
	if len(got.Events) != 0 || got.Done {
		t.Fatalf("system event should be ignored, got %+v", got)
	}
}

func TestCLIAgentTool_Execute_ClaudeStreamJSON(t *testing.T) {
	script := `echo '{"type":"system","subtype":"init"}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}'
echo '{"type":"result","subtype":"success","result":"hello"}'`

	agt := NewCLIAgentTool(CLIConfig{
		Name:    "claude-mock",
		Command: "bash",
		Args:    []string{"-c", script},
	})
	defer agt.Stop()

	ch, err := agt.Execute(context.Background(), "sess_claude", Request{Task: "test"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var events []Event
	for evt := range ch {
		events = append(events, evt)
	}
	if len(events) < 2 {
		t.Fatalf("events = %+v, want text + complete path", events)
	}
	if events[0].Type != "text" || events[0].Content != "hello" {
		t.Fatalf("first event = %+v", events[0])
	}
	last := events[len(events)-1]
	if last.Type != "complete" || last.Content != "hello" {
		t.Fatalf("last event = %+v", last)
	}
}
