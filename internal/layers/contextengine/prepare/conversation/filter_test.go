package conversation_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestFilterIncompleteToolCalls_should_drop_pending_tool_use(t *testing.T) {
	calls := `[{"id":"c1","type":"function","function":{"name":"bash","arguments":"{}"}}]`
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "hi"},
		{Role: types.MessageRoleAssistant, Content: "", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
	}
	out := conversation.FilterIncompleteToolCalls(msgs)
	if len(out) != 1 {
		t.Fatalf("expected incomplete assistant dropped, got %d messages", len(out))
	}
}

func TestFilterIncompleteToolCalls_should_keep_complete_round(t *testing.T) {
	calls := `[{"id":"c1","type":"function","function":{"name":"bash","arguments":"{}"}}]`
	msgs := []types.Message{
		{Role: types.MessageRoleAssistant, Content: "", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
		{Role: types.MessageRoleTool, Content: "ok", Metadata: map[string]string{conversation.MetaToolCallID: "c1"}},
	}
	out := conversation.FilterIncompleteToolCalls(msgs)
	if len(out) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out))
	}
}
