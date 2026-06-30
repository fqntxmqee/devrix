package workmodel

import (
	"fmt"

	"github.com/devrix/devrix/internal/shared/types"
)

// ApplyPipelineRound persists pipeline output on a WorkItem (Phase B).
func (t *WorkTree) ApplyPipelineRound(sessionID, itemID string, round *WorkItemPipelineRound, phase RoundPhase) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)
	item, ok := t.items[sessionID][itemID]
	if !ok {
		return fmt.Errorf("work item not found: %s", itemID)
	}
	if err := t.checkMutable(item); err != nil {
		return err
	}
	if round != nil {
		item.LastRound = round
		item.Uncertainty = round.UncertaintyMean
	}
	if phase != "" {
		item.RoundPhase = phase
	}
	t.touch(item)
	t.persistLocked(sessionID)
	return nil
}

// SetRoundPhase updates only the pipeline phase for a WorkItem.
func (t *WorkTree) SetRoundPhase(sessionID, itemID string, phase RoundPhase) error {
	return t.ApplyPipelineRound(sessionID, itemID, nil, phase)
}

// StatusAfterSpawnNone maps a terminal VerdictKind to TaskStatus when spawn is none.
// When schema is applicable, Partial/Pass without a complete deliverable stays InProgress
// so inline retry or rollup can converge (DM-20260630-012).
func StatusAfterSpawnNone(kind types.VerdictKind, schema DeliverableSchema, deliverable DeliverableStatus) TaskStatus {
	if schema != DeliverableSchemaNotApplicable && schema != "" &&
		deliverable != DeliverableStatusNotApplicable {
		if deliverable != DeliverableStatusComplete {
			switch kind {
			case types.VerdictFail:
				return TaskStatusFailed
			default:
				return TaskStatusInProgress
			}
		}
	}
	switch kind {
	case types.VerdictPass, types.VerdictPartial:
		return TaskStatusCompleted
	case types.VerdictFail:
		return TaskStatusFailed
	default:
		return TaskStatusInProgress
	}
}

// DecomposeDailyLimitWouldExceed reports whether adding n decompositions would
// hit the 24h per-kind session limit (TD-WT-05 precursor).
func DecomposeDailyLimitWouldExceed(sessionID string, kind WorkKind, add int) bool {
	return checkDailyDecomposeLimit(sessionID, kind, add) != nil
}
