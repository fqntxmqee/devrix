package memory_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/snapshot"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

func tempSQLiteDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "memory.db")
}

// T: D2-S3-A01-T05
func TestLongTermMemory_should_return_not_implemented_when_disabled(t *testing.T) {
	lt := memory.NewDisabledLongTermMemory()
	_, err := lt.Recall(context.Background(), "query", 5)
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.ErrorCode(err) != errors.CodeMemoryNotImplemented {
		t.Errorf("unexpected code: %v", err)
	}
}

// T: D2-S3-A01-T04
func TestSQLiteLongTermMemory_should_store_and_recall_entries(t *testing.T) {
	dbPath := tempSQLiteDB(t)
	lt, err := memory.OpenSQLiteLongTerm(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer lt.Close()

	ctx := context.Background()
	if err := lt.Store(ctx, memory.MemoryEntry{
		ID:        "mem-1",
		SessionID: "sess-1",
		Topic:     "architecture",
		Content:   "use SQLite for long-term memory",
	}); err != nil {
		t.Fatalf("store: %v", err)
	}

	entries, err := lt.Recall(ctx, "SQLite", 5)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Topic != "architecture" {
		t.Fatalf("unexpected topic: %s", entries[0].Topic)
	}
}

// T: D2-S3-A01-T03
func TestManager_should_inject_longterm_recall_into_system_prompt(t *testing.T) {
	dbPath := tempSQLiteDB(t)
	lt, err := memory.OpenSQLiteLongTerm(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer lt.Close()

	ctx := context.Background()
	if err := lt.Store(ctx, memory.MemoryEntry{
		SessionID: "sess-2",
		Topic:     "bugs",
		Content:   "fixed gauge precision issue",
	}); err != nil {
		t.Fatalf("store: %v", err)
	}

	cfg := config.DefaultContextEngineConfig()
	cfg.LongTerm.Enabled = true
	cfg.LongTerm.RecallMaxEntries = 5
	cfg.LongTerm.RecallMaxTokens = 500

	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), lt)
	session := types.NewSession("sess-2", "cli", "/tmp")
	sc, err := mgr.LoadOrInit(session, "base prompt")
	if err != nil {
		t.Fatalf("LoadOrInit: %v", err)
	}
	if err := mgr.EnrichWithLongTermRecall(ctx, sc, "gauge"); err != nil {
		t.Fatalf("recall inject: %v", err)
	}
	if !strings.Contains(sc.SystemPrompt, "## 项目记忆（LongTerm）") {
		t.Fatalf("expected longterm appendix, got: %s", sc.SystemPrompt)
	}
	if !strings.Contains(sc.SystemPrompt, "[bugs]") {
		t.Fatalf("expected topic in appendix, got: %s", sc.SystemPrompt)
	}
}

// T: D2-S3-A01-T04
func TestManager_should_auto_store_when_enabled(t *testing.T) {
	dbPath := tempSQLiteDB(t)
	lt, err := memory.OpenSQLiteLongTerm(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer lt.Close()

	cfg := config.DefaultContextEngineConfig()
	cfg.LongTerm.Enabled = true
	cfg.LongTerm.AutoStore = true
	cfg.LongTerm.Topics = []string{"architecture", "decisions"}

	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), lt)
	session := types.NewSession("sess-3", "cli", "/tmp")
	sc, _ := mgr.LoadOrInit(session, "prompt")

	ctx := context.Background()
	if err := mgr.AutoStoreLongTerm(ctx, sc, "discuss architecture choices", "chose SQLite over JSON files"); err != nil {
		t.Fatalf("auto store: %v", err)
	}

	entries, err := lt.Recall(ctx, "SQLite", 5)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 stored entry, got %d", len(entries))
	}
	if entries[0].Topic != "architecture" {
		t.Fatalf("expected architecture topic, got %s", entries[0].Topic)
	}
}

func TestFormatLongTermAppendix_should_respect_token_budget(t *testing.T) {
	entries := []memory.MemoryEntry{
		{Topic: "a", Content: strings.Repeat("x", 500)},
		{Topic: "b", Content: strings.Repeat("y", 500)},
	}
	appendix := memory.FormatLongTermAppendix(entries, 50)
	if len(appendix) > 50*4+100 {
		t.Fatalf("appendix too long: %d chars", len(appendix))
	}
}

func TestNewLongTermFromConfig_should_use_temp_db_when_enabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.db")
	cfg := config.LongTermConfig{
		Enabled: true,
		DBPath:  path,
	}
	lt, err := memory.NewLongTermFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewLongTermFromConfig: %v", err)
	}
	defer lt.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected db file created: %v", err)
	}
}
