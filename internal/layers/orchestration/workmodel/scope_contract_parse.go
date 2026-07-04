package workmodel

import (
	"strings"

	"github.com/devrix/devrix/internal/shared/prompttags"
)

// ParseScopeContractBlock extracts JSON ScopeContract from Execute output.
func ParseScopeContractBlock(content string) (*ScopeContract, bool) {
	sc, ok := prompttags.ExtractOne[ScopeContract](prompttags.TagScopeContract, content)
	if !ok {
		return nil, false
	}
	return &sc, true
}

// ResolveGoalScopeContract derives ScopeContract from Execute content or directive rules.
func ResolveGoalScopeContract(item *WorkItem, directive, executeContent string) *ScopeContract {
	if item == nil || item.Kind != WorkKindGoal {
		return nil
	}
	if sc, ok := ParseScopeContractBlock(executeContent); ok && sc != nil {
		return sc
	}
	if paths := InferScopeInFromDirective(directive); len(paths) > 0 {
		return &ScopeContract{
			GoalStatement: strings.TrimSpace(directive),
			InScope:       paths,
		}
	}
	if qs := parseOpenQuestionsBlock(executeContent); len(qs) > 0 {
		return &ScopeContract{
			GoalStatement: strings.TrimSpace(directive),
			OpenQuestions: qs,
		}
	}
	return nil
}

func parseOpenQuestionsBlock(content string) []string {
	lines, ok := prompttags.ExtractOne[[]string](prompttags.TagOpenQuestions, content)
	if !ok {
		return nil
	}
	return lines
}

// SetScopeContract persists ScopeContract on a WorkItem.
func (m *TaskManager) SetScopeContract(sessionID, workItemID string, sc *ScopeContract) error {
	if m == nil || workItemID == "" {
		return nil
	}
	item, ok := m.GetWorkItem(sessionID, workItemID)
	if !ok || item == nil {
		return errWorkItem("work item not found")
	}
	if err := m.tree.checkMutable(item); err != nil {
		return err
	}
	if sc == nil {
		item.ScopeContract = nil
	} else {
		copy := *sc
		item.ScopeContract = &copy
	}
	m.tree.touch(item)
	m.tree.persistLocked(sessionID)
	return nil
}
