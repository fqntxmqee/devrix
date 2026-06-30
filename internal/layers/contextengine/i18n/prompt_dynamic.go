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

// EscapeArbitratorJSONSchemaHint is the format-hint retry message
// injected by LLMArbitrator.parseWithRetry when the first response
// fails to parse as a JSON object. RH-D2-CC-05 (DM-20260630-013
// T-P2-11.1) moves this from a hard-coded Chinese literal in
// arbitrator.go to a locale-aware i18n key. The "JSON-schema" line is
// the only tactical content the LLM needs; the rest of the prompt
// (system role / context) is already locale-aware.
func EscapeArbitratorJSONSchemaHint(loc Locale) string {
	if loc == LocaleEN {
		return "\n\nMust return JSON: {\"action\":\"Continue|Exit\",\"reason\":\"...\"}"
	}
	return "\n\n必须返回 JSON: {\"action\":\"Continue|Exit\",\"reason\":\"...\"}"
}

// StrategicPlanAppendix is the system-prompt appendix injected after
// the D2-prepared system prompt for the strategic plan proposer
// (sessionorchestrator.LLMStrategicPlanProposer). RH-D7-13
// (DM-20260630-013 T-P2-11.2) lifts these strings from package-level
// `strategicPlanAppendixZH/EN` constants into the i18n package so
// future locale expansion does not require editing orchestration
// code. The schema (execution_mode/scope_in/child_specs/...) is
// kept verbatim to preserve prompt-snapshot test parity.
func StrategicPlanAppendix(loc Locale) string {
	if loc == LocaleEN {
		return "You propose strategic execution plans for an orchestration Plan node.\n" +
			"Return ONLY a JSON object (no markdown):\n" +
			"{\"execution_mode\":\"single|decompose|parallel_probe\",\"scope_in\":[\"path/\"],\"child_specs\":[],\"deliverable_schema\":\"p0_p1_file_line|not_applicable\",\"react_iters_hint\":5,\"rationale\":\"...\"}\n\n" +
			"Rules:\n" +
			"- Use ONLY the directive and observation summary below; do not invent files.\n" +
			"- Prefer execution_mode=single when scope is clear and small enough for one pass.\n" +
			"- For decompose, each child_specs entry needs title, directive_suffix, expected_return; max 2.\n" +
			"- react_iters_hint between 1 and 5."
	}
	return "你是编排 Plan 节点的战略提案助手。仅返回 JSON 对象（不要 markdown）：\n" +
		"{\"execution_mode\":\"single|decompose|parallel_probe\",\"scope_in\":[\"path/\"],\"child_specs\":[],\"deliverable_schema\":\"p0_p1_file_line|not_applicable\",\"react_iters_hint\":5,\"rationale\":\"...\"}\n\n" +
		"规则：\n" +
		"- 只能使用下方 directive 与 Obs 摘要；不要编造未提供的文件列表。\n" +
		"- 范围清晰且可一次完成时优先 execution_mode=single。\n" +
		"- decompose 时 child_specs 每项含 title、directive_suffix、expected_return；最多 2 项。\n" +
		"- react_iters_hint 范围 1-5。"
}
