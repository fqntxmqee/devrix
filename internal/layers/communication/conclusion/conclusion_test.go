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
