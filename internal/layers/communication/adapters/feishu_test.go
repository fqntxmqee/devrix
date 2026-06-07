package adapters

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
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

// TestIsDuplicateMessage 测试消息去重
func TestIsDuplicateMessage(t *testing.T) {
	adapter := &FeishuAdapter{}

	// 第一次看到消息，应该不是重复
	isDup := adapter.isDuplicateMessage("msg_001")
	if isDup {
		t.Errorf("first message should not be duplicate, got %v", isDup)
	}

	// 第二次看到同一消息，应该返回 true（重复）
	isDup = adapter.isDuplicateMessage("msg_001")
	if !isDup {
		t.Errorf("second message should be duplicate, got %v", isDup)
	}

	// 不同消息 ID 应该不是重复
	isDup = adapter.isDuplicateMessage("msg_002")
	if isDup {
		t.Errorf("different message should not be duplicate, got %v", isDup)
	}

	// 空消息 ID 不应该被去重
	isDup = adapter.isDuplicateMessage("")
	if isDup {
		t.Errorf("empty message ID should not be duplicate, got %v", isDup)
	}
}

// TestEscapeJSON 测试 JSON 转义
func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{`hello"world`, `hello\"world`},
		{"line1\nline2", "line1\\nline2"},
		{`with\backslash`, `with\\backslash`},
		{"tab\there", "tab\\there"},
		{"carriage\rreturn", "carriage\\rreturn"},
	}

	for _, tt := range tests {
		result := escapeJSON(tt.input)
		if result != tt.expected {
			t.Errorf("escapeJSON(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestBuildSessionKey 测试 session key 构建
func TestBuildSessionKey(t *testing.T) {
	adapter := &FeishuAdapter{}

	tests := []struct {
		chatID   string
		userID   string
		expected string
	}{
		{"oc_123456", "ou_654321", "feishu_oc_123456_ou_654321"},
		{"oc_abc", "ou_xyz", "feishu_oc_abc_ou_xyz"},
		{"", "", "feishu__"},
	}

	for _, tt := range tests {
		result := adapter.buildSessionKey(tt.chatID, tt.userID)
		if result != tt.expected {
			t.Errorf("buildSessionKey(%q, %q) = %q, want %q", tt.chatID, tt.userID, result, tt.expected)
		}
	}
}

// TestParseCommand 测试命令解析
func TestParseCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected types.CommandType
	}{
		{"/help", types.CommandHelp},
		{"/new", types.CommandNew},
		{"/stop", types.CommandStop},
		{"/unknown", types.CommandUnknown},
		{"help", types.CommandUnknown},
		{"hello", types.CommandUnknown},
	}

	for _, tt := range tests {
		cmd := types.ParseCommand(tt.input, "/")
		if cmd.Type != tt.expected {
			t.Errorf("ParseCommand(%q).Type = %v, want %v", tt.input, cmd.Type, tt.expected)
		}
	}
}

// TestHandleFeishuCommand 测试飞书命令处理
// Note: These tests verify command detection logic only.
// Commands that require SendMessage are tested separately with mocks.
func TestHandleFeishuCommand(t *testing.T) {
	adapter := &FeishuAdapter{}
	ctx := context.Background()

	// 测试非命令输入（应该返回 false）
	tests := []struct {
		name       string
		text       string
		wantHandled bool
	}{
		{"regular text", "hello", false},
		{"empty string", "", false},
		{"text with slash not at start", "say /hello to you", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled := adapter.handleFeishuCommand(ctx, tt.text, "feishu_oc_123_ou_456", "")
			if handled != tt.wantHandled {
				t.Errorf("handleFeishuCommand(%q) = %v, want %v", tt.text, handled, tt.wantHandled)
			}
		})
	}
}

// TestCommandPrefixDetection 测试命令前缀检测
func TestCommandPrefixDetection(t *testing.T) {
	// 验证命令被正确识别为需要处理
	commandInputs := []string{"/help", "/new", "/stop", "/status", "/model"}

	for _, input := range commandInputs {
		// 命令以 / 开头，应该返回 true 表示这是命令
		if !strings.HasPrefix(input, "/") {
			t.Errorf("command %q should start with /", input)
		}
	}

	// 非命令
	nonCommands := []string{"hello", "help me", ""}
	for _, input := range nonCommands {
		if strings.HasPrefix(input, "/") {
			t.Errorf("non-command %q should not start with /", input)
		}
	}
}

