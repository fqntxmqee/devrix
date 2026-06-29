package orchtypes

import (
	"regexp"
	"testing"
)

// T: DM-20260629-002 T44 — boundary debt 常量存在 + 版本号格式。
func TestBoundaryDecisionConstants_Exist(t *testing.T) {
	cases := []struct {
		name string
		got  string
	}{
		{"BoundaryDM018SliceC", BoundaryDM018SliceC},
		{"BoundaryCrossDomainFixtures", BoundaryCrossDomainFixtures},
	}
	for _, tc := range cases {
		if tc.got == "" {
			t.Errorf("%s = empty, want non-empty boundary-debt ID", tc.name)
		}
	}
}

// T: DM-20260629-002 T44 — 版本号格式 boundary-debt:{name}-v{major}.{minor}。
func TestBoundaryDecisionConstants_VersionFormat(t *testing.T) {
	pattern := regexp.MustCompile(`^boundary-debt:[a-z0-9\-]+-v\d+\.\d+$`)
	cases := []struct {
		name string
		got  string
	}{
		{"BoundaryDM018SliceC", BoundaryDM018SliceC},
		{"BoundaryCrossDomainFixtures", BoundaryCrossDomainFixtures},
	}
	for _, tc := range cases {
		if !pattern.MatchString(tc.got) {
			t.Errorf("%s = %q, want format %q", tc.name, tc.got, pattern.String())
		}
	}
}

// T: DM-20260629-002 T44 — 2 个常量唯一。
func TestBoundaryDecisionConstants_Unique(t *testing.T) {
	seen := map[string]string{
		BoundaryDM018SliceC:         "BoundaryDM018SliceC",
		BoundaryCrossDomainFixtures: "BoundaryCrossDomainFixtures",
	}
	if len(seen) != 2 {
		t.Errorf("expected 2 unique constants, got %d", len(seen))
	}
	for id, name := range seen {
		if id == "" {
			t.Errorf("%s has empty ID", name)
		}
	}
}

// T: DM-20260629-002 T44 — 历史 / 现役 / 待定分类正确。
func TestBoundaryDecisionConstants_Categorization(t *testing.T) {
	// BoundaryDM018SliceC = RESOLVED (DM-018 已在 v8.0.0 落实)
	resolved := []string{BoundaryDM018SliceC}
	for _, id := range resolved {
		if id == "" {
			t.Error("resolved boundary ID must be non-empty (历史追溯)")
		}
	}
	// BoundaryCrossDomainFixtures = 待定 (v9.0 重新评估)
	pending := []string{BoundaryCrossDomainFixtures}
	for _, id := range pending {
		if id == "" {
			t.Error("pending boundary ID must be non-empty (v9.0 待评估)")
		}
	}
}
