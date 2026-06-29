package orchtypes

import (
	"regexp"
	"testing"
)

// TestBoundaryDecisions_Exist ensures all 3 governance constants are non-empty.
//
// DM-20260629-004 PR-7 #5 boundary-decision (T30): each RESOLVED boundary debt
// MUST be a non-empty string. Drift toward empty would silently break the
// `grep boundary-debt:` audit trail used by D2/D3/D7 cross-domain lint.
func TestBoundaryDecisions_Exist(t *testing.T) {
	decisions := []struct {
		name string
		val  string
	}{
		{"BoundaryD4ToD7AgentEventBridge", BoundaryD4ToD7AgentEventBridge},
		{"BoundaryD4ToD6EvolutionObserver", BoundaryD4ToD6EvolutionObserver},
		{"BoundaryD4ForbiddenFlowHubPublish", BoundaryD4ForbiddenFlowHubPublish},
	}
	for _, d := range decisions {
		if d.val == "" {
			t.Errorf("%s is empty; expected non-empty boundary decision constant", d.name)
		}
	}
}

// TestBoundaryDecisions_VersionFormat enforces the
// `^boundary-debt:[a-z0-9\-]+-v\d+\.\d+$` contract.
//
// Format pins the namespace (boundary-debt:) and version scheme (-v{major}.{minor}).
// All 3 RESOLVED D4 debts MUST match. Lint test (PR-7 T30) — fail fast on
// future drift; new boundary debts adopt the same prefix so a single
// `grep -r 'boundary-debt:' openspec/ internal/` audit holds across D2/D3/D4/D7.
func TestBoundaryDecisions_VersionFormat(t *testing.T) {
	pattern := regexp.MustCompile(`^boundary-debt:[a-z0-9\-]+-v\d+\.\d+$`)
	decisions := []struct {
		name string
		val  string
	}{
		{"BoundaryD4ToD7AgentEventBridge", BoundaryD4ToD7AgentEventBridge},
		{"BoundaryD4ToD6EvolutionObserver", BoundaryD4ToD6EvolutionObserver},
		{"BoundaryD4ForbiddenFlowHubPublish", BoundaryD4ForbiddenFlowHubPublish},
	}
	for _, d := range decisions {
		if !pattern.MatchString(d.val) {
			t.Errorf("%s = %q does not match format %q",
				d.name, d.val, pattern.String())
		}
	}
}

// TestBoundaryDecisions_Unique ensures AllBoundaryDecisions returns 3 distinct
// strings. Duplicate constants indicate a copy-paste error in
// boundary_decision.go and would silently alias two boundary semantics under
// one ID, breaking the audit trail.
//
// DM-20260629-004 PR-7 #5 boundary-decision (T30): uniqueness invariant.
func TestBoundaryDecisions_Unique(t *testing.T) {
	all := AllBoundaryDecisions()
	if len(all) != 3 {
		t.Fatalf("AllBoundaryDecisions() returned %d entries; expected 3", len(all))
	}
	seen := make(map[string]bool, len(all))
	for _, d := range all {
		if seen[d] {
			t.Errorf("duplicate boundary decision: %q", d)
		}
		seen[d] = true
	}
	if len(seen) != 3 {
		t.Errorf("expected 3 unique decisions, got %d", len(seen))
	}
}