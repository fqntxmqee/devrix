package bootstrap

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

type stubItemPipelineToolExec struct{}

func (stubItemPipelineToolExec) ExecuteRound(_ context.Context, _ sessionorchestrator.ToolRoundRequest) (sessionorchestrator.ToolRoundResult, error) {
	return sessionorchestrator.ToolRoundResult{}, nil
}

type stubItemPipelineLLM struct{}

func (stubItemPipelineLLM) InvokeStream(_ context.Context, _ orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	ch := make(chan llmgateway.Chunk)
	close(ch)
	return ch, nil
}

func TestWireItemPipeline(t *testing.T) {
	tm := workmodel.NewTaskManager()
	runner, learner, err := WireItemPipeline(ItemPipelineWireDeps{
		ToolExec:   stubItemPipelineToolExec{},
		Tasks:      tm,
		LLMInvoker: stubItemPipelineLLM{},
	})
	if err != nil {
		t.Fatalf("WireItemPipeline: %v", err)
	}
	if runner == nil || learner == nil {
		t.Fatal("expected runner and learner")
	}
}

func TestWireItemPipeline_RequiresLLMInvoker(t *testing.T) {
	// Regression: without LLMInvoker the synthetic work_item_execute path
	// would silently short-circuit (DM-20260626-008 follow-up). WireItemPipeline
	// must fail-fast so bootstrap can't ship without the LLM wiring.
	tm := workmodel.NewTaskManager()
	_, _, err := WireItemPipeline(ItemPipelineWireDeps{
		ToolExec: stubItemPipelineToolExec{},
		Tasks:    tm,
	})
	if err == nil {
		t.Fatal("expected error when LLMInvoker is nil")
	}
}
