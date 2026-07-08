package sessionorchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/prompttags"
	"github.com/devrix/devrix/internal/shared/textutil"
	"github.com/devrix/devrix/internal/shared/types"
)

// ItemPipelineRunner executes Observe→Plan→Execute→Verify→Learn→Decide for one WorkItem.
//
// DM-20260626-009: the Execute phase goes through WorkItemExecutor
// directly (per-WorkItem ReAct loop). Planner is kept for round
// metadata + Learn lineage; the Plan.Steps are vestigial (Executor reads
// the directive from WorkItem directly, not from Plan.Steps[0].ToolArgs).
type ItemPipelineRunner struct {
	Classifier            decisionplanning.IntentClassifier
	Planner               plan.Planner
	Learner               learn.Learner
	Tasks                 *workmodel.TaskManager
	TrackMode             string
	ContextProposer       workmodel.ContextProposer
	ObservationProposer   ObservationProposer
	StrategicPlanProposer StrategicPlanProposer
	// Executor runs the per-WorkItem ReAct loop (DM-20260626-009).
	// Replaces the prior Router/Channel/tool-pipeline path.
	Executor WorkItemExecutor
	// Verify overrides deterministic artifact verification (tests / future LLM verifier).
	Verify func(*wavescheduler.Artifact) workmodel.Verdict
	// SemanticVerifier asks the LLM to judge whether the round's
	// ArtifactSummary actually answers the user's original question
	// (DM-20260706-006). nil → no semantic verify; pipeline falls
	// back to the code-based verdict (current behavior). When wired,
	// the cheap Jaccard pre-check (looksLikeTemplateMimicry via
	// SemanticConfig) decides whether to actually CALL the verifier,
	// so the LLM cost is bounded to stagnation-suspect rounds only.
	SemanticVerifier SemanticVerifier
	// SemanticConfig controls when SemanticVerifier is consulted. nil →
	// DefaultSemanticSimilarityConfig() (Enabled=false by default).
	SemanticConfig SemanticSimilarityConfig
	// DAGExecutor drives the multi-intent DAG path (DM-20260707-001 PR-C).
	// When Plan emits pl.DAG + pl.IntentSegmentSet, Run() forks from the
	// legacy WorkItemExecutor path into executePlanDAG (this file's
	// companion helper). nil → DAG plan fall back to legacy path (defensive
	// default so pre-PR-C callers continue to compile).
	DAGExecutor wavescheduler.DAGExecutor
	// DAGEnabled is the runtime gate (T29, devrix.d7.dag_executor.enabled,
	// default false). When false, the multi-intent DAG fork at Run() is
	// suppressed regardless of whether pl.DAG is set — staging rollout
	// per proposal.md §6 ("first 5% → 100% across 2 weeks"). True means
	// ops has flipped the flag and the DAG executor handles PlanDAG
	// round trips. nil executor + true flag → degrade gracefully to
	// legacy path (log warning, no panic).
	DAGEnabled bool
	// StreamingEmitter is the IM-side streaming emit adapter used by the
	// PR-C DAG path. Decoupled from adapters.FeishuAdapter via this
	// interface to avoid a test-only import cycle
	// (sessionorchestrator → adapters → capture → sessionorchestrator-test).
	// *adapters.FeishuAdapter satisfies this interface implicitly via its
	// EmitPartialCard / EmitFinalCard methods. nil → DAG path still runs
	// the inner Execute + Learn, but skips the IM streaming card emit.
	StreamingEmitter StreamingEmitter
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
	// TurnNo is the session turn counter from TurnState (0 when unwired).
	TurnNo int
	// LoopTick is the 1-based RunSessionTurnLoop for{} iteration index.
	LoopTick int
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
	ObservationProposer   ObservationProposer
	StrategicPlanProposer StrategicPlanProposer
	// DM-20260706-006: pass through the semantic verify wiring. Both
	// fields are optional — nil cfg → production default
	// (Enabled=true), nil verifier → no LLM call (the cheap Jaccard
	// pre-check still runs but short-circuits when verifier is nil).
	SemanticConfig   SemanticSimilarityConfig
	SemanticVerifier SemanticVerifier
	// DM-20260707-001 PR-C: DAGExecutor + StreamingEmitter. Optional —
	// nil DAGExecutor → legacy single-WorkItem path; nil StreamingEmitter
	// → DAG path still runs, but no IM streaming card is emitted.
	DAGExecutor     wavescheduler.DAGExecutor
	StreamingEmitter StreamingEmitter
	// DM-20260707-001 PR-D (T29): DAGEnabled gates the multi-intent fork
	// at Run(). Mirrors config.DAGExecutor.Enabled. false (zero value)
	// keeps the legacy single-WorkItem path even when DAGExecutor is
	// wired; true flips the fork on. Defaults preserve backward
	// compatibility for any pre-PR-D caller.
	DAGEnabled bool
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
	semanticCfg := deps.SemanticConfig
	// Use the zero-value of MinSimilarityForVerify as a sentinel for
	// "not wired"; fall back to defaults in that case so the bootstrap
	// path always gets a valid config.
	if semanticCfg.MinSimilarityForVerify == 0 && semanticCfg.MinArtifactChars == 0 && !semanticCfg.Enabled {
		semanticCfg = DefaultSemanticSimilarityConfig()
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
		SemanticConfig:        semanticCfg,
		SemanticVerifier:      deps.SemanticVerifier,
		DAGExecutor:           deps.DAGExecutor,
		StreamingEmitter:      deps.StreamingEmitter,
		DAGEnabled:            deps.DAGEnabled,
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
	if got, ok := r.Tasks.GetWorkItem(sessionID, item.ID); ok && got != nil {
		item = got
	}
	isRollup := item.NeedsRollup
	isDeliverableSynth := isRollup && workmodel.IsDeliverableFormatRollupSynth(r.Tasks, sessionID, item)
	isParentRollup := isRollup && workmodel.IsParentRollupSynth(r.Tasks, sessionID, item)
	r.Tasks.Tree().EnsureSemanticID(sessionID, item)
	if got, ok := r.Tasks.GetWorkItem(sessionID, item.ID); ok && got != nil {
		item = got
	}
	directive := DirectiveForItem(sessionID, item, r.Tasks)
	if item.Kind == workmodel.WorkKindGoal {
		directive = DirectiveForGoalPlan(item, directive)
	}

	started := time.Now()
	roundNo := 1
	if item.LastRound != nil {
		roundNo = item.LastRound.RoundNo + 1
	}
	trigger := workmodel.InferMUPSTrigger(item, isRollup)
	frame := workmodel.LocatorFrame{
		SessionID:    sessionID,
		TurnNo:       opts.TurnNo,
		LoopTick:     opts.LoopTick,
		WorkItemID:   item.ID,
		SemanticID:   item.SemanticID,
		Depth:        r.Tasks.Tree().Depth(sessionID, item.ID),
		SiblingIndex: r.Tasks.Tree().SiblingIndex(sessionID, item.ID),
		RoundNo:      roundNo,
		Trigger:      trigger,
	}
	ctx = workmodel.WithLocatorFrame(ctx, frame)
	// DM-20260626-009 hotfix: emit the v6.0.0 5-node MUPS root span so the
	// per-WorkItem ItemPipelineRunner path is observable in Jaeger. hardening
	// uses a package-level bridge (SetBridge in bootstrap/wire_coordinator.go),
	// so this works without an obsBridge field on ItemPipelineRunner.
	ctx, endMUPS := hardening.EmitMUPSPipeline(ctx, sessionID, item.ID, "item_pipeline")
	defer func() { endMUPS(nil) }()
	ctx = contracts.WithMUPSPrepareCache(ctx)

	// DM-20260706-004 (devrix-d7-frame-delta-phase2-production-wiring):
	// construct the prior-round WorkItemExecContext (Phase 2 wiring) so
	// observeWorkItem → buildObserveSignalInput → BuildObservePriorDelta
	// can synthesize a non-zero FrameDelta on Round 2+. Item + Tasks are
	// the only fields BuildObservePriorDelta reads; MaxItersOverride /
	// DeliverableContract / PlanFrameDelta / Emit are all Execute-side
	// concerns and intentionally stay nil/zero here. Designed not to
	// affect subsequent Execute-side WithWorkItemExecContext construction
	// at line ~513 — that path binds the per-Execute fields; both
	// pointers reference the same WorkItem tree node so callers reading
	// LastRound stay consistent across phases.
	prevExecCtx := &WorkItemExecContext{
		Item:  item,
		Tasks: r.Tasks,
	}
	ctx, endObservePhase := r.enterMUPSPhase(ctx, sessionID, item.ID, workmodel.RoundPhaseObserve)
	report, obsIDs, observeParseReject, err := observeWorkItem(ctx, sessionID, item, r.Classifier, r.Learner, r.TrackMode, r.Tasks, prevExecCtx, r.ObservationProposer)
	if err != nil {
		endObservePhase(err)
		return nil, err
	}
	endObservePhase(nil)

	// DM-20260706-011 observational_answer fast-path: when Observe emits a
	// high-strength CatBusiness ObsFact with no ObsUncertainty, skip the
	// Plan + Execute + Verify 3 nodes and emit ObsFact.Statement directly
	// as the user-visible finalText. Learn still runs so reputation scoring
	// keeps observing the observer's accuracy. Saves ~1 LLM call + 3-5s
	// latency for trivial deterministic Q&A (1+1=几, 2×3=几, 法国首都是哪).
	//
	// Gates (all required):
	//   1. !isRollup && !isDeliverableSynth && !isParentRollup
	//   2. r.Learner != nil (Learn is the only reputation writer)
	//   3. !hasObsUncertainty(report) — any unresolved question blocks fast-return
	//   4. pickHighStrengthBusinessFact(report, 0.85) returns a non-empty
	//      Statement (CatBusiness, strength ≥ 0.85 — matches
	//      maxLLMObsFactStrength cap, so the LLM proposer can hit it
	//      deterministically without bypassing validation)
	//
	// Degradation: any gate miss → fall through to the legacy Plan+Execute
	// path with no error.
	if !isRollup && !isDeliverableSynth && !isParentRollup && r.Learner != nil &&
		!hasObsUncertainty(report) {
		if obsID, factStmt, ok := pickHighStrengthBusinessFact(report, 0.85); ok {
			round, fpErr := r.maybeObservationalAnswer(ctx, sessionID, item, report, obsIDs, obsID, factStmt, started, roundNo, frame.Trigger)
			if fpErr != nil {
				// Fast-path construction failed (e.g. tree mutation error);
				// fall through to the legacy Plan+Execute path. The user's
				// experience is "slightly slower" not "error".
				slog.Warn("item_pipeline: observational_answer fast-path failed; falling through to Plan",
					"session_id", sessionID, "work_item_id", item.ID,
					"obs_fact_id", obsID, "err", fpErr)
			} else if round != nil {
				return round, nil
			}
		}
	}

	ctx, endPlanPhase := r.enterMUPSPhase(ctx, sessionID, item.ID, workmodel.RoundPhasePlan)

	var strategic *StrategicPlanProposal
	strategicRejectRationale := ""
	planParseReject := ""
	priorPlanReject := ""
	if item.LastRound != nil {
		priorPlanReject = strings.TrimSpace(item.LastRound.PlanParseReject)
	}
	if r.StrategicPlanProposer != nil && !isRollup {
		intentKind := ""
		if report.QuantizedIntent != nil {
			intentKind = string(report.QuantizedIntent.Kind)
		}
		// RH-MUPS-07 (DM-20260701-001 T-P1-2): build the divergence budget
		// snapshot + carry the parent's in-scope paths so the LLM proposer
		// sees the same numbers the cap function will use. Without this
		// the proposer would propose 7 children on a 3-remaining budget
		// and CapChildSpecs would silently truncate to 3.
		var parentScopeIn []string
		if item.ScopeContract != nil {
			parentScopeIn = append([]string(nil), item.ScopeContract.InScope...)
		}
		prop, propErr := r.StrategicPlanProposer.ProposeStrategicPlan(ctx, StrategicPlanInput{
			SessionID:       sessionID,
			WorkItemID:      item.ID,
			Directive:       directive,
			ObservationIDs:  obsIDs,
			ReportSummary:   uncertaintyReportSummary(report, intentKind),
			Budget:          workmodel.StrategicPlanBudget(sessionID, item, r.Tasks),
			ParentScopeIn:   parentScopeIn,
			UncertaintyMean: item.Uncertainty,
			PriorParseReject: priorPlanReject,
			Report:          report,
		})
		if propErr == nil && prop != nil {
			strategic = prop
			workDir := r.Tasks.SessionWorkDir(sessionID)
			scopeIn, scopeAccepted, scopeReason := workmodel.PrepareStrategicScopeIn(directive, prop.ScopeIn, workDir)
			prop.ScopeIn = scopeIn
			if scopeReason != "" && !scopeAccepted {
				if strategicRejectRationale != "" {
					strategicRejectRationale += "\n"
				}
				strategicRejectRationale += "scope: " + scopeReason
				planParseReject = prompttags.NewPlanParseReject(prompttags.RejectScopeGate, "scope_in", scopeReason, 0, 0).CompactJSON()
			}
			if len(prop.ScopeIn) > 0 {
				applyStrategicScope(sessionID, item, prop, r.Tasks)
			}
		} else if propErr != nil {
			var reject *StrategicPlanReject
			if errors.As(propErr, &reject) {
				strategicRejectRationale = formatStrategicPlanReject(reject)
				planParseReject = parseRejectFromStrategicPlan(reject).CompactJSON()
			} else {
				planParseReject = parseRejectFromPlanError(propErr).CompactJSON()
			}
		}
	}

	expectedReturn := workmodel.ExpectedReturnForItem(r.Tasks, sessionID, item)
	deliverableContract := workmodel.InferDeliverableContract(item, directive, expectedReturn)
	if strategic != nil {
		if strategic.DeliverableContract.ContractApplicable() {
			deliverableContract = workmodel.NarrowestContract(deliverableContract, strategic.DeliverableContract)
		} else if strategic.DeliverableSchema != "" {
			deliverableContract = workmodel.NarrowestContract(
				deliverableContract,
				workmodel.ExpandLegacySchemaToContract(strategic.DeliverableSchema),
			)
		}
	}
	deliverableSchema := workmodel.InferDeliverableSchema(item, directive, expectedReturn)
	if strategic != nil && strategic.DeliverableSchema != "" {
		deliverableSchema = workmodel.NarrowestSchema(deliverableSchema, strategic.DeliverableSchema)
	}
	deliverableContract = workmodel.ClampDeliverableContract(deliverableContract)

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
	if isParentRollup {
		planInput.FailureCriteria = rollupFailureCriteria()
	}
	_, endPlan := hardening.EmitTaskGraphSynthesize(ctx, sessionID, len(planInput.Steps), 0, 1, false)
	pl, err := r.Planner.Plan(planInput)
	endPlan(err)
	if err != nil {
		endPlanPhase(err)
		return nil, fmt.Errorf("item_pipeline: plan: %w", err)
	}
	if isParentRollup && pl != nil {
		pl.Kind = plan.CommitmentPlan
	}
	endPlanPhase(nil)

	// DM-20260707-001 PR-C: fork into the multi-intent DAG execution path
	// when Plan emits a non-nil DAG + IntentSegmentSet. codex Risk A8 HIGH
	// — fork on pl.DAG (NOT proposal.DAG; strategic plan proposals don't
	// carry the DAG). The helper handles Execute + Verify + Learn
	// internally and persists the rollup round, so we return immediately.
	//
	// DM-20260707-001 PR-D (T29): the `r.DAGEnabled` gate is the
	// runtime config flag (devrix.d7.dag_executor.enabled, default false).
	// When ops has not flipped the flag, fall through to the legacy
	// single-WorkItem path even if pl.DAG is non-nil — the fork is
	// suppressed but the rest of the pipeline runs unchanged so the
	// plan content still gets exercised via the legacy channels.
	if r.DAGEnabled && pl != nil && pl.DAG != nil && pl.IntentSegmentSet != nil && r.DAGExecutor != nil {
		// Build obsLookups once for the DAG path (the legacy Learn block
		// at line ~585 builds the same slice; hoisting it here keeps the
		// DAG helper's signature clean).
		dagObsLookups := make([]learn.ObservationLookup, 0, len(obsIDs))
		for _, id := range obsIDs {
			dagObsLookups = append(dagObsLookups, observationRef(id))
		}
		round, dagErr := r.executePlanDAG(
			ctx, sessionID, userID, item, pl, directive, roundNo, trigger, started,
			isParentRollup, dagObsLookups)
		if dagErr != nil {
			return nil, dagErr
		}
		return round, nil
	}
	// PR-D T29 degraded path: flag is OFF but Plan still emitted a DAG
	// (e.g. a previous turn set it). Drop a single audit log so ops can
	// notice the mismatch and investigate. The legacy path will run with
	// the plan's Children if present, otherwise with Steps[0].
	if !r.DAGEnabled && pl != nil && pl.DAG != nil {
		slog.Debug("item_pipeline: dag_executor flag is OFF; falling through to legacy path despite PlanDAG",
			"session_id", sessionID, "work_item_id", item.ID,
			"dag_node_count", len(pl.DAG.Nodes))
	}

	ctx, endExecutePhase := r.enterMUPSPhase(ctx, sessionID, item.ID, workmodel.RoundPhaseExecute)

	// Execute via WorkItemExecutor (per-WorkItem ReAct loop). Emit Wave +
	// Execute sub-spans so Jaeger shows the full 5-node tree.
	_, endWave := hardening.EmitExecutorSelect(ctx, sessionID, 1, "workitem", "0", "item_pipeline")
	endWave(nil)
	_, endExecute := hardening.EmitChannelRoute(ctx, sessionID, "item", "workitem", "0", "")
	maxItersOverride := 0
	if strategic != nil && strategic.ReactItersHint > 0 {
		maxItersOverride = strategic.ReactItersHint
	}
	if isDeliverableSynth {
		maxItersOverride = 1
	} else {
		maxItersOverride = workmodel.EffectiveExecuteMaxIters(maxItersOverride, DefaultWorkItemMaxIters, deliverableContract)
	}
	// RH-MUPS-10 (DM-20260701-001): surface deliverable failure + scope anchor on
	// inline retry; omit spawn rationale / artifact prose (scope drift).
	execDeliverableContract := deliverableContract
	if isParentRollup {
		execDeliverableContract = workmodel.RollupDeliverableContract()
	} else if isDeliverableSynth && item.LastRound != nil {
		prior := item.LastRound.DeliverableContract
		if !prior.ContractApplicable() {
			prior = workmodel.ExpandLegacySchemaToContract(item.LastRound.DeliverableSchema)
		}
		if prior.ContractApplicable() {
			execDeliverableContract = workmodel.NarrowestContract(deliverableContract, prior)
		}
	}
	priorReason := PriorDeliverableRetryHint(item, execDeliverableContract)
	if extra := machineSpawnFeedback(item); extra != "" {
		if priorReason != "" {
			priorReason += "\n"
		}
		priorReason += extra
	}
	// DM-20260706-006 (Semantic Convergence): if the prior round's LLM
	// verifier asked the model to self-correct (e.g. "your last answer
	// was template-mimicry; address the original question with concrete
	// content"), surface that hint to the next round's LLM directive.
	// Without this injection, the LLM would re-emit the same template
	// and the convergence check would re-fire indefinitely.
	if item != nil && item.LastRound != nil {
		if hint := strings.TrimSpace(item.LastRound.SemanticRetryHint); hint != "" {
			if priorReason != "" {
				priorReason += "\n"
			}
			priorReason += "semantic_retry: " + hint
		}
	}
	execCtx := WithWorkItemExecContext(ctx, WorkItemExecContext{
		Item:                 item,
		Tasks:                r.Tasks,
		MaxItersOverride:     maxItersOverride,
		DeliverableContract:  execDeliverableContract,
		DeliverableSchema:    deliverableSchema,
		PriorVerifyReason:    priorReason,
		Emit:                 opts.Emit,
		ResolutionStrategies: pl.ResolutionStrategies,
		// DM-20260705-010 (devrix-d7-mups-frame-delta-closure) Phase 1 T4:
		// bind the typed Plan→Execute FrameDelta into WorkItemExecContext so
		// Execute's system_prompt materialization (subturn_materialize.go) can
		// inject it via InjectPlanFrameDelta. nil/empty FrameDelta → legacy
		// baseline path (InjectPlanFrameDelta no-ops + emit prior_delta_empty).
		PlanFrameDelta: buildPlanFrameDeltaForExecCtx(strategic),
	})
	result, execErr := r.Executor.ExecuteWorkItem(execCtx, sessionID, item.ID, directive)
	endExecute(execErr)
	endExecutePhase(execErr)
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

	ctx, endVerifyPhase := r.enterMUPSPhase(ctx, sessionID, item.ID, workmodel.RoundPhaseVerify)

	// RH-MUPS-04 (DM-20260701-001): compute child-outcome stats BEFORE the
	// verify step so rollup verify can refuse to mark the parent Completed
	// when Failed==Total. Without this, a rollup that synthesizes a
	// well-formed summary from all-failed children would still Pass and the
	// parent would be marked Completed — the all-failure case gets washed
	// into apparent success.
	rollupChildStats := workmodel.ChildOutcomeStats{}
	if isParentRollup && r.Tasks != nil {
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
	verifyContract := deliverableContract
	if isDeliverableSynth {
		verifyContract = execDeliverableContract
	}
	if isParentRollup {
		verdict = verifyRollupArtifact(art, rollupChildStats)
		deliverableResult = DeliverableVerifyResult{Status: workmodel.DeliverableStatusNotApplicable}
	} else if r.Verify != nil {
		verdict = r.Verify(art)
		deliverableResult = VerifyDeliverableContract(verifyContract, art)
	} else {
		out := verifyArtifactForWorkItemWithContract(art, item, pl, verifyContract)
		verdict = out.Verdict
		deliverableResult = out.Deliverable
	}

	// DM-20260706-006 (Semantic Convergence): when the code-based verdict
	// rubber-stamped Pass but the current ArtifactSummary looks structurally
	// identical to a prior round (template-mimicry), ask the LLM whether
	// the round actually answered the user's question. The LLM verdict
	// overrides the code path → Decide sees VerdictFail → SpawnNone → loop
	// terminates instead of looping 20 rounds × 67s.
	//
	// Token-cost guard: looksLikeTemplateMimicry uses a cheap Jaccard
	// pre-check (interfaces.Jaccard, 0.85 threshold). The LLM call only
	// fires when there is structural stagnation. Healthy rounds (1-3 in a
	// fresh session) skip the verify entirely — zero overhead.
	//
	// Disabled by default (SemanticConfig.Enabled=false); the hotfix
	// default preserves current behavior until production validation.
	var semanticRetryHint string
	verdict, semanticRetryHint = r.maybeRunSemanticVerifier(ctx, sessionID, item, directive, art, verdict, roundNo)
	exitReason := exitReasonForVerdict(verdict, sessionID)
	endVerifyPhase(nil)

	// DM-20260707-001 PR-D (T46-T51) — run the Stage-5 Decision node
	// after Verify emits its verdict. The Decision node is a pure
	// function over the 11-row static mapping table; no LLM, no IO,
	// < 1ms. Its outputs (kind + reason + map_row + next_spec) get
	// persisted to round.DecisionKind/DecisionReason/DecisionMapRow/
	// DecisionJSON so D5 dashboards + Learn can read the routing
	// decision without re-running Decide. Old workmodel.ApplyPipelineDecide
	// is left intact for now — Decision's NextWorkItemSpec drives a
	// future C-path spawn loop; the A/B/D/E paths piggyback on the
	// existing SpawnPolicy machinery.
	decision := r.runDecisionStage(sessionID, item, verdict, roundNo)
	exitReason = exitReasonForDecision(exitReason, decision)

	ctx, endLearnPhase := r.enterMUPSPhase(ctx, sessionID, item.ID, workmodel.RoundPhaseLearn)

	// DM-20260626-009 hotfix: emit the 5th-node sub-span (system.anomaly_detect
	// as a verify stand-in). The current per-WorkItem path uses deterministic
	// verifyArtifact and shares the same op name so dashboards see a consistent
	// 5-node tree.
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
			endLearnPhase(err)
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
		WilsonLower:               wilsonLower,
		ChildStats:                stats,
		VerdictConfidence:         verdict.Confidence,
		EvidenceCount:             len(obsIDs),
		FormatFailureWithEvidence: formatFailureWithEvidence(stats, deliverableResult.Status, art, item),
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
		Trigger:           trigger,
		LoopTick:          opts.LoopTick,
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
		// DM-20260706-006 (Semantic Convergence): pass the LLM-emitted
		// retry hint into the round struct so the next round's
		// PriorDeliverableRetryHint can surface it to the LLM directive.
		// Without this, the hint stays local to maybeRunSemanticVerifier
		// and the next round's LLM re-emits the same template.
		SemanticRetryHint:   semanticRetryHint,
		DeliverableSchema:   deliverableSchema,
		DeliverableContract: deliverableContract,
		// DM-20260707-001 PR-D (T51): persist the Stage-5 Decision node
		// outcome. Kind/Reason/MapRow are the searchable fields; JSON is
		// the canonical wire blob for D5 dashboards and Learn attribution.
		DecisionKind:   decision.Kind.String(),
		DecisionReason: decision.Reason,
		DecisionMapRow: decision.MapRow,
		StartedAt:      started,
		CompletedAt:    time.Now(),
	}
	if decisionJSON, err := MarshalDecisionJSON(decision); err == nil {
		round.DecisionJSON = decisionJSON
	} else {
		slog.Warn("item_pipeline: marshal decision JSON failed",
			"session_id", sessionID, "work_item_id", item.ID, "err", err)
	}
	// DM-20260704-006 S4 Phase 2 wiring: compute the Verify → Decide
	// ResolutionCoverage report from the Plan's ResolutionStrategies and
	// the Execute artifact's ResolutionClaims. The Execute-side emission
	// is Phase 1.5 (D7-S16-A105-T01/T02); until that lands the claims
	// slice is empty and the report degrades to "no_resolution_claim"
	// for every ObsID — by design, the safety-net gate in
	// ComputeResolutionCoverage returns nil when Plan has no
	// ResolutionStrategies (legacy LLM rounds).
	if pl != nil && len(pl.ResolutionStrategies) > 0 {
		claims := extractResolutionClaimsFromArtifact(art, obsIDs)
		round.ResolutionClaims = claims
		round.ResolutionReport = verify.ComputeResolutionCoverage(
			pl.ResolutionStrategies, claims, sessionID, item.ID, roundNo,
		)
		// DM-20260704-006 Phase 5: emit the Verify→Decide handoff span
		// with CoverageRatio + unresolved_count metrics. Skipped when
		// the report is nil (legacy LLM rounds or empty strategies —
		// see verify.ComputeResolutionCoverage safety-net gate).
		if round.ResolutionReport != nil {
			_, endResCov := hardening.EmitResolutionCoverage(
				ctx, sessionID, item.ID, roundNo,
				round.ResolutionReport.TotalStrategies,
				round.ResolutionReport.TotalClaims,
				len(round.ResolutionReport.UnresolvedObs),
				round.ResolutionReport.CoverageRatio,
			)
			endResCov(nil)
		}
	}
	// DM-20260705-010 (devrix-d7-mups-frame-delta-closure) Phase 3 T13:
	// deterministic per-round ConvergenceMetric (工具结果 diff + claim 数 +
	// obs_uncertainty 残量), 0 LLM. Emitted as a Jaeger span so the next
	// round's Observe can read the closed-gap counts (Markov 链闭环). The
	// FrameDeltaConsumed flag records whether this round's Execute prompt
	// actually carried the injected Plan frame delta (not baseline fallback).
	frameDeltaConsumed := false
	if pfd := buildPlanFrameDeltaForExecCtx(strategic); pfd != nil && !pfd.IsZero() {
		frameDeltaConsumed = true
	}
	convMetric := ComputeConvergenceMetric([]SubTurnRecord{
		BuildRoundSubTurnRecord(obsIDs, round.ResolutionClaims, round.ResolutionReport, frameDeltaConsumed),
	}, nil)
	endConv := hardening.EmitConvergenceMetric(ctx, sessionID,
		convMetric.UncertaintyReductionRate, convMetric.ObservedGapsClosedCount,
		convMetric.FrameDeltaConsumed)
	endConv(nil)
	if observeParseReject != "" {
		round.ObserveParseReject = observeParseReject
	}
	if planParseReject != "" {
		round.PlanParseReject = planParseReject
	}
	// RH-MUPS-03 (DM-20260701-001): increment RollupRetries on non-Pass
	// rollup rounds so SpawnPolicyEvaluator can escalate after the
	// configured ceiling. Pass resets the counter so a later successful
	// rollup immediately stops the retry clock.
	if isParentRollup {
		if verdict.Kind == types.VerdictPass {
			round.RollupRetries = 0
		} else if item.LastRound != nil {
			round.RollupRetries = item.LastRound.RollupRetries + 1
		}
		// Parent rollup synthesis is not a deliverable-schema round.
		round.DeliverableSchema = workmodel.DeliverableSchemaNotApplicable
		round.DeliverableContract = workmodel.DeliverableContract{}
		round.DeliverableStatus = workmodel.DeliverableStatusNotApplicable
	} else {
		round.DeliverableStatus = deliverableResult.Status
		round.StructuredDeliverable = deliverableResult.Payload
		round.DeliverableReason = deliverableResult.Reason
	}
	if art != nil {
		if tc, ok := art.Metadata["tool_calls"].(int); ok {
			round.ExecuteToolCalls = tc
		}
	}
	if item.ScopeContract != nil && len(item.ScopeContract.InScope) > 0 {
		round.ScopeInPresent = true
	}
	if strategic != nil {
		if s := strings.TrimSpace(strategic.Rationale); s != "" {
			round.SpawnRationale = s
		}
		if len(strategic.ChildSpecs) > 0 {
			round.ChildSpecs = append([]workmodel.ChildSpec(nil), strategic.ChildSpecs...)
		}
	} else if strategicRejectRationale != "" {
		round.SpawnRationale = strategicRejectRationale
	}

	treeCtx := workmodel.DefaultTreeEvalContext(sessionID, item.ID, userID, r.Tasks)
	if item.LastRound != nil && item.LastRound.VerdictKind == types.VerdictIndeterminate {
		treeCtx.IndeterminateRetries = item.LastRound.IndeterminateRetries
	}
	treeCtx.DailyLimitExceeded = workmodel.DecomposeDailyLimitWouldExceed(sessionID, item.Kind, 1)

	ctxOut := workmodel.ProposeContextPipelineOutput(sessionID, item, round, r.Tasks, r.ContextProposer)
	workmodel.ApplyPipelineDecide(sessionID, item, round, ctxOut, treeCtx, r.Tasks)
	if strategicRejectRationale != "" {
		if round.SpawnRationale != "" {
			round.SpawnRationale += "\n"
		}
		round.SpawnRationale += strategicRejectRationale
	}
	if round.VerdictKind == types.VerdictIndeterminate {
		round.IndeterminateRetries = treeCtx.IndeterminateRetries + 1
	}
	workmodel.TouchInlineRetryAtMaxDepth(r.Tasks.Tree(), sessionID, item, round, treeCtx)
	endLearnPhase(nil)

	ctx, endDecidePhase := r.enterMUPSPhase(ctx, sessionID, item.ID, workmodel.RoundPhaseDecide)

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
			endDecidePhase(err)
			return nil, err
		}
		end(nil)
	}

	if round.SpawnPolicy == workmodel.SpawnNone {
		opts := workmodel.SpawnNoneTerminalOpts{
			IsRollup:                  isParentRollup,
			StripDeliverableForStatus: r.Verify != nil && !isParentRollup,
		}
		if err := workmodel.ApplyRoundTerminalization(
			r.Tasks.Tree(), sessionID, item.ID,
			verdict.Kind, deliverableSchema, deliverableResult.Status, opts,
		); err != nil {
			endDecidePhase(err)
			return nil, err
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
	endDecidePhase(nil)
	return round, nil
}

// maybeRunSemanticVerifier implements the DM-20260706-006 hotfix path:
//
//   1. Cheap Jaccard pre-check (looksLikeTemplateMimicry) gates the LLM call.
//      No stagnation signal → return code verdict unchanged (zero LLM cost).
//   2. On stagnation signal: call SemanticVerifier.VerifySemantically —
//      the LLM answers "did I actually address the user's question?".
//   3. If the LLM says VerdictFail (template_mimicry), override the code
//      verdict with the semantic verdict so Decide routes to SpawnNone.
//
// Behavior preserved when:
//   - SemanticVerifier is nil → no-op.
//   - SemanticConfig.Enabled=false → no-op.
//   - LLM call fails / times out → log + return code verdict unchanged.
//
// All thresholds + cost gates live in SemanticSimilarityConfig; this
// function contains zero hardcoded numeric loops.
//
// Returns (verdict, retryHint):
//   - verdict overrides codeVerdict when the LLM verdict is worse (Fail / Partial).
//     When the LLM agrees (Pass), the code verdict is preserved.
//   - retryHint is non-empty ONLY when the LLM chose decision="retry"
//     on a VerdictFail — it carries the LLM's reason to be fed into the
//     next round's ExecuteWorkItem via PriorVerifyReason so the LLM can
//     self-correct. When decision="stop", retryHint is empty (the loop
//     terminates via SpawnNone and no retry is desired).
func (r *ItemPipelineRunner) maybeRunSemanticVerifier(
	ctx context.Context,
	sessionID string,
	item *workmodel.WorkItem,
	directive string,
	art *wavescheduler.Artifact,
	codeVerdict workmodel.Verdict,
	roundNo int,
) (workmodel.Verdict, string) {
	if r == nil || r.SemanticVerifier == nil {
		return codeVerdict, ""
	}
	cfg := r.SemanticConfig
	if !cfg.Enabled {
		return codeVerdict, ""
	}
	// Fast-path: code already says Fail. Trust the code path (e.g. real
	// execute error / max_iters) and skip the LLM call. The semantic
	// verifier is for catching FALSE PASSES, not double-checking TRUE
	// FAILURES.
	if codeVerdict.Kind == types.VerdictFail {
		return codeVerdict, ""
	}

	// Rollup round: do not invoke semantic verifier. Rollup synthesis is
	// handled by verifyRollupArtifact + rollupDecisionTable; semantic
	// mimicry of a rollup summary is harmless because rollup retry
	// guards (RollupRetries >= MaxRollupRetries) terminate the loop
	// regardless. Avoids an LLM call on the hottest loop in the system.
	if item != nil && item.NeedsRollup {
		return codeVerdict, ""
	}

	currentSummary := ""
	if art != nil {
		currentSummary = art.Summary
	}
	if currentSummary == "" {
		return codeVerdict, ""
	}

	// Collect prior round summaries from this WorkItem's history. We
	// limit to cfg.LookbackN (default 5) to bound the LLM prompt size.
	priors := collectPriorRoundSummaries(item, 5)

	if !looksLikeTemplateMimicry(currentSummary, priors, cfg) {
		return codeVerdict, ""
	}

	// Cheap pre-check passed. Call the LLM semantic verifier.
	req := SemanticVerifyRequest{
		SessionID:            sessionID,
		ItemID:               itemSafeID(item),
		RoundNo:              roundNo,
		UserOriginalQuestion: directive,
		ArtifactSummary:      currentSummary,
		PriorRoundSummaries:  priors,
		CodeBasedVerdict:     codeVerdict,
	}
	newVerdict, err := r.SemanticVerifier.VerifySemantically(ctx, req)
	if err != nil {
		// Fail-open: log + keep code verdict. A missing semantic verdict
		// must never make things worse than current behavior (always Pass).
		slog.Warn("item_pipeline: semantic verifier error; fall back to code verdict",
			"item_id", itemSafeID(item), "round_no", roundNo, "err", err)
		return codeVerdict, ""
	}
	// Only override when the LLM verdict is WORSE than code. A LLM "pass"
	// on a code-Pass round is a no-op; a LLM "partial" downgrades Pass to
	// Partial; a LLM "fail" forces SpawnNone.
	if newVerdict.Kind == types.VerdictPass {
		return codeVerdict, ""
	}

	// Extract decision hint from the verdict Reason (set by
	// DefaultSemanticVerifier.VerifySemantically as "[decision=...] ...").
	decision, retryHint := extractDecisionAndHint(newVerdict)

	hardening.EmitSemanticConvergence(
		ctx,
		sessionID,
		itemSafeID(item),
		roundNo,
		verdictKindString(newVerdict.Kind),
		newVerdict.Confidence,
		newVerdict.Reason,
	)

	// decision="retry" on a VerdictFail: do NOT override verdict. Keep
	// the code VerdictPass so Decide picks SpawnNone... wait, that would
	// converge too aggressively. Better: downgrade Pass → Partial so
	// Decide routes to SpawnInline (existing deliverable-continuation
	// path) and the retry hint is surfaced to the LLM next round.
	if newVerdict.Kind == types.VerdictFail && decision == "retry" && retryHint != "" {
		downgraded := codeVerdict
		downgraded.Kind = types.VerdictPartial
		downgraded.Reason = "semantic_retry: " + retryHint
		downgraded.Confidence = newVerdict.Confidence
		downgraded.SourceID = newVerdict.SourceID
		return downgraded, retryHint
	}

	// decision="stop" (or default for fail): override to VerdictFail so
	// Decide routes to SpawnNone (non-Scenario + non-Exploration) →
	// TaskStatusFailed → loop terminates.
	if newVerdict.Kind == types.VerdictFail {
		return newVerdict, ""
	}

	// VerdictPartial (regardless of decision): downgrade code Pass to
	// Partial so Decide picks SpawnInline + surfaces the retry hint.
	if newVerdict.Kind == types.VerdictPartial && retryHint != "" {
		downgraded := codeVerdict
		downgraded.Kind = types.VerdictPartial
		downgraded.Reason = "semantic_partial: " + retryHint
		downgraded.Confidence = newVerdict.Confidence
		downgraded.SourceID = newVerdict.SourceID
		return downgraded, retryHint
	}
	return newVerdict, ""
}

// extractDecisionAndHint pulls "[decision=stop]" / "[decision=retry]"
// prefix out of verdict.Reason and returns (decision, cleanedReason).
// cleanedReason is what callers should surface as the retry hint.
// Returns ("", "") when no decision prefix is present.
func extractDecisionAndHint(v workmodel.Verdict) (string, string) {
	r := strings.TrimSpace(v.Reason)
	if !strings.HasPrefix(r, "[decision=") {
		return "", ""
	}
	end := strings.Index(r, "]")
	if end < 0 {
		return "", ""
	}
	decision := strings.TrimSpace(r[len("[decision="):end])
	cleaned := strings.TrimSpace(r[end+1:])
	return decision, cleaned
}

// verdictKindString renders a VerdictKind as a stable string for span
// attributes. Mirrors types.VerdictKind.String() if present; falls back
// to a fmt of the int value to avoid an import dependency.
func verdictKindString(k types.VerdictKind) string {
	switch k {
	case types.VerdictPass:
		return "pass"
	case types.VerdictPartial:
		return "partial"
	case types.VerdictFail:
		return "fail"
	case types.VerdictIndeterminate:
		return "indeterminate"
	default:
		return "unknown"
	}
}

// collectPriorRoundSummaries walks item.LastRound history backward and
// returns up to maxN prior ArtifactSummary strings, oldest first.
//
// Note: as of 2026-07-06 the ItemPipelineRunner only retains a single
// LastRound pointer on the WorkItem (older rounds are dropped after the
// next round overwrites LastRound). This function returns whatever is
// available; for items that only persist the latest round, the slice
// is empty and the Jaccard pre-check trivially returns false (no
// stagnation signal possible). When RoundHistory is added in a future
// change, this function will automatically surface more priors without
// any caller updates.
func collectPriorRoundSummaries(item *workmodel.WorkItem, maxN int) []string {
	if item == nil || item.LastRound == nil {
		return nil
	}
	// Single-round history today. Walk the LastRound chain via ParentID
	// siblings only if a future change adds a RoundHistory slice; for
	// now we read the only available prior — LastRound.ArtifactSummary.
	// Returning a 1-element slice is still correct (1 prior is enough
	// to detect identical-replay stagnation across two consecutive rounds).
	if item.LastRound.ArtifactSummary == "" {
		return nil
	}
	if maxN <= 0 {
		maxN = 5
	}
	return []string{item.LastRound.ArtifactSummary}
}

func itemSafeID(item *workmodel.WorkItem) string {
	if item == nil {
		return ""
	}
	return item.ID
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
		content = textutil.StripMiniMaxStreamMarkers(result.Content)
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
	if result != nil && len(result.ResolutionClaims) > 0 {
		raw, err := json.Marshal(result.ResolutionClaims)
		if err == nil {
			art.Metadata["resolution_claims"] = string(raw)
		} else {
			slog.Warn("item_pipeline: marshal resolution_claims failed; degrade to no_claim",
				"session_id", sessionID, "work_item_id", item.ID, "err", err)
		}
	}
	if pl != nil {
		art.SourcePlanID = pl.ID
	}
	return art
}

// observationalAnswerSourceLabel is the artifact Metadata["source"] value
// emitted by the DM-20260706-011 fast-path. Used by dashboards / D6 learners
// to count trivial Q&A traffic without parsing the work item's directive.
const observationalAnswerSourceLabel = "observational_answer_fastpath"

// observationalAnswerLearnHint is the artifact Metadata["learn_hint"] value.
// Records the ObsFact ID + strength so Learn can build the round's
// provenance without re-walking the UncertaintyReport.
type observationalAnswerLearnHint struct {
	ObsFactID string  `json:"obs_fact_id"`
	Strength  float64 `json:"strength"`
	Statement string  `json:"statement"`
}

// maybeObservationalAnswer is the DM-20260706-011 fast-path implementation.
// Called after Observe completes and before the Plan phase opens. When
// pickHighStrengthBusinessFact returns a non-empty Statement (and the
// caller has already verified no ObsUncertainty exists), this function
// constructs a minimal Artifact + Verdict + Round, persists the round,
// runs Learn for reputation scoring, and returns the round so the caller
// can short-circuit the Plan + Execute + Verify phases.
//
// Returns (nil, err) only on infrastructure failures (tree mutation error).
// All "skip" outcomes return (nil, nil) so the caller can fall through
// to the legacy Plan+Execute path without losing the round.
//
// Emits a "fastpath" Jaeger span so trivial Q&A is observable in the trace
// tree as a child of the D7_MUPS_Pipeline root, alongside the legacy
// Plan/Execute/Verify sub-spans.
func (r *ItemPipelineRunner) maybeObservationalAnswer(
	ctx context.Context,
	sessionID string,
	item *workmodel.WorkItem,
	report orchtypes.UncertaintyReport,
	obsIDs []string,
	obsFactID string,
	statement string,
	started time.Time,
	roundNo int,
	trigger string,
) (*workmodel.WorkItemPipelineRound, error) {
	_, endFastPath := hardening.EmitMUPSFastPath(ctx, sessionID, item.ID, "observational_answer")
	defer endFastPath(nil)

	ended := time.Now()
	verdict := workmodel.Verdict{
		Kind:       types.VerdictPass,
		Confidence: 0.95, // observational answer is structurally high-confidence
		Reason:     "observational_answer_fastpath",
		SourceID:   "obs_fact:" + obsFactID,
	}
	// Use the original ObsFact strength as Confidence floor so reputation
	// scoring reflects the LLM's stated certainty, not a flat 0.95.
	if obs := findObservationByID(report, obsFactID); obs != nil && obs.Strength > 0 {
		verdict.Confidence = obs.Strength
	}

	hintBytes, _ := json.Marshal(observationalAnswerLearnHint{
		ObsFactID: obsFactID,
		Strength:  verdict.Confidence,
		Statement: statement,
	})
	art := &wavescheduler.Artifact{
		TaskID:     item.ID,
		SessionID:  sessionID,
		WorkerType: wavescheduler.WorkerWorkItem,
		Summary:    statement,
		ExitCode:   0,
		StartedAt:  started,
		EndedAt:    ended,
		Duration:   ended.Sub(started),
		Metadata: map[string]any{
			"source":         observationalAnswerSourceLabel,
			"obs_fact_id":    obsFactID,
			"learn_hint":     string(hintBytes),
			"stop_reason":    "observational_answer",
			"iterations":     0,
			"tool_calls":     0,
			"verdict_kind":   verdict.Kind.String(),
			"verdict_source": verdict.SourceID,
		},
	}

	// Build the round before persisting so the Learn call can include the
	// artifact in its provenance without a second walk over the tree.
	round := &workmodel.WorkItemPipelineRound{
		RoundNo:           roundNo,
		Trigger:           trigger,
		WorkItemID:        item.ID,
		SessionID:         sessionID,
		ObservationIDs:    append([]string(nil), obsIDs...),
		ArtifactID:        art.TaskID,
		ArtifactSummary:   statement,
		VerdictKind:       verdict.Kind,
		VerdictConfidence: verdict.Confidence,
		ExitReason:        "observational_answer",
		UncertaintyMean:   0,
	}

	// Run Learn for reputation scoring. BayesianUpdate(VerdictPass) bumps
	// Alpha → next round's EffectivePrior shifts toward the observer's
	// recommendation. Failures here are non-fatal: the user already has
	// their answer; reputation drift is recoverable next round.
	var learningClass types.LearningClass
	if r.Learner != nil {
		obsLookups := make([]learn.ObservationLookup, 0, len(obsIDs))
		for _, id := range obsIDs {
			obsLookups = append(obsLookups, observationRef(id))
		}
		assets, err := r.Learner.Learn(ctx, learn.LearnRequest{
			SessionID:    sessionID,
			Verdict:      verdict,
			Plan:         nil, // Plan node skipped — observational answer has no Plan
			Artifact:     art,
			Observations: obsLookups,
		})
		if err != nil {
			slog.Warn("item_pipeline: observational_answer Learn failed (non-fatal); round still persisted",
				"session_id", sessionID, "work_item_id", item.ID, "err", err)
		} else if len(assets) > 0 && assets[0] != nil {
			learningClass = assets[0].Class
		}
	}
	round.LearningClass = learningClass

	// Persist the round so the next round's Observe sees the artifact
	// history and SessionWorkItem retrieval is consistent. Phase = Idle so
	// downstream SpawnPolicy doesn't re-trigger the item.
	if err := r.Tasks.Tree().ApplyPipelineRound(sessionID, item.ID, round, workmodel.RoundPhaseIdle); err != nil {
		return nil, fmt.Errorf("item_pipeline: observational_answer ApplyPipelineRound: %w", err)
	}

	// Update the in-memory copy so callers reading item.LastRound see the
	// fast-path round without re-fetching from the tree.
	item.LastRound = round
	return round, nil
}

// findObservationByID locates a single observation by ID in the report.
// Returns nil if not found. Used by maybeObservationalAnswer to recover the
// ObsFact's strength for verdict Confidence.
func findObservationByID(report orchtypes.UncertaintyReport, id string) *orchtypes.Observation {
	for i := range report.Observations {
		if report.Observations[i].ID == id {
			return &report.Observations[i]
		}
	}
	return nil
}

// extractResolutionClaimsFromArtifact reads per-ObsID ResolutionClaim[]
// out of the Execute artifact.
//
// DM-20260704-006 S4 Phase 1.5 (D7-S16-A105-T01): buildArtifactFromWorkItemResult
// stores the typed claims under Metadata["resolution_claims"] (JSON-encoded).
// This function reverses that and is the only path that decodes claims into
// the WorkItemPipelineRound so Verify doesn't have to re-parse prose.
//
// obsIDs is the round's observation IDs from Observe. Retained on the
// signature for stability — Phase 1.5 doesn't consume it but future
// changes may want to cross-check claim ObsIDs against the round's
// allowed ObsID set.
//
// Failure modes:
//
//   - nil artifact or absent metadata key → return nil; Verify reads
//     "no_resolution_claim" for every strategy (Phase 1.5 safety net).
//   - malformed JSON → log + return nil (same degradation path).
//   - empty array → empty slice (no claims, all strategies unresolved).
func extractResolutionClaimsFromArtifact(art *wavescheduler.Artifact, obsIDs []string) []interfaces.ResolutionClaim {
	_ = obsIDs
	if art == nil || art.Metadata == nil {
		return nil
	}
	raw, ok := art.Metadata["resolution_claims"]
	if !ok {
		return nil
	}
	text, ok := raw.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return nil
	}
	var claims []interfaces.ResolutionClaim
	if err := json.Unmarshal([]byte(text), &claims); err != nil {
		slog.Warn("item_pipeline: malformed resolution_claims JSON; degrade to no_claim",
			"err", err, "payload_preview", truncateForArtifact(text, 80))
		return nil
	}
	return claims
}

