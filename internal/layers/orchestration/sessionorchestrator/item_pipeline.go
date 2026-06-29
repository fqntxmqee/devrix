package sessionorchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// ItemPipelineRunner executes Observe→Plan→Execute→Verify→Learn→Decide for one WorkItem.
//
// DM-20260626-009: the Execute phase goes through WorkItemExecutor
// directly (per-WorkItem ReAct loop). Planner is kept for round
// metadata + Learn lineage; the Plan.Steps are vestigial (Executor reads
// the directive from WorkItem directly, not from Plan.Steps[0].ToolArgs).
type ItemPipelineRunner struct {
	Classifier      decisionplanning.IntentClassifier
	Planner         plan.Planner
	Learner         learn.Learner
	Tasks           *workmodel.TaskManager
	TrackMode       string
	ContextProposer      workmodel.ContextProposer
	ObservationProposer  ObservationProposer
	// Executor runs the per-WorkItem ReAct loop (DM-20260626-009).
	// Replaces the prior Router/Channel/tool-pipeline path.
	Executor WorkItemExecutor
	// Verify overrides deterministic artifact verification (tests / future LLM verifier).
	Verify func(*wavescheduler.Artifact) workmodel.Verdict
	// Emit forwards intermediate engine events from the ReAct loop to the
	// gateway so feishu cards show live tool_call / tool_result / text
	// alongside the final ArtifactSummary. nil → no-op (legacy / test
	// fixtures). RunSessionTurnLoop sets this in its goroutine so the
	// per-WorkItem path produces the same observable stream on feishu cards.
	//
	// Hotfix (2026-06-27): without this, ItemPipelineRunner ran 4 tool.bash
	// calls per ReAct loop but the feishu card only saw the final
	// ArtifactSummary text + complete — tool invocations were invisible.
	// See devrix-inner-spans-dedup-remove memory note.
	Emit func(*contracts.EngineEvent)
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
	// ObservationProposer optional LLM proposer @ Observe (T35); nil → rules only.
	ObservationProposer ObservationProposer
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
		Classifier:          classifier,
		Planner:             planner,
		Learner:             deps.Learner,
		Tasks:               deps.Tasks,
		TrackMode:           deps.TrackMode,
		Executor:            deps.Executor,
		ObservationProposer: deps.ObservationProposer,
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
	// Propagate Emit hook to Executor so the ReAct loop's intermediate
	// events (text / thinking / tool_call / tool_result) flow to the
	// gateway. nil-safe — legacy / test runners without Emit continue to
	// work unchanged.
	//
	// Hotfix 2026-06-28 (DM-20260628-002): overwrite (not "set if nil") so
	// each Run() invocation picks up the freshest r.Emit. Production
	// RunSessionTurnLoop assigns a new emitFn per turn that captures the
	// current turn's `out` channel — without this overwrite, a multi-turn
	// session's later Run() inherits the earlier turn's executor.Emit, and
	// once that turn's `out` is closed via defer close(out), the LLM
	// stream's chunk emit panics with "send on closed channel". Tests that
	// inject an Emit stub continue to work because they leave r.Emit nil
	// and instead set exec.Emit directly on the executor.
	if r.Emit != nil {
		if exec, ok := r.Executor.(*DefaultWorkItemExecutor); ok {
			exec.Emit = r.Emit
		}
	}

	// DM-20260626-009 hotfix: emit the v6.0.0 5-node MUPS root span so the
	// per-WorkItem ItemPipelineRunner path is observable in Jaeger. hardening
	// uses a package-level bridge (SetBridge in bootstrap/wire_coordinator.go),
	// so this works without an obsBridge field on ItemPipelineRunner.
	ctx, endMUPS := hardening.EmitMUPSPipeline(ctx, sessionID, item.ID, "item_pipeline")
	defer func() { endMUPS(nil) }()

	if got, ok := r.Tasks.GetWorkItem(sessionID, item.ID); ok && got != nil {
		item = got
	}
	isRollup := item.NeedsRollup
	directive := DirectiveForItem(sessionID, item, r.Tasks)
	if item.Kind == workmodel.WorkKindGoal {
		directive = DirectiveForGoalPlan(item, directive)
	}

	started := time.Now()
	roundNo := 1
	if item.LastRound != nil {
		roundNo = item.LastRound.RoundNo + 1
	}

	{
		end := hardening.EmitWorktreeOp(ctx, sessionID, "set_round_phase", item.ID, string(workmodel.RoundPhaseObserve))
		_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhaseObserve)
		end(nil)
	}

	report, obsIDs, err := observeWorkItem(ctx, sessionID, item, r.Classifier, r.Learner, r.TrackMode, r.Tasks, r.ObservationProposer)
	if err != nil {
		return nil, err
	}
	{
		end := hardening.EmitWorktreeOp(ctx, sessionID, "set_round_phase", item.ID, string(workmodel.RoundPhasePlan))
		_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhasePlan)
		end(nil)
	}

	qKind := planQuantizedKind(item, report)
	planInput := plan.PlanInput{
		SessionID:      sessionID,
		ObservationIDs: obsIDs,
		QuantizedKind:  qKind,
		AnomaliesCount: len(report.Anomalies),
		Steps: []plan.Step{{
			ID:        "step_" + item.ID,
			Directive: directive,
			// ToolName/ToolArgs are vestigial: Execute calls WorkItemExecutor
			// directly with the directive. Kept so the Plan validates and
			// Plan inspectors see the directive.
			ToolName:        "workitem_executor_direct",
			ToolArgs:        map[string]any{"directive": directive},
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
	if isRollup {
		planInput.FailureCriteria = rollupFailureCriteria()
	}
	_, endPlan := hardening.EmitTaskGraphSynthesize(ctx, sessionID, len(planInput.Steps), 0, 1, false)
	pl, err := r.Planner.Plan(planInput)
	endPlan(err)
	if err != nil {
		return nil, fmt.Errorf("item_pipeline: plan: %w", err)
	}
	if isRollup && pl != nil {
		pl.Kind = plan.CommitmentPlan
	}
	{
		end := hardening.EmitWorktreeOp(ctx, sessionID, "set_round_phase", item.ID, string(workmodel.RoundPhaseExecute))
		_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhaseExecute)
		end(nil)
	}

	// Execute via WorkItemExecutor (per-WorkItem ReAct loop). Emit Wave +
	// Execute sub-spans so Jaeger shows the full 5-node tree.
	_, endWave := hardening.EmitExecutorSelect(ctx, sessionID, 1, "workitem", "0", "item_pipeline")
	endWave(nil)
	_, endExecute := hardening.EmitChannelRoute(ctx, sessionID, "item", "workitem", "0", "")
	execCtx := WithWorkItemExecContext(ctx, WorkItemExecContext{Item: item, Tasks: r.Tasks})
	result, execErr := r.Executor.ExecuteWorkItem(execCtx, sessionID, item.ID, directive)
	endExecute(execErr)
	if execErr != nil {
		// Non-fatal: continue with whatever Content was accumulated so the
		// round still produces an Artifact + Verdict for downstream Verify.
		// Empty content is fine — Verify will mark the round as failed and
		// the parent pipeline decides whether to retry / spawn.
		if result == nil {
			return nil, fmt.Errorf("item_pipeline: execute: %w", execErr)
		}
	}
	if result != nil {
		ApplyGoalScopeFromExecute(sessionID, item, result.Content, r.Tasks)
	}
	if got, ok := r.Tasks.GetWorkItem(sessionID, item.ID); ok && got != nil {
		item = got
	}
	art := buildArtifactFromWorkItemResult(pl, item, sessionID, started, result, execErr)

	{
		end := hardening.EmitWorktreeOp(ctx, sessionID, "set_round_phase", item.ID, string(workmodel.RoundPhaseVerify))
		_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhaseVerify)
		end(nil)
	}

	verdict := verifyArtifactForWorkItem(art, item, pl)
	if isRollup {
		verdict = verifyRollupArtifact(art)
	} else if r.Verify != nil {
		verdict = r.Verify(art)
	}
	exitReason := exitReasonForVerdict(verdict, sessionID)
	{
		end := hardening.EmitWorktreeOp(ctx, sessionID, "set_round_phase", item.ID, string(workmodel.RoundPhaseLearn))
		_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhaseLearn)
		end(nil)
	}

	// DM-20260626-009 hotfix: emit the 5th-node sub-span (system.anomaly_detect
	// as a verify stand-in: the legacy OrchestratePath uses real anomaly
	// detection; the per-WorkItem path uses deterministic verifyArtifact and
	// shares the same op name so dashboards see a consistent 5-node tree).
	_, endVerify := hardening.EmitSystemAnomalyDetect(ctx, sessionID, "n/a", "n/a", "0", verdict.SourceID)
	endVerify(nil)

	var learningClass types.LearningClass
	if r.Learner != nil {
		obsLookups := make([]learn.ObservationLookup, 0, len(obsIDs))
		for _, id := range obsIDs {
			obsLookups = append(obsLookups, observationRef(id))
		}
		_, endLearn := hardening.EmitMemoryPersist(ctx, sessionID, "item", "round", 0, 0)
		assets, err := r.Learner.Learn(ctx, learn.LearnRequest{
			SessionID:    sessionID,
			Verdict:      verdict,
			Plan:         pl,
			Artifact:     art,
			Observations: obsLookups,
		})
		endLearn(err)
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
		var children []*workmodel.WorkItem
		{
			end := hardening.EmitWorktreeOp(ctx, sessionID, "list_children", item.ID, "")
			children = r.Tasks.Tree().ListChildren(sessionID, item.ID)
			end(nil)
		}
		for _, c := range children {
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
	{
		end := hardening.EmitWorktreeOp(ctx, sessionID, "set_round_phase", item.ID, string(workmodel.RoundPhaseDecide))
		_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhaseDecide)
		end(nil)
	}

	phase := workmodel.RoundPhaseIdle
	switch round.SpawnPolicy {
	case workmodel.SpawnAwait, workmodel.SpawnDecompose, workmodel.SpawnParallelExplore:
		phase = workmodel.RoundPhaseAwaitChild
	case workmodel.SpawnInline:
		phase = workmodel.RoundPhaseIdle
	default:
		phase = workmodel.RoundPhaseIdle
	}
	{
		end := hardening.EmitWorktreeOp(ctx, sessionID, "apply_pipeline_round", item.ID, string(phase))
		if err := r.Tasks.Tree().ApplyPipelineRound(sessionID, item.ID, round, phase); err != nil {
			end(err)
			return nil, err
		}
		end(nil)
	}

	if round.SpawnPolicy == workmodel.SpawnNone {
		status := workmodel.StatusAfterSpawnNone(verdict.Kind)
		if isRollup && verdict.Kind != types.VerdictPass {
			status = workmodel.TaskStatusInProgress
		}
		got, _ := r.Tasks.GetWorkItem(sessionID, item.ID)
		if got != nil && got.Status == workmodel.TaskStatusPending {
			end := hardening.EmitWorktreeOp(ctx, sessionID, "update_status", item.ID, string(workmodel.TaskStatusInProgress))
			_ = r.Tasks.Tree().UpdateStatus(sessionID, item.ID, workmodel.TaskStatusInProgress)
			end(nil)
		}
		if status != workmodel.TaskStatusInProgress {
			end := hardening.EmitWorktreeOp(ctx, sessionID, "update_status", item.ID, string(status))
			_ = r.Tasks.Tree().UpdateStatus(sessionID, item.ID, status)
			end(nil)
		}
	} else if item.Status == workmodel.TaskStatusPending {
		end := hardening.EmitWorktreeOp(ctx, sessionID, "update_status", item.ID, string(workmodel.TaskStatusInProgress))
		_ = r.Tasks.Tree().UpdateStatus(sessionID, item.ID, workmodel.TaskStatusInProgress)
		end(nil)
	}

	if isRollup && verdict.Kind == types.VerdictPass {
		_ = r.Tasks.Tree().SetNeedsRollup(sessionID, item.ID, false)
	}

	item.LastRound = round
	item.RoundPhase = phase
	item.Uncertainty = uncertaintyMean
	if got, ok := r.Tasks.GetWorkItem(sessionID, item.ID); ok && got != nil && workmodel.IsTerminalStatus(got.Status) {
		r.Tasks.RecordPeerStatusOnTerminal(sessionID, got)
	}
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
		TaskID:     item.ID,
		SessionID:  sessionID,
		WorkerType: wavescheduler.WorkerWorkItem,
		// DM-20260626-009 follow-up: WorkerWorkItem artifacts carry the LLM's
		// full ReAct response — this IS the user's answer, not a brief task
		// digest. Truncating at 200 chars (the wave-worker summary cap) cut
		// long reviews short and the feishu reply card showed only the first
		// 200 chars + ellipsis. Skip truncation here; Learn node truncates
		// evidence further (asset_builder.go:272 truncates to 64) so the
		// downstream path is unaffected.
		Summary:   content,
		ExitCode:  exit,
		Error:     errMsg,
		StartedAt: started,
		EndedAt:   ended,
		Duration:  ended.Sub(started),
		Metadata: map[string]any{
			"source":      WorkItemSourceLabel,
			"stop_reason": stopReason,
			"iterations":  iterations,
			"tool_calls":  toolCalls,
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