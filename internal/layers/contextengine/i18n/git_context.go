package i18n

import (
	"fmt"
	"strings"
)

const maxGitStatusTruncSuffixEN = "\n... (truncated — run git status for full output)"
const maxGitStatusTruncSuffixZH = "\n... (已截断 — 运行 git status 查看完整输出)"

// FormatGitStatus renders the git_status dynamic system prompt section.
func FormatGitStatus(loc Locale, branch, status, logLines string, statusTruncated bool) string {
	if branch == "" && status == "" && logLines == "" {
		return ""
	}
	var b strings.Builder
	if loc == LocaleEN {
		b.WriteString("Git snapshot (not updated during the conversation):\n\n")
		if branch != "" {
			fmt.Fprintf(&b, "Current branch: %s\n\n", branch)
		}
		if status == "" {
			b.WriteString("Status:\n(clean)\n\n")
		} else {
			fmt.Fprintf(&b, "Status:\n%s\n\n", status)
		}
		if logLines != "" {
			fmt.Fprintf(&b, "Recent commits:\n%s", logLines)
		}
	} else {
		b.WriteString("Git 快照（对话中不自动更新）：\n\n")
		if branch != "" {
			fmt.Fprintf(&b, "当前分支: %s\n\n", branch)
		}
		if status == "" {
			b.WriteString("状态:\n(干净)\n\n")
		} else {
			fmt.Fprintf(&b, "状态:\n%s\n\n", status)
		}
		if logLines != "" {
			fmt.Fprintf(&b, "最近提交:\n%s", logLines)
		}
	}
	if statusTruncated {
		if loc == LocaleEN {
			b.WriteString(maxGitStatusTruncSuffixEN)
		} else {
			b.WriteString(maxGitStatusTruncSuffixZH)
		}
	}
	return strings.TrimSpace(b.String())
}
