package memory_test

import (
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/snapshot"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-CTX-01, L5-CTX-02
func TestManager_should_initialize_new_session_context(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil)
	session := types.NewSession("sess_1", "cli", "/tmp")

	sc, err := mgr.LoadOrInit(session, "system prompt")
	if err != nil {
		t.Fatalf("LoadOrInit: %v", err)
	}
	if sc.SystemPrompt != "system prompt" {
		t.Errorf("expected system prompt")
	}
	if len(sc.Messages) != 0 {
		t.Errorf("expected empty messages")
	}
}

// Covers: L5-CTX-02
func TestManager_should_append_user_message_and_dedupe_request_id(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil)
	session := types.NewSession("sess_2", "cli", "/tmp")
	sc, _ := mgr.LoadOrInit(session, "prompt")

	if !mgr.AppendUserMessage(sc, "req-1", "hello") {
		t.Fatal("expected first append")
	}
	if mgr.AppendUserMessage(sc, "req-1", "hello") {
		t.Fatal("expected duplicate skipped")
	}
	if len(sc.Messages) != 1 || sc.Messages[0].Content != "hello" {
		t.Errorf("unexpected messages: %+v", sc.Messages)
	}
}

// Covers: L5-CTX-05
func TestManager_should_restore_from_backup_when_session_snapshot_empty(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	cfg.Snapshot.Enabled = true
	cfg.Snapshot.BackupDir = t.TempDir()
	store := snapshot.NewStore(&cfg.Snapshot)
	mgr := memory.NewManager(cfg, store, nil)

	orig := &types.SessionContext{
		SessionID:    "sess_backup",
		WorkDir:      "/tmp",
		SystemPrompt: "prompt",
		Messages:     []types.Message{*types.NewMessage("m1", "sess_backup", types.MessageRoleUser, "prior turn")},
		TokenBudget:  types.DefaultTokenBudget(),
		PEVState:     types.DefaultPEVState(3),
	}
	data, err := store.Serialize(orig)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	if err := store.WriteBackup(orig.SessionID, data); err != nil {
		t.Fatalf("WriteBackup: %v", err)
	}

	session := types.NewSession(orig.SessionID, "cli", orig.WorkDir)
	sc, err := mgr.LoadOrInit(session, "fresh prompt")
	if err != nil {
		t.Fatalf("LoadOrInit: %v", err)
	}
	if len(sc.Messages) != 1 || sc.Messages[0].Content != "prior turn" {
		t.Fatalf("messages = %+v, want prior turn restored from backup", sc.Messages)
	}
	if _, err := filepath.Abs(cfg.Snapshot.BackupDir); err != nil {
		t.Fatalf("backup dir: %v", err)
	}
}
