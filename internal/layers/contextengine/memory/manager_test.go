package memory_test

import (
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