// setRoundPhaseWithLog wraps r.Tasks.Tree().SetRoundPhase with the standard
// RH-D7-05 (DM-20260630-013) observability pattern: emit a WorktreeOp span
// that opens on entry and closes with the SetRoundPhase result, and surface
// any error via slog.Warn (the worktree's eventual consistency model means
// the next round's GetPipelineFocus will recover, so we don't fail the
// pipeline — but operators need to know it happened). The previous code
// silently dropped the error via `_, _ =`, hiding WorkTree lock contention
// and replay-skew bugs from Jaeger.
func (r *ItemPipelineRunner) setRoundPhaseWithLog(
	ctx context.Context,
	sessionID, itemID string,
	phase workmodel.RoundPhase,
) {
	end := hardening.EmitWorktreeOp(ctx, sessionID, "set_round_phase", itemID, string(phase))
	if err := r.Tasks.Tree().SetRoundPhase(sessionID, itemID, phase); err != nil {
		slog.Warn("item_pipeline: SetRoundPhase failed; worktree will self-heal on next round",
			"session_id", sessionID, "work_item_id", itemID,
			"phase", string(phase), "err", err)
		end(err)
		return
	}
	end(nil)
}

// enterMUPSPhase sets the locator phase, opens a D7_MUPS_Phase span (parent for
// D2/D3 work), then persists the round phase on the worktree. Locator must be
// updated before EmitWorktreeOp so Jaeger breadcrumbs match the active node.
func (r *ItemPipelineRunner) enterMUPSPhase(
	ctx context.Context,
	sessionID, itemID string,
	phase workmodel.RoundPhase,
) (context.Context, func(error)) {
	ctx = workmodel.WithLocatorPhase(ctx, string(phase))
	ctx, endPhase := hardening.EmitMUPSPhase(ctx, sessionID, itemID, string(phase))
	r.setRoundPhaseWithLog(ctx, sessionID, itemID, phase)
	return ctx, endPhase
}

