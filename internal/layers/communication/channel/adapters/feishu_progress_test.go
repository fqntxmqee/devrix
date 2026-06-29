package adapters

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/devrix/devrix/internal/layers/communication/kernel"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestStripOuterCodeFence(t *testing.T) {
	input := "```go\nfunc main() {}\n```"
	got := stripOuterCodeFence(input)
	if got != "func main() {}" {
		t.Fatalf("stripOuterCodeFence() = %q, want func main() {}", got)
	}
}

// ============================================================================
// stripTrailingSummary (DM-20260621-008)
//
// The LLM (notably minimax M2.7) emits the D7 final summary as ordinary
// text events, so the streaming reply card's textBuffer ends with the same
// paragraph that the standalone 任务总结 card will carry. Without the
// strip, the user sees the same conclusion twice. Pin the helper so a
// future refactor of finalizeReplyCardStreaming / finalizeStructuredSession
// doesn't regress the duplicate-display bug.
// ============================================================================

func TestStripTrailingSummary(t *testing.T) {
	cases := []struct {
		name    string
		content string
		summary string
		want    string
	}{
		{
			name:    "empty_summary_returns_content_unchanged",
			content: "我已完成 4 路 review。",
			summary: "",
			want:    "我已完成 4 路 review。",
		},
		{
			name:    "whitespace_only_summary_returns_content_unchanged",
			content: "我已完成 4 路 review。",
			summary: "   \t\n  ",
			want:    "我已完成 4 路 review。",
		},
		{
			name:    "content_ends_with_summary_strips_it",
			content: "我已完成 4 路 review。\n\n最终结论：4 路并行 deep review 全部 PASS。",
			summary: "最终结论：4 路并行 deep review 全部 PASS。",
			want:    "我已完成 4 路 review。",
		},
		{
			name:    "trailing_whitespace_between_report_and_summary_stripped",
			content: "我已完成 4 路 review。\n\n\n   \n最终结论：4 路并行 deep review 全部 PASS。\n",
			summary: "最终结论：4 路并行 deep review 全部 PASS。",
			want:    "我已完成 4 路 review。",
		},
		{
			name:    "content_does_not_end_with_summary_returns_unchanged",
			content: "我已完成 4 路 review。\n\n接下来做归档。",
			summary: "最终结论：4 路并行 deep review 全部 PASS。",
			want:    "我已完成 4 路 review。\n\n接下来做归档。",
		},
		{
			name:    "content_shorter_than_summary_returns_unchanged",
			content: "短文本",
			summary: "这是一段很长的最终结论：4 路并行 deep review 全部 PASS。",
			want:    "短文本",
		},
		{
			name:    "exact_match_no_trailing_newline_strips",
			content: "report正文最终结论：4 路并行 deep review 全部 PASS。",
			summary: "最终结论：4 路并行 deep review 全部 PASS。",
			want:    "report正文",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripTrailingSummary(tc.content, tc.summary)
			if got != tc.want {
				t.Errorf("stripTrailingSummary(%q, %q) = %q, want %q", tc.content, tc.summary, got, tc.want)
			}
		})
	}
}

func TestNormalizeProgressStyle_DefaultStructured(t *testing.T) {
	if got := normalizeProgressStyle(""); got != progressStyleStructured {
		t.Fatalf("normalizeProgressStyle(\"\") = %q, want %q", got, progressStyleStructured)
	}
	if got := normalizeProgressStyle("card"); got != progressStyleStructured {
		t.Fatalf("normalizeProgressStyle(\"card\") = %q, want structured only", got)
	}
}

func TestBuildTaskProgressCard_ProgressBarAndSummaries(t *testing.T) {
	stream := &feishuSessionStream{
		progressPct: 50,
		taskName:    "排查 OK 确认图标",
		summaries:   []string{"代码逻辑正确", "需排查运行时日志"},
	}
	card := buildTaskProgressCard(stream, false, "")
	body := cardBodyMarkdown(card)
	if !strings.Contains(body, "50%") || !strings.Contains(body, "█") {
		t.Fatalf("progress bar missing: %q", body)
	}
	if !strings.Contains(body, "排查 OK 确认图标") {
		t.Fatalf("task name missing: %q", body)
	}
	if !strings.Contains(body, "代码逻辑正确") {
		t.Fatalf("summaries missing: %q", body)
	}

	done := buildTaskProgressCard(stream, true, "")
	doneBody := cardBodyMarkdown(done)
	if !strings.Contains(doneBody, "已完成") || !strings.Contains(doneBody, "100%") {
		t.Fatalf("completed state missing: %q", doneBody)
	}
}

// TestBuildTaskProgressCard_FailedExitReason covers the hotfix
// 2026-06-27 (sess_1782541795374_7000) path: when the orchestrator
// surfaces a failed exit_reason (verifier_fail / verifier_abstain /
// system_anomaly / intent_only_aborted), the progress card renders
// red "任务失败" with the reason, not the misleading green
// "任务完成" / "✅ 已完成".
func TestBuildTaskProgressCard_FailedExitReason(t *testing.T) {
	cases := []struct {
		name       string
		exitReason string
		wantTitle  string
		wantMarker string
	}{
		{"verifier_fail", "verifier_fail", "任务失败", "❌ **失败（verifier_fail）**"},
		{"verifier_abstain", "verifier_abstain", "任务失败", "❌ **失败（verifier_abstain）**"},
		{"system_anomaly", "system_anomaly", "任务失败", "❌ **失败（system_anomaly）**"},
		{"intent_only_aborted", "intent_only_aborted", "任务失败", "❌ **失败（intent_only_aborted）**"},
		{"natural (sanity)", "natural", "任务完成", "✅ **已完成**"},
		{"empty (sanity)", "", "任务完成", "✅ **已完成**"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stream := &feishuSessionStream{
				progressPct: 100,
				taskName:    "review d2 kernel",
			}
			card := buildTaskProgressCard(stream, true, tc.exitReason)
			title := ""
			if card.Header != nil {
				title = card.Header.Title
			}
			if title != tc.wantTitle {
				t.Errorf("title = %q, want %q", title, tc.wantTitle)
			}
			body := cardBodyMarkdown(card)
			if !strings.Contains(body, tc.wantMarker) {
				t.Errorf("body missing marker %q in:\n%s", tc.wantMarker, body)
			}
		})
	}
}

// TestIsFailedExitReason covers the failedExitReasons membership test
// used by the hotfix 2026-06-27 (sess_1782541795374_7000) display
// path. Mirrors the verify.ExitReason taxonomy in
// orchestration/executionflow/verify/exit_reason.go.
func TestIsFailedExitReason(t *testing.T) {
	cases := []struct {
		reason string
		want   bool
	}{
		{"verifier_fail", true},
		{"verifier_abstain", true},
		{"system_anomaly", true},
		{"intent_only_aborted", true},
		{"natural", false},
		{"max_turns", false},
		{"repeated_tool", false},
		{"aborted_user", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			if got := isFailedExitReason(tc.reason); got != tc.want {
				t.Errorf("isFailedExitReason(%q) = %v, want %v", tc.reason, got, tc.want)
			}
		})
	}
}

