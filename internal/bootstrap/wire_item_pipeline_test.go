package bootstrap

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
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

// stubItemPipelineCtxPreparer implements sessionorchestrator.ContextPreparer
// with no SystemPrompt + no Tools, matching the bare call path used by
// legacy tests. Real bootstrap uses newContextEngineAdapter.
type stubItemPipelineCtxPreparer struct{}

func (stubItemPipelineCtxPreparer) Prepare(_ context.Context, _ sessionorchestrator.PrepareRequest) (sessionorchestrator.PreparedContext, error) {
	return sessionorchestrator.PreparedContext{}, nil
}

func TestWireItemPipeline(t *testing.T) {
	tm := workmodel.NewTaskManager()
	runner, learner, err := WireItemPipeline(ItemPipelineWireDeps{
		ToolExec:    stubItemPipelineToolExec{},
		Tasks:       tm,
		LLMInvoker:  stubItemPipelineLLM{},
		CtxPreparer: stubItemPipelineCtxPreparer{},
	})
	if err != nil {
		t.Fatalf("WireItemPipeline: %v", err)
	}
	if runner == nil || learner == nil {
		t.Fatal("expected runner and learner")
	}
	if runner.ObservationProposer == nil {
		t.Fatal("DM-20260630-001: expected LLMObservationProposer wired in production")
	}
	if _, ok := runner.ObservationProposer.(*sessionorchestrator.LLMObservationProposer); !ok {
		t.Fatalf("ObservationProposer type = %T", runner.ObservationProposer)
	}
	if runner.StrategicPlanProposer == nil {
		t.Fatal("DM-20260630-012: expected LLMStrategicPlanProposer wired in production")
	}
	if _, ok := runner.StrategicPlanProposer.(*sessionorchestrator.LLMStrategicPlanProposer); !ok {
		t.Fatalf("StrategicPlanProposer type = %T", runner.StrategicPlanProposer)
	}
}

func TestWireItemPipeline_RequiresLLMInvoker(t *testing.T) {
	// DM-20260626-009: WorkItemExecutor needs the LLMInvoker to call
	// the LLM (D7-S2-A07). WireItemPipeline must fail-fast so bootstrap
	// can't ship without the LLM wiring.
	tm := workmodel.NewTaskManager()
	_, _, err := WireItemPipeline(ItemPipelineWireDeps{
		ToolExec:    stubItemPipelineToolExec{},
		Tasks:       tm,
		CtxPreparer: stubItemPipelineCtxPreparer{},
	})
	if err == nil {
		t.Fatal("expected error when LLMInvoker is nil")
	}
}

func TestWireItemPipeline_RequiresCtxPreparer(t *testing.T) {
	// DM-20260626-009: WorkItemExecutor needs ContextPreparer to
	// assemble SystemPrompt + Tools before each LLM call (D2-S15).
	// WireItemPipeline must fail-fast so bootstrap can't ship without
	// the context engine adapter.
	tm := workmodel.NewTaskManager()
	_, _, err := WireItemPipeline(ItemPipelineWireDeps{
		ToolExec:   stubItemPipelineToolExec{},
		Tasks:      tm,
		LLMInvoker: stubItemPipelineLLM{},
	})
	if err == nil {
		t.Fatal("expected error when CtxPreparer is nil")
	}
}

// Reference types to satisfy imports and keep the stub surface honest.
var _ = types.MessageRoleUser