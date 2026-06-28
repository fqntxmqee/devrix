package sessionorchestrator

import (
	"strings"

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
