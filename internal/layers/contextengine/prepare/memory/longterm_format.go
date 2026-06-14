package memory

import (
	"fmt"
	"strings"
)

// FormatLongTermAppendix renders recalled entries for system prompt injection.
func FormatLongTermAppendix(entries []MemoryEntry, maxTokens int) string {
	if len(entries) == 0 {
		return ""
	}
	maxChars := maxTokens * 4
	if maxChars <= 0 {
		maxChars = 8000
	}
	var b strings.Builder
	b.WriteString("\n\n## 项目记忆（LongTerm）\n")
	used := 0
	for _, e := range entries {
		preview := e.Content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		line := fmt.Sprintf("- [%s] %s\n", e.Topic, preview)
		if used+len(line) > maxChars {
			break
		}
		b.WriteString(line)
		used += len(line)
	}
	return b.String()
}

const memoryTruncationNoticeZH = "\n... (记忆已截断 — 更多内容请依赖 LongTerm recall 或项目文档) ..."

// FormatMemoryContext renders recalled entries for XML memory_context (no outer heading).
func FormatMemoryContext(entries []MemoryEntry, maxTokens int) (string, bool) {
	if len(entries) == 0 {
		return "", false
	}
	maxChars := maxTokens * 4
	if maxChars <= 0 {
		maxChars = 8000
	}
	var b strings.Builder
	truncated := false
	used := 0
	for _, e := range entries {
		preview := e.Content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		line := fmt.Sprintf("- [%s] %s\n", e.Topic, preview)
		if used+len(line) > maxChars {
			truncated = true
			break
		}
		b.WriteString(line)
		used += len(line)
	}
	if truncated {
		b.WriteString(memoryTruncationNoticeZH)
	}
	return b.String(), truncated
}

// ResolveStoreTopic picks a whitelist topic from the user message.
func ResolveStoreTopic(message string, topics []string) string {
	if len(topics) == 0 {
		return ""
	}
	lower := strings.ToLower(message)
	for _, topic := range topics {
		if strings.Contains(lower, topic) {
			return topic
		}
	}
	return topics[0]
}
