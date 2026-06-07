package adapters

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/core"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestStripOuterCodeFence(t *testing.T) {
	input := "```go\nfunc main() {}\n```"
	got := stripOuterCodeFence(input)
	if got != "func main() {}" {
		t.Fatalf("stripOuterCodeFence() = %q, want func main() {}", got)
	}
}

func TestBuildCoalescedProgressCard_IncludesMilestoneTask(t *testing.T) {
	items := []progressItem{
		{kind: progressKindThinking, text: "思考中..."},
		{kind: progressKindMilestone, progress: "50%", task: "读取代码文件"},
	}
	card := buildCoalescedProgressCard(items, progressStyleCard, false)
	if card.Header == nil || card.Header.Title != "Devrix 处理中" {
		t.Fatalf("header title = %#v", card.Header)
	}
	body := cardBodyMarkdown(card)
	if !strings.Contains(body, "50%") || !strings.Contains(body, "读取代码文件") {
		t.Fatalf("milestone task missing from card body: %q", body)
	}
}

func TestFeishuAdapter_CoalescedProgress_ReplyOncePatchMany(t *testing.T) {
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
		ProgressStyle: progressStyleCard,
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_1", feishuReplyContext{userMessageID: "om_root"})

	events := []types.OutboundMessage{
		{SessionID: "sess_1", ChatID: "feishu_oc_123456_ou_654321", Content: "思考中...", Metadata: map[string]string{"event_type": "thinking"}},
		{SessionID: "sess_1", ChatID: "feishu_oc_123456_ou_654321", Content: "read", Metadata: map[string]string{"event_type": "tool_call", "tool_name": "read"}},
		{SessionID: "sess_1", ChatID: "feishu_oc_123456_ou_654321", Content: "ok", Metadata: map[string]string{"event_type": "tool_result", "tool_name": "read"}},
		{SessionID: "sess_1", ChatID: "feishu_oc_123456_ou_654321", Metadata: map[string]string{"event_type": "milestone_progress", "progress": "50%", "task": "读取代码文件"}},
	}
	for i := range events {
		adapter.OnMessage(&events[i])
	}

	if replyCount != 1 {
		t.Fatalf("replyCount = %d, want 1", replyCount)
	}
	if patchCount != 3 {
		t.Fatalf("patchCount = %d, want 3", patchCount)
	}
}

func TestNormalizeProgressStyle_DefaultStructured(t *testing.T) {
	if got := normalizeProgressStyle(""); got != progressStyleStructured {
		t.Fatalf("normalizeProgressStyle(\"\") = %q, want %q", got, progressStyleStructured)
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

	// thinking + tool + task progress create = 3 replies
	if replyCount != 3 {
		t.Fatalf("replyCount = %d, want 3", replyCount)
	}
	// info + complete patch the same task progress card
	if patchCount != 2 {
		t.Fatalf("patchCount = %d, want 2", patchCount)
	}
}

func cardBodyMarkdown(card *core.Card) string {
	var parts []string
	for _, elem := range card.Elements {
		if md, ok := elem.(core.CardMarkdown); ok {
			parts = append(parts, md.Content)
		}
	}
	return strings.Join(parts, "\n")
}
