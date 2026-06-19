package contextengine_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S10-A01-T34 (engine integration via D7 PreparedTurnRunner)
func TestContextEngine_query_loop_enabled_multi_turn(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	cfg.Harness.Enabled = false
	cfg.QueryLoop.MaxTurns = 5

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		PreparedTurnRunner: &multiTurnPreparedTurnRunner{},
		Summarizer:         &mockctx.StaticSummarizer{},
		Tools:              &mockctx.ToolRunner{Output: "tool output"},
		ToolsReg:           mustBuiltinRegistry(t),
		Permission:         mockctx.AllowAllPermission{},
		Config:             cfg,
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
		t.Fatalf("expected at least one tool call event, got %d", toolCalls)
	}
}
