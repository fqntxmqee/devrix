package workmodel

// PrepareDecomposeSpecs ensures round carries capped child specs and passes I4.
func PrepareDecomposeSpecs(sessionID string, item *WorkItem, round *WorkItemPipelineRound, tm *TaskManager) error {
	if round == nil {
		return errSpawnRoundRequired
	}
	if len(round.ChildSpecs) == 0 {
		round.ChildSpecs = DefaultDecomposeProposer(item, round)
	}
	workDir := ""
	if tm != nil {
		workDir = tm.SessionWorkDir(sessionID)
	}
	round.ChildSpecs = FilterValidatedChildSpecs(item, round.ChildSpecs, workDir)
	if len(round.ChildSpecs) == 0 {
		round.ChildSpecs = DefaultDecomposeProposer(item, round)
		round.ChildSpecs = FilterValidatedChildSpecs(item, round.ChildSpecs, workDir)
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
		if item != nil && !CanDecompose(item.Kind) {
			round.SpawnPolicy = SpawnInline
			round.SpawnRationale = "spawn guard: kind " + string(item.Kind) + " cannot decompose → inline retry"
			return nil
		}
		if err := PrepareDecomposeSpecs(sessionID, item, round, tm); err != nil {
			return err
		}
		_, err := tm.DecomposeChildren(sessionID, item.ID, round.ChildSpecs)
		if err == nil {
			_ = tm.Tree().ResetInlineRetriesAtMaxDepth(sessionID, item.ID)
		}
		return err
	case SpawnEscalateHuman:
		if err := createHumanReviewWorkItem(sessionID, item, round, tm); err != nil {
			return err
		}
		_ = tm.Tree().ResetInlineRetriesAtMaxDepth(sessionID, item.ID)
		return nil
	default:
		if round.RollupSynthRequested && item != nil {
			_ = tm.Tree().SetNeedsRollup(sessionID, item.ID, true)
			_ = tm.Tree().ResetInlineRetriesAtMaxDepth(sessionID, item.ID)
			if item.Status == TaskStatusCompleted || item.Status == TaskStatusFailed {
				_ = tm.Tree().ReopenForRollup(sessionID, item.ID)
			}
		}
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

// pipelineItemNeedsContinuation reports whether the session loop should schedule
// another MUPS round on this WorkItem (DM-20260703-001 CC-1 / D7-S2-A86-T02).
func pipelineItemNeedsContinuation(item *WorkItem) bool {
	if item == nil || item.LastRound == nil || item.Status != TaskStatusInProgress {
		return false
	}
	if item.LastRound.SpawnPolicy == SpawnInline {
		return DeliverableContinuationRequired(item.LastRound)
	}
	return false
}

// GetPipelineFocus selects the next WorkItem for RunSessionTurnLoop (Phase C).
// Pending ready items win; otherwise in_progress items needing inline retry
// or deliverable continuation after SpawnNone stagnation.
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
		if pipelineItemNeedsContinuation(item) {
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
		if item.Kind == WorkKindGoal && item.NeedsRollup && item.ParentID == "" {
			return true
		}
		if !isTerminalStatus(item.Status) {
			return true
		}
	}
	return false
}
