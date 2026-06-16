//go:build acceptance && d2

package p0_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/registry"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

type accPlanLLM struct {
	lastSystemPrompt string
}

func (p *accPlanLLM) Call(_ context.Context, req contracts.LLMRequest) (<-chan contracts.LLMChunk, error) {
	p.lastSystemPrompt = req.SystemPrompt
	ch := make(chan contracts.LLMChunk, 2)
	go func() {
		ch <- contracts.LLMChunk{Content: "Done.", Done: true}
		close(ch)
	}()
	return ch, nil
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

	llm := &accPlanLLM{}
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = true
	ctxCfg.LongTerm.RecallMaxEntries = 5
	ctxCfg.LongTerm.RecallMaxTokens = 500

	toolsReg, err := registry.NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry: %v", err)
	}

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		QueryLLMCaller: llm,
		Summarizer:     &mockctx.StaticSummarizer{},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   toolsReg,
		Permission: mockctx.AllowAllPermission{},
		LongTerm:   lt,
		Config:     ctxCfg,
	})

	session := &types.Session{
		SessionID: "sess_lt",
		WorkDir:   t.TempDir(),
		Model:     "mock",
	}

	ch := engine.Process(context.Background(), session, "SQLite")
	for range ch {
	}

	if !strings.Contains(llm.lastSystemPrompt, "<memory_context>") {
		t.Fatalf("expected memory_context block in system prompt, got: %q", llm.lastSystemPrompt)
	}
	if !strings.Contains(llm.lastSystemPrompt, "[architecture]") {
		t.Fatalf("expected recalled topic in system prompt, got: %q", llm.lastSystemPrompt)
	}
}
