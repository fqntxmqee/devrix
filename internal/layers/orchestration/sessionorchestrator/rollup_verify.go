package sessionorchestrator

import (
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
func verifyRollupArtifact(art *wavescheduler.Artifact) workmodel.Verdict {
	if art == nil || art.Error != "" || art.ExitCode != 0 {
		return verifyArtifact(art)
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
