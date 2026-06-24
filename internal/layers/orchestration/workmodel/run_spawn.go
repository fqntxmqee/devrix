package workmodel

import (
	"log/slog"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/runregistry"
)

// SpawnForWorkItem registers a run, attaches run_ref, and wires terminal → WorkItem sync.
func SpawnForWorkItem(sessionID, workItemID, kind string, tm *TaskManager) (runID string, cancel func()) {
	reg := tm.Registry()
	if reg == nil || tm == nil || workItemID == "" {
		return "", func() {}
	}
	runID, cancel = reg.Register(sessionID, workItemID, kind)
	_ = tm.Tree().SetRunRef(sessionID, workItemID, runID)
	if err := tm.Tree().UpdateStatus(sessionID, workItemID, TaskStatusInProgress); err != nil {
		slog.Warn("workmodel: mark in_progress", "work_item", workItemID, "err", err)
	}
	reg.OnTerminal(runID, func(e runregistry.Entry) {
		syncTerminalWithRetry(tm, sessionID, workItemID, e)
	})
	return runID, cancel
}

// CompleteByWorkItem marks the run for a work item terminal (async worker
// completion). reg is the run registry — nil disables the call.
func CompleteByWorkItem(reg *runregistry.Registry, sessionID, workItemID, summary string, runErr error) {
	if reg == nil || workItemID == "" {
		return
	}
	_ = sessionID
	runID, ok := reg.GetByWorkItem(workItemID)
	if !ok {
		return
	}
	status := runregistry.StatusCompleted
	errStr := ""
	if runErr != nil {
		status = runregistry.StatusFailed
		errStr = runErr.Error()
	}
	reg.SetTerminal(runID, status, summary, errStr)
}

func syncTerminalWithRetry(tm *TaskManager, sessionID, workItemID string, e runregistry.Entry) {
	status := mapRunStatus(e.Status)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = tm.Tree().UpdateStatus(sessionID, workItemID, status)
		if lastErr == nil {
			ReevaluateParentAfterChild(sessionID, workItemID, tm)
			return
		}
		time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
	}
	slog.Error("workmodel: terminal sync failed after retries",
		"work_item", workItemID, "run", e.ID, "err", lastErr)
}

func mapRunStatus(s string) TaskStatus {
	switch s {
	case runregistry.StatusCompleted:
		return TaskStatusCompleted
	case runregistry.StatusFailed:
		return TaskStatusFailed
	case runregistry.StatusCancelled:
		return TaskStatusCancelled
	default:
		return TaskStatusInProgress
	}
}
