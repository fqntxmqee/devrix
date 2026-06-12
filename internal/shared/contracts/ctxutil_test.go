// Tests for shared/contracts cross-layer helpers.
//
// Covers: L5-0-0-02  (cross-layer contract surface available for any D)
// Domain: shared/contracts
// Stage: s0_unit
package contracts

import "testing"

// TestComputeCtxPct_Boundaries verifies clamp and zero-safety semantics.
func TestComputeCtxPct_Boundaries(t *testing.T) {
	cases := []struct {
		name           string
		promptTokens   int
		maxContextToks int
		want           int
	}{
		{"zero max returns 0", 1000, 0, 0},
		{"zero prompt returns 0", 0, 1000, 0},
		{"normal 50pct", 500, 1000, 50},
		{"normal 12pct", 120, 1000, 12},
		{"clamp to 100 when over", 2000, 1000, 100},
		{"negative max returns 0", 100, -1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeCtxPct(tc.promptTokens, tc.maxContextToks)
			if got != tc.want {
				t.Fatalf("ComputeCtxPct(%d, %d) = %d, want %d",
					tc.promptTokens, tc.maxContextToks, got, tc.want)
			}
		})
	}
}
