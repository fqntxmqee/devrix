package bootstrap

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// ItemPipelineWireDeps holds production wiring for per-WorkItem MUPS (Phase D).
type ItemPipelineWireDeps struct {
	ToolExec   sessionorchestrator.ToolRoundExecutor
	Tasks      *workmodel.TaskManager
	Classifier decisionplanning.IntentClassifier
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
	learner := WireDefaultMUPSLearner()
	runner, err := sessionorchestrator.NewItemPipelineRunner(sessionorchestrator.ItemPipelineDeps{
		Classifier: deps.Classifier,
		Runner:     sessionorchestrator.NewItemToolRunner(deps.ToolExec),
		Learner:    learner,
		Tasks:      deps.Tasks,
		TrackMode:  deps.TrackMode,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("wire item pipeline: %w", err)
	}
	return runner, learner, nil
}
