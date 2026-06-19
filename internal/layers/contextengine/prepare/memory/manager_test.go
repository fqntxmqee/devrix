package memory_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/persist/snapshot"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S3-A01-T01, D2-S3-A01-T01
func TestManager_should_initialize_new_session_context(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)
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

// T: D2-S3-A01-T01
func TestManager_should_append_user_message_and_dedupe_request_id(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)
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

// T: D2-S3-A01-T02
func TestManager_should_restore_from_backup_when_session_snapshot_empty(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	cfg.Snapshot.Enabled = true
	cfg.Snapshot.BackupDir = t.TempDir()
	store := snapshot.NewStore(&cfg.Snapshot)
	mgr := memory.NewManager(cfg, store, nil, nil)

	orig := &types.SessionContext{
		SessionID:    "sess_backup",
		WorkDir:      "/tmp",
		SystemPrompt: "prompt",
		Messages:     []types.Message{*types.NewMessage("m1", "sess_backup", types.MessageRoleUser, "prior turn")},
		TokenBudget:  types.DefaultTokenBudget(),
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

func TestManager_RemoveLastUserMessage(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)
	session := types.NewSession("sess_rm", "cli", "/tmp")
	sc, _ := mgr.LoadOrInit(session, "prompt")

	mgr.AppendMessage(sc, types.MessageRoleUser, "msg1")
	mgr.AppendMessage(sc, types.MessageRoleAssistant, "resp1")
	mgr.AppendMessage(sc, types.MessageRoleUser, "msg2")

	mgr.RemoveLastUserMessage(sc)
	if len(sc.Messages) != 2 {
		t.Fatalf("len = %d, want 2", len(sc.Messages))
	}
	if sc.Messages[1].Content == "msg2" {
		t.Fatalf("last user message was not removed: %+v", sc.Messages)
	}
}

func TestManager_RemoveLastUserMessage_noop_when_no_user_message(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)
	session := types.NewSession("sess_rm2", "cli", "/tmp")
	sc, _ := mgr.LoadOrInit(session, "prompt")

	mgr.AppendMessage(sc, types.MessageRoleAssistant, "resp1")
	mgr.AppendMessage(sc, types.MessageRoleAssistant, "resp2")
	mgr.RemoveLastUserMessage(sc)
	if len(sc.Messages) != 2 {
		t.Fatalf("len = %d, want 2 (noop)", len(sc.Messages))
	}
}

func TestManager_RemoveLastUserMessage_only_removes_last_user(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)
	session := types.NewSession("sess_rm3", "cli", "/tmp")
	sc, _ := mgr.LoadOrInit(session, "prompt")

	mgr.AppendMessage(sc, types.MessageRoleUser, "user1")
	mgr.AppendMessage(sc, types.MessageRoleAssistant, "resp1")
	mgr.AppendMessage(sc, types.MessageRoleUser, "user2")

	mgr.RemoveLastUserMessage(sc)
	if len(sc.Messages) != 2 {
		t.Fatalf("len = %d, want 2", len(sc.Messages))
	}
	if sc.Messages[0].Content != "user1" {
		t.Fatalf("first user message was removed but should remain: %+v", sc.Messages)
	}
}

func TestManager_TrimMessages(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.MaxMessages = 8
	cfg.Compression.KeepTailMessages = 4
	cfg.Compression.Autocompact.PreserveHeadTurns = 1
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)
	session := types.NewSession("sess_trim", "cli", "/tmp")
	sc, _ := mgr.LoadOrInit(session, "prompt")

	mgr.AppendMessage(sc, types.MessageRoleUser, "first task")
	for i := range 20 {
		role := types.MessageRoleAssistant
		if i%2 == 0 {
			role = types.MessageRoleUser
		}
		mgr.AppendMessage(sc, role, fmt.Sprintf("msg%d", i))
	}

	mgr.TrimMessages(sc)
	if len(sc.Messages) > 8 {
		t.Fatalf("len = %d, want <= 8", len(sc.Messages))
	}
	if sc.Messages[0].Content != "first task" {
		t.Fatalf("first message = %q, want first task preserved", sc.Messages[0].Content)
	}
}

func TestManager_TrimMessages_noop_when_within_limit(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)
	session := types.NewSession("sess_trim2", "cli", "/tmp")
	sc, _ := mgr.LoadOrInit(session, "prompt")

	mgr.AppendMessage(sc, types.MessageRoleUser, "a")
	mgr.AppendMessage(sc, types.MessageRoleAssistant, "b")

	mgr.TrimMessages(sc)
	if len(sc.Messages) != 2 {
		t.Fatalf("len = %d, want 2 (noop)", len(sc.Messages))
	}
}

