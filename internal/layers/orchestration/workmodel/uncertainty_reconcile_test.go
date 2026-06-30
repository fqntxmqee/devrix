package workmodel

import (
	"math"
	"testing"
)

// T: D7-S1-A88-T01 (DM-20260701-001 RH-MUPS-01/02 ReconcileUncertainty)
//
// Table-driven coverage of the convergence-visibility contract:
//   - All children terminal (pass): value MUST drop from prevStored.
//   - All children terminal (fail): value MUST stay high.
//   - Children running: single-round optimism MUST NOT collapse value.
//   - prevStored=1.0 with all-pass terminal children: result MUST drop.
func TestReconcileUncertainty_AllChildrenTerminal_ConvergenceVisibility(t *testing.T) {
	cases := []struct {
		name       string
		prevStored float64
		round      float64
		stats      ChildOutcomeStats
		wantBelow  *float64 // result MUST be ≤ this; nil = no check
		wantAbove  *float64 // result MUST be ≥ this; nil = no check
	}{
		{
			name:       "all_pass_terminal_drops_from_prev",
			prevStored: 0.9,
			round:      0.1,
			stats:      ChildOutcomeStats{Total: 4, Completed: 4, Failed: 0, Running: 0},
			wantBelow:  ptrF(0.45), // hist=0, blended = 0.7*0.1 + 0.3*0 = 0.07
			wantAbove:  ptrF(0.0),
		},
		{
			name:       "all_pass_terminal_pinned_prev_must_drop",
			prevStored: 1.0,
			round:      0.05,
			stats:      ChildOutcomeStats{Total: 3, Completed: 3, Failed: 0, Running: 0},
			wantBelow:  ptrF(0.4), // prevStored=1.0 must NOT ratchet the converged value
		},
		{
			name:       "all_fail_terminal_stays_high",
			prevStored: 0.2,
			round:      0.8,
			stats:      ChildOutcomeStats{Total: 3, Completed: 0, Failed: 3, Running: 0},
			wantAbove:  ptrF(0.6),
		},
		{
			name:       "mixed_terminal_proportional",
			prevStored: 0.5,
			round:      0.5,
			stats:      ChildOutcomeStats{Total: 4, Completed: 3, Failed: 1, Running: 0},
			wantBelow:  ptrF(0.7),
			wantAbove:  ptrF(0.3),
		},
		{
			name:       "children_running_prevents_optimism_collapse",
			prevStored: 0.6,
			round:      0.05, // single-round optimism
			stats:      ChildOutcomeStats{Total: 4, Completed: 1, Failed: 0, Running: 3},
			// damped: 0.5*0.6 + 0.5*historical. historicalUncertainty: stats.Total>0,
			// stats.Running>0 → 0.3 + (0/4)*0.4 = 0.3. damped = 0.5*0.6 + 0.5*0.3 = 0.45
			// roundSignal=0.05 → max(0.05, 0.45) = 0.45
			wantAbove:  ptrF(0.3),
			wantBelow:  ptrF(0.6),
		},
		{
			name:       "no_children_at_all_returns_blend",
			prevStored: 0.5,
			round:      0.4,
			stats:      ChildOutcomeStats{}, // Total==0 → falls through to in-flight branch
			// damped: 0.5*0.5 + 0.5*historical=0.5 → 0.5; round=0.4 → max(0.4, 0.5)=0.5
			wantAbove:  ptrF(0.3),
			wantBelow:  ptrF(0.7),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ReconcileUncertainty(tc.prevStored, tc.round, tc.stats)
			if tc.wantAbove != nil && got < *tc.wantAbove {
				t.Errorf("got %.4f, want ≥ %.4f", got, *tc.wantAbove)
			}
			if tc.wantBelow != nil && got > *tc.wantBelow {
				t.Errorf("got %.4f, want ≤ %.4f", got, *tc.wantBelow)
			}
			if got < 0 || got > 1 {
				t.Errorf("got %.4f, must be in [0,1]", got)
			}
		})
	}
}

func ptrF(v float64) *float64 { return &v }

// T: D7-S1-A88-T02 (DM-20260701-001 RH-MUPS-01)
//
// Idempotency: repeated calls with the same converged inputs MUST produce
// the same value (the function is pure). This guards against future edits
// that might smuggle in a ratchet via prevStored comparison.
func TestReconcileUncertainty_Idempotent(t *testing.T) {
	stats := ChildOutcomeStats{Total: 2, Completed: 2, Running: 0}
	first := ReconcileUncertainty(0.7, 0.1, stats)
	for i := 0; i < 10; i++ {
		again := ReconcileUncertainty(0.7, 0.1, stats)
		if math.Abs(first-again) > 1e-12 {
			t.Errorf("non-idempotent: first=%.12f iter=%d got=%.12f", first, i, again)
		}
	}
}

// T: D7-S1-A88-T03 (DM-20260701-001 RH-MUPS-02)
//
// Writer-equivalence: given the same three inputs, ReconcileUncertainty
// produces a deterministic value regardless of "call order". This guards
// against future edits that try to make it stateful. The contract under
// RH-MUPS-02 is: two writers (item_pipeline + reevaluateParentAfterChild)
// MUST converge to the same value when their effective inputs match.
func TestReconcileUncertainty_WriterEquivalence(t *testing.T) {
	stats := ChildOutcomeStats{Total: 5, Completed: 5, Running: 0}
	// Writer 1: pipeline round just finished with strong evidence.
	v1 := ReconcileUncertainty(0.4, 0.1, stats)
	// Writer 2: reevaluate runs after — same stats, same round signal,
	// prevStored is what writer 1 just wrote. The result should NOT climb
	// from writer 1 to writer 2 (the old code's race created such climbs).
	v2 := ReconcileUncertainty(v1, 0.1, stats)
	if math.Abs(v1-v2) > 1e-12 {
		t.Errorf("writer-equivalence violated: writer1=%.4f writer2=%.4f", v1, v2)
	}
}

// T: D7-S1-A88-T04 (DM-20260701-001 RH-MUPS-01)
//
// Negative inputs and NaN are clamped/defensively bounded. A naked-max
// ratchet with negative round signals would let prevStored dominate even
// when the round evidence is strongly convergent (negative = certain).
func TestReconcileUncertainty_DefensiveBounds(t *testing.T) {
	stats := ChildOutcomeStats{Total: 1, Completed: 1, Running: 0}
	got := ReconcileUncertainty(0.0, -1.0, stats)
	if got < 0 || got > 1 {
		t.Errorf("got %.4f, expected [0,1]", got)
	}
	// Converged child with strongly-negative round signal (impossible but
	// defensive): the result must clamp to 0, not panic.
	if got > 0.01 {
		t.Errorf("expected near-zero (clamped negative round), got %.4f", got)
	}
}