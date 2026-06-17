package contextengine_test

import (
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S3-A02-T01 (DM-20260617-003 devrix-d7-turn-history-persist)
func TestAppendAndTrimMessages_EmptyMessages(t *testing.T) {
	engine := newTestEngine(t)
	if err := engine.AppendAndTrimMessages("sess-empty", nil); err != nil {
		t.Errorf("AppendAndTrimMessages(nil): expected nil, got %v", err)
	}
	if err := engine.AppendAndTrimMessages("sess-empty", []types.Message{}); err != nil {
		t.Errorf("AppendAndTrimMessages([]): expected nil, got %v", err)
	}
	if _, ok := engine.SessionContext("sess-empty"); ok {
		t.Error("AppendAndTrimMessages(empty): should not create session")
	}
}

// T: D2-S3-A02-T01
func TestAppendAndTrimMessages_ExistingSession(t *testing.T) {
	engine := newTestEngine(t)
	sid := "sess-existing"

	// Seed an existing session via Process (creates D2 session in memory).
	session := types.NewSession(sid, "cli", t.TempDir())
	ch := engine.Process(t.Context(), session, "hello")
	for range ch {
	}

	preSC, _ := engine.SessionContext(sid)
	preLen := len(preSC.Messages)

	// Now AppendAndTrimMessages adds user + assistant.
	incoming := []types.Message{
		{Role: types.MessageRoleUser, Content: "Q2"},
		{Role: types.MessageRoleAssistant, Content: "A2"},
	}
	if err := engine.AppendAndTrimMessages(sid, incoming); err != nil {
		t.Fatalf("AppendAndTrimMessages: %v", err)
	}

	postSC, _ := engine.SessionContext(sid)
	if got := len(postSC.Messages); got != preLen+2 {
		t.Errorf("Messages len: got %d, want %d", got, preLen+2)
	}
	if postSC.Messages[preLen].Role != types.MessageRoleUser || postSC.Messages[preLen].Content != "Q2" {
		t.Errorf("appended user msg: got %+v", postSC.Messages[preLen])
	}
	if postSC.Messages[preLen+1].Role != types.MessageRoleAssistant || postSC.Messages[preLen+1].Content != "A2" {
		t.Errorf("appended assistant msg: got %+v", postSC.Messages[preLen+1])
	}
}

// T: D2-S3-A02-T02 (DM-20260617-003 lazy-init)
func TestAppendAndTrimMessages_FreshSession(t *testing.T) {
	engine := newTestEngine(t)
	sid := "sess-fresh-1"

	if _, ok := engine.SessionContext(sid); ok {
		t.Fatal("precondition: session should not exist")
	}

	incoming := []types.Message{
		{Role: types.MessageRoleUser, Content: "first user"},
		{Role: types.MessageRoleAssistant, Content: "first assistant"},
	}
	if err := engine.AppendAndTrimMessages(sid, incoming); err != nil {
		t.Fatalf("AppendAndTrimMessages: %v", err)
	}

	sc, ok := engine.SessionContext(sid)
	if !ok {
		t.Fatal("lazy-init: session not created")
	}
	if sc.SessionID != sid {
		t.Errorf("SessionID: got %s, want %s", sc.SessionID, sid)
	}
	if len(sc.Messages) != 2 {
		t.Errorf("Messages len: got %d, want 2", len(sc.Messages))
	}
}

// T: D2-S3-A02-T01
func TestAppendAndTrimMessages_TrimTriggered(t *testing.T) {
	engine := newTestEngine(t)
	sid := "sess-trim"

	// Pre-seed a fresh sc with N messages just under the trim boundary (50).
	if err := engine.AppendAndTrimMessages(sid, makeMsgs(60)); err != nil {
		t.Fatalf("seed Append: %v", err)
	}
	sc, _ := engine.SessionContext(sid)
	if len(sc.Messages) > 50 {
		t.Fatalf("trim did not trigger: len=%d", len(sc.Messages))
	}
}

func makeMsgs(n int) []types.Message {
	out := make([]types.Message, n)
	for i := range out {
		out[i] = types.Message{
			Role:    types.MessageRoleUser,
			Content: "msg-content-filler",
		}
	}
	return out
}

// T: D2-S3-A02-T01 (race-safety: per-session turn-serial pattern)
// In production, AppendAndTrimMessages is called per-session from D7 turn
// orchestrator (one turn at a time per session). The race-safety test
// validates that 100 distinct sessions can be persisted concurrently
// without races or panics.
func TestAppendAndTrimMessages_RaceSafety(t *testing.T) {
	engine := newTestEngine(t)

	const goroutines = 100
	const perG = 5

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			sid := "sess-race-" + string(rune('a'+i%26)) + "-" + string(rune('A'+(i/26)%26))
			batch := make([]types.Message, perG)
			for j := range batch {
				batch[j] = types.Message{
					Role:    types.MessageRoleUser,
					Content: "race-content",
				}
			}
			if err := engine.AppendAndTrimMessages(sid, batch); err != nil {
				t.Errorf("concurrent AppendAndTrimMessages: %v", err)
			}
		}()
	}
	wg.Wait()
}
