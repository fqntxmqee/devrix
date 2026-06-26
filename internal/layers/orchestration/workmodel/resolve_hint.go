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
	if focus.LastRound != nil && focus.LastRound.SpawnPolicy != "" {
		if hint := hintFromSpawnPolicy(focus.LastRound); hint != "" {
			return hint
		}
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
	} else if stats.Total == 0 && focus.Uncertainty >= EffectiveDecomposeThreshold(tm, "") && CanDecompose(focus.Kind) {
		threshold := EffectiveDecomposeThreshold(tm, "")
		parts = append(parts, fmt.Sprintf(
			"Resolve: uncertainty %.2f ≥ %.2f — decompose with task_write mode=decompose (max depth %d).",
			focus.Uncertainty, threshold, maxDepth))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

func hintFromSpawnPolicy(round *WorkItemPipelineRound) string {
	switch round.SpawnPolicy {
	case SpawnAwait:
		return "Resolve: await running children before next pipeline round."
	case SpawnDecompose:
		n := len(round.ChildSpecs)
		if n == 0 {
			return "Resolve: decompose approved — propose child specs via task_write mode=decompose."
		}
		return fmt.Sprintf("Resolve: decompose approved — %d child item(s) pending creation.", n)
	case SpawnParallelExplore:
		return "Resolve: run parallel explore probes (ephemeral) before persisting child work items."
	case SpawnInline:
		return "Resolve: max depth or indeterminate retry — execute remaining work inline."
	case SpawnEscalateHuman:
		return "Resolve: escalate to human review — use /task review approve <id> after inspection."
	case SpawnNone:
		return ""
	default:
		return ""
	}
}