// TestBuildCompletionFooter covers the non-cardkit hotfix footer
// branch: failed tasks render "❌ 任务失败（<reason>）_" instead of
// the misleading "✅ 任务已完成".
func TestBuildCompletionFooter(t *testing.T) {
	cases := []struct {
		name       string
		stripped   string
		failed     bool
		exitReason string
		want       string
	}{
		{"success with body", "report body", false, "natural", "report body\n\n---\n_✅ 任务已完成_"},
		{"success empty body", "", false, "natural", "_✅ 任务已完成_"},
		{"verifier_fail with body", "report body", true, "verifier_fail", "report body\n\n---\n_❌ 任务失败（verifier_fail）_"},
		{"verifier_fail empty body", "", true, "verifier_fail", "_❌ 任务失败（verifier_fail）_"},
		{"intent_only_aborted with body", "让我先...", true, "intent_only_aborted", "让我先...\n\n---\n_❌ 任务失败（intent_only_aborted）_"},
		{"failed but no reason (defensive)", "x", true, "", "x\n\n---\n_❌ 任务失败_"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildCompletionFooter(tc.stripped, tc.failed, tc.exitReason)
			if got != tc.want {
				t.Errorf("buildCompletionFooter = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildToolsCardMarkdown_MultipleTools(t *testing.T) {
	body := buildToolsCardMarkdown([]toolCallEntry{
		{name: "Grep", input: "auth"},
		{name: "Read", input: "main.go"},
	}, false)
	if !strings.Contains(body, "**工具 #1:** `Grep`") || !strings.Contains(body, "**工具 #2:** `Read`") {
		t.Fatalf("missing tool entries: %q", body)
	}
	if strings.Contains(body, "**结果**") {
		t.Fatalf("results should be hidden: %q", body)
	}
}

func TestFeishuAdapter_AggregatedToolCalls_OneCard(t *testing.T) {
	var replyCount int
	var patchCount int
	msgID := "om_tools"
	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			replyCount++
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
		patchFunc: func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
			patchCount++
			return &larkim.PatchMessageResp{}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
	}, &config.CommunicationConfig{}, WithFeishuAPI(&mockFeishuAPI{imAPI: mockImAPI}))
	adapter.sessionReplyCtx.Store("sess_agg", feishuReplyContext{userMessageID: "om_root"})

	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_agg", ChatID: "feishu_oc_123456_ou_654321",
		Content: "Grep", Metadata: map[string]string{"event_type": "tool_call", "tool_name": "Grep", "input": "auth"},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_agg", ChatID: "feishu_oc_123456_ou_654321",
		Content: "Read", Metadata: map[string]string{"event_type": "tool_call", "tool_name": "Read", "input": "main.go"},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_agg", ChatID: "feishu_oc_123456_ou_654321",
		Content: "Read", Metadata: map[string]string{"event_type": "tool_call", "tool_name": "Shell", "input": "go test"},
	})

	if replyCount != 1 {
		t.Fatalf("replyCount = %d, want 1 aggregated tools card", replyCount)
	}
	if patchCount != 2 {
		t.Fatalf("patchCount = %d, want 2 patches for 2nd and 3rd tool calls", patchCount)
	}

	stream := adapter.sessionStream("sess_agg")
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(stream.toolCalls) != 3 {
		t.Fatalf("toolCalls len = %d, want 3", len(stream.toolCalls))
	}
	if stream.toolsMsgID != msgID {
		t.Fatalf("toolsMsgID = %q, want %q", stream.toolsMsgID, msgID)
	}
}

func TestFeishuAdapter_StructuredProgress_SeparateThinkingToolAndTaskCard(t *testing.T) {
	var replyCount int
	var patchCount int

	msgID := "om_progress"
	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			replyCount++
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
		patchFunc: func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
			patchCount++
			return &larkim.PatchMessageResp{}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:         "test_app",
		AppSecret:     "test_secret",
		ProgressStyle: progressStyleStructured,
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_1", feishuReplyContext{userMessageID: "om_root"})

	events := []types.OutboundMessage{
		{SessionID: "sess_1", ChatID: "feishu_oc_123456_ou_654321", Content: "思考步骤 1", Metadata: map[string]string{"event_type": "thinking"}},
		{SessionID: "sess_1", ChatID: "feishu_oc_123456_ou_654321", Content: "Grep", Metadata: map[string]string{"event_type": "tool_call", "tool_name": "Grep", "input": "OK.*确认"}},
		{SessionID: "sess_1", ChatID: "feishu_oc_123456_ou_654321", Content: "matched line", Metadata: map[string]string{"event_type": "tool_result", "tool_name": "Grep"}},
		{SessionID: "sess_1", ChatID: "feishu_oc_123456_ou_654321", Metadata: map[string]string{"event_type": "milestone_progress", "progress": "50%", "task": "排查图标"}},
		{SessionID: "sess_1", ChatID: "feishu_oc_123456_ou_654321", Content: "小结 A", Metadata: map[string]string{"event_type": "info"}},
	}
	for i := range events {
		adapter.OnMessage(&events[i])
	}
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_1",
		ChatID:    "feishu_oc_123456_ou_654321",
		Metadata:  map[string]string{"event_type": "complete"},
	})

	// thinking + tool + task progress create = 3 replies; tool_result hidden by default
	if replyCount != 3 {
		t.Fatalf("replyCount = %d, want 3", replyCount)
	}
	if patchCount < 2 {
		t.Fatalf("patchCount = %d, want at least 2", patchCount)
	}
}

func TestFeishuAdapter_should_show_tool_result_when_enabled(t *testing.T) {
	var patchCount int
	msgID := "om_tool"
	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
		patchFunc: func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
			patchCount++
			return &larkim.PatchMessageResp{}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:           "test_app",
		AppSecret:       "test_secret",
		ShowToolResults: true,
	}, &config.CommunicationConfig{}, WithFeishuAPI(&mockFeishuAPI{imAPI: mockImAPI}))
	adapter.sessionReplyCtx.Store("sess_tool", feishuReplyContext{userMessageID: "om_root"})

	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_tool", ChatID: "feishu_oc_123456_ou_654321",
		Content: "read_file", Metadata: map[string]string{"event_type": "tool_call", "tool_name": "read_file", "input": "main.go"},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_tool", ChatID: "feishu_oc_123456_ou_654321",
		Content: "package main", Metadata: map[string]string{"event_type": "tool_result", "tool_name": "read_file"},
	})
	if patchCount != 1 {
		t.Fatalf("patchCount = %d, want 1 tool result patch", patchCount)
	}
}

