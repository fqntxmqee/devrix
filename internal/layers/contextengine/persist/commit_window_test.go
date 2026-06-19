package persist_test

import (
	"context"
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubCommitWindowStore struct {
	setActiveCalled bool
	setActiveMsgs   []types.Message
	trimCalled      bool
}

func (s *stubCommitWindowStore) SetActiveMessages(_ *types.SessionContext, msgs []types.Message) {
	s.setActiveCalled = true
	s.setActiveMsgs = msgs
}

func (s *stubCommitWindowStore) TrimMessages(_ *types.SessionContext) {
	s.trimCalled = true
}

type stubCommitWindowPipeline struct {
	compressed []types.Message
	report     types.CompressionReport
	err        error
}

func (s *stubCommitWindowPipeline) Run(_ context.Context, _ []types.Message, _ string, _ types.TokenBudget) ([]types.Message, types.CompressionReport, error) {
	return s.compressed, s.report, s.err
}

// T: D2-S17-A04-T50
func TestRunCommitWindow_skips_when_within_budget(t *testing.T) {
	store := &stubCommitWindowStore{}
	pipeline := &stubCommitWindowPipeline{}
	sc := &types.SessionContext{
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: "hi"},
		},
		TokenBudget: types.TokenBudget{MaxContextTokens: 100000},
	}
	_, err := persist.RunCommitWindow(context.Background(), persist.CommitWindowDeps{
		Store:       store,
		Pipeline:    pipeline,
		MaxMessages: 50,
	}, sc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.setActiveCalled {
		t.Error("SetActiveMessages should not be called when within budget")
	}
	if store.trimCalled {
		t.Error("TrimMessages should not be called when within budget")
	}
}

// T: D2-S17-A04-T51
func TestRunCommitWindow_trims_when_over_max_messages(t *testing.T) {
	store := &stubCommitWindowStore{}
	pipeline := &stubCommitWindowPipeline{
		compressed: []types.Message{{Role: types.MessageRoleUser, Content: "kept"}},
		report:     types.CompressionReport{StepsApplied: []string{"snip"}},
	}
	msgs := make([]types.Message, 60)
	for i := range msgs {
		msgs[i] = types.Message{Role: types.MessageRoleUser, Content: "x"}
	}
	sc := &types.SessionContext{
		Messages:    msgs,
		TokenBudget: types.TokenBudget{MaxContextTokens: 100000},
	}
	_, err := persist.RunCommitWindow(context.Background(), persist.CommitWindowDeps{
		Store:       store,
		Pipeline:    pipeline,
		MaxMessages: 50,
	}, sc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.setActiveCalled {
		t.Error("expected SetActiveMessages to be called when over MaxMessages")
	}
	if !store.trimCalled {
		t.Error("expected TrimMessages to be called when over MaxMessages")
	}
	if len(store.setActiveMsgs) != 1 {
		t.Errorf("expected 1 compressed message, got %d", len(store.setActiveMsgs))
	}
}

// T: D2-S17-A04-T52
func TestRunCommitWindow_strips_leading_system_message(t *testing.T) {
	store := &stubCommitWindowStore{}
	pipeline := &stubCommitWindowPipeline{
		compressed: []types.Message{
			{Role: types.MessageRoleSystem, Content: "sys"},
			{Role: types.MessageRoleUser, Content: "user"},
		},
		report: types.CompressionReport{StepsApplied: []string{"snip"}},
	}
	msgs := make([]types.Message, 60)
	for i := range msgs {
		msgs[i] = types.Message{Role: types.MessageRoleUser, Content: "x"}
	}
	sc := &types.SessionContext{
		Messages:    msgs,
		TokenBudget: types.TokenBudget{MaxContextTokens: 100000},
	}
	_, err := persist.RunCommitWindow(context.Background(), persist.CommitWindowDeps{
		Store:       store,
		Pipeline:    pipeline,
		MaxMessages: 50,
	}, sc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.setActiveMsgs) != 1 || store.setActiveMsgs[0].Role != types.MessageRoleUser {
		t.Errorf("expected leading system stripped; got %d msgs, first role=%s",
			len(store.setActiveMsgs), store.setActiveMsgs[0].Role)
	}
}

// T: D2-S17-A04-T53
func TestRunCommitWindow_returns_error_when_pipeline_fails(t *testing.T) {
	store := &stubCommitWindowStore{}
	pipeline := &stubCommitWindowPipeline{
		err: errors.New("compression pipeline crashed"),
	}
	msgs := make([]types.Message, 60)
	for i := range msgs {
		msgs[i] = types.Message{Role: types.MessageRoleUser, Content: "x"}
	}
	sc := &types.SessionContext{Messages: msgs, TokenBudget: types.TokenBudget{}}
	_, err := persist.RunCommitWindow(context.Background(), persist.CommitWindowDeps{
		Store:       store,
		Pipeline:    pipeline,
		MaxMessages: 50,
	}, sc)
	if err == nil {
		t.Fatal("expected pipeline error to propagate")
	}
	if store.setActiveCalled {
		t.Error("SetActiveMessages must NOT be called when pipeline fails")
	}
}

// T: D2-S17-A04-T54
func TestRunCommitWindow_skips_write_when_no_steps_applied(t *testing.T) {
	store := &stubCommitWindowStore{}
	pipeline := &stubCommitWindowPipeline{
		compressed: []types.Message{{Role: types.MessageRoleUser, Content: "kept"}},
		report:     types.CompressionReport{StepsApplied: nil}, // no compression actually applied
	}
	msgs := make([]types.Message, 60)
	for i := range msgs {
		msgs[i] = types.Message{Role: types.MessageRoleUser, Content: "x"}
	}
	sc := &types.SessionContext{Messages: msgs, TokenBudget: types.TokenBudget{}}
	_, err := persist.RunCommitWindow(context.Background(), persist.CommitWindowDeps{
		Store:       store,
		Pipeline:    pipeline,
		MaxMessages: 50,
	}, sc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.setActiveCalled {
		t.Error("SetActiveMessages should not be called when no steps applied")
	}
}

// T: D2-S17-A04-T55
func TestRunCommitWindow_returns_error_for_nil_inputs(t *testing.T) {
	_, err := persist.RunCommitWindow(context.Background(), persist.CommitWindowDeps{}, nil)
	if err == nil {
		t.Fatal("expected error for nil session context")
	}
}

// T: D2-S17-A04-T56
func TestRunCommitWindow_uses_shouldCompress_predicate(t *testing.T) {
	store := &stubCommitWindowStore{}
	pipeline := &stubCommitWindowPipeline{
		compressed: []types.Message{{Role: types.MessageRoleUser, Content: "kept"}},
		report:     types.CompressionReport{StepsApplied: []string{"snip"}},
	}
	sc := &types.SessionContext{
		Messages:    []types.Message{{Role: types.MessageRoleUser, Content: "x"}},
		TokenBudget: types.TokenBudget{MaxContextTokens: 100000},
	}
	called := false
	_, err := persist.RunCommitWindow(context.Background(), persist.CommitWindowDeps{
		Store:       store,
		Pipeline:    pipeline,
		MaxMessages: 50,
		ShouldCompress: func(msgs []types.Message, _ types.TokenBudget) bool {
			called = true
			return true
		},
	}, sc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected ShouldCompress predicate to be called")
	}
	if !store.setActiveCalled {
		t.Error("expected SetActiveMessages to be called when ShouldCompress returns true")
	}
}
