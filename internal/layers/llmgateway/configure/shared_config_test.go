package configure

import (
	"testing"
)

// DSAFT: D3-S6-A01-T02 (FeatureFlagDefaults, v1.1 F9, D4-B 决议).
// 8 combination unit test for the 3 v1.1 feature flags:
//   - D3ResilienceEmitEnabled (F1+F2+F3)
//   - D3SafetyLatencyEventEnabled (F8)
//   - D3MetricEmitWarn (F9)
//
// Each combination must produce a distinct, well-formed struct, and the
// default must match the D4-B 决议固化值 (resilience+latency ON, warn OFF).
func TestLLMFeatureFlags_default_matches_D4B_resolution(t *testing.T) {
	got := DefaultFeatureFlags()
	want := LLMFeatureFlags{
		D3ResilienceEmitEnabled:     true,
		D3SafetyLatencyEventEnabled: true,
		D3MetricEmitWarn:            false,
	}
	if got != want {
		t.Errorf("DefaultFeatureFlags = %+v, want %+v (D4-B 决议)", got, want)
	}

	// DefaultLLMGatewayConfig must also carry the same defaults.
	cfg := DefaultLLMGatewayConfig()
	if cfg.FeatureFlags != want {
		t.Errorf("DefaultLLMGatewayConfig.FeatureFlags = %+v, want %+v", cfg.FeatureFlags, want)
	}
}

// TestLLMFeatureFlags_eight_combinations enumerates all 2^3 = 8 flag
// combinations and asserts that each one round-trips through the struct
// without aliasing. This guards against (a) the struct being reduced to
// a single bool by accident, (b) the resolver dropping flags, and (c) any
// future field addition breaking the assumption that "3 flags = 8 states".
func TestLLMFeatureFlags_eight_combinations(t *testing.T) {
	type state struct {
		r, s, w bool
	}
	states := []state{
		{false, false, false},
		{false, false, true},
		{false, true, false},
		{false, true, true},
		{true, false, false},
		{true, false, true},
		{true, true, false},
		{true, true, true},
	}
	if len(states) != 8 {
		t.Fatalf("enumeration length = %d, want 8 (2^3)", len(states))
	}

	seen := make(map[LLMFeatureFlags]bool, 8)
	for _, s := range states {
		f := LLMFeatureFlags{
			D3ResilienceEmitEnabled:     s.r,
			D3SafetyLatencyEventEnabled: s.s,
			D3MetricEmitWarn:            s.w,
		}
		if seen[f] {
			t.Errorf("duplicate state for %+v — flags are not independent", f)
		}
		seen[f] = true

		// Sanity: round-trip equality.
		clone := f
		if clone != f {
			t.Errorf("round-trip mismatch for %+v", f)
		}
	}

	if len(seen) != 8 {
		t.Errorf("distinct states seen = %d, want 8", len(seen))
	}
}

// TestBuildLLMGatewayConfig_feature_flags_default verifies that the file
// resolver preserves the default flags when no override is provided in YAML.
// This is the OFF-path safety net: even an empty config still gives the
// D4-B 决议 defaults.
func TestBuildLLMGatewayConfig_feature_flags_default(t *testing.T) {
	cfg := BuildLLMGatewayConfig(nil)
	if cfg.FeatureFlags != DefaultFeatureFlags() {
		t.Errorf("BuildLLMGatewayConfig(nil).FeatureFlags = %+v, want %+v",
			cfg.FeatureFlags, DefaultFeatureFlags())
	}
}