func TestFeishuAdapter_should_hide_tool_result_by_default(t *testing.T) {
	var patchCount int
	msgID := "om_tool"
	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
		patchFunc: func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
			patchCount++
			return &larkim.PatchMessageResp{}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
	}, &config.CommunicationConfig{}, WithFeishuAPI(&mockFeishuAPI{imAPI: mockImAPI}))
	adapter.sessionReplyCtx.Store("sess_tool", feishuReplyContext{userMessageID: "om_root"})

	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_tool", ChatID: "feishu_oc_123456_ou_654321",
		Content: "read_file", Metadata: map[string]string{"event_type": "tool_call", "tool_name": "read_file"},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_tool", ChatID: "feishu_oc_123456_ou_654321",
		Content: "package main", Metadata: map[string]string{"event_type": "tool_result", "tool_name": "read_file"},
	})
	if patchCount != 0 {
		t.Fatalf("patchCount = %d, want 0 when show_tool_results disabled", patchCount)
	}
}

func TestFormatWorkerProgressSummary_SkipsToolCall(t *testing.T) {
	msg := &types.OutboundMessage{
		Metadata: map[string]string{"kind": "tool_call", "role": "explore"},
		Content:  "grep auth",
	}
	if got := formatWorkerProgressSummary(msg); got != "" {
		t.Fatalf("formatWorkerProgressSummary(tool_call) = %q, want empty", got)
	}
	started := &types.OutboundMessage{
		Metadata: map[string]string{"kind": "started", "role": "explore", "worker_id": "w1"},
		Content:  "started explore",
	}
	if got := formatWorkerProgressSummary(started); !strings.Contains(got, "explore") {
		t.Fatalf("formatWorkerProgressSummary(started) = %q", got)
	}
}

// TestFeishuAdapter_StructuredProgress_SimpleReplySummaryAsSeparateCard
// pins the non-streaming path behavior after the DM-20260621-001 /
// DM-20260621-008 fixes. For a simple text → complete flow where the
// LLM text does NOT end with the summary:
//   1. The response card is created via Reply (1 reply).
//   2. The non-empty summary is delivered as a SEPARATE reply (2nd
//      reply) — the legacy "summary glued onto the response card with
//      `---`" behavior is intentionally removed.
//   3. The response card IS patched with a minimal "✅ 任务已完成"
//      completion footer (DM-20260621-008: previously left untouched, but
//      that made the card feel dangling when a separate summary card
//      followed). The patch body must NOT contain the summary text.
func TestFeishuAdapter_StructuredProgress_SimpleReplySummaryAsSeparateCard(t *testing.T) {
	var replyCount int
	var patchCount int

	msgID := "om_response"
	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			replyCount++
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
		patchFunc: func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
			patchCount++
			return &larkim.PatchMessageResp{}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:         "test_app",
		AppSecret:     "test_secret",
		ProgressStyle: progressStyleStructured,
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_1", feishuReplyContext{userMessageID: "om_root"})

	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_1", ChatID: "feishu_oc_123456_ou_654321",
		Content: "你好！", Metadata: map[string]string{"event_type": "text"},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_1", ChatID: "feishu_oc_123456_ou_654321",
		Content: "用时: 8s, 消耗: 1500 tokens, 模型: claude-sonnet-4-6", Metadata: map[string]string{"event_type": "complete"},
	})

	// 1 reply for the response card + 1 reply for the separate summary card.
	if replyCount != 2 {
		t.Fatalf("replyCount = %d, want 2 (1 response card + 1 separate summary card)", replyCount)
	}
	// DM-20260621-008: the response card is patched with a minimal
	// "✅ 任务已完成" completion marker so the user still sees the task
	// closed when a separate summary card follows.
	if patchCount < 1 {
		t.Fatalf("patchCount = %d, want >=1 (response card patched with completion marker)", patchCount)
	}
}

// TestFeishuAdapter_FinalizeStructuredSession_EmptySummaryPatchesFooter
// pins the fix for the user-reported bug
// ("devrix 好像理解我的意思，但最后没有总结信息发送给我"):
// when the D7 orchestrator emits a complete event with an empty
// summary (e.g. max-turns reached mid-tool-call while the LLM was still
// looping), the IM adapter MUST still patch the reply card with a
// minimal completion footer so the user sees "✅ 任务已完成" rather
// than a dangling partial card with no closure. Without the fix, the
// session silently returned from finalizeStructuredSession and the
// user received no signal that the task had finished.
func TestFeishuAdapter_FinalizeStructuredSession_EmptySummaryPatchesFooter(t *testing.T) {
	var replyCount int
	var patchCount int

	msgID := "om_response"
	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			replyCount++
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
		patchFunc: func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
			patchCount++
			return &larkim.PatchMessageResp{}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:         "test_app",
		AppSecret:     "test_secret",
		ProgressStyle: progressStyleStructured,
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_empty_summary", feishuReplyContext{userMessageID: "om_root"})

	// First emit a text event so responseMsgID is populated.
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_empty_summary", ChatID: "feishu_oc_123456_ou_654321",
		Content: "好的，让我继续增加上下文", Metadata: map[string]string{"event_type": "text"},
	})
	// Then emit a complete event with EMPTY content (the failure mode).
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_empty_summary", ChatID: "feishu_oc_123456_ou_654321",
		Content: "", Metadata: map[string]string{"event_type": "complete"},
	})

	if replyCount != 1 {
		t.Fatalf("replyCount = %d, want 1 (response card created)", replyCount)
	}
	// The fix: a patch MUST be issued even with empty summary.
	// Without the fix, finalizeStructuredSession returned nil silently
	// (because strings.TrimSpace("") is "" and the responseMsgID != ""
	// check at the bottom short-circuited without ever patching) and
	// the user received no "任务完成" signal at all.
	if patchCount < 1 {
		t.Fatalf("patchCount = %d, want >=1 (empty-summary fallback must still patch the reply card)", patchCount)
	}
}

// extractPatchedCardJSON was removed; see the simpler
// TestFeishuAdapter_FinalizeStructuredSession_EmptySummaryPatchesFooter
// which validates behavior via the patch call count + stream state.

