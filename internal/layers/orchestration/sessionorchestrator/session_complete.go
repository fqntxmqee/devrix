package sessionorchestrator

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// taskIncompleteUserMessage matches D1 conclusion.TaskIncompleteMessage (DM-20260630-011).
const taskIncompleteUserMessage = "（任务未能完成，AI 未产生有效结论。请重新发起。）"

// buildSessionCompleteEvent assembles the terminal complete EngineEvent for
// RunSessionTurnLoop (DM-20260630-012): rollup deliverable first, quality gate,
// TaskIncomplete when both summary and content classify as bad.
func buildSessionCompleteEvent(
	ctx context.Context,
	sessionID string,
	tm *workmodel.TaskManager,
	lastArtifactSummary string,
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
	}
	if summaryQuality.Kind == SummaryQualityTooShort || summaryQuality.Kind == SummaryQualityInconclusive {
		meta["summary"] = summary
	}
	if isBothSummaryAndFinalBad(summaryQuality.Kind, finalQuality.Kind) {
		content = taskIncompleteUserMessage
		meta["task_incomplete"] = "true"
	} else if hasOpenIncompleteDeliverable(tm, sessionID) {
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
