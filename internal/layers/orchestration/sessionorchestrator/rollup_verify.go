package sessionorchestrator

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

const rollupMinSummaryRunes = 500

var rollupPlanningDenylist = []string{
	"parallel explore",
	"我将要",
	"我将",
	"todo_write",
}

// verifyRollupArtifact applies R1-V1-C production heuristic for rollup deliverables.
//
// RH-MUPS-04 (DM-20260701-001): the prior signature was a single-artifact
// heuristic — the rollup gate (ShouldRollupAfterChildren in
// workmodel/rollup_gate.go) admitted any "all non-running" terminal mix
// including Failed==Total, and the verify step looked only at the
// synthesized summary shape. The result: an all-failed child set could
// produce a well-formed rollup summary and the parent would be marked
// Completed. With this signature change, when stats.Total > 0 and
// stats.Failed == stats.Total the verdict MUST be Fail (or Partial when
// some running) regardless of summary shape — failure is not washable.
func verifyRollupArtifact(art *wavescheduler.Artifact, stats workmodel.ChildOutcomeStats) workmodel.Verdict {
	if art == nil || art.Error != "" || art.ExitCode != 0 {
		return verifyArtifact(art)
	}
	// RH-MUPS-04: all children failed → refuse Pass. The rollup product is
	// structurally valid but the children it aggregates are all failures;
	// presenting that as a successful rollup hides the failure from the
	// user. Force Fail and surface the failure count in Reason.
	if stats.Total > 0 && stats.Failed == stats.Total {
		return workmodel.Verdict{
			Kind: types.VerdictFail,
			Reason: fmt.Sprintf("all %d rollup children failed; refusing Pass", stats.Failed),
			SourceID: art.TaskID, Confidence: 0.95,
		}
	}
	// Partial failure: surface a degraded Pass only when at least one
	// child succeeded AND none are running. If some are still running we
	// refuse to commit either way — the rollup is premature.
	if stats.Total > 0 && stats.Failed > 0 && stats.Running > 0 {
		return workmodel.Verdict{
			Kind: types.VerdictPartial,
			Reason: fmt.Sprintf("rollup synthesized with %d failed + %d running children", stats.Failed, stats.Running),
			SourceID: art.TaskID, Confidence: 0.8,
		}
	}
	summary := strings.TrimSpace(art.Summary)
	if summary == "" {
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     "rollup summary empty",
			SourceID:   art.TaskID,
			Confidence: 0.9,
		}
	}
	if utf8.RuneCountInString(summary) < rollupMinSummaryRunes {
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     "rollup summary too short",
			SourceID:   art.TaskID,
			Confidence: 0.85,
		}
	}
	lower := strings.ToLower(summary)
	if !strings.Contains(lower, "p0") && !strings.Contains(lower, "p1") {
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     "rollup summary missing P0/P1 sections",
			SourceID:   art.TaskID,
			Confidence: 0.85,
		}
	}
	for _, phrase := range rollupPlanningDenylist {
		if strings.Contains(lower, phrase) {
			return workmodel.Verdict{
				Kind:       types.VerdictFail,
				Reason:     "rollup summary looks like planning meta: " + phrase,
				SourceID:   art.TaskID,
				Confidence: 0.9,
			}
		}
	}
	return workmodel.Verdict{
		Kind:       types.VerdictPass,
		Reason:     summary,
		SourceID:   art.TaskID,
		Confidence: 0.9,
	}
}