// TestFeishuAdapter_FinalizeReplyCardStreaming_UpdateCardFallbackOnStreamClosed
// verifies that when the cardkit streaming channel is closed by Feishu at
// finalize time (idle timeout or prior finalization), the finalize path
// falls through to UpdateCard instead of silently leaving the reply card
// stale. Without this fallback, deep-review sessions that took longer
// than the cardkit stream lifetime (>30min) lost the conclusion — the
// user saw no report on the feishu card.
func TestFeishuAdapter_FinalizeReplyCardStreaming_UpdateCardFallbackOnStreamClosed(t *testing.T) {
	var streamPutCount int
	var updateCount int

	mockAPI := &mockFeishuAPI{
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			// Distinguish the two endpoints by path:
			//   /open-apis/cardkit/v1/cards/{id}/elements/{eid}/content  → stream element
			//   /open-apis/cardkit/v1/cards/{id}                          → update card
			if strings.Contains(path, "/elements/") {
				streamPutCount++
				// Simulate Feishu closing the streaming channel mid-session.
				return &larkcore.ApiResp{
					StatusCode: 200,
					RawBody:    []byte(`{"code":300309,"msg":"card stream closed"}`),
				}, nil
			}
			updateCount++
			return &larkcore.ApiResp{StatusCode: 200, RawBody: []byte(`{"code":0}`)}, nil
		},
	}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))

	// Pre-populate the cardkit state so finalizeReplyCardStreaming thinks
	// the session already has a live cardkit reply card with streamed
	// text. This mirrors what appendResponseText → startStreamingReplyCard
	// would have built during a real session.
	const sessionID = "sess_cardkit_closed"
	stream := adapter.sessionStream(sessionID)
	stream.mu.Lock()
	stream.replyCardID = "card_xyz"
	stream.cardkitEnabled = true
	stream.textBuffer.WriteString("first: exploring repo\n")
	stream.textBuffer.WriteString("second: analyzing tools\n")
	stream.textBuffer.WriteString("third: writing report")
	stream.mu.Unlock()

	if err := adapter.finalizeReplyCardStreaming(context.Background(), stream, "conclusion summary", sessionID, ""); err != nil {
		t.Fatalf("finalizeReplyCardStreaming: %v", err)
	}

	// The stream PUT to /elements/.../content must have been attempted.
	if streamPutCount != 1 {
		t.Errorf("streamPutCount = %d, want 1 (stream PUT must be attempted before fallback)", streamPutCount)
	}
	// The bug: without the fix, updateCount == 0 and the user never
	// sees the report. With the fix, UpdateCard fires after the stream
	// PUT returns ErrFeishuCardStreamClosed.
	if updateCount != 1 {
		t.Errorf("updateCount = %d, want 1 (UpdateCard fallback after stream-closed)", updateCount)
	}
}

func cardBodyMarkdown(card *kernel.Card) string {
	var parts []string
	for _, elem := range card.Elements {
		if md, ok := elem.(kernel.CardMarkdown); ok {
			parts = append(parts, md.Content)
		}
	}
	return strings.Join(parts, "\n")
}

func TestEnsureAgentStreamCard_OnToolCall(t *testing.T) {
	replyCount := 0
	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			replyCount++
			msgID := "om_agent_start"
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID: "test_app", AppSecret: "test_secret",
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_agent", feishuReplyContext{userMessageID: "om_root"})

	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_agent",
		ChatID:    "feishu_oc_123456_ou_654321",
		Content:   "call_claude-code",
		Metadata:  map[string]string{"event_type": "tool_call", "tool_name": "call_claude-code"},
	})

	if replyCount < 1 {
		t.Fatalf("replyCount = %d, want at least 1 reply for agent card on tool_call", replyCount)
	}
	stream := adapter.sessionStream("sess_agent")
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.agentOutputMsgID == "" {
		t.Fatal("expected agentOutputMsgID after tool_call")
	}
}

func TestAppendAgentStreamText_UsesDedicatedCard(t *testing.T) {
	replyCount := 0
	patchCount := 0
	msgID := "om_agent_stream"
	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			replyCount++
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
		patchFunc: func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
			patchCount++
			return &larkim.PatchMessageResp{}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:         "test_app",
		AppSecret:     "test_secret",
		ProgressStyle: progressStyleStructured,
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_agent", feishuReplyContext{userMessageID: "om_root"})

	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_agent", ChatID: "feishu_oc_123456_ou_654321",
		Content: "line 1", Metadata: map[string]string{"event_type": "text", "source": "agent_tool", "agent": "Claude Code"},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_agent", ChatID: "feishu_oc_123456_ou_654321",
		Content: "line 2", Metadata: map[string]string{"event_type": "text", "source": "agent_tool", "agent": "Claude Code"},
	})

	if replyCount != 1 {
		t.Fatalf("replyCount = %d, want 1", replyCount)
	}
	if patchCount != 1 {
		t.Fatalf("patchCount = %d, want 1", patchCount)
	}

	stream := adapter.sessionStream("sess_agent")
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.agentOutputMsgID != msgID {
		t.Fatalf("agentOutputMsgID = %q, want %q", stream.agentOutputMsgID, msgID)
	}
	if got := stream.agentOutputBuffer.String(); got != "line 1\nline 2" {
		t.Fatalf("agentOutputBuffer = %q", got)
	}
}

// ============================================================================
// Thinking-card replay dedup (DM-20260621-006)
//
// Without these dedup layers, minimax M2.7 streaming replay causes the
// thinking card to render the same paragraph 2-3 times. Pin the fix so a
// future refactor of sendStructuredThinkingCard doesn't regress.
// ============================================================================

func TestSendStructuredThinkingCard_DropsReplayChunk(t *testing.T) {
	msgID := "om_thinking"
	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
		patchFunc: func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
			return &larkim.PatchMessageResp{}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID: "test_app", AppSecret: "test_secret",
		ProgressStyle: progressStyleStructured,
	}, &config.CommunicationConfig{}, WithFeishuAPI(&mockFeishuAPI{imAPI: mockImAPI}))
	adapter.sessionReplyCtx.Store("sess_thinking", feishuReplyContext{userMessageID: "om_root"})

	opening := strings.Repeat("我先快速摸清项目结构与规模。", 5) +
		"这是一个 Go 项目（Devrix 多智能体协作助手），6 域架构，规模较大。\n" +
		"由于域之间是相对独立的（D1-D7 分别对应一个层），适合按域并行 review。\n" +
		"我设计如下并行策略。"

	// First event: legitimate long opening narration.
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_thinking", ChatID: "feishu_oc_123456_ou_654321",
		Content:   opening,
		Metadata:  map[string]string{"event_type": "thinking"},
	})

	// Second event: LLM re-emits the opening (replay). Must be dropped.
	replayChunk := "我先快速摸清项目结构与规模。我先快速摸清项目结构与规模。我先快速摸清项目结构与规模。\n接下来要做的是："
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_thinking", ChatID: "feishu_oc_123456_ou_654321",
		Content:   replayChunk,
		Metadata:  map[string]string{"event_type": "thinking"},
	})

	stream := adapter.sessionStream("sess_thinking")
	stream.mu.Lock()
	defer stream.mu.Unlock()
	got := stream.thinkingBuffer.String()
	if strings.Contains(got, "接下来要做的是") {
		t.Fatalf("thinkingBuffer contains replay chunk text — dedup missed:\n%s", got)
	}
	if !strings.Contains(got, "6 域架构") {
		t.Fatalf("thinkingBuffer missing legitimate opening content:\n%s", got)
	}
}

