// T: D5-S25-A01/A02/A03 + D2-S15-A02-T25 — Advisory mode for L4-L6.
// DM-20260702-008 / D2-S15-A02-T25.
package termination

import (
	"testing"
)

// D2-S15-A02-T25: in Advisory mode the violation is recorded and
// Check returns (true, nil) so the channel continues.
func TestBoundedInvariant_AdvisoryMode_RecordsAndContinues(t *testing.T) {
	inv, _ := NewBoundedInvariant("ProbeToolChannel", 15)
	state := &State{ChannelName: "ProbeToolChannel", IterationsUsed: 16, BoundMax: 15, Advisory: true}

	ok, reason := inv.Check(state)
	if !ok {
		t.Errorf("Advisory mode must return ok=true, got false (reason=%q)", reason)
	}
	if reason != "" {
		t.Errorf("Advisory mode must return empty reason, got %q", reason)
	}
	if len(state.AdvisoryViolations) != 1 {
		t.Fatalf("Advisory mode must record 1 violation, got %d", len(state.AdvisoryViolations))
	}
	v := state.AdvisoryViolations[0]
	if v.Invariant != "L4-BOUNDED-ITERATIONS" {
		t.Errorf("violation.Invariant = %q, want L4-BOUNDED-ITERATIONS", v.Invariant)
	}
	if v.IterationsUsed != 16 || v.BoundMax != 15 {
		t.Errorf("violation context: iter=%d bound=%d, want 16/15", v.IterationsUsed, v.BoundMax)
	}
}

// D2-S15-A02-T25: hard mode (default) preserves the original治标
// behavior — Check returns (false, reason), no violation recorded.
func TestBoundedInvariant_HardMode_FiresWithoutRecording(t *testing.T) {
	inv, _ := NewBoundedInvariant("ProbeToolChannel", 15)
	state := &State{ChannelName: "ProbeToolChannel", IterationsUsed: 16, BoundMax: 15} // Advisory=false default

	ok, reason := inv.Check(state)
	if ok {
		t.Errorf("Hard mode must return ok=false, got true")
	}
	if reason == "" {
		t.Errorf("Hard mode must return non-empty reason")
	}
	if len(state.AdvisoryViolations) != 0 {
		t.Errorf("Hard mode must NOT record AdvisoryViolation, got %d", len(state.AdvisoryViolations))
	}
}

// D2-S15-A02-T25: under the bound in advisory mode → no violation.
func TestBoundedInvariant_AdvisoryMode_UnderBound_NoRecord(t *testing.T) {
	inv, _ := NewBoundedInvariant("ProbeToolChannel", 15)
	state := &State{ChannelName: "ProbeToolChannel", IterationsUsed: 10, BoundMax: 15, Advisory: true}

	ok, _ := inv.Check(state)
	if !ok {
		t.Errorf("under bound must return ok=true even in advisory mode")
	}
	if len(state.AdvisoryViolations) != 0 {
		t.Errorf("under bound must not record a violation, got %d", len(state.AdvisoryViolations))
	}
}

// D2-S15-A02-T25: Quotient invariant also respects Advisory.
func TestQuotientInvariant_AdvisoryMode_RecordsAndContinues(t *testing.T) {
	inv := NewQuotientInvariant("ExperimentToolChannel", 0.8,
		func(s *State) float64 { return 0.9 })
	state := &State{Advisory: true}

	ok, reason := inv.Check(state)
	if !ok {
		t.Errorf("Advisory mode must return ok=true, got false (reason=%q)", reason)
	}
	if len(state.AdvisoryViolations) != 1 {
		t.Fatalf("Advisory mode must record 1 violation, got %d", len(state.AdvisoryViolations))
	}
	if state.AdvisoryViolations[0].Invariant != "L5-QUOTIENT-THRESHOLD" {
		t.Errorf("violation.Invariant = %q, want L5-QUOTIENT-THRESHOLD", state.AdvisoryViolations[0].Invariant)
	}
}

// D2-S15-A02-T25: RecordAdvisory is safe on nil state.
func TestState_RecordAdvisory_NilSafe(t *testing.T) {
	var s *State
	s.RecordAdvisory("test", "reason") // must not panic
	if s != nil {
		t.Errorf("nil state should remain nil")
	}
}

// D2-S15-A02-T25: RecordAdvisory no-op when Advisory is false.
func TestState_RecordAdvisory_NoOpWhenNotAdvisory(t *testing.T) {
	s := &State{Advisory: false}
	s.RecordAdvisory("test", "reason")
	if len(s.AdvisoryViolations) != 0 {
		t.Errorf("RecordAdvisory must be no-op when Advisory=false, got %d violations", len(s.AdvisoryViolations))
	}
}

// D2-S15-A02-T25: nowUnixNano is stubable for tests.
func TestState_RecordAdvisory_StubNowFn(t *testing.T) {
	// Save and restore the global nowFn so we don't leak state.
	orig := nowFn
	t.Cleanup(func() { nowFn = orig })
	nowFn = func() int64 { return 1234567890 }

	s := &State{Advisory: true}
	s.RecordAdvisory("test", "reason")
	if len(s.AdvisoryViolations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(s.AdvisoryViolations))
	}
	if s.AdvisoryViolations[0].Timestamp != 1234567890 {
		t.Errorf("Timestamp = %d, want 1234567890 (stubbed)", s.AdvisoryViolations[0].Timestamp)
	}
}
