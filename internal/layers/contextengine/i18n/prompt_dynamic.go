package i18n

import "fmt"

// DynamicBoundaryMarker separates static and dynamic system prompt sections.
const DynamicBoundaryMarker = "<!-- DYNAMIC_CONTENT_BOUNDARY -->"

// SessionContextHeader returns the session context section for the given locale.
func SessionContextHeader(loc Locale) string {
	if loc == LocaleEN {
		return "## Session Context"
	}
	return "## 会话上下文"
}

// EnvInfoHeader returns the environment info section header.
func EnvInfoHeader(loc Locale) string {
	if loc == LocaleEN {
		return "## Environment"
	}
	return "## 运行环境"
}

// EnvInfoBody formats workspace and model lines.
func EnvInfoBody(loc Locale, workDir, model string) string {
	if loc == LocaleEN {
		return fmt.Sprintf("Workspace directory: %s\nModel: %s\n", workDir, model)
	}
	return fmt.Sprintf("工作区目录: %s\n模型: %s\n", workDir, model)
}

// Layer3Header returns the loaded-context section preamble.
func Layer3Header(loc Locale) string {
	if loc == LocaleEN {
		return "## Workspace Files (Injected)\nThe following <loaded_context> was loaded from workspace.\n\n"
	}
	return "## 工作区文件（已注入）\n以下 <loaded_context> 来自工作区加载。\n\n"
}

// MemoryTruncationNotice is appended when long-term memory is truncated in the prompt.
func MemoryTruncationNotice(loc Locale) string {
	if loc == LocaleEN {
		return "\n... (memory truncated — use LongTerm recall or project docs for more) ..."
	}
	return "\n... (记忆已截断 — 更多内容请依赖 LongTerm recall 或项目文档) ..."
}

// CoreTemplateFallback is used when EmbedCoreTemplate is false.
func CoreTemplateFallback(loc Locale) string {
	if loc == LocaleEN {
		return "You are Devrix, a multi-agent development assistant."
	}
	return "你是 Devrix，多智能体开发助手。"
}
