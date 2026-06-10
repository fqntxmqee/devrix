package harness_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/harness"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-2-9-07
func TestTranscriptManager_should_append_and_compact(t *testing.T) {
	mgr := harness.NewTranscriptManager(config.TranscriptConfig{
		Enabled:           true,
		CompactAfterTurns: 2,
	})
	sc := &types.SessionContext{SessionID: "sess_tx"}
	mgr.AppendTurn(sc, "hello", "hi there")
	mgr.AppendTurn(sc, "second", "reply")
	mgr.AppendTurn(sc, "third", "ok")

	if sc.Transcript == nil {
		t.Fatal("expected transcript store")
	}
	if len(sc.Transcript.Entries) > 4 {
		t.Fatalf("expected compact to keep at most 4 entries, got %d", len(sc.Transcript.Entries))
	}
	replay := sc.Transcript.Replay()
	if len(replay) != len(sc.Transcript.Entries) {
		t.Fatal("replay length mismatch")
	}
}
