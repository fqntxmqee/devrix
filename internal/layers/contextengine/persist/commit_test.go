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
