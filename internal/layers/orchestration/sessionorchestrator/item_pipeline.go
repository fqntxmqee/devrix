package sessionorchestrator

import (
	"context"
	"fmt"
	"strings"
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
	StrategicPlanProposer StrategicPlanProposer
	// Executor runs the per-WorkItem ReAct loop (DM-20260626-009).
	// Replaces the prior Router/Channel/tool-pipeline path.
	Executor WorkItemExecutor
	// Verify overrides deterministic artifact verification (tests / future LLM verifier).
	Verify func(*wavescheduler.Artifact) workmodel.Verdict
	// Emit is the deprecated single-emit field. Use ItemPipelineRunOpts.Emit
	// on Run() / RunParallelExplore() so concurrent sessions each carry
	// their own emit closure instead of racing on a shared struct field.
	//
	// Deprecated (DM-20260630-013, RH-D7-01): retained for one release so
	// external callers and tests that pre-date PerInvocationEmit continue
	// to compile. New code MUST use the opts-based path. Setting this
	// field while two sessions run concurrently is the root cause of
	// cross-session event leakage (RH-D7-01).
	Emit func(*contracts.EngineEvent)
}

// ItemPipelineRunOpts carries per-invocation state that must NOT live on
// the shared ItemPipelineRunner struct (RH-D7-01, DM-20260630-013).
//
// Per-invocation scoping matters because a single ItemPipelineRunner is
// typically shared across many sessions (singleton bootstrap). Putting
// Emit on the struct produced cross-session event leakage when two
// sessions ran concurrently — RunSessionTurnLoop for session A would
// install emitFn_A on the shared runner, then session B's loop would
// install emitFn_B, and session A's later ReAct events would flow to
// session B's sink. ItemPipelineRunOpts breaks that aliasing by making
// the closure an argument of Run() rather than a field of the runner.
type ItemPipelineRunOpts struct {
	// Emit forwards intermediate engine events from the ReAct loop to the
	// gateway so feishu cards show live tool_call / tool_result / text
	// alongside the final ArtifactSummary. nil → no-op.
	Emit func(*contracts.EngineEvent)
}

// Resolve returns opts, falling back to the runner's deprecated Emit
// field for callers that have not migrated yet. Removed in a follow-up
// release once all callers are migrated.
func (r *ItemPipelineRunner) Resolve(opts ItemPipelineRunOpts) ItemPipelineRunOpts {
	if opts.Emit == nil && r != nil && r.Emit != nil {
		opts.Emit = r.Emit
	}
	return opts
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
	// ObservationProposer optional @ Observe; production uses D2 Prepare → D3 (DM-20260630-001).
	ObservationProposer ObservationProposer
	StrategicPlanProposer StrategicPlanProposer
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
		Classifier:            classifier,
		Planner:               planner,
		Learner:               deps.Learner,
		Tasks:                 deps.Tasks,
		TrackMode:             deps.TrackMode,
		Executor:              deps.Executor,
		ObservationProposer:   deps.ObservationProposer,
		StrategicPlanProposer: deps.StrategicPlanProposer,
	}, nil
}

