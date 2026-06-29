package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// maxItersWorkItemExecutor simulates a goal round that hits the iteration cap
// after making tool progress — the production failure mode for open-ended goals.
type maxItersWorkItemExecutor struct{}

func (maxItersWorkItemExecutor) ExecuteWorkItem(_ context.Context, _, _, _ string) (*WorkItemResult, error) {
	return &WorkItemResult{
		Content:    "partial exploration output",
		Done:       false,
		Iterations: 5,
		ToolCalls:  4,
		StopReason: "max_iters",
	}, nil
}

func TestItemPipeline_GoalPartialAtThresholdSpawnsDecompose(t *testing.T) {
	tm := workmodel.NewTaskManager()
	runner, err := NewItemPipelineRunner(ItemPipelineDeps{
		Tasks:    tm,
		Executor: maxItersWorkItemExecutor{},
	})
	if err != nil {
		t.Fatalf("NewItemPipelineRunner: %v", err)
	}
	goal, err := tm.EnsureGoal("s1", "investigate internal/layers/contextengine/kernel")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	if goal.Uncertainty != workmodel.DefaultUncertaintyDecomposeThreshold {
		t.Fatalf("seed uncertainty = %v, want %v", goal.Uncertainty, workmodel.DefaultUncertaintyDecomposeThreshold)
	}

	round, err := runner.Run(context.Background(), "s1", goal, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.PlanKind != plan.ExplorationPlan {
		t.Fatalf("PlanKind = %v, want ExplorationPlan", round.PlanKind)
	}
	if round.VerdictKind != types.VerdictPartial {
		t.Fatalf("VerdictKind = %v, want partial", round.VerdictKind)
	}
	if round.SpawnPolicy != workmodel.SpawnDecompose {
		t.Fatalf("SpawnPolicy = %q, want decompose (%s)", round.SpawnPolicy, round.SpawnRationale)
	}
}

func TestItemPipeline_GoalMaxItersSpawnsDecompose(t *testing.T) {
	tm := workmodel.NewTaskManager()
	runner, err := NewItemPipelineRunner(ItemPipelineDeps{
		Tasks:    tm,
		Executor: maxItersWorkItemExecutor{},
	})
	if err != nil {
		t.Fatalf("NewItemPipelineRunner: %v", err)
	}
	goal, err := tm.EnsureGoal("s1", "investigate internal/layers/contextengine/kernel")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}

	round, err := runner.Run(context.Background(), "s1", goal, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.SpawnPolicy != workmodel.SpawnDecompose {
		t.Fatalf("SpawnPolicy = %q, want decompose (got %q)", round.SpawnPolicy, round.SpawnRationale)
	}

	if err := workmodel.ApplySpawnPolicy("s1", goal, round, tm); err != nil {
		t.Fatalf("ApplySpawnPolicy: %v", err)
	}
	children := tm.Tree().ListChildren("s1", goal.ID)
	if len(children) < 2 {
		t.Fatalf("children = %d, want >= 2 after decompose", len(children))
	}
}
