package workmodel

// IsDeliverableFormatRollupSynth reports CC-U3 leaf re-synthesis: NeedsRollup is set
// to reform an incomplete findings deliverable, not to synthesize child outcomes.
func IsDeliverableFormatRollupSynth(tm *TaskManager, sessionID string, item *WorkItem) bool {
	if tm == nil || item == nil || !item.NeedsRollup {
		return false
	}
	for _, c := range tm.Tree().ListChildren(sessionID, item.ID) {
		if c == nil || c.Ephemeral || c.Kind == WorkKindChecklist {
			continue
		}
		return false
	}
	if item.LastRound == nil {
		return false
	}
	if item.LastRound.RollupSynthRequested {
		return true
	}
	if !deliverableContinuationRequired(item.LastRound) {
		return false
	}
	contract := item.LastRound.DeliverableContract
	if !contract.ContractApplicable() {
		contract = ExpandLegacySchemaToContract(item.LastRound.DeliverableSchema)
	}
	return contract.Normalized().Structure == DeliverableStructureFindingsJSON
}

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
