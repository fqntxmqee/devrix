package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/core"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
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
	gateway    gateway.GatewayAPI
	cfg        *config.CommunicationConfig
	feishuCfg  *FeishuConfig
	eventHandler gateway.EventHandler
	api        FeishuAPI

	client      *lark.Client
	wsClient    *larkws.Client
	server      *http.Server
	dispatcher  *dispatcher.EventDispatcher

	mu        sync.RWMutex
	running   bool
	botOpenID string
	cancel    context.CancelFunc

	sessionMap sync.Map // sessionKey -> sessionID mapping
	dedupMap   sync.Map // messageID -> timestamp for deduplication

	// OK confirmation and done_emoji settings
	reactionEmoji string
	doneEmoji     string
	replyInThread bool
	progressStyle string

	sessionMsgMap   sync.Map // sessionID -> userMessageID mapping
	sessionReplyCtx sync.Map // sessionID -> feishuReplyContext
	sessionStreams  sync.Map // sessionID -> *feishuSessionStream for coalesced progress
	obsBridge       *observability.Bridge
}

// feishuReplyContext tracks the user's root message for threaded replies and typing reaction.
type feishuReplyContext struct {
	userMessageID string
	reactionID    string
}

// isDuplicateMessage checks if the message has been seen before
// Returns true if duplicate, and records the message ID
func (a *FeishuAdapter) isDuplicateMessage(messageID string) bool {
	if messageID == "" {
		return false
	}

	// Clean up old entries (older than 5 minutes)
	now := time.Now().Unix()
	a.dedupMap.Range(func(key, value interface{}) bool {
		if timestamp, ok := value.(int64); ok {
			if now-timestamp > 300 {
				a.dedupMap.Delete(key)
			}
		}
		return true
	})

	// Check if already seen
	if _, exists := a.dedupMap.LoadOrStore(messageID, now); exists {
		return true
	}
	return false
}

// Ensure FeishuAdapter implements gateway.EventHandler
var _ gateway.EventHandler = (*FeishuAdapter)(nil)

// OnMessage handles outbound messages from the gateway
func (a *FeishuAdapter) OnMessage(msg *types.OutboundMessage) {
	slog.Info("feishu: OnMessage called", "sessionID", msg.SessionID, "chatID", msg.ChatID, "contentLen", len(msg.Content), "eventType", msg.Metadata["event_type"])

	ctx := context.Background()
	eventType := msg.Metadata["event_type"]
	if eventType == "" {
		eventType = "text"
	}
	content := msg.Content

	switch eventType {
	case "thinking", "tool_call", "tool_result", "milestone_progress":
		if err := a.handleProgressEvent(ctx, msg); err != nil {
			slog.Error("feishu: failed to send progress", "error", err, "eventType", eventType)
		}

	case "complete":
		if err := a.finalizeStructuredSession(ctx, msg.SessionID, msg.ChatID, content); err != nil {
			slog.Error("feishu: failed to finalize structured session", "error", err)
		}

		if a.doneEmoji != "" {
			go a.finishUserMessageReaction(context.Background(), msg.SessionID)
		} else {
			a.clearSessionReplyContext(msg.SessionID)
		}
		a.clearSessionStream(msg.SessionID)

	case "error":
		card := NewCard().
			Title("错误", "red").
			Markdown(content).
			Build()
		if err := a.sendCardToSession(ctx, msg.SessionID, msg.ChatID, card); err != nil {
			slog.Error("feishu: failed to send error card", "error", err)
			return
		}
		a.clearSessionStream(msg.SessionID)

	case "info":
		if err := a.handleProgressEvent(ctx, msg); err != nil {
			slog.Error("feishu: failed to send info progress", "error", err)
		}

	case "text":
		if err := a.appendResponseText(ctx, msg.SessionID, msg.ChatID, content); err != nil {
			slog.Error("feishu: failed to send response text", "error", err)
		}
		return

	default:
		slog.Warn("feishu: unhandled outbound event", "eventType", eventType)
	}

	slog.Info("feishu: message sent successfully", "chatID", msg.ChatID)
}