func formatStrategicPlanReject(reject *StrategicPlanReject) string {
	if reject == nil {
		return ""
	}
	field := strings.TrimSpace(reject.Field)
	if field == "" {
		field = strings.TrimSpace(reject.Reason)
	}
	if field == "" {
		field = "budget"
	}
	return fmt.Sprintf("strategic_plan_rejected: field=%s requested=%d max_allowed=%d reason=%s",
		field, reject.Requested, reject.MaxAllowed, reject.Reason)
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

func formatFailureWithEvidence(
	stats workmodel.ChildOutcomeStats,
	deliverableStatus workmodel.DeliverableStatus,
	art *wavescheduler.Artifact,
	item *workmodel.WorkItem,
) bool {
	if deliverableStatus != workmodel.DeliverableStatusIncomplete {
		return false
	}
	toolCalls := 0
	if art != nil {
		if tc, ok := art.Metadata["tool_calls"].(int); ok {
			toolCalls = tc
		}
	}
	hasScope := item != nil && item.ScopeContract != nil && len(item.ScopeContract.InScope) > 0
	progress := workmodel.EvidenceProgress(workmodel.EvidenceInput{
		ToolCalls:  toolCalls,
		HasScopeIn: hasScope,
		ChildStats: stats,
	})
	return progress >= workmodel.EvidenceSufficientThreshold
}
