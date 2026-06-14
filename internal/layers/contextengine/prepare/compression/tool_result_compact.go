package compression

import (
	"strings"

	"github.com/devrix/devrix/internal/shared/types"
)

const clearedToolResultContent = "[cleared tool result]"

// clearStaleToolResults replaces content of older tool messages while preserving
// tool_call_id metadata so provider APIs keep a valid assistant/tool chain.
func clearStaleToolResults(msgs []types.Message, keepRecent int) ([]types.Message, bool) {
	if keepRecent <= 0 {
		keepRecent = 3
	}
	toolIdx := make([]int, 0, len(msgs))
	for i, m := range msgs {
		if m.Role == types.MessageRoleTool {
			toolIdx = append(toolIdx, i)
		}
	}
	if len(toolIdx) <= keepRecent {
		return msgs, false
	}
	clearUntil := len(toolIdx) - keepRecent
	clearSet := make(map[int]struct{}, clearUntil)
	for _, i := range toolIdx[:clearUntil] {
		clearSet[i] = struct{}{}
	}

	out := make([]types.Message, len(msgs))
	changed := false
	for i, m := range msgs {
		if _, ok := clearSet[i]; !ok {
			out[i] = m
			continue
		}
		if strings.HasPrefix(m.Content, clearedToolResultContent) {
			out[i] = m
			continue
		}
		nm := m
		nm.Content = clearedToolResultContent
		out[i] = nm
		changed = true
	}
	return out, changed
}
