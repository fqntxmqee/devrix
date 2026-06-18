package workmodel

import (
	"fmt"
	"strings"
)

// ResolveHint builds RunTurn resolve guidance for the current focus (v1.5/v2.0).
func ResolveHint(sessionID string, tm *TaskManager, focus *WorkItem) string {
	if tm == nil || focus == nil {
		return ""
	}
	stats := childOutcomeStats(tm, sessionID, focus.ID)
	pending := stats.Total - stats.Completed - stats.Failed - stats.Running
	var parts []string

	if stats.Running > 0 {
		parts = append(parts, fmt.Sprintf("Resolve: %d child(ren) running — use task_await before spawning duplicates.", stats.Running))
	}
	if pending > 0 && stats.Running == 0 {
		parts = append(parts, fmt.Sprintf("Resolve: %d pending child(ren) — use task_spawn to execute them.", pending))
	}
	if stats.Failed > 0 && stats.Running == 0 {
		parts = append(parts, fmt.Sprintf("Resolve: %d failed child(ren) — review failures before continuing.", stats.Failed))
	}

	depth := tm.Tree().Depth(sessionID, focus.ID)
	maxDepth := tm.Tree().MaxDecomposeDepth()
	if depth >= maxDepth {
		parts = append(parts, "Resolve: max decompose depth reached — execute remaining work inline.")
	} else if stats.Total == 0 && focus.Uncertainty >= DefaultUncertaintyDecomposeThreshold && CanDecompose(focus.Kind) {
		parts = append(parts, fmt.Sprintf(
			"Resolve: uncertainty %.2f ≥ %.2f — decompose with task_write mode=decompose (max depth %d).",
			focus.Uncertainty, DefaultUncertaintyDecomposeThreshold, maxDepth))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}
