package decisionplanning_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: TOOL-SURFACE-1-A01-T29 — PlanModeOpenWorldPolicy: in plan_mode
// + OpenWorld=true, deny unless the name matches the allowlist.
func TestPlanModeOpenWorldPolicy_ApplyWithContext(t *testing.T) {
	cases := []struct {
		name      string
		mode      string
		tool      string
		openWorld bool
		allowList []string
		want      contracts.Decision
	}{
		{"plan + openworld + no allowlist → Deny", "plan_mode", "web_fetch", true, nil, contracts.DecisionDeny},
		{"plan + openworld + exact allowlist match → Allow", "plan_mode", "web_fetch", true, []string{"web_fetch"}, contracts.DecisionAllow},
		{"plan + openworld + wildcard allowlist match → Allow", "plan_mode", "git_status", true, []string{"git_*"}, contracts.DecisionAllow},
		{"plan + openworld + no allowlist match → Deny", "plan_mode", "free_fork", true, []string{"web_fetch"}, contracts.DecisionDeny},
		{"build + openworld → pass", "build_mode", "web_fetch", true, nil, contracts.DecisionAllow},
		{"plan + not openworld → pass", "plan_mode", "read_file", false, nil, contracts.DecisionAllow},
		{"no mode + openworld → pass", "", "web_fetch", true, nil, contracts.DecisionAllow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			if c.mode != "" {
				ctx = context.WithValue(ctx, decisionplanning.ModeKey{}, c.mode)
			}
			p := decisionplanning.NewPlanModeOpenWorldPolicy(c.allowList)
			spec := contracts.ToolSpec{Name: c.tool, OpenWorld: c.openWorld, Risk: types.RiskLevelHigh}
			got := p.ApplyWithContext(ctx, spec, contracts.DecisionAllow)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// T: TOOL-SURFACE-1-A01-T29 — NewPlanModeOpenWorldPolicy normalizes the
// allowlist (trims whitespace, drops empty entries).
func TestNewPlanModeOpenWorldPolicy_Normalizes(t *testing.T) {
	p := decisionplanning.NewPlanModeOpenWorldPolicy([]string{"", "  ", "web_fetch", "\t"})
	if len(p.AllowList) != 1 {
		t.Errorf("normalization: got %d entries, want 1", len(p.AllowList))
	}
	if p.AllowList[0] != "web_fetch" {
		t.Errorf("normalization: got %q, want web_fetch", p.AllowList[0])
	}
}

// T: TOOL-SURFACE-1-T27 — PlanModeOpenWorldPolicy.ShouldDefer runtime defer
// (mirrors ApplyWithContext but answers the LLM-prompt-filter question).
func TestPlanModeOpenWorldPolicy_ShouldDefer(t *testing.T) {
	cases := []struct {
		name      string
		mode      string
		tool      string
		openWorld bool
		allowList []string
		want      bool
	}{
		{"plan + openworld + no allowlist → defer", "plan_mode", "web_fetch", true, nil, true},
		{"plan + openworld + exact match → no defer", "plan_mode", "web_fetch", true, []string{"web_fetch"}, false},
		{"plan + openworld + wildcard match → no defer", "plan_mode", "delegate_research", true, []string{"delegate_*"}, false},
		{"plan + openworld + no match → defer", "plan_mode", "free_fork", true, []string{"web_fetch"}, true},
		{"build + openworld → no defer", "build_mode", "web_fetch", true, nil, false},
		{"plan + not openworld → no defer", "plan_mode", "read_file", false, nil, false},
		{"no mode + openworld → no defer", "", "web_fetch", true, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			if c.mode != "" {
				ctx = context.WithValue(ctx, decisionplanning.ModeKey{}, c.mode)
			}
			p := decisionplanning.NewPlanModeOpenWorldPolicy(c.allowList)
			spec := contracts.ToolSpec{Name: c.tool, OpenWorld: c.openWorld}
			got := p.ShouldDefer(ctx, spec)
			if got != c.want {
				t.Errorf("ShouldDefer = %v, want %v", got, c.want)
			}
		})
	}
}
