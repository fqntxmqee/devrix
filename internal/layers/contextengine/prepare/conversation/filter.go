package conversation

import "github.com/devrix/devrix/internal/shared/types"

// FilterIncompleteToolCalls drops a trailing assistant tool_use block missing tool results.
func FilterIncompleteToolCalls(msgs []types.Message) []types.Message {
	if len(msgs) == 0 {
		return msgs
	}
	lastAssistantIdx := -1
	pending := map[string]struct{}{}
	for i, m := range msgs {
		if m.Role == types.MessageRoleAssistant {
			calls := ToolCallsFromAssistant(m)
			if len(calls) == 0 {
				continue
			}
			lastAssistantIdx = i
			pending = map[string]struct{}{}
			for _, c := range calls {
				id := c.ID
				if id == "" {
					continue
				}
				pending[id] = struct{}{}
			}
		}
		if m.Role == types.MessageRoleTool {
			delete(pending, ToolCallIDFromResult(m))
		}
	}
	if lastAssistantIdx >= 0 && len(pending) > 0 {
		return append([]types.Message(nil), msgs[:lastAssistantIdx]...)
	}
	return append([]types.Message(nil), msgs...)
}
