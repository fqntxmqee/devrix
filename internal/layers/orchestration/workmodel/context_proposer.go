package workmodel

import "github.com/devrix/devrix/internal/shared/types"

// ItemPipelineContextOutput holds LLM or default proposer output for Decide (design §8.2).
type ItemPipelineContextOutput struct {
	ContextLinkSpecs  []ContextLinkSpec
	ContextBubbleSpec *ContextBubbleSpec
}

// ContextProposer proposes context links and bubble specs; rules evaluate them (CG3).
type ContextProposer interface {
	ProposeContext(sessionID string, item *WorkItem, round *WorkItemPipelineRound, tm *TaskManager) ItemPipelineContextOutput
}

// DefaultContextProposer is the rule-based baseline when no LLM proposer is wired (F4).
type DefaultContextProposer struct{}

// ProposeContext implements ContextProposer.
func (DefaultContextProposer) ProposeContext(sessionID string, item *WorkItem, round *WorkItemPipelineRound, tm *TaskManager) ItemPipelineContextOutput {
	if item == nil || tm == nil || round == nil {
		return ItemPipelineContextOutput{}
	}
	var specs []ContextLinkSpec
	if item.ParentID != "" {
		parent, ok := tm.GetWorkItem(sessionID, item.ParentID)
		if ok && parent != nil {
			parentRound := parent.LastRound
			for _, sib := range tm.Tree().ListChildren(sessionID, item.ParentID) {
				if sib == nil || sib.ID == item.ID {
					continue
				}
				spec := proposeSiblingShareSummary(item, sib, parentRound)
				if spec != nil {
					specs = append(specs, *spec)
				}
			}
		}
	}
	var bubble *ContextBubbleSpec
	if item.ParentID != "" && round.VerdictKind == types.VerdictFail && IsExploratoryPlanKind(round.PlanKind) {
		bubble = &ContextBubbleSpec{
			TargetWorkItemID: item.ParentID,
			Kind:             BubbleKeyMessages,
			MaxTokens:        512,
			Rationale:        "default_proposer:fail_exploratory",
		}
	}
	return ItemPipelineContextOutput{ContextLinkSpecs: specs, ContextBubbleSpec: bubble}
}

func proposeSiblingShareSummary(a, b *WorkItem, parentRound *WorkItemPipelineRound) *ContextLinkSpec {
	rel := ClassifySiblingRelation(a, b)
	if !AllowsLLMShareSummary(rel) {
		return nil
	}
	if parentRound == nil || parentRound.SpawnPolicy != SpawnDecompose {
		return nil
	}
	from, to := a, b
	if a.Status != TaskStatusCompleted && b.Status == TaskStatusCompleted {
		from, to = b, a
	}
	if from.Status != TaskStatusCompleted || to.Status == TaskStatusCompleted {
		return nil
	}
	return &ContextLinkSpec{
		FromWorkItemID: from.ID,
		ToWorkItemID:   to.ID,
		Kind:           LinkShareSummary,
		MaxTokens:      DefaultShareSummaryMaxTokens,
		Rationale:      "default_proposer:share_summary",
	}
}

// ProposeContextPipelineOutput selects proposer output for the Decide phase.
func ProposeContextPipelineOutput(sessionID string, item *WorkItem, round *WorkItemPipelineRound, tm *TaskManager, proposer ContextProposer) ItemPipelineContextOutput {
	if !FeatureWorkItemContextGraphEnabled() {
		return ItemPipelineContextOutput{}
	}
	if proposer == nil {
		proposer = DefaultContextProposer{}
	}
	return proposer.ProposeContext(sessionID, item, round, tm)
}
