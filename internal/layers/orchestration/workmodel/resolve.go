package workmodel

import "errors"

// ReevaluateParentAfterChild updates parent uncertainty and status after a child terminals (AC29, AC43).
// Returns the *RollupReport aggregated from the child's last round so callers
// that need the typed envelope can read it; existing callers that ignore the
// return value remain unaffected.
//
// DM-20260629-001 / PR-3-extended / T53: signature migrated from `func()`
// to `func() *RollupReport`. Backward-compatible: 3 call sites in
// session_turn_loop.go / run_spawn.go / cli_commands.go already
// discard the return value; the typed envelope is available for future
// callers that want the aggregated rollup signal.
//
// TD-WT-06: serializes concurrent child terminal updates per parent via TaskManager lock.
func ReevaluateParentAfterChild(sessionID, childID string, tm *TaskManager) *RollupReport {
	if tm == nil || childID == "" {
		return nil
	}
	child, ok := tm.GetWorkItem(sessionID, childID)
	if !ok || child.ParentID == "" {
		return nil
	}
	mu := tm.parentReevalLock(sessionID, child.ParentID)
	mu.Lock()
	defer mu.Unlock()
	return reevaluateParentAfterChild(sessionID, childID, tm)
}

func reevaluateParentAfterChild(sessionID, childID string, tm *TaskManager) *RollupReport {
	child, ok := tm.GetWorkItem(sessionID, childID)
	if !ok || child.ParentID == "" {
		return nil
	}
	parent, ok := tm.GetWorkItem(sessionID, child.ParentID)
	if !ok {
		return nil
	}

	stats := childOutcomeStats(tm, sessionID, parent.ID)
	// RH-MUPS-02 (DM-20260701-001): route through ReconcileUncertainty
	// instead of ComputeUncertainty so both write paths (item_pipeline.go
	// after a round + this reevaluate path after a child terminals) share
	// a single semantic entry point. The prior split — ComputeUncertainty
	// here (replace semantics, may drop on all-pass) vs naked-max ratchet
	// in item_pipeline.go (never drop) — created a write race whose
	// outcome depended on call order. With ReconcileUncertainty both paths
	// apply the same convergence contract; the persisted value no longer
	// ratchets up regardless of which path fires last.
	u := ReconcileUncertainty(parent.Uncertainty, parent.Uncertainty, stats)
	_ = tm.Tree().SetUncertainty(sessionID, parent.ID, u)

	if stats.Running > 0 {
		return NewRollupReportFromRound(childID, child.LastRound)
	}
	if parent.Status == TaskStatusPending {
		_ = tm.Tree().UpdateStatus(sessionID, parent.ID, TaskStatusInProgress)
	}
	if ShouldRollupAfterChildren(parent, RollupGatePolicyFor(parent), stats) {
		_ = tm.Tree().SetNeedsRollup(sessionID, parent.ID, true)
		if isTerminalStatus(parent.Status) {
			_ = tm.Tree().ReopenForRollup(sessionID, parent.ID)
		}
		return NewRollupReportFromRound(childID, child.LastRound)
	}
	if stats.Failed > 0 {
		_ = tm.Tree().UpdateStatus(sessionID, parent.ID, TaskStatusFailed)
		return NewRollupReportFromRound(childID, child.LastRound)
	}
	if stats.Total > 0 && stats.Completed == stats.Total {
		_ = tm.Tree().UpdateStatus(sessionID, parent.ID, TaskStatusCompleted)
	}
	return NewRollupReportFromRound(childID, child.LastRound)
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
