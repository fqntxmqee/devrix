package orchtypes

import (
	"regexp"
	"testing"
)

// T: DM-20260629-003 T44 — 4 个 boundary debt 常量存在 (非空)。
func TestBoundaryDecisionConstants_Exist(t *testing.T) {
	cases := []struct {
		name string
		got  string
	}{
		{"BoundaryD2D3ImportBan", BoundaryD2D3ImportBan},
		{"BoundaryD3S5VsD2S18Grayzone", BoundaryD3S5VsD2S18Grayzone},
		{"BoundaryD3S4BudgetSpanInjection", BoundaryD3S4BudgetSpanInjection},
		{"BoundaryD3S6FailFastOnObsNil", BoundaryD3S6FailFastOnObsNil},
	}
	for _, tc := range cases {
		if tc.got == "" {
			t.Errorf("%s = empty, want non-empty boundary-debt ID", tc.name)
		}
	}
}

// T: DM-20260629-003 T44 — 版本号格式 boundary-debt:{name}-v{major}.{minor}。
func TestBoundaryDecisionConstants_VersionFormat(t *testing.T) {
	pattern := regexp.MustCompile(`^boundary-debt:[a-z0-9\-]+-v\d+\.\d+$`)
	cases := []struct {
		name string
		got  string
	}{
		{"BoundaryD2D3ImportBan", BoundaryD2D3ImportBan},
		{"BoundaryD3S5VsD2S18Grayzone", BoundaryD3S5VsD2S18Grayzone},
		{"BoundaryD3S4BudgetSpanInjection", BoundaryD3S4BudgetSpanInjection},
		{"BoundaryD3S6FailFastOnObsNil", BoundaryD3S6FailFastOnObsNil},
	}
	for _, tc := range cases {
		if !pattern.MatchString(tc.got) {
			t.Errorf("%s = %q, want format %q", tc.name, tc.got, pattern.String())
		}
	}
}

// T: DM-20260629-003 T44 — 4 个常量唯一 (无重复 boundary-debt ID)。
func TestBoundaryDecisionConstants_Unique(t *testing.T) {
	seen := map[string]string{
		BoundaryD2D3ImportBan:         "BoundaryD2D3ImportBan",
		BoundaryD3S5VsD2S18Grayzone:   "BoundaryD3S5VsD2S18Grayzone",
		BoundaryD3S4BudgetSpanInjection: "BoundaryD3S4BudgetSpanInjection",
		BoundaryD3S6FailFastOnObsNil:  "BoundaryD3S6FailFastOnObsNil",
	}
	if len(seen) != 4 {
		t.Errorf("expected 4 unique constants, got %d", len(seen))
	}
	for id, name := range seen {
		if id == "" {
			t.Errorf("%s has empty ID", name)
		}
	}
}
