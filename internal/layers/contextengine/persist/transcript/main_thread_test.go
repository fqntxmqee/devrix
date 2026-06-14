package transcript_test

import (
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/persist/transcript"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestMainThreadStore_should_append_and_load(t *testing.T) {
	dir := t.TempDir()
	store, err := transcript.NewMainThreadStore(dir)
	if err != nil {
		t.Fatalf("NewMainThreadStore: %v", err)
	}
	msg := types.Message{Role: types.MessageRoleUser, Content: "hello", SessionID: "sess_1"}
	if err := store.Append("sess_1", msg); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := store.Load("sess_1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0].Content != "hello" {
		t.Fatalf("loaded = %+v", got)
	}
	if _, err := filepath.Abs(dir); err != nil {
		t.Fatalf("temp dir: %v", err)
	}
}
