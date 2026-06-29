package bootstrap

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// TestPessimisticCommitEnabled_DefaultsOff — unset → false (0 行为变更).
func TestPessimisticCommitEnabled_DefaultsOff(t *testing.T) {
	t.Setenv("D7_PESSIMISTIC_COMMIT_ENABLED", "")
	if PessimisticCommitEnabled() {
		t.Error("unset env should report disabled (0 行为变更)")
	}
}

// TestPessimisticCommitEnabled_Truthy — "1" / "true" / "yes" / "on" enable.
func TestPessimisticCommitEnabled_Truthy(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "True", "yes", "YES", "on", "ON"} {
		t.Setenv("D7_PESSIMISTIC_COMMIT_ENABLED", v)
		if !PessimisticCommitEnabled() {
			t.Errorf("D7_PESSIMISTIC_COMMIT_ENABLED=%q should be enabled", v)
		}
	}
}

// TestPessimisticCommitEnabled_Falsy — anything not in truthy set = off.
func TestPessimisticCommitEnabled_Falsy(t *testing.T) {
	for _, v := range []string{"0", "false", "no", "off", "random", "  "} {
		t.Setenv("D7_PESSIMISTIC_COMMIT_ENABLED", v)
		if PessimisticCommitEnabled() {
			t.Errorf("D7_PESSIMISTIC_COMMIT_ENABLED=%q should be disabled", v)
		}
	}
}

// TestPessimisticRuleStrategy_Default — unset → min_uncertainty (default).
func TestPessimisticRuleStrategy_Default(t *testing.T) {
	t.Setenv("D7_RULE_FALLBACK_STRATEGY", "")
	name, rec := PessimisticRuleStrategy()
	if name != interfaces.DefaultFallbackRule {
		t.Errorf("name = %q, want %q", name, interfaces.DefaultFallbackRule)
	}
	if !rec {
		t.Error("unset should be recognized=true (fall back to default by design)")
	}
}

// TestPessimisticRuleStrategy_AllValid — all 4 candidates round-trip.
func TestPessimisticRuleStrategy_AllValid(t *testing.T) {
	for _, rule := range interfaces.FallbackPolicyRuleNames {
		t.Setenv("D7_RULE_FALLBACK_STRATEGY", rule)
		name, rec := PessimisticRuleStrategy()
		if name != rule || !rec {
			t.Errorf("rule=%q: got (%q,%v), want (%q,true)", rule, name, rec, rule)
		}
	}
}

// TestPessimisticRuleStrategy_InvalidFallsBack — unknown rule name → default.
func TestPessimisticRuleStrategy_InvalidFallsBack(t *testing.T) {
	t.Setenv("D7_RULE_FALLBACK_STRATEGY", "totally_made_up")
	name, rec := PessimisticRuleStrategy()
	if name != interfaces.DefaultFallbackRule {
		t.Errorf("invalid rule → got %q, want %q", name, interfaces.DefaultFallbackRule)
	}
	if rec {
		t.Error("invalid rule should be recognized=false")
	}
}

// TestNewPessimisticCommitGuardFromEnv_OffByDefault — factory returns
// a guard with Enabled=false when env is unset.
func TestNewPessimisticCommitGuardFromEnv_OffByDefault(t *testing.T) {
	t.Setenv("D7_PESSIMISTIC_COMMIT_ENABLED", "")
	t.Setenv("D7_RULE_FALLBACK_STRATEGY", "")
	g := NewPessimisticCommitGuardFromEnv()
	if g == nil {
		t.Fatal("guard must never be nil")
	}
	if g.Enabled {
		t.Error("Enabled should be false by default")
	}
	if g.RuleName != interfaces.DefaultFallbackRule {
		t.Errorf("RuleName = %q, want %q", g.RuleName, interfaces.DefaultFallbackRule)
	}
}

// TestNewPessimisticCommitGuardFromEnv_EnabledWithCustomRule — env wires
// both the flag and the rule.
func TestNewPessimisticCommitGuardFromEnv_EnabledWithCustomRule(t *testing.T) {
	t.Setenv("D7_PESSIMISTIC_COMMIT_ENABLED", "true")
	t.Setenv("D7_RULE_FALLBACK_STRATEGY", "most_tests_passed")
	g := NewPessimisticCommitGuardFromEnv()
	if !g.Enabled {
		t.Error("Enabled should be true")
	}
	if g.RuleName != "most_tests_passed" {
		t.Errorf("RuleName = %q, want most_tests_passed", g.RuleName)
	}
}
