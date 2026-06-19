package capture

import (
	"github.com/devrix/devrix/internal/layers/communication/conclusion"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// ComputeCtxPct is a thin shim kept for backward compatibility with any caller
// that still imports the function from D1. The canonical implementation lives
// in shared/contracts (CROSS-A02-T03: cross-layer contract surface must be free of
// D{N}→D{N} imports). D7 turn runtime calls contracts.ComputeCtxPct directly.
// New code should import the shared helper.
func ComputeCtxPct(promptTokens, maxContextTokens int) int {
	return contracts.ComputeCtxPct(promptTokens, maxContextTokens)
}

// buildCompletionSummary delegates to conclusion (S16-A02-F).
func buildCompletionSummary(durationStr, usageStr, model, ctxPctStr string) string {
	return conclusion.BuildCompletionSummary(durationStr, usageStr, model, ctxPctStr)
}
