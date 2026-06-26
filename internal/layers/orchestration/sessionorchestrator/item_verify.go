package sessionorchestrator

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// verifyArtifact derives a deterministic Verdict from Execute output (Phase B).
// Production may swap in LLM Verifier via ItemPipelineDeps.Verifier later.
func verifyArtifact(art *wavescheduler.Artifact) workmodel.Verdict {
	if art == nil {
		return workmodel.Verdict{
			Kind:       types.VerdictIndeterminate,
			Reason:     "missing artifact",
			Confidence: 0,
		}.WithIndeterminateReason("env_limited")
	}
	id := art.TaskID
	if id == "" {
		id = "artifact_unknown"
	}
	if art.Error != "" || art.ExitCode != 0 {
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     fmt.Sprintf("execute failed: %s", art.Error),
			SourceID:   id,
			Confidence: 0.9,
		}
	}
	switch art.SideEffectStatus {
	case types.SideEffectRolledBack:
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     "side effect rolled back",
			SourceID:   id,
			Confidence: 0.85,
		}
	case types.SideEffectUnknown, types.SideEffectInflight:
		return workmodel.Verdict{
			Kind:       types.VerdictPartial,
			Reason:     "side effect uncertain",
			SourceID:   id,
			Confidence: 0.6,
		}
	default:
		return workmodel.Verdict{
			Kind:       types.VerdictPass,
			Reason:     art.Summary,
			SourceID:   id,
			Confidence: 0.9,
		}
	}
}

func exitReasonForVerdict(v workmodel.Verdict, sessionID string) string {
	return string(verify.VerdictToExitReason(v, sessionID))
}
