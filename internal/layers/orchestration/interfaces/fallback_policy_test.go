package interfaces

import "testing"

// TestFallbackPolicy_Valid — D7-S18-A12-T01 sub-case 1.
func TestFallbackPolicy_Valid(t *testing.T) {
	cases := []struct {
		p    FallbackPolicy
		want bool
	}{
		{FallbackAbort, true},
		{FallbackPessimistic, true},
		{FallbackRuleBased, true},
		{FallbackPolicy(99), false},
		{FallbackPolicy(-1), false},
	}
	for _, c := range cases {
		if got := c.p.Valid(); got != c.want {
			t.Errorf("FallbackPolicy(%d).Valid() = %v, want %v", int(c.p), got, c.want)
		}
	}
}

// TestFallbackPolicy_ValidNonLegacy — FallbackAbort (the v6.0.x legacy
// path) must be excluded from the "non-legacy" check.
func TestFallbackPolicy_ValidNonLegacy(t *testing.T) {
	cases := []struct {
		p    FallbackPolicy
		want bool
	}{
		{FallbackAbort, false},
		{FallbackPessimistic, true},
		{FallbackRuleBased, true},
		{FallbackPolicy(99), false},
	}
	for _, c := range cases {
		if got := c.p.ValidNonLegacy(); got != c.want {
			t.Errorf("FallbackPolicy(%d).ValidNonLegacy() = %v, want %v", int(c.p), got, c.want)
		}
	}
}

// TestParseFallbackRuleName — D7-S18-A12-T01 sub-case 2. Empty → default
// (recognized=true). Unrecognized → default + recognized=false. All four
// canonical names must round-trip.
func TestParseFallbackRuleName(t *testing.T) {
	cases := []struct {
		in            string
		wantName      string
		wantRecognize bool
	}{
		{"", DefaultFallbackRule, true},
		{"   ", DefaultFallbackRule, true},
		{"\t\n", DefaultFallbackRule, true},
		{"min_uncertainty", "min_uncertainty", true},
		{"most_tests_passed", "most_tests_passed", true},
		{"compiled_clean", "compiled_clean", true},
		{"min_cost", "min_cost", true},
		{"unknown_rule", DefaultFallbackRule, false},
		{"  min_uncertainty  ", "min_uncertainty", true},
	}
	for _, c := range cases {
		got, rec := ParseFallbackRuleName(c.in)
		if got != c.wantName {
			t.Errorf("ParseFallbackRuleName(%q) name = %q, want %q", c.in, got, c.wantName)
		}
		if rec != c.wantRecognize {
			t.Errorf("ParseFallbackRuleName(%q) recognized = %v, want %v", c.in, rec, c.wantRecognize)
		}
	}
}

// TestFallbackPolicyRuleNames_ClosedSet — the rule set must contain
// exactly 4 entries and include the default. Guards against accidental
// expansion without a corresponding design.md update.
func TestFallbackPolicyRuleNames_ClosedSet(t *testing.T) {
	if got := len(FallbackPolicyRuleNames); got != 4 {
		t.Errorf("FallbackPolicyRuleNames length = %d, want 4", got)
	}
	found := false
	for _, n := range FallbackPolicyRuleNames {
		if n == DefaultFallbackRule {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DefaultFallbackRule %q not in FallbackPolicyRuleNames", DefaultFallbackRule)
	}
}

// TestDefaultFallbackRule_Stable — the default is part of the audit
// contract. Renaming it would invalidate operator dashboards.
func TestDefaultFallbackRule_Stable(t *testing.T) {
	if DefaultFallbackRule != "min_uncertainty" {
		t.Errorf("DefaultFallbackRule = %q, want min_uncertainty", DefaultFallbackRule)
	}
}
