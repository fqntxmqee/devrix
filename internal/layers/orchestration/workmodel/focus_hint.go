package workmodel

import (
	"context"
	"fmt"
	"strings"
)

// FocusHintProvider supplies work-tree focus context for RunTurn (v1.5 hook).
type FocusHintProvider struct {
	Manager *TaskManager
}

// FocusHint returns a short system prompt augmentation for the current focus item.
func (p *FocusHintProvider) FocusHint(_ context.Context, sessionID string) string {
	if p == nil || p.Manager == nil || sessionID == "" {
		return ""
	}
	focus, err := ResolveFocus(sessionID, p.Manager)
	if err != nil || focus == nil {
		return ""
	}
	children := p.Manager.Tree().ListChildren(sessionID, focus.ID)
	var pending, running, done, failed int
	for _, c := range children {
		if c == nil || (c.Kind == WorkKindChecklist && c.Ephemeral) {
			continue
		}
		switch c.Status {
		case TaskStatusCompleted:
			done++
		case TaskStatusFailed, TaskStatusCancelled:
			failed++
		case TaskStatusInProgress:
			running++
		default:
			pending++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Work focus: [%s] %s (status=%s, uncertainty=%.2f).",
		focus.Kind, focus.Title, focus.Status, focus.Uncertainty)
	if len(children) > 0 {
		fmt.Fprintf(&b, " Children: %d pending, %d running, %d done, %d failed.",
			pending, running, done, failed)
	}
	if running > 0 {
		b.WriteString(" Consider task_await for in-progress children before spawning duplicates.")
	}
	return b.String()
}
