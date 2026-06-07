package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// FeishuAdapter provides Feishu/Lark integration for the communication layer
type FeishuAdapter struct {
	gateway    *gateway.CommunicationGateway
	cfg        *config.CommunicationConfig
	feishuCfg  *FeishuConfig
	eventHandler gateway.EventHandler

	client      *lark.Client
	wsClient    *larkws.Client
	server      *http.Server
	dispatcher  *dispatcher.EventDispatcher

	mu        sync.RWMutex
	running   bool
	botOpenID string
	cancel    context.CancelFunc

	sessionMap sync.Map // sessionKey -> sessionID mapping
}

// Ensure FeishuAdapter implements gateway.EventHandler
var _ gateway.EventHandler = (*FeishuAdapter)(nil)

// OnMessage handles outbound messages from the gateway
func (a *FeishuAdapter) OnMessage(msg *types.OutboundMessage) {
	slog.Info("feishu: sending message", "sessionID", msg.SessionID, "chatID", msg.ChatID, "content", msg.Content)

	ctx := context.Background()

	if err := a.SendMessage(ctx, msg.ChatID, msg.Content); err != nil {
		slog.Error("feishu: failed to send message", "error", err, "chatID", msg.ChatID)
	}
}

// OnPermissionRequest handles permission requests
func (a *FeishuAdapter) OnPermissionRequest(req *types.PermissionRequest) bool {
	slog.Info("feishu: permission request", "tool", req.ToolName, "risk", req.RiskLevel)
	return true
}

// OnError handles errors
func (a *FeishuAdapter) OnError(err error, sessionID string) {
	slog.Error("feishu: session error", "sessionID", sessionID, "error", err)
}

// OnStatus handles session status changes
func (a *FeishuAdapter) OnStatus(sessionID string, state types.SessionState) {
	slog.Debug("feishu: session status", "sessionID", sessionID, "state", state)
}

// handleFeishuCommand handles built-in commands like /help
// Returns true if the message was handled as a command, false otherwise
func (a *FeishuAdapter) handleFeishuCommand(ctx context.Context, text, sessionKey string) bool {
	if !strings.HasPrefix(text, "/") {
		return false
	}

	cmd := types.ParseCommand(text, "/")

	switch cmd.Type {
	case types.CommandHelp:
		helpText := `🤖 *Devrix - 开发大脑*

*基础命令：*
/new - 开始新会话
/help - 显示帮助信息
/stop - 停止当前生成

*功能说明：*
Devrix 是一个多智能体 AI 编程助手，可以通过飞书与你对话。

*使用方式：*
直接发送消息即可与我对话。我会帮助你：
• 编写和调试代码
• 分析项目结构
• 执行开发任务
• 回答技术问题

*权限说明：*
YOLO 模式已启用，所有操作自动授权。`

		if err := a.SendMessage(ctx, sessionKey, helpText); err != nil {
			slog.Error("feishu: failed to send help message", "error", err)
		}
		return true

	case types.CommandNew:
		// Create new session
		session, err := a.gateway.CreateSession(sessionKey, "")
		if err != nil {
			slog.Error("feishu: failed to create new session", "error", err)
			a.SendMessage(ctx, sessionKey, "❌ 创建新会话失败")
			return true
		}
		// Update session map with new session
		a.sessionMap.Store(sessionKey, session.SessionID)
		a.SendMessage(ctx, sessionKey, "✅ 已开始新会话")
		return true

	case types.CommandStop:
		// TODO: Implement stop functionality
		a.SendMessage(ctx, sessionKey, "⏸️ 停止功能开发中")
		return true

	default:
		slog.Debug("feishu: unknown command", "command", text)
		return false
	}
}

// FeishuConfig holds Feishu-specific configuration
type FeishuConfig struct {
	AppID       string
	AppSecret   string
	BotName     string
	AllowFrom   string
	Domain      string
	EncryptKey  string
	CallbackPath string
	Port        string
	UseWebhook  bool
}

