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
	if tm != nil && sessionID != "" {
		for _, b := range workmodel.CollectStructuredChildBubbles(tm, sessionID, parent.ID) {
			summary := ""
			verdict := types.VerdictIndeterminate
			if b.Round != nil {
				summary = strings.TrimSpace(b.Round.ArtifactSummary)
				verdict = b.Round.VerdictKind
			}
			childLines = append(childLines, formatRollupChildLine(b.ChildID, verdict, summary))
		}
		for _, b := range workmodel.CollectChecklistChildBubbles(tm, sessionID, parent.ID) {
			directive := itemDirective(b.Item)
			childLines = append(childLines, formatRollupChildLine(b.ChildID, checklistVerdict(b.Item), directive))
		}
	}
	body := strings.Join(childLines, "\n")
	if body == "" {
		body = "(no child summaries available)"
	}
	expected := workmodel.DefaultChildExpectedReturn(parent, itemDirective(parent))
	var b strings.Builder
	fmt.Fprintf(&b, "ParentGoal: %s\n", title)
	fmt.Fprintf(&b, "ChildOutcomes: %d\n", len(childLines))
	b.WriteString(body)
	if expected != "" {
		b.WriteByte('\n')
		b.WriteString("ExpectedReturn: ")
		b.WriteString(expected)
	}
	return b.String()
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
