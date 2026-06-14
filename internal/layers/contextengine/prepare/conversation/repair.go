package conversation

import "github.com/devrix/devrix/internal/shared/types"

// RepairToolMessageChain removes orphan tool results and incomplete tool rounds
// so provider APIs (e.g. MiniMax) receive a valid assistant/tool_call_id sequence.
func RepairToolMessageChain(msgs []types.Message) []types.Message {
	if len(msgs) == 0 {
		return msgs
	}

	out := make([]types.Message, 0, len(msgs))
	pending := map[string]struct{}{}

	for _, m := range msgs {
		switch m.Role {
		case types.MessageRoleAssistant:
			calls := ToolCallsFromAssistant(m)
			if len(calls) == 0 {
				out = append(out, m)
				continue
			}
			out = append(out, m)
			pending = map[string]struct{}{}
			for _, c := range calls {
				if id := c.ID; id != "" {
					pending[id] = struct{}{}
				}
			}
		case types.MessageRoleTool:
			id := ToolCallIDFromResult(m)
			if id == "" || len(pending) == 0 {
				continue
			}
			if _, ok := pending[id]; !ok {
				continue
			}
			delete(pending, id)
			out = append(out, m)
		case types.MessageRoleUser:
			// Reset pending on user messages so tool results from prior
			// rounds don't "leak through" and get paired with a stale
			// assistant tool call across the user boundary. MiniMax
			// rejects such cross-boundary pairs with error 2013.
			if len(pending) > 0 {
				pending = map[string]struct{}{}
			}
			out = append(out, m)
		default:
			out = append(out, m)
		}
	}

	if len(pending) > 0 {
		return FilterIncompleteToolCalls(out)
	}
	return out
}
