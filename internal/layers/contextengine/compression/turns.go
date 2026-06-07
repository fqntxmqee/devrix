package compression

import "github.com/devrix/devrix/internal/shared/types"

// splitTurns groups messages by user-role boundaries.
// A turn starts at a user message (inclusive) and ends before the next user message.
func splitTurns(msgs []types.Message) [][]types.Message {
	if len(msgs) == 0 {
		return nil
	}
	var turns [][]types.Message
	var current []types.Message
	for _, m := range msgs {
		if m.Role == types.MessageRoleUser && len(current) > 0 {
			turns = append(turns, current)
			current = nil
		}
		current = append(current, m)
	}
	if len(current) > 0 {
		turns = append(turns, current)
	}
	return turns
}

func flattenTurns(turns [][]types.Message) []types.Message {
	var out []types.Message
	for _, t := range turns {
		out = append(out, t...)
	}
	return out
}
