package conversation_test

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/conversation"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestRepairToolMessageChain_should_drop_orphan_tool_results(t *testing.T) {
	calls := `[{"id":"call_a","type":"function","function":{"name":"Read","arguments":"{}"}}]`
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "hi"},
		{Role: types.MessageRoleAssistant, Content: "read", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
		{Role: types.MessageRoleTool, Content: "ok", Metadata: map[string]string{conversation.MetaToolCallID: "call_a"}},
		{Role: types.MessageRoleTool, Content: "orphan", Metadata: map[string]string{conversation.MetaToolCallID: "call_b"}},
	}
	out := conversation.RepairToolMessageChain(msgs)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
	if out[2].Role != types.MessageRoleTool || out[2].Content != "ok" {
		t.Fatalf("last tool = %+v", out[2])
	}
}

func TestSanitizeToolErrorForLLM_should_strip_config_hint(t *testing.T) {
	raw := `sandbox: command not allowed: { (add to tool.allowlist in config). This is a sandbox policy (not permission/YOLO); use relative paths under WorkDir or read_file/glob/list_dir for files.`
	got := conversation.SanitizeToolErrorForLLM("bash", raw)
	if strings.Contains(got, "allowlist") || strings.Contains(got, "YOLO") {
		t.Fatalf("got %q", got)
	}
}

func TestRepairToolMessageChain_should_reset_pending_on_user_message(t *testing.T) {
	// Regression: without the user message case, pending leaks through, causing
	// a tool result from a prior round to pair with a stale assistant call across
	// the user boundary. MiniMax rejects such pairs with error 2013.
	calls := `[{"id":"call_a","type":"function","function":{"name":"Read","arguments":"{}"}}]`
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "check files"},
		{Role: types.MessageRoleAssistant, Content: "thinking", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
		// Process interruption — no tool result for call_a
		{Role: types.MessageRoleUser, Content: "did that work?"},
		// Stale tool result from prior round arriving after user message
		{Role: types.MessageRoleTool, Content: "files ok", Metadata: map[string]string{conversation.MetaToolCallID: "call_a"}},
	}
	out := conversation.RepairToolMessageChain(msgs)
	// Without the fix: 4 messages (tool result leaks through)
	// With the fix: 3 messages (pending reset on user, tool result dropped)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3 (stale tool result dropped after user boundary)", len(out))
	}
	if out[len(out)-1].Role != types.MessageRoleUser {
		t.Fatalf("last message should be 'did that work?' user, got role=%v content=%q",
			out[len(out)-1].Role, out[len(out)-1].Content)
	}
}

func TestRepairToolMessageChain_should_drop_incomplete_last_tool_round(t *testing.T) {
	calls := `[{"id":"call_x","type":"function","function":{"name":"bash","arguments":"{}"}}]`
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "run"},
		{Role: types.MessageRoleAssistant, Content: "ok", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
		{Role: types.MessageRoleTool, Content: "done", Metadata: map[string]string{conversation.MetaToolCallID: "call_x"}},
		{Role: types.MessageRoleUser, Content: "again"},
		{Role: types.MessageRoleAssistant, Content: "sure", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
		// Missing tool result for call_x — the assistant should be stripped
	}
	out := conversation.RepairToolMessageChain(msgs)
	if len(out) != 4 {
		t.Fatalf("len = %d, want 4 (incomplete assistant round stripped)", len(out))
	}
	if out[len(out)-1].Role == types.MessageRoleAssistant {
		t.Fatalf("last message should not be assistant without tool result")
	}
}

func TestRepairToolMessageChain_should_preserve_valid_complete_chain(t *testing.T) {
	calls := `[{"id":"call_a","type":"function","function":{"name":"bash","arguments":"{}"}}]`
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "deploy"},
		{Role: types.MessageRoleAssistant, Content: "", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
		{Role: types.MessageRoleTool, Content: "done", Metadata: map[string]string{conversation.MetaToolCallID: "call_a"}},
		{Role: types.MessageRoleUser, Content: "thanks"},
	}
	out := conversation.RepairToolMessageChain(msgs)
	if len(out) != 4 {
		t.Fatalf("len = %d, want 4 (valid chain preserved)", len(out))
	}
}

