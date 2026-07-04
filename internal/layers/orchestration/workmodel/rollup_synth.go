package workmodel

// ApplyRollupSynthTrigger sets NeedsRollup when CC-U3 rollup synthesis was requested.
// Idempotent when NeedsRollup is already true.
func ApplyRollupSynthTrigger(tm *TaskManager, sessionID string, item *WorkItem, round *WorkItemPipelineRound) error {
	if tm == nil || item == nil || round == nil || !round.RollupSynthRequested {
		return nil
	}
	if item.NeedsRollup {
		return nil
	}
	return tm.Tree().SetNeedsRollup(sessionID, item.ID, true)
}
