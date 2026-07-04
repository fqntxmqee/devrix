package i18n

import (
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/prompttags"
)

// ObservationTaskAppendix returns the Observe-node JSON schema appendix (DM-20260704-001).
// Semantic bullets precede DocBlock schema (DM-20260705-001).
func ObservationTaskAppendix(loc Locale) string {
	schema := prompttags.DocBlockObserveSchema()
	semantic := RenderSemanticAppendix(contracts.MUPSPhaseObserve, loc)
	if loc == LocaleEN {
		return observationTaskAppendixENIntro + "\n" + semantic + "\nEach element:\n" + schema + observationTaskAppendixENSuffix
	}
	return observationTaskAppendixZHIntro + "\n" + semantic + "\n每个元素：\n" + schema + observationTaskAppendixZHSuffix
}

const observationTaskAppendixZHIntro = `你是编排 Observe 节点的观察提案助手。仅返回 JSON 数组（不要 markdown）。`

const observationTaskAppendixZHSuffix = `

规则：
- 只能使用下方提供的 directive 与结构化 signal；不要编造工具输出。
- 空数组 [] 合法。`

const observationTaskAppendixENIntro = `You propose structured observations for an orchestration Observe node.
Return ONLY a JSON array (no markdown).`

const observationTaskAppendixENSuffix = `

Rules:
- Use ONLY the provided directive and structured signals; do not invent tool outputs.
- Empty array [] is valid.`

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
