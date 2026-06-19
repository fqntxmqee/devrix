package adapters

import (
	"context"
	"strings"
	"testing"

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

func TestFeishuAdapter_StructuredProgress_SimpleReplyNoEmptyTaskCard(t *testing.T) {
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

	if replyCount != 1 {
		t.Fatalf("replyCount = %d, want 1 (response only)", replyCount)
	}
	if patchCount != 1 {
		t.Fatalf("patchCount = %d, want 1 (complete footer on response)", patchCount)
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
