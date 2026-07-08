package conclusion

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// recordingEmitter captures every OnMessage / OnStatus call so we can assert
// on the resulting OutboundMessage without coupling the test to the adapter
// layer (Feishu / CLI / capture).
type recordingEmitter struct {
	messages []*types.OutboundMessage
	statuses []types.SessionState
}

func (r *recordingEmitter) OnMessage(msg *types.OutboundMessage) {
	r.messages = append(r.messages, msg)
}

func (r *recordingEmitter) OnStatus(_ string, state types.SessionState) {
	r.statuses = append(r.statuses, state)
}

// TestEmitComplete_PrefersSummaryOverFullTranscript pins the DM-20260621-003
// fix: the IM "任务总结" card must show the LLM's BRIEF conclusion (the last
// turn's text, surfaced via D7 metadata["summary"]), not the full multi-turn
// transcript (event.Content, accumulated across turns since PR #137).
// Without this, a 99-tool-call deep review dumps 75K chars of in-flight
// tool-loop output into the summary card — content the user already saw via
// streaming text chunks.
func TestEmitComplete_PrefersSummaryOverFullTranscript(t *testing.T) {
	sess := &types.Session{SessionID: "sess_test", ChatID: "chat_test"}
	const brief = "代码审查完成，整体结构清晰，无阻塞问题。"
	const fullTranscript = brief + "\n\n[earlier turn 1]\n[earlier turn 2]\n[earlier turn 3 — tool output inline]\n[earlier turn 99 — final synthesis]"
	ev := &contracts.EngineEvent{
		Type:      "complete",
		Content:   fullTranscript,
		SessionID: "sess_test",
		Metadata: map[string]string{
			"duration": "8500",
			"usage":    "1500",
			"model":    "claude-sonnet-4-6",
			"ctx_pct":  "12",
			"summary":  brief,
		},
	}
	em := &recordingEmitter{}

	EmitComplete(sess, ev, contracts.IMOutboundSignal{}, false, em)

	if got := len(em.messages); got != 1 {
		t.Fatalf("OnMessage call count = %d, want 1", got)
	}
	msg := em.messages[0]
	if msg.Content != brief {
		t.Fatalf("Content = %q, want brief LLM conclusion %q", msg.Content, brief)
	}
	if !msg.IsComplete {
		t.Fatalf("IsComplete = false, want true")
	}
	if msg.Metadata["event_type"] != "complete" {
		t.Fatalf("metadata[event_type] = %q, want %q", msg.Metadata["event_type"], "complete")
	}
	if msg.Metadata["summary"] != brief {
		t.Fatalf("metadata[summary] = %q, want %q", msg.Metadata["summary"], brief)
	}
	wantStats := "用时: 9s, 消耗: 1500 tokens, ctx: 12%, 模型: claude-sonnet-4-6"
	if msg.Metadata["stats"] != wantStats {
		t.Fatalf("metadata[stats] = %q, want %q", msg.Metadata["stats"], wantStats)
	}
	if got := len(em.statuses); got != 1 || em.statuses[0] != types.SessionStateCompleted {
		t.Fatalf("OnStatus calls = %v, want [completed]", em.statuses)
	}
}

// TestEmitComplete_FallsBackToEventContent covers the single-turn reply
// case: D7 does not populate metadata["summary"] (lastTurnText is identical
// to Content, and the brief path is a no-op for single-turn loops), so
// EmitComplete must use event.Content directly. This preserves backward
// compatibility for the common case where the LLM emits a single reply
// with no tool calls.
func TestEmitComplete_FallsBackToEventContent(t *testing.T) {
	sess := &types.Session{SessionID: "sess_test", ChatID: "chat_test"}
	ev := &contracts.EngineEvent{
		Type:      "complete",
		Content:   "单轮回复，无工具调用。",
		SessionID: "sess_test",
		Metadata: map[string]string{
			"duration": "3000",
			"usage":    "200",
			"model":    "claude-sonnet-4-6",
		},
	}
	em := &recordingEmitter{}

	EmitComplete(sess, ev, contracts.IMOutboundSignal{}, false, em)

	msg := em.messages[0]
	if msg.Content != "单轮回复，无工具调用。" {
		t.Fatalf("Content = %q, want event.Content fallback", msg.Content)
	}
	if _, ok := msg.Metadata["summary"]; ok {
		t.Fatalf("metadata[summary] should NOT be set when D7 didn't populate it, got %q", msg.Metadata["summary"])
	}
}

