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

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// DingTalkConfig holds DingTalk adapter settings.
type DingTalkConfig struct {
	AppKey       string
	AppSecret    string
	BotCode      string
	CallbackURL  string
	EncryptKey   string
	UseWebhook   bool
	Port         string
	CallbackPath string
}

// DingTalkAdapter integrates DingTalk chatbot callbacks with the capture.
type DingTalkAdapter struct {
	gateway capture.GatewayAPI
	cfg     *config.CommunicationConfig
	dtCfg   *DingTalkConfig
	api     DingTalkAPI

	server *http.Server
	mu     sync.RWMutex
	running bool
	cancel  context.CancelFunc

	sessionMap  sync.Map // sessionKey -> sessionID
	webhookMap  sync.Map // conversationID -> sessionWebhook
	dedupMap    sync.Map // msgID -> timestamp
}

var _ capture.EventHandler = (*DingTalkAdapter)(nil)

type dingTalkInboundPayload struct {
	MsgType        string `json:"msgtype"`
	ConversationID string `json:"conversationId"`
	SenderNick     string `json:"senderNick"`
	MsgID          string `json:"msgId"`
	SessionWebhook string `json:"sessionWebhook"`
	Text           struct {
		Content string `json:"content"`
	} `json:"text"`
}

// DingTalkAdapterOption configures optional dependencies.
type DingTalkAdapterOption func(*DingTalkAdapter)

// WithDingTalkAPI injects a custom API implementation (tests).
func WithDingTalkAPI(api DingTalkAPI) DingTalkAdapterOption {
	return func(a *DingTalkAdapter) {
		a.api = api
	}
}

// WithDingTalkGateway injects the gateway API (tests).
func WithDingTalkGateway(gw capture.GatewayAPI) DingTalkAdapterOption {
	return func(a *DingTalkAdapter) {
		a.gateway = gw
	}
}

// NewDingTalkAdapter creates a DingTalk adapter.
func NewDingTalkAdapter(
	gw capture.GatewayAPI,
	dtCfg *DingTalkConfig,
	cfg *config.CommunicationConfig,
	opts ...DingTalkAdapterOption,
) *DingTalkAdapter {
	if dtCfg == nil {
		dtCfg = &DingTalkConfig{}
	}
	adapter := &DingTalkAdapter{
		gateway: gw,
		cfg:     cfg,
		dtCfg:   dtCfg,
	}
	for _, opt := range opts {
		opt(adapter)
	}
	if adapter.api == nil {
		adapter.api = NewDingTalkHTTPAPI()
	}
	return adapter
}

// SetGateway sets the gateway after construction.
func (a *DingTalkAdapter) SetGateway(gw capture.GatewayAPI) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.gateway = gw
}

// Start starts webhook mode (V3 default).
func (a *DingTalkAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("dingtalk adapter already running")
	}
	a.running = true
	a.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	if _, err := a.api.GetAccessToken(ctx, a.dtCfg.AppKey, a.dtCfg.AppSecret); err != nil {
		slog.Warn("dingtalk: token prefetch failed", "error", err)
	}

	return a.startWebhookMode(ctx)
}

func (a *DingTalkAdapter) startWebhookMode(_ context.Context) error {
	port := a.dtCfg.Port
	if port == "" {
		port = "8081"
	}
	callbackPath := a.dtCfg.CallbackPath
	if callbackPath == "" {
		callbackPath = "/dingtalk/webhook"
	}

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, a.webhookHandler)

	a.server = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		slog.Info("dingtalk: webhook server starting", "port", port, "path", callbackPath)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("dingtalk: webhook server error", "error", err)
		}
	}()
	return nil
}

// Stop shuts down the adapter.
func (a *DingTalkAdapter) Stop() error {
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.server.Shutdown(ctx)
	}
	return nil
}

func (a *DingTalkAdapter) webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	payload, err := parseDingTalkPayload(body)
	if err != nil {
		slog.Warn("dingtalk: invalid payload", "error", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if err := a.handleInbound(r.Context(), payload); err != nil {
		slog.Error("dingtalk: handle inbound failed", "error", err)
		http.Error(w, "handle failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"msg":"ok"}`))
}

func parseDingTalkPayload(body []byte) (*dingTalkInboundPayload, error) {
	var payload dingTalkInboundPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.ConversationID == "" {
		return nil, fmt.Errorf("conversationId required")
	}
	if strings.TrimSpace(payload.Text.Content) == "" {
		return nil, fmt.Errorf("text content required")
	}
	if payload.MsgType == "" {
		payload.MsgType = "text"
	}
	return &payload, nil
}

func (a *DingTalkAdapter) isDuplicateMessage(msgID string) bool {
	if msgID == "" {
		return false
	}
	now := time.Now().Unix()
	a.dedupMap.Range(func(key, value any) bool {
		if ts, ok := value.(int64); ok && now-ts > 300 {
			a.dedupMap.Delete(key)
		}
		return true
	})
	if _, exists := a.dedupMap.LoadOrStore(msgID, now); exists {
		return true
	}
	return false
}

func (a *DingTalkAdapter) handleInbound(ctx context.Context, payload *dingTalkInboundPayload) error {
	if a.isDuplicateMessage(payload.MsgID) {
		return nil
	}
	if payload.SessionWebhook != "" {
		a.webhookMap.Store(payload.ConversationID, payload.SessionWebhook)
	}

	sessionKey := a.buildSessionKey(payload.ConversationID, payload.SenderNick)
	session, err := a.getOrCreateSession(sessionKey)
	if err != nil {
		return err
	}

	inbound := &types.InboundMessage{
		SessionID:  session.SessionID,
		ChatID:     payload.ConversationID,
		UserID:     payload.SenderNick,
		Content:    payload.Text.Content,
		ReceivedAt: time.Now(),
		Metadata: map[string]string{
			"platform": "dingtalk",
			"msg_type": payload.MsgType,
		},
	}

	if a.gateway == nil {
		return fmt.Errorf("gateway not configured")
	}
	return a.gateway.RouteInbound(ctx, inbound)
}

func (a *DingTalkAdapter) buildSessionKey(conversationID, senderNick string) string {
	return fmt.Sprintf("dingtalk_%s_%s", conversationID, senderNick)
}

func (a *DingTalkAdapter) getOrCreateSession(sessionKey string) (*types.Session, error) {
	return resolveOrCreateSession(a.gateway, &a.sessionMap, sessionKey)
}

// OnMessage handles outbound messages from the capture.
func (a *DingTalkAdapter) OnMessage(msg *types.OutboundMessage) {
	if msg == nil {
		return
	}
	ctx := context.Background()
	webhook := msg.Metadata["session_webhook"]
	if webhook == "" {
		if v, ok := a.webhookMap.Load(msg.ChatID); ok {
			webhook = v.(string)
		}
	}
	if webhook == "" {
		slog.Warn("dingtalk: missing session webhook", "chatID", msg.ChatID)
		return
	}
	content := renderDingTalkOutboundContent(msg)
	if err := a.api.SendSessionMessage(ctx, webhook, content); err != nil {
		slog.Error("dingtalk: send message failed", "error", err, "chatID", msg.ChatID)
	}
}

func (a *DingTalkAdapter) OnPermissionRequest(req *types.PermissionRequest) bool {
	return false
}

func (a *DingTalkAdapter) OnError(err error, sessionID string) {
	slog.Error("dingtalk: gateway error", "error", err, "sessionID", sessionID)
}

func (a *DingTalkAdapter) OnStatus(sessionID string, state types.SessionState) {
	slog.Debug("dingtalk: session status", "sessionID", sessionID, "state", state)
}