func TestSendStructuredThinkingCard_NaturalRepetitionPassesThrough(t *testing.T) {
	// DM-20260626-009 follow-up: pin that natural Chinese repetition
	// (the kind DedupAdjacentRepeats used to false-positive on) flows
	// through patchThinkingCard untouched. Before the dedup removal the
	// LCP-based dedup flagged "先看一下代码" as an echo and stripped the
	// second occurrence, leaving "先看一下代码。的结构。" — semantically
	// mangled. After the removal, the buffer must render verbatim so
	// legitimate repetition survives.
	opening := "先看一下代码。先看一下代码的结构。让我开始 review。"

	stream := &feishuSessionStream{}
	stream.thinkingBuffer.WriteString(opening)
	// patchThinkingCard's text-only path: StripToolCallXML + TrimSpace.
	// We bypass the network call by reading the buffer post-processing
	// directly (the test asserts no over-dedup happens, not the network
	// round-trip).
	got := strings.TrimSpace(stream.thinkingBuffer.String())
	if !strings.Contains(got, "先看一下代码") || !strings.Contains(got, "先看一下代码的结构") {
		t.Fatalf("natural repetition was over-deduped:\n  in:  %q\n  out: %q", opening, got)
	}
	if !strings.Contains(got, "让我开始 review") {
		t.Fatalf("tail text was lost:\n  in:  %q\n  out: %q", opening, got)
	}
}
//
// minimax M2.7 (and similar models with streaming artifacts) occasionally
// re-emits a previously streamed prefix from scratch. The feishu reply
// card must show the report only once. The two helpers below plus their
// hook points (appendResponseText + finalizeReplyCardStreaming /
// finalizeStructuredSession) implement that dedup. These tests pin the
// behavior so the next refactor doesn't silently regress.
// ============================================================================

func TestDetectDuplicateReplay(t *testing.T) {
	// Build a long buffer of a representative LLM reply: 600+ runes
	// describing a deep review, structured as the transcript chunks
	// show. We then construct a "replayed" chunk that re-emits the
	// opening narration and assert detectDuplicateReplay catches it.
	buffer := strings.Repeat("我先快速摸清项目结构与规模。", 5) +
		"这是一个 Go 项目（Devrix 多智能体协作助手），6 域架构，规模较大。\n" +
		"由于域之间是相对独立的（D1-D7 分别对应一个层），适合按域并行 review。\n" +
		"我设计如下并行策略。"

	cases := []struct {
		name  string
		chunk string
		want  bool
	}{
		{
			name:  "replay-of-opening-narration",
			chunk: "我先快速摸清项目结构与规模。我先快速摸清项目结构与规模。我先快速摸清项目结构与规模。\n接下来要做的是：",
			want:  true,
		},
		{
			name:  "natural-continuation",
			chunk: "继续探索第二个域 D3 LLM 网关的子包分布。",
			want:  false,
		},
		{
			name:  "short-chunk-passes-through",
			chunk: "OK",
			want:  false,
		},
		{
			name:  "long-unique-chunk-passes-through",
			chunk: "接下来将分两路执行：路径 A 走人工 review 路径 B 走自动化覆盖率验证，两路并行完成后我会汇总报告。",
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectDuplicateReplay(buffer, tc.chunk); got != tc.want {
				t.Fatalf("detectDuplicateReplay() = %v, want %v (buffer runes=%d, chunk runes=%d)",
					got, tc.want, utf8.RuneCountInString(buffer), utf8.RuneCountInString(tc.chunk))
			}
		})
	}
}

func TestDetectDuplicateReplay_ShortBufferSkips(t *testing.T) {
	// Buffer shorter than dedupReplayMinBufferRunes (100) → always false
	// even if the chunk is a verbatim prefix. This avoids false positives
	// at the very start of a session when the LLM legitimately repeats
	// short opening phrases.
	shortBuffer := "我先快速摸清项目结构与规模。"
	chunk := "我先快速摸清项目结构与规模。我先快速摸清项目结构与规模。我先快速摸清项目结构与规模。\n接下来"
	if detectDuplicateReplay(shortBuffer, chunk) {
		t.Fatalf("detectDuplicateReplay() = true on short buffer, want false")
	}
}

func TestDetectDuplicateReplay_NaturalRepetitionPassesThrough(t *testing.T) {
	// DM-20260626-009 follow-up: pin that detectDuplicateReplay does NOT
	// fire on natural Chinese repetition. Before the dedup removal
	// (DM-20260621-006) we relied on the streaming-time layer plus the
	// LCP-based post-hoc dedup; after the removal, detectDuplicateReplay
	// is the sole surviving layer and it must remain conservative on
	// legitimate repetition. The buffer here is short (~30 runes total),
	// which trips the dedupReplayMinBufferRunes=100 short-circuit; the
	// chunk is also short and shares no 30+ rune prefix with the buffer,
	// so the function returns false (i.e. the chunk flows through).
	buffer := "先看一下代码。先看一下代码的结构。"
	chunk := "让我开始 review。"
	if detectDuplicateReplay(buffer, chunk) {
		t.Fatalf("detectDuplicateReplay fired on natural repetition — should pass through:\n  buffer: %q\n  chunk:  %q", buffer, chunk)
	}
}

// TestFeishuAdapter_AppendResponseText_DropsReplayedChunk pins the
// streaming-time dedup. The LLM replays a 200-rune opening narration
// (the same text it just streamed in the first two text events). The
// adapter must drop the replay and NOT increment textBuffer / NOT call
// the cardkit stream PUT. Without this fix, the live cardkit stream
// would show the opening twice while the LLM is still talking.
func TestFeishuAdapter_AppendResponseText_DropsReplayedChunk(t *testing.T) {
	opening := "我先快速摸清项目结构与规模。这是一段开场白描述，描述项目的总体情况与策略选择依据。"
	openingFull := opening + strings.Repeat("补", 80) + "结束。"

	var streamPutCount int
	mockAPI := &mockFeishuAPI{
		postFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			// cardkit CreateCard is called once when the streaming reply
			// card starts. We don't count this against "stream PUTs".
			return &larkcore.ApiResp{StatusCode: 200, RawBody: []byte(`{"code":0,"data":{"card_id":"card_dd"}}`)}, nil
		},
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			if strings.Contains(path, "/elements/") {
				streamPutCount++
			}
			return &larkcore.ApiResp{StatusCode: 200, RawBody: []byte(`{"code":0}`)}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: &mockMessageAPI{}, messageReactionAPI: &mockMessageReactionAPI{}}
	mockAPI.imAPI = mockImAPI

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.streamingEnabled = true
	adapter.sessionReplyCtx.Store("sess_replay", feishuReplyContext{userMessageID: "om_root"})

	// First text event: open the streaming card and stream the opening.
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_replay", ChatID: "feishu_oc_1",
		Content: openingFull, Metadata: map[string]string{"event_type": "text"},
	})
	stream := adapter.sessionStream("sess_replay")
	stream.mu.Lock()
	bufferAfterFirst := stream.textBuffer.String()
	stream.mu.Unlock()
	if bufferAfterFirst != openingFull {
		t.Fatalf("after first emit, buffer = %q, want %q", bufferAfterFirst, openingFull)
	}

	// Second text event: LLM replays the same opening from scratch.
	// Adapter MUST drop this chunk.
	streamPutCount = 0
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_replay", ChatID: "feishu_oc_1",
		Content: openingFull + "接着是新的内容。",
		Metadata: map[string]string{"event_type": "text"},
	})
	stream.mu.Lock()
	bufferAfterReplay := stream.textBuffer.String()
	stream.mu.Unlock()
	if bufferAfterReplay != openingFull {
		t.Fatalf("replay was not dropped: buffer grew to %q, want unchanged %q", bufferAfterReplay, openingFull)
	}
}

