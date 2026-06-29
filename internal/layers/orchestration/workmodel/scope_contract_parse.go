package workmodel

import (
	"encoding/json"
	"regexp"
	"strings"
)

var scopeContractBlockRE = regexp.MustCompile(`(?s)<scope_contract>(.*?)</scope_contract>`)

// GoalScopeContractPlanHint is appended to Goal directives at Plan (D7-S16-A60-T02).
const GoalScopeContractPlanHint = `

Before decomposition, emit a scope contract block:
<scope_contract>
{"goal_statement":"...","in_scope":["..."],"out_of_scope":["..."],"assumptions":[],"open_questions":[],"success_criteria":["..."]}
</scope_contract>
Resolve ambiguity in assumptions[] (not open_questions[]). Pipeline execute must NOT call ask_user_question — unresolved scope becomes child work via decomposition after this round.`

// ParseScopeContractBlock extracts JSON ScopeContract from Execute output.
func ParseScopeContractBlock(content string) (*ScopeContract, bool) {
	m := scopeContractBlockRE.FindStringSubmatch(content)
	if len(m) < 2 {
		return nil, false
	}
	raw := strings.TrimSpace(m[1])
	if raw == "" {
		return nil, false
	}
	var sc ScopeContract
	if err := json.Unmarshal([]byte(raw), &sc); err != nil {
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
			SuccessCriteria: []string{"Changes limited to inferred paths"},
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

var openQuestionsBlockRE = regexp.MustCompile(`(?s)<open_questions>(.*?)</open_questions>`)

func parseOpenQuestionsBlock(content string) []string {
	m := openQuestionsBlockRE.FindStringSubmatch(content)
	if len(m) < 2 {
		return nil
	}
	var out []string
	for _, line := range strings.Split(m[1], "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
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
