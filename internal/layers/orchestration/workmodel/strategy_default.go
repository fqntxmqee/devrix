// Package workmodel — strategy_default.go
//
// DefaultStrategy registry + LookupStrategy helper.
// 1:1 binding PlanKind → Strategy, validated at init() time.
// LookupStrategy(planKind) returns the bound Strategy or protocolStrategy
// as the safe default (matches Phase 2 PR-B1 DefaultPlanner pattern).

package workmodel

import (
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// defaultStrategies maps PlanKind to Strategy. 1:1 binding, validated at
// init() time. RegisterStrategy is the extension point for tests or
// future PlanKinds (e.g., DelegationPlan).
var defaultStrategies = map[plan.PlanKind]Strategy{
	plan.CommitmentPlan:  commitmentStrategy{},
	plan.ProtocolPlan:    protocolStrategy{},
	plan.ScenarioPlan:    scenarioStrategy{},
	plan.ExplorationPlan: explorationStrategy{},
}

// LookupStrategy returns the Strategy bound to the given PlanKind.
// If planKind is unknown (e.g., KindUnset or future PlanKind), returns
// protocolStrategy as the safe default — matches Phase 2 PR-B1
// DefaultPlanner fallback semantics (no behavior change for unknown kinds).
func LookupStrategy(planKind plan.PlanKind) Strategy {
	if s, ok := defaultStrategies[planKind]; ok {
		return s
	}
	return protocolStrategy{}
}

// RegisterStrategy binds a custom Strategy to a PlanKind. Extension point
// for tests and future PlanKinds. Idempotent: re-registering overwrites.
func RegisterStrategy(planKind plan.PlanKind, s Strategy) {
	defaultStrategies[planKind] = s
}

func init() {
	// Validate 1:1 binding at init time. We expect exactly 4 named PlanKinds.
	if len(defaultStrategies) != 4 {
		panic("defaultStrategies: expected exactly 4 PlanKind bindings, got " +
			itoa(len(defaultStrategies)))
	}
}

// itoa is a tiny helper to avoid pulling in strconv for a single use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
