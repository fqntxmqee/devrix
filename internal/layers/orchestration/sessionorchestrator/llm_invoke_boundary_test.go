package sessionorchestrator

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/usercontext"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestMessagesForLLMInvoke_prependsWhenAbsent(t *testing.T) {
	uc := map[string]string{"workDir": "/tmp/proj"}
	msgs := []types.Message{{Role: types.MessageRoleUser, Content: "hi"}}
	out := messagesForLLMInvoke(msgs, uc)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2 (meta + user)", len(out))
	}
	if out[0].Metadata[conversation.MetaIsMeta] != "true" {
		t.Fatal("expected meta prepend message")
	}
}

func TestMessagesForLLMInvoke_idempotentWhenMetaPresent(t *testing.T) {
	uc := map[string]string{"workDir": "/tmp/proj"}
	withMeta := usercontext.PrependForAPI(
		[]types.Message{{Role: types.MessageRoleUser, Content: "hi"}},
		uc,
	)
	out := messagesForLLMInvoke(withMeta, uc)
	if len(out) != len(withMeta) {
		t.Fatalf("should not double-prepend: got %d msgs, want %d", len(out), len(withMeta))
	}
}
