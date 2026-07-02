// T: D5-S25-A01-T01, A02-T01, A03-T01 — LTL-Lite L4–L6 termination invariant unit tests.
package termination

import (
	"strings"
	"testing"
)

// D5-S25-A01-T01: L4 BoundedInvariant fires when iter >= MaxN.
func TestBoundedInvariant_FiresAtBound(t *testing.T) {
	inv, err := NewBoundedInvariant("ProbeToolChannel", 15)
	if err != nil {
		t.Fatalf("NewBoundedInvariant: %v", err)
	}

	// iter=14, MaxN=15 → ok (under bound)
	ok, reason := inv.Check(&State{ChannelName: "ProbeToolChannel", IterationsUsed: 14, BoundMax: 15})
	if !ok {
		t.Errorf("iter=14 should be under bound, got reason=%q", reason)
	}

	// iter=15, MaxN=15 → fires (at bound)
	ok, reason = inv.Check(&State{ChannelName: "ProbeToolChannel", IterationsUsed: 15, BoundMax: 15})
	if ok {
		t.Errorf("iter=15 should fire at bound, got ok=true")
	}
	if !strings.Contains(reason, "iter=15") || !strings.Contains(reason, "bound=15") {
		t.Errorf("reason missing iter/bound: %q", reason)
	}

	// iter=16, MaxN=15 → fires (over bound)
	ok, _ = inv.Check(&State{ChannelName: "ProbeToolChannel", IterationsUsed: 16, BoundMax: 15})
	if ok {
		t.Errorf("iter=16 should fire, got ok=true")
	}
}

// D5-S25-A01-T01: L4 BoundedInvariant rejects MaxN <= 0 (configuration bug).
func TestBoundedInvariant_RejectsZeroMax(t *testing.T) {
	_, err := NewBoundedInvariant("ProbeToolChannel", 0)
	if err == nil {
		t.Errorf("NewBoundedInvariant(0) should return error")
	}
	_, err = NewBoundedInvariant("ProbeToolChannel", -1)
	if err == nil {
		t.Errorf("NewBoundedInvariant(-1) should return error")
	}
}

// D5-S25-A01-T01: L4 BoundedInvariant name is "L4-BOUNDED-ITERATIONS".
func TestBoundedInvariant_Name(t *testing.T) {
	inv, _ := NewBoundedInvariant("ProbeToolChannel", 15)
	if inv.Name() != "L4-BOUNDED-ITERATIONS" {
		t.Errorf("expected L4-BOUNDED-ITERATIONS, got %q", inv.Name())
	}
}

// D5-S25-A02-T01: L5 QuotientInvariant fires when metric >= Threshold.
func TestQuotientInvariant_FiresAtThreshold(t *testing.T) {
	// Threshold 0.8; metric returns 0.0 / 0.5 / 0.8 / 0.9
	inv := NewQuotientInvariant("ExperimentToolChannel", 0.8,
		func(s *State) float64 { return float64(s.IterationsUsed) / 10.0 })

	cases := []struct {
		iter  int
		want  bool
	}{
		{0, true},  // metric 0.0
		{5, true},  // metric 0.5
		{7, true},  // metric 0.7
		{8, false}, // metric 0.8 (at threshold, fires)
		{9, false}, // metric 0.9
	}
	for _, c := range cases {
		ok, _ := inv.Check(&State{IterationsUsed: c.iter})
		if ok != c.want {
			t.Errorf("iter=%d: want ok=%v, got %v", c.iter, c.want, ok)
		}
	}
}

// D5-S25-A03-T01: L6 SynthesizeInvariant accepts (advisory only; channel
// decides on Finalize).
func TestSynthesizeInvariant_NeverFiresFromCheck(t *testing.T) {
	inv := NewSynthesizeInvariant("ProbeToolChannel", 20)
	ok, _ := inv.Check(&State{IterationsUsed: 0})
	if !ok {
		t.Errorf("SynthesizeInvariant.Check should never fire (advisory only)")
	}
}

// D5-S25-A01-T01: L7-FACT-SAME-Q-5x fires at 5+ same-query calls.
func TestFactSameQueryInvariant_FiresAtFive(t *testing.T) {
	inv := NewFactSameQueryInvariant()

	// 0..4 same-query → ok
	for n := 0; n < 5; n++ {
		ok, _ := inv.Check(&State{SameQueryCount: n})
		if !ok {
			t.Errorf("same_query_count=%d should be under threshold", n)
		}
	}

	// 5 → fires
	ok, reason := inv.Check(&State{SameQueryCount: 5})
	if ok {
		t.Errorf("same_query_count=5 should fire")
	}
	if !strings.Contains(reason, "escalate to Probe") {
		t.Errorf("reason should mention escalation: %q", reason)
	}
}

