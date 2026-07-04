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

// MaybeParentRollup triggers rollup on any decompose/await parent whose direct
// non-checklist children are all terminal but synthesis has not run (CC-3).
func MaybeParentRollup(sessionID string, tm *TaskManager) (*WorkItem, bool) {
	if tm == nil {
		return nil, false
	}
	var best *WorkItem
	bestDepth := -1
	for _, item := range tm.Tree().List(sessionID) {
		if item == nil || item.Ephemeral || item.NeedsRollup {
			continue
		}
		if !parentHadDecomposeSpawn(item) {
			continue
		}
		stats := childOutcomeStatsForParent(tm, sessionID, item.ID)
		if stats.Total == 0 || stats.Running > 0 {
			continue
		}
		depth := tm.Tree().Depth(sessionID, item.ID)
		if depth > bestDepth {
			best = item
			bestDepth = depth
		}
	}
	if best == nil {
		return nil, false
	}
	if err := tm.Tree().SetNeedsRollup(sessionID, best.ID, true); err != nil {
		return nil, false
	}
	if isTerminalStatus(best.Status) {
		if err := tm.Tree().ReopenForRollup(sessionID, best.ID); err != nil {
			return nil, false
		}
	}
	got, ok := tm.GetWorkItem(sessionID, best.ID)
	if !ok {
		return nil, false
	}
	return got, true
}

// MaybeDecomposeParentRollup is retained for callers; delegates to MaybeParentRollup.
func MaybeDecomposeParentRollup(sessionID string, tm *TaskManager) (*WorkItem, bool) {
	return MaybeParentRollup(sessionID, tm)
}

// MaybeSiblingBestEffortRollup fails stuck max-depth siblings and opens rollup
// when at least one sibling completed with a satisfactory deliverable (CC-3).
func MaybeSiblingBestEffortRollup(sessionID, parentID string, tm *TaskManager) bool {
	if tm == nil || parentID == "" {
		return false
	}
	parent, ok := tm.GetWorkItem(sessionID, parentID)
	if !ok || parent == nil || parent.NeedsRollup {
		return false
	}
	if !parentHadDecomposeSpawn(parent) {
		return false
	}
	maxDepth := tm.Tree().MaxDecomposeDepth()
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDecomposeDepth
	}
	hasCompleteSibling := false
	var stuckIDs []string
	for _, c := range tm.Tree().ListChildren(sessionID, parentID) {
		if c == nil || c.Ephemeral || c.Kind == WorkKindChecklist {
			continue
		}
		if c.Status == TaskStatusCompleted && c.LastRound != nil &&
			!DeliverableContinuationRequired(c.LastRound) {
			hasCompleteSibling = true
		}
		if IsInlineRetryExhaustedAtMaxDepth(c, maxDepth) &&
			(c.Status == TaskStatusInProgress || c.Status == TaskStatusPending) {
			stuckIDs = append(stuckIDs, c.ID)
		}
	}
	if !hasCompleteSibling || len(stuckIDs) == 0 {
		return false
	}
	for _, id := range stuckIDs {
		if c, ok := tm.GetWorkItem(sessionID, id); ok && c != nil && c.LastRound != nil {
			c.LastRound.ExitReason = TerminalReasonInlineRetriesExhaustedAtMaxDepth
			_ = tm.Tree().ApplyPipelineRound(sessionID, id, c.LastRound, c.RoundPhase)
		}
		_ = tm.Tree().UpdateStatus(sessionID, id, TaskStatusFailed)
	}
	if err := tm.Tree().SetNeedsRollup(sessionID, parentID, true); err != nil {
		return false
	}
	if isTerminalStatus(parent.Status) {
		_ = tm.Tree().ReopenForRollup(sessionID, parentID)
	}
	return true
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
	if root.LastRound != nil && root.LastRound.StructuredDeliverable != nil {
		if formatted := FormatDeliverablePayloadForIM(root.LastRound.StructuredDeliverable); formatted != "" {
			return formatted
		}
	}
	// CC-U2: best-effort salvage from session work items before raw artifact text.
	if salvaged := SalvageSessionDeliverable(tm, sessionID); salvaged != "" {
		return salvaged
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
