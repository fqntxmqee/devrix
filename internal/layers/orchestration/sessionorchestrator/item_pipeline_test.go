package sessionorchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/execute"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubItemToolRunner struct{}

func (stubItemToolRunner) Invoke(_ context.Context, req execute.ToolRequest) (execute.ToolResult, error) {
	now := time.Now()
	return execute.ToolResult{
		ToolName:    req.ToolName,
		ExitCode:    0,
		Output:      "ok",
		StartedAt:   now,
		CompletedAt: now.Add(5 * time.Millisecond),
	}, nil
}

func newItemPipelineTestRunner(t *testing.T) (*ItemPipelineRunner, *workmodel.TaskManager, *learn.InMemoryReputationStore) {
	t.Helper()
	tm := workmodel.NewTaskManager()
	skill := learn.NewSkillMemory()
	feedback := learn.NewFeedbackMemory()
	scheduled := learn.NewScheduledMemory()
	rep := learn.NewInMemoryReputationStore()
	learner := learn.NewDefaultLearner(skill, feedback, scheduled, rep, learn.NewAssetBuilder())
	runner, err := NewItemPipelineRunner(ItemPipelineDeps{
		Runner:  stubItemToolRunner{},
		Learner: learner,
		Tasks:   tm,
	})
	if err != nil {
		t.Fatalf("NewItemPipelineRunner: %v", err)
	}
	return runner, tm, rep
}

func TestRunItemPipeline_SingleWorkItem_Completed(t *testing.T) {
	runner, tm, rep := newItemPipelineTestRunner(t)
	sessionID := "sess-item-pipeline"
	goal, err := tm.EnsureGoal(sessionID, "implement cache layer")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	_ = tm.Tree().SetUncertainty(sessionID, goal.ID, 0.2)
	goal, _ = tm.GetWorkItem(sessionID, goal.ID)

	round, err := runner.Run(context.Background(), sessionID, goal, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.SpawnPolicy != workmodel.SpawnNone {
		t.Fatalf("SpawnPolicy = %q, want none", round.SpawnPolicy)
	}
	if len(round.ObservationIDs) == 0 || round.PlanID == "" || round.VerdictID == "" {
		t.Fatalf("LP-5 incomplete round: %+v", round)
	}
	if round.VerdictKind != types.VerdictPass {
		t.Fatalf("VerdictKind = %v, want Pass", round.VerdictKind)
	}

	got, _ := tm.GetWorkItem(sessionID, goal.ID)
	if got.Status != workmodel.TaskStatusCompleted {
		t.Fatalf("status = %q, want completed", got.Status)
	}
	if got.LastRound == nil || got.LastRound.PlanID != round.PlanID {
		t.Fatal("LastRound not persisted on WorkItem")
	}

	ev, err := rep.Get(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("rep.Get: %v", err)
	}
	if ev == nil || ev.Alpha < 1 {
		t.Fatalf("reputation after learn = %+v, want Alpha>=1", ev)
	}
}

func TestRunItemPipeline_PartialHighUncertainty_SpawnDecompose(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	runner.Verify = func(_ *wavescheduler.Artifact) workmodel.Verdict {
		return workmodel.Verdict{
			Kind:       types.VerdictPartial,
			Reason:     "needs more exploration",
			SourceID:   "v_partial",
			Confidence: 0.5,
		}
	}

	sessionID := "sess-decompose"
	goal, _ := tm.EnsureGoal(sessionID, "compare three cache strategies")
	_ = tm.Tree().SetUncertainty(sessionID, goal.ID, 0.85)
	goal, _ = tm.GetWorkItem(sessionID, goal.ID)

	round, err := runner.Run(context.Background(), sessionID, goal, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.SpawnPolicy != workmodel.SpawnDecompose {
		t.Fatalf("SpawnPolicy = %q, want decompose (G1 partial+uncertainty)", round.SpawnPolicy)
	}
	if round.UncertaintyMean <= workmodel.DefaultUncertaintyDecomposeThreshold {
		t.Fatalf("UncertaintyMean = %.2f, expected > threshold", round.UncertaintyMean)
	}

	got, _ := tm.GetWorkItem(sessionID, goal.ID)
	if got.Status == workmodel.TaskStatusCompleted {
		t.Fatal("partial decompose path should not mark item completed yet")
	}
	if got.RoundPhase != workmodel.RoundPhaseAwaitChild {
		t.Fatalf("RoundPhase = %q, want await_child", got.RoundPhase)
	}
}

func TestRunItemPipeline_LP5_LineageFields(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	sessionID := "sess-lp5"
	goal, _ := tm.EnsureGoal(sessionID, "verify login flow")
	round, err := runner.Run(context.Background(), sessionID, goal, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(round.ObservationIDs) < 1 {
		t.Fatal("expected observation IDs")
	}
	if round.PlanID == "" || round.ArtifactID == "" || round.VerdictID == "" {
		t.Fatalf("LP-5 chain broken: %+v", round)
	}
	if round.ExitReason == "" {
		t.Fatal("ExitReason required")
	}
}
