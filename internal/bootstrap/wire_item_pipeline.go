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
// LLMInvoker wires the D7-S2-A07 streaming call used by ItemToolRunner.invokeWorkItemExecute
// so chat-style directives actually reach the LLM. Without this the
// per-WorkItem pipeline returns a synthetic "work item executed: <directive>"
// string and the user sees an instant empty reply (DM-20260626-008 follow-up
// regression after PR #243 + PR #246 made ItemPipelineRunner the default
// ingress).
type ItemPipelineWireDeps struct {
	ToolExec   sessionorchestrator.ToolRoundExecutor
	Tasks      *workmodel.TaskManager
	Classifier decisionplanning.IntentClassifier
	LLMInvoker orchtypes.LLMInvoker
	TrackMode  string
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
		return nil, nil, fmt.Errorf("wire item pipeline: LLMInvoker required (ItemToolRunner needs D7-S2-A07 to call the LLM)")
	}
	learner := WireDefaultMUPSLearner()
	runner, err := sessionorchestrator.NewItemPipelineRunner(sessionorchestrator.ItemPipelineDeps{
		Classifier: deps.Classifier,
		Runner:     sessionorchestrator.NewItemToolRunnerWithLLM(deps.ToolExec, deps.LLMInvoker),
		Learner:    learner,
		Tasks:      deps.Tasks,
		TrackMode:  deps.TrackMode,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("wire item pipeline: %w", err)
	}
	return runner, learner, nil
}
