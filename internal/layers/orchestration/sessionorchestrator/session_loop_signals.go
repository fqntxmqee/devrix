package sessionorchestrator

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/escape"
	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// deliverableIncompleteEscapeCriterion returns the escape failure hash when
// a round still owes a deliverable on an inline/none stagnation path. Decompose
// / await / parallel explore are forward-progress spawns and must not share
// the same hash (DM-20260703-001 / review session false ForceExit).
func deliverableIncompleteEscapeCriterion(round *workmodel.WorkItemPipelineRound) string {
	if round == nil {
		return ""
	}
	if round.DeliverableStatus != workmodel.DeliverableStatusIncomplete {
		return ""
	}
	key := deliverableEscapeKey(round)
	if key == "" {
		return ""
	}
	switch round.SpawnPolicy {
	case workmodel.SpawnDecompose, workmodel.SpawnAwait, workmodel.SpawnParallelExplore:
		return ""
	}
	return "deliverable_incomplete:" + key
}

func deliverableEscapeKey(round *workmodel.WorkItemPipelineRound) string {
	if round == nil {
		return ""
	}
	if round.DeliverableContract.ContractApplicable() {
		return round.DeliverableContract.CacheKey()
	}
	if round.DeliverableSchema != "" && round.DeliverableSchema != workmodel.DeliverableSchemaNotApplicable {
		return string(round.DeliverableSchema)
	}
	return ""
}

// SessionLoopExitKind classifies why the session turn loop stopped.
type SessionLoopExitKind string

const (
	SessionLoopExitContinue SessionLoopExitKind = ""
	// SessionLoopExitNatural — no open work; normal completion path.
	SessionLoopExitNatural SessionLoopExitKind = "natural"
	// SessionLoopExitAnomaly — verify/LLM anomaly signals indicate stagnation.
	SessionLoopExitAnomaly SessionLoopExitKind = "anomaly"
	// SessionLoopExitEscalate — SpawnEscalateHuman on the last round.
	SessionLoopExitEscalate SessionLoopExitKind = "escalate_human"
	// SessionLoopExitEscape — EscapeEngine force-exit / abort.
	SessionLoopExitEscape SessionLoopExitKind = "escape"
)

// SessionLoopExitDecision is returned after each pipeline round (or when
// focus is nil) to decide whether RunSessionTurnLoop should terminate.
// RH-D7-05 (2026-07-03): replaces the fixed defaultSessionTurnLoopMax=16
// backstop — loop exit is driven by anomaly / spawn / escape signals, not
// an iteration budget.
type SessionLoopExitDecision struct {
	Kind   SessionLoopExitKind
	Reason string
}

func (d SessionLoopExitDecision) ShouldExit() bool {
	return d.Kind != SessionLoopExitContinue
}

func buildEscapeLoopContextFromRound(sessionID string, round *workmodel.WorkItemPipelineRound) escape.LoopContext {
	kind := escape.PlanKind(0)
	failure := ""
	if round != nil {
		kind = escape.PlanKind(round.PlanKind)
		if fc := deliverableIncompleteEscapeCriterion(round); fc != "" {
			// Stable mode hash for repeated inline retries on the same
			// deliverable contract — lets LoopDepthTracker force-exit
			// without a fixed session iteration budget.
			failure = fc
		} else {
			failure = strings.TrimSpace(round.ExitReason)
			if failure == "" {
				failure = string(round.SpawnPolicy)
			}
		}
	}
	return escape.LoopContext{
		SessionID:        sessionID,
		PlanKind:         kind,
		FailureCriterion: failure,
	}
}

// evaluateSessionLoopExitAfterRound inspects the latest MUPS round plus
// escape decision to determine whether the session loop should stop.
func evaluateSessionLoopExitAfterRound(
	ctx context.Context,
	sessionID string,
	tm *workmodel.TaskManager,
	round *workmodel.WorkItemPipelineRound,
	esc escape.EscapeDecision,
) SessionLoopExitDecision {
	if round != nil && round.SpawnPolicy == workmodel.SpawnEscalateHuman {
		return SessionLoopExitDecision{
			Kind:   SessionLoopExitEscalate,
			Reason: strings.TrimSpace(round.SpawnRationale),
		}
	}
	switch esc.Action {
	case escape.EscapeForceExit, escape.EscapeAbortWithAudit:
		return SessionLoopExitDecision{
			Kind:   SessionLoopExitEscape,
			Reason: esc.Reason,
		}
	case escape.EscapePendingHuman, escape.EscalateToHuman, escape.EscalateToRule:
		return SessionLoopExitDecision{
			Kind:   SessionLoopExitEscalate,
			Reason: esc.Reason,
		}
	}
	if round == nil {
		return SessionLoopExitDecision{}
	}

	// Deliverable contract unsatisfied with no remaining spawn path → anomaly exit.
	if deliverableStagnation(round, tm, sessionID) {
		anomaly := verify.DetectEmptyConclusion(ctx, sessionID, round.ArtifactSummary)
		reason := "deliverable_stagnation"
		if anomaly.Triggered {
			reason = string(anomaly.Kind)
		}
		return SessionLoopExitDecision{Kind: SessionLoopExitAnomaly, Reason: reason}
	}

	// Planning/recap artifact without deliverable — only terminate when spawn
	// policy offers no continuation path (SpawnNone stagnation).
	if round.DeliverableStatus == workmodel.DeliverableStatusIncomplete &&
		(round.SpawnPolicy == workmodel.SpawnNone || round.SpawnPolicy == "") {
		if res := verify.DetectEmptyConclusion(ctx, sessionID, round.ArtifactSummary); res.Triggered {
			return SessionLoopExitDecision{
				Kind:   SessionLoopExitAnomaly,
				Reason: string(res.Kind),
			}
		}
		if isExplorationPlanningText(round.ArtifactSummary) {
			return SessionLoopExitDecision{
				Kind:   SessionLoopExitAnomaly,
				Reason: "exploration_planning_text",
			}
		}
	}

	return SessionLoopExitDecision{}
}

