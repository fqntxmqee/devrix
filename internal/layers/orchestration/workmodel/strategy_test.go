// Package workmodel — strategy_test.go
//
// Unit tests for Strategy.SpawnOverride (M3, DM-20260705-008):
//   - 4 PlanKind × 5 VerdictKind = 20 cases (4 M3 行为增量, 16 兜底)
//   - 4 兜底 tests (CC-1.4 deliverable continuation precedence,
//     non-PlanKind calls, nil round, safe default for unknown PlanKind)
//   - RouteChannel/ShouldDecompose/IsReadOnly sanity tests

package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// -----------------------------------------------------------------------------
// Strategy.SpawnOverride: 4 PlanKind × 5 VerdictKind = 20 cases (M3 行为增量)
// -----------------------------------------------------------------------------

// T: D7-SX-AXX-T01 — CommitmentPlan + VerdictFail → SpawnNone (M3 行为增量, 1-step terminal)
func TestStrategy_SpawnOverride_CommitmentPlan_Fail(t *testing.T) {
	round := baseRound(types.VerdictFail, plan.CommitmentPlan, 0.8)
	got, ok := commitmentStrategy{}.SpawnOverride(round)
	if !ok || got != SpawnNone {
		t.Fatalf("CommitmentPlan+Fail: got (%q,%v), want (SpawnNone, true)", got, ok)
	}
}

// T: D7-SX-AXX-T02 — CommitmentPlan + VerdictPartial → SpawnNone (M3 行为增量, terminal partial)
// CC-1.4 deliverable continuation precedence: incomplete deliverable → fall through.
func TestStrategy_SpawnOverride_CommitmentPlan_Partial(t *testing.T) {
	t.Run("low U no deliverable → SpawnNone (M3 行为增量)", func(t *testing.T) {
		round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.3)
		got, ok := commitmentStrategy{}.SpawnOverride(round)
		if !ok || got != SpawnNone {
			t.Fatalf("got (%q,%v), want (SpawnNone, true)", got, ok)
		}
	})
	t.Run("high U no deliverable → SpawnNone (M3 overrides decompose)", func(t *testing.T) {
		round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.9)
		got, ok := commitmentStrategy{}.SpawnOverride(round)
		if !ok || got != SpawnNone {
			t.Fatalf("got (%q,%v), want (SpawnNone, true)", got, ok)
		}
	})
	t.Run("incomplete deliverable → fall through (CC-1.4 precedence)", func(t *testing.T) {
		round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.3)
		round.DeliverableSchema = FirstRegisteredDeliverableSchema()
		round.DeliverableStatus = DeliverableStatusIncomplete
		got, ok := commitmentStrategy{}.SpawnOverride(round)
		if ok {
			t.Fatalf("got (%q,%v), want fall through (ok=false) for deliverable continuation", got, ok)
		}
	})
}

// T: D7-SX-AXX-T03 — CommitmentPlan + VerdictPass → fall through (no M3 override)
func TestStrategy_SpawnOverride_CommitmentPlan_Pass(t *testing.T) {
	round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
	_, ok := commitmentStrategy{}.SpawnOverride(round)
	if ok {
		t.Fatalf("CommitmentPlan+Pass: want fall through (ok=false)")
	}
}

// T: D7-SX-AXX-T04 — CommitmentPlan + VerdictIndeterminate → fall through
func TestStrategy_SpawnOverride_CommitmentPlan_Indeterminate(t *testing.T) {
	round := baseRound(types.VerdictIndeterminate, plan.CommitmentPlan, 0.5)
	_, ok := commitmentStrategy{}.SpawnOverride(round)
	if ok {
		t.Fatalf("CommitmentPlan+Indeterminate: want fall through")
	}
}

// T: D7-SX-AXX-T05 — CommitmentPlan + unknown Verdict → fall through
func TestStrategy_SpawnOverride_CommitmentPlan_UnknownVerdict(t *testing.T) {
	round := baseRound(types.VerdictKind(99), plan.CommitmentPlan, 0.5)
	_, ok := commitmentStrategy{}.SpawnOverride(round)
	if ok {
		t.Fatalf("CommitmentPlan+unknown: want fall through")
	}
}

// T: D7-SX-AXX-T06 — ScenarioPlan + VerdictFail → SpawnNone (M3 行为增量, read-only no retry)
func TestStrategy_SpawnOverride_ScenarioPlan_Fail(t *testing.T) {
	round := baseRound(types.VerdictFail, plan.ScenarioPlan, 0.8)
	got, ok := scenarioStrategy{}.SpawnOverride(round)
	if !ok || got != SpawnNone {
		t.Fatalf("ScenarioPlan+Fail: got (%q,%v), want (SpawnNone, true)", got, ok)
	}
}

// T: D7-SX-AXX-T07 — ScenarioPlan + other verdicts → fall through (16/20 unaffected)
func TestStrategy_SpawnOverride_ScenarioPlan_Others(t *testing.T) {
	for _, v := range []types.VerdictKind{
		types.VerdictPass, types.VerdictPartial, types.VerdictIndeterminate, types.VerdictKind(99),
	} {
		round := baseRound(v, plan.ScenarioPlan, 0.5)
		_, ok := scenarioStrategy{}.SpawnOverride(round)
		if ok {
			t.Fatalf("ScenarioPlan+%v: want fall through", v)
		}
	}
}

