package fallback_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/fallback"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S9-A01-T07
func TestTranscriptManager_should_append_and_compact(t *testing.T) {
	mgr := fallback.NewTranscriptManager(config.TranscriptConfig{
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