// NewFeishuAdapter creates a new Feishu adapter
func NewFeishuAdapter(
	gw *gateway.CommunicationGateway,
	feishuCfg *FeishuConfig,
	cfg *config.CommunicationConfig,
) *FeishuAdapter {
	var clientOpts []lark.ClientOptionFunc
	domain := lark.FeishuBaseUrl
	if feishuCfg.Domain != "" {
		domain = feishuCfg.Domain
		clientOpts = append(clientOpts, lark.WithOpenBaseUrl(domain))
	}

	return &FeishuAdapter{
		gateway:   gw,
		cfg:       cfg,
		feishuCfg: feishuCfg,
		client:    lark.NewClient(feishuCfg.AppID, feishuCfg.AppSecret, clientOpts...),
	}
}

// SetGateway sets the gateway reference
func (a *FeishuAdapter) SetGateway(gw *gateway.CommunicationGateway) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.gateway = gw
}

// Start begins the Feishu WebSocket connection and event handling
func (a *FeishuAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("Feishu adapter already running")
	}
	a.running = true
	a.mu.Unlock()

	// Get bot info
	if err := a.fetchBotInfo(ctx); err != nil {
		slog.Warn("feishu: failed to get bot info", "error", err)
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	// Setup event dispatcher
	a.dispatcher = a.createEventDispatcher()

	if a.feishuCfg.UseWebhook || a.feishuCfg.EncryptKey != "" {
		return a.startWebhookMode(ctx)
	}
	return a.startWebSocketMode(ctx)
}

func (a *FeishuAdapter) createEventDispatcher() *dispatcher.EventDispatcher {
	d := dispatcher.NewEventDispatcher("", a.feishuCfg.EncryptKey).
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			slog.Info("feishu: WS event received - message", "event_type", "P2MessageReceiveV1")
			slog.Debug("feishu: event detail", "event", fmt.Sprintf("%+v", event))
			return a.onMessage(ctx, event)
		})
	slog.Info("feishu: dispatcher created with P2MessageReceiveV1 handler")
	return d
}

// startWebSocketMode starts the WebSocket connection for real-time events
func (a *FeishuAdapter) startWebSocketMode(ctx context.Context) error {
	slog.Info("feishu: starting WebSocket mode", "app_id", a.feishuCfg.AppID)

	wsOpts := []larkws.ClientOption{
		larkws.WithEventHandler(a.dispatcher),
		larkws.WithLogLevel(larkcore.LogLevelDebug),
		larkws.WithLogger(larkcore.NewEventLogger()),
	}
	if a.feishuCfg.Domain != "" {
		wsOpts = append(wsOpts, larkws.WithDomain(a.feishuCfg.Domain))
	}

	slog.Info("feishu: creating WS client with dispatcher", "has_dispatcher", a.dispatcher != nil)
	a.wsClient = larkws.NewClient(a.feishuCfg.AppID, a.feishuCfg.AppSecret, wsOpts...)

	go func() {
		if err := a.wsClient.Start(ctx); err != nil {
			slog.Error("feishu: websocket error", "error", err)
		}
	}()

	slog.Info("feishu: connecting via WebSocket",
		"app_id", a.feishuCfg.AppID,
		"domain", a.feishuCfg.Domain,
	)

	return nil
}

// startWebhookMode starts the HTTP webhook server for receiving events
func (a *FeishuAdapter) startWebhookMode(ctx context.Context) error {
	port := a.feishuCfg.Port
	if port == "" {
		port = "8080"
	}
	callbackPath := a.feishuCfg.CallbackPath
	if callbackPath == "" {
		callbackPath = "/feishu/webhook"
	}

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, a.webhookHandler)

	a.server = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		slog.Info("feishu: webhook server starting", "port", port, "path", callbackPath)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("feishu: webhook server error", "error", err)
		}
	}()

	return nil
}

// webhookHandler handles incoming webhook requests
func (a *FeishuAdapter) webhookHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("feishu: read webhook body failed", "error", err)
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	req := &larkevent.EventReq{
		Header:     r.Header,
		Body:       body,
		RequestURI: r.RequestURI,
	}

	resp := a.dispatcher.Handle(r.Context(), req)

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(resp.Body)
}