// TestFeishuAdapter_FinalizeReplyCardStreaming_PreservesDuplicateText
// pins the post-DM-20260626-009 behaviour: post-hoc dedup is gone, so
// a duplicate that survives detectDuplicateReplay (e.g. suffix-overlap
// rather than prefix) now flows through to the streaming card verbatim.
// The trade-off is deliberate — false-positive dedup was mangling
// natural Chinese repetition, so we accept the rare genuine replay in
// exchange for legitimate text staying intact. The completion marker
// must still be present so the user sees the task closed.
func TestFeishuAdapter_FinalizeReplyCardStreaming_PreservesDuplicateText(t *testing.T) {
	opening := "我先快速摸清项目结构与规模。这是一段开场白描述，描述项目的总体情况与策略选择依据。"
	openingFull := opening + strings.Repeat("补", 80) + "结束。"

	var updateCardData string
	mockAPI := &mockFeishuAPI{
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			if !strings.Contains(path, "/elements/") {
				if m, ok := body.(map[string]any); ok {
					if card, ok := m["card"].(map[string]any); ok {
						if data, ok := card["data"].(string); ok {
							updateCardData = data
						}
					}
				}
			}
			return &larkcore.ApiResp{StatusCode: 200, RawBody: []byte(`{"code":0}`)}, nil
		},
	}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))

	const sessionID = "sess_finalize_dedup"
	stream := adapter.sessionStream(sessionID)
	stream.mu.Lock()
	stream.replyCardID = "card_dd"
	stream.cardkitEnabled = true
	// Simulate a buffer where the LLM streamed its full report, then
	// re-emitted the same opening narration verbatim (suffix overlap
	// pattern that detectDuplicateReplay's prefix-based heuristic
	// might miss).
	stream.textBuffer.WriteString(openingFull)
	stream.textBuffer.WriteString("\n\n我接下来对各域做并行 review。\n")
	stream.textBuffer.WriteString(openingFull) // duplicate suffix
	stream.textBuffer.WriteString("\n\n最终结论：4 路并行都成功完成。")
	stream.mu.Unlock()

	if err := adapter.finalizeReplyCardStreaming(context.Background(), stream, "", sessionID, ""); err != nil {
		t.Fatalf("finalizeReplyCardStreaming: %v", err)
	}

	if updateCardData == "" {
		t.Fatalf("UpdateCard card.data was not captured")
	}
	// Post-DM-20260626-009: both occurrences of the duplicate survive.
	// If the LLM genuinely replayed (suffix-overlap), the user sees it
	// twice — accepted trade-off for natural-text preservation.
	if got := strings.Count(updateCardData, opening); got < 1 {
		t.Errorf("opening missing from UpdateCard card.data. card.data=%s", updateCardData)
	}
	// The conclusion must still be present.
	if !strings.Contains(updateCardData, "最终结论：4 路并行都成功完成") {
		t.Errorf("UpdateCard card.data missing conclusion: %s", updateCardData)
	}
	// Empty-summary path: the streaming card carries the minimal
	// "✅ 任务已完成" completion marker so the user sees the task
	// closed even when no D7 final summary was emitted.
	if !strings.Contains(updateCardData, "✅ 任务已完成") {
		t.Errorf("UpdateCard card.data missing completion marker: %s", updateCardData)
	}
}

// TestFeishuAdapter_FinalizeStructuredSession_SendsSummaryAsSeparateCard
// pins the user-reported fix
// ("最后总结信息不是一个单独的恢复卡片，而是追加到前面的卡片了"):
// when the D7 orchestrator emits a non-empty final summary, the IM
// adapter MUST deliver it as a standalone "任务总结" card (new message)
// rather than gluing it onto the existing response / streaming card with
// a `---` separator. This test exercises the cardkit streaming path
// (the most common case for the user) and asserts:
//   1. the streaming reply card does NOT contain the summary text,
//   2. a new interactive message is sent (replyCount == 2: 1 streaming
//      card + 1 summary card), and
//   3. the streaming card carries the minimal "✅ 任务已完成" completion
//      marker so the user still sees a closed task when a separate
//      summary card follows.
//
// Note: the larkim library stores the request body in the private
// apiReq.Body field (the public Body field is reserved for response
// deserialization), so the test cannot inspect the reply body directly.
// The behavioral assertions above are sufficient: a separate card
// cannot be created without invoking a second Reply, and the absence of
// the summary text from the streaming card's body proves the report and
// summary are no longer glued together.
func TestFeishuAdapter_FinalizeStructuredSession_SendsSummaryAsSeparateCard(t *testing.T) {
	const summary = "4 路并行 deep review 全部 PASS：架构 / 测试 / 安全 / 性能 / 命名 各域无 Critical 问题。"

	var streamingUpdateCardData string
	var summaryReplyCount int
	msgID := "om_summary"

	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			summaryReplyCount++
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	mockAPI := &mockFeishuAPI{
		imAPI: mockImAPI,
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			if !strings.Contains(path, "/elements/") {
				if m, ok := body.(map[string]any); ok {
					if card, ok := m["card"].(map[string]any); ok {
						if data, ok := card["data"].(string); ok {
							streamingUpdateCardData = data
						}
					}
				}
			}
			return &larkcore.ApiResp{StatusCode: 200, RawBody: []byte(`{"code":0}`)}, nil
		},
	}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
		Streaming: FeishuStreamingConfig{Enabled: true},
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_summary_card", feishuReplyContext{userMessageID: "om_root"})

	// Stream a short report so a cardkit reply card is created.
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_summary_card", ChatID: "feishu_oc_1",
		Content: "我已完成 4 路并行 review：架构 / 测试 / 安全 / 性能 / 命名。", Metadata: map[string]string{"event_type": "text"},
	})
	// Emit the complete event with the final summary.
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_summary_card", ChatID: "feishu_oc_1",
		Content: summary, Metadata: map[string]string{"event_type": "complete"},
	})

	// The streaming card must NOT contain the summary text — that would
	// reproduce the original bug (summary glued onto the report card).
	if streamingUpdateCardData != "" && strings.Contains(streamingUpdateCardData, summary) {
		t.Errorf("streaming card contains summary text — should be sent as a separate card. card.data=%s", streamingUpdateCardData)
	}
	// A separate interactive message MUST be sent carrying the summary.
	// replyCount == 2 means: (1) streaming card + (2) summary card.
	if summaryReplyCount != 2 {
		t.Fatalf("summaryReplyCount = %d, want 2 (1 streaming card + 1 summary card)", summaryReplyCount)
	}
	// The streaming card must carry the completion marker so the user
	// sees the task closed even when the summary lives on a separate card.
	if !strings.Contains(streamingUpdateCardData, "✅ 任务已完成") {
		t.Errorf("streaming card missing completion marker: %s", streamingUpdateCardData)
	}
}

