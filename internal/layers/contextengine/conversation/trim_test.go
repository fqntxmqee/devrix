package conversation_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/conversation"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestHeadTailTrim_should_preserve_first_user_when_trimming(t *testing.T) {
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "build hot reload"},
	}
	for i := range 20 {
		id := fmt.Sprintf("c%d", i)
		msgs = append(msgs,
			types.Message{
				Role:     types.MessageRoleAssistant,
				Metadata: map[string]string{"tool_calls": `[{"id":"` + id + `","type":"function","function":{"name":"bash","arguments":"{}"}}]`},
			},
			types.Message{Role: types.MessageRoleTool, Content: "ok", Metadata: map[string]string{"tool_call_id": id}},
		)
	}
	msgs = append(msgs, types.Message{Role: types.MessageRoleUser, Content: "rollback"})

	out := conversation.HeadTailTrim(msgs, 10, 1, 6)
	if len(out) > 10 {
		t.Fatalf("len = %d, want <= 10", len(out))
	}
	if out[0].Role != types.MessageRoleUser || out[0].Content != "build hot reload" {
		t.Fatalf("first message = %+v, want original user task", out[0])
	}
	foundSnip := false
	for _, m := range out {
		if m.Role == types.MessageRoleSystem && strings.Contains(m.Content, "snipped") {
			foundSnip = true
		}
	}
	if !foundSnip {
		t.Fatal("expected snip placeholder in trimmed history")
	}
	for _, m := range out {
		if m.Role == types.MessageRoleSystem && strings.Contains(m.Content, "snipped") {
			if !conversation.IsCompactBoundary(m) {
				t.Fatal("snip placeholder should be a compact boundary")
			}
		}
	}
	last := out[len(out)-1]
	if last.Role != types.MessageRoleUser || last.Content != "rollback" {
		t.Fatalf("tail user = %+v, want rollback", last)
	}
}

func TestHeadTailTrim_should_noop_when_within_limit(t *testing.T) {
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "hi"},
		{Role: types.MessageRoleAssistant, Content: "hello"},
	}
	out := conversation.HeadTailTrim(msgs, 50, 2, 40)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
}
