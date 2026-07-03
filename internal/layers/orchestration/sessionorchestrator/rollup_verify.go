package sessionorchestrator

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// verifyRollupArtifact applies child-outcome gates then shared deliverable contract verify.
func verifyRollupArtifact(art *wavescheduler.Artifact, stats workmodel.ChildOutcomeStats) workmodel.Verdict {
	if art == nil || art.Error != "" || art.ExitCode != 0 {
		return verifyArtifact(art)
	}
	if stats.Total > 0 && stats.Failed == stats.Total {
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     fmt.Sprintf("all %d rollup children failed; refusing Pass", stats.Failed),
			SourceID:   art.TaskID,
			Confidence: 0.95,
		}
	}
	if stats.Total > 0 && stats.Failed > 0 && stats.Running > 0 {
		return workmodel.Verdict{
			Kind:       types.VerdictPartial,
			Reason:     fmt.Sprintf("rollup synthesized with %d failed + %d running children", stats.Failed, stats.Running),
			SourceID:   art.TaskID,
			Confidence: 0.8,
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
	contract := workmodel.RollupDeliverableContract()
	got := workmodel.VerifyDeliverableContract(contract, summary, "")
	switch got.Status {
	case workmodel.DeliverableStatusComplete:
		return workmodel.Verdict{
			Kind:       types.VerdictPass,
			Reason:     summary,
			SourceID:   art.TaskID,
			Confidence: 0.9,
		}
	default:
		reason := got.Reason
		if reason == "" {
			reason = "rollup deliverable contract not satisfied"
		}
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     reason,
			SourceID:   art.TaskID,
			Confidence: 0.85,
		}
	}
}
