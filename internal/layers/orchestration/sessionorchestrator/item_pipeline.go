package sessionorchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/execute"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// ItemPipelineRunner executes Observe→Plan→Execute→Verify→Learn→Decide for one WorkItem.
type ItemPipelineRunner struct {
	Classifier decisionplanning.IntentClassifier
	Planner    plan.Planner
	Router     *execute.ChannelRouter
	Learner    learn.Learner
	Tasks      *workmodel.TaskManager
	TrackMode  string
	// Verify overrides deterministic artifact verification (tests / future LLM verifier).
	Verify func(*wavescheduler.Artifact) workmodel.Verdict
}

// ItemPipelineDeps wires a production-style runner. Nil Planner defaults to DefaultPlanner.
func NewItemPipelineRunner(deps ItemPipelineDeps) (*ItemPipelineRunner, error) {
	if deps.Tasks == nil {
		return nil, fmt.Errorf("item_pipeline: TaskManager required")
	}
	if deps.Runner == nil {
		return nil, fmt.Errorf("item_pipeline: ToolRunner required")
	}
	planner := deps.Planner
	if planner == nil {
		planner = plan.NewDefaultPlanner()
	}
	classifier := deps.Classifier
	if classifier == nil {
		classifier = decisionplanning.NewRuleClassifier(nil)
	}
	reg := execute.NewChannelRegistry()
	for _, regFn := range []struct {
		name string
		fn   func(execute.ToolRunner) (execute.Channel, error)
	}{
		{"commit", func(r execute.ToolRunner) (execute.Channel, error) {
			return execute.NewCommitChannel(r, execute.CommitChannelConfig{})
		}},
		{"protocol", func(r execute.ToolRunner) (execute.Channel, error) {
			return execute.NewProtocolChannel(r, execute.ProtocolChannelConfig{})
		}},
		{"scenario", func(r execute.ToolRunner) (execute.Channel, error) {
			return execute.NewScenarioChannel(r, execute.ScenarioChannelConfig{})
		}},
		{"exploration", func(r execute.ToolRunner) (execute.Channel, error) {
			return execute.NewExplorationChannel(r, execute.ExplorationChannelConfig{})
		}},
	} {
		ch, err := regFn.fn(deps.Runner)
		if err != nil {
			return nil, fmt.Errorf("item_pipeline: %s channel: %w", regFn.name, err)
		}
		if err := reg.Register(ch); err != nil {
			return nil, fmt.Errorf("item_pipeline: register %s: %w", regFn.name, err)
		}
	}
	return &ItemPipelineRunner{
		Classifier: classifier,
		Planner:    planner,
		Router:     execute.NewChannelRouter(reg),
		Learner:    deps.Learner,
		Tasks:      deps.Tasks,
		TrackMode:  deps.TrackMode,
	}, nil
}

// ItemPipelineDeps holds dependencies for NewItemPipelineRunner.
type ItemPipelineDeps struct {
	Classifier decisionplanning.IntentClassifier
	Planner    plan.Planner
	Runner     execute.ToolRunner
	Learner    learn.Learner
	Tasks      *workmodel.TaskManager
	TrackMode  string
}

