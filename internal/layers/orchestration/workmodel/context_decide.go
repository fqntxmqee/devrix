package workmodel

// ApplyPipelineDecide runs ContextGraph + Spawn decide in design §8.3 order (F4).
func ApplyPipelineDecide(
	sessionID string,
	item *WorkItem,
	round *WorkItemPipelineRound,
	props ItemPipelineContextOutput,
	treeCtx TreeEvalContext,
	tm *TaskManager,
) {
	if round == nil || tm == nil || item == nil {
		return
	}
	parent, _ := tm.GetWorkItem(sessionID, item.ParentID)
	bubbleCtx := DefaultContextBubbleEvalContext(item, parent, round, tm, sessionID)
	ApplyContextBubbleDecision(round, props.ContextBubbleSpec, bubbleCtx)

	if FeatureWorkItemContextGraphEnabled() {
		ApplyAcceptedContextLinks(sessionID, item, props.ContextLinkSpecs, tm)
	}

	EvaluateSpawnPolicy(round, treeCtx)
}

// ApplyAcceptedContextLinks evaluates CL0–CL8 proposals and mandatory R2 edges.
func ApplyAcceptedContextLinks(sessionID string, item *WorkItem, specs []ContextLinkSpec, tm *TaskManager) {
	if tm == nil || item == nil {
		return
	}
	existing := tm.ListContextLinks(sessionID)
	parent, _ := tm.GetWorkItem(sessionID, item.ParentID)
	var parentRound *WorkItemPipelineRound
	if parent != nil {
		parentRound = parent.LastRound
	}

	accept := func(rec *ContextLinkRecord) {
		if rec == nil || linkRecordExists(existing, *rec) {
			return
		}
		tm.appendContextLink(sessionID, *rec)
		existing = append(existing, *rec)
		target, ok := tm.GetWorkItem(sessionID, rec.ToWorkItemID)
		if ok && target != nil {
			policy := rec.Kind
			if policy == "" {
				policy = LinkFresh
			}
			_ = tm.Tree().SetContextPolicy(sessionID, rec.ToWorkItemID, policy)
		}
	}

	for _, blockerID := range item.BlockedBy {
		upstream, ok := tm.GetWorkItem(sessionID, blockerID)
		if !ok || upstream == nil {
			continue
		}
		upScope := tm.EnsureContextScope(sessionID, upstream)
		depScope := tm.EnsureContextScope(sessionID, item)
		accept(InferDependencyContextLink(upstream, item, upScope, depScope))
	}

	for _, spec := range specs {
		from, okFrom := tm.GetWorkItem(sessionID, spec.FromWorkItemID)
		to, okTo := tm.GetWorkItem(sessionID, spec.ToWorkItemID)
		if !okFrom || !okTo {
			continue
		}
		ctx := ContextLinkEvalContext{
			SessionID:     sessionID,
			Parent:        parent,
			ParentRound:   parentRound,
			FromItem:      from,
			ToItem:        to,
			FromScope:     tm.EnsureContextScope(sessionID, from),
			ToScope:       tm.EnsureContextScope(sessionID, to),
			ExistingLinks: existing,
		}
		dec := EvaluateContextLinkSpec(spec, ctx)
		if dec.Accepted {
			accept(dec.Record)
		}
	}
}