// deliverableStagnation is true when the round still owes a deliverable,
// spawn policy is SpawnNone (no inline/decompose/await left), and the work
// item cannot make forward progress in-tree.
func deliverableStagnation(round *workmodel.WorkItemPipelineRound, tm *workmodel.TaskManager, sessionID string) bool {
	if round == nil || tm == nil {
		return false
	}
	if !round.DeliverableContract.ContractApplicable() &&
		(round.DeliverableSchema == "" || round.DeliverableSchema == workmodel.DeliverableSchemaNotApplicable) {
		return false
	}
	if round.DeliverableStatus == workmodel.DeliverableStatusComplete ||
		round.DeliverableStatus == workmodel.DeliverableStatusNotApplicable {
		return false
	}
	switch round.SpawnPolicy {
	case workmodel.SpawnInline, workmodel.SpawnDecompose, workmodel.SpawnAwait, workmodel.SpawnParallelExplore:
		return false
	case workmodel.SpawnEscalateHuman:
		return false // handled above
	}
	// SpawnNone + incomplete deliverable: check if focus will pick this item up again.
	item, ok := tm.GetWorkItem(sessionID, round.WorkItemID)
	if !ok || item == nil {
		return true
	}
	if item.LastRound != nil && item.LastRound.SpawnPolicy == workmodel.SpawnInline {
		return false
	}
	return true
}

func isExplorationPlanningText(summary string) bool {
	return workmodel.DetectPlanningMeta(summary)
}

// sessionNoForwardProgress reports whether every non-terminal WorkItem is
// stuck on inline retry (or spawn none) with an unsatisfied deliverable
// contract — no item still has decompose/await/escalate ahead. This is the
// session-level structural stagnation signal that replaces the old fixed
// iteration budget (RH-D7-05).
func sessionNoForwardProgress(sessionID string, tm *workmodel.TaskManager) bool {
	if tm == nil {
		return false
	}
	sawOpen := false
	for _, item := range tm.Tree().List(sessionID) {
		if item == nil || item.Ephemeral || workmodel.IsTerminalStatus(item.Status) {
			continue
		}
		sawOpen = true
		if item.RoundPhase == workmodel.RoundPhaseAwaitChild {
			if subtreeDeliverableInlineStuck(tm, sessionID, item.ID) {
				continue
			}
			return false
		}
		if item.LastRound == nil {
			return false
		}
		switch item.LastRound.SpawnPolicy {
		case workmodel.SpawnDecompose, workmodel.SpawnAwait, workmodel.SpawnParallelExplore, workmodel.SpawnEscalateHuman:
			return false
		case workmodel.SpawnInline, workmodel.SpawnNone:
			if !workmodel.DeliverableContinuationRequired(item.LastRound) {
				return false
			}
			if item.LastRound.SpawnPolicy == workmodel.SpawnInline {
				if workmodel.IsDeliverableInlineBudgetExhausted(item) {
					continue
				}
				return false
			}
		default:
			return false
		}
	}
	return sawOpen
}

// subtreeDeliverableInlineStuck walks descendants: all open leaves owe deliverable
// continuation via inline/none with no decompose/await forward path (CC-4).
func subtreeDeliverableInlineStuck(tm *workmodel.TaskManager, sessionID, parentID string) bool {
	if tm == nil {
		return false
	}
	sawOpen := false
	for _, c := range tm.Tree().ListChildren(sessionID, parentID) {
		if c == nil || c.Ephemeral || c.Kind == workmodel.WorkKindChecklist {
			continue
		}
		if workmodel.IsTerminalStatus(c.Status) {
			continue
		}
		sawOpen = true
		if c.RoundPhase == workmodel.RoundPhaseAwaitChild {
			if !subtreeDeliverableInlineStuck(tm, sessionID, c.ID) {
				return false
			}
			continue
		}
		if c.LastRound == nil {
			return false
		}
		switch c.LastRound.SpawnPolicy {
		case workmodel.SpawnDecompose, workmodel.SpawnAwait, workmodel.SpawnParallelExplore, workmodel.SpawnEscalateHuman:
			return false
		case workmodel.SpawnInline, workmodel.SpawnNone:
			if !workmodel.DeliverableContinuationRequired(c.LastRound) {
				return false
			}
		default:
			return false
		}
	}
	return sawOpen
}

func childrenDeliverableInlineStuck(tm *workmodel.TaskManager, sessionID, parentID string) bool {
	if tm == nil {
		return false
	}
	sawChild := false
	for _, c := range tm.Tree().ListChildren(sessionID, parentID) {
		if c == nil || c.Ephemeral || c.Kind == workmodel.WorkKindChecklist {
			continue
		}
		if workmodel.IsTerminalStatus(c.Status) {
			continue
		}
		sawChild = true
		if c.LastRound == nil || c.LastRound.SpawnPolicy != workmodel.SpawnInline {
			return false
		}
		if !workmodel.DeliverableContinuationRequired(c.LastRound) {
			return false
		}
	}
	return sawChild
}
