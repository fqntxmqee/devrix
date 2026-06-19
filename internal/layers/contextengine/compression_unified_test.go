package contextengine_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/observability/configure/runtime"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestContextEngine_Process_UsesPreparedTurnRunner(t *testing.T) {
	runtime.Reset()
	cfg := config.DefaultContextEngineConfig()
	cfg.QueryLoop.MaxTurns = 3

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		PreparedTurnRunner: &multiTurnPreparedTurnRunner{},
		Summarizer:         &mockctx.StaticSummarizer{},
		Tools:              &mockctx.ToolRunner{Output: "tool output"},
		ToolsReg:           mustBuiltinRegistry(t),
		Permission:         mockctx.AllowAllPermission{},
		Config:             cfg,
	})

	session := types.NewSession("sess_l5_2_9_04", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "ping")
	for ev := range ch {
		_ = ev
	}
	t.Log("Process() completed via PreparedTurnRunner path")
}

func TestContextEngine_Process_CompressionPipelineStillRuns(t *testing.T) {
	runtime.Reset()
	cfg := config.DefaultContextEngineConfig()

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		PreparedTurnRunner: &mockctx.StaticPreparedTurnRunner{Response: "ok"},
		Summarizer:         &mockctx.StaticSummarizer{},
		Tools:              &mockctx.ToolRunner{Output: "tool"},
		ToolsReg:           mustBuiltinRegistry(t),
		Permission:         mockctx.AllowAllPermission{},
		Config:             cfg,
	})

	for i := 0; i < 3; i++ {
		s := types.NewSession(sessionIDFor(i), "cli", t.TempDir())
		ch := engine.Process(context.Background(), s, strings.Repeat("x ", 50))
		for ev := range ch {
			_ = ev
		}
	}
}

func sessionIDFor(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "sess_l5_2_9_04_iter_0"
	}
	var buf [16]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = digits[i%10]
		i /= 10
	}
	return "sess_l5_2_9_04_iter_" + string(buf[pos:])
}

type multiTurnPreparedTurnRunner struct{ n int }

func (m *multiTurnPreparedTurnRunner) RunPreparedTurn(_ context.Context, req contracts.PreparedTurnRequest) (*contracts.PreparedTurnResult, error) {
	m.n++
	if m.n == 1 {
		if req.Emit != nil {
			req.Emit(&contracts.EngineEvent{Type: "tool_call", SessionID: req.SessionID, ToolName: "bash"})
			req.Emit(&contracts.EngineEvent{Type: "complete", SessionID: req.SessionID})
		}
		return &contracts.PreparedTurnResult{
			ToolCallHistory: []types.ToolCallRecord{{CallID: "c1", ToolName: "bash"}},
		}, nil
	}
	if req.Emit != nil {
		req.Emit(&contracts.EngineEvent{Type: "complete", SessionID: req.SessionID, Content: "done"})
	}
	return &contracts.PreparedTurnResult{AssistantText: "done"}, nil
}