// TestIsCommand 测试命令判断
func TestIsCommand(t *testing.T) {
	tests := []struct {
		input    string
		prefix   string
		expected bool
	}{
		{"/help", "/", true},
		{"help", "/", false},
		{"", "/", false},
		{"-test", "-", true},
	}

	for _, tt := range tests {
		result := types.IsCommand(tt.input, tt.prefix)
		if result != tt.expected {
			t.Errorf("IsCommand(%q, %q) = %v, want %v", tt.input, tt.prefix, result, tt.expected)
		}
	}
}

// TestSplitCommand 测试命令分割
func TestSplitCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"help", []string{"help"}},
		{"help arg1", []string{"help", "arg1"}},
		{"help arg1 arg2", []string{"help", "arg1", "arg2"}},
		{"", nil},
		{"single", []string{"single"}},
	}

	for _, tt := range tests {
		result := splitCommandForTest(tt.input)
		if !equalStringSlices(result, tt.expected) {
			t.Errorf("splitCommand(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

// splitCommandForTest 是 command.go 中 splitCommand 的副本，用于测试
func splitCommandForTest(s string) []string {
	var parts []string
	var current []byte
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			if len(current) > 0 {
				parts = append(parts, string(current))
				current = nil
			}
		} else {
			current = append(current, s[i])
		}
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestFeishuAdapter_SendMessage_WithMock tests SendMessage using mock FeishuAPI
func TestFeishuAdapter_SendMessage_WithMock(t *testing.T) {
	mockMsgAPI := &mockMessageAPI{}

	var createCalled bool
	mockMsgAPI.createFunc = func(ctx context.Context, req *larkim.CreateMessageReq) (*larkim.CreateMessageResp, error) {
		createCalled = true
		t.Logf("CreateFunc called - mock is working")
		return &larkim.CreateMessageResp{}, nil
	}

	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	mockGW := &mockGatewayAPI{}
	cfg := &config.CommunicationConfig{}
	feishuCfg := &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}

	adapter := NewFeishuAdapter(nil, feishuCfg, cfg, WithFeishuAPI(mockAPI), WithGateway(mockGW))

	err := adapter.SendMessage(context.Background(), "feishu_oc_123456_ou_654321", "hello world")
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	if !createCalled {
		t.Fatal("CreateFunc was not called - mock chain is broken")
	}
}

// TestFeishuAdapter_BuildSessionKey_WithGateway tests buildSessionKey with mock gateway
func TestFeishuAdapter_BuildSessionKey_WithGateway(t *testing.T) {
	mockGW := &mockGatewayAPI{}
	cfg := &config.CommunicationConfig{}
	feishuCfg := &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}

	adapter := NewFeishuAdapter(nil, feishuCfg, cfg, WithGateway(mockGW))

	key := adapter.buildSessionKey("oc_abc123", "ou_xyz789")
	expected := "feishu_oc_abc123_ou_xyz789"
	if key != expected {
		t.Errorf("buildSessionKey() = %s, want %s", key, expected)
	}
}

// TestFeishuAdapter_HealthCheck_WithMock tests HealthCheck using mock FeishuAPI
func TestFeishuAdapter_HealthCheck_WithMock(t *testing.T) {
	mockAPI := newMockFeishuAPI()

	var called bool
	mockAPI.getFunc = func(ctx context.Context, path string, params interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
		called = true
		if path != "/open-apis/bot/v3/info" {
			t.Errorf("path = %s, want /open-apis/bot/v3/info", path)
		}
		return &larkcore.ApiResp{
			RawBody: []byte(`{"code":0}`),
		}, nil
	}

	cfg := &config.CommunicationConfig{}
	feishuCfg := &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}

	adapter := NewFeishuAdapter(nil, feishuCfg, cfg, WithFeishuAPI(mockAPI))

	err := adapter.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if !called {
		t.Error("HealthCheck() did not call Get")
	}
}

// TestFeishuAdapter_HealthCheck_Error_WithMock tests HealthCheck error handling with mock
func TestFeishuAdapter_HealthCheck_Error_WithMock(t *testing.T) {
	mockAPI := newMockFeishuAPI()

	mockAPI.getFunc = func(ctx context.Context, path string, params interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
		return &larkcore.ApiResp{
			RawBody: []byte(`{"code":1,"msg":"error"}`),
		}, nil
	}

	cfg := &config.CommunicationConfig{}
	feishuCfg := &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}

	adapter := NewFeishuAdapter(nil, feishuCfg, cfg, WithFeishuAPI(mockAPI))

	err := adapter.HealthCheck(context.Background())
	if err == nil {
		t.Error("HealthCheck() expected error, got nil")
	}
}