func TestManager_TrimMessages_should_repair_incomplete_chain_when_within_limit(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)
	session := types.NewSession("sess_trim4", "cli", "/tmp")
	sc, _ := mgr.LoadOrInit(session, "prompt")

	calls := `[{"id":"call_stop","type":"function","function":{"name":"call_claude-code","arguments":"{}"}}]`
	mgr.AppendMessage(sc, types.MessageRoleUser, "test claude")
	mgr.AppendFullMessage(sc, types.Message{
		Role:     types.MessageRoleAssistant,
		Content:  "",
		Metadata: map[string]string{"tool_calls": calls},
	})

	mgr.TrimMessages(sc)
	if len(sc.Messages) != 1 {
		t.Fatalf("len = %d, want 1 (incomplete assistant dropped)", len(sc.Messages))
	}
	if sc.Messages[0].Role != types.MessageRoleUser {
		t.Fatalf("role = %s, want user", sc.Messages[0].Role)
	}
}

func TestManager_StopCleanup_should_repair_incomplete_tool_round(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)
	session := types.NewSession("sess_stop_cleanup", "cli", "/tmp")
	sc, _ := mgr.LoadOrInit(session, "prompt")

	calls := `[{"id":"call_claude","type":"function","function":{"name":"call_claude-code","arguments":"{}"}}]`
	mgr.AppendMessage(sc, types.MessageRoleUser, "测试 call claude")
	mgr.AppendFullMessage(sc, types.Message{
		Role:     types.MessageRoleAssistant,
		Content:  "",
		Metadata: map[string]string{"tool_calls": calls},
	})

	// Mirrors engine.go cancel path on /stop.
	mgr.RemoveLastUserMessage(sc)
	mgr.TrimMessages(sc)

	for _, m := range sc.Messages {
		if m.Role == types.MessageRoleAssistant && m.Metadata["tool_calls"] != "" {
			t.Fatal("incomplete assistant tool_calls should be removed after stop cleanup")
		}
	}
}

func TestManager_TrimMessages_repairs_chain_after_trim(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.MaxMessages = 2
	cfg.Compression.KeepTailMessages = 1
	cfg.Compression.Autocompact.PreserveHeadTurns = 1
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)
	session := types.NewSession("sess_trim3", "cli", "/tmp")
	sc, _ := mgr.LoadOrInit(session, "prompt")

	calls := `[{"id":"call_x","type":"function","function":{"name":"test","arguments":"{}"}}]`
	mgr.AppendMessage(sc, types.MessageRoleUser, "do it")
	mgr.AppendFullMessage(sc, types.Message{
		Role:     types.MessageRoleAssistant,
		Content:  "ok",
		Metadata: map[string]string{"tool_calls": calls},
	})
	mgr.AppendMessage(sc, types.MessageRoleTool, "result")
	mgr.AppendMessage(sc, types.MessageRoleUser, "more")

	mgr.TrimMessages(sc)
	if len(sc.Messages) == 0 {
		t.Fatal("expected at least one message after trim")
	}
	last := sc.Messages[len(sc.Messages)-1]
	if last.Content != "more" {
		t.Fatalf("expected tail user 'more', got %q", last.Content)
	}
}

func TestManager_LoadOrInit_cache_hit_repairs_messages(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	store := snapshot.NewStore(&cfg.Snapshot)
	mgr := memory.NewManager(cfg, store, nil, nil)
	session := types.NewSession("sess_cache", "cli", "/tmp")

	// Load fresh
	sc, _ := mgr.LoadOrInit(session, "prompt")
	sc.Messages = append(sc.Messages, *types.NewMessage("m1", "sess_cache", types.MessageRoleUser, "hi"))

	// Second load returns cached, should repair (no orphan)
	sc2, err := mgr.LoadOrInit(session, "prompt")
	if err != nil {
		t.Fatalf("LoadOrInit: %v", err)
	}
	if sc2 != sc {
		t.Fatal("expected same pointer (cache hit)")
	}
	if len(sc2.Messages) != 1 {
		t.Fatalf("len = %d, want 1", len(sc2.Messages))
	}
}