// ============================================================================
// DM-20260621-008: LLM text events whose tail duplicates the D7 final
// summary must not leak into the streaming reply card.
//
// User-reported scenario: "飞书思考卡片又有总结数据了，这个不需要，因为
// 最后总结卡片也有." The LLM (minimax M2.7 in particular) emits the
// final summary as ordinary text events during streaming. textBuffer
// therefore ends with the same paragraph that the standalone 任务总结
// card will carry. Without the strip, the user sees the conclusion
// twice — once at the tail of the streaming reply card, once on the
// 任务总结 card.
//
// The tests below cover both the cardkit (streaming) and non-cardkit
// (simple-reply) paths. The strip is keyed off exact-match of the
// trimmed tail; whitespace-only tolerance is built in. Pinning the
// exact-match behavior here so a future fuzzy-match change can't
// silently regress to either under-strip (duplication) or over-strip
// (data loss).
// ============================================================================

// TestFeishuAdapter_FinalizeStructuredSession_LLMTextEndsWithSummary_StripsDuplicate
// pins the cardkit streaming path. The streaming reply card's
// UpdateCard body must contain the report + completion marker, but
// NOT the summary paragraph.
func TestFeishuAdapter_FinalizeStructuredSession_LLMTextEndsWithSummary_StripsDuplicate(t *testing.T) {
	const summary = "4 路并行 deep review 全部 PASS：架构 / 测试 / 安全 / 性能 / 命名 各域无 Critical 问题。"

	var streamingUpdateCardData string
	var summaryReplyCount int
	msgID := "om_summary_strip"

	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			summaryReplyCount++
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	mockAPI := &mockFeishuAPI{
		imAPI: mockImAPI,
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			if !strings.Contains(path, "/elements/") {
				if m, ok := body.(map[string]any); ok {
					if card, ok := m["card"].(map[string]any); ok {
						if data, ok := card["data"].(string); ok {
							streamingUpdateCardData = data
						}
					}
				}
			}
			return &larkcore.ApiResp{StatusCode: 200, RawBody: []byte(`{"code":0}`)}, nil
		},
	}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
		Streaming: FeishuStreamingConfig{Enabled: true},
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_dup_strip", feishuReplyContext{userMessageID: "om_root"})

	// Simulate the LLM streaming the report, then the summary as
	// ordinary text events (the minimax M2.7 pattern that triggered
	// the user-visible duplicate).
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_dup_strip", ChatID: "feishu_oc_1",
		Content: "我已完成 4 路并行 review：架构 / 测试 / 安全 / 性能 / 命名。\n\n", Metadata: map[string]string{"event_type": "text"},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_dup_strip", ChatID: "feishu_oc_1",
		Content: summary, Metadata: map[string]string{"event_type": "text"},
	})
	// Then the D7 complete event with the same summary.
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_dup_strip", ChatID: "feishu_oc_1",
		Content: summary, Metadata: map[string]string{"event_type": "complete"},
	})

	// The streaming card MUST NOT contain the summary text — the
	// strip must have removed the LLM-emitted tail. Without the
	// strip, the user would see the same paragraph twice (once on
	// the streaming card, once on the 任务总结 card).
	if streamingUpdateCardData == "" {
		t.Fatal("UpdateCard card.data was not captured — streaming path did not run")
	}
	if strings.Contains(streamingUpdateCardData, summary) {
		t.Errorf("streaming card still contains the summary text — stripTrailingSummary regression.\ncard.data=%s", streamingUpdateCardData)
	}
	// The report prefix must still be present (the strip targets
	// the tail, not the whole buffer).
	if !strings.Contains(streamingUpdateCardData, "我已完成 4 路并行 review") {
		t.Errorf("streaming card missing report prefix after strip: %s", streamingUpdateCardData)
	}
	// The completion marker must be present so the user still sees
	// the task closed.
	if !strings.Contains(streamingUpdateCardData, "✅ 任务已完成") {
		t.Errorf("streaming card missing completion marker: %s", streamingUpdateCardData)
	}
	// A separate summary card MUST be sent carrying the summary.
	// replyCount == 2 means: (1) streaming card + (2) summary card.
	if summaryReplyCount != 2 {
		t.Fatalf("summaryReplyCount = %d, want 2 (1 streaming card + 1 summary card)", summaryReplyCount)
	}
}

// TestFeishuAdapter_StructuredProgress_LLMTextEndsWithSummary_StripsDuplicate
// pins the non-cardkit (simple-reply) path. The response card must
// still be patched (with report + footer, no summary) so the user
// doesn't see the conclusion twice.
func TestFeishuAdapter_StructuredProgress_LLMTextEndsWithSummary_StripsDuplicate(t *testing.T) {
	const summary = "用时: 8s, 消耗: 1500 tokens, 模型: claude-sonnet-4-6"

	var replyCount int
	var patchCount int

	msgID := "om_response_dup_strip"
	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			replyCount++
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
		patchFunc: func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
			patchCount++
			return &larkim.PatchMessageResp{}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:         "test_app",
		AppSecret:     "test_secret",
		ProgressStyle: progressStyleStructured,
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_simple_dup_strip", feishuReplyContext{userMessageID: "om_root"})

	// Simulate LLM streaming the report and the summary as plain
	// text events (this is what the non-cardkit path accumulates into
	// textBuffer via appendResponseText).
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_simple_dup_strip", ChatID: "feishu_oc_1",
		Content: "已处理 5 个文件。\n\n", Metadata: map[string]string{"event_type": "text"},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_simple_dup_strip", ChatID: "feishu_oc_1",
		Content: summary, Metadata: map[string]string{"event_type": "text"},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_simple_dup_strip", ChatID: "feishu_oc_1",
		Content: summary, Metadata: map[string]string{"event_type": "complete"},
	})

	// 1 reply for the response card + 1 reply for the separate summary card.
	if replyCount != 2 {
		t.Fatalf("replyCount = %d, want 2 (1 response card + 1 separate summary card)", replyCount)
	}
	// The response card MUST be patched with the report (summary
	// stripped) + completion marker. Without the strip fix, the
	// response card stays untouched and the user sees the same
	// paragraph twice.
	if patchCount < 1 {
		t.Fatalf("patchCount = %d, want >=1 (response card must be patched with stripped report + footer)", patchCount)
	}
}

