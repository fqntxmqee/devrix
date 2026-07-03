package workmodel

import (
	"time"

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
	StructuredDeliverable *DeliverablePayload `json:"structured_deliverable,omitempty"`
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
