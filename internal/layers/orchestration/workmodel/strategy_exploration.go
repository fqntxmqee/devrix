// Package workmodel — strategy_exploration.go
//
// explorationStrategy: parallel experiments exploration plan.
// M3 行为增量:
//   - VerdictPass + ExplorationPlan → SpawnDecompose (parallel explore continues)
//
// Other PlanKinds: SpawnOverride returns (SpawnNone, false) → fall through to default.

package workmodel

import (
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// explorationStrategy is the per-PlanKind Strategy for ExplorationPlan.
type explorationStrategy struct{}

// RouteChannel returns "exploration_channel" for ExplorationPlan, empty otherwise.
func (explorationStrategy) RouteChannel(planKind plan.PlanKind) string {
	if planKind == plan.ExplorationPlan {
		return "exploration_channel"
	}
	return ""
}

// SpawnOverride enforces parallel-experiment behavior for exploration plans.
// Returns ok=false for non-ExplorationPlan or for verdicts that should
// fall through to the default 5-case logic.
func (explorationStrategy) SpawnOverride(round *WorkItemPipelineRound) (SpawnPolicy, bool) {
	if round == nil || round.PlanKind != plan.ExplorationPlan {
		return SpawnNone, false
	}
	switch round.VerdictKind {
	case types.VerdictPass:
		// M3 行为增量: exploration plans continue to decompose even on Pass
		// (parallel probe + majority voting). Matches Phase 3 PR-C2
		// ExplorationChannel "多 agent + 优先级排序 + PersistScope" semantics.
		return SpawnDecompose, true
	default:
		return SpawnNone, false
	}
}

// ShouldDecompose reports whether exploration plans decompose.
// Exploration plans are parallel experiments; decomposition allowed.
func (explorationStrategy) ShouldDecompose(planKind plan.PlanKind) bool {
	_ = planKind
	return true
}

// IsReadOnly reports whether exploration plans have side effects.
// Exploration plans have side effects (compare implementations, A/B test).
func (explorationStrategy) IsReadOnly(planKind plan.PlanKind) bool {
	_ = planKind
	return true
}
