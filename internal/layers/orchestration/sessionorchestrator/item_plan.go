package sessionorchestrator

import (
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// DirectiveForGoalPlan returns the Execute directive for Goal WorkItems, including
// the scope contract template on the first pipeline round (D7-S16-A60-T02).
func DirectiveForGoalPlan(item *workmodel.WorkItem, baseDirective string) string {
	if item == nil || item.Kind != workmodel.WorkKindGoal {
		return baseDirective
	}
	if item.LastRound != nil {
		return baseDirective
	}
	if strings.Contains(baseDirective, "<scope_contract>") {
		return baseDirective
	}
	return baseDirective + workmodel.GoalScopeContractPlanHint
}

// planQuantizedKind selects the Planner MatchKind input for a WorkItem.
// loop_first ingress classifies most messages as IntentFast, which would
// collapse to CommitmentPlan (single step) and skip exploration spawn
// rules (R6 decompose). Goal WorkItems are multi-phase by design — force
// intent_orchestrate unless this round is rollup synthesis.
func planQuantizedKind(item *workmodel.WorkItem, report orchtypes.UncertaintyReport) string {
	if item == nil {
		return "intent_orchestrate"
	}
	if item.NeedsRollup {
		return "intent_command"
	}
	switch item.Kind {
	case workmodel.WorkKindGoal, workmodel.WorkKindExplore, workmodel.WorkKindPlan:
		return "intent_orchestrate"
	}
	if report.QuantizedIntent != nil {
		return quantizedKindFromIntent(report.QuantizedIntent.Kind)
	}
	return "intent_orchestrate"
}

// ApplyGoalScopeFromExecute parses Execute output and persists ScopeContract on Goal items.
func ApplyGoalScopeFromExecute(sessionID string, item *workmodel.WorkItem, executeContent string, tm *workmodel.TaskManager) {
	if tm == nil || item == nil || item.Kind != workmodel.WorkKindGoal {
		return
	}
	directive := itemDirective(item)
	sc := workmodel.ResolveGoalScopeContract(item, directive, executeContent)
	if sc == nil {
		return
	}
	_ = tm.SetScopeContract(sessionID, item.ID, sc)
	item.ScopeContract = sc
}