// TestEmitComplete_EmptySummaryAndContent_FallsBackToStats ensures the
// regression guard: if both summary and Content are empty (e.g. provider
// that emitted no text at all), the message body must still carry the
// stats string so the user sees a non-empty "任务总结" card.
func TestEmitComplete_EmptySummaryAndContent_FallsBackToStats(t *testing.T) {
	sess := &types.Session{SessionID: "sess_test", ChatID: "chat_test"}
	ev := &contracts.EngineEvent{
		Type:      "complete",
		Content:   "   \n  ",
		SessionID: "sess_test",
		Metadata: map[string]string{
			"duration": "7655",
			"usage":    "1500",
			"model":    "claude-sonnet-4-6",
			"ctx_pct":  "12",
		},
	}
	em := &recordingEmitter{}

	EmitComplete(sess, ev, contracts.IMOutboundSignal{}, false, em)

	if got := len(em.messages); got != 1 {
		t.Fatalf("OnMessage call count = %d, want 1", got)
	}
	msg := em.messages[0]
	wantStats := "用时: 8s, 消耗: 1500 tokens, ctx: 12%, 模型: claude-sonnet-4-6"
	if msg.Content != wantStats {
		t.Fatalf("Content = %q, want stats fallback %q", msg.Content, wantStats)
	}
	if msg.Metadata["stats"] != wantStats {
		t.Fatalf("metadata[stats] = %q, want %q", msg.Metadata["stats"], wantStats)
	}
}

// TestEmitComplete_NilGuards pins the early-return semantics so future
// refactors don't accidentally panic on partial D7 init failures.
func TestEmitComplete_NilGuards(t *testing.T) {
	em := &recordingEmitter{}
	EmitComplete(nil, &contracts.EngineEvent{Type: "complete"}, contracts.IMOutboundSignal{}, false, em)
	EmitComplete(&types.Session{}, nil, contracts.IMOutboundSignal{}, false, em)
	EmitComplete(&types.Session{}, &contracts.EngineEvent{Type: "complete"}, contracts.IMOutboundSignal{}, false, nil)
	if got := len(em.messages); got != 0 {
		t.Fatalf("OnMessage call count = %d, want 0 (all nil-guards must short-circuit)", got)
	}
}

// TestEmitComplete_SummaryQualityTooShort_FallsBackToContent —
// DM-20260630-011 AC2 regression.
//
// When D7's LastTextQualityGate classified the LLM's last-turn text as
// too_short (planning/recap artifact), EmitComplete MUST fall back to
// event.Content rather than rendering the truncated summary on the IM
// card. The original summary is preserved on meta["summary"] for
// observability / CLI / transcript; the rendered Content uses the full
// transcript instead.
func TestEmitComplete_SummaryQualityTooShort_FallsBackToContent(t *testing.T) {
	sess := &types.Session{SessionID: "sess_test", ChatID: "chat_test"}
	const shortSummary = "ok" // 2 chars — classifies as too_short
	const fullContent = "[full multi-turn transcript from earlier turns]\nReal review here."
	ev := &contracts.EngineEvent{
		Type:      "complete",
		Content:   fullContent,
		SessionID: "sess_test",
		Metadata: map[string]string{
			"summary":         shortSummary,
			"summary_quality": "too_short",
		},
	}
	em := &recordingEmitter{}

	EmitComplete(sess, ev, contracts.IMOutboundSignal{}, false, em)

	msg := em.messages[0]
	if msg.Content != fullContent {
		t.Fatalf("too_short summary must fall back to event.Content; got %q, want %q", msg.Content, fullContent)
	}
	// Original summary preserved on meta for CLI / transcript.
	if msg.Metadata["summary"] != shortSummary {
		t.Fatalf("metadata[summary] must preserve original: got %q, want %q", msg.Metadata["summary"], shortSummary)
	}
	if msg.Metadata["summary_quality"] != "too_short" {
		t.Fatalf("metadata[summary_quality] must propagate from D7: got %q, want too_short", msg.Metadata["summary_quality"])
	}
}

