// Package filter — D2 ToolFilter implementations (DM-20260617-007).
//
// This package holds the policy filters that sit between ToolSurface
// discovery (Tools()) and ToolSurface execution (Execute). The shared
// composition primitives (Composite / Allow / Deny / ApplyFilters) live
// in internal/shared/contracts; this package is the place for filters
// that depend on D2-specific allowlists (per-agent, per-risk).
//
// DSAFT: TOOL-SURFACE-1-A03 (DM-20260617-007 devrix-tool-surface-contract)
package filter

import (
	"github.com/devrix/devrix/internal/shared/contracts"
)

// PerAgentFilter restricts the visible tool set per agent type. The
// allowlist is a static map; the same instance is safe to reuse across
// many filter calls (no mutable state).
//
// Allowlist semantics (from design.md §2.5):
//   - main / fix: no restriction (return input unchanged)
//   - explore: read-only subset (read_file / glob / grep / list_dir)
//   - plan: explore subset + enter/exit_plan_mode
//   - worker: explore subset + edit_file / bash / task_* / todo_write
//   - delegate: delegate_* only
//   - unknown agent type: no restriction (conservative — log + pass)
type PerAgentFilter struct {
	allowlist map[string]map[string]bool
}

// NewPerAgentFilter returns a PerAgentFilter with the default per-agent
// allowlists. Use WithAllowlist to add custom agent types in tests.
func NewPerAgentFilter() *PerAgentFilter {
	return &PerAgentFilter{
		allowlist: defaultAllowlist(),
	}
}

// defaultAllowlist is the canonical D2 per-agent allowlist. Exposed as a
// var so tests can mutate the singleton in-process (each test calls
// NewPerAgentFilter to get a fresh copy with a new map).
func defaultAllowlist() map[string]map[string]bool {
	return map[string]map[string]bool{
		"main":     {}, // empty = pass-through
		"fix":      {}, // empty = pass-through
		"explore":  exploreSubset,
		"plan":     planSubset,
		"worker":   workerSubset,
		"delegate": delegateSubset,
	}
}

var (
	exploreSubset = map[string]bool{
		"read_file": true,
		"glob":      true,
		"grep":      true,
		"list_dir":  true,
	}
	planSubset = merge(exploreSubset, map[string]bool{
		"enter_plan_mode": true,
		"exit_plan_mode":  true,
	})
	workerSubset = merge(exploreSubset, map[string]bool{
		"edit_file":   true,
		"bash":        true,
		"task_create": true,
		"task_get":    true,
		"task_list":   true,
		"task_update": true,
		"todo_write":  true,
	})
	delegateSubset = map[string]bool{
		"delegate_explore":   true,
		"delegate_plan":      true,
		"delegate_implement": true,
		"delegate_status":    true,
	}
)

func merge(base, extra map[string]bool) map[string]bool {
	out := make(map[string]bool, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// WithAllowlist installs a custom allowlist for the given agent type.
// Replaces any existing entry. Intended for tests and D4 extensions.
func (f *PerAgentFilter) WithAllowlist(agentType string, toolNames ...string) *PerAgentFilter {
	if f.allowlist == nil {
		f.allowlist = make(map[string]map[string]bool)
	}
	m := make(map[string]bool, len(toolNames))
	for _, n := range toolNames {
		m[n] = true
	}
	f.allowlist[agentType] = m
	return f
}

// Apply implements contracts.ToolFilter.
func (f *PerAgentFilter) Apply(specs []contracts.ToolSpec, ctx contracts.FilterCtx) []contracts.ToolSpec {
	if ctx.AgentType == "" || ctx.AgentType == "main" || ctx.AgentType == "fix" {
		return specs
	}
	allow, ok := f.allowlist[ctx.AgentType]
	if !ok {
		return specs // unknown agent → pass-through (conservative)
	}
	if len(allow) == 0 {
		return specs // explicit empty = pass-through
	}
	out := make([]contracts.ToolSpec, 0, len(specs))
	for _, s := range specs {
		if allow[s.Name] {
			out = append(out, s)
		}
	}
	return out
}
