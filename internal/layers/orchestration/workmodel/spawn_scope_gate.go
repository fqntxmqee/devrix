package workmodel

// ApplyScopeContractSpawnGate downgrades decompose when Goal has open scope questions.
func ApplyScopeContractSpawnGate(item *WorkItem, round *WorkItemPipelineRound) {
	if item == nil || round == nil {
		return
	}
	if item.Kind != WorkKindGoal || item.ScopeContract == nil {
		return
	}
	if !item.ScopeContract.HasOpenQuestions() {
		return
	}
	if round.SpawnPolicy == SpawnDecompose {
		round.SpawnPolicy = SpawnInline
		round.SpawnRationale = "scope_contract: open_questions block decompose"
	}
}
