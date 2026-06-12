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