func TestManager_LoadOrInit_cache_hit_drops_orphan_tool_results(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	store := snapshot.NewStore(&cfg.Snapshot)
	mgr := memory.NewManager(cfg, store, nil, nil)
	session := types.NewSession("sess_cache2", "cli", "/tmp")

	// Load fresh, inject corrupted state (tool result without preceding assistant)
	sc, _ := mgr.LoadOrInit(session, "prompt")
	sc.Messages = []types.Message{
		{ID: "1", Role: types.MessageRoleUser, Content: "hi"},
		{ID: "2", Role: types.MessageRoleTool, Content: "orphan", Metadata: map[string]string{"tool_call_id": "call_orphan"}},
	}

	// Second load should repair: drop orphan tool result
	sc2, _ := mgr.LoadOrInit(session, "prompt")
	if len(sc2.Messages) != 1 {
		t.Fatalf("len = %d, want 1 (orphan tool result dropped)", len(sc2.Messages))
	}
	if sc2.Messages[0].Content != "hi" {
		t.Fatalf("expected 'hi', got %q", sc2.Messages[0].Content)
	}
}

func TestManager_AppendFullMessage(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)
	session := types.NewSession("sess_full", "cli", "/tmp")
	sc, _ := mgr.LoadOrInit(session, "prompt")

	msg := types.Message{
		Role:    types.MessageRoleAssistant,
		Content: "hello",
		Metadata: map[string]string{
			"tool_calls": `[{"id":"c1","type":"function","function":{"name":"bash","arguments":"{}"}}]`,
		},
	}
	mgr.AppendFullMessage(sc, msg)
	if len(sc.Messages) != 1 {
		t.Fatalf("len = %d, want 1", len(sc.Messages))
	}
	got := sc.Messages[0]
	if got.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if got.SessionID != "sess_full" {
		t.Fatalf("session = %q", got.SessionID)
	}
	if got.Timestamp.IsZero() {
		t.Fatal("expected timestamp")
	}
}

func TestManager_PersistSnapshot(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	cfg.Snapshot.BackupDir = t.TempDir()
	store := snapshot.NewStore(&cfg.Snapshot)
	mgr := memory.NewManager(cfg, store, nil, nil)
	session := types.NewSession("sess_persist", "cli", "/tmp")
	sc, _ := mgr.LoadOrInit(session, "prompt")

	mgr.AppendMessage(sc, types.MessageRoleUser, "test")
	data, err := mgr.PersistSnapshot(sc)
	if err != nil {
		t.Fatalf("PersistSnapshot: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty snapshot data")
	}

	// Verify it round-trips
	sc2, err := store.Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}
	if len(sc2.Messages) != 1 || sc2.Messages[0].Content != "test" {
		t.Fatalf("round-trip: %+v", sc2.Messages)
	}
}

func TestManager_Get_returns_nil_for_missing(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)

	sc, ok := mgr.Get("nonexistent")
	if ok {
		t.Fatal("expected false for missing session")
	}
	if sc != nil {
		t.Fatal("expected nil for missing session")
	}
}

// T: D2-S3-A01-T03 (tool message dedup without call_id)
// Migrated from internal/layers/contextengine/legacy/engine_tool_history_test.go
// during the 2026-06-19 D2 legacy test cleanup. The legacy file tested
// real production code (`memory.Manager.AppendFullMessage`) that belongs
// next to its source in prepare/memory/, not in the legacy/ deprecation home.
func TestManager_AppendFullMessage_should_not_duplicate_tool_messages_without_call_id(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil, nil)
	sc := &types.SessionContext{
		SessionID:   "sess_tool_hist",
		Messages:    []types.Message{{Role: types.MessageRoleUser, Content: "first"}},
		TokenBudget: types.DefaultTokenBudget(),
	}

	callID := "call_cursor_0"
	tcJSON, _ := json.Marshal([]map[string]string{
		{
			"id":   callID,
			"type": "function",
		},
	})
	tcMsg := types.Message{
		Role:     types.MessageRoleAssistant,
		Metadata: map[string]string{"tool_calls": string(tcJSON)},
	}
	mgr.AppendFullMessage(sc, tcMsg)

	resultMsg := types.Message{
		Role:     types.MessageRoleTool,
		Content:  "ok",
		Metadata: map[string]string{"tool_call_id": callID},
	}
	mgr.AppendFullMessage(sc, resultMsg)

	if len(sc.Messages) != 3 {
		t.Fatalf("message count = %d, want 3", len(sc.Messages))
	}
	toolMsg := sc.Messages[2]
	if toolMsg.Role != types.MessageRoleTool {
		t.Fatalf("third message role = %s", toolMsg.Role)
	}
	if toolMsg.Metadata["tool_call_id"] != callID {
		t.Fatalf("tool_call_id = %q", toolMsg.Metadata["tool_call_id"])
	}
}
