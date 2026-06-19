package contextengine_test

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func newTestEngine(t *testing.T) *contextengine.ContextEngine {
	t.Helper()
	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = false
	// Disable on-disk snapshots in tests to prevent cross-test pollution via
	// the shared ~/.devrix/context backup dir. Manager.LoadOrInit falls back
	// to ReadBackup when session.ContextSnapshot is empty, which previously
	// caused tests to load stale state from previous runs.
	cfg.Snapshot.Enabled = false
	return contextengine.NewContextEngine(contextengine.EngineDeps{
		PreparedTurnRunner: &mockctx.StaticPreparedTurnRunner{Response: "ok"},
		Summarizer:           &mockctx.StaticSummarizer{},
		Tools:          &mockctx.ToolRunner{Output: "ok"},
		ToolsReg:       mustBuiltinRegistry(t),
		Permission:     mockctx.AllowAllPermission{},
		Config:         cfg,
	})
}

func TestEngine_Shutdown_NoAsyncCompact(t *testing.T) {
	engine := newTestEngine(t)
	err := engine.Shutdown(5 * time.Second)
	if err != nil {
		t.Errorf("Shutdown: expected nil, got %v", err)
	}
}

func TestEngine_SessionContext_NotFound(t *testing.T) {
	engine := newTestEngine(t)
	_, ok := engine.SessionContext("nonexistent")
	if ok {
		t.Error("SessionContext: expected false for nonexistent session")
	}
}

func TestEngine_ExportSessionSnapshot_NotFound(t *testing.T) {
	engine := newTestEngine(t)
	_, err := engine.ExportSessionSnapshot("nonexistent")
	if err == nil {
		t.Error("ExportSessionSnapshot: expected error for nonexistent session")
	}
}

func TestEngine_ToolRunner(t *testing.T) {
	engine := newTestEngine(t)
	tr := engine.ToolRunner()
	if tr == nil {
		t.Error("ToolRunner: expected non-nil")
	}
}

func TestEngine_PermissionGate(t *testing.T) {
	engine := newTestEngine(t)
	pg := engine.PermissionGate()
	if pg == nil {
		t.Error("PermissionGate: expected non-nil")
	}
}

func TestEngine_Process_BasicFlow(t *testing.T) {
	engine := newTestEngine(t)
	session := types.NewSession("sess-accessor-test", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "hello")
	for ev := range ch {
		if ev.Type == "error" {
			t.Errorf("Process: unexpected error event: %s", ev.Content)
		}
	}
}

func TestEngine_ExportSessionSnapshot_AfterProcess(t *testing.T) {
	engine := newTestEngine(t)
	session := types.NewSession("sess-snap-1", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "hello")
	for range ch {
	}
	data, err := engine.ExportSessionSnapshot("sess-snap-1")
	if err != nil {
		t.Errorf("ExportSessionSnapshot after Process: %v", err)
	}
	if len(data) == 0 {
		t.Error("ExportSessionSnapshot: expected non-empty data")
	}
}

func TestEngine_SessionContext_AfterProcess(t *testing.T) {
	engine := newTestEngine(t)
	session := types.NewSession("sess-ctx-1", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "hello")
	for range ch {
	}
	sc, ok := engine.SessionContext("sess-ctx-1")
	if !ok {
		t.Fatal("SessionContext: expected session to exist after Process")
	}
	if sc.SessionID != "sess-ctx-1" {
		t.Errorf("SessionContext: sessionID = %s, want sess-ctx-1", sc.SessionID)
	}
}

func TestRegisterPlanModeTools(t *testing.T) {
	reg := contextengine.NewToolRegistry()
	err := contextengine.RegisterPlanModeTools(reg, config.DefaultContextEngineConfig())
	if err != nil {
		t.Errorf("RegisterPlanModeTools: %v", err)
	}
}

func TestRegisterPlanModeTools_NilRegistry(t *testing.T) {
	err := contextengine.RegisterPlanModeTools(nil, config.DefaultContextEngineConfig())
	if err != nil {
		t.Errorf("RegisterPlanModeTools nil reg: %v", err)
	}
}

func TestRegisterPlanModeTools_NilConfig(t *testing.T) {
	reg := contextengine.NewToolRegistry()
	err := contextengine.RegisterPlanModeTools(reg, nil)
	if err != nil {
		t.Errorf("RegisterPlanModeTools nil cfg: %v", err)
	}
}
