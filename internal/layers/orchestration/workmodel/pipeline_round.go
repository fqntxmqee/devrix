package workmodel

import (
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// RoundPhase tracks where a WorkItem sits inside the per-item MUPS pipeline.
// Orthogonal to TaskStatus (lifecycle).
type RoundPhase string

const (
	RoundPhaseIdle       RoundPhase = "idle"
	RoundPhaseObserve    RoundPhase = "observe"
	RoundPhasePlan       RoundPhase = "plan"
	RoundPhaseExecute    RoundPhase = "execute"
	RoundPhaseVerify     RoundPhase = "verify"
	RoundPhaseLearn      RoundPhase = "learn"
	RoundPhaseDecide     RoundPhase = "decide"
	RoundPhaseAwaitChild RoundPhase = "await_child"
)

// SpawnPolicy is the rule-only output of SpawnPolicyEvaluator (invariant I5).
type SpawnPolicy string

const (
	SpawnNone            SpawnPolicy = "none"
	SpawnDecompose       SpawnPolicy = "decompose"
	SpawnParallelExplore SpawnPolicy = "parallel_explore"
	SpawnAwait           SpawnPolicy = "await"
	SpawnInline          SpawnPolicy = "inline"
	SpawnEscalateHuman   SpawnPolicy = "escalate_human"
	// SpawnUserGate (DM-20260704-006 RC-4b) is emitted when Verify's
	// ResolutionReport has UnresolvedObs with Strength >= DefaultUnresolvedStrengthThreshold
	// (0.85) and no SubWorktree is available. The SpawnApply step creates
	// a verify child with tool_filter=["ask_user_question"] so the LLM
	// cannot bypass the gate. Without this hook, a Pass+high-strength
	// unresolved ObsID pair would SpawnNone and silently strand the
	// user-facing question.
	SpawnUserGate SpawnPolicy = "user_gate"
)

// DefaultMaxIndeterminateRetries matches MVE MaxRetries (OQ-4).
const DefaultMaxIndeterminateRetries = 3

// DefaultMaxRollupRetries caps how many consecutive non-Pass rollup rounds
// a WorkItem may run before SpawnPolicyEvaluator forces a SpawnEscalateHuman.
//
// RH-MUPS-03 (DM-20260701-001): before this constant the rollup branch in
// SpawnPolicyEvaluator returned SpawnInline unconditionally on any non-Pass
// verdict. After MaxRollupRetries failed inline rounds the session loop's
// own max=16 became the only termination backstop, and it would exit
// silently with the parent still InProgress — the loop's "no more focus"
// break hid the unresolved rollup. With this constant the rollup path has
// a real termination guarantee: escalate to human review.
const DefaultMaxRollupRetries = 3

// DefaultMaxInlineRetriesAtMaxDepth caps consecutive inline retries on a leaf
// at max decompose depth before SpawnEscalateHuman (DM-20260703-001 CC-1.2).
const DefaultMaxInlineRetriesAtMaxDepth = 3

// TerminalReasonInlineRetriesExhaustedAtMaxDepth is recorded when max-depth
// inline budget is exhausted (L5-D7-CC-02).
const TerminalReasonInlineRetriesExhaustedAtMaxDepth = "inline_retries_exhausted_at_max_depth"

// WorkItemPipelineRound is the typed signal bundle for one WorkItem pipeline
// iteration. Parent spawn decisions MUST read this struct (goal G2).
//
// ArtifactSummary carries the execute-phase artifact's Summary text (the
// WorkItemExecutor LLM final answer). RunSessionTurnLoop emits it as a
// "text" EngineEvent so the user sees the answer; without this field the
// round's ArtifactID would be opaque to the gateway. Populated by
// ItemPipelineRunner.Run.
type WorkItemPipelineRound struct {
	RoundNo         int               `json:"round_no,omitempty"`
	// Trigger labels why this MUPS round ran (initial|inline|rollup|…).
	Trigger         string            `json:"trigger,omitempty"`
	// LoopTick is the RunSessionTurnLoop for{} iteration when this round started.
	LoopTick        int               `json:"loop_tick,omitempty"`
	WorkItemID      string            `json:"work_item_id,omitempty"`
	SessionID       string            `json:"session_id,omitempty"`
	ObservationIDs  []string          `json:"observation_ids,omitempty"`
	PlanID          string            `json:"plan_id,omitempty"`
	PlanKind        plan.PlanKind     `json:"plan_kind,omitempty"`
	ArtifactID      string            `json:"artifact_id,omitempty"`
	ArtifactSummary string            `json:"artifact_summary,omitempty"`
	VerdictID       string            `json:"verdict_id,omitempty"`
	VerdictKind     types.VerdictKind `json:"verdict_kind,omitempty"`
	ExitReason      string            `json:"exit_reason,omitempty"`
	LearningClass   types.LearningClass `json:"learning_class,omitempty"`
	UncertaintyMean float64           `json:"uncertainty_mean,omitempty"`
	VerdictConfidence float64         `json:"verdict_confidence,omitempty"`
	IndeterminateReason string        `json:"indeterminate_reason,omitempty"`
	IndeterminateRetries int          `json:"indeterminate_retries,omitempty"`
	// RollupRetries counts consecutive non-Pass rollup rounds on this
	// WorkItem. SpawnPolicyEvaluator reads the previous round's counter via
	// TreeEvalContext.RollupRetries and forces SpawnEscalateHuman once the
	// count reaches MaxRollupRetries. RH-MUPS-03 (DM-20260701-001).
	RollupRetries       int             `json:"rollup_retries,omitempty"`
	SpawnPolicy       SpawnPolicy       `json:"spawn_policy,omitempty"`
	ContextBubbleKind ContextBubbleKind `json:"context_bubble_kind,omitempty"`
	ChildSpecs        []ChildSpec       `json:"child_specs,omitempty"`
	SpawnRationale    string            `json:"spawn_rationale,omitempty"`
	DeliverableSchema DeliverableSchema `json:"deliverable_schema,omitempty"`
	DeliverableContract DeliverableContract `json:"deliverable_contract,omitempty"`
	DeliverableStatus DeliverableStatus `json:"deliverable_status,omitempty"`
	DeliverableReason string            `json:"deliverable_reason,omitempty"`
	// ExecuteToolCalls from artifact metadata (CC-U1 evidence progress).
	ExecuteToolCalls int `json:"execute_tool_calls,omitempty"`
	// ScopeInPresent mirrors ScopeContract.InScope non-empty at round end.
	ScopeInPresent bool `json:"scope_in_present,omitempty"`
	// RollupSynthRequested set by EvaluateSpawnPolicy when CC-U3 rollup synth applies.
	RollupSynthRequested bool `json:"rollup_synth_requested,omitempty"`
	// ObserveParseReject carries compact JSON for the next Observe user frame (DM-20260705-002).
	ObserveParseReject string `json:"observe_parse_reject,omitempty"`
	// SemanticRetryHint (DM-20260706-006) is set when the LLM semantic
	// verifier returns VerdictFail with decision="retry". The hint is
	// passed to the next round's ExecuteWorkItem via PriorVerifyReason
	// so the LLM can self-correct ("the previous answer was template-
	// mimicking; address the user's original question with concrete
	// content"). Empty when the verifier agrees the answer passes
	// or when decision="stop" (SpawnNone terminates the loop).
	SemanticRetryHint string `json:"semantic_retry_hint,omitempty"`
	// PlanParseReject carries compact JSON for the next Plan user frame (DM-20260705-002).
	PlanParseReject string `json:"plan_parse_reject,omitempty"`
	StructuredDeliverable *DeliverablePayload `json:"structured_deliverable,omitempty"`

	// ResolutionClaims (DM-20260704-006) — per-ObsID answers extracted
	// from the Execute artifact. Populated by the Execute layer; Verify
	// reads this slice to compute CoverageRatio. Optional for legacy
	// rounds that pre-date the ResolutionContract.
	ResolutionClaims []interfaces.ResolutionClaim `json:"resolution_claims,omitempty"`

	// ResolutionReport (DM-20260704-006) — the cross-round handoff
	// produced by Verify. Decide reads AnySubWorktreePending +
	// MaxUnresolvedStrength to pick SpawnDecompose / SpawnUserGate /
	// SpawnInline. Nil when the round pre-dates the contract or when
	// Verify chose not to compute one (legacy Verifier fall-back).
	ResolutionReport *interfaces.ResolutionReport `json:"resolution_report,omitempty"`

	StartedAt       time.Time         `json:"started_at,omitempty"`
	CompletedAt     time.Time         `json:"completed_at,omitempty"`
}

// TreeEvalContext supplies deterministic tree constraints to SpawnPolicyEvaluator.
type TreeEvalContext struct {
	Depth                   int
	MaxDepth                int
	RunningChildren         int
	ChildTotal              int
	CanDecompose            bool
	RollupRound             bool
	DailyLimitExceeded      bool
	Threshold               float64
	UserID                  string
	IndeterminateRetries    int
	MaxIndeterminateRetries int
	// RollupRetries mirrors item.LastRound.RollupRetries so the policy
	// evaluator can compare against MaxRollupRetries without touching the
	// WorkItem. Zero when the item has no prior round. RH-MUPS-03.
	RollupRetries    int
	MaxRollupRetries int
	// InlineRetriesAtMaxDepth mirrors WorkItem.InlineRetriesAtMaxDepth for R1.
	InlineRetriesAtMaxDepth    int
	MaxInlineRetriesAtMaxDepth int
}

// DefaultTreeEvalContext returns evaluator defaults aligned with WorkTree limits.
func DefaultTreeEvalContext(sessionID, workItemID, userID string, tm *TaskManager) TreeEvalContext {
	ctx := TreeEvalContext{
		MaxDepth:                DefaultMaxDecomposeDepth,
		Threshold:               EffectiveDecomposeThreshold(tm, userID),
		MaxIndeterminateRetries: DefaultMaxIndeterminateRetries,
		MaxRollupRetries:           DefaultMaxRollupRetries,
		MaxInlineRetriesAtMaxDepth:   DefaultMaxInlineRetriesAtMaxDepth,
		UserID:                     userID,
	}
	if tm == nil || workItemID == "" {
		return ctx
	}
	ctx.Depth = tm.Tree().Depth(sessionID, workItemID)
	if tm.Tree().MaxDecomposeDepth() > 0 {
		ctx.MaxDepth = tm.Tree().MaxDecomposeDepth()
	}
	stats := childOutcomeStats(tm, sessionID, workItemID)
	ctx.RunningChildren = stats.Running
	ctx.ChildTotal = stats.Total
	if item, ok := tm.GetWorkItem(sessionID, workItemID); ok && item != nil {
		ctx.CanDecompose = CanDecompose(item.Kind)
		ctx.RollupRound = item.NeedsRollup
		if item.LastRound != nil {
			ctx.RollupRetries = item.LastRound.RollupRetries
		}
		ctx.InlineRetriesAtMaxDepth = item.InlineRetriesAtMaxDepth
	}
	if ctx.MaxInlineRetriesAtMaxDepth <= 0 {
		ctx.MaxInlineRetriesAtMaxDepth = DefaultMaxInlineRetriesAtMaxDepth
	}
	return ctx
}

// IsExploratoryPlanKind reports scenario/exploration plans (design §4 R4–R6).
func IsExploratoryPlanKind(k plan.PlanKind) bool {
	return k == plan.ScenarioPlan || k == plan.ExplorationPlan
}

// IsCommitmentPlanKind reports strict execution plans (design §4 R3).
func IsCommitmentPlanKind(k plan.PlanKind) bool {
	return k == plan.CommitmentPlan || k == plan.ProtocolPlan
}

// ValidateSpawnDecompose enforces invariant I4 before DecomposeChildren.
func ValidateSpawnDecompose(round *WorkItemPipelineRound) error {
	if round == nil {
		return errSpawnRoundRequired
	}
	if round.SpawnPolicy != SpawnDecompose {
		return errSpawnPolicyNotDecompose
	}
	if round.WorkItemID == "" || round.PlanID == "" || round.VerdictID == "" {
		return errSpawnRoundIncomplete
	}
	if len(round.ObservationIDs) == 0 {
		return errSpawnRoundIncomplete
	}
	return nil
}

// CapChildSpecs limits proposer output to DefaultMaxChildren (OQ-2).
func CapChildSpecs(specs []ChildSpec) []ChildSpec {
	if len(specs) <= DefaultMaxChildren {
		return specs
	}
	return specs[:DefaultMaxChildren]
}

// ChildKindForHypothesis maps decompose proposals to WorkKind (OQ-3).
func ChildKindForHypothesis(exploratory bool) WorkKind {
	if exploratory {
		return WorkKindExplore
	}
	return WorkKindImplement
}
