package coordinator

// RoutingMode selects ingress routing strategy for ProcessMessage.
type RoutingMode string

const (
	// RoutingModeLoopFirst routes non-command messages to the Turn loop.
	// Wave/Plan are tool-gated inside the loop (Clawcode-aligned harness).
	RoutingModeLoopFirst RoutingMode = "loop_first"
	// RoutingModeRuleOrchestrate preserves DM-20260615-004 ingress rules
	// including FastPathThreshold downgrade to OrchestratePath.
	RoutingModeRuleOrchestrate RoutingMode = "rule_orchestrate"
)

func normalizeRoutingMode(mode RoutingMode) RoutingMode {
	switch mode {
	case RoutingModeRuleOrchestrate:
		return RoutingModeRuleOrchestrate
	default:
		return RoutingModeLoopFirst
	}
}

// IsLoopFirst reports whether the configured routing mode dispatches
// ingress through D7-S2-A06 RunTurnLoop (the canonical main path) or
// falls back to the D2 QueryLoop.Run legacy path.
//
// # ⚠️ LEGACY PATH WARNING (DM-20260617-001)
//
// When this returns false, ingress hits D2.QueryLoop.Run, which is
// Deprecated. Returning true is the supported production state and
// the default. Operators who flip RoutingMode to rule_orchestrate
// must accept the deprecation contract: a one-shot `slog.Warn` per
// process, and bumps of
// `d2_query_loop_legacy_invocations_total` on every legacy Run().
// See openspec/specs/d7-orchestration/spec.md "D2 QueryLoop Legacy
// Path Decommission" for the full contract and decommission triggers.
func (c *Config) IsLoopFirst() bool {
	if c == nil {
		return true
	}
	return normalizeRoutingMode(c.RoutingMode) == RoutingModeLoopFirst
}
