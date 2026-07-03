package workmodel

import (
	"sort"
	"strings"
)

// NeedsRollup marks a WorkItem that must run a synthesize (rollup) MUPS round
// after direct children reach terminal (Path A) or root fallback (Path B).
// DM-20260627-001 / OQ-1.

// RollupGatePolicy selects when a decompose parent may enter rollup (Path A).
type RollupGatePolicy string

const (
	RollupGateAllPass     RollupGatePolicy = "all_pass"
	RollupGateBestEffort  RollupGatePolicy = "best_effort"
	RollupGateMinCoverage RollupGatePolicy = "min_coverage"
)

// RollupGatePolicyFor returns the gate policy for parent WorkItem.
// Phase 1 ships best_effort only; persistence on WorkItem is Phase 2.
func RollupGatePolicyFor(_ *WorkItem) RollupGatePolicy {
	return RollupGateBestEffort
}

// ShouldRollupAfterChildren reports Path A: decompose/await parent whose direct
// non-checklist children satisfy the rollup gate policy.
func ShouldRollupAfterChildren(parent *WorkItem, policy RollupGatePolicy, stats ChildOutcomeStats) bool {
	if parent == nil || parent.NeedsRollup {
		return false
	}
	if stats.Running > 0 || stats.Total == 0 {
		return false
	}
	// DM-20260629-001 / T53: read SpawnPolicy via typed RollupReport
	// envelope instead of scattered parent.LastRound.* access.
	report := NewRollupReportFromRound(parent.ID, parent.LastRound)
	if report == nil {
		return false
	}
	switch report.SpawnPolicy {
	case SpawnDecompose, SpawnAwait:
	default:
		return false
	}
	if policy == "" {
		policy = RollupGateBestEffort
	}
	switch policy {
	case RollupGateAllPass:
		return stats.Failed == 0 && stats.Completed == stats.Total
	case RollupGateMinCoverage:
		// Phase 2: min_coverage threshold not persisted in Phase 1.
		return false
	case RollupGateBestEffort:
		return stats.Completed+stats.Failed == stats.Total
	default:
		return stats.Completed+stats.Failed == stats.Total
	}
}

// HasEphemeralChecklistChildren returns true when parent has ≥1 direct
// ephemeral checklist child (Path B input).
func HasEphemeralChecklistChildren(tm *TaskManager, sessionID, parentID string) bool {
	if tm == nil {
		return false
	}
	for _, c := range tm.Tree().ListChildren(sessionID, parentID) {
		if c != nil && c.Kind == WorkKindChecklist && c.Ephemeral {
			return true
		}
	}
	return false
}

// HasPendingNonChecklistWork reports pending/in_progress non-checklist non-ephemeral children.
func HasPendingNonChecklistWork(tm *TaskManager, sessionID, parentID string) bool {
	if tm == nil {
		return false
	}
	for _, c := range tm.Tree().ListChildren(sessionID, parentID) {
		if c == nil || c.Ephemeral || c.Kind == WorkKindChecklist {
			continue
		}
		if c.Status == TaskStatusPending || c.Status == TaskStatusInProgress {
			return true
		}
	}
	return false
}

// MaybeRootRollupFallback triggers Path B when session would close with root goal,
// spawn none/failed round 1, and checklist children exist (DM-20260627-001).
func MaybeRootRollupFallback(sessionID string, tm *TaskManager) (*WorkItem, bool) {
	if tm == nil {
		return nil, false
	}
	root := sessionRootGoal(tm, sessionID)
	if root == nil || root.NeedsRollup {
		return nil, false
	}
	if root.LastRound == nil {
		return nil, false
	}
	if HasPendingNonChecklistWork(tm, sessionID, root.ID) {
		return nil, false
	}
	if !HasEphemeralChecklistChildren(tm, sessionID, root.ID) {
		return nil, false
	}
	if !rootRollupFallbackEligible(root) {
		return nil, false
	}
	if err := tm.Tree().SetNeedsRollup(sessionID, root.ID, true); err != nil {
		return nil, false
	}
	if err := tm.Tree().ReopenForRollup(sessionID, root.ID); err != nil {
		return nil, false
	}
	got, ok := tm.GetWorkItem(sessionID, root.ID)
	if !ok {
		return nil, false
	}
	return got, true
}