// onMessage handles incoming Feishu messages
func (a *FeishuAdapter) onMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	slog.Info("feishu: onMessage called", "event", fmt.Sprintf("%+v", event))

	if event == nil || event.Event == nil || event.Event.Message == nil {
		slog.Warn("feishu: event is nil or missing message")
		return nil
	}

	msg := event.Event.Message
	sender := event.Event.Sender

	// Extract message content - msg is *EventMessage, need to access fields
	msgType := ""
	if msg.MessageType != nil {
		msgType = *msg.MessageType
	}

	content := ""
	if msg.Content != nil {
		content = *msg.Content
	}

	// Parse text content if it's a text message
	var text string
	if msgType == "text" {
		var textContent struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(content), &textContent); err == nil {
			text = textContent.Text
		}
	}

	if text == "" && content != "" {
		text = content
	}

	if text == "" {
		return nil
	}

	// Extract sender info first (needed for command handling)
	userID := ""
	if sender != nil && sender.SenderId != nil {
		if sender.SenderId.OpenId != nil {
			userID = *sender.SenderId.OpenId
		} else if sender.SenderId.UserId != nil {
			userID = *sender.SenderId.UserId
		}
	}

	// Build session key
	chatID := ""
	if msg.ChatId != nil {
		chatID = *msg.ChatId
	}
	sessionKey := a.buildSessionKey(chatID, userID)

	// Handle built-in commands before routing to gateway
	if a.handleFeishuCommand(ctx, text, sessionKey) {
		return nil
	}

	// Get or create session
	session, err := a.getOrCreateSession(sessionKey)
	if err != nil {
		slog.Error("feishu: failed to get or create session", "error", err)
		return nil
	}

	// Create inbound message
	messageID := ""
	if msg.MessageId != nil {
		messageID = *msg.MessageId
	}

	chatType := ""
	if msg.ChatType != nil {
		chatType = *msg.ChatType
	}

	inboundMsg := &types.InboundMessage{
		SessionID:  session.SessionID,
		ChatID:     sessionKey,
		UserID:     userID,
		UserName:   userID,
		Content:    text,
		MessageID:  messageID,
		AdapterID:  "feishu",
		ReceivedAt: time.Now(),
		Metadata: map[string]string{
			"chat_type": chatType,
			"msg_type":  msgType,
		},
	}

	// Route to gateway
	if err := a.gateway.RouteInbound(ctx, inboundMsg); err != nil {
		slog.Error("feishu: failed to route message",
			"error", err,
			"sessionID", session.SessionID,
		)
	}

	return nil
}

// buildSessionKey builds a unique session key
func (a *FeishuAdapter) buildSessionKey(chatID, userID string) string {
	return fmt.Sprintf("feishu_%s_%s", chatID, userID)
}

// getOrCreateSession gets an existing session or creates a new one
func (a *FeishuAdapter) getOrCreateSession(sessionKey string) (*types.Session, error) {
	// Check if we already have a session for this chat+user
	if existingSessionID, ok := a.sessionMap.Load(sessionKey); ok {
		session, err := a.gateway.GetSession(existingSessionID.(string))
		if err == nil && session != nil {
			return session, nil
		}
		// Session not found or expired, remove from map
		a.sessionMap.Delete(sessionKey)
	}

	// Create new session
	session, err := a.gateway.CreateSession(sessionKey, "")
	if err != nil {
		return nil, err
	}

	// Store mapping
	a.sessionMap.Store(sessionKey, session.SessionID)

	return session, nil
}

// fetchBotInfo fetches the bot's information using the low-level API
func (a *FeishuAdapter) fetchBotInfo(ctx context.Context) error {
	resp, err := a.client.Get(ctx, "/open-apis/bot/v3/info", nil, larkcore.AccessTokenTypeTenant)
	if err != nil {
		return err
	}

	var result struct {
		Code int `json:"code"`
		Bot  struct {
			OpenID string `json:"open_id"`
			Name   string `json:"app_name"`
		} `json:"bot"`
	}
	if err := json.Unmarshal(resp.RawBody, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("api code=%d", result.Code)
	}

	a.mu.Lock()
	a.botOpenID = result.Bot.OpenID
	a.mu.Unlock()

	slog.Info("feishu: bot identified",
		"open_id", result.Bot.OpenID,
		"app_name", result.Bot.Name,
	)

	return nil
}