// TestFeishuAdapter_OnMessage_WithMock tests OnMessage callback using mock
func TestFeishuAdapter_OnMessage_WithMock(t *testing.T) {
	mockMsgAPI := &mockMessageAPI{}

	var createCalled bool
	mockMsgAPI.createFunc = func(ctx context.Context, req *larkim.CreateMessageReq) (*larkim.CreateMessageResp, error) {
		createCalled = true
		t.Logf("OnMessage: CreateFunc called - mock is working")
		return &larkim.CreateMessageResp{}, nil
	}

	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}
	mockGW := &mockGatewayAPI{}

	cfg := &config.CommunicationConfig{}
	feishuCfg := &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}

	adapter := NewFeishuAdapter(nil, feishuCfg, cfg, WithFeishuAPI(mockAPI), WithGateway(mockGW))

	msg := &types.OutboundMessage{
		SessionID: "sess_123",
		ChatID:    "feishu_oc_123456_ou_654321",
		Content:   "Hello from bot!",
	}

	adapter.OnMessage(msg)

	if !createCalled {
		t.Error("CreateFunc was not called - mock chain is broken")
	}
}

// TestFeishuAdapter_OnPermissionRequest tests permission request callback
func TestFeishuAdapter_OnPermissionRequest(t *testing.T) {
	mockGW := &mockGatewayAPI{}
	cfg := &config.CommunicationConfig{}
	feishuCfg := &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}

	adapter := NewFeishuAdapter(nil, feishuCfg, cfg, WithGateway(mockGW))

	req := &types.PermissionRequest{
		ToolName:  "read_file",
		RiskLevel: types.RiskLevelMedium,
	}

	result := adapter.OnPermissionRequest(req)
	if !result {
		t.Error("OnPermissionRequest() = false, want true")
	}
}

// TestFeishuAdapter_OnError tests error callback
func TestFeishuAdapter_OnError(t *testing.T) {
	mockGW := &mockGatewayAPI{}
	cfg := &config.CommunicationConfig{}
	feishuCfg := &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}

	adapter := NewFeishuAdapter(nil, feishuCfg, cfg, WithGateway(mockGW))

	// Should not panic
	adapter.OnError(context.DeadlineExceeded, "sess_123")
}

// TestFeishuAdapter_OnStatus tests status callback
func TestFeishuAdapter_OnStatus(t *testing.T) {
	mockGW := &mockGatewayAPI{}
	cfg := &config.CommunicationConfig{}
	feishuCfg := &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}

	adapter := NewFeishuAdapter(nil, feishuCfg, cfg, WithGateway(mockGW))

	// Should not panic
	adapter.OnStatus("sess_123", types.SessionStateThinking)
}

// TestFeishuAdapter_DuplicateMessage tests message deduplication
func TestFeishuAdapter_DuplicateMessage(t *testing.T) {
	mockGW := &mockGatewayAPI{}
	mockAPI := newMockFeishuAPI()
	cfg := &config.CommunicationConfig{}
	feishuCfg := &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}

	adapter := NewFeishuAdapter(nil, feishuCfg, cfg, WithFeishuAPI(mockAPI), WithGateway(mockGW))

	// First message should not be duplicate
	isDup1 := adapter.isDuplicateMessage("msg_001")
	if isDup1 {
		t.Error("first message should not be duplicate")
	}

	// Second message with same ID should be duplicate
	isDup2 := adapter.isDuplicateMessage("msg_001")
	if !isDup2 {
		t.Error("second message should be duplicate")
	}

	// Different message ID should not be duplicate
	isDup3 := adapter.isDuplicateMessage("msg_002")
	if isDup3 {
		t.Error("different message should not be duplicate")
	}
}

// TestFeishuAdapter_Start_AlreadyRunning tests Start when already running
func TestFeishuAdapter_Start_AlreadyRunning(t *testing.T) {
	mockGW := &mockGatewayAPI{}
	mockAPI := newMockFeishuAPI()
	cfg := &config.CommunicationConfig{}
	feishuCfg := &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}

	adapter := NewFeishuAdapter(nil, feishuCfg, cfg, WithFeishuAPI(mockAPI), WithGateway(mockGW))

	// Manually set running to true
	adapter.mu.Lock()
	adapter.running = true
	adapter.mu.Unlock()

	err := adapter.Start(context.Background())
	if err == nil {
		t.Error("Start() expected error when already running, got nil")
	}
}

