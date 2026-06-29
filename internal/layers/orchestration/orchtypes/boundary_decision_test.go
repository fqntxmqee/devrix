package orchtypes

import (
	"regexp"
	"testing"
)

// T: DM-20260629-001 T48 — boundary debt 常量存在 + 版本号格式。
func TestBoundaryDecisionConstants_Exist(t *testing.T) {
	cases := []struct {
		name string
		got  string
	}{
		{"BoundaryReputationEvidence", BoundaryReputationEvidence},
		{"BoundarySystemAnomaly", BoundarySystemAnomaly},
		{"BoundaryAdaptivePrior", BoundaryAdaptivePrior},
	}
	for _, tc := range cases {
		if tc.got == "" {
			t.Errorf("%s = empty, want non-empty boundary-debt ID", tc.name)
		}
	}
}

// T: DM-20260629-001 T48 — 版本号格式 boundary-debt:{name}-v{major}.{minor}。
func TestBoundaryDecisionConstants_VersionFormat(t *testing.T) {
	pattern := regexp.MustCompile(`^boundary-debt:[a-z\-]+-v\d+\.\d+$`)
	cases := []struct {
		name string
		got  string
	}{
		{"BoundaryReputationEvidence", BoundaryReputationEvidence},
		{"BoundarySystemAnomaly", BoundarySystemAnomaly},
		{"BoundaryAdaptivePrior", BoundaryAdaptivePrior},
	}
	for _, tc := range cases {
		if !pattern.MatchString(tc.got) {
			t.Errorf("%s = %q, want format %q", tc.name, tc.got, pattern.String())
		}
	}
}

// T: DM-20260629-001 T48 — 3 个常量唯一。
func TestBoundaryDecisionConstants_Unique(t *testing.T) {
	seen := map[string]string{
		BoundaryReputationEvidence: "BoundaryReputationEvidence",
		BoundarySystemAnomaly:      "BoundarySystemAnomaly",
		BoundaryAdaptivePrior:      "BoundaryAdaptivePrior",
	}
	if len(seen) != 3 {
		t.Errorf("expected 3 unique boundary-debt IDs, got %d", len(seen))
	}
}

// T: DM-20260629-001 T48 — 3 个常量非空且非 default 哨兵。
func TestBoundaryDecisionConstants_NonSentinel(t *testing.T) {
	cases := []string{
		BoundaryReputationEvidence,
		BoundarySystemAnomaly,
		BoundaryAdaptivePrior,
	}
	for _, c := range cases {
		if c == "" || c == "boundary-debt:unknown" {
			t.Errorf("boundary debt ID %q is empty or sentinel", c)
		}
	}
}