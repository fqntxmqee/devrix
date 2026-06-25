package decisionplanning

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// ModeKey is the context key used by turn_adapter to pass the
// current agent mode ("plan_mode" | "build_mode" | ...) to
// PlanModeOpenWorldPolicy.Apply / ShouldDefer.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
type ModeKey struct{}

// PlanModeOpenWorldPolicy denies OpenWorld tools in plan_mode unless
// the tool name matches an allowlist entry. This is the per-tool
// complement of the turn-level "plan_mode" decision (DM-006) — the
// turn-level decision can keep the tool visible but the per-tool
// decision refuses to execute it.
//
// Matches clawcode's shouldAvoidPermissionPrompts (hooks/tools.ts:43-58)
// which uses isOpenWorld() to drop web/agent tools in plan mode.
type PlanModeOpenWorldPolicy struct {
	// AllowList supports exact names ("web_fetch") and glob
	// wildcards ("git_*", "delegate_*"). Empty means deny all
	// OpenWorld tools in plan_mode.
	AllowList []string
}

// NewPlanModeOpenWorldPolicy builds a policy from an allowlist slice.
// Pass nil / empty to deny all OpenWorld tools in plan_mode.
func NewPlanModeOpenWorldPolicy(allowList []string) *PlanModeOpenWorldPolicy {
	out := make([]string, 0, len(allowList))
	for _, e := range allowList {
		e = strings.TrimSpace(e)
		if e != "" {
			out = append(out, e)
		}
	}
	return &PlanModeOpenWorldPolicy{AllowList: out}
}

// ApplyWithContext is the version that consults ctx for the mode.
// Use this from turn_adapter (it owns the ctx). Returns Deny when
// (a) spec.OpenWorld is true, (b) ctx carries mode="plan_mode",
// and (c) the tool name is not in the allowlist (exact or wildcard
// match). Otherwise returns `current` unchanged.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
func (p *PlanModeOpenWorldPolicy) ApplyWithContext(ctx context.Context, spec contracts.ToolSpec, current contracts.Decision) contracts.Decision {
	if !spec.OpenWorld {
		return current
	}
	modeVal := ctx.Value(ModeKey{})
	mode, _ := modeVal.(string)
	if mode != "plan_mode" {
		return current
	}
	for _, allowed := range p.AllowList {
		if spec.Name == allowed {
			return current
		}
		if strings.Contains(allowed, "*") {
			if ok, _ := filepath.Match(allowed, spec.Name); ok {
				return current
			}
		}
	}
	return contracts.DecisionDeny
}

// ShouldDefer is the DeferDecision hook (DM-20260618-003). It mirrors
// ApplyWithContext but answers the LLM-prompt-filter question: should
// this tool's schema be omitted from the default system prompt and
// only surfaced via tool_search? Returns true under the same conditions
// as ApplyWithContext (plan_mode + OpenWorld + not in allowlist).
//
// DSAFT: TOOL-SURFACE-1-A01-F08.
func (p *PlanModeOpenWorldPolicy) ShouldDefer(ctx context.Context, spec contracts.ToolSpec) bool {
	if !spec.OpenWorld {
		return false
	}
	modeVal := ctx.Value(ModeKey{})
	mode, _ := modeVal.(string)
	if mode != "plan_mode" {
		return false
	}
	for _, allowed := range p.AllowList {
		if spec.Name == allowed {
			return false
		}
		if strings.Contains(allowed, "*") {
			if ok, _ := filepath.Match(allowed, spec.Name); ok {
				return false
			}
		}
	}
	return true
}
