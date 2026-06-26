package bootstrap

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

type stubItemPipelineToolExec struct{}

func (stubItemPipelineToolExec) ExecuteRound(_ context.Context, _ sessionorchestrator.ToolRoundRequest) (sessionorchestrator.ToolRoundResult, error) {
	return sessionorchestrator.ToolRoundResult{}, nil
}

func TestWireItemPipeline(t *testing.T) {
	tm := workmodel.NewTaskManager()
	runner, learner, err := WireItemPipeline(ItemPipelineWireDeps{
		ToolExec: stubItemPipelineToolExec{},
		Tasks:    tm,
	})
	if err != nil {
		t.Fatalf("WireItemPipeline: %v", err)
	}
	if runner == nil || learner == nil {
		t.Fatal("expected runner and learner")
	}
}