func sessionRootGoal(tm *TaskManager, sessionID string) *WorkItem {
	items := tm.Tree().List(sessionID)
	roots := make([]*WorkItem, 0, len(items))
	for _, item := range items {
		if item != nil && item.Kind == WorkKindGoal && item.ParentID == "" {
			roots = append(roots, item)
		}
	}
	if len(roots) == 0 {
		return nil
	}
	// DM-20260629-001 / T54: deterministic order. Multiple goal roots can
	// exist during multi-goal sessions (rare, but the tree API does not
	// forbid it). Sort by item.ID and return the lexicographically smallest
	// so callers (SessionRootGoal, ExtractSessionDeliverable,
	// MaybeRootRollupFallback) observe a stable choice across restarts.
	// Without this, the choice depends on map iteration order in
	// TaskManager.Tree().List(sessionID) which can change between runs.
	sort.SliceStable(roots, func(i, j int) bool {
		return roots[i].ID < roots[j].ID
	})
	return roots[0]
}

func rootRollupFallbackEligible(root *WorkItem) bool {
	if root == nil {
		return false
	}
	// DM-20260629-001 / T53: read SpawnPolicy via typed RollupReport
	// envelope instead of scattered root.LastRound.* access.
	report := NewRollupReportFromRound(root.ID, root.LastRound)
	if report == nil {
		return false
	}
	switch report.SpawnPolicy {
	case SpawnNone:
		return true
	default:
		return root.Status == TaskStatusFailed
	}
}

// MaybeDecomposeParentRollup triggers rollup on a decomposed parent when all
// direct non-checklist children are terminal but the parent has not yet
// synthesized (Path A extension, RH-D7-05). Prevents session loop exit with
// an await_child parent and no rollup deliverable.
func MaybeDecomposeParentRollup(sessionID string, tm *TaskManager) (*WorkItem, bool) {
	if tm == nil {
		return nil, false
	}
	for _, item := range tm.Tree().List(sessionID) {
		if item == nil || item.Kind != WorkKindGoal || item.ParentID != "" {
			continue
		}
		if item.NeedsRollup {
			continue
		}
		stats := childOutcomeStatsForParent(tm, sessionID, item.ID)
		if stats.Total == 0 || stats.Running > 0 {
			continue
		}
		if !parentHadDecomposeSpawn(item) {
			continue
		}
		if err := tm.Tree().SetNeedsRollup(sessionID, item.ID, true); err != nil {
			continue
		}
		if isTerminalStatus(item.Status) {
			if err := tm.Tree().ReopenForRollup(sessionID, item.ID); err != nil {
				continue
			}
		}
		got, ok := tm.GetWorkItem(sessionID, item.ID)
		if !ok {
			return nil, false
		}
		return got, true
	}
	return nil, false
}

func parentHadDecomposeSpawn(parent *WorkItem) bool {
	if parent == nil || parent.LastRound == nil {
		return false
	}
	switch parent.LastRound.SpawnPolicy {
	case SpawnDecompose, SpawnAwait:
		return true
	default:
		return false
	}
}

func childOutcomeStatsForParent(tm *TaskManager, sessionID, parentID string) ChildOutcomeStats {
	var stats ChildOutcomeStats
	for _, c := range tm.Tree().ListChildren(sessionID, parentID) {
		if c == nil || c.Kind == WorkKindChecklist || c.Ephemeral {
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

func SessionRootGoal(tm *TaskManager, sessionID string) *WorkItem {
	return sessionRootGoal(tm, sessionID)
}

// ExtractSessionDeliverable returns the best post-rollup summary for complete.Content.
func ExtractSessionDeliverable(tm *TaskManager, sessionID string) string {
	root := sessionRootGoal(tm, sessionID)
	if root == nil {
		return ""
	}
	// DM-20260629-001 / T53: read ArtifactSummary via typed RollupReport
	// envelope instead of scattered root.LastRound.* access.
	report := NewRollupReportFromRound(root.ID, root.LastRound)
	if report == nil {
		return ""
	}
	if s := strings.TrimSpace(report.ArtifactSummary); s != "" {
		return s
	}
	return bestEffortChildSummaries(tm, sessionID, root.ID)
}

// BestEffortSessionSummary aggregates child artifact summaries when the root
// goal has no rollup deliverable yet (RH-D7-05).
func BestEffortSessionSummary(tm *TaskManager, sessionID string) string {
	root := sessionRootGoal(tm, sessionID)
	if root == nil {
		return ""
	}
	return bestEffortChildSummaries(tm, sessionID, root.ID)
}

func bestEffortChildSummaries(tm *TaskManager, sessionID, parentID string) string {
	if tm == nil {
		return ""
	}
	var parts []string
	for _, c := range tm.Tree().ListChildren(sessionID, parentID) {
		if c == nil || c.LastRound == nil {
			continue
		}
		s := strings.TrimSpace(c.LastRound.ArtifactSummary)
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n")
}
