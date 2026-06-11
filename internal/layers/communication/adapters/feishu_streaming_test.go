package adapters

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestFeishuAdapter_StreamingReply_UsesCardkit(t *testing.T) {
	var putPaths []string
	msgID := "om_stream"
	mockAPI := &mockFeishuAPI{
		postFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			return &larkcore.ApiResp{
				StatusCode: http.StatusOK,
				RawBody:    []byte(`{"code":0,"data":{"card_id":"card_stream_1"}}`),
			}, nil
		},
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			putPaths = append(putPaths, path)
			return &larkcore.ApiResp{StatusCode: http.StatusOK, RawBody: []byte(`{"code":0}`)}, nil
		},
		imAPI: &mockImAPI{
			messageAPI: &mockMessageAPI{
				replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
					return &larkim.ReplyMessageResp{
						Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
					}, nil
				},
			},
			messageReactionAPI: &mockMessageReactionAPI{},
		},
	}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
		Streaming: FeishuStreamingConfig{Enabled: true, IntervalMs: 0, MinDeltaChars: 1},
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_stream", feishuReplyContext{userMessageID: "om_root"})

	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_stream", ChatID: "feishu_oc_123456_ou_654321",
		Content: "hello", Metadata: map[string]string{"event_type": "text"},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_stream", ChatID: "feishu_oc_123456_ou_654321",
		Content: " world", Metadata: map[string]string{"event_type": "text"},
	})

	stream := adapter.sessionStream("sess_stream")
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if !stream.cardkitEnabled || stream.replyCardID != "card_stream_1" {
		t.Fatalf("cardkit not enabled: enabled=%v cardID=%q", stream.cardkitEnabled, stream.replyCardID)
	}
	if len(putPaths) == 0 {
		t.Fatal("expected cardkit PUT calls")
	}
	foundElement := false
	for _, p := range putPaths {
		if strings.Contains(p, "/elements/"+replyTextElementID+"/content") {
			foundElement = true
		}
	}
	if !foundElement {
		t.Fatalf("put paths = %v", putPaths)
	}
}

func TestFeishuAdapter_StreamingDisabled_UsesPatch(t *testing.T) {
	putCount := 0
	patchCount := 0
	msgID := "om_patch"
	mockAPI := &mockFeishuAPI{
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			putCount++
			return &larkcore.ApiResp{StatusCode: http.StatusOK, RawBody: []byte(`{"code":0}`)}, nil
		},
		imAPI: &mockImAPI{
			messageAPI: &mockMessageAPI{
				replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
					return &larkim.ReplyMessageResp{
						Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
					}, nil
				},
				patchFunc: func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
					patchCount++
					return &larkim.PatchMessageResp{}, nil
				},
			},
			messageReactionAPI: &mockMessageReactionAPI{},
		},
	}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
		Streaming: FeishuStreamingConfig{Enabled: false},
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_patch", feishuReplyContext{userMessageID: "om_root"})

	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_patch", ChatID: "feishu_oc_123456_ou_654321",
		Content: "a", Metadata: map[string]string{"event_type": "text"},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_patch", ChatID: "feishu_oc_123456_ou_654321",
		Content: "b", Metadata: map[string]string{"event_type": "text"},
	})

	if putCount != 0 {
		t.Fatalf("putCount = %d, want 0", putCount)
	}
	if patchCount != 1 {
		t.Fatalf("patchCount = %d, want 1", patchCount)
	}
}

func TestFeishuAdapter_Complete_ClosesStreamingMode(t *testing.T) {
	var lastUpdateBody map[string]any
	mockAPI := &mockFeishuAPI{
		postFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			return &larkcore.ApiResp{
				StatusCode: http.StatusOK,
				RawBody:    []byte(`{"code":0,"data":{"card_id":"card_done"}}`),
			}, nil
		},
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			if strings.HasSuffix(path, "/content") {
				return &larkcore.ApiResp{StatusCode: http.StatusOK, RawBody: []byte(`{"code":0}`)}, nil
			}
			if m, ok := body.(map[string]any); ok && strings.Contains(path, "card_done") && !strings.Contains(path, "elements") {
				lastUpdateBody = m
			}
			return &larkcore.ApiResp{StatusCode: http.StatusOK, RawBody: []byte(`{"code":0}`)}, nil
		},
		imAPI: &mockImAPI{
			messageAPI: &mockMessageAPI{
				replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
					id := "om_done"
					return &larkim.ReplyMessageResp{Data: &larkim.ReplyMessageRespData{MessageId: &id}}, nil
				},
			},
			messageReactionAPI: &mockMessageReactionAPI{},
		},
	}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
		Streaming: FeishuStreamingConfig{Enabled: true, IntervalMs: 0, MinDeltaChars: 1},
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_done", feishuReplyContext{userMessageID: "om_root"})

	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_done", ChatID: "feishu_oc_123456_ou_654321",
		Content: "answer", Metadata: map[string]string{"event_type": "text"},
	})
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_done", ChatID: "feishu_oc_123456_ou_654321",
		Content: "用时: 1s, 消耗: 0 tokens", Metadata: map[string]string{"event_type": "complete"},
	})

	if lastUpdateBody == nil {
		t.Fatal("expected final cardkit card update")
	}
	card, _ := lastUpdateBody["card"].(map[string]any)
	data, _ := card["data"].(string)
	if !strings.Contains(data, `"streaming_mode":false`) && !strings.Contains(data, `"streaming_mode": false`) {
		t.Fatalf("final card should disable streaming: %q", data)
	}
}

