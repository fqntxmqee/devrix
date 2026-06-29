package sessionorchestrator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

var (
	fileLineCitationRE = regexp.MustCompile(`\w[\w./-]*\.(go|py|ts|tsx|js|rs):\d+`)
	userGatePhrases    = []string{
		"awaiting your",
		"awaiting user",
		"reply with your",
		"等待您的",
		"等待你的",
		"before i proceed, i need to clarify",
	}
	userGateToolRE = regexp.MustCompile(`ask_user_question\s*[\({]`)
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
		if reason, _ := art.Metadata["stop_reason"].(string); reason == "max_iters" {
			if calls, _ := art.Metadata["tool_calls"].(int); calls > 0 {
				return workmodel.Verdict{
					Kind:       types.VerdictPartial,
					Reason:     "iteration cap with partial progress",
					SourceID:   id,
					Confidence: 0.55,
				}
			}
		}
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

// verifyArtifactForWorkItem applies pipeline-aware checks so autonomous rounds
// do not Pass on user-gate or scope-only output (which would block decompose).
func verifyArtifactForWorkItem(art *wavescheduler.Artifact, item *workmodel.WorkItem, pl *plan.Plan) workmodel.Verdict {
	v := verifyArtifact(art)
	if art == nil {
		return v
	}
	id := art.TaskID
	if id == "" {
		id = "artifact_unknown"
	}
	if artifactAwaitingUserGate(art) {
		return workmodel.Verdict{
			Kind:       types.VerdictPartial,
			Reason:     "interactive user gate not allowed in pipeline execute",
			SourceID:   id,
			Confidence: 0.85,
		}
	}
	if item != nil && workmodel.CanDecompose(item.Kind) && pl != nil && pl.Kind == plan.ExplorationPlan {
		if isScopeOnlyDeliverable(art, item) {
			return workmodel.Verdict{
				Kind:       types.VerdictPartial,
				Reason:     "scope contract emitted without deliverable; decompose required",
				SourceID:   id,
				Confidence: 0.8,
			}
		}
	}
	return v
}

func artifactAwaitingUserGate(art *wavescheduler.Artifact) bool {
	if used, _ := art.Metadata["used_ask_user_question"].(bool); used {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(art.Summary))
	if userGateToolRE.MatchString(lower) {
		return true
	}
	for _, phrase := range userGatePhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func isScopeOnlyDeliverable(art *wavescheduler.Artifact, item *workmodel.WorkItem) bool {
	if art == nil || item == nil || item.ScopeContract == nil {
		return false
	}
	if fileLineCitationRE.MatchString(art.Summary) {
		return false
	}
	// Scope contract persisted from execute with unresolved questions means
	// the round converged scope only — decompose must continue downstream.
	return item.ScopeContract.HasOpenQuestions()
}

func exitReasonForVerdict(v workmodel.Verdict, sessionID string) string {
	return string(verify.VerdictToExitReason(v, sessionID))
}
