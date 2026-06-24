package workmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/runregistry"
)

// DefaultResolveAwaitTimeout is the per-child blocking await budget at RunTurn start.
const DefaultResolveAwaitTimeout = 600 * time.Second

// ResolveAwaiter blocks until focus running children with run_ref reach terminal.
type ResolveAwaiter struct {
	Manager *TaskManager
	Timeout time.Duration
}

// AwaitRunningChildren waits for in-progress children of the current focus (RunTurn resolve hook).
func (a *ResolveAwaiter) AwaitRunningChildren(ctx context.Context, sessionID string) string {
	if a == nil || a.Manager == nil || sessionID == "" {
		return ""
	}
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = DefaultResolveAwaitTimeout
	}
	focus, err := ResolveFocus(sessionID, a.Manager)
	if err != nil || focus == nil {
		return ""
	}
	children := runningChildrenWithRun(a.Manager, sessionID, focus.ID)
	if len(children) == 0 {
		return ""
	}

	var parts []string
	reg := a.Manager.Registry()
	for _, child := range children {
		runID := child.RunRef
		if runID == "" && reg != nil {
			runID, _ = reg.GetByWorkItem(child.ID)
		}
		if runID == "" {
			continue
		}
		out, err := runregistry.Await(ctx, reg, runID, true, timeout)
		if err != nil {
			parts = append(parts, fmt.Sprintf("%s: await error: %v", child.ID, err))
			continue
		}
		a.ensureSynced(sessionID, child.ID, runID, out)
		parts = append(parts, fmt.Sprintf("%s: %s", child.Title, awaitStatusSummary(out)))
	}

	if len(parts) == 0 {
		return ""
	}
	return "Resolve await: " + strings.Join(parts, "; ") + "."
}

func (a *ResolveAwaiter) ensureSynced(sessionID, workItemID, runID, awaitOut string) {
	if a == nil || a.Manager == nil {
		return
	}
	reg := a.Manager.Registry()
	if reg == nil {
		return
	}
	entry, ok := reg.Get(runID)
	if !ok {
		return
	}
	item, ok := a.Manager.GetWorkItem(sessionID, workItemID)
	if !ok || item.Status != TaskStatusInProgress {
		return
	}
	if !isRunTerminalStatus(entry.Status) {
		return
	}
	syncTerminalWithRetry(a.Manager, sessionID, workItemID, entry)
}

func runningChildrenWithRun(tm *TaskManager, sessionID, parentID string) []*WorkItem {
	var out []*WorkItem
	reg := tm.Registry()
	for _, c := range tm.Tree().ListChildren(sessionID, parentID) {
		if c == nil || (c.Kind == WorkKindChecklist && c.Ephemeral) {
			continue
		}
		if c.Status != TaskStatusInProgress {
			continue
		}
		if c.RunRef != "" {
			out = append(out, c)
			continue
		}
		if reg != nil {
			if _, ok := reg.GetByWorkItem(c.ID); ok {
				out = append(out, c)
			}
		}
	}
	return out
}

func awaitStatusSummary(awaitJSON string) string {
	var raw struct {
		Status  string `json:"status"`
		Summary string `json:"summary"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(awaitJSON), &raw); err != nil {
		return "unknown"
	}
	if raw.Error != "" {
		return raw.Status + " (" + raw.Error + ")"
	}
	if raw.Summary != "" {
		return raw.Status + ": " + raw.Summary
	}
	return raw.Status
}

func isRunTerminalStatus(s string) bool {
	return s == runregistry.StatusCompleted || s == runregistry.StatusFailed || s == runregistry.StatusCancelled
}
