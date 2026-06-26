package conversation_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestMessagesAfterCompactBoundary_should_return_from_last_boundary(t *testing.T) {
	boundary := conversation.NewCompactBoundaryMessage("s", "auto", 10, i18n.LocaleEN)
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "old task"},
		{Role: types.MessageRoleAssistant, Content: "old reply"},
		boundary,
		{Role: types.MessageRoleUser, Content: "new task"},
	}

	out := conversation.MessagesAfterCompactBoundary(msgs)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2 (boundary + new task)", len(out))
	}
	if !conversation.IsCompactBoundary(out[0]) {
		t.Fatal("expected boundary at start of active window")
	}
	if out[1].Content != "new task" {
		t.Fatalf("tail = %q", out[1].Content)
	}
}

func TestMessagesAfterCompactBoundary_should_return_all_when_no_boundary(t *testing.T) {
	msgs := []types.Message{{Role: types.MessageRoleUser, Content: "hi"}}
	out := conversation.MessagesAfterCompactBoundary(msgs)
	if len(out) != 1 || out[0].Content != "hi" {
		t.Fatalf("unexpected: %+v", out)
	}
}