// hasComplexMarkdown returns true if content contains code blocks or tables
func hasComplexMarkdown(content string) bool {
	if strings.Contains(content, "```") {
		return true
	}
	// Check for table-like lines
	lines := strings.Split(content, "\n")
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 && trimmed[0] == '|' {
			count++
		}
	}
	return count >= 2 // At least 2 table rows
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

// handleFeishuCommand handles built-in commands like /help.
// Returns true if the message was handled as a command, false otherwise.
func (a *FeishuAdapter) handleFeishuCommand(ctx context.Context, text, sessionKey, userMessageID string) bool {
	if !strings.HasPrefix(text, "/") {
		return false
	}

	cmd := types.ParseCommand(text, "/")

	switch cmd.Type {
	case types.CommandHelp:
		helpText := `🤖 **Devrix - 开发大脑**

**基础命令：**
/new - 开始新会话
/help - 显示帮助信息
/stop - 停止当前生成

**功能说明：**
Devrix 是一个多智能体 AI 编程助手，可以通过飞书与你对话。

**使用方式：**
直接发送消息即可与我对话。我会帮助你：
• 编写和调试代码
• 分析项目结构
• 执行开发任务
• 回答技术问题

**权限说明：**
YOLO 模式已启用，所有操作自动授权。`

		if err := a.replyAckToUser(ctx, userMessageID, sessionKey, helpText); err != nil {
			slog.Error("feishu: failed to send help message", "error", err)
		}
		return true

	case types.CommandNew:
		if oldSessionID, ok := a.sessionMap.Load(sessionKey); ok {
			if sid, ok := oldSessionID.(string); ok && sid != "" {
				a.clearSessionStream(sid)
				a.clearSessionReplyContext(sid)
			}
		}
		session, err := a.gateway.CreateSession(sessionKey, "")
		if err != nil {
			slog.Error("feishu: failed to create new session", "error", err)
			_ = a.replyAckToUser(ctx, userMessageID, sessionKey, "❌ 创建新会话失败")
			return true
		}
		a.sessionMap.Store(sessionKey, session.SessionID)
		if err := a.replyAckToUser(ctx, userMessageID, sessionKey, "✅ 新会话已创建"); err != nil {
			slog.Error("feishu: failed to send new session ack", "error", err)
		}
		return true

	case types.CommandStop:
		// TODO: Implement stop functionality
		if err := a.replyAckToUser(ctx, userMessageID, sessionKey, "⏸️ 停止功能开发中"); err != nil {
			slog.Error("feishu: failed to send stop ack", "error", err)
		}
		return true

	default:
		slog.Debug("feishu: unknown command", "command", text)
		return false
	}
}

// replyAckToUser sends a command acknowledgement as a reply under the user's message (cc-connect style).
func (a *FeishuAdapter) replyAckToUser(ctx context.Context, userMessageID, sessionKey, text string) error {
	card := NewCard().Markdown(text).Build()
	cardJSON := BuildCardJSON(card)
	if userMessageID != "" {
		_, err := a.replyToUserMessage(ctx, userMessageID, "interactive", cardJSON)
		return err
	}
	return a.SendCard(ctx, sessionKey, card)
}

// FeishuConfig holds Feishu-specific configuration
type FeishuConfig struct {
	AppID         string
	AppSecret     string
	BotName       string
	AllowFrom     string
	Domain        string
	EncryptKey    string
	CallbackPath  string
	Port          string
	UseWebhook    bool
	ReactionEmoji string // emoji reaction on incoming messages (default: OnIt); "none" to disable
	DoneEmoji     string // emoji reaction when agent completes (e.g. Done); "none" or empty to disable
	ReplyInThread bool   // reply in thread under user's message (shows "N 条回复")
	ProgressStyle string // legacy | compact | card | structured — structured 为默认
}

// FeishuAdapterOption is a functional option for FeishuAdapter
type FeishuAdapterOption func(*FeishuAdapter)

