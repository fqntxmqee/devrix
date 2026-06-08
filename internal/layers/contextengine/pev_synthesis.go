package contextengine

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/shared/types"
)

func hasSuccessfulToolOutput(results []ToolResult) bool {
	for _, r := range results {
		if r.Error == "" && strings.TrimSpace(r.Output) != "" {
			return true
		}
	}
	return false
}

func summarizeSuccessfulToolResults(results []ToolResult) string {
	var parts []string
	for _, r := range results {
		if r.Error != "" || strings.TrimSpace(r.Output) == "" {
			continue
		}
		parts = append(parts, strings.TrimSpace(r.Output))
	}
	return strings.Join(parts, "\n\n")
}

func truncateSpanAttr(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// buildSynthesisMessages converts tool results into plain text so the synthesis LLM
// call matches the first-round request shape (no tool_calls / tool_call_id history).
func buildSynthesisMessages(base []types.Message, preamble string, results []ToolResult) []types.Message {
	msgs := append([]types.Message{}, base...)
	if trimmed := strings.TrimSpace(preamble); trimmed != "" {
		msgs = append(msgs, types.Message{Role: types.MessageRoleAssistant, Content: trimmed})
	}
	var b strings.Builder
	b.WriteString("以下是工具执行结果，请用自然语言向用户总结回答，不要再调用工具：\n\n")
	for i, r := range results {
		if r.Error != "" {
			fmt.Fprintf(&b, "[工具 %d 失败] %s\n\n", i+1, r.Error)
			continue
		}
		if out := strings.TrimSpace(r.Output); out != "" {
			fmt.Fprintf(&b, "[工具 %d 输出]\n%s\n\n", i+1, out)
		}
	}
	msgs = append(msgs, types.Message{Role: types.MessageRoleUser, Content: b.String()})
	return msgs
}
