package sessionorchestrator

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// taskIncompleteUserMessage matches D1 conclusion.TaskIncompleteMessage (DM-20260630-011).
const taskIncompleteUserMessage = "（任务未能完成，AI 未产生有效结论。请重新发起。）"

// completeEventSourceObservationalAnswerFastPath marks the terminal complete
// event as derived from the DM-20260706-011 fast-path. The source disables
// the task_incomplete override: a short CatBusiness ObsFact answer (e.g.
// "2×3=6", "巴黎是法国首都") is structurally trustworthy even though it
// falls under the 100-rune too_short threshold. Without this, a correct
// fast-path answer would be replaced by taskIncompleteUserMessage and the
// user would see "❌ 任务未完成" right after the right answer.
const completeEventSourceObservationalAnswerFastPath = "observational_answer_fastpath"

// buildSessionCompleteEvent assembles the terminal complete EngineEvent for
// RunSessionTurnLoop (DM-20260730-012): rollup deliverable first, quality gate,
// TaskIncomplete when both summary and content classify as bad.
//
// `source` records the originator of the terminal content (e.g.
// `observational_answer_fastpath`). When the source is the fast-path, the
// task_incomplete override is suppressed because the answer is structurally
// pre-validated by pickHighStrengthBusinessFact (strength ≥ 0.9, CatBusiness
// ObsFact, no ObsUncertainty) and persisted with VerdictPass — overriding
// it would mask a correct answer as a failure.
func buildSessionCompleteEvent(
	ctx context.Context,
	sessionID string,
	tm *workmodel.TaskManager,
	lastArtifactSummary string,
	source string,
) *contracts.EngineEvent {
	deliverable := strings.TrimSpace(workmodel.ExtractSessionDeliverable(tm, sessionID))
	content := deliverable
	if content == "" {
		content = strings.TrimSpace(lastArtifactSummary)
	}
	if content == "" {
		content = strings.TrimSpace(workmodel.BestEffortSessionSummary(tm, sessionID))
	}
	summary := content
	summaryQuality := EmitLastTextQuality(ctx, sessionID, summary, "")
	finalQuality := ClassifyLastTextQuality(content)

	meta := map[string]string{
		"summary":          summary,
		"summary_quality":  string(summaryQuality.Kind),
		"final_quality":    string(finalQuality.Kind),
		"event_type":       "complete",
		"source":           source,
	}
	if summaryQuality.Kind == SummaryQualityTooShort || summaryQuality.Kind == SummaryQualityInconclusive {
		meta["summary"] = summary
	}
	switch {
	case isBothSummaryAndFinalBad(summaryQuality.Kind, finalQuality.Kind) && source != completeEventSourceObservationalAnswerFastPath:
		// Both summary and final classify as too_short/inconclusive AND the
		// content did NOT come from the fast-path. The fast-path bypasses
		// this because its short CatBusiness ObsFact answer is structurally
		// pre-validated (see package doc for pickHighStrengthBusinessFact).
		content = taskIncompleteUserMessage
		meta["task_incomplete"] = "true"
	case hasOpenIncompleteDeliverable(tm, sessionID):
		meta["task_incomplete"] = "true"
	}
	return &contracts.EngineEvent{
		Type:      "complete",
		Content:   content,
		SessionID: sessionID,
		Metadata:  meta,
	}
}

func isBothSummaryAndFinalBad(summaryQ, finalQ SummaryQualityKind) bool {
	bad := func(k SummaryQualityKind) bool {
		return k == SummaryQualityTooShort || k == SummaryQualityInconclusive
	}
	return bad(summaryQ) && bad(finalQ)
}

func hasOpenIncompleteDeliverable(tm *workmodel.TaskManager, sessionID string) bool {
	if tm == nil {
		return false
	}
	for _, item := range tm.Tree().List(sessionID) {
		if item == nil || item.Ephemeral || workmodel.IsTerminalStatus(item.Status) {
			continue
		}
		if item.LastRound != nil && workmodel.DeliverableContinuationRequired(item.LastRound) {
			return true
		}
	}
	return false
}

func buildUserFacingEscalationSummary(tm *workmodel.TaskManager, sessionID string) string {
	if s := strings.TrimSpace(workmodel.ExtractSessionDeliverable(tm, sessionID)); s != "" {
		return s + "\n\n---\n（Review 未完全通过验证，请核对上述结论或重新发起。）"
	}
	return taskIncompleteUserMessage
}
