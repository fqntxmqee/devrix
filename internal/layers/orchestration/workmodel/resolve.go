package workmodel

import "errors"

// ReevaluateParentAfterChild updates parent uncertainty and status after a child terminals (AC29, AC43).
func ReevaluateParentAfterChild(sessionID, childID string, tm *TaskManager) {
	if tm == nil || childID == "" {
		return
	}
	child, ok := tm.GetWorkItem(sessionID, childID)
	if !ok || child.ParentID == "" {
		return
	}
	parent, ok := tm.GetWorkItem(sessionID, child.ParentID)
	if !ok {
		return
	}

	stats := childOutcomeStats(tm, sessionID, parent.ID)
	u := ComputeUncertainty(parent, stats, parent.Uncertainty, 0)
	_ = tm.Tree().SetUncertainty(sessionID, parent.ID, u)

	if stats.Running > 0 {
		return
	}
	if parent.Status == TaskStatusPending {
		_ = tm.Tree().UpdateStatus(sessionID, parent.ID, TaskStatusInProgress)
	}
	if stats.Failed > 0 {
		_ = tm.Tree().UpdateStatus(sessionID, parent.ID, TaskStatusFailed)
		return
	}
	if stats.Total > 0 && stats.Completed == stats.Total {
		_ = tm.Tree().UpdateStatus(sessionID, parent.ID, TaskStatusCompleted)
	}
}

func childOutcomeStats(tm *TaskManager, sessionID, parentID string) ChildOutcomeStats {
	var stats ChildOutcomeStats
	for _, c := range tm.Tree().ListChildren(sessionID, parentID) {
		if c == nil || c.Kind == WorkKindChecklist {
			continue
		}
		stats.Total++
		switch c.Status {
		case TaskStatusCompleted:
			stats.Completed++
		case TaskStatusFailed, TaskStatusCancelled:
			stats.Failed++
		case TaskStatusInProgress, TaskStatusPending:
			stats.Running++
		}
	}
	return stats
}

// ResolveFocus returns the current focus work item for RunTurn resolve hook (Phase 1.5).
func ResolveFocus(sessionID string, tm *TaskManager) (*WorkItem, error) {
	if tm == nil {
		return nil, errors.New("task manager nil")
	}
	return tm.Tree().GetFocus(sessionID)
}
