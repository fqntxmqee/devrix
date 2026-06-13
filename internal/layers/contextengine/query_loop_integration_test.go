package contextengine_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S10-A01-T34 (engine integration)
func TestContextEngine_query_loop_enabled_multi_turn(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	cfg.Harness.Enabled = false
	cfg.QueryLoop.Enabled = true
	cfg.QueryLoop.MaxTurns = 5

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &twoRoundToolLLM{},
		Tools:      &mockctx.ToolRunner{Output: "tool output"},
		ToolsReg:   mustBuiltinRegistry(t),
		Permission: mockctx.AllowAllPermission{},
		Config:     cfg,
	})

	session := types.NewSession("sess_ql_int", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "run tools")
	var toolCalls int
	for ev := range ch {
		if ev.Type == "tool_call" {
			toolCalls++
		}
		if ev.Type == "error" {
			t.Fatalf("error: %s", ev.Content)
		}
	}
	if toolCalls < 1 {
		t.Fatalf("expected at least one tool call with query loop enabled, got %d", toolCalls)
	}
}

type twoRoundToolLLM struct{ n int }

func (m *twoRoundToolLLM) ChatStream(_ context.Context, req *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	m.n++
	ch := make(chan contextengine.LLMChunk, 1)
	go func() {
		defer close(ch)
		if m.n == 1 {
			ch <- contextengine.LLMChunk{
				ToolCalls: []contextengine.ToolCall{{ID: "c1", Name: "bash", Input: `{"command":"echo hi"}`}},
				Done:      true,
			}
			return
		}
		ch <- contextengine.LLMChunk{Content: "finished", Done: true}
	}()
	_ = req
	return ch, nil
}
