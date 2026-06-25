package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
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

// T: D7-S2-A06 — Prepare returns transcript history only; TurnOrchestrator appends the current user sessionorchestrator.
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
	prepared, err := adapter.Prepare(context.Background(), sessionorchestrator.PrepareRequest{
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

// stubTokenCounter returns len(text)/4 as a deterministic char/4 token estimate.
type stubTokenCounter struct{}

func (stubTokenCounter) CountText(text string) int    { return len(text) / 4 }
func (stubTokenCounter) CountMessages(msgs []types.Message) int {
	n := 0
	for _, m := range msgs {
		n += len(m.Content) / 4
	}
	return n
}
func (stubTokenCounter) CountWithSystemPrompt(sp string, msgs []types.Message) int {
	return len(sp)/4 + stubTokenCounter{}.CountMessages(msgs)
}
func (stubTokenCounter) TruncateToTokens(text string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(text) <= max*4 {
		return text
	}
	return text[:max*4]
}
func (stubTokenCounter) EncodingForModel(string) string { return "stub" }

// T: DM-20260621-009 — Multi-turn history must NOT be replaced by a CompressHint
// summary when the cumulative context is comfortably under the budget. Bug:
// turn 3 of a multi-turn conversation was getting `prepared.Messages =
// [summary, user]` instead of the full prior history, because the prior
// 4000-token threshold fired prematurely. The user's "2" (a reply to a
// choice prompt in turn 2) lost all context, so the LLM could not match
// it back to the prior question.
func TestContextEngineAdapter_Prepare_noCompressHint_underThreshold(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)
	sess := types.NewSession("sess-multiturn", "test", "/tmp")
	if err := store.Create(sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Build a realistic 3-turn prior history: ~22000 chars / ~5500 tokens.
	// At the old 4000-token threshold this fired CompressHint, replacing the
	// full history with a single summary, which made turn 3's short reply
	// ("2") lose its context. DM-20260621-009 raises the threshold to
	// 32K so this size stays un-compressed.
	var history []types.Message
	turn1Assistant := strings.Repeat("A", 3000) // user + assistant text
	turn1Tool := strings.Repeat("B", 1500)      // tool result
	turn2Assistant := strings.Repeat("C", 12000) // turn 2 markdown report
	turn2Tool := strings.Repeat("D", 5500)      // tool result
	history = append(history,
		types.Message{Role: types.MessageRoleUser, Content: "看一眼 devrix repo 的当前 git status", SessionID: "sess-multiturn"},
		types.Message{Role: types.MessageRoleAssistant, Content: turn1Assistant, SessionID: "sess-multiturn"},
		types.Message{Role: types.MessageRoleTool, Content: turn1Tool, SessionID: "sess-multiturn"},
		types.Message{Role: types.MessageRoleUser, Content: "看看changes目录", SessionID: "sess-multiturn"},
		types.Message{Role: types.MessageRoleAssistant, Content: turn2Assistant, SessionID: "sess-multiturn"},
		types.Message{Role: types.MessageRoleTool, Content: turn2Tool, SessionID: "sess-multiturn"},
	)
	engine := &stubSessionEngine{
		sc: &types.SessionContext{
			SessionID: "sess-multiturn",
			Messages:  history,
		},
	}
	adapter := newContextEngineAdapter(gw, engine, stubTokenCounter{})

	prepared, err := adapter.Prepare(context.Background(), sessionorchestrator.PrepareRequest{
		SessionID: "sess-multiturn",
		Message:   types.Message{Role: types.MessageRoleUser, Content: "2", SessionID: "sess-multiturn"},
	})
	if err != nil {
		t.Fatalf("Prepare err: %v", err)
	}

	// DM-20260621-009 regression: a 3-turn session that fits comfortably
	// under 32K tokens must NOT trigger CompressHint (i.e. prepared.Messages
	// must still equal the full prior history, not a single summary).
	if prepared.CompressHint != nil {
		t.Fatalf("regression: CompressHint fired at %d history messages (<32K threshold); turn 3 will lose context. prepared.CompressHint=%+v", len(history), prepared.CompressHint)
	}
	if len(prepared.Messages) != len(history) {
		t.Fatalf("regression: prepared.Messages len=%d, want %d (full history). CompressHint would have replaced this with a single summary.", len(prepared.Messages), len(history))
	}
}
