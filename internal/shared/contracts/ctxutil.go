// Package contracts exposes the cross-layer contract surface used by all D{N} layers.
//
// ComputeCtxPct was previously defined in D1 (communication/gateway) but is a pure
// arithmetic helper that D2 PEV/QueryLoop also needs. It lives here so that no D{N}
// has to import another D{N} just to compute a context-window percentage.
//
// Covers: L5-0-0-02  (cross-layer contract surface available for any D)
package contracts

// ComputeCtxPct returns the current prompt tokens as a percentage of the context
// window (0-100, clamped). It returns 0 when either input is non-positive.
//
// D1 summary cards ("ctx: X%") and D2 PEV/QueryLoop complete events both use this
// helper to keep the two sides byte-identical. See DM-20260611-008.
func ComputeCtxPct(promptTokens, maxContextTokens int) int {
	if maxContextTokens <= 0 || promptTokens <= 0 {
		return 0
	}
	pct := promptTokens * 100 / maxContextTokens
	if pct > 100 {
		pct = 100
	}
	return pct
}
