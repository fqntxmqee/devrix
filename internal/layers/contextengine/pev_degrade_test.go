package contextengine_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

type failOnNthLLM struct {
	n     int
	calls int
	err   error
}

func (f *failOnNthLLM) ChatStream(_ context.Context, _ *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	f.calls++
	if f.calls == f.n {
		return nil, f.err
	}
	ch := make(chan contextengine.LLMChunk, 1)
	if f.calls > f.n {
		ch <- contextengine.LLMChunk{Content: "degraded summary", Done: true}
	} else {
		ch <- contextengine.LLMChunk{
			ToolCalls: []contextengine.ToolCall{{ID: "call_test_1", Name: "bash", Input: "{}"}},
			Done:      true,
		}
	}
	close(ch)
	return ch, nil
}

// Covers: L5-CTX-29
func TestPEVEngine_should_degrade_when_followup_llm_fails_after_tools(t *testing.T) {
	llmErr := sharederrors.NewProviderUnavailableError(
		fmt.Errorf("provider minimax status 400: tool_call_id mismatch"),
	)
	llm := &failOnNthLLM{n: 2, err: llmErr}

	cfg := config.DefaultContextEngineConfig()
	cfg.PEV.MaxIterations = 3
	cfg.PEV.VerifyMode = config.VerifyModeCommands
	cfg.Plan.Enabled = false

	engine := contextengine.NewPEVEngine(
		llm,
		failingToolRunner{},
		registry.NewBuiltinRegistry(),
		mockctx.AllowAllPermission{},
		contextengine.NoOpObserver{},
		&cfg.PEV,
		nil,
		contextengine.NewBuiltinVerifyRunner(t.TempDir()),
		contextengine.NoOpPEVObserver{},
		nil,
		cfg.Plan,
	)

	sc := &types.SessionContext{
		SessionID: "sess_degrade",
		Model:     "test",
		WorkDir:   t.TempDir(),
		PEVState:  types.DefaultPEVState(3),
	}

	var events []gateway.EngineEvent
	result, err := engine.Run(context.Background(), sc, nil, "run tools", func(ev *gateway.EngineEvent) {
		events = append(events, *ev)
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want degradation without error", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if llm.calls < 2 {
		t.Fatalf("expected at least 2 LLM calls, got %d", llm.calls)
	}
}
