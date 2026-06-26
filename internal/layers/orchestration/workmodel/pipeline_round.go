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

// WorkItemPipelineRound is the typed signal bundle for one WorkItem pipeline
// iteration. Parent spawn decisions MUST read this struct (goal G2).
//
// ArtifactSummary carries the execute-phase artifact's Summary text (the
// LLM response for chat-style work_item_execute tool calls). RunSessionTurnLoop
// emits it as a "text" EngineEvent so the user sees the answer; without this
// field the round's ArtifactID would be opaque to the gateway and the LLM
// response would never reach feishu. Populated by ItemPipelineRunner.Run.
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
	SpawnPolicy       SpawnPolicy       `json:"spawn_policy,omitempty"`
	ContextBubbleKind ContextBubbleKind `json:"context_bubble_kind,omitempty"`
	ChildSpecs        []ChildSpec       `json:"child_specs,omitempty"`
	SpawnRationale    string            `json:"spawn_rationale,omitempty"`
	StartedAt       time.Time         `json:"started_at,omitempty"`
	CompletedAt     time.Time         `json:"completed_at,omitempty"`
}

// TreeEvalContext supplies deterministic tree constraints to SpawnPolicyEvaluator.
type TreeEvalContext struct {
	Depth                   int
	MaxDepth                int
	RunningChildren         int
	DailyLimitExceeded      bool
	Threshold               float64
	UserID                  string
	IndeterminateRetries    int
	MaxIndeterminateRetries int
}

// DefaultTreeEvalContext returns evaluator defaults aligned with WorkTree limits.
func DefaultTreeEvalContext(sessionID, workItemID, userID string, tm *TaskManager) TreeEvalContext {
	ctx := TreeEvalContext{
		MaxDepth:                DefaultMaxDecomposeDepth,
		Threshold:               EffectiveDecomposeThreshold(tm, userID),
		MaxIndeterminateRetries: DefaultMaxIndeterminateRetries,
		UserID:                  userID,
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
