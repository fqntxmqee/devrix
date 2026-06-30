package sessionorchestrator

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// DirectiveForItem returns the WorkItemExecutor directive, using rollup synthesis
// when NeedsRollup is set (DM-20260627-001).
func DirectiveForItem(sessionID string, item *workmodel.WorkItem, tm *workmodel.TaskManager) string {
	if item != nil && item.NeedsRollup {
		return buildRollupDirective(sessionID, item, tm)
	}
	return itemDirective(item)
}

func buildRollupDirective(sessionID string, parent *workmodel.WorkItem, tm *workmodel.TaskManager) string {
	if parent == nil {
		return ""
	}
	title := strings.TrimSpace(parent.Title)
	if title == "" {
		title = itemDirective(parent)
	}
	var childLines []string
	// RH-MUPS-04 (DM-20260701-001): track failed children separately so
	// the rollup directive can include a "FailedSubset" section listing
	// each failed child with its verdict reason. Without this, the rollup
	// synthesizer has the failed children's summaries in `childLines` but
	// no structural marker — the LLM is free to downweight or omit them.
	var failedChildLines []string
	if tm != nil && sessionID != "" {
		for _, b := range workmodel.CollectStructuredChildBubbles(tm, sessionID, parent.ID) {
			summary := ""
			verdict := types.VerdictIndeterminate
			reason := ""
			if b.Round != nil {
				summary = strings.TrimSpace(b.Round.ArtifactSummary)
				verdict = b.Round.VerdictKind
				reason = b.Round.ExitReason
			}
			line := formatRollupChildLine(b.ChildID, verdict, summary)
			childLines = append(childLines, line)
			if verdict == types.VerdictFail || verdict == types.VerdictPartial {
				failedChildLines = append(failedChildLines, formatFailedChildLine(b.ChildID, verdict, reason, summary))
			}
		}
		for _, b := range workmodel.CollectChecklistChildBubbles(tm, sessionID, parent.ID) {
			directive := itemDirective(b.Item)
			cv := checklistVerdict(b.Item)
			line := formatRollupChildLine(b.ChildID, cv, directive)
			childLines = append(childLines, line)
			if cv == types.VerdictFail || cv == types.VerdictPartial {
				failedChildLines = append(failedChildLines, formatFailedChildLine(b.ChildID, cv, "", directive))
			}
		}
	}
	body := strings.Join(childLines, "\n")
	if body == "" {
		body = "(no child summaries available)"
	}
	// RH-MUPS-06 (DM-20260701-001 T-P2-3): surface any high-uncertainty
	// child as a separate "UncertainChildren" section so the rollup
	// synthesizer cannot accidentally drop the signal. Without this
	// the parent's only view of the child's high uncertainty was the
	// aggregated mean, and a single high-u child gets washed by
	// confident siblings.
	uncertainLines := []string{}
	if tm != nil && sessionID != "" {
		for _, ub := range workmodel.CollectChildUncertaintyBubbles(tm, sessionID, parent.ID) {
			uncertainLines = append(uncertainLines, workmodel.ChildUncertaintyBubbleStatement(ub))
		}
	}
	expected := workmodel.DefaultChildExpectedReturn(parent, itemDirective(parent))
	var b strings.Builder
	fmt.Fprintf(&b, "ParentGoal: %s\n", title)
	fmt.Fprintf(&b, "ChildOutcomes: %d\n", len(childLines))
	b.WriteString(body)
	if len(uncertainLines) > 0 {
		b.WriteByte('\n')
		fmt.Fprintf(&b, "UncertainChildren: %d\n", len(uncertainLines))
		b.WriteString(strings.Join(uncertainLines, "\n"))
	}
	if len(failedChildLines) > 0 {
		b.WriteByte('\n')
		fmt.Fprintf(&b, "FailedSubset: %d\n", len(failedChildLines))
		b.WriteString(strings.Join(failedChildLines, "\n"))
	}
	if expected != "" {
		b.WriteByte('\n')
		b.WriteString("ExpectedReturn: ")
		b.WriteString(expected)
	}
	return b.String()
}

// formatFailedChildLine renders one failed-child entry for the FailedSubset
// section. Includes verdict + reason (when available) + truncated summary
// so the LLM synthesising the rollup cannot accidentally drop the failure
// from its summary.
func formatFailedChildLine(childID string, verdict types.VerdictKind, reason, summary string) string {
	preview := strings.TrimSpace(summary)
	if preview == "" {
		preview = "(empty)"
	}
	if reason != "" {
		return fmt.Sprintf("- wi_id=%s verdict=%s reason=%q summary=%q",
			childID, verdict, reason, workmodel.TruncateArtifactSummary(preview, 240))
	}
	return fmt.Sprintf("- wi_id=%s verdict=%s summary=%q",
		childID, verdict, workmodel.TruncateArtifactSummary(preview, 240))
}

func formatRollupChildLine(childID string, verdict types.VerdictKind, text string) string {
	preview := strings.TrimSpace(text)
	if preview == "" {
		preview = "(empty)"
	}
	return fmt.Sprintf("- wi_id=%s verdict=%s summary=%q", childID, verdict, preview)
}

func checklistVerdict(item *workmodel.WorkItem) types.VerdictKind {
	if item == nil {
		return types.VerdictIndeterminate
	}
	switch item.Status {
	case workmodel.TaskStatusCompleted:
		return types.VerdictPass
	case workmodel.TaskStatusFailed, workmodel.TaskStatusCancelled:
		return types.VerdictFail
	default:
		return types.VerdictPartial
	}
}

func rollupFailureCriteria() []plan.FailureCriterion {
	return []plan.FailureCriterion{{Field: "exit_code", Op: "eq", Value: 0}}
}
