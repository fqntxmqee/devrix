//go:build acceptance && d2

package p0_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type accPlanLLM struct {
	lastSystemPrompt string
}

func (p *accPlanLLM) ChatStream(_ context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	p.lastSystemPrompt = req.SystemPrompt
	ch := make(chan llmgateway.Chunk, 2)
	go func() {
		ch <- llmgateway.Chunk{Content: "Done.", Done: true}
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
		LLM:        llm,
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

	if !strings.Contains(llm.lastSystemPrompt, "## 项目记忆（LongTerm）") {
		t.Fatalf("expected longterm appendix in system prompt, got: %q", llm.lastSystemPrompt)
	}
	if !strings.Contains(llm.lastSystemPrompt, "[architecture]") {
		t.Fatalf("expected recalled topic in system prompt, got: %q", llm.lastSystemPrompt)
	}
}