// WithFeishuAPI sets the Feishu API implementation
func WithFeishuAPI(api FeishuAPI) FeishuAdapterOption {
	return func(a *FeishuAdapter) {
		a.api = api
	}
}

// WithGateway sets the gateway implementation
func WithGateway(gw gateway.GatewayAPI) FeishuAdapterOption {
	return func(a *FeishuAdapter) {
		a.gateway = gw
	}
}

// WithObservability wires tracing into the Feishu adapter.
func WithObservability(obs *observability.Observability) FeishuAdapterOption {
	return func(a *FeishuAdapter) {
		if obs == nil {
			a.obsBridge = nil
			return
		}
		a.obsBridge = observability.NewBridge(obs)
	}
}

// NewFeishuAdapter creates a new Feishu adapter
func NewFeishuAdapter(
	gw *gateway.CommunicationGateway,
	feishuCfg *FeishuConfig,
	cfg *config.CommunicationConfig,
	opts ...FeishuAdapterOption,
) *FeishuAdapter {
	var clientOpts []lark.ClientOptionFunc
	domain := lark.FeishuBaseUrl
	if feishuCfg.Domain != "" {
		domain = feishuCfg.Domain
		clientOpts = append(clientOpts, lark.WithOpenBaseUrl(domain))
	}

	adapter := &FeishuAdapter{
		gateway:       gw,
		cfg:           cfg,
		feishuCfg:     feishuCfg,
		client:        lark.NewClient(feishuCfg.AppID, feishuCfg.AppSecret, clientOpts...),
		reactionEmoji: normalizeReactionEmoji(feishuCfg.ReactionEmoji),
		doneEmoji:     normalizeDoneEmoji(feishuCfg.DoneEmoji),
		replyInThread: feishuCfg.ReplyInThread,
		progressStyle: normalizeProgressStyle(feishuCfg.ProgressStyle),
	}

	// Apply functional options
	for _, opt := range opts {
		opt(adapter)
	}

	// If no API was provided, use the default lark implementation
	if adapter.api == nil {
		adapter.api = NewLarkFeishuAPI(adapter.client)
	}

	return adapter
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
		}).
		OnP2MessageReactionCreatedV1(func(ctx context.Context, event *larkim.P2MessageReactionCreatedV1) error {
			return nil // ignore reaction events triggered by our own reactions
		}).
		OnP2MessageReactionDeletedV1(func(ctx context.Context, event *larkim.P2MessageReactionDeletedV1) error {
			return nil // ignore reaction removal events triggered by our own reactions
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
	slog.Info("feishu: onMessage called", "event_type", fmt.Sprintf("%T", event))

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

	slog.Info("feishu: message received", "msgType", msgType, "content", content)

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
		slog.Warn("feishu: text is empty, ignoring message")
		return nil
	}

	slog.Info("feishu: parsed text", "text", text)

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

	messageID := ""
	if msg.MessageId != nil {
		messageID = *msg.MessageId
	}

	// Handle built-in commands before routing to gateway (reply under user's message).
	if a.handleFeishuCommand(ctx, text, sessionKey, messageID) {
		return nil
	}

	// Get or create session
	session, err := a.getOrCreateSession(sessionKey)
	if err != nil {
		slog.Error("feishu: failed to get or create session", "error", err)
		return nil
	}

	// Create inbound message (messageID extracted above for commands)

	// Deduplicate messages
	if a.isDuplicateMessage(messageID) {
		slog.Info("feishu: duplicate message ignored", "messageID", messageID, "text", text)
		return nil
	}

	chatType := ""
	if msg.ChatType != nil {
		chatType = *msg.ChatType
	}

	// Check message age and filter out old messages (older than 5 minutes)
	// This prevents replaying old messages after reconnection
	if msg.CreateTime != nil {
		if ms, err := strconv.ParseInt(*msg.CreateTime, 10, 64); err == nil {
			msgTime := time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond))
			if time.Since(msgTime) > 5*time.Minute {
				slog.Info("feishu: ignoring old message", "messageID", messageID, "text", text, "age", time.Since(msgTime))
				return nil
			}
		}
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

	// Store reply context synchronously before routing so outbound replies use
	// Im.Message.Reply (thread/quote) instead of CreateMessage.
	if messageID != "" {
		a.sessionReplyCtx.Store(session.SessionID, feishuReplyContext{
			userMessageID: messageID,
		})
	}

	// Immediate ACK: add reaction emoji on user's message (like cc-connect "OnIt")
	if a.reactionEmoji != "" && messageID != "" {
		go func(sessionID, msgID string) {
			ackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			reactionID := a.addReactionWithEmoji(ackCtx, msgID, a.reactionEmoji)
			if reactionID == "" {
				slog.Warn("feishu: failed to add typing reaction", "emoji", a.reactionEmoji)
				return
			}
			a.sessionReplyCtx.Store(sessionID, feishuReplyContext{
				userMessageID: msgID,
				reactionID:    reactionID,
			})
			slog.Info("feishu: added typing reaction", "emoji", a.reactionEmoji, "messageID", msgID)
		}(session.SessionID, messageID)
	}

	// Store the user message ID for done_emoji later
	a.sessionMsgMap.Store(session.SessionID, messageID)

	routeCtx := ctx
	if a.obsBridge != nil && a.obsBridge.Tracer() != nil {
		var span tracer.Span
		routeCtx, span = a.obsBridge.Tracer().Start(ctx, telemetry.OpAdapterMessageReceive,
			tracer.WithSpanKind(tracer.SpanKindServer),
			tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpAdapterMessageReceive,
				tracer.Attribute{Key: "adapter", Value: "feishu"},
				tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(text))},
			)...),
		)
		defer span.End()
	}

	// Route to gateway
	if err := a.gateway.RouteInbound(routeCtx, inboundMsg); err != nil {
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
	resp, err := a.api.Get(ctx, "/open-apis/bot/v3/info", nil, larkcore.AccessTokenTypeTenant)
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

// SendMessage sends a standalone message to a Feishu chat (used for commands without reply context).
func (a *FeishuAdapter) SendMessage(ctx context.Context, chatID, content string) error {
	feishuChatID, err := parseFeishuChatID(chatID)
	if err != nil {
		return err
	}

	msgType := "text"
	contentJSON := fmt.Sprintf(`{"text":"%s"}`, escapeJSON(content))

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(feishuChatID).
			MsgType(msgType).
			Content(contentJSON).
			Build()).
		Build()

	var resp *larkim.CreateMessageResp
	err = a.withRetry(ctx, "send message", func() error {
		var apiErr error
		resp, apiErr = a.api.Im().Message().Create(ctx, req)
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

func (a *FeishuAdapter) sendMessageToSession(ctx context.Context, sessionID, chatID, content string) error {
	if replyCtx, ok := a.getSessionReplyContext(sessionID); ok && replyCtx.userMessageID != "" {
		_, err := a.replyToUserMessage(ctx, replyCtx.userMessageID, "text", fmt.Sprintf(`{"text":"%s"}`, escapeJSON(content)))
		return err
	}
	return a.SendMessage(ctx, chatID, content)
}

// SendCard sends an interactive card to Feishu user (standalone, no reply context).
func (a *FeishuAdapter) SendCard(ctx context.Context, chatID string, card *core.Card) error {
	feishuChatID, err := parseFeishuChatID(chatID)
	if err != nil {
		return err
	}

	cardJSON := BuildCardJSON(card)

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(feishuChatID).
			MsgType("interactive").
			Content(cardJSON).
			Build()).
		Build()

	var resp *larkim.CreateMessageResp
	err = a.withRetry(ctx, "send card", func() error {
		var apiErr error
		resp, apiErr = a.api.Im().Message().Create(ctx, req)
		return apiErr
	})

	if err != nil {
		return fmt.Errorf("feishu: send card failed: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

func (a *FeishuAdapter) sendCardToSession(ctx context.Context, sessionID, chatID string, card *core.Card) error {
	cardJSON := BuildCardJSON(card)
	_, err := a.sendCardReplyAndGetID(ctx, sessionID, chatID, cardJSON)
	return err
}

// AddReaction adds a Feishu emoji reaction to a message.
func (a *FeishuAdapter) AddReaction(ctx context.Context, messageID, emoji string) error {
	if emoji == "" || messageID == "" {
		return nil
	}
	if a.addReactionWithEmoji(ctx, messageID, emoji) == "" {
		return fmt.Errorf("feishu: add reaction failed")
	}
	return nil
}

func (a *FeishuAdapter) addReactionWithEmoji(ctx context.Context, messageID, emojiType string) string {
	if emojiType == "" || messageID == "" {
		return ""
	}

	req := larkim.NewCreateMessageReactionReqBuilder().
		MessageId(messageID).
		Body(larkim.NewCreateMessageReactionReqBodyBuilder().
			ReactionType(&larkim.Emoji{EmojiType: &emojiType}).
			Build()).
		Build()

	var resp *larkim.CreateMessageReactionResp
	err := a.withRetry(ctx, "add reaction", func() error {
		var apiErr error
		resp, apiErr = a.api.Im().MessageReaction().Create(ctx, req)
		return apiErr
	})
	if err != nil {
		slog.Debug("feishu: add reaction failed", "error", err, "emoji", emojiType)
		return ""
	}
	if resp == nil || !resp.Success() {
		code, msg := 0, ""
		if resp != nil {
			code, msg = resp.Code, resp.Msg
		}
		slog.Debug("feishu: add reaction API error", "code", code, "msg", msg, "emoji", emojiType)
		return ""
	}
	if resp.Data != nil && resp.Data.ReactionId != nil {
		return *resp.Data.ReactionId
	}
	return ""
}

func (a *FeishuAdapter) removeReaction(ctx context.Context, messageID, reactionID string) {
	if reactionID == "" || messageID == "" {
		return
	}

	req := larkim.NewDeleteMessageReactionReqBuilder().
		MessageId(messageID).
		ReactionId(reactionID).
		Build()

	err := a.withRetry(ctx, "remove reaction", func() error {
		resp, apiErr := a.api.Im().MessageReaction().Delete(ctx, req)
		if apiErr != nil {
			return apiErr
		}
		if resp != nil && !resp.Success() {
			return fmt.Errorf("feishu API error: code=%d msg=%s", resp.Code, resp.Msg)
		}
		return nil
	})
	if err != nil {
		slog.Debug("feishu: remove reaction failed", "error", err)
	}
}

func (a *FeishuAdapter) finishUserMessageReaction(ctx context.Context, sessionID string) {
	replyCtx, ok := a.getSessionReplyContext(sessionID)
	if !ok {
		return
	}
	defer a.clearSessionReplyContext(sessionID)

	if replyCtx.reactionID != "" {
		a.removeReaction(ctx, replyCtx.userMessageID, replyCtx.reactionID)
	}
	if a.doneEmoji != "" {
		if reactionID := a.addReactionWithEmoji(ctx, replyCtx.userMessageID, a.doneEmoji); reactionID != "" {
			slog.Info("feishu: added done emoji", "emoji", a.doneEmoji)
		} else {
			slog.Warn("feishu: failed to add done emoji", "emoji", a.doneEmoji)
		}
	}
}

func (a *FeishuAdapter) getSessionReplyContext(sessionID string) (feishuReplyContext, bool) {
	if value, ok := a.sessionReplyCtx.Load(sessionID); ok {
		if replyCtx, ok := value.(feishuReplyContext); ok {
			return replyCtx, true
		}
	}
	return feishuReplyContext{}, false
}

func (a *FeishuAdapter) clearSessionReplyContext(sessionID string) {
	a.sessionReplyCtx.Delete(sessionID)
	a.sessionMsgMap.Delete(sessionID)
}

func (a *FeishuAdapter) replyToUserMessage(ctx context.Context, messageID, msgType, content string) (string, error) {
	bodyBuilder := larkim.NewReplyMessageReqBodyBuilder().
		MsgType(msgType).
		Content(content)
	if a.replyInThread {
		bodyBuilder.ReplyInThread(true)
	}

	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(bodyBuilder.Build()).
		Build()

	var resp *larkim.ReplyMessageResp
	err := a.withRetry(ctx, "reply message", func() error {
		var apiErr error
		resp, apiErr = a.api.Im().Message().Reply(ctx, req)
		return apiErr
	})

	if err != nil {
		return "", fmt.Errorf("feishu: reply message failed: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("feishu API error: code=%d msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data != nil && resp.Data.MessageId != nil {
		return *resp.Data.MessageId, nil
	}
	return "", nil
}

func (a *FeishuAdapter) sendCardReplyAndGetID(ctx context.Context, sessionID, chatID, cardJSON string) (string, error) {
	if replyCtx, ok := a.getSessionReplyContext(sessionID); ok && replyCtx.userMessageID != "" {
		return a.replyToUserMessage(ctx, replyCtx.userMessageID, "interactive", cardJSON)
	}
	return a.createInteractiveMessage(ctx, chatID, cardJSON)
}

func (a *FeishuAdapter) createInteractiveMessage(ctx context.Context, chatID, cardJSON string) (string, error) {
	feishuChatID, err := parseFeishuChatID(chatID)
	if err != nil {
		return "", err
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(feishuChatID).
			MsgType("interactive").
			Content(cardJSON).
			Build()).
		Build()

	var resp *larkim.CreateMessageResp
	err = a.withRetry(ctx, "create interactive message", func() error {
		var apiErr error
		resp, apiErr = a.api.Im().Message().Create(ctx, req)
		return apiErr
	})
	if err != nil {
		return "", fmt.Errorf("feishu: create message failed: %w", err)
	}
	if !resp.Success() {
		return "", fmt.Errorf("feishu API error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data != nil && resp.Data.MessageId != nil {
		return *resp.Data.MessageId, nil
	}
	return "", nil
}

func (a *FeishuAdapter) patchMessage(ctx context.Context, messageID, cardJSON string) error {
	req := larkim.NewPatchMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewPatchMessageReqBodyBuilder().
			Content(cardJSON).
			Build()).
		Build()

	var resp *larkim.PatchMessageResp
	err := a.withRetry(ctx, "patch message", func() error {
		var apiErr error
		resp, apiErr = a.api.Im().Message().Patch(ctx, req)
		return apiErr
	})
	if err != nil {
		return fmt.Errorf("feishu: patch message failed: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("feishu API error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// ReplyMessage replies to a specific message in Feishu (without thread context lookup).
func (a *FeishuAdapter) ReplyMessage(ctx context.Context, messageID, content string) error {
	_, err := a.replyToUserMessage(ctx, messageID, "text", fmt.Sprintf(`{"text":"%s"}`, escapeJSON(content)))
	return err
}

func parseFeishuChatID(sessionKey string) (string, error) {
	// sessionKey format: feishu_{chat_id}_{user_id}
	parts := strings.Split(sessionKey, "_")
	if len(parts) < 5 {
		return "", fmt.Errorf("invalid session key: %s", sessionKey)
	}
	return parts[1] + "_" + parts[2], nil
}

func normalizeReactionEmoji(value string) string {
	if value == "none" {
		return ""
	}
	if value == "" {
		return "OnIt"
	}
	return value
}

func normalizeDoneEmoji(value string) string {
	if value == "none" {
		return ""
	}
	return value
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
		case '/':
			builder.WriteString("\\/")
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
	if a.api == nil {
		return fmt.Errorf("feishu API not initialized")
	}

	resp, err := a.api.Get(ctx, "/open-apis/bot/v3/info", nil, larkcore.AccessTokenTypeTenant)
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
