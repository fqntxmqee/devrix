package i18n

// WorkItemExecuteFieldLabels are machine-readable field headings for Execute-phase
// WorkItem system prompts. Always English — values (directive text) follow user locale.
var WorkItemExecuteFieldLabels = WorkItemExecuteLabels{
	Directive:      "Directive",
	ScopeIn:        "ScopeIn",
	ScopeOut:       "ScopeOut",
	ExpectedReturn: "ExpectedReturn",
}

// WorkItemExecuteLabels holds English field keys for WorkItem execute prompts.
type WorkItemExecuteLabels struct {
	Directive      string
	ScopeIn        string
	ScopeOut       string
	ExpectedReturn string
}

// WorkItemExecuteIntro is the opening line for Execute WorkItem system prompts.
func WorkItemExecuteIntro(loc Locale) string {
	if loc == LocaleEN {
		return "You are executing one WorkItem in a layered work tree."
	}
	return "你正在分层工作树中执行一个 WorkItem。"
}

// WorkItemExecuteOutputHints documents machine-readable output blocks for Execute.
// I/O shape only — no business tactics.
func WorkItemExecuteOutputHints(loc Locale) string {
	if loc == LocaleEN {
		return WorkItemOutputFormatHintsEN
	}
	return WorkItemOutputFormatHintsZH
}

// WorkItemOutputFormatHintsEN is the English Execute output-hints block (tests / default).
const WorkItemOutputFormatHintsEN = `
## Work item output blocks (machine-readable)
- Verifiable conclusion: <conclusion>...</conclusion>
- Residual uncertainty: <open_questions>one question per line</open_questions>
- Scope boundary (optional JSON): <scope_contract>{"goal_statement":"","in_scope":[],"out_of_scope":[],"assumptions":[],"open_questions":[],"success_criteria":[]}</scope_contract>
- Deliverable schema (only when ExpectedReturn or directive already names one): <deliverable_schema>{registered_schema}</deliverable_schema>
- Do not label observations as ObsFact/ObsSignal/ObsDeviation/ObsUncertainty; Observe classifies signals.
`

// WorkItemOutputFormatHintsZH is the Chinese Execute output-hints block.
const WorkItemOutputFormatHintsZH = `
## WorkItem 输出块（机器可读）
- 可验证结论：<conclusion>...</conclusion>
- 剩余不确定性：<open_questions>每行一个问题</open_questions>
- 范围边界（可选 JSON）：<scope_contract>{"goal_statement":"","in_scope":[],"out_of_scope":[],"assumptions":[],"open_questions":[],"success_criteria":[]}</scope_contract>
- 交付物 schema（仅当 ExpectedReturn 或 directive 已指定时）：<deliverable_schema>{registered_schema}</deliverable_schema>
- 不要自行标注 ObsFact/ObsSignal/ObsDeviation/ObsUncertainty；Observe 节点负责分类。
`
