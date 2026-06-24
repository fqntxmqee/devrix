package escape

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// mockFeishuClient is a test double for FeishuCardClient.
type mockFeishuClient struct {
	mu         sync.Mutex
	sentCards  []HumanReviewCard
	updateCards []HumanReviewCard
	sendErr    error
	updateErr  error
}

func (m *mockFeishuClient) SendCard(ctx context.Context, userID string, card HumanReviewCard) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sentCards = append(m.sentCards, card)
	return nil
}

func (m *mockFeishuClient) UpdateCard(ctx context.Context, userID string, card HumanReviewCard) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updateCards = append(m.updateCards, card)
	return nil
}

// --- TestFeishuCardNotifier_BuildCard ----------------------------------------

func TestFeishuCardNotifier_BuildCard(t *testing.T) {
	client := &mockFeishuClient{}
	n := NewFeishuCardNotifier(client, "user-123")

	loopCtx := LoopContext{
		SessionID: "sess-abc",
		PlanKind:  plan.ExplorationPlan,
	}
	decisions := []EscapeDecision{
		{Action: EscapeContinue, Reason: "depth=2 under max"},
		{Action: EscapeForceExit, Reason: "loop_depth_exceeded"},
	}

	err := n.Notify(context.Background(), loopCtx, "pending-xyz", decisions)
	if err != nil {
		t.Fatalf("Notify failed: %v", err)
	}

	if len(client.sentCards) != 1 {
		t.Fatalf("sent %d cards, want 1", len(client.sentCards))
	}
	card := client.sentCards[0]
	if !strings.Contains(card.Title, "回路") {
		t.Errorf("Title=%q, want contains 回路", card.Title)
	}
	if len(card.Buttons) != 3 {
		t.Errorf("got %d buttons, want 3 (A/B/C)", len(card.Buttons))
	}
	if card.Buttons[0].Value != "A" || card.Buttons[1].Value != "B" || card.Buttons[2].Value != "C" {
		t.Errorf("button values = %v, want [A, B, C]", card.Buttons)
	}
	if card.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set")
	}
	if card.ExpiresAt.Before(nowFunc()) {
		t.Error("ExpiresAt should be in the future")
	}
}

// --- TestFeishuCardNotifier_SubmitOverrideCard -------------------------------

func TestFeishuCardNotifier_SubmitOverrideCard(t *testing.T) {
	client := &mockFeishuClient{}
	n := NewFeishuCardNotifier(client, "user-123")

	err := n.SubmitOverrideCard(context.Background(), "pending-xyz", "已强制退出", nil)
	if err != nil {
		t.Fatalf("SubmitOverrideCard failed: %v", err)
	}

	if len(client.updateCards) != 1 {
		t.Fatalf("updated %d cards, want 1", len(client.updateCards))
	}
	card := client.updateCards[0]
	if !strings.Contains(card.Title, "已强制退出") {
		t.Errorf("override Title=%q, want contains 已强制退出", card.Title)
	}
}

// --- TestChainedNotifier_FeishuSuccess ---------------------------------------

func TestChainedNotifier_FeishuSuccess(t *testing.T) {
	client := &mockFeishuClient{}
	feishu := NewFeishuCardNotifier(client, "user-1")

	chain := NewChainedNotifier(feishu)
	err := chain.Notify(context.Background(), LoopContext{}, "p1", nil)
	if err != nil {
		t.Fatalf("chain.Notify failed: %v", err)
	}
	if len(client.sentCards) != 1 {
		t.Errorf("sent %d cards, want 1", len(client.sentCards))
	}
}

// --- TestChainedNotifier_FeishuFail_CLISuccess -------------------------------

func TestChainedNotifier_FeishuFail_CLISuccess(t *testing.T) {
	failingClient := &mockFeishuClient{sendErr: errors.New("feishu timeout")}
	feishu := NewFeishuCardNotifier(failingClient, "user-1")

	cli := &mockCLINotifier{}
	chain := NewChainedNotifier(feishu, cli)

	err := chain.Notify(context.Background(), LoopContext{}, "p1", nil)
	if err != nil {
		t.Fatalf("chain.Notify failed (CLI should fallback): %v", err)
	}
	if len(failingClient.sentCards) != 0 {
		t.Errorf("feishu should not record card on failure")
	}
	if cli.calls != 1 {
		t.Errorf("CLI should be called once, got %d", cli.calls)
	}
}

// --- TestChainedNotifier_AllFail ---------------------------------------------

func TestChainedNotifier_AllFail(t *testing.T) {
	failingClient := &mockFeishuClient{sendErr: errors.New("feishu down")}
	feishu := NewFeishuCardNotifier(failingClient, "user-1")

	cli := &mockCLINotifier{failErr: errors.New("cli failed")}
	chain := NewChainedNotifier(feishu, cli)

	err := chain.Notify(context.Background(), LoopContext{}, "p1", nil)
	if err == nil {
		t.Fatal("all-fail should return error")
	}
	if !strings.Contains(err.Error(), "cli failed") {
		t.Errorf("error should include CLI failure: %v", err)
	}
}

// --- TestChainedNotifier_EmptyChain_Panics -----------------------------------

func TestChainedNotifier_EmptyChain_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("empty chain should panic")
		}
	}()
	_ = NewChainedNotifier()
}

// --- TestChainedNotifier_OverrideCard ---------------------------------------

func TestChainedNotifier_OverrideCard(t *testing.T) {
	client := &mockFeishuClient{}
	feishu := NewFeishuCardNotifier(client, "user-1")
	cli := &mockCLINotifier{}

	chain := NewChainedNotifier(feishu, cli)
	err := chain.SubmitOverrideCard(context.Background(), "p1", "已退出", nil)
	if err != nil {
		t.Fatalf("SubmitOverrideCard failed: %v", err)
	}
	if len(client.updateCards) != 1 {
		t.Errorf("feishu should be called for override, got %d", len(client.updateCards))
	}
}

// --- TestChainedNotifier_OverrideCard_NoSupport -----------------------------

// When no Notifier supports OverrideCard, return an error.
func TestChainedNotifier_OverrideCard_NoSupport(t *testing.T) {
	cli := &mockCLINotifier{} // doesn't implement OverrideCardNotifier
	chain := NewChainedNotifier(cli)

	err := chain.SubmitOverrideCard(context.Background(), "p1", "test", nil)
	if err == nil {
		t.Error("expected error when no notifier supports override")
	}
}

// --- mockCLINotifier for testing fallback chain ------------------------------

type mockCLINotifier struct {
	mu      sync.Mutex
	calls   int
	failErr error
}

func (m *mockCLINotifier) Notify(ctx context.Context, loopCtx LoopContext, pendingID string, decisions []EscapeDecision) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.failErr
}

// --- helpers ------------------------------------------------------------------

var _ = time.Second // suppress unused import in some build configs