// TestEmitComplete_SummaryQualityInconclusive_FallsBackToContent covers
// the DM-20260630-011 detection path: planning/recap artifact leakage
// (containing <scope_contract>/<planning> markers).
func TestEmitComplete_SummaryQualityInconclusive_FallsBackToContent(t *testing.T) {
	sess := &types.Session{SessionID: "sess_test", ChatID: "chat_test"}
	const recapSummary = "<scope_contract> scope is bounded by 1530-1740 chunks."
	const realContent = "[actual review findings]"
	ev := &contracts.EngineEvent{
		Type:      "complete",
		Content:   realContent,
		SessionID: "sess_test",
		Metadata: map[string]string{
			"summary":         recapSummary,
			"summary_quality": "inconclusive",
		},
	}
	em := &recordingEmitter{}

	EmitComplete(sess, ev, contracts.IMOutboundSignal{}, false, em)

	msg := em.messages[0]
	if msg.Content != realContent {
		t.Fatalf("inconclusive summary must fall back to event.Content; got %q, want %q", msg.Content, realContent)
	}
}

// TestEmitComplete_SummaryQualityValid_PreservesSummary is the
// happy-path: when D7 confirms the summary is structurally valid, the
// summary is kept on Content (no fallback).
func TestEmitComplete_SummaryQualityValid_PreservesSummary(t *testing.T) {
	sess := &types.Session{SessionID: "sess_test", ChatID: "chat_test"}
	const validSummary = "代码审查完成，共发现 3 处问题。建议：(1) xxx；(2) yyy；(3) zzz。"
	ev := &contracts.EngineEvent{
		Type:      "complete",
		Content:   validSummary + "\n[earlier turn transcript]",
		SessionID: "sess_test",
		Metadata: map[string]string{
			"summary":         validSummary,
			"summary_quality": "valid",
		},
	}
	em := &recordingEmitter{}

	EmitComplete(sess, ev, contracts.IMOutboundSignal{}, false, em)

	msg := em.messages[0]
	if msg.Content != validSummary {
		t.Fatalf("valid summary must NOT trigger fallback: got %q, want %q", msg.Content, validSummary)
	}
	if msg.Metadata["summary_quality"] != "valid" {
		t.Fatalf("summary_quality must propagate unchanged: got %q, want valid", msg.Metadata["summary_quality"])
	}
}

// TestEmitComplete_BothSummaryAndFinalBad_EmitsTaskIncomplete pins the
// DM-20260630-011 follow-up regression fix for sess_1782826968112_7000:
// when D7 classified BOTH the summary AND the fallback Content as bad
// (LLM ended mid-tool-call with transitional text like "Now let me look
// at..."), EmitComplete MUST replace the fallback Content with a clear
// "task incomplete" message rather than forwarding the transitional
// phrase to the user. The task_incomplete=true metadata flag is also
// surfaced so dashboards can alert on the pattern.
func TestEmitComplete_BothSummaryAndFinalBad_EmitsTaskIncomplete(t *testing.T) {
	sess := &types.Session{SessionID: "sess_test", ChatID: "chat_test"}
	const transitionalSummary = "ok" // 2 chars — too_short
	const transitionalFinal = "Now let me look at the cross-package contracts referenced from the kernel package." // 82 chars — too_short
	ev := &contracts.EngineEvent{
		Type:      "complete",
		Content:   transitionalFinal,
		SessionID: "sess_test",
		Metadata: map[string]string{
			"summary":         transitionalSummary,
			"summary_quality": "too_short",
			"final_quality":   "too_short",
		},
	}
	em := &recordingEmitter{}

	EmitComplete(sess, ev, contracts.IMOutboundSignal{}, false, em)

	msg := em.messages[0]
	if msg.Content != TaskIncompleteMessage {
		t.Fatalf("both-bad case must emit task-incomplete message; got %q, want %q", msg.Content, TaskIncompleteMessage)
	}
	if msg.Metadata["task_incomplete"] != "true" {
		t.Fatalf("metadata[task_incomplete] = %q, want \"true\"", msg.Metadata["task_incomplete"])
	}
	// Original summary is still preserved for observability / CLI / transcript.
	if msg.Metadata["summary"] != transitionalSummary {
		t.Fatalf("metadata[summary] must preserve original: got %q, want %q", msg.Metadata["summary"], transitionalSummary)
	}
}

