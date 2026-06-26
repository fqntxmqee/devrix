package i18n

import (
	"strings"
	"time"
)

// CompressSystemPrompt is the D7 runCompress LLM system prompt.
func CompressSystemPrompt(loc Locale) string {
	if loc == LocaleEN {
		return "Summarize the following conversation compactly, preserving key decisions, tool outputs, and facts. Keep the summary concise enough to fit within the remaining token budget."
	}
	return "简洁总结以下对话，保留关键决策、工具输出和事实。摘要需足够短以适配剩余 token 预算。"
}

// BuildAutocompactPrompt builds the user prompt for autocompact JSON summarization.
func BuildAutocompactPrompt(loc Locale, conversationSegment string) string {
	var b strings.Builder
	if loc == LocaleEN {
		b.WriteString(`You are a conversation summarizer. Below is the middle segment of a developer-AI conversation.
Summarize ONLY what was explicitly discussed. Do NOT invent any details not present in the input.

Output strict JSON:
{
  "topics": ["list of technical topics discussed"],
  "decisions": ["list of decisions made or actions agreed"],
  "open_items": ["list of unresolved questions or pending tasks"]
}

Rules:
- If unsure about any detail, omit it rather than guess.
- Do NOT mention file paths, code, or tool outputs unless they appear in the input.
- Limit each array to at most 5 items.
- If a category has nothing to report, use an empty array.

Conversation segment:
`)
	} else {
		b.WriteString(`你是会话摘要助手。以下是开发者与 AI 对话的中间片段。
仅总结对话中明确讨论的内容，不要编造输入中不存在的信息。

输出严格 JSON：
{
  "topics": ["讨论过的技术主题"],
  "decisions": ["已做出的决策或达成的行动"],
  "open_items": ["未解决的问题或待办"]
}

规则：
- 不确定的细节直接省略，不要猜测。
- 除非输入中出现，否则不要提及文件路径、代码或工具输出。
- 每个数组最多 5 项。
- 某类无内容时使用空数组。

对话片段：
`)
	}
	b.WriteString(conversationSegment)
	return b.String()
}

// FormatAutocompactSummaryContent renders parsed autocompact JSON for message history.
func FormatAutocompactSummaryContent(loc Locale, topics, decisions, openItems []string, rawFallback string) string {
	if len(topics) == 0 && len(decisions) == 0 && len(openItems) == 0 {
		if rawFallback != "" {
			prefix := "[autocompact summary]\n"
			if loc != LocaleEN {
				prefix = "[autocompact 摘要]\n"
			}
			return prefix + rawFallback
		}
		return ""
	}
	var b strings.Builder
	if loc == LocaleEN {
		b.WriteString("[autocompact summary]\n")
		if len(topics) > 0 {
			b.WriteString("Topics: ")
			b.WriteString(strings.Join(topics, "; "))
			b.WriteString("\n")
		}
		if len(decisions) > 0 {
			b.WriteString("Decisions: ")
			b.WriteString(strings.Join(decisions, "; "))
			b.WriteString("\n")
		}
		if len(openItems) > 0 {
			b.WriteString("Open items: ")
			b.WriteString(strings.Join(openItems, "; "))
			b.WriteString("\n")
		}
	} else {
		b.WriteString("[autocompact 摘要]\n")
		if len(topics) > 0 {
			b.WriteString("主题: ")
			b.WriteString(strings.Join(topics, "; "))
			b.WriteString("\n")
		}
		if len(decisions) > 0 {
			b.WriteString("决策: ")
			b.WriteString(strings.Join(decisions, "; "))
			b.WriteString("\n")
		}
		if len(openItems) > 0 {
			b.WriteString("待办: ")
			b.WriteString(strings.Join(openItems, "; "))
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

// CompactBoundaryMessage is the system message content for a compaction boundary.
func CompactBoundaryMessage(loc Locale) string {
	if loc == LocaleEN {
		return "Conversation compacted"
	}
	return "对话已压缩"
}

// TodoVerificationNudge is returned when todo_write detects missing verification steps.
func TodoVerificationNudge(loc Locale) string {
	if loc == LocaleEN {
		return "You have completed 3+ tasks without a verification step. Consider running a verification agent or using /verify to validate completeness and quality before concluding."
	}
	return "你已连续完成 3 个以上任务但未做验证步骤。建议在结束前运行验证 agent 或使用 /verify 检查完整性与质量。"
}

// WorkspaceGuidanceTitle returns the Layer 2 guidance section heading.
func WorkspaceGuidanceTitle(loc Locale) string {
	if loc == LocaleEN {
		return "## Workspace Guidance"
	}
	return "## 工作区指引"
}

// FormatSessionDate formats today's date for session context.
func FormatSessionDate(loc Locale, t time.Time) string {
	if loc == LocaleEN {
		return t.Format("Monday Jan 2, 2006")
	}
	weekdays := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	return fmtSessionDateZH(weekdays[t.Weekday()], t)
}

func fmtSessionDateZH(weekday string, t time.Time) string {
	return weekday + " " + t.Format("2006年1月2日")
}

// SessionContextBody formats the session context field block (without header).
func SessionContextBody(loc Locale, agentName, date, osName, workDir, sessionID, requestID, model string) string {
	if loc == LocaleEN {
		return fmtSessionEN(agentName, date, osName, workDir, sessionID, requestID, model)
	}
	return fmtSessionZH(agentName, date, osName, workDir, sessionID, requestID, model)
}

func fmtSessionEN(agent, date, osName, workDir, sessionID, requestID, model string) string {
	return "Agent: " + agent + "\nToday's date: " + date + "\nOperating system: " + osName +
		"\nWorkspace directory: " + workDir + "\nSession ID: " + sessionID +
		"\nRequest ID: " + requestID + "\nModel: " + model + "\n"
}

func fmtSessionZH(agent, date, osName, workDir, sessionID, requestID, model string) string {
	return "Agent: " + agent + "\n日期: " + date + "\n操作系统: " + osName +
		"\n工作区目录: " + workDir + "\n会话 ID: " + sessionID +
		"\n请求 ID: " + requestID + "\n模型: " + model + "\n"
}