// TestFeishuAdapter_FinalizeStructuredSession_TaskCardDoesNotIncludeSummary
// pins DM-20260621-008 part 2: when a task ("进度") card exists, the
// finalize step must NOT push the LLM's final summary into the card's
// 小结 list. Otherwise the user sees the same conclusion paragraph
// twice — once on the green "任务完成" card, once on the blue "任务总结"
// card. The standalone "任务总结" card is the single owner of the
// conclusion.
//
// We invoke finalizeStructuredSession directly so we can inspect
// stream.summaries mid-finalize. clearSessionStream is what the
// production OnMessage path calls after finalizeStructuredSession
// returns; without that here, the stream stays alive and we can
// assert what buildTaskProgressCard would have rendered for the
// finalize-completed=true patch.
func TestFeishuAdapter_FinalizeStructuredSession_TaskCardDoesNotIncludeSummary(t *testing.T) {
	const summary = "deep review 全部 PASS：4 路并行无 Critical 问题。"
	const workerLine = "[code-reviewer/w1] reading guard sources"

	var summaryReplyCount int
	var patchCount int

	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			summaryReplyCount++
			msgID := "om_msg"
			return &larkim.ReplyMessageResp{
				Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
			}, nil
		},
		patchFunc: func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
			patchCount++
			return &larkim.PatchMessageResp{}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:         "test_app",
		AppSecret:     "test_secret",
		ProgressStyle: progressStyleStructured,
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_no_dup", feishuReplyContext{userMessageID: "om_root"})

	// Streaming-time worker progress summary (legit 小结 content).
	// First worker_progress creates the progress card via ReplyMessage;
	// the second one re-patches it (this is when PatchMessage first fires
	// in the test trace, and where the regression — pushing the D7
	// summary into stream.summaries — would surface in the card body).
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_no_dup", ChatID: "feishu_oc_1",
		Content: "reading guard sources",
		Metadata: map[string]string{
			"event_type": "worker_progress",
			"role":       "code-reviewer",
			"worker_id":  "w1",
			"kind":       "started",
		},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_no_dup", ChatID: "feishu_oc_1",
		Content: "reading guard sources",
		Metadata: map[string]string{
			"event_type": "worker_progress",
			"role":       "code-reviewer",
			"worker_id":  "w1",
			"kind":       "in_progress",
		},
	})

	// Snapshot the stream BEFORE finalize so we can compare.
	preStream, _ := adapter.sessionStreams.Load("sess_no_dup")
	pre := preStream.(*feishuSessionStream)
	pre.mu.Lock()
	preSummaries := append([]string(nil), pre.summaries...)
	pre.mu.Unlock()
	if len(preSummaries) != 1 || preSummaries[0] != workerLine {
		t.Fatalf("pre-finalize summaries = %v, want [%s]", preSummaries, workerLine)
	}

	// Invoke finalizeStructuredSession directly. clearSessionStream
	// (the production OnMessage step) is NOT called here, so the
	// stream stays alive for inspection after.
	if err := adapter.finalizeStructuredSession(context.Background(), "sess_no_dup", "feishu_oc_1", summary, ""); err != nil {
		t.Fatalf("finalizeStructuredSession: %v", err)
	}

	// Behavior: 2 patches (worker_progress #1 creates via Reply, #2
	// patches, finalize patches again with completed=true) + 2 replies
	// (worker_progress #1's initial Reply for the progress card +
	// finalize's standalone summary Reply).
	if patchCount != 2 {
		t.Errorf("patchCount = %d, want 2 (1 worker_progress re-patch + 1 finalize completed=true patch)", patchCount)
	}
	if summaryReplyCount != 2 {
		t.Errorf("summaryReplyCount = %d, want 2 (1 progress card + 1 summary card)", summaryReplyCount)
	}

	// State: stream.summaries after finalize MUST NOT contain the D7
	// final summary. This is the direct regression guard for
	// `stream.summaries = append(stream.summaries, trimmedSummary)`.
	// buildTaskProgressCard reads stream.summaries to render the "小结"
	// list — if the summary were here, the patched progress card would
	// duplicate the LLM's final paragraph.
	postStream, ok := adapter.sessionStreams.Load("sess_no_dup")
	if !ok {
		t.Fatal("session stream was cleared during finalize — test cannot verify state")
	}
	post := postStream.(*feishuSessionStream)
	post.mu.Lock()
	postSummaries := append([]string(nil), post.summaries...)
	post.mu.Unlock()
	for _, s := range postSummaries {
		if s == summary {
			t.Errorf("regression: stream.summaries contains D7 final summary after finalize.\nsummaries=%v", postSummaries)
		}
	}
	if len(postSummaries) != len(preSummaries) {
		t.Errorf("stream.summaries length changed: pre=%d post=%d (finalize must not append)", len(preSummaries), len(postSummaries))
	}

	// Rendering: buildTaskProgressCard on the post-finalize stream
	// produces the exact body that was patched onto the progress card.
	// Pin it so the user-visible card cannot regress to carry the D7
	// summary.
	card := buildTaskProgressCard(post, true, "")
	cardBody := BuildCardJSON(card)
	if strings.Contains(cardBody, summary) {
		t.Errorf("rendered progress card body contains D7 final summary — DM-20260621-008 regression.\nbody=%s", cardBody)
	}
	if !strings.Contains(cardBody, workerLine) {
		t.Errorf("rendered progress card body missing worker progress summary.\nbody=%s", cardBody)
	}
}

// ============================================================================
// DM-20260626-009 follow-up: chunk-self and buffer-self dedup tests were
// removed. The dedup they exercised was a fragile band-aid for minimax
// M2.7 streaming replay and was deleted from feishu_progress.go. Pinning
// the OLD behaviour (force-collapse duplicate chunks) would re-introduce
// the bug the dedup removal fixed: natural Chinese repetition in legitimate
// LLM output was getting false-positively truncated.
//
// What remains: detectDuplicateReplay as the streaming-time guard (the
// tests at TestSendStructuredThinkingCard_DropsReplayChunk,
// TestFeishuAdapter_AppendResponseText_DropsReplayedChunk,
// TestDetectDuplicateReplay, TestDetectDuplicateReplay_ShortBufferSkips,
// TestDetectDuplicateReplay_NaturalRepetitionPassesThrough all assert
// the conservative prefix-overlap-only heuristic). Anything that slips
// past detectDuplicateReplay (suffix-overlap, in-chunk self-duplication)
// now flows through verbatim — accepted trade-off documented at
// feishu_progress.go:appendResponseText.
// ============================================================================
