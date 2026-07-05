// Package verify — resolution_coverage.go
//
// ComputeResolutionCoverage is the orchestrator-facing wrapper that the
// ItemPipelineRunner calls between Execute and Decide (D7-S16-A106-T01).
// The pure compute lives in interfaces.NewResolutionReport; this file
// exists to:
//
//   1. Document the verify-side integration point so callers have a single
//      name to search for instead of reaching into interfaces/ directly.
//   2. Hold the "no Plan.ResolutionStrategies → nil report" safety-net
//      gate that keeps the legacy verdict-based Decide behavior intact when
//      the LLM does not fill the new RC-1 contract.
//   3. Reserve the hook for span attributes (D7-S16-A106-T01 observability:
//      d7.mups.resolution_coverage) — Phase 5 wires the actual emit calls;
//      keeping the constants here so call sites and dashboards agree.
//
// DSAFT: D7-S16-A106-T01..T02 (Phase 2, S4 implementation).
// Change: devrix-d7-uncertainty-resolution-traceability (DM-20260704-006).
package verify

import (
	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// Span attribute names (D7 design §6 Observable metrics table). Emit
// paths land in Phase 5 once the ItemPipelineRunner spans are finalized.
const (
	SpanAttributeResolutionStrategies   = "d7.mups.resolution.strategies"
	SpanAttributeResolutionClaims       = "d7.mups.resolution.claims"
	SpanAttributeResolutionCoverageRatio = "d7.mups.resolution.coverage_ratio"
	SpanAttributeResolutionUnresolvedLen = "d7.mups.resolution.unresolved_count"
)

// ComputeResolutionCoverage builds the Verify → Decide handoff report
// from the Plan-emitted strategies and the Execute-emitted claims.
//
// Returns nil (the "no report" sentinel) when any of the following hold:
//
//   - strategies is empty: Plan did not follow the RC-1 contract; Decide
//     must fall back to its existing verdict-based routing (safety net).
//   - input validation failed: NewResolutionReport rejected an entry; the
//     caller should treat this as "skip report" rather than crash the run.
//
// Otherwise returns *interfaces.ResolutionReport ready to attach to
// WorkItemPipelineRound.ResolutionReport.
//
// Pure function — no side effects, no observability emits (those live in
// the runner). Re-entrant.
func ComputeResolutionCoverage(strategies []interfaces.ResolutionStrategy, claims []interfaces.ResolutionClaim, sessionID, workItemID string, roundNo int) *interfaces.ResolutionReport {
	// Safety-net gate: Plan LLM did not fill the RC-1 contract. Returning
	// nil here preserves the existing Decide path that triggered the
	// break-chain-B bug in the first place — the new pipeline activates
	// only when the LLM starts emitting resolution_strategies[].
	if len(strategies) == 0 {
		return nil
	}

	report, err := interfaces.NewResolutionReport(sessionID, workItemID, roundNo, strategies, claims)
	if err != nil {
		// Validate-on-construct: bad input → drop the report rather than
		// corrupt the round. Callers see nil and Decide falls back to
		// verdict-based routing as if the contract was never filed.
		return nil
	}
	return &report
}
