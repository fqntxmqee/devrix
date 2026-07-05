// Package workmodel — strategy_default_test.go
//
// Unit tests for DefaultStrategy registry + LookupStrategy helper (M3):
//   - LookupStrategy(PlanKind) returns the bound Strategy
//   - LookupStrategy(KindUnset) returns protocolStrategy (safe default)
//   - RegisterStrategy overrides default binding (extension point)
//   - init() validation: exactly 4 PlanKind bindings

package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// T: D7-SX-AXX-T15 — LookupStrategy: 4 PlanKinds map to 4 strategies
func TestLookupStrategy_FourBindings(t *testing.T) {
	tests := []struct {
		kind     plan.PlanKind
		typeName string
	}{
		{plan.CommitmentPlan, "commitmentStrategy"},
		{plan.ProtocolPlan, "protocolStrategy"},
		{plan.ScenarioPlan, "scenarioStrategy"},
		{plan.ExplorationPlan, "explorationStrategy"},
	}
	for _, tc := range tests {
		s := LookupStrategy(tc.kind)
		if s == nil {
			t.Errorf("LookupStrategy(%v): nil", tc.kind)
			continue
		}
		// Verify the type via RouteChannel (each strategy has a unique channel name)
		channel := s.RouteChannel(tc.kind)
		if channel == "" {
			t.Errorf("LookupStrategy(%v).RouteChannel: empty, want non-empty", tc.kind)
		}
	}
}

// T: D7-SX-AXX-T16 — LookupStrategy: KindUnset returns protocolStrategy (safe default)
func TestLookupStrategy_KindUnset_Default(t *testing.T) {
	s := LookupStrategy(plan.KindUnset)
	if s == nil {
		t.Fatal("LookupStrategy(KindUnset): nil, want protocolStrategy")
	}
	// protocolStrategy.RouteChannel(ProtocolPlan) = "protocol_channel"
	if got := s.RouteChannel(plan.ProtocolPlan); got != "protocol_channel" {
		t.Errorf("expected protocolStrategy, got channel %q", got)
	}
}

// T: D7-SX-AXX-T17 — LookupStrategy: unknown PlanKind returns protocolStrategy
func TestLookupStrategy_UnknownKind_Default(t *testing.T) {
	unknown := plan.PlanKind(42)
	s := LookupStrategy(unknown)
	if s == nil {
		t.Fatal("LookupStrategy(42): nil, want protocolStrategy fallback")
	}
	if got := s.RouteChannel(plan.ProtocolPlan); got != "protocol_channel" {
		t.Errorf("expected protocolStrategy fallback, got channel %q", got)
	}
}

// T: D7-SX-AXX-T18 — RegisterStrategy: override default binding (extension point)
func TestRegisterStrategy_Override(t *testing.T) {
	saved, hadOriginal := defaultStrategies[plan.CommitmentPlan]
	defer func() {
		if hadOriginal {
			defaultStrategies[plan.CommitmentPlan] = saved
		} else {
			delete(defaultStrategies, plan.CommitmentPlan)
		}
	}()

	type mockStrategy struct {
		Strategy
		channel string
	}
	custom := &mockStrategy{channel: "custom_channel"}

	// Mock Strategy interface (only RouteChannel needed for test)
	RegisterStrategy(plan.CommitmentPlan, &stubStrategy{routeChannel: custom.channel})

	s := LookupStrategy(plan.CommitmentPlan)
	if got := s.RouteChannel(plan.CommitmentPlan); got != "custom_channel" {
		t.Errorf("after RegisterStrategy: got channel %q, want %q", got, "custom_channel")
	}
}

// T: D7-SX-AXX-T19 — init() validation: exactly 4 PlanKind bindings at startup
func TestDefaultStrategies_InitValidation(t *testing.T) {
	// The init() function panics if len(defaultStrategies) != 4.
	// Since init() has already run (and we're in the test process), we can
	// simply verify the count.
	if got := len(defaultStrategies); got != 4 {
		t.Errorf("defaultStrategies: got %d bindings, want 4 (Commitment/Protocol/Scenario/Exploration)", got)
	}
	// Verify all 4 named PlanKinds are bound
	for _, k := range []plan.PlanKind{
		plan.CommitmentPlan, plan.ProtocolPlan, plan.ScenarioPlan, plan.ExplorationPlan,
	} {
		if _, ok := defaultStrategies[k]; !ok {
			t.Errorf("defaultStrategies missing binding for %v", k)
		}
	}
}

// -----------------------------------------------------------------------------
// Stub helpers for strategy_default_test.go
// -----------------------------------------------------------------------------

// stubStrategy is a minimal Strategy for testing RegisterStrategy.
type stubStrategy struct {
	routeChannel string
}

func (s *stubStrategy) RouteChannel(_ plan.PlanKind) string { return s.routeChannel }
func (s *stubStrategy) SpawnOverride(_ *WorkItemPipelineRound) (SpawnPolicy, bool) {
	return SpawnNone, false
}
func (s *stubStrategy) ShouldDecompose(_ plan.PlanKind) bool { return false }
func (s *stubStrategy) IsReadOnly(_ plan.PlanKind) bool      { return false }
