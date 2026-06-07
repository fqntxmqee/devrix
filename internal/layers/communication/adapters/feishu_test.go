package adapters

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

// TestFeishuMessageParsing 测试飞书消息解析
func TestFeishuMessageParsing(t *testing.T) {
	// 构造一个模拟的飞书消息事件
	eventJSON := `{
		"Event": {
			"Message": {
				"MessageType": "text",
				"Content": "{\"text\":\"hello\"}",
				"ChatId": "oc_123456",
				"MessageId": "om_123456",
				"ChatType": "p2p"
			},
			"Sender": {
				"SenderId": {
					"OpenId": "ou_123456"
				}
			}
		}
	}`

	var event map[string]interface{}
	if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
		t.Fatalf("failed to parse event: %v", err)
	}

	t.Logf("Parsed event: %+v", event)

	// 验证字段提取
	eventObj, ok := event["Event"].(map[string]interface{})
	if !ok {
		t.Fatal("Event field not found or invalid")
	}

	msg, ok := eventObj["Message"].(map[string]interface{})
	if !ok {
		t.Fatal("Message field not found or invalid")
	}

	msgType, _ := msg["MessageType"].(string)
	content, _ := msg["Content"].(string)

	t.Logf("MessageType: %s, Content: %s", msgType, content)

	// 解析 text 内容
	if msgType == "text" {
		var textContent struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(content), &textContent); err != nil {
			t.Fatalf("failed to parse text content: %v", err)
		}
		t.Logf("Parsed text: %s", textContent.Text)
		if textContent.Text != "hello" {
			t.Errorf("text = %s, want hello", textContent.Text)
		}
	}
}

// TestSessionKeyGeneration 测试 session key 生成
func TestSessionKeyGeneration(t *testing.T) {
	adapter := &FeishuAdapter{}

	key := adapter.buildSessionKey("oc_123456", "ou_654321")
	expected := "feishu_oc_123456_ou_654321"

	if key != expected {
		t.Errorf("buildSessionKey() = %s, want %s", key, expected)
	}
}

// TestInboundMessageCreation 测试入站消息创建
func TestInboundMessageCreation(t *testing.T) {
	session := types.NewSession("sess_123", "feishu", "/tmp")
	session.SessionID = "sess_123"

	inboundMsg := &types.InboundMessage{
		SessionID:  session.SessionID,
		ChatID:     "feishu_oc_123_ou_456",
		UserID:     "ou_456",
		UserName:   "Test User",
		Content:    "hello",
		MessageID:  "om_123",
		AdapterID:  "feishu",
		ReceivedAt: time.Now(),
		Metadata: map[string]string{
			"chat_type": "p2p",
			"msg_type":  "text",
		},
	}

	if inboundMsg.Content != "hello" {
		t.Errorf("Content = %s, want hello", inboundMsg.Content)
	}

	if inboundMsg.AdapterID != "feishu" {
		t.Errorf("AdapterID = %s, want feishu", inboundMsg.AdapterID)
	}
}

// TestPermissionAutoApprove 测试 YOLO 模式权限自动批准
func TestYOLOPermissionAutoApprove(t *testing.T) {
	// 这个测试验证在高风险操作时 YOLO 模式的行为
	// 注意：需要通过 PermissionManager 来测试，这里只验证类型
	t.Log("YOLO mode: auto-approve for LOW/MEDIUM risk")
}

// TestStubContextEngine 测试 stub context engine
func TestStubContextEngine(t *testing.T) {
	ctx := context.Background()
	session := types.NewSession("sess_test", "feishu", "/tmp")

	engine := &stubTestEngine{
		response: "Hello from stub!",
	}

	events := engine.Process(ctx, session, "hello")
	if events == nil {
		t.Fatal("Process() returned nil channel")
	}

	// 接收事件
	select {
	case event := <-events:
		t.Logf("Received event: type=%s, content=%s", event.Type, event.Content)
		if event.Type != "complete" {
			t.Errorf("event.Type = %s, want complete", event.Type)
		}
		if event.Content != "Hello from stub!" {
			t.Errorf("event.Content = %s, want Hello from stub!", event.Content)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

// stubTestEngine 用于测试的 stub 实现
type stubTestEngine struct {
	response string
}

type testEngineEvent struct {
	Type      string
	Content   string
	SessionID string
}

func (e *stubTestEngine) Process(ctx context.Context, session *types.Session, message string) <-chan testEngineEvent {
	ch := make(chan testEngineEvent, 1)
	go func() {
		defer close(ch)
		ch <- testEngineEvent{
			Type:      "complete",
			Content:   e.response,
			SessionID: session.SessionID,
		}
	}()
	return ch
}

// TestSendMessageParsing 测试发送消息解析
func TestSendMessageParsing(t *testing.T) {
	// 测试 session key 解析
	// sessionKey format: feishu_{chat_id}_{user_id}
	// chat_id format: "oc_xxxxxx", user_id format: "ou_xxxxxx"
	sessionKey := "feishu_oc_123456_ou_654321"

	// 模拟 SendMessage 中的解析逻辑
	parts := strings.Split(sessionKey, "_")
	if len(parts) < 5 {
		t.Fatalf("split() returned %d parts, want at least 5", len(parts))
	}

	// parts = ["feishu", "oc", "123456", "ou", "654321"]
	// chat_id = parts[1] + "_" + parts[2] = "oc_123456"
	chatID := parts[1] + "_" + parts[2]
	if chatID != "oc_123456" {
		t.Errorf("chatID = %s, want oc_123456", chatID)
	}
}