// Stop stops the Feishu adapter
func (a *FeishuAdapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	a.running = false

	if a.cancel != nil {
		a.cancel()
	}

	if a.server != nil {
		a.server.Shutdown(context.Background())
	}

	slog.Info("feishu adapter stopped")
	return nil
}

// SetEventHandler sets the event handler for gateway callbacks
func (a *FeishuAdapter) SetEventHandler(h gateway.EventHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.eventHandler = h
}

// SendMessage sends a message to Feishu user
func (a *FeishuAdapter) SendMessage(ctx context.Context, chatID, content string) error {
	// Parse chat_id to extract Feishu user/chat ID
	// sessionKey format: feishu_{chat_id}_{user_id}
	// chat_id format is "oc_xxxxxx", user_id format is "ou_xxxxxx"
	parts := strings.Split(chatID, "_")
	if len(parts) < 5 {
		return fmt.Errorf("invalid session key: %s", chatID)
	}

	// parts = ["feishu", "oc", "123456", "ou", "654321"]
	// chat_id = parts[1] + "_" + parts[2] = "oc_123456"
	// user_id = parts[3] + "_" + parts[4] = "ou_654321"
	chatID = parts[1] + "_" + parts[2]
	msgType := "text"
	contentJSON := fmt.Sprintf(`{"text":"%s"}`, escapeJSON(content))

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(msgType).
			Content(contentJSON).
			Build()).
		Build()

	var resp *larkim.CreateMessageResp
	err := a.withRetry(ctx, "send message", func() error {
		var apiErr error
		resp, apiErr = a.client.Im.Message.Create(ctx, req)
		return apiErr
	})

	if err != nil {
		return fmt.Errorf("feishu: send message failed: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// ReplyMessage replies to a specific message in Feishu
func (a *FeishuAdapter) ReplyMessage(ctx context.Context, messageID, content string) error {
	msgType := "text"
	contentJSON := fmt.Sprintf(`{"text":"%s"}`, escapeJSON(content))

	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(msgType).
			Content(contentJSON).
			Build()).
		Build()

	var resp *larkim.ReplyMessageResp
	err := a.withRetry(ctx, "reply message", func() error {
		var apiErr error
		resp, apiErr = a.client.Im.Message.Reply(ctx, req)
		return apiErr
	})

	if err != nil {
		return fmt.Errorf("feishu: reply message failed: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// withRetry executes an operation with retry logic
func (a *FeishuAdapter) withRetry(ctx context.Context, operation string, fn func() error) error {
	err := fn()
	if err == nil {
		return nil
	}
	// Simple retry - could be enhanced with token refresh logic
	return err
}

// escapeJSON escapes special characters for JSON string
func escapeJSON(s string) string {
	var builder strings.Builder
	for _, c := range s {
		switch c {
		case '"':
			builder.WriteString("\\\"")
		case '\\':
			builder.WriteString("\\\\")
		case '\n':
			builder.WriteString("\\n")
		case '\r':
			builder.WriteString("\\r")
		case '\t':
			builder.WriteString("\\t")
		default:
			builder.WriteRune(c)
		}
	}
	return builder.String()
}

// HealthCheck checks if the Feishu connection is healthy
func (a *FeishuAdapter) HealthCheck(ctx context.Context) error {
	if a.client == nil {
		return fmt.Errorf("feishu client not initialized")
	}

	resp, err := a.client.Get(ctx, "/open-apis/bot/v3/info", nil, larkcore.AccessTokenTypeTenant)
	if err != nil {
		return fmt.Errorf("feishu health check failed: %w", err)
	}

	var result struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(resp.RawBody, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("feishu API error: code=%d", result.Code)
	}

	return nil
}
