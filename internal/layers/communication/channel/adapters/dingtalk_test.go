package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestParseDingTalkPayload(t *testing.T) {
	raw := `{
		"msgtype":"text",
		"conversationId":"cid-1",
		"senderNick":"alice",
		"msgId":"m-1",
		"sessionWebhook":"https://example.com/hook",
		"text":{"content":"hello"}
	}`
	payload, err := parseDingTalkPayload([]byte(raw))
	if err != nil {
		t.Fatalf("parseDingTalkPayload() error = %v", err)
	}
	if payload.ConversationID != "cid-1" || payload.Text.Content != "hello" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

// T: D1-S2-A01-T02
func TestDingTalkWebhookHandler_routesInbound(t *testing.T) {
	var routed bool
	mockGW := &mockGatewayAPI{
		createSessionFunc: func(chatID, workDir string) (*types.Session, error) {
			return &types.Session{SessionID: "sess-1", ChatID: chatID}, nil
		},
		routeInboundFunc: func(_ context.Context, msg *types.InboundMessage) error {
			routed = true
			if msg.Content != "ping" {
				t.Fatalf("content = %q", msg.Content)
			}
			return nil
		},
	}

	adapter := NewDingTalkAdapter(nil, &DingTalkConfig{
		AppKey: "k", AppSecret: "s", CallbackPath: "/dingtalk/webhook",
	}, config.DefaultConfig(), WithDingTalkGateway(mockGW), WithDingTalkAPI(&mockDingTalkAPI{}))

	body, _ := json.Marshal(map[string]any{
		"msgtype":        "text",
		"conversationId": "cid-1",
		"senderNick":     "bob",
		"msgId":          "mid-1",
		"sessionWebhook": "https://example.com/hook",
		"text":           map[string]string{"content": "ping"},
	})
	req := httptest.NewRequest(http.MethodPost, "/dingtalk/webhook", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	adapter.webhookHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !routed {
		t.Fatal("expected RouteInbound to be called")
	}
}

// T: D1-S2-A01-T02
func TestDingTalkAdapter_OnMessage_sendsViaWebhook(t *testing.T) {
	mockAPI := &mockDingTalkAPI{}
	adapter := NewDingTalkAdapter(nil, &DingTalkConfig{}, config.DefaultConfig(), WithDingTalkAPI(mockAPI))
	adapter.webhookMap.Store("cid-1", "https://example.com/hook")

	adapter.OnMessage(&types.OutboundMessage{
		ChatID:  "cid-1",
		Content: "pong",
	})

	if len(mockAPI.sendCalls) != 1 {
		t.Fatalf("sendCalls = %d", len(mockAPI.sendCalls))
	}
	if mockAPI.sendContents[0] != "pong" {
		t.Fatalf("content = %q", mockAPI.sendContents[0])
	}
}

func TestDingTalkAdapter_Start_prefetchesToken(t *testing.T) {
	mockAPI := &mockDingTalkAPI{}
	adapter := NewDingTalkAdapter(nil, &DingTalkConfig{
		AppKey: "k", AppSecret: "s", Port: "0", CallbackPath: "/dingtalk/webhook",
	}, config.DefaultConfig(), WithDingTalkAPI(mockAPI))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if mockAPI.tokenCalls != 1 {
		t.Fatalf("tokenCalls = %d", mockAPI.tokenCalls)
	}
	_ = adapter.Stop()
}

// T: D1-S2-A02-T03
func TestDingTalkAdapter_OnMessage_milestoneRender(t *testing.T) {
	mockAPI := &mockDingTalkAPI{}
	adapter := NewDingTalkAdapter(nil, &DingTalkConfig{}, config.DefaultConfig(), WithDingTalkAPI(mockAPI))
	adapter.webhookMap.Store("cid-1", "https://example.com/hook")

	adapter.OnMessage(&types.OutboundMessage{
		ChatID:  "cid-1",
		Content: "raw should not be sent",
		Metadata: map[string]string{
			"render":             "milestone",
			"milestone_id":       "m1",
			"milestone_name":     "Design API",
			"milestone_status":   "in_progress",
			"milestone_progress": "0.5",
		},
	})

	if len(mockAPI.sendContents) != 1 {
		t.Fatalf("sendContents = %d", len(mockAPI.sendContents))
	}
	sent := mockAPI.sendContents[0]
	if sent == "raw should not be sent" {
		t.Fatal("expected rendered milestone card, got raw content")
	}
	if !strings.Contains(sent, "Design API") {
		t.Fatalf("content = %q", sent)
	}
}

func TestRenderDingTalkOutboundContent_milestoneProgressEvent(t *testing.T) {
	out := renderDingTalkOutboundContent(&types.OutboundMessage{
		Content: "ignored",
		Metadata: map[string]string{
			"event_type": "milestone_progress",
			"task":       "Fix auth",
			"progress":   "50%",
		},
	})
	if out == "" || !strings.Contains(out, "Fix auth") {
		t.Fatalf("content = %q", out)
	}
}
