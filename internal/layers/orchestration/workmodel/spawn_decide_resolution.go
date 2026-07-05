// Package workmodel — spawn_decide_resolution.go
//
// 4th sub-decision for SpawnPolicyEvaluator (DM-20260704-006).
//
// Closes break-chain B (Plan→Decide) by reading Verify's ResolutionReport
// and forcing SpawnDecompose / SpawnUserGate / fall-through based on the
// 4-state compute. This is the治本 (root-cause fix) the original
// `execution_mode: "decompose"` + `child_specs[]` carrier couldn't provide:
//
//   - AnySubWorktreePending → SpawnDecompose (RC-4a, RC-1 SubWorktreeSpec)
//     Decide builds ChildSpecs from the SubWorktree specs the Verify layer
//     already extracted; the Legacy `DefaultDecomposeProposer` is bypassed
//     because the proposals are bound to specific unresolved ObsIDs.
//
//   - MaxUnresolvedStrength >= DefaultUnresolvedStrengthThreshold (no
//     SubWorktree available) → SpawnUserGate (RC-4b)
//     SpawnApply creates a verify child with tool_filter=["ask_user_question"]
//     so the LLM must surface the question rather than guess.
//
//   - else → fall through to checkVerdictDirection (RC-4c).
//     "current behavior preserved when neither RC-4a nor RC-4b fires"
//
// Order: SpawnPolicyEvaluator calls checkBudget → checkResolutionReport →
// checkRollupGuard → checkVerdictDirection. checkBudget wins (depth/children/daily
// limits are hard caps); checkResolutionReport wins over rollup guard +
// verdict direction (RC-4a/b explicitly override SpawnNone when the report
// shows pending resolution work).
//
// 0-behavior-change for rounds with no ResolutionReport (legacy LLM rounds
// or rollup synth — see item_pipeline.go:556 Phase 2 wiring): the function
// returns (SpawnNone, false) and the chain falls through to checkVerdictDirection.

package workmodel

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// DefaultUnresolvedStrengthThreshold (RC-4b) mirrors interfaces.DefaultUnresolvedStrengthThreshold
// so Decide can compare against the same constant without re-exporting.
//
// We re-declare here (rather than importing interfaces) because workmodel is
// the SoT for SpawnPolicy enum values; importing interfaces would create
// the same cycle the contract package was created to break. The duplication
// is intentional and tested by TestDefaultUnresolvedStrengthThreshold_Matches
// in spawn_decide_resolution_test.go.
//
// If the two ever drift the test will catch it; do NOT delete one without
// updating the other.
const DefaultUnresolvedStrengthThreshold = interfaces.DefaultUnresolvedStrengthThreshold

// checkResolutionReport is the 4th sub-decision (RC-4). Returns
// (SpawnDecompose, true) when the report has at least one UnresolvedObs
// with HasSubWorktree=true (RC-4a), or (SpawnUserGate, true) when the
// largest unresolved strength meets the gate threshold and no sub_worktree
// is available (RC-4b). Otherwise returns (SpawnNone, false) to fall
// through to checkRollupGuard / checkVerdictDirection.
//
// Side effects: when RC-4a fires, this function populates round.ChildSpecs
// from the SubWorktree specs in the unresolved slice. The downstream
// SpawnDecompose branch in ApplySpawnPolicy then sees a non-empty
// round.ChildSpecs and calls tm.DecomposeChildren instead of falling back
// to DefaultDecomposeProposer.
//
// Pre-condition: the round's ResolutionReport has already been built by
// the Verify layer (item_pipeline.go:556-562). When the round pre-dates
// the contract or Plan emitted no ResolutionStrategies, the report is
// nil and this function short-circuits to (SpawnNone, false).
func checkResolutionReport(round *WorkItemPipelineRound, ctx TreeEvalContext) (SpawnPolicy, bool) {
	if round == nil || round.ResolutionReport == nil {
		return SpawnNone, false
	}
	report := *round.ResolutionReport

	// RC-4a: at least one UnresolvedObs has HasSubWorktree=true →
	// SpawnDecompose regardless of verdict direction. The SubWorktree
	// specs came from the upstream Plan's ResolutionStrategy[].SubWorktree;
	// Verify copied them into UnresolvedObs so Decide doesn't have to
	// walk back to Plan.
	if report.AnySubWorktreePending() {
		round.ChildSpecs = buildChildSpecsFromSubWorktrees(report.UnresolvedObs)
		return SpawnDecompose, true
	}

	// RC-4b: high-strength unresolved obs without a SubWorktree → user gate.
	// threshold check uses DefaultUnresolvedStrengthThreshold (0.85) so a
	// safe cold-start fires SpawnUserGate only when the unresolved ObsID
	// is material; low-strength noise (Strength < 0.7) flows through to
	// RC-4c where the verdict-direction logic decides inline vs terminal.
	if report.MaxUnresolvedStrength() >= DefaultUnresolvedStrengthThreshold {
		return SpawnUserGate, true
	}

	// RC-4c: report is empty / low-strength — fall through to default.
	return SpawnNone, false
}

