//go:build acceptance

package p0_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	milestonebridge "github.com/devrix/devrix/internal/bridges/milestone"
	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type accPlanLLM struct {
	planJSON         string
	lastSystemPrompt string
}

func (p *accPlanLLM) ChatStream(_ context.Context, req *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	p.lastSystemPrompt = req.SystemPrompt
	ch := make(chan contextengine.LLMChunk, 2)
	text := "Done."
	if strings.Contains(req.SystemPrompt, "task planner") {
		text = p.planJSON
	}
	go func() {
		ch <- contextengine.LLMChunk{Content: text, Done: true}
		close(ch)
	}()
	return ch, nil
}

// Covers: L5-CTX-19, L5-CTX-21
func TestAcceptance_PlanMilestoneP0(t *testing.T) {
	msSvc := milestone.NewMilestoneService(nil)
	planner := milestonebridge.NewPlannerAdapter(msSvc)

	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.Plan.Enabled = true
	ctxCfg.Plan.AutoDetect = false

	planJSON := `{
		"task_id": "task_accept",
		"milestones": [
			{"id": "ms_a", "name": "analyze", "description": "read", "dependencies": []},
			{"id": "ms_b", "name": "fix", "description": "patch", "dependencies": ["ms_a"]}
		]
	}`

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &accPlanLLM{planJSON: planJSON},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   registry.NewBuiltinRegistry(),
		Permission: mockctx.AllowAllPermission{},
		Planner:    planner,
		Config:     ctxCfg,
	})

	session := &types.Session{
		SessionID: "sess_accept_plan",
		WorkDir:   t.TempDir(),
		Model:     "mock",
	}

	ch := engine.Process(context.Background(), session, "/plan refactor auth module")
	var progress int
	for ev := range ch {
		if ev.Type == "milestone_progress" {
			progress++
			if ev.Metadata["milestone_id"] == "" || ev.Metadata["task"] == "" {
				t.Fatalf("invalid milestone_progress metadata: %+v", ev.Metadata)
			}
		}
	}
	if progress < 2 {
		t.Fatalf("expected >=2 milestone_progress events, got %d", progress)
	}
}

// Covers: L5-CTX-22
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
	ctxCfg.Plan.Enabled = false
	ctxCfg.LongTerm.Enabled = true
	ctxCfg.LongTerm.RecallMaxEntries = 5
	ctxCfg.LongTerm.RecallMaxTokens = 500

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        llm,
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   registry.NewBuiltinRegistry(),
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
