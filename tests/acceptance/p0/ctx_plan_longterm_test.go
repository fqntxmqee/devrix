//go:build acceptance && d2

package p0_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/registry"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

type accPlanPreparedTurn struct {
	lastSystemPrompt string
}

func (p *accPlanPreparedTurn) RunPreparedTurn(_ context.Context, req contracts.PreparedTurnRequest) (*contracts.PreparedTurnResult, error) {
	p.lastSystemPrompt = req.SystemPrompt
	if req.Emit != nil {
		req.Emit(&contracts.EngineEvent{Type: "complete", SessionID: req.SessionID, Content: "Done."})
	}
	return &contracts.PreparedTurnResult{AssistantText: "Done."}, nil
}

// T: D2-S3-A01-T03
func TestAcceptance_LongTermRecallP0(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	lt, err := memory.OpenSQLiteLongTerm(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer lt.Close()

	ctx := context.Background()
	if err := lt.Store(ctx, memory.MemoryEntry{
		SessionID: "sess_lt",
		Topic:     "architecture",
		Content:   "prefer SQLite for cross-session memory",
	}); err != nil {
		t.Fatalf("store: %v", err)
	}

	turn := &accPlanPreparedTurn{}
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = true
	ctxCfg.LongTerm.RecallMaxEntries = 5
	ctxCfg.LongTerm.RecallMaxTokens = 500

	toolsReg, err := registry.NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry: %v", err)
	}

	engine := kernel.NewContextEngine(kernel.EngineDeps{
		PreparedTurnRunner: turn,
		Summarizer:         &contextengine.StaticSummarizer{},
		Tools:              &enforce.ToolRunner{},
		ToolsReg:           toolsReg,
		Permission:         enforce.AllowAllPermission{},
		LongTerm:           lt,
		Config:             ctxCfg,
	})

	session := &types.Session{
		SessionID: "sess_lt",
		WorkDir:   t.TempDir(),
		Model:     "mock",
	}

	ch := engine.Process(context.Background(), session, "SQLite")
	for range ch {
	}

	if !strings.Contains(turn.lastSystemPrompt, "<memory_context>") {
		t.Fatalf("expected memory_context block in system prompt, got: %q", turn.lastSystemPrompt)
	}
	if !strings.Contains(turn.lastSystemPrompt, "[architecture]") {
		t.Fatalf("expected recalled topic in system prompt, got: %q", turn.lastSystemPrompt)
	}
}
