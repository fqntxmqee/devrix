package workmodel

// DefaultDecomposeProposer fills ChildSpecs when the rule engine selects
// SpawnDecompose but no LLM proposer ran yet (Phase C baseline, OQ-3).
func DefaultDecomposeProposer(item *WorkItem, round *WorkItemPipelineRound) []ChildSpec {
	if item == nil || round == nil {
		return nil
	}
	exploratory := IsExploratoryPlanKind(round.PlanKind)
	kind := ChildKindForHypothesis(exploratory)
	base := itemDirectiveForProposer(item)
	return []ChildSpec{
		{Kind: kind, Title: "explore path A", Directive: base + " — hypothesis A", ExpectedReturn: "Evidence comparing hypothesis A"},
		{Kind: kind, Title: "explore path B", Directive: base + " — hypothesis B", ExpectedReturn: "Evidence comparing hypothesis B"},
	}
}

func itemDirectiveForProposer(item *WorkItem) string {
	if item == nil {
		return ""
	}
	if d := item.Directive; d != "" {
		return d
	}
	return item.Title
}

// PrepareDecomposeSpecs ensures round carries capped child specs and passes I4.
func PrepareDecomposeSpecs(item *WorkItem, round *WorkItemPipelineRound) error {
	if round == nil {
		return errSpawnRoundRequired
	}
	if len(round.ChildSpecs) == 0 {
		round.ChildSpecs = DefaultDecomposeProposer(item, round)
	}
	round.ChildSpecs = CapChildSpecs(round.ChildSpecs)
	return ValidateSpawnDecompose(round)
}

// ApplySpawnPolicy executes spawn side effects after SpawnPolicyEvaluator (Phase C).
func ApplySpawnPolicy(sessionID string, item *WorkItem, round *WorkItemPipelineRound, tm *TaskManager) error {
	if round == nil || tm == nil {
		return nil
	}
	switch round.SpawnPolicy {
	case SpawnDecompose:
		if err := PrepareDecomposeSpecs(item, round); err != nil {
			return err
		}
		_, err := tm.DecomposeChildren(sessionID, item.ID, round.ChildSpecs)
		return err
	case SpawnEscalateHuman:
		return createHumanReviewWorkItem(sessionID, item, round, tm)
	default:
		return nil
	}
}

// createHumanReviewWorkItem opens a verify child for human gate (TD-WT-05).
func createHumanReviewWorkItem(sessionID string, item *WorkItem, round *WorkItemPipelineRound, tm *TaskManager) error {
	if tm == nil || item == nil || round == nil {
		return nil
	}
	directive := round.SpawnRationale
	if item.Directive != "" {
		directive = round.SpawnRationale + "\n\nContext: " + item.Directive
	}
	_, err := tm.CreateWorkItem(sessionID, CreateWorkItemInput{
		ParentID:  item.ID,
		Kind:      WorkKindVerify,
		Title:     HumanReviewItemTitle,
		Directive: directive,
		Policy:    ExecPolicyReadonly,
	})
	if err != nil {
		return err
	}
	_ = tm.Tree().SetRoundPhase(sessionID, item.ID, RoundPhaseAwaitChild)
	return nil
}

// GetPipelineFocus selects the next WorkItem for RunSessionTurnLoop (Phase C).
// Pending ready items win; otherwise in_progress items awaiting SpawnInline retry.
func (t *WorkTree) GetPipelineFocus(sessionID string) (*WorkItem, error) {
	if t == nil {
		return nil, nil
	}
	if focus, err := t.GetFocus(sessionID); focus != nil || err != nil {
		return focus, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	t.ensureSessionLocked(sessionID)
	for _, item := range t.items[sessionID] {
		if item == nil || item.Ephemeral || isTerminalStatus(item.Status) {
			continue
		}
		if item.Status == TaskStatusInProgress && item.LastRound != nil &&
			item.LastRound.SpawnPolicy == SpawnInline {
			return item, nil
		}
	}
	return nil, nil
}

// HasOpenWork reports whether the session still has non-terminal work items.
func (t *WorkTree) HasOpenWork(sessionID string) bool {
	if t == nil {
		return false
	}
	if focus, _ := t.GetPipelineFocus(sessionID); focus != nil {
		return true
	}
	for _, item := range t.List(sessionID) {
		if item == nil || item.Ephemeral {
			continue
		}
		if !isTerminalStatus(item.Status) {
			return true
		}
	}
	return false
}
