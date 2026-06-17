package bootstrap

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubSessionEngine struct {
	sc *types.SessionContext
}

func (s *stubSessionEngine) Process(context.Context, *types.Session, string) <-chan *contracts.EngineEvent {
	ch := make(chan *contracts.EngineEvent)
	close(ch)
	return ch
}

func (s *stubSessionEngine) SessionContext(sessionID string) (*types.SessionContext, bool) {
	if s.sc != nil && s.sc.SessionID == sessionID {
		return s.sc, true
	}
	return nil, false
}

// T: D7-S2-A06 — Prepare returns transcript history only; TurnOrchestrator appends the current user turn.
func TestContextEngineAdapter_Prepare_excludes_current_user_message(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)
	sess := types.NewSession("sess-turn", "test", "/tmp")
	if err := store.Create(sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	history := []types.Message{
		{Role: types.MessageRoleUser, Content: "上一轮问题", SessionID: "sess-turn"},
		{Role: types.MessageRoleAssistant, Content: "上一轮回答", SessionID: "sess-turn"},
	}
	engine := &stubSessionEngine{
		sc: &types.SessionContext{
			SessionID: "sess-turn",
			Messages:  history,
		},
	}
	adapter := newContextEngineAdapter(gw, engine, nil)

	current := types.Message{
		Role:      types.MessageRoleUser,
		Content:   "d5和d6重构需求应该交付了，请结合代码判断一下",
		SessionID: "sess-turn",
	}
	prepared, err := adapter.Prepare(context.Background(), turn.PrepareRequest{
		SessionID: "sess-turn",
		Message:   current,
	})
	if err != nil {
		t.Fatalf("Prepare err: %v", err)
	}
	if len(prepared.Messages) != len(history) {
		t.Fatalf("Prepare messages len = %d, want %d (history only)", len(prepared.Messages), len(history))
	}
	for i, want := range history {
		if prepared.Messages[i].Content != want.Content || prepared.Messages[i].Role != want.Role {
			t.Fatalf("history[%d] = %+v, want %+v", i, prepared.Messages[i], want)
		}
	}
	for _, m := range prepared.Messages {
		if m.Role == types.MessageRoleUser && m.Content == current.Content {
			t.Fatalf("Prepare must not include current user message: %+v", prepared.Messages)
		}
	}
}