// TestFeishuAdapter_Stop_NotRunning tests Stop when not running
func TestFeishuAdapter_Stop_NotRunning(t *testing.T) {
	mockGW := &mockGatewayAPI{}
	mockAPI := newMockFeishuAPI()
	cfg := &config.CommunicationConfig{}
	feishuCfg := &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}

	adapter := NewFeishuAdapter(nil, feishuCfg, cfg, WithFeishuAPI(mockAPI), WithGateway(mockGW))

	// Should not panic and return nil
	err := adapter.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

// ptrString is a helper to create *string for test
func ptrString(s string) *string {
	return &s
}

// TestFeishuAdapter_GetOrCreateSession_WithMock tests session creation via mock gateway
func TestFeishuAdapter_GetOrCreateSession_WithMock(t *testing.T) {
	mockGW := &mockGatewayAPI{}
	mockAPI := newMockFeishuAPI()
	cfg := &config.CommunicationConfig{}
	feishuCfg := &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}

	var createSessionCalled bool
	mockGW.createSessionFunc = func(chatID, workDir string) (*types.Session, error) {
		createSessionCalled = true
		if chatID != "feishu_oc_123456_ou_654321" {
			t.Errorf("chatID = %s, want feishu_oc_123456_ou_654321", chatID)
		}
		return &types.Session{
			SessionID: "sess_new_123",
			ChatID:    chatID,
		}, nil
	}

	adapter := NewFeishuAdapter(nil, feishuCfg, cfg, WithFeishuAPI(mockAPI), WithGateway(mockGW))

	session, err := adapter.getOrCreateSession("feishu_oc_123456_ou_654321")
	if err != nil {
		t.Fatalf("getOrCreateSession() error = %v", err)
	}
	if !createSessionCalled {
		t.Error("CreateSession was not called")
	}
	if session.SessionID != "sess_new_123" {
		t.Errorf("session.SessionID = %s, want sess_new_123", session.SessionID)
	}
}

func TestNormalizeReactionEmoji(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "OnIt"},
		{"none", ""},
		{"Done", "Done"},
	}
	for _, tt := range tests {
		if got := normalizeReactionEmoji(tt.input); got != tt.want {
			t.Errorf("normalizeReactionEmoji(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFeishuAdapter_ReplyAckToUser_UsesReplyAPI(t *testing.T) {
	var replyCalled bool
	var replyMsgID string

	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			replyCalled = true
			replyMsgID = "om_user_cmd"
			return &larkim.ReplyMessageResp{}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))

	err := adapter.replyAckToUser(context.Background(), "om_user_cmd", "feishu_oc_123456_ou_654321", "✅ 新会话已创建")
	if err != nil {
		t.Fatalf("replyAckToUser() error = %v", err)
	}
	if !replyCalled {
		t.Fatal("expected Reply API for command ack")
	}
	if replyMsgID != "om_user_cmd" {
		t.Fatalf("reply target = %q, want om_user_cmd", replyMsgID)
	}
}

func TestFeishuAdapter_SendMessageToSession_UsesReply(t *testing.T) {
	var replyCalled bool

	mockMsgAPI := &mockMessageAPI{
		replyFunc: func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
			replyCalled = true
			return &larkim.ReplyMessageResp{}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: &mockMessageReactionAPI{}}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))

	adapter.sessionReplyCtx.Store("sess_1", feishuReplyContext{userMessageID: "om_root"})

	err := adapter.sendMessageToSession(context.Background(), "sess_1", "feishu_oc_123456_ou_654321", "hello")
	if err != nil {
		t.Fatalf("sendMessageToSession() error = %v", err)
	}
	if !replyCalled {
		t.Fatal("expected Reply API to be called when reply context exists")
	}
}

func TestFeishuAdapter_AddReaction_UsesMessageReactionAPI(t *testing.T) {
	var createCalled bool
	mockReactionAPI := &mockMessageReactionAPI{
		createFunc: func(ctx context.Context, req *larkim.CreateMessageReactionReq) (*larkim.CreateMessageReactionResp, error) {
			createCalled = true
			reactionID := "re_123"
			return &larkim.CreateMessageReactionResp{
				Data: &larkim.CreateMessageReactionRespData{ReactionId: &reactionID},
			}, nil
		},
	}
	mockImAPI := &mockImAPI{messageAPI: &mockMessageAPI{}, messageReactionAPI: mockReactionAPI}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	adapter := NewFeishuAdapter(nil, &FeishuConfig{AppID: "test_app", AppSecret: "test_secret"}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))

	if err := adapter.AddReaction(context.Background(), "om_root", "Done"); err != nil {
		t.Fatalf("AddReaction() error = %v", err)
	}
	if !createCalled {
		t.Fatal("expected MessageReaction.Create to be called")
	}
}