func TestRepairToolMessageChain_should_drop_stale_tool_result_after_complete_round(t *testing.T) {
	// A stray tool result that arrives after a complete round (all calls resolved
	// before the user message) and uses a different call ID than the next round.
	calls := `[{"id":"call_a","type":"function","function":{"name":"Read","arguments":"{}"}}]`
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "round1"},
		{Role: types.MessageRoleAssistant, Content: "", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
		{Role: types.MessageRoleTool, Content: "r1 done", Metadata: map[string]string{conversation.MetaToolCallID: "call_a"}},
		{Role: types.MessageRoleUser, Content: "round2"},
		{Role: types.MessageRoleAssistant, Content: "", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
		{Role: types.MessageRoleTool, Content: "r2 done", Metadata: map[string]string{conversation.MetaToolCallID: "call_a"}},
		// Stale tool result using a never-declared call ID
		{Role: types.MessageRoleTool, Content: "stale", Metadata: map[string]string{conversation.MetaToolCallID: "call_stale"}},
	}
	out := conversation.RepairToolMessageChain(msgs)
	if len(out) != 6 {
		t.Fatalf("len = %d, want 6 (stale tool result dropped)", len(out))
	}
}

func TestRepairToolMessageChain_should_drop_tool_result_without_any_assistant(t *testing.T) {
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "hello"},
		{Role: types.MessageRoleTool, Content: "stray", Metadata: map[string]string{conversation.MetaToolCallID: "call_orphan"}},
	}
	out := conversation.RepairToolMessageChain(msgs)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (stray tool dropped)", len(out))
	}
}

func TestRepairToolMessageChain_should_handle_empty_input(t *testing.T) {
	out := conversation.RepairToolMessageChain(nil)
	if out != nil {
		t.Fatalf("expected nil for nil input")
	}
	out = conversation.RepairToolMessageChain([]types.Message{})
	if len(out) != 0 {
		t.Fatalf("len = %d, want 0", len(out))
	}
}

func TestRepairToolMessageChain_should_handle_assistant_without_tool_calls(t *testing.T) {
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "hello"},
		{Role: types.MessageRoleAssistant, Content: "hi there"},
	}
	out := conversation.RepairToolMessageChain(msgs)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
}

func TestRepairToolMessageChain_should_handle_multiple_tool_calls_in_one_assistant(t *testing.T) {
	calls := `[{"id":"c1","type":"function","function":{"name":"bash","arguments":"{}"}},{"id":"c2","type":"function","function":{"name":"read","arguments":"{}"}}]`
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "do things"},
		{Role: types.MessageRoleAssistant, Content: "", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
		{Role: types.MessageRoleTool, Content: "bash ok", Metadata: map[string]string{conversation.MetaToolCallID: "c1"}},
		// c2 result missing — assistant should be stripped
	}
	out := conversation.RepairToolMessageChain(msgs)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (incomplete multi-call assistant stripped)", len(out))
	}
}

func TestFilterIncompleteToolCalls_should_trim_trailing_incomplete_assistant(t *testing.T) {
	calls := `[{"id":"call_x","type":"function","function":{"name":"bash","arguments":"{}"}}]`
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "hi"},
		{Role: types.MessageRoleAssistant, Content: "", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
	}
	out := conversation.FilterIncompleteToolCalls(msgs)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (incomplete trimmed)", len(out))
	}
}

func TestFilterIncompleteToolCalls_preserves_complete_when_all_resolved(t *testing.T) {
	calls := `[{"id":"call_a","type":"function","function":{"name":"bash","arguments":"{}"}}]`
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "hi"},
		{Role: types.MessageRoleAssistant, Content: "", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
		{Role: types.MessageRoleTool, Content: "ok", Metadata: map[string]string{conversation.MetaToolCallID: "call_a"}},
	}
	out := conversation.FilterIncompleteToolCalls(msgs)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3 (complete preserved)", len(out))
	}
}

func TestFilterIncompleteToolCalls_should_handle_empty(t *testing.T) {
	out := conversation.FilterIncompleteToolCalls(nil)
	if out != nil {
		t.Fatal("expected nil")
	}
	out = conversation.FilterIncompleteToolCalls([]types.Message{})
	if len(out) != 0 {
		t.Fatalf("len = %d", len(out))
	}
}
