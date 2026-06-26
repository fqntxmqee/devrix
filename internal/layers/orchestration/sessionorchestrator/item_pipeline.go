package sessionorchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// ItemPipelineRunner executes Observe→Plan→Execute→Verify→Learn→Decide for one WorkItem.
//
// DM-20260626-009: the Execute phase now goes through WorkItemExecutor
// directly (per-WorkItem ReAct loop), bypassing the legacy CommitChannel +
// ItemToolRunner + work_item_execute shim. Planner is kept for round
// metadata + Learn lineage; the Plan.Steps are vestigial (Executor reads
// the directive from WorkItem directly, not from Plan.Steps[0].ToolArgs).
type ItemPipelineRunner struct {
	Classifier      decisionplanning.IntentClassifier
	Planner         plan.Planner
	Learner         learn.Learner
	Tasks           *workmodel.TaskManager
	TrackMode       string
	ContextProposer workmodel.ContextProposer
	// Executor runs the per-WorkItem ReAct loop (DM-20260626-009).
	// Replaces the prior Router/Channel/tool-pipeline path.
	Executor WorkItemExecutor
	// Verify overrides deterministic artifact verification (tests / future LLM verifier).
	Verify func(*wavescheduler.Artifact) workmodel.Verdict
}

// ItemPipelineDeps wires a production-style runner. Nil Planner defaults to
// DefaultPlanner; nil Classifier defaults to RuleClassifier. Executor is
// required (DM-20260626-009).
type ItemPipelineDeps struct {
	Classifier decisionplanning.IntentClassifier
	Planner    plan.Planner
	Learner    learn.Learner
	Tasks      *workmodel.TaskManager
	TrackMode  string
	// Executor runs the per-WorkItem ReAct loop (DM-20260626-009).
	// Required. nil → NewItemPipelineRunner returns an error.
	Executor WorkItemExecutor
}

// NewItemPipelineRunner constructs an ItemPipelineRunner with the given deps.
// Executor is required; the Planner/Classifier fall back to defaults.
func NewItemPipelineRunner(deps ItemPipelineDeps) (*ItemPipelineRunner, error) {
	if deps.Tasks == nil {
		return nil, fmt.Errorf("item_pipeline: TaskManager required")
	}
	if deps.Executor == nil {
		return nil, fmt.Errorf("item_pipeline: WorkItemExecutor required (DM-20260626-009)")
	}
	planner := deps.Planner
	if planner == nil {
		planner = plan.NewDefaultPlanner()
	}
	classifier := deps.Classifier
	if classifier == nil {
		classifier = decisionplanning.NewRuleClassifier(nil)
	}
	return &ItemPipelineRunner{
		Classifier: classifier,
		Planner:    planner,
		Learner:    deps.Learner,
		Tasks:      deps.Tasks,
		TrackMode:  deps.TrackMode,
		Executor:   deps.Executor,
	}, nil
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
			ID:        "step_" + item.ID,
			Directive: itemDirective(item),
			// ToolName/ToolArgs are vestigial post-DM-20260626-009: the
			// Execute phase calls WorkItemExecutor directly with the
			// directive, not through CommitChannel+work_item_execute.
			// Kept so the Plan still validates (DefaultPlanner requires
			// ≥1 Step) and so any Plan inspector sees the directive.
			ToolName:        "workitem_executor_direct",
			ToolArgs:        map[string]any{"directive": itemDirective(item)},
			IdempotencyKey:  "idem_" + item.ID,
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

	// DM-20260626-009: bypass CommitChannel/ItemToolRunner/work_item_execute;
	// call WorkItemExecutor directly with the WorkItem's directive.
	result, execErr := r.Executor.ExecuteWorkItem(ctx, sessionID, item.ID, itemDirective(item))
	if execErr != nil {
		// Non-fatal: continue with whatever Content was accumulated so the
		// round still produces an Artifact + Verdict for downstream Verify.
		// Empty content is fine — Verify will mark the round as failed and
		// the parent pipeline decides whether to retry / spawn.
		if result == nil {
			return nil, fmt.Errorf("item_pipeline: execute: %w", execErr)
		}
	}
	art := buildArtifactFromWorkItemResult(pl, item, sessionID, started, result, execErr)

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
	artifactSummary := ""
	if art != nil {
		artifactID = art.TaskID
		artifactSummary = art.Summary
	}
	round := &workmodel.WorkItemPipelineRound{
		RoundNo:           roundNo,
		WorkItemID:        item.ID,
		SessionID:         sessionID,
		ObservationIDs:    obsIDs,
		PlanID:            pl.ID,
		PlanKind:          pl.Kind,
		ArtifactID:        artifactID,
		ArtifactSummary:   artifactSummary,
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

	ctxOut := workmodel.ProposeContextPipelineOutput(sessionID, item, round, r.Tasks, r.ContextProposer)
	workmodel.ApplyPipelineDecide(sessionID, item, round, ctxOut, treeCtx, r.Tasks)
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

// buildArtifactFromWorkItemResult converts a WorkItemExecutor result into a
// wavescheduler.Artifact. Sets SourcePlanID to pl.ID so downstream consumers
// can correlate Artifact → Plan. WorkerType=WorkerWorkItem distinguishes
// ReAct-origin artifacts from wave-spawned runner artifacts.
func buildArtifactFromWorkItemResult(pl *plan.Plan, item *workmodel.WorkItem, sessionID string, started time.Time, result *WorkItemResult, execErr error) *wavescheduler.Artifact {
	ended := time.Now()
	content := ""
	stopReason := ""
	iterations := 0
	toolCalls := 0
	exit := 0
	errMsg := ""
	if result != nil {
		content = result.Content
		stopReason = result.StopReason
		iterations = result.Iterations
		toolCalls = result.ToolCalls
		if !result.Done {
			exit = 1
		}
		if !result.EndedAt.IsZero() {
			ended = result.EndedAt
		}
	}
	if execErr != nil {
		exit = 1
		errMsg = execErr.Error()
	}
	art := &wavescheduler.Artifact{
		TaskID:        item.ID,
		SessionID:     sessionID,
		WorkerType:    wavescheduler.WorkerWorkItem,
		Summary:       truncateForArtifact(content, 200),
		ExitCode:      exit,
		Error:         errMsg,
		StartedAt:     started,
		EndedAt:       ended,
		Duration:      ended.Sub(started),
		Metadata: map[string]any{
			"source":     WorkItemSourceLabel,
			"stop_reason": stopReason,
			"iterations": iterations,
			"tool_calls": toolCalls,
		},
	}
	if pl != nil {
		art.SourcePlanID = pl.ID
	}
	return art
}

// truncateForArtifact returns the first n runes of s followed by an
// ellipsis when truncation occurred. Empty input returns empty.
func truncateForArtifact(s string, n int) string {
	if s == "" || n <= 0 {
		return s
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i] + "…"
		}
		count++
	}
	return s
}

type observationRef string

func (o observationRef) GetID() string { return string(o) }