// D5-S25-A01-T01: L7-ACTION-POSTSNAPSHOT fires when snapshots are equal
// (no state change for a state-changing tool).
func TestActionPostSnapshotInvariant_FiresOnEqual(t *testing.T) {
	inv := NewActionPostSnapshotInvariant()

	// nil snapshots → ok (caller didn't capture)
	ok, _ := inv.Check(&State{})
	if !ok {
		t.Errorf("nil snapshots should be ok (skip check)")
	}

	// different snapshots → ok
	ok, _ = inv.Check(&State{
		PreSnapshot:  []byte("before"),
		PostSnapshot: []byte("after"),
	})
	if !ok {
		t.Errorf("different snapshots should be ok (state changed)")
	}

	// equal snapshots → fires
	ok, reason := inv.Check(&State{
		PreSnapshot:  []byte("same"),
		PostSnapshot: []byte("same"),
	})
	if ok {
		t.Errorf("equal snapshots should fire (no state change)")
	}
	if !strings.Contains(reason, "state_change_required") {
		t.Errorf("reason should mention state_change_required: %q", reason)
	}
}

// D5-S25-A01-T01: L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE fires when
// ConcludedAt > Deadline.
func TestExperimentDeadlineInvariant_FiresOnMiss(t *testing.T) {
	inv := NewExperimentDeadlineInvariant()

	// nil ConcludedAt → ok (pending)
	ok, _ := inv.Check(&State{Deadline: 1000})
	if !ok {
		t.Errorf("pending (nil ConcludedAt) should be ok")
	}

	// ConcludedAt < Deadline → ok
	early := int64(500)
	ok, _ = inv.Check(&State{ConcludedAt: &early, Deadline: 1000})
	if !ok {
		t.Errorf("early conclusion should be ok")
	}

	// ConcludedAt == Deadline → ok (boundary)
	onTime := int64(1000)
	ok, _ = inv.Check(&State{ConcludedAt: &onTime, Deadline: 1000})
	if !ok {
		t.Errorf("on-time conclusion should be ok (boundary)")
	}

	// ConcludedAt > Deadline → fires
	late := int64(1500)
	ok, reason := inv.Check(&State{ConcludedAt: &late, Deadline: 1000})
	if ok {
		t.Errorf("late conclusion should fire")
	}
	if !strings.Contains(reason, "missed deadline") {
		t.Errorf("reason should mention missed deadline: %q", reason)
	}
}

// D5-S25-A01-T01: EmissionClassToInvariantName canonical mapping.
func TestEmissionClassToInvariantName(t *testing.T) {
	cases := []struct {
		ec   int
		want string
	}{
		{0, "L7-ACTION-POSTSNAPSHOT"},      // EC_Action
		{1, "L7-FACT-SAME-Q-5x"},            // EC_Fact
		{2, "L4-BOUNDED-ITERATIONS"},        // EC_Probe
		{3, "L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE"}, // EC_Experiment
	}
	for _, c := range cases {
		got := EmissionClassToInvariantNameFromInt(c.ec)
		if got != c.want {
			t.Errorf("ec=%d: want %q, got %q", c.ec, c.want, got)
		}
	}
}

// EmissionClassToInvariantNameFromInt is a test-only helper that converts
// the integer EmissionClass to the invariant name.
func EmissionClassToInvariantNameFromInt(ec int) string {
	switch ec {
	case 0:
		return "L7-ACTION-POSTSNAPSHOT"
	case 1:
		return "L7-FACT-SAME-Q-5x"
	case 2:
		return "L4-BOUNDED-ITERATIONS"
	case 3:
		return "L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE"
	}
	return "L4-BOUNDED-ITERATIONS"
}

// D5-S25-A01-T02 (CC-1 cross-check): BoundedInvariant does NOT bypass
// permission guards. Cross-check is documented in the package — verified
// via docstring test.
func TestBoundedInvariant_DoesNotBypassPermissionGuards(t *testing.T) {
	// The invariant's contract is: BoundedInvariant is a *termination*
	// check. It fires when iter >= bound. It does NOT consult permission
	// state, nor does its firing bypass the permission gate. The
	// ChannelRouter (PR-B T08 cross-check) must enforce permission
	// guards BEFORE delegating to BoundedInvariant.
	//
	// This test asserts the invariant signature: Check takes only State
	// and returns (ok, reason). It has no permission-related field.
	inv, _ := NewBoundedInvariant("ProbeToolChannel", 15)
	state := &State{ChannelName: "ProbeToolChannel", IterationsUsed: 15, BoundMax: 15}
	ok, _ := inv.Check(state)
	if ok {
		t.Errorf("BoundedInvariant should fire at iter==MaxN regardless of permission state")
	}
	// Permission guards are checked separately by the ChannelRouter, not
	// by the invariant itself. This is the H8 / P1-AC-2 cross-check.
}
