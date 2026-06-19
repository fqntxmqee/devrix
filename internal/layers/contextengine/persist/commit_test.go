package persist_test

import (
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubStore struct {
	sessions map[string]*types.SessionContext
}

func (s *stubStore) Get(sessionID string) (*types.SessionContext, bool) {
	sc, ok := s.sessions[sessionID]
	return sc, ok
}

func (s *stubStore) AppendFullMessage(sc *types.SessionContext, msg types.Message) {
	sc.Messages = append(sc.Messages, msg)
}

func (s *stubStore) TrimMessages(sc *types.SessionContext) {
	if len(sc.Messages) > 2 {
		sc.Messages = sc.Messages[len(sc.Messages)-2:]
	}
}

// threadSafeStubStore is a stubStore variant with a mutex for race-safety tests.
// Required because AppendAndTrimMessages runs without per-session locking;
// cross-session concurrency is guarded by the Store contract.
type threadSafeStubStore struct {
	mu       sync.Mutex
	sessions map[string]*types.SessionContext
}

func (s *threadSafeStubStore) Get(sessionID string) (*types.SessionContext, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc, ok := s.sessions[sessionID]
	return sc, ok
}

func (s *threadSafeStubStore) AppendFullMessage(sc *types.SessionContext, msg types.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc.Messages = append(sc.Messages, msg)
}

func (s *threadSafeStubStore) TrimMessages(sc *types.SessionContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(sc.Messages) > 2 {
		sc.Messages = sc.Messages[len(sc.Messages)-2:]
	}
}

func TestAppendAndTrimMessages_empty(t *testing.T) {
	store := &stubStore{sessions: map[string]*types.SessionContext{}}
	if err := persist.AppendAndTrimMessages(persist.CommitDeps{Store: store}, "s1", nil); err != nil {
		t.Fatalf("empty: %v", err)
	}
}

func TestAppendAndTrimMessages_existingSession(t *testing.T) {
	store := &stubStore{sessions: map[string]*types.SessionContext{
		"s1": {SessionID: "s1", Messages: []types.Message{{Role: types.MessageRoleUser, Content: "hi"}}},
	}}
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "q2"},
		{Role: types.MessageRoleAssistant, Content: "a2"},
	}
	if err := persist.AppendAndTrimMessages(persist.CommitDeps{Store: store}, "s1", msgs); err != nil {
		t.Fatalf("append: %v", err)
	}
	if got := len(store.sessions["s1"].Messages); got != 2 {
		t.Errorf("after trim: got %d messages, want 2", got)
	}
}

func TestAppendAndTrimMessages_bootstrap(t *testing.T) {
	store := &stubStore{sessions: map[string]*types.SessionContext{}}
	booted := false
	err := persist.AppendAndTrimMessages(persist.CommitDeps{
		Store: store,
		Bootstrap: func(sessionID string) (*types.SessionContext, error) {
			booted = true
			sc := &types.SessionContext{SessionID: sessionID}
			store.sessions[sessionID] = sc
			return sc, nil
		},
	}, "fresh", []types.Message{{Role: types.MessageRoleUser, Content: "first"}})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if !booted {
		t.Fatal("expected bootstrap to run")
	}
}

func TestAppendAndTrimMessages_noBootstrapMissingSession(t *testing.T) {
	store := &stubStore{sessions: map[string]*types.SessionContext{}}
	err := persist.AppendAndTrimMessages(persist.CommitDeps{Store: store}, "missing", []types.Message{
		{Role: types.MessageRoleUser, Content: "x"},
	})
	if err == nil {
		t.Fatal("expected error when session missing and no bootstrap")
	}
}

