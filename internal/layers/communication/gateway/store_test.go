package gateway

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestFileSessionStore_Create(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session := types.NewSession("sess_123", "cli", "/tmp")
	session.Model = "claude-3-5-sonnet"

	if err := store.Create(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, "sess_123.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("session file was not created")
	}
}

func TestFileSessionStore_Get(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session := types.NewSession("sess_456", "cli", "/tmp")

	if err := store.Create(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	got, err := store.Get("sess_456")
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if got == nil {
		t.Fatal("expected session, got nil")
	}

	if got.SessionID != "sess_456" {
		t.Errorf("expected session ID 'sess_456', got '%s'", got.SessionID)
	}
}

func TestFileSessionStore_Get_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	got, err := store.Get("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != nil {
		t.Error("expected nil for nonexistent session")
	}
}

func TestFileSessionStore_Update(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session := types.NewSession("sess_789", "cli", "/tmp")
	session.Model = "claude-3-5-sonnet"

	if err := store.Create(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Update session
	session.Model = "claude-3-5-haiku"
	session.State = types.SessionStateThinking

	if err := store.Update(session); err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	got, err := store.Get("sess_789")
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if got.Model != "claude-3-5-haiku" {
		t.Errorf("expected model 'claude-3-5-haiku', got '%s'", got.Model)
	}

	if got.State != types.SessionStateThinking {
		t.Errorf("expected state 'thinking', got '%s'", got.State)
	}
}

func TestFileSessionStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session := types.NewSession("sess_delete", "cli", "/tmp")

	if err := store.Create(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := store.Delete("sess_delete"); err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}

	got, err := store.Get("sess_delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != nil {
		t.Error("expected nil after deletion")
	}
}

func TestFileSessionStore_List(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		session := types.NewSession("sess_list_"+string(rune('a'+i)), "cli", "/tmp")
		if err := store.Create(session); err != nil {
			t.Fatalf("failed to create session: %v", err)
		}
	}

	sessions, err := store.List()
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestFileSessionStore_GetIdleSessions(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create an idle session (last message 1 hour ago)
	idleSession := types.NewSession("sess_idle", "cli", "/tmp")
	idleSession.LastMessageAt = time.Now().Add(-1 * time.Hour)
	if err := store.Create(idleSession); err != nil {
		t.Fatalf("failed to create idle session: %v", err)
	}

	// Create a recent session
	recentSession := types.NewSession("sess_recent", "cli", "/tmp")
	if err := store.Create(recentSession); err != nil {
		t.Fatalf("failed to create recent session: %v", err)
	}

	idleSessions, err := store.GetIdleSessions(30 * time.Minute)
	if err != nil {
		t.Fatalf("failed to get idle sessions: %v", err)
	}

	if len(idleSessions) != 1 {
		t.Errorf("expected 1 idle session, got %d", len(idleSessions))
	}

	if idleSessions[0].SessionID != "sess_idle" {
		t.Errorf("expected idle session 'sess_idle', got '%s'", idleSessions[0].SessionID)
	}
}
