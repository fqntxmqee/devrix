package contextengine_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type twoRoundLLM struct {
	calls int
}

func (m *twoRoundLLM) ChatStream(_ context.Context, _ *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	m.calls++
	ch := make(chan contextengine.LLMChunk, 1)
	go func() {
		defer close(ch)
		if m.calls == 1 {
			ch <- contextengine.LLMChunk{
				ToolCalls: []contextengine.ToolCall{{ID: "tc1", Name: "bash", Input: `{"command":"echo hi"}`}},
				Done:      true,
			}
			return
		}
		ch <- contextengine.LLMChunk{Content: "done after tools", Done: true}
	}()
	return ch, nil
}

// Covers: L5-CTX-34
func TestPEVEngine_queryLoop_should_run_multi_turn_until_no_tools(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	cfg.PEV.VerifyMode = config.VerifyModeBasic
	cfg.Plan.Enabled = false
	engine := contextengine.NewPEVEngine(
		&twoRoundLLM{},
		&mockctx.ToolRunner{Output: "hi"},
		mustBuiltinRegistry(t),
		mockctx.AllowAllPermission{},
		contextengine.NoOpObserver{},
		&cfg.PEV,
		nil,
		contextengine.NewBuiltinVerifyRunner(t.TempDir()),
		contextengine.NoOpPEVObserver{},
		nil,
		cfg.Plan,
	)
	engine.SetQueryLoopSupport(contextengine.QueryLoopSupport{
		Enabled:  true,
		MaxTurns: 5,
	})

	sc := &types.SessionContext{
		SessionID: "sess_ql",
		WorkDir:   t.TempDir(),
		Model:     "test",
		PEVState:  types.DefaultPEVState(3),
	}
	var toolCalls int
	res, err := engine.Run(context.Background(), sc, nil, "run", func(ev *gateway.EngineEvent) {
		if ev.Type == "tool_call" {
			toolCalls++
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if toolCalls != 1 {
		t.Fatalf("expected 1 tool call, got %d", toolCalls)
	}
	if res == nil || len(res.Messages) == 0 || res.Messages[0].Content == "" {
		t.Fatal("expected final assistant message from query loop")
	}
}

type yoloPermission struct{}

func (yoloPermission) Request(context.Context, string, string, string, types.RiskLevel) bool {
	return true
}

func (yoloPermission) IsYOLOMode() bool { return true }

// Covers: L5-CTX-34
func TestPEVEngine_queryLoop_yolo_should_emit_complete_after_tool_round(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	cfg.PEV.VerifyMode = config.VerifyModeBasic
	cfg.Plan.Enabled = false
	engine := contextengine.NewPEVEngine(
		&twoRoundLLM{},
		&mockctx.ToolRunner{Output: "hi"},
		mustBuiltinRegistry(t),
		yoloPermission{},
		contextengine.NoOpObserver{},
		&cfg.PEV,
		nil,
		contextengine.NewBuiltinVerifyRunner(t.TempDir()),
		contextengine.NoOpPEVObserver{},
		nil,
		cfg.Plan,
	)
	engine.SetQueryLoopSupport(contextengine.QueryLoopSupport{
		Enabled:  true,
		MaxTurns: 5,
	})

	sc := &types.SessionContext{
		SessionID: "sess_ql_yolo",
		WorkDir:   t.TempDir(),
		Model:     "test",
		PEVState:  types.DefaultPEVState(3),
	}
	var gotComplete bool
	_, err := engine.Run(context.Background(), sc, nil, "run", func(ev *gateway.EngineEvent) {
		if ev.Type == "complete" {
			gotComplete = true
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !gotComplete {
		t.Fatal("expected complete event after YOLO query loop with tool calls")
	}
	if sc.PEVState.Phase != types.PEVPhaseDone {
		t.Fatalf("expected PEV phase done, got %q", sc.PEVState.Phase)
	}
}
