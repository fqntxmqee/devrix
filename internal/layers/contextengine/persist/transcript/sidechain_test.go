package transcript_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/persist/transcript"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S10-A01-T42
func TestSidechainStore_append_and_load_rebuilds_messages(t *testing.T) {
	dir := t.TempDir()
	store, err := transcript.NewSidechainStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "explore auth"},
		{Role: types.MessageRoleAssistant, Content: "found jwt middleware"},
	}
	for _, m := range msgs {
		if err := store.Append("sess1", "explore_abc", m); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := store.Load("sess1", "explore_abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded))
	}
	if loaded[1].Content != "found jwt middleware" {
		t.Fatalf("unexpected content: %q", loaded[1].Content)
	}
}
