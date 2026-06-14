package adapters

import (
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestResolveOrCreateSession_should_restore_from_store(t *testing.T) {
	sessionKey := "feishu_oc_123456_ou_654321"
	restored := &types.Session{
		SessionID: "sess_restored_123",
		ChatID:    sessionKey,
	}

	mockGW := &mockGatewayAPI{
		resolveSessionByChatIDFunc: func(chatID string) (*types.Session, error) {
			if chatID != sessionKey {
				t.Fatalf("chatID = %s, want %s", chatID, sessionKey)
			}
			return restored, nil
		},
		createSessionFunc: func(chatID, workDir string) (*types.Session, error) {
			t.Fatal("CreateSession should not be called when restore succeeds")
			return nil, nil
		},
	}

	var sessionMap sync.Map
	session, err := resolveOrCreateSession(mockGW, &sessionMap, sessionKey)
	if err != nil {
		t.Fatalf("resolveOrCreateSession() error = %v", err)
	}
	if session.SessionID != restored.SessionID {
		t.Fatalf("sessionID = %s, want %s", session.SessionID, restored.SessionID)
	}

	stored, ok := sessionMap.Load(sessionKey)
	if !ok {
		t.Fatal("expected session map entry")
	}
	if stored.(string) != restored.SessionID {
		t.Fatalf("stored sessionID = %v, want %s", stored, restored.SessionID)
	}
}

func TestResolveOrCreateSession_should_create_when_restore_misses(t *testing.T) {
	sessionKey := "feishu_oc_new_ou_654321"
	var createCalled bool

	mockGW := &mockGatewayAPI{
		resolveSessionByChatIDFunc: func(chatID string) (*types.Session, error) {
			return nil, nil
		},
		createSessionFunc: func(chatID, workDir string) (*types.Session, error) {
			createCalled = true
			return &types.Session{
				SessionID: "sess_new_456",
				ChatID:    chatID,
			}, nil
		},
	}

	var sessionMap sync.Map
	session, err := resolveOrCreateSession(mockGW, &sessionMap, sessionKey)
	if err != nil {
		t.Fatalf("resolveOrCreateSession() error = %v", err)
	}
	if !createCalled {
		t.Fatal("expected CreateSession to be called")
	}
	if session.SessionID != "sess_new_456" {
		t.Fatalf("sessionID = %s, want sess_new_456", session.SessionID)
	}
}
