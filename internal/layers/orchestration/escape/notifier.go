// Notifier + FeishuCardNotifier + ChainedNotifier (DM-20260625-003, PR-V5.3)
//
// 关键设计 (doc 38 §21.3.4, design.md §5.3.1):
//   - Notifier interface 可插拔, dev 默认 FeishuCardNotifier
//   - ChainedNotifier 链式 fallback: Feishu → CLI → Email
//   - OverrideCardNotifier 是可选 interface (用于 ctx.Done() 兜底覆盖卡片)
//   - 同步发送 + 立即返回 (HumanArbitrator goroutine 兜底 timeout)
package escape

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// UserChoice is the human-arbitrator input. Mapped from UI buttons A/B/C
// to EscapeContinue / EscapeForceExit / EscapeAbortWithAudit.
type UserChoice struct {
	// Value is the button label: "A" (continue), "B" (accept), "C" (cancel).
	Value string

	// PendingID is the decision identifier the choice applies to.
	PendingID string

	// Timestamp is when the user made the choice (audit trail).
	Timestamp time.Time
}

// EscapeDecisionNotifier is the minimal interface a Notifier needs to
// construct the user-facing message.
//
// The interface is kept narrow (just the decision + context) so
// implementations don't accidentally depend on internal escape state.
// LoopContext is passed by value (small struct) — the receiver only
// reads SessionID / PlanKind for templating.
type EscapeDecisionNotifier interface {
	// Notify sends a user-facing prompt requesting a decision.
	// Returns error if the notification failed (caller may fall back
	// to the next Notifier in a chain).
	Notify(ctx context.Context, loopCtx LoopContext, pendingID string, decisions []EscapeDecision) error
}

// OverrideCardNotifier is an OPTIONAL interface a Notifier may satisfy.
// Used by HumanArbitrator's ctx.Done() path to overwrite the user-visible
// card (so users don't see "waiting for input" while the system has
// already ForceExit'd).
//
// Implemented separately from EscapeDecisionNotifier so non-overridable
// Notifiers (e.g. EmailNotifier) don't have to provide a no-op.
type OverrideCardNotifier interface {
	// SubmitOverrideCard replaces the existing user-facing card with msg.
	// buttons may be empty (e.g. for "已强制退出" notifications).
	SubmitOverrideCard(ctx context.Context, pendingID string, msg string, buttons []UserChoiceButton) error
}

// UserChoiceButton represents a single button on a user-facing card.
// The Value maps to UserChoice.Value (A/B/C).
type UserChoiceButton struct {
	Label     string // Display text (e.g. "A. 继续尝试")
	Value     string // Submitted value (e.g. "A")
	PendingID string // Bound to the originating pending decision
}

// HumanReviewCard is the user-facing card payload. Concrete Notifier
// implementations decide how to render it (Feishu card / CLI prompt /
// email subject+body).
type HumanReviewCard struct {
	Title     string
	Body      string
	Buttons   []UserChoiceButton
	ExpiresAt time.Time
}

// FeishuCardNotifier is the dev default: posts a Feishu interactive card
// with 3 buttons (A/B/C) and a 10s expiry. The actual Feishu API call
// is delegated to FeishuCardClient (injected at construction time).
//
// In tests, FeishuCardClient is a mock that records calls without making
// real HTTP requests.
type FeishuCardNotifier struct {
	cardClient FeishuCardClient
	userID     string
}

// FeishuCardClient is the minimal HTTP client surface the escape package
// needs. Production wires this to the D1 communication layer's Feishu
// adapter; tests inject a mock.
type FeishuCardClient interface {
	SendCard(ctx context.Context, userID string, card HumanReviewCard) error
	UpdateCard(ctx context.Context, userID string, card HumanReviewCard) error
}

// NewFeishuCardNotifier constructs a Feishu-backed notifier.
func NewFeishuCardNotifier(client FeishuCardClient, userID string) *FeishuCardNotifier {
	return &FeishuCardNotifier{
		cardClient: client,
		userID:     userID,
	}
}

