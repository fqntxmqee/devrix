package workmodel

import "math"

// ReconcileUncertainty merges the previous stored uncertainty value, the
// current pipeline round signal (typically ComputeUnifiedUncertainty), and
// the child outcome stats into a single converged value.
//
// Background (RH-MUPS-01/02, DM-20260701-001): before this function the
// pipeline used a naked max ratchet (item.Uncertainty > uncertaintyMean ?
// item.Uncertainty : uncertaintyMean) which made convergence numerically
// invisible — even after all children passed and rollup succeeded, the
// stored value was pinned at the historical maximum. A second writer
// (reevaluateParentAfterChild, workmodel/resolve.go) wrote a different
// signal via SetUncertainty with replace semantics, creating a write
// race where the final value depended on call order.
//
// ReconcileUncertainty is the SINGLE semantic entry point for setting
// item.Uncertainty. Two properties guarantee convergence visibility:
//
//  1. When all children are terminal (Running == 0 and Total > 0), the
//     child-outcome signal dominates via a weighted blend (0.7 round + 0.3
//     historical). prevStored is NOT used here, so a previously-stuck
//     high value cannot ratchet the converged value up.
//
//  2. When children are still in flight, we conservatively take the
//     higher of the current round signal and a damped historical (50%
//     prevStored + 50% hist) — this prevents single-round optimism from
//     collapsing uncertainty while children are still running, but does
//     NOT lock prevStored at the top.
//
// The function is pure (no globals, no IO) so it is exhaustively
// table-testable. All callers MUST go through this function rather than
// touching item.Uncertainty directly.
func ReconcileUncertainty(prevStored, roundSignal float64, stats ChildOutcomeStats) float64 {
	hist := historicalUncertainty(stats)
	if stats.Total > 0 && stats.Running == 0 {
		// Terminal state on every child: convergence owns the value.
		// 70% round signal + 30% child-historical — the round signal
		// (MUPS reputation + Wilson + verdict confidence) reflects the
		// most recent evidence; the historical anchors the children's
		// terminal mix.
		return clamp01(0.7*clamp01(roundSignal) + 0.3*hist)
	}
	// Still in flight: conservative — use higher of current round vs
	// damped historical. The damping (50% prev + 50% hist) ensures
	// single-round optimism cannot unilaterally drive the value down.
	damped := 0.5*clamp01(prevStored) + 0.5*hist
	return clamp01(math.Max(clamp01(roundSignal), damped))
}

// ReconcileUncertaintyFromChildStats is the parent-rollup writer used by
// ReevaluateParentAfterChild. Unlike the full pipeline writer, it has no fresh
// MUPS round signal, so the child outcome distribution itself becomes the
// current signal. Passing prevStored as both previous and current input would
// recreate a hidden ratchet and make all-pass child convergence less visible.
func ReconcileUncertaintyFromChildStats(prevStored float64, stats ChildOutcomeStats) float64 {
	return ReconcileUncertainty(prevStored, historicalUncertainty(stats), stats)
}
