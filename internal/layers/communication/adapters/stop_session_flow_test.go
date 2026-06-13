package adapters

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// Covers: L5 — /stop 后下一条消息应复用当前 session，而非从磁盘恢复旧的大 snapshot session
func TestFeishuAdapter_stop_should_keep_session_mapping(t *testing.T) {
	sessionKey := "feishu_oc_stop_ou_456"
	activeSID := "sess_active_after_stop"

	var stoppedSID string
	mockGW := &mockGatewayAPI{
		stopProcessFunc: func(sessionID string) error {
			stoppedSID = sessionID
			return nil
		},
	}

	mockMsgAPI := &mockMessageAPI{}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	adapter := NewFeishuAdapter(
		nil,
		&FeishuConfig{AppID: "test_app", AppSecret: "test_secret"},
		&config.CommunicationConfig{},
		WithFeishuAPI(mockAPI),
		WithGateway(mockGW),
	)
	adapter.sessionMap.Store(sessionKey, activeSID)

	handled := adapter.handleFeishuCommand(context.Background(), "/stop", sessionKey, "msg_stop_1")
	if !handled {
		t.Fatal("expected /stop to be handled")
	}
	if stoppedSID != activeSID {
		t.Fatalf("StopProcess sessionID = %q, want %q", stoppedSID, activeSID)
	}

	stored, ok := adapter.sessionMap.Load(sessionKey)
	if !ok {
		t.Fatal("sessionMap entry was cleared after /stop")
	}
	if got, _ := stored.(string); got != activeSID {
		t.Fatalf("stored sessionID = %q, want %q", got, activeSID)
	}
}

func TestFeishuAdapter_stop_then_resolve_should_reuse_active_session(t *testing.T) {
	sessionKey := "feishu_oc_stop_flow_ou_456"
	activeSID := "sess_current_9000"
	staleSID := "sess_stale_531kb"

	resolveCalled := false
	mockGW := &mockGatewayAPI{
		stopProcessFunc: func(sessionID string) error { return nil },
		getSessionFunc: func(sessionID string) (*types.Session, error) {
			if sessionID == activeSID {
				return &types.Session{SessionID: activeSID, ChatID: sessionKey}, nil
			}
			return nil, nil
		},
		resolveSessionByChatIDFunc: func(chatID string) (*types.Session, error) {
			resolveCalled = true
			return &types.Session{
				SessionID:       staleSID,
				ChatID:          chatID,
				ContextSnapshot: make([]byte, 531_678),
			}, nil
		},
		createSessionFunc: func(chatID, workDir string) (*types.Session, error) {
			t.Fatal("CreateSession should not run when in-memory map still points to active session")
			return nil, nil
		},
	}

	mockMsgAPI := &mockMessageAPI{}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI}
	mockAPI := &mockFeishuAPI{imAPI: mockImAPI}

	adapter := NewFeishuAdapter(
		nil,
		&FeishuConfig{AppID: "test_app", AppSecret: "test_secret"},
		&config.CommunicationConfig{},
		WithFeishuAPI(mockAPI),
		WithGateway(mockGW),
	)
	adapter.sessionMap.Store(sessionKey, activeSID)

	if !adapter.handleFeishuCommand(context.Background(), "/stop", sessionKey, "msg_stop_2") {
		t.Fatal("expected /stop to be handled")
	}

	session, err := adapter.getOrCreateSession(sessionKey)
	if err != nil {
		t.Fatalf("getOrCreateSession() error = %v", err)
	}
	if session.SessionID != activeSID {
		t.Fatalf("sessionID = %q, want active %q", session.SessionID, activeSID)
	}
	if resolveCalled {
		t.Fatal("ResolveSessionByChatID should not run when sessionMap still maps to active session")
	}
}

