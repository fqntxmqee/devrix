// Package workmodel — strategy_scenario.go
//
// scenarioStrategy: read-only probe scenario plan.
// M3 行为增量:
//   - VerdictFail + ScenarioPlan → SpawnNone (read-only, no retry, no decompose)
//
// Other PlanKinds: SpawnOverride returns (SpawnNone, false) → fall through to default.

package workmodel

import (
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// scenarioStrategy is the per-PlanKind Strategy for ScenarioPlan.
type scenarioStrategy struct{}

// RouteChannel returns "scenario_channel" for ScenarioPlan, empty otherwise.
func (scenarioStrategy) RouteChannel(planKind plan.PlanKind) string {
	if planKind == plan.ScenarioPlan {
		return "scenario_channel"
	}
	return ""
}

// SpawnOverride enforces read-only behavior for scenario plans.
// Returns ok=false for non-ScenarioPlan or for verdicts that should
// fall through to the default 5-case logic.
func (scenarioStrategy) SpawnOverride(round *WorkItemPipelineRound) (SpawnPolicy, bool) {
	if round == nil || round.PlanKind != plan.ScenarioPlan {
		return SpawnNone, false
	}
	switch round.VerdictKind {
	case types.VerdictFail:
		// M3 行为增量: read-only probes don't retry on Fail.
		// Matches Phase 3 PR-C2 ScenarioChannel "并行探测 + 多数派投票" semantics
		// (probe is a one-shot, not a retry loop).
		return SpawnNone, true
	default:
		return SpawnNone, false
	}
}

// ShouldDecompose reports whether scenario plans decompose.
// Scenario plans are read-only probes; no decomposition.
func (scenarioStrategy) ShouldDecompose(planKind plan.PlanKind) bool {
	_ = planKind
	return false
}

// IsReadOnly reports whether scenario plans have side effects.
// Scenario plans are read-only probes; no side effects.
func (scenarioStrategy) IsReadOnly(planKind plan.PlanKind) bool {
	_ = planKind
	return false
}