func TestStreamThrottle_RespectsInterval(t *testing.T) {
	cfg := streamThrottleConfig{Enabled: true, Interval: time.Second, MinDeltaRunes: 1}
	last := time.Now().Add(-100 * time.Millisecond)
	if cfg.shouldFlush(last, 0, 10, false) {
		t.Fatal("should not flush before interval")
	}
	if !cfg.shouldFlush(last.Add(-2*time.Second), 0, 10, false) {
		t.Fatal("should flush after interval")
	}
	if !cfg.shouldFlush(last, 0, 10, true) {
		t.Fatal("force should always flush")
	}
}

func TestFeishuUserConfig_ResolveStreamingDefaults(t *testing.T) {
	enabled, interval, delta := (config.FeishuUserConfig{}).ResolveFeishuStreamingConfig()
	if !enabled || interval != 400 || delta != 24 {
		t.Fatalf("defaults = %v %d %d", enabled, interval, delta)
	}
	disabled := false
	enabled, _, _ = (config.FeishuUserConfig{Streaming: config.FeishuStreamingUserConfig{Enabled: &disabled}}).ResolveFeishuStreamingConfig()
	if enabled {
		t.Fatal("expected disabled")
	}
}

// Covers: DM-20260611-006 T14 — tool cards use Im.Message.Patch, not cardkit sequence.
func TestFeishuAdapter_ToolCardPatch_DoesNotAffectCardkitSequence(t *testing.T) {
	var putPaths []string
	imOutbound := 0
	msgID := "om_tool_seq"
	mockAPI := &mockFeishuAPI{
		postFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			return &larkcore.ApiResp{
				StatusCode: http.StatusOK,
				RawBody:    []byte(`{"code":0,"data":{"card_id":"card_tool_seq"}}`),
			}, nil
		},
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			putPaths = append(putPaths, path)
			return &larkcore.ApiResp{StatusCode: http.StatusOK, RawBody: []byte(`{"code":0}`)}, nil
		},
		imAPI: &mockImAPI{
			messageAPI: &mockMessageAPI{
				replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
					imOutbound++
					return &larkim.ReplyMessageResp{
						Data: &larkim.ReplyMessageRespData{MessageId: &msgID},
					}, nil
				},
				patchFunc: func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
					imOutbound++
					return &larkim.PatchMessageResp{}, nil
				},
			},
			messageReactionAPI: &mockMessageReactionAPI{},
		},
	}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
		Streaming: FeishuStreamingConfig{Enabled: true, IntervalMs: 0, MinDeltaChars: 1},
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))
	adapter.sessionReplyCtx.Store("sess_tool_seq", feishuReplyContext{userMessageID: "om_root"})

	// Start reply card via cardkit.
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_tool_seq", ChatID: "feishu_oc_123456_ou_654321",
		Content: "hello", Metadata: map[string]string{"event_type": "text"},
	})

	stream := adapter.sessionStream("sess_tool_seq")
	stream.mu.Lock()
	seqAfterReply := stream.cardkitSequence
	putCountBeforeTool := len(putPaths)
	stream.mu.Unlock()
	if seqAfterReply == 0 {
		t.Fatal("expected cardkit sequence after first reply chunk")
	}

	// Tool card uses Im API — must not touch cardkitSequence or add reply element PUTs.
	imBeforeTool := imOutbound
	adapter.OnMessage(&types.OutboundMessage{
		SessionID: "sess_tool_seq", ChatID: "feishu_oc_123456_ou_654321",
		Content: "read_file", Metadata: map[string]string{"event_type": "tool_call", "tool_name": "read_file"},
	})

	stream.mu.Lock()
	seqAfterTool := stream.cardkitSequence
	putCountAfterTool := len(putPaths)
	stream.mu.Unlock()
	if seqAfterTool != seqAfterReply {
		t.Fatalf("tool handling changed cardkitSequence: before=%d after=%d", seqAfterReply, seqAfterTool)
	}
	if imOutbound <= imBeforeTool {
		t.Fatal("expected Im API outbound for tool card (reply or patch)")
	}
	for _, p := range putPaths[putCountBeforeTool:putCountAfterTool] {
		if strings.Contains(p, "/elements/"+replyTextElementID) {
			t.Fatalf("tool handling issued reply cardkit PUT: %s", p)
		}
	}
}
