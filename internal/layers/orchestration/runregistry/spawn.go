package runregistry

import (
	"log/slog"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// SpawnForWorkItem registers a run, attaches run_ref, and wires terminal → WorkItem sync.
func SpawnForWorkItem(sessionID, workItemID, kind string, tm *workmodel.TaskManager) (runID string, cancel func()) {
	if Global == nil || tm == nil || workItemID == "" {
		return "", func() {}
	}
	runID, cancel = Global.Register(sessionID, workItemID, kind)
	_ = tm.Tree().SetRunRef(sessionID, workItemID, runID)
	if err := tm.Tree().UpdateStatus(sessionID, workItemID, workmodel.TaskStatusInProgress); err != nil {
		slog.Warn("runregistry: mark in_progress", "work_item", workItemID, "err", err)
	}
	Global.OnTerminal(runID, func(e Entry) {
		syncTerminalWithRetry(tm, sessionID, workItemID, e)
	})
	return runID, cancel
}

// CompleteByWorkItem marks the run for a work item terminal (async worker completion).
func CompleteByWorkItem(sessionID, workItemID, summary string, runErr error) {
	if Global == nil || workItemID == "" {
		return
	}
	runID, ok := Global.GetByWorkItem(workItemID)
	if !ok {
		return
	}
	status := StatusCompleted
	errStr := ""
	if runErr != nil {
		status = StatusFailed
		errStr = runErr.Error()
	}
	Global.SetTerminal(runID, status, summary, errStr)
	_ = sessionID
}

func syncTerminalWithRetry(tm *workmodel.TaskManager, sessionID, workItemID string, e Entry) {
	status := mapRunStatus(e.Status)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = tm.UpdateStatus(sessionID, workItemID, status)
		if lastErr == nil {
			workmodel.ReevaluateParentAfterChild(sessionID, workItemID, tm)
			return
		}
		time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
	}
	slog.Error("runregistry: terminal sync failed after retries",
		"work_item", workItemID, "run", e.ID, "err", lastErr)
}

func mapRunStatus(s string) workmodel.TaskStatus {
	switch s {
	case StatusCompleted:
		return workmodel.TaskStatusCompleted
	case StatusFailed:
		return workmodel.TaskStatusFailed
	case StatusCancelled:
		return workmodel.TaskStatusCancelled
	default:
		return workmodel.TaskStatusInProgress
	}
}
