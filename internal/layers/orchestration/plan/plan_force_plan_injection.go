// Package plan: ForcePlan Plan-injection helpers (DM-20260707-001 PR-F, T71).
//
// Force-plan is a reputation-driven bypass signal — when a session's
// (α, β) crosses the force_plan_threshold (DM-20260707-001 PR-E T63), the
// next Observe call MUST skip the observational fast-path and route the
// directive through the LLM Planner (instead of the FastReturn gate from
// DM-20260706-011).
//
// The Learn layer persists the force_plan signal in a `map[string]string`
// metadata blob keyed by:
//
//	"force_plan"             = "true" / absent
//	"force_plan_ratio"       = "0.85" (β/(α+β))
//	"force_plan_alpha"       = "12"
//	"force_plan_beta"        = "67"
//	"force_plan_reason"      = "force_plan_threshold_crossed"
//	"force_plan_computed_at" = "2026-07-07T10:30:00Z"
//	"force_plan_session_id"  = "sess_xxx"
//
// PR-F responsibility (T71): inject this metadata into the Plan so the next
// Execute round's pipeline runner can read it and route accordingly. We
// keep the Plan struct append-only (add `Metadata map[string]string`
// nullable field) instead of refactoring, so existing call sites compile
// unchanged.
//
// Boundary: this file does NOT import `mups/learn` (avoids plan→learn cycle)
// — the metadata contract is documented above + tested in
// plan_force_plan_injection_test.go. The Learn-side equivalent is
// `learn.EmitForcePlanMetadata(signal)` which produces the same field names
// (verified by TestEmitMetadataContract_LearnAndPlanMatch, cross-package).
package plan

import "strconv"

// Force-plan metadata field names. Shared contract between this package
// (Plan-side injector/reader) and mups/learn (Learn-side emitter). Must
// stay in sync — divergence causes silent routing bugs.
const (
	ForcePlanMetaKey           = "force_plan"
	ForcePlanMetaRatioKey      = "force_plan_ratio"
	ForcePlanMetaAlphaKey      = "force_plan_alpha"
	ForcePlanMetaBetaKey       = "force_plan_beta"
	ForcePlanMetaReasonKey     = "force_plan_reason"
	ForcePlanMetaComputedAtKey = "force_plan_computed_at"
	ForcePlanMetaSessionIDKey  = "force_plan_session_id"
)

// PlanForcePlanHint is the structured payload carried in Plan.Metadata when
// the force-plan signal was injected. Defined here (not in mups/learn) so
// the Plan package stays free of cross-layer imports. The Learn side reads
// the same fields out of the persisted metadata map.
//
// All fields are populated from the learn.ForcePlanSignal at injection
// time, then frozen for the round's lifetime (immutable value object — the
// pipeline runner should never modify it).
type PlanForcePlanHint struct {
	Triggered  bool
	BetaRatio  float64
	Alpha      int
	Beta       int
	Reason     string
	ComputedAt string
	SessionID  string
}

// ShouldForcePlanFromPlan is the one-liner read-side helper. Returns true
// when the Plan carries force_plan=true in its Metadata (i.e. the next
// Execute round MUST skip the observational fast-path).
//
// Returns false when the Plan is nil, has no Metadata, or lacks the
// force_plan key. This is the canonical "always Plan" default.
func ShouldForcePlanFromPlan(p *Plan) bool {
	if p == nil || p.Metadata == nil {
		return false
	}
	return ShouldForcePlanFromMetadata(p.Metadata)
}

// ShouldForcePlanFromMetadata is the metadata-only reader. Same semantics
// as ShouldForcePlanFromPlan but takes the map directly.
func ShouldForcePlanFromMetadata(metadata map[string]string) bool {
	if metadata == nil {
		return false
	}
	return metadata[ForcePlanMetaKey] == "true"
}

// ReadForcePlanHint extracts the PlanForcePlanHint from a Plan's Metadata
// map. Returns nil when the plan doesn't carry force_plan metadata. Safe
// to call on a nil Plan (returns nil).
//
// The float / int conversions tolerate partial-metadata states — bad input
// maps to zero (Hint reads are best-effort telemetry; malformed ratio is
// not worth failing the round).
func ReadForcePlanHint(p *Plan) *PlanForcePlanHint {
	if p == nil || p.Metadata == nil {
		return nil
	}
	if p.Metadata[ForcePlanMetaKey] != "true" {
		return nil
	}
	return &PlanForcePlanHint{
		Triggered:  true,
		BetaRatio:  parseFloat(p.Metadata[ForcePlanMetaRatioKey]),
		Alpha:      parseInt(p.Metadata[ForcePlanMetaAlphaKey]),
		Beta:       parseInt(p.Metadata[ForcePlanMetaBetaKey]),
		Reason:     p.Metadata[ForcePlanMetaReasonKey],
		ComputedAt: p.Metadata[ForcePlanMetaComputedAtKey],
		SessionID:  p.Metadata[ForcePlanMetaSessionIDKey],
	}
}

// InjectForcePlanHint copies a Plan with the given hint merged into the
// Metadata map. Existing keys are preserved (hint keys take precedence on
// collision). When hint is nil or not Triggered, returns the plan unchanged.
//
// Use this instead of mutating p.Metadata directly to preserve Plan's
// immutable value-object contract.
func InjectForcePlanHint(p Plan, hint *PlanForcePlanHint) Plan {
	if hint == nil || !hint.Triggered {
		return p
	}
	cp := make(map[string]string, len(p.Metadata)+7)
	for k, v := range p.Metadata {
		cp[k] = v
	}
	cp[ForcePlanMetaKey] = "true"
	cp[ForcePlanMetaRatioKey] = strconv.FormatFloat(hint.BetaRatio, 'f', 4, 64)
	cp[ForcePlanMetaAlphaKey] = strconv.Itoa(hint.Alpha)
	cp[ForcePlanMetaBetaKey] = strconv.Itoa(hint.Beta)
	cp[ForcePlanMetaReasonKey] = hint.Reason
	cp[ForcePlanMetaComputedAtKey] = hint.ComputedAt
	cp[ForcePlanMetaSessionIDKey] = hint.SessionID
	p.Metadata = cp
	return p
}

// ClearForcePlanHint returns a copy of the plan with the force_plan
// metadata keys removed. Useful when the signal expires (e.g. a new round
// resets reputation after a successful interaction).
func ClearForcePlanHint(p Plan) Plan {
	if p.Metadata == nil {
		return p
	}
	cp := make(map[string]string, len(p.Metadata))
	touched := false
	for k, v := range p.Metadata {
		if isForcePlanKey(k) {
			touched = true
			continue
		}
		cp[k] = v
	}
	if !touched {
		return p
	}
	p.Metadata = cp
	return p
}

func isForcePlanKey(k string) bool {
	switch k {
	case ForcePlanMetaKey, ForcePlanMetaRatioKey,
		ForcePlanMetaAlphaKey, ForcePlanMetaBetaKey,
		ForcePlanMetaReasonKey, ForcePlanMetaComputedAtKey,
		ForcePlanMetaSessionIDKey:
		return true
	}
	return false
}

// parseFloat / parseInt are the lenient parsers used by ReadForcePlanHint.
// They never error — bad input maps to zero — because Hint reads are
// best-effort telemetry.
func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
