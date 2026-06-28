package orchtypes

// RoutingMode selects ingress routing strategy for ProcessMessage.
// v2.6.0: only RoutingModeLoopFirst is active. RoutingModeRuleOrchestrate
// retired in v6.0.0 (DM-20260629-001); FastPath retired in PR #239 (DM-20260626-009).
type RoutingMode string

const (
	// RoutingModeLoopFirst routes non-command messages to the Turn loop.
	// Wave/Plan are tool-gated inside the loop (Clawcode-aligned harness).
	RoutingModeLoopFirst RoutingMode = "loop_first"
)

func normalizeRoutingMode(mode RoutingMode) RoutingMode {
	if mode == RoutingModeLoopFirst {
		return RoutingModeLoopFirst
	}
	// Unknown / retired routing modes default to loop_first (v6.0.0 forward).
	return RoutingModeLoopFirst
}

// IsLoopFirst reports whether ingress uses loop_first (v6.0.0+: always true
// after rule_orchestrate retirement; retained for backward config compat).
func (c *Config) IsLoopFirst() bool {
	if c == nil {
		return true
	}
	return normalizeRoutingMode(c.RoutingMode) == RoutingModeLoopFirst
}
