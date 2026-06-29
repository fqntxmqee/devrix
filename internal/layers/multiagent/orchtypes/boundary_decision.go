// Package orchtypes — boundary debt decisions for D4 multiagent.
//
// Format `boundary-debt:{name}-v{major}.{minor}` aligns with D2/D3/D7
// (DM-20260629-001/002/003) so a single grep across all governance files
// yields the universe of RESOLVED cross-domain boundaries. Versioning records
// when the decision was made (not when debt was incurred).
//
// DM-20260629-004 PR-7 #5 boundary-decision: 3 D4 boundary debts RESOLVED.
package orchtypes

// D4 Boundary Debt Decisions — 3 entries, all RESOLVED.
//
// See d4-domain.md §Boundary Debt Decisions for the full audit table.
const (
	// BoundaryD4ToD7AgentEventBridge: D4 emits agent.{started,error,terminated,
	// iterating,forked,joined} via FlowEvent (D7 subscriber). Governance resolves
	// after PR-6 locks the 6 literals into orchtypes.EventAgent* constants.
	BoundaryD4ToD7AgentEventBridge = "boundary-debt:d4-to-d7-agent-event-bridge-v1.0"

	// BoundaryD4ToD6EvolutionObserver: D4 emits agent.{forked,joined} +
	// permission_required to evolution/guard/observer for fail-fast and
	// reputation tracking. Governance resolves after PR-6 constant switch.
	BoundaryD4ToD6EvolutionObserver = "boundary-debt:d4-to-d6-evolution-observer-v1.0"

	// BoundaryD4ForbiddenFlowHubPublish: D4 code paths MUST NOT publish to
	// flow.Hub directly (D7 v2.0-b lint gate). Cross-domain emit MUST go
	// through orchtypes.EventAgent* const switch (PR-6).
	BoundaryD4ForbiddenFlowHubPublish = "boundary-debt:d4-forbidden-flow-hub-publish-v2.0"
)

// AllBoundaryDecisions returns the 3 governance constants. Used by the
// boundary_decision_test.go uniqueness check and the d4-domain.md
// §Boundary Debt Decisions audit table.
func AllBoundaryDecisions() []string {
	return []string{
		BoundaryD4ToD7AgentEventBridge,
		BoundaryD4ToD6EvolutionObserver,
		BoundaryD4ForbiddenFlowHubPublish,
	}
}