// Notify builds a 3-button A/B/C card and sends it to the user.
func (n *FeishuCardNotifier) Notify(ctx context.Context, loopCtx LoopContext, pendingID string, decisions []EscapeDecision) error {
	card := n.buildCard(loopCtx, pendingID, decisions)
	return n.cardClient.SendCard(ctx, n.userID, card)
}

// SubmitOverrideCard updates the existing card (used by ctx.Done()
// fallback). Implements OverrideCardNotifier.
func (n *FeishuCardNotifier) SubmitOverrideCard(ctx context.Context, pendingID string, msg string, buttons []UserChoiceButton) error {
	card := HumanReviewCard{
		Title:   "⚠️ " + msg,
		Body:    "本轮已自动终止, 无需操作",
		Buttons: buttons,
	}
	return n.cardClient.UpdateCard(ctx, n.userID, card)
}

// buildCard assembles the HumanReviewCard payload.
func (n *FeishuCardNotifier) buildCard(loopCtx LoopContext, pendingID string, decisions []EscapeDecision) HumanReviewCard {
	body := buildHumanReviewBody(decisions)
	return HumanReviewCard{
		Title: "🔧 回路需要人工接管",
		Body:  body,
		Buttons: []UserChoiceButton{
			{Label: "A. 继续尝试", Value: "A", PendingID: pendingID},
			{Label: "B. 接受当前", Value: "B", PendingID: pendingID},
			{Label: "C. 取消", Value: "C", PendingID: pendingID},
		},
		ExpiresAt: time.Now().Add(10 * time.Second),
	}
}

// buildHumanReviewBody summarizes the decision chain leading to human arbitration.
func buildHumanReviewBody(decisions []EscapeDecision) string {
	if len(decisions) == 0 {
		return "回路深度已超过上限, 需要人工决策"
	}
	last := decisions[len(decisions)-1]
	return fmt.Sprintf("回路深度已超过上限 (%s)\n最近决策: %s", last.Reason, last.Action)
}

// ChainedNotifier is the fallback decorator: tries each Notifier in order,
// returns nil on the first success, otherwise the last error.
//
// Order matters: most-preferred (FeishuCard) → least-preferred (Email).
type ChainedNotifier struct {
	notifiers []EscapeDecisionNotifier
}

// NewChainedNotifier constructs a fallback chain. Empty chain panics at
// construction (caller bug).
func NewChainedNotifier(notifiers ...EscapeDecisionNotifier) *ChainedNotifier {
	if len(notifiers) == 0 {
		panic("escape: ChainedNotifier requires at least one Notifier")
	}
	return &ChainedNotifier{notifiers: notifiers}
}

// Notify tries each Notifier in order; returns nil on first success.
func (c *ChainedNotifier) Notify(ctx context.Context, loopCtx LoopContext, pendingID string, decisions []EscapeDecision) error {
	var lastErr error
	for i, n := range c.notifiers {
		if err := n.Notify(ctx, loopCtx, pendingID, decisions); err == nil {
			return nil
		} else {
			slog.Warn("notifier_fallback",
				"index", i,
				"notifier_type", fmt.Sprintf("%T", n),
				"pending_id", pendingID,
				"error", err.Error(),
			)
			lastErr = err
		}
	}
	return fmt.Errorf("chained_notifier_all_failed: %w", lastErr)
}

// SubmitOverrideCard iterates and tries each Notifier that supports
// OverrideCardNotifier. Notifiers without the optional interface are
// skipped with a slog.Warn (so the fallback chain still works without
// all members being overridable).
func (c *ChainedNotifier) SubmitOverrideCard(ctx context.Context, pendingID string, msg string, buttons []UserChoiceButton) error {
	var lastErr error
	for i, n := range c.notifiers {
		override, ok := n.(OverrideCardNotifier)
		if !ok {
			slog.Warn("notifier_does_not_support_override",
				"index", i,
				"notifier_type", fmt.Sprintf("%T", n),
				"pending_id", pendingID,
			)
			continue
		}
		if err := override.SubmitOverrideCard(ctx, pendingID, msg, buttons); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		return fmt.Errorf("chained_notifier_no_override_support")
	}
	return fmt.Errorf("chained_notifier_override_all_failed: %w", lastErr)
}