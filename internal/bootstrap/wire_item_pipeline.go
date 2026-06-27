package bootstrap

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// ItemPipelineWireDeps holds production wiring for per-WorkItem MUPS (Phase D).
//
// DM-20260626-009: per-WorkItem execution uses WorkItemExecutor
// (see sessionorchestrator.workitem_executor.go). The Executor drives a
// per-WorkItem ReAct loop (LLM ↔ Tool) and needs LLMInvoker for the LLM
// side and ContextPreparer for workspace context assembly.
type ItemPipelineWireDeps struct {
	ToolExec      sessionorchestrator.ToolRoundExecutor
	Tasks         *workmodel.TaskManager
	Classifier    decisionplanning.IntentClassifier
	LLMInvoker    orchtypes.LLMInvoker
	CtxPreparer   sessionorchestrator.ContextPreparer
	TrackMode     string
}

// WireDefaultMUPSLearner constructs the in-process LP-1 learner used by both
// ProcessMessage (prior injection) and ItemPipelineRunner (Learn node).
func WireDefaultMUPSLearner() learn.Learner {
	skill := learn.NewSkillMemory()
	feedback := learn.NewFeedbackMemory()
	scheduled := learn.NewScheduledMemory()
	rep := learn.NewInMemoryReputationStore()
	return learn.NewDefaultLearner(skill, feedback, scheduled, rep, learn.NewAssetBuilder())
}

// WireItemPipeline builds ItemPipelineRunner + shared Learner for bootstrap.
func WireItemPipeline(deps ItemPipelineWireDeps) (*sessionorchestrator.ItemPipelineRunner, learn.Learner, error) {
	if deps.Tasks == nil {
		return nil, nil, fmt.Errorf("wire item pipeline: TaskManager required")
	}
	if deps.ToolExec == nil {
		return nil, nil, fmt.Errorf("wire item pipeline: ToolRoundExecutor required")
	}
	if deps.LLMInvoker == nil {
		return nil, nil, fmt.Errorf("wire item pipeline: LLMInvoker required (WorkItemExecutor needs D7-S2-A07 to call the LLM)")
	}
	if deps.CtxPreparer == nil {
		return nil, nil, fmt.Errorf("wire item pipeline: ContextPreparer required (WorkItemExecutor needs D2-S15 to assemble SystemPrompt + Tools)")
	}
	learner := WireDefaultMUPSLearner()
	executor := sessionorchestrator.NewWorkItemExecutor(deps.LLMInvoker, deps.CtxPreparer, deps.ToolExec)
	runner, err := sessionorchestrator.NewItemPipelineRunner(sessionorchestrator.ItemPipelineDeps{
		Classifier: deps.Classifier,
		Executor:   executor,
		Learner:    learner,
		Tasks:      deps.Tasks,
		TrackMode:  deps.TrackMode,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("wire item pipeline: %w", err)
	}
	return runner, learner, nil
}