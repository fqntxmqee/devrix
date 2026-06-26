package sessionorchestrator

import (
	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

// v6.1.0 helper: wire OrchestratePath with a fakeWaveScheduler that returns a
// caller-supplied artifact list. Used by tests that previously relied on
// FastPath behavior (executor.RunTurn called directly). With v6.1.0 routing,
// IntentFast and IntentOrchestrate both flow through OrchestratePath, so
// tests that need a deterministic D2-like surface wire this in via
// WithOrchestratePath.
//
// exec is still passed to NewSessionOrchestrator so the constructor type
// checks succeed, but the v6.1.0 routing does not invoke exec.RunTurn
// directly — the OrchestratePath's scheduler emits events from artifacts.
//
// Additional OrchestratorOptions can be passed via opts (e.g. WithValidator,
// WithLearner, WithClassifier) and are applied AFTER WithOrchestratePath.
func newOrchestratorWithFakeOrchestratePath(
	cfg *orchtypes.Config,
	exec TurnExecutor,
	artifacts []wavescheduler.Artifact,
	opts ...OrchestratorOption,
) (*SessionOrchestrator, *fakeWaveScheduler) {
	decomp := decisionplanning.NewTaskDecomposer()
	sched := &fakeWaveScheduler{artifacts: artifacts}
	op := NewOrchestratePath(decomp, sched, nil)
	all := append([]OrchestratorOption{WithOrchestratePath(op)}, opts...)
	orch := NewSessionOrchestrator(cfg, exec, all...)
	return orch, sched
}