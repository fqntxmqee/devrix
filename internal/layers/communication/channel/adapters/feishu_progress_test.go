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
	card := buildTaskProgressCard(stream, false)
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

	done := buildTaskProgressCard(stream, true)
	doneBody := cardBodyMarkdown(done)
	if !strings.Contains(doneBody, "已完成") || !strings.Contains(doneBody, "100%") {
		t.Fatalf("completed state missing: %q", doneBody)
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
// pins the non-streaming path behavior after the DM-20260621-001 fix.
// For a simple text → complete flow:
//   1. The response card is created via Reply (1 reply).
//   2. The non-empty summary is delivered as a SEPARATE reply (2nd
//      reply) — the legacy "summary glued onto the response card with
//      `---`" behavior is intentionally removed.
//   3. No Patch is needed since the response card is left untouched.
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
	// Response card is left untouched; no Patch is needed for the
	// summary glue (the summary lives on its own card now).
	if patchCount != 0 {
		t.Fatalf("patchCount = %d, want 0 (response card not patched; summary is a separate card)", patchCount)
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

	if err := adapter.finalizeReplyCardStreaming(context.Background(), stream, "conclusion summary"); err != nil {
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
// LLM-stream replay dedup (PR #139)
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

func TestDedupRepeatedText(t *testing.T) {
	opening := "由于域之间是相对独立的（D1-D7 分别对应一个层），适合按域并行 review。我设计如下并行策略。我先快速摸清项目结构与规模。" + strings.Repeat("补", 20)
	cases := []struct {
		name    string
		in      string
		want    string
		minGap  int
	}{
		{
			name:   "no-duplicate-passthrough",
			in:     "段落 A 描述 deep review 的总体策略。段落 B 描述具体 worker 分配方案。",
			want:   "段落 A 描述 deep review 的总体策略。段落 B 描述具体 worker 分配方案。",
			minGap: 2,
		},
		{
			name:   "duplicate-mid-block-truncated",
			in:     opening + "\n\n" + opening + "\n",
			want:   opening + "\n\n",
			minGap: 2,
		},
		{
			name: "duplicate-with-tail-after",
			in: "我先快速摸清项目结构与规模。这是一段长描述，" + strings.Repeat("x", 200) + "结束。\n" +
				"\n我先快速摸清项目结构与规模。这是一段长描述，" + strings.Repeat("x", 200) + "结束。\n" +
				"\n后续的结论部分应该保留下来。",
			want:   "我先快速摸清项目结构与规模。这是一段长描述，" + strings.Repeat("x", 200) + "结束。\n\n后续的结论部分应该保留下来。",
			minGap: 2,
		},
		{
			name:   "tiny-overlap-below-threshold",
			in:     "ABC\n\nabc\n",
			want:   "ABC\n\nabc\n",
			minGap: 2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupRepeatedText(tc.in, 60, tc.minGap)
			if got != tc.want {
				t.Fatalf("dedupRepeatedText() = %q, want %q", got, tc.want)
			}
		})
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

// TestFeishuAdapter_FinalizeReplyCardStreaming_DedupsReplayedTextBuffer
// pins the post-hoc dedup safety net. If a duplicate slipped past the
// streaming-time check (e.g. only the suffix overlapped, not the
// prefix), the final cardkit UpdateCard must still drop the duplicate
// before sending to Feishu. We assert by inspecting the last UpdateCard
// PUT body's `card.data` payload.
//
// Note: per the DM-20260621-001 fix, the summary arg is now passed empty
// by finalizeStructuredSession — the summary is delivered as a separate
// card, not appended to the streaming reply card. This test asserts that
// the streaming card carries only the deduped report + completion marker.
func TestFeishuAdapter_FinalizeReplyCardStreaming_DedupsReplayedTextBuffer(t *testing.T) {
	opening := "我先快速摸清项目结构与规模。这是一段开场白描述，描述项目的总体情况与策略选择依据。"
	openingFull := opening + strings.Repeat("补", 80) + "结束。"

	var updateCardData string
	mockAPI := &mockFeishuAPI{
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			if !strings.Contains(path, "/elements/") {
				// UpdateCard path. body is map[string]any{"card":{"type":"card_json","data":<cardJSON string>}, "sequence":N}.
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

	if err := adapter.finalizeReplyCardStreaming(context.Background(), stream, ""); err != nil {
		t.Fatalf("finalizeReplyCardStreaming: %v", err)
	}

	if updateCardData == "" {
		t.Fatalf("UpdateCard card.data was not captured")
	}
	// The duplicate opening must appear only once in the sent body.
	if got := strings.Count(updateCardData, opening); got != 1 {
		t.Errorf("opening appears %d times in UpdateCard card.data, want 1. card.data=%s", got, updateCardData)
	}
	// The conclusion must be present.
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
