package persist_test

import (
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
