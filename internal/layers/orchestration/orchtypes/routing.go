package orchtypes

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

// IsLoopFirst reports whether ingress uses loop_first (default) or
// rule_orchestrate (FastPath threshold gating + OrchestratePath).
func (c *Config) IsLoopFirst() bool {
	if c == nil {
		return true
	}
	return normalizeRoutingMode(c.RoutingMode) == RoutingModeLoopFirst
}