// Run executes the full per-WorkItem MUPS pipeline and persists LastRound (Phase B).
func (r *ItemPipelineRunner) Run(ctx context.Context, sessionID string, item *workmodel.WorkItem, userID string) (*workmodel.WorkItemPipelineRound, error) {
	if r == nil {
		return nil, fmt.Errorf("item_pipeline: runner nil")
	}
	if item == nil || item.ID == "" {
		return nil, fmt.Errorf("item_pipeline: work item required")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("item_pipeline: sessionID required")
	}
	if workmodel.IsHumanReviewItem(item) && item.Status == workmodel.TaskStatusPending {
		return r.runHumanReviewAwait(ctx, sessionID, item)
	}

	started := time.Now()
	roundNo := 1
	if item.LastRound != nil {
		roundNo = item.LastRound.RoundNo + 1
	}

	_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhaseObserve)

	report, obsIDs, err := observeWorkItem(ctx, sessionID, item, r.Classifier, r.Learner, r.TrackMode, r.Tasks)
	if err != nil {
		return nil, err
	}
	_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhasePlan)

	qKind := "intent_orchestrate"
	if report.QuantizedIntent != nil {
		qKind = quantizedKindFromIntent(report.QuantizedIntent.Kind)
	}
	planInput := plan.PlanInput{
		SessionID:      sessionID,
		ObservationIDs: obsIDs,
		QuantizedKind:  qKind,
		AnomaliesCount: len(report.Anomalies),
		Steps: []plan.Step{{
			ID:             "step_" + item.ID,
			Directive:      itemDirective(item),
			ToolName:       "work_item_execute",
			IdempotencyKey: "idem_" + item.ID,
			EstimatedTokens: 100,
		}},
		FailureCriteria: []plan.FailureCriterion{{Field: "exit_code", Op: "eq", Value: 0}},
		BlastRadius: plan.BlastRadius{
			FileCount:    1,
			APICallCount: 1,
			TokenCost:    100,
			PersistScope: plan.PersistSession,
		},
	}
	pl, err := r.Planner.Plan(planInput)
	if err != nil {
		return nil, fmt.Errorf("item_pipeline: plan: %w", err)
	}
	_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhaseExecute)

	art, err := r.Router.Route(ctx, pl, execute.ChannelRequest{
		SessionID: sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("item_pipeline: execute: %w", err)
	}
	if art != nil && art.SourcePlanID == "" {
		art.SourcePlanID = pl.ID
	}
	_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhaseVerify)

	verdict := verifyArtifact(art)
	if r.Verify != nil {
		verdict = r.Verify(art)
	}
	exitReason := exitReasonForVerdict(verdict, sessionID)
	_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhaseLearn)

	var learningClass types.LearningClass
	if r.Learner != nil {
		obsLookups := make([]learn.ObservationLookup, 0, len(obsIDs))
		for _, id := range obsIDs {
			obsLookups = append(obsLookups, observationRef(id))
		}
		assets, err := r.Learner.Learn(ctx, learn.LearnRequest{
			SessionID:    sessionID,
			Verdict:      verdict,
			Plan:         pl,
			Artifact:     art,
			Observations: obsLookups,
		})
		if err != nil {
			return nil, fmt.Errorf("item_pipeline: learn: %w", err)
		}
		if len(assets) > 0 && assets[0] != nil {
			learningClass = assets[0].Class
		}
	}

	wilsonLower := 0.0
	if r.Learner != nil {
		if rep, err := r.Learner.Inject(ctx, sessionID, r.TrackMode); err == nil && rep != nil {
			wilsonLower = rep.PriorBeta.Mean()
		}
	}
	stats := workmodel.ChildOutcomeStats{}
	if r.Tasks != nil {
		for _, c := range r.Tasks.Tree().ListChildren(sessionID, item.ID) {
			if c == nil || c.Kind == workmodel.WorkKindChecklist {
				continue
			}
			stats.Total++
			switch c.Status {
			case workmodel.TaskStatusCompleted:
				stats.Completed++
			case workmodel.TaskStatusFailed, workmodel.TaskStatusCancelled:
				stats.Failed++
			case workmodel.TaskStatusInProgress, workmodel.TaskStatusPending:
				stats.Running++
			}
		}
	}
	uncertaintyMean := workmodel.ComputeUnifiedUncertainty(workmodel.UnifiedUncertaintyInput{
		WilsonLower:       wilsonLower,
		ChildStats:        stats,
		VerdictConfidence: verdict.Confidence,
		EvidenceCount:     len(obsIDs),
	})
	if item.Uncertainty > uncertaintyMean {
		uncertaintyMean = item.Uncertainty
	}

	artifactID := ""
	if art != nil {
		artifactID = art.TaskID
	}
	round := &workmodel.WorkItemPipelineRound{
		RoundNo:           roundNo,
		WorkItemID:        item.ID,
		SessionID:         sessionID,
		ObservationIDs:    obsIDs,
		PlanID:            pl.ID,
		PlanKind:          pl.Kind,
		ArtifactID:        artifactID,
		VerdictID:         verdict.SourceID,
		VerdictKind:       verdict.Kind,
		ExitReason:        exitReason,
		LearningClass:     learningClass,
		UncertaintyMean:   uncertaintyMean,
		VerdictConfidence: verdict.Confidence,
		StartedAt:         started,
		CompletedAt:       time.Now(),
	}

	treeCtx := workmodel.DefaultTreeEvalContext(sessionID, item.ID, userID, r.Tasks)
	if item.LastRound != nil && item.LastRound.VerdictKind == types.VerdictIndeterminate {
		treeCtx.IndeterminateRetries = item.LastRound.IndeterminateRetries
	}
	treeCtx.DailyLimitExceeded = workmodel.DecomposeDailyLimitWouldExceed(sessionID, item.Kind, 1)

	parent, _ := r.Tasks.GetWorkItem(sessionID, item.ParentID)
	bubbleCtx := workmodel.DefaultContextBubbleEvalContext(item, parent, round, r.Tasks, sessionID)
	workmodel.ApplyContextBubbleDecision(round, nil, bubbleCtx)

	workmodel.EvaluateSpawnPolicy(round, treeCtx)
	if round.VerdictKind == types.VerdictIndeterminate {
		round.IndeterminateRetries = treeCtx.IndeterminateRetries + 1
	}
	_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhaseDecide)

	phase := workmodel.RoundPhaseIdle
	switch round.SpawnPolicy {
	case workmodel.SpawnAwait, workmodel.SpawnDecompose, workmodel.SpawnParallelExplore:
		phase = workmodel.RoundPhaseAwaitChild
	case workmodel.SpawnInline:
		phase = workmodel.RoundPhaseIdle
	default:
		phase = workmodel.RoundPhaseIdle
	}
	if err := r.Tasks.Tree().ApplyPipelineRound(sessionID, item.ID, round, phase); err != nil {
		return nil, err
	}

	if round.SpawnPolicy == workmodel.SpawnNone {
		status := workmodel.StatusAfterSpawnNone(verdict.Kind)
		got, _ := r.Tasks.GetWorkItem(sessionID, item.ID)
		if got != nil && got.Status == workmodel.TaskStatusPending {
			_ = r.Tasks.Tree().UpdateStatus(sessionID, item.ID, workmodel.TaskStatusInProgress)
		}
		if status != workmodel.TaskStatusInProgress {
			_ = r.Tasks.Tree().UpdateStatus(sessionID, item.ID, status)
		}
	} else if item.Status == workmodel.TaskStatusPending {
		_ = r.Tasks.Tree().UpdateStatus(sessionID, item.ID, workmodel.TaskStatusInProgress)
	}

	item.LastRound = round
	item.RoundPhase = phase
	item.Uncertainty = uncertaintyMean
	return round, nil
}

type observationRef string

func (o observationRef) GetID() string { return string(o) }