// TestAppendAndTrimMessages_DedupByID reproduces the context-bleed pattern
// observed 2026-06-20: the D7 turn orchestrator passes the full history
// slice [prepared.Messages + req.UserMessage] to PersistTurn, where
// prepared.Messages is sourced from sc.Messages. Without ID-based dedup,
// every prior message would be appended a second time each turn (2^N
// growth). After the fix, the existing IDs are detected and skipped; only
// genuinely-new messages land in sc.Messages.
func TestAppendAndTrimMessages_DedupByID(t *testing.T) {
	store := &stubStore{sessions: map[string]*types.SessionContext{
		"s1": {
			SessionID: "s1",
			Messages: []types.Message{
				{ID: "m1", Role: types.MessageRoleUser, Content: "你好"},
				{ID: "m2", Role: types.MessageRoleAssistant, Content: "hi"},
				{ID: "m3", Role: types.MessageRoleUser, Content: "请尝试多轮"},
				{ID: "m4", Role: types.MessageRoleAssistant, Content: "ok"},
			},
		},
	}}
	// D7 caller mistakenly re-passes the whole history plus the new turn.
	msgs := []types.Message{
		{ID: "m1", Role: types.MessageRoleUser, Content: "你好"},
		{ID: "m2", Role: types.MessageRoleAssistant, Content: "hi"},
		{ID: "m3", Role: types.MessageRoleUser, Content: "请尝试多轮"},
		{ID: "m4", Role: types.MessageRoleAssistant, Content: "ok"},
		{ID: "m5", Role: types.MessageRoleUser, Content: "跑一个复杂指令"},
	}
	if err := persist.AppendAndTrimMessages(persist.CommitDeps{Store: store}, "s1", msgs); err != nil {
		t.Fatalf("append: %v", err)
	}
	// After TrimMessages (stub keeps last 2), exactly the last two distinct
	// IDs should remain: m4 + m5.
	got := store.sessions["s1"].Messages
	if len(got) != 2 {
		t.Fatalf("after dedup+trim: got %d messages, want 2: %#v", len(got), got)
	}
	if got[0].ID != "m4" || got[1].ID != "m5" {
		t.Fatalf("expected [m4, m5], got [%s, %s]", got[0].ID, got[1].ID)
	}
}

// TestAppendAndTrimMessages_NoDedupWhenIDsAbsent ensures the dedup path is
// non-destructive for messages without IDs (legacy callers / bootstrap
// paths). Such messages must still be appended, with the Store assigning IDs.
func TestAppendAndTrimMessages_NoDedupWhenIDsAbsent(t *testing.T) {
	store := &stubStore{sessions: map[string]*types.SessionContext{
		"s1": {SessionID: "s1", Messages: []types.Message{{Role: types.MessageRoleUser, Content: "old"}}},
	}}
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "new1"},
		{Role: types.MessageRoleAssistant, Content: "new2"},
	}
	if err := persist.AppendAndTrimMessages(persist.CommitDeps{Store: store}, "s1", msgs); err != nil {
		t.Fatalf("append: %v", err)
	}
	// Stub TrimMessages keeps only the last 2 entries. What matters here is
	// that both new messages landed (no ID-based drop of empty-ID msgs).
	got := store.sessions["s1"].Messages
	if len(got) != 2 {
		t.Fatalf("expected 2 messages after trim, got %d: %#v", len(got), got)
	}
	if got[0].Content != "new1" || got[1].Content != "new2" {
		t.Fatalf("unexpected contents after dedup-bypass: [%q, %q]", got[0].Content, got[1].Content)
	}
}

// T: D2-S17-A02-T01 (race-safety: per-session turn-serial pattern)
// In production, AppendAndTrimMessages is called per-session from D7 turn
// orchestrator (one turn at a time per session). This test validates that
// 100 distinct sessions can be bootstrapped and persisted concurrently
// without races or panics.
//
// Migrated from internal/layers/contextengine/engine_persist_bridge_test.go
// during the 2026-06-19 D2 root test cleanup (was routing through the legacy
// engine facade; now exercises persist.AppendAndTrimMessages directly).
func TestAppendAndTrimMessages_RaceSafety(t *testing.T) {
	store := &threadSafeStubStore{sessions: map[string]*types.SessionContext{}}
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
			err := persist.AppendAndTrimMessages(persist.CommitDeps{
				Store: store,
				Bootstrap: func(sessionID string) (*types.SessionContext, error) {
					store.mu.Lock()
					defer store.mu.Unlock()
					sc := &types.SessionContext{SessionID: sessionID}
					store.sessions[sessionID] = sc
					return sc, nil
				},
			}, sid, batch)
			if err != nil {
				t.Errorf("concurrent AppendAndTrimMessages: %v", err)
			}
		}()
	}
	wg.Wait()
}