// buildChildSpecsFromSubWorktrees converts the UnresolvedObs slice into
// ChildSpecs ready for tm.DecomposeChildren. Only entries with
// HasSubWorktree=true are emitted (the RC-4a trigger); others are
// irrelevant to the decompose decision because the verifier will
// process them in subsequent rounds.
//
// Phase 4 (D7-S15-A109-T01): when the UnresolvedObs carries the
// originating SubWorktreeSpec (added in interfaces/resolution_contract.go),
// we lift its Title + DirectiveSuffix + ExpectedReturn + ScopeIn so the
// child WorkItem starts with the same directive shape as the Plan's
// child_specs[] carrier (legacy Phase 2 fallback). When the spec is nil
// (Phase 1/2/3 rounds still in flight, or a malformed upstream payload)
// we degrade to a deterministic ObsID-based placeholder so the child
// LLM still has a meaningful directive to work from.
func buildChildSpecsFromSubWorktrees(unresolved []interfaces.UnresolvedObs) []ChildSpec {
	out := make([]ChildSpec, 0, len(unresolved))
	for _, uo := range unresolved {
		if !uo.HasSubWorktree {
			continue
		}
		if uo.SubWorktree != nil && uo.SubWorktree.Title != "" {
			out = append(out, ChildSpec{
				Kind:           WorkKindImplement,
				Title:          uo.SubWorktree.Title,
				Directive:      composeSubWorktreeDirective(uo),
				ScopeIn:        append([]string(nil), uo.SubWorktree.ScopeIn...),
				ExpectedReturn: uo.SubWorktree.ExpectedReturn,
			})
			continue
		}
		// Fallback: ObsID-based placeholder directive. Phase 1/2/3
		// reports without the SubWorktree field set fall here.
		out = append(out, ChildSpec{
			Kind:      WorkKindImplement,
			Title:     "Resolve unresolved ObsID " + uo.ObsID,
			Directive: "Resolve unresolved ObsID " + uo.ObsID +
				" (verify_reason=" + string(uo.Reason) + ", strength=" +
				formatFloat(uo.Strength) + "). Use the SubWorktree spec from the parent Plan's ResolutionStrategy.",
		})
	}
	return out
}

// composeSubWorktreeDirective builds the child WorkItem's directive by
// appending the SubWorktreeSpec.DirectiveSuffix (when present) to a
// context line that anchors the directive to the originating ObsID +
// Reason + Strength. Mirrors the Phase 2 PR-A1 PR-RF directive-suffix
// pattern so legacy `child_specs[]` callers see the same shape.
func composeSubWorktreeDirective(uo interfaces.UnresolvedObs) string {
	header := "Resolve unresolved ObsID " + uo.ObsID +
		" (verify_reason=" + string(uo.Reason) +
		", strength=" + formatFloat(uo.Strength) + ")"
	if uo.SubWorktree == nil || uo.SubWorktree.DirectiveSuffix == "" {
		return header
	}
	return header + "\n\n" + uo.SubWorktree.DirectiveSuffix
}

// formatFloat renders f with up to 3 decimals; used only by the
// ChildSpec.Directive generator for logging-style diagnostics. Empty /
// NaN cases are guarded so the directive stays human-readable.
func formatFloat(f float64) string {
	if f != f { // NaN
		return "0.000"
	}
	return fmt.Sprintf("%.3f", f)
}