// DM-20260708-002 (devrix hotfix for "2×3=6" → "❌ 任务未完成" screenshot):
// when the terminal complete event came from the observational_answer
// fast-path, summary AND Content are intentionally short (e.g. "2×3=6")
// and structurally pre-validated. The task_incomplete override must be
// suppressed — the user wants to see the fast-path answer, not a generic
// failure message. The fast-path source is propagated by D7
// buildSessionCompleteEvent as event.Metadata["source"] with value
// CompleteEventSourceObservationalAnswerFastPath.
//
// Regression guard: removing the source == fast-path branch in
// EmitComplete (or removing the source field from the D7 side) makes
// this test fail.
func TestEmitComplete_FastPathSource_BothBad_PreservesAnswer(t *testing.T) {
	sess := &types.Session{SessionID: "sess_fastpath_d1", ChatID: "chat_test"}
	const fastPathAnswer = "2×3=6" // 4 runes — too_short, but structurally correct
	ev := &contracts.EngineEvent{
		Type:      "complete",
		Content:   fastPathAnswer,
		SessionID: "sess_fastpath_d1",
		Metadata: map[string]string{
			"summary":         fastPathAnswer,
			"summary_quality": "too_short",
			"final_quality":   "too_short",
			"source":          CompleteEventSourceObservationalAnswerFastPath,
		},
	}
	em := &recordingEmitter{}

	EmitComplete(sess, ev, contracts.IMOutboundSignal{}, false, em)

	msg := em.messages[0]
	if msg.Content != fastPathAnswer {
		t.Fatalf("fast-path answer must be preserved; got %q, want %q", msg.Content, fastPathAnswer)
	}
	if msg.Metadata["task_incomplete"] == "true" {
		t.Fatalf("fast-path source must suppress task_incomplete; meta=%v", msg.Metadata)
	}
	// Quality meta is still recorded for observability (Jaeger / dashboards).
	if msg.Metadata["summary_quality"] != "too_short" {
		t.Errorf("summary_quality = %q, want too_short (gate still runs, just doesn't override)", msg.Metadata["summary_quality"])
	}
	// Source must be propagated to the outbound message metadata for D6
	// Evolution / dashboards to count fast-path traffic.
	if msg.Metadata["source"] != CompleteEventSourceObservationalAnswerFastPath {
		t.Errorf("source = %q, want %q", msg.Metadata["source"], CompleteEventSourceObservationalAnswerFastPath)
	}
}

// DM-20260708-002: same content + same bad quality, but source="" —
// the existing task_incomplete override must still trigger. This pins
// the asymmetric behavior: the fast-path bypass is opt-in via source,
// not implicit.
func TestEmitComplete_NoSource_BothBad_StillTriggersTaskIncomplete(t *testing.T) {
	sess := &types.Session{SessionID: "sess_no_source", ChatID: "chat_test"}
	ev := &contracts.EngineEvent{
		Type:      "complete",
		Content:   "2×3=6", // same short content as the fast-path test
		SessionID: "sess_no_source",
		Metadata: map[string]string{
			"summary":         "2×3=6",
			"summary_quality": "too_short",
			"final_quality":   "too_short",
			// no source field — LLM/transitional path, not fast-path
		},
	}
	em := &recordingEmitter{}

	EmitComplete(sess, ev, contracts.IMOutboundSignal{}, false, em)

	msg := em.messages[0]
	if msg.Content != TaskIncompleteMessage {
		t.Fatalf("non-fast-path short content must trigger task_incomplete; got %q, want %q",
			msg.Content, TaskIncompleteMessage)
	}
	if msg.Metadata["task_incomplete"] != "true" {
		t.Fatal("expected task_incomplete meta when source is empty")
	}
}

// TestEmitComplete_OnlySummaryBad_FallsBackToContent pins the original
// behavior preserved for the case where the summary is bad but the
// final Content IS a real review. This is the common path when D7's
// brief extraction mangles a long response — fallback to the full
// transcript is correct.
func TestEmitComplete_OnlySummaryBad_FallsBackToContent(t *testing.T) {
	sess := &types.Session{SessionID: "sess_test", ChatID: "chat_test"}
	const shortSummary = "ok"
	const realContent = "代码审查完成，整体结构清晰，无阻塞问题。建议关注 xxx 模块。"
	ev := &contracts.EngineEvent{
		Type:      "complete",
		Content:   realContent,
		SessionID: "sess_test",
		Metadata: map[string]string{
			"summary":         shortSummary,
			"summary_quality": "too_short",
			"final_quality":   "valid",
		},
	}
	em := &recordingEmitter{}

	EmitComplete(sess, ev, contracts.IMOutboundSignal{}, false, em)

	msg := em.messages[0]
	if msg.Content != realContent {
		t.Fatalf("only-summary-bad must fall back to event.Content; got %q, want %q", msg.Content, realContent)
	}
	if _, ok := msg.Metadata["task_incomplete"]; ok {
		t.Fatalf("task_incomplete must NOT be set when fallback Content is valid; got %q", msg.Metadata["task_incomplete"])
	}
}
