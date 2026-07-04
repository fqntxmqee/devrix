package i18n

// ObservationTaskAppendix returns the Observe-node JSON schema appendix (DM-20260704-001).
// Migrated from sessionorchestrator/llm_observation_proposer.go.
func ObservationTaskAppendix(loc Locale) string {
	if loc == LocaleEN {
		return observationTaskAppendixEN
	}
	return observationTaskAppendixZH
}

const observationTaskAppendixZH = `你是编排 Observe 节点的观察提案助手。仅返回 JSON 数组（不要 markdown）。每个元素：
{"kind":"obs_fact|obs_signal|obs_uncertainty|obs_deviation","strength":0.0-1.0,"statement":"...","question":"...","evidence":["wi_id"]}

规则：
- 只能使用下方提供的 directive 与结构化 signal；不要编造工具输出。
- 范围不清时优先 obs_uncertainty；obs_fact 仅在有强依据时使用。
- 最多 3 条提案；空数组 [] 合法。`

const observationTaskAppendixEN = `You propose structured observations for an orchestration Observe node.
Return ONLY a JSON array (no markdown). Each element:
{"kind":"obs_fact|obs_signal|obs_uncertainty|obs_deviation","strength":0.0-1.0,"statement":"...","question":"...","evidence":["wi_id"]}

Rules:
- Use ONLY the provided directive and structured signals; do not invent tool outputs.
- Prefer obs_uncertainty when scope is unclear; obs_fact only when strongly supported.
- Maximum 3 proposals. Empty array [] is valid.`

// RollupSynthAppendix returns synthesis instructions for parent rollup WorkItems.
func RollupSynthAppendix(loc Locale) string {
	if loc == LocaleEN {
		return rollupSynthAppendixEN
	}
	return rollupSynthAppendixZH
}

const rollupSynthAppendixZH = `你是父 WorkItem 的 rollup 合成助手。
- 合并下方子 WorkItem 结论；禁止发起新的 tool call。
- 输出可验证结论与剩余 open_questions；不要输出 planning meta。`

const rollupSynthAppendixEN = `You synthesize parent WorkItem rollup conclusions.
- Merge child WorkItem findings below; do NOT issue new tool calls.
- Output a verifiable conclusion and remaining open_questions; no planning meta.`