// T: D7-SX-AXX-T08 — ExplorationPlan + VerdictPass → SpawnDecompose (M3 行为增量, parallel explore)
func TestStrategy_SpawnOverride_ExplorationPlan_Pass(t *testing.T) {
	round := baseRound(types.VerdictPass, plan.ExplorationPlan, 0.2)
	got, ok := explorationStrategy{}.SpawnOverride(round)
	if !ok || got != SpawnDecompose {
		t.Fatalf("ExplorationPlan+Pass: got (%q,%v), want (SpawnDecompose, true)", got, ok)
	}
}

// T: D7-SX-AXX-T09 — ExplorationPlan + other verdicts → fall through
func TestStrategy_SpawnOverride_ExplorationPlan_Others(t *testing.T) {
	for _, v := range []types.VerdictKind{
		types.VerdictFail, types.VerdictPartial, types.VerdictIndeterminate, types.VerdictKind(99),
	} {
		round := baseRound(v, plan.ExplorationPlan, 0.5)
		_, ok := explorationStrategy{}.SpawnOverride(round)
		if ok {
			t.Fatalf("ExplorationPlan+%v: want fall through", v)
		}
	}
}

// T: D7-SX-AXX-T10 — ProtocolPlan: all 5 verdicts fall through (safe default)
func TestStrategy_SpawnOverride_ProtocolPlan_AllVerdicts(t *testing.T) {
	for _, v := range []types.VerdictKind{
		types.VerdictPass, types.VerdictFail, types.VerdictPartial,
		types.VerdictIndeterminate, types.VerdictKind(99),
	} {
		round := baseRound(v, plan.ProtocolPlan, 0.5)
		_, ok := protocolStrategy{}.SpawnOverride(round)
		if ok {
			t.Fatalf("ProtocolPlan+%v: want fall through (safe default)", v)
		}
	}
}

// -----------------------------------------------------------------------------
// Strategy interface contract tests (兜底, nil handling, channel routing)
// -----------------------------------------------------------------------------

// T: D7-SX-AXX-T11 — SpawnOverride: nil round → fall through
func TestStrategy_SpawnOverride_NilRound(t *testing.T) {
	strategies := []Strategy{
		commitmentStrategy{}, protocolStrategy{}, scenarioStrategy{}, explorationStrategy{},
	}
	for _, s := range strategies {
		_, ok := s.SpawnOverride(nil)
		if ok {
			t.Fatalf("%T: nil round should fall through", s)
		}
	}
}

// T: D7-SX-AXX-T12 — SpawnOverride: wrong PlanKind → fall through (each strategy only handles its own)
func TestStrategy_SpawnOverride_WrongPlanKind(t *testing.T) {
	tests := []struct {
		s        Strategy
		wrongKind plan.PlanKind
	}{
		{commitmentStrategy{}, plan.ProtocolPlan},
		{protocolStrategy{}, plan.CommitmentPlan},
		{scenarioStrategy{}, plan.ExplorationPlan},
		{explorationStrategy{}, plan.ScenarioPlan},
	}
	for _, tc := range tests {
		round := baseRound(types.VerdictFail, tc.wrongKind, 0.5)
		_, ok := tc.s.SpawnOverride(round)
		if ok {
			t.Fatalf("%T with wrong kind %v: want fall through", tc.s, tc.wrongKind)
		}
	}
}

// T: D7-SX-AXX-T13 — RouteChannel: 4 PlanKinds route to 4 channel names
func TestStrategy_RouteChannel(t *testing.T) {
	tests := []struct {
		s        Strategy
		kind     plan.PlanKind
		expected string
	}{
		{commitmentStrategy{}, plan.CommitmentPlan, "commit_channel"},
		{protocolStrategy{}, plan.ProtocolPlan, "protocol_channel"},
		{scenarioStrategy{}, plan.ScenarioPlan, "scenario_channel"},
		{explorationStrategy{}, plan.ExplorationPlan, "exploration_channel"},
	}
	for _, tc := range tests {
		if got := tc.s.RouteChannel(tc.kind); got != tc.expected {
			t.Errorf("RouteChannel(%v): got %q, want %q", tc.kind, got, tc.expected)
		}
		// Wrong PlanKind: empty channel
		if got := tc.s.RouteChannel(plan.KindUnset); got != "" {
			t.Errorf("RouteChannel(KindUnset) on %T: got %q, want empty", tc.s, got)
		}
	}
}

// T: D7-SX-AXX-T14 — ShouldDecompose / IsReadOnly sanity check
func TestStrategy_ShouldDecomposeAndIsReadOnly(t *testing.T) {
	type expect struct {
		decompose bool
		readOnly  bool
	}
	tests := []struct {
		s        Strategy
		kind     plan.PlanKind
		expected expect
	}{
		// Commitment: 1-step, no decompose, side effects (NOT read-only)
		{commitmentStrategy{}, plan.CommitmentPlan, expect{false, true}},
		// Protocol: multi-step, decompose, side effects
		{protocolStrategy{}, plan.ProtocolPlan, expect{true, true}},
		// Scenario: read-only, no decompose, no side effects (read-only=true from caller perspective)
		{scenarioStrategy{}, plan.ScenarioPlan, expect{false, false}},
		// Exploration: parallel, decompose, side effects
		{explorationStrategy{}, plan.ExplorationPlan, expect{true, true}},
	}
	for _, tc := range tests {
		if got := tc.s.ShouldDecompose(tc.kind); got != tc.expected.decompose {
			t.Errorf("%T.ShouldDecompose(%v): got %v, want %v", tc.s, tc.kind, got, tc.expected.decompose)
		}
		if got := tc.s.IsReadOnly(tc.kind); got != tc.expected.readOnly {
			t.Errorf("%T.IsReadOnly(%v): got %v, want %v", tc.s, tc.kind, got, tc.expected.readOnly)
		}
	}
}
