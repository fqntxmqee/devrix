package sessionorchestrator

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// newOrchestratorWithItemPipeline wires RunSessionTurnLoop ingress with a
// production-style ItemPipelineRunner backed by the shared test executor stub.
func newOrchestratorWithItemPipeline(t *testing.T, opts ...OrchestratorOption) (*SessionOrchestrator, *ItemPipelineRunner, *workmodel.TaskManager) {
	t.Helper()
	runner, tm, _ := newItemPipelineTestRunner(t)
	all := append([]OrchestratorOption{
		WithTaskManager(tm),
		WithItemPipelineRunner(runner),
	}, opts...)
	return NewSessionOrchestrator(orchtypes.DefaultConfig(), nil, all...), runner, tm
}

func newTestOrch(t *testing.T, opts ...OrchestratorOption) *SessionOrchestrator {
	t.Helper()
	orch, _, _ := newOrchestratorWithItemPipeline(t, opts...)
	return orch
}