// Run executes the full per-WorkItem MUPS pipeline and persists LastRound (Phase B).
//
// Per-invocation Emit is carried in opts (RH-D7-01, DM-20260630-013). The
// shared runner's Emit field is retained only as a transitional fallback
// for callers that have not migrated yet.
func (r *ItemPipelineRunner) Run(ctx context.Context, sessionID string, item *workmodel.WorkItem, userID string, opts ItemPipelineRunOpts) (*workmodel.WorkItemPipelineRound, error) {
	if r == nil {
		return nil, fmt.Errorf("item_pipeline: runner nil")
	}
	if item == nil || item.ID == "" {
		return nil, fmt.Errorf("item_pipeline: work item required")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("item_pipeline: sessionID required")
	}
	opts = r.Resolve(opts)
	if workmodel.IsHumanReviewItem(item) && item.Status == workmodel.TaskStatusPending {
		return r.runHumanReviewAwait(ctx, sessionID, item)
	}
	// Propagate Emit hook to Executor so the ReAct loop's intermediate
	// events (text / thinking / tool_call / tool_result) flow to the
	// gateway. nil-safe — legacy / test runners without Emit continue to
	// work unchanged.
	//
	// RH-D7-01 (DM-20260630-013): emit is sourced from per-invocation opts
	// rather than the shared runner field. The runner field is consulted
	// only as a transitional fallback by Resolve() above. This is the
	// architectural fix that prevents cross-session event leakage when two
	// sessions share one runner.
	if opts.Emit != nil {
		if exec, ok := r.Executor.(*DefaultWorkItemExecutor); ok {
			exec.Emit = opts.Emit
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

	var strategic *StrategicPlanProposal
	if r.StrategicPlanProposer != nil && !isRollup {
		intentKind := ""
		if report.QuantizedIntent != nil {
			intentKind = string(report.QuantizedIntent.Kind)
		}
		prop, propErr := r.StrategicPlanProposer.ProposeStrategicPlan(ctx, StrategicPlanInput{
			SessionID:      sessionID,
			WorkItemID:     item.ID,
			Directive:      directive,
			ObservationIDs: obsIDs,
			ReportSummary:  uncertaintyReportSummary(len(report.Anomalies), intentKind),
		})
		if propErr == nil && prop != nil {
			strategic = prop
			applyStrategicScope(sessionID, item, prop, r.Tasks)
		}
	}

	expectedReturn := workmodel.ExpectedReturnForItem(r.Tasks, sessionID, item)
	deliverableSchema := workmodel.InferDeliverableSchema(item, directive, expectedReturn)
	if strategic != nil && strategic.DeliverableSchema != "" {
		deliverableSchema = strategic.DeliverableSchema
	}

	qKind := planQuantizedKind(item, report)
	if strategic != nil && strings.TrimSpace(strategic.QuantizedKind) != "" {
		qKind = strategic.QuantizedKind
	}
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
	maxItersOverride := 0
	if strategic != nil && strategic.ReactItersHint > 0 {
		maxItersOverride = strategic.ReactItersHint
	}
	// RH-MUPS-10 (DM-20260701-001): when this is a retry of a non-Pass
	// round (the spawn policy above decided SpawnInline rather than
	// SpawnNone/Decompose), the producer must see WHY the previous
	// attempt failed so it can self-correct. We only surface a non-empty
	// reason when the prior round's verdict is Fail/Partial AND we are
	// not the first round; a Pass round carries no reason to learn from.
	priorReason := ""
	if item.LastRound != nil && item.LastRound.VerdictKind != types.VerdictPass {
		priorReason = strings.TrimSpace(item.LastRound.ExitReason)
	}
	execCtx := WithWorkItemExecContext(ctx, WorkItemExecContext{
		Item:              item,
		Tasks:             r.Tasks,
		MaxItersOverride:  maxItersOverride,
		DeliverableSchema: deliverableSchema,
		PriorVerifyReason: priorReason,
	})
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

	// RH-MUPS-04 (DM-20260701-001): compute child-outcome stats BEFORE the
	// verify step so rollup verify can refuse to mark the parent Completed
	// when Failed==Total. Without this, a rollup that synthesizes a
	// well-formed summary from all-failed children would still Pass and the
	// parent would be marked Completed — the all-failure case gets washed
	// into apparent success.
	rollupChildStats := workmodel.ChildOutcomeStats{}
	if isRollup && r.Tasks != nil {
		for _, c := range r.Tasks.Tree().ListChildren(sessionID, item.ID) {
			if c == nil || c.Kind == workmodel.WorkKindChecklist {
				continue
			}
			rollupChildStats.Total++
			switch c.Status {
			case workmodel.TaskStatusCompleted:
				rollupChildStats.Completed++
			case workmodel.TaskStatusFailed, workmodel.TaskStatusCancelled:
				rollupChildStats.Failed++
			case workmodel.TaskStatusInProgress, workmodel.TaskStatusPending:
				rollupChildStats.Running++
			}
		}
	}

	var verdict workmodel.Verdict
	var deliverableResult DeliverableVerifyResult
	if isRollup {
		verdict = verifyRollupArtifact(art, rollupChildStats)
		deliverableResult = DeliverableVerifyResult{Status: workmodel.DeliverableStatusNotApplicable}
	} else if r.Verify != nil {
		verdict = r.Verify(art)
		deliverableResult = VerifyDeliverable(deliverableSchema, art)
	} else {
		out := verifyArtifactForWorkItemWithSchema(art, item, pl, deliverableSchema)
		verdict = out.Verdict
		deliverableResult = out.Deliverable
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
	// RH-MUPS-01/02 (DM-20260701-001): route through ReconcileUncertainty so
	// convergence (all children terminal) is numerically visible. The prior
	// naked max ratchet (`item.Uncertainty > uncertaintyMean ? item.Uncertainty : uncertaintyMean`)
	// made the value monotonically non-decreasing across the WorkItem
	// lifetime — even after rollup succeeded, item.Uncertainty stayed pinned
	// at its historical peak. Downstream readers (uncertainty thresholds,
	// llmClaim feedback) then overestimated remaining uncertainty and could
	// trigger spurious decomposition / escalation. ReconcileUncertainty
	// drops prevStored once children are terminal; while children run, a
	// damped max protects against single-round optimism. See
	// workmodel/uncertainty_reconcile.go.
	uncertaintyMean = workmodel.ReconcileUncertainty(item.Uncertainty, uncertaintyMean, stats)

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
		DeliverableSchema: deliverableSchema,
		StartedAt:         started,
		CompletedAt:       time.Now(),
	}
	// RH-MUPS-03 (DM-20260701-001): increment RollupRetries on non-Pass
	// rollup rounds so SpawnPolicyEvaluator can escalate after the
	// configured ceiling. Pass resets the counter so a later successful
	// rollup immediately stops the retry clock.
	if isRollup {
		if verdict.Kind == types.VerdictPass {
			round.RollupRetries = 0
		} else if item.LastRound != nil {
			round.RollupRetries = item.LastRound.RollupRetries + 1
		}
	}
	if !isRollup {
		round.DeliverableStatus = deliverableResult.Status
		round.StructuredDeliverable = deliverableResult.Payload
	}
	if strategic != nil {
		if s := strings.TrimSpace(strategic.Rationale); s != "" {
			round.SpawnRationale = s
		}
		if len(strategic.ChildSpecs) > 0 {
			round.ChildSpecs = append([]workmodel.ChildSpec(nil), strategic.ChildSpecs...)
		}
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
		schemaForStatus := deliverableSchema
		deliverableStatus := deliverableResult.Status
		if r.Verify != nil && !isRollup {
			schemaForStatus = workmodel.DeliverableSchemaNotApplicable
		}
		if isRollup {
			schemaForStatus = workmodel.DeliverableSchemaNotApplicable
			deliverableStatus = workmodel.DeliverableStatusNotApplicable
		}
		status := workmodel.StatusAfterSpawnNone(verdict.Kind, schemaForStatus, deliverableStatus)
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