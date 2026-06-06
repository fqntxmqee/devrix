package types

import (
	"testing"
	"time"
)

func TestSession_SetState(t *testing.T) {
	session := NewSession("sess_1", "cli", "/tmp")
	initialTime := session.UpdatedAt

	session.SetState(SessionStateThinking)

	if session.State != SessionStateThinking {
		t.Errorf("expected state 'thinking', got '%s'", session.State)
	}
	if !session.UpdatedAt.After(initialTime) {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestSession_IsIdle(t *testing.T) {
	session := NewSession("sess_1", "cli", "/tmp")

	// Recent message
	session.LastMessageAt = time.Now()
	if session.IsIdle(30 * time.Minute) {
		t.Error("expected not idle for recent message")
	}

	// Old message
	session.LastMessageAt = time.Now().Add(-1 * time.Hour)
	if !session.IsIdle(30 * time.Minute) {
		t.Error("expected idle for message older than 30 minutes")
	}
}

func TestNewSession(t *testing.T) {
	session := NewSession("sess_123", "cli", "/workspace")

	if session.SessionID != "sess_123" {
		t.Errorf("expected SessionID 'sess_123', got '%s'", session.SessionID)
	}
	if session.AdapterID != "cli" {
		t.Errorf("expected AdapterID 'cli', got '%s'", session.AdapterID)
	}
	if session.WorkDir != "/workspace" {
		t.Errorf("expected WorkDir '/workspace', got '%s'", session.WorkDir)
	}
	if session.State != SessionStateIdle {
		t.Errorf("expected state 'idle', got '%s'", session.State)
	}
	if session.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if session.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
	if session.LastMessageAt.IsZero() {
		t.Error("expected LastMessageAt to be set")
	}
}
