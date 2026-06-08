package contextengine

import (
	"encoding/json"
	"testing"
)

func TestBuildAssistantToolCallsMessage_should_serialize_tool_calls(t *testing.T) {
	msg := buildAssistantToolCallsMessage("sess_1", []ToolCall{
		{ID: "call_1", Name: "bash", Input: `{"command":"pwd"}`},
	})
	raw := msg.Metadata[metaToolCalls]
	if raw == "" {
		t.Fatal("tool_calls metadata missing")
	}
	var calls []map[string]any
	if err := json.Unmarshal([]byte(raw), &calls); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("calls len = %d", len(calls))
	}
	fn, ok := calls[0]["function"].(map[string]any)
	if !ok || fn["name"] != "bash" {
		t.Fatalf("function: %+v", calls[0])
	}
}

func TestBuildToolResultMessage_should_include_tool_call_id(t *testing.T) {
	msg := buildToolResultMessage("sess_1", "call_1", "/tmp/work")
	if msg.Metadata[metaToolCallID] != "call_1" {
		t.Fatalf("tool_call_id = %q", msg.Metadata[metaToolCallID])
	}
	if msg.Role != "tool" || msg.Content != "/tmp/work" {
		t.Fatalf("msg: %+v", msg)
	}
}

func TestEnsureToolCallID_should_generate_when_missing(t *testing.T) {
	id := ensureToolCallID(ToolCall{Name: "bash"}, 0)
	if id == "" {
		t.Fatal("expected generated id")
	}
}
