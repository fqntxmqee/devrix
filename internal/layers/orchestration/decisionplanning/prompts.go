package decisionplanning

import "github.com/devrix/devrix/internal/layers/orchestration/orchtypes"

// loopFirstSystemPrompt is appended to the Turn loop system prompt when
// routing_mode=loop_first. It mirrors Clawcode harness guidance: answer
// directly when possible; use tools only when planning or parallel work is needed.
const loopFirstSystemPrompt = `
## Routing guidance (loop-first harness)

- Greetings and short conversational messages: reply directly in text. Do NOT call delegate_wave or enter_plan_mode.
- Use enter_plan_mode when the implementation approach is genuinely ambiguous and user approval is needed before coding.
- Use delegate_wave for multi-step goals that benefit from task-graph decomposition and parallel workers (investigation + implementation across many files).
- Prefer answering yourself when the request is clear and can be handled in this turn.
`

// TurnSystemPrompt appends loop-first routing guidance to the turn system prompt.
func TurnSystemPrompt(cfg *orchtypes.Config, extra string) string {
	if cfg == nil || !cfg.IsLoopFirst() {
		return extra
	}
	if extra == "" {
		return loopFirstSystemPrompt
	}
	return extra + "\n" + loopFirstSystemPrompt
}
