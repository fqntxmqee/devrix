package materialize

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestCompressMessages_preservesAssistantToolRoundFromTail(t *testing.T) {
	calls := `[{"id":"call_a","type":"function","function":{"name":"read_file","arguments":"{}"}}]`
	big := strings.Repeat("x", 12000)
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "review"},
		{Role: types.MessageRoleAssistant, Content: "old", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
		{Role: types.MessageRoleTool, Content: strings.Repeat("y", 12000), Metadata: map[string]string{conversation.MetaToolCallID: "call_a"}},
		{Role: types.MessageRoleAssistant, Content: "read", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
		{Role: types.MessageRoleTool, Content: big, Metadata: map[string]string{conversation.MetaToolCallID: "call_a"}},
	}
	got := compressMessages(msgs, 8000)
	if len(got) == 0 {
		t.Fatal("expected non-empty compressed chain")
	}
	hasAssistant := false
	hasTool := false
	for _, m := range got {
		if m.Role == types.MessageRoleAssistant && len(conversation.ToolCallsFromAssistant(m)) > 0 {
			hasAssistant = true
		}
		if m.Role == types.MessageRoleTool && strings.Contains(m.Content, "x") {
			hasTool = true
		}
	}
	if !hasAssistant || !hasTool {
		t.Fatalf("expected assistant+tool round preserved, got roles=%v", messageRoles(got))
	}
	repaired := conversation.RepairToolMessageChain(got)
	if len(repaired) == 0 {
		t.Fatal("repaired chain empty")
	}
	foundToolEvidence := false
	for _, m := range repaired {
		if m.Role == types.MessageRoleTool && len(m.Content) > 100 {
			foundToolEvidence = true
		}
	}
	if !foundToolEvidence {
		t.Fatal("repair dropped all tool evidence after segment compress")
	}
}

func messageRoles(msgs []types.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = string(m.Role)
	}
	return out
}