func TestResolveOrCreateSession_should_fallback_to_disk_when_map_empty(t *testing.T) {
	sessionKey := "feishu_oc_fallback_ou_456"
	recentSID := "sess_recent"

	resolveCalled := false
	mockGW := &mockGatewayAPI{
		resolveSessionByChatIDFunc: func(chatID string) (*types.Session, error) {
			resolveCalled = true
			if chatID != sessionKey {
				t.Fatalf("chatID = %q", chatID)
			}
			return &types.Session{
				SessionID: recentSID,
				ChatID:    chatID,
			}, nil
		},
		createSessionFunc: func(chatID, workDir string) (*types.Session, error) {
			t.Fatal("CreateSession should not run when restore succeeds")
			return nil, nil
		},
	}

	var sessionMap sync.Map
	session, err := resolveOrCreateSession(mockGW, &sessionMap, sessionKey)
	if err != nil {
		t.Fatalf("resolveOrCreateSession() error = %v", err)
	}
	if !resolveCalled {
		t.Fatal("expected ResolveSessionByChatID fallback when map is empty")
	}
	if session.SessionID != recentSID {
		t.Fatalf("sessionID = %q, want %q", session.SessionID, recentSID)
	}
}

func TestFeishuAdapter_stop_should_clear_stale_stream_state(t *testing.T) {
	sessionKey := "feishu_oc_stop_stream_ou_456"
	activeSID := "sess_stop_stream"

	mockGW := &mockGatewayAPI{
		stopProcessFunc: func(sessionID string) error { return nil },
	}
	mockMsgAPI := &mockMessageAPI{}
	mockAPI := &mockFeishuAPI{imAPI: &mockImAPI{messageAPI: mockMsgAPI}}

	adapter := NewFeishuAdapter(
		nil,
		&FeishuConfig{AppID: "test_app", AppSecret: "test_secret"},
		&config.CommunicationConfig{},
		WithFeishuAPI(mockAPI),
		WithGateway(mockGW),
	)
	adapter.sessionMap.Store(sessionKey, activeSID)

	stream := adapter.sessionStream(activeSID)
	stream.mu.Lock()
	stream.responseMsgID = "om_old_reply"
	stream.replyCardID = "card_old"
	stream.cardkitEnabled = true
	stream.textBuffer.WriteString("interrupted partial")
	stream.agentOutputMsgID = "om_agent_old"
	stream.mu.Unlock()

	if !adapter.handleFeishuCommand(context.Background(), "/stop", sessionKey, "msg_stop_stream") {
		t.Fatal("expected /stop to be handled")
	}

	if _, ok := adapter.sessionStreams.Load(activeSID); ok {
		t.Fatal("expected session stream cleared after /stop")
	}
}

func TestFeishuAdapter_new_inbound_should_reset_stream_before_reply(t *testing.T) {
	var createCardCalls int
	msgID := "om_new_turn"
	mockAPI := &mockFeishuAPI{
		postFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			createCardCalls++
			return &larkcore.ApiResp{
				StatusCode: http.StatusOK,
				RawBody:    []byte(`{"code":0,"data":{"card_id":"card_new_turn"}}`),
			}, nil
		},
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
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

	sessionID := "sess_new_turn"
	adapter := NewFeishuAdapter(nil, &FeishuConfig{
		AppID:     "test_app",
		AppSecret: "test_secret",
		Streaming: FeishuStreamingConfig{Enabled: true, IntervalMs: 0, MinDeltaChars: 1},
	}, &config.CommunicationConfig{}, WithFeishuAPI(mockAPI))

	// Simulate stale stream left over from an interrupted prior turn.
	stale := adapter.sessionStream(sessionID)
	stale.mu.Lock()
	stale.responseMsgID = "om_old"
	stale.replyCardID = "card_old"
	stale.cardkitEnabled = true
	stale.textBuffer.WriteString("stale")
	stale.mu.Unlock()

	// Simulate what onMessage does before routing a new user turn.
	adapter.clearSessionStream(sessionID)
	adapter.sessionReplyCtx.Store(sessionID, feishuReplyContext{userMessageID: "om_user_new"})

	err := adapter.appendResponseText(context.Background(), sessionID, "feishu_oc_123_ou_456", "你好")
	if err != nil {
		t.Fatalf("appendResponseText() error = %v", err)
	}
	if createCardCalls != 1 {
		t.Fatalf("createCardCalls = %d, want 1 (fresh card for new turn)", createCardCalls)
	}

	stream := adapter.sessionStream(sessionID)
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.replyCardID != "card_new_turn" {
		t.Fatalf("replyCardID = %q, want card_new_turn", stream.replyCardID)
	}
	if stream.textBuffer.String() != "你好" {
		t.Fatalf("textBuffer = %q, want 你好", stream.textBuffer.String())
	}
}
