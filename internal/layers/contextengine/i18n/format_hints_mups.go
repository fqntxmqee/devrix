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

// DM-20260705-009: Observe 节点封闭式分类器定位强化
// 明确 LLM 角色: 输入=directive+signal, 输出=Obs* 数组; 不执行工具, 不评估任务本身.
const observationTaskAppendixZHIntro = `你是编排 Observe 节点的封闭式分类助手。
角色定位：
- 输入 = directive + 结构化 signal；输出 = Obs* 数组（每个元素: kind/strength/statement/question/evidence）
- 不执行工具、不评估任务完成度、不分析任务本身
- 不返回 markdown、不返回散文、不返回非 Obs* 格式

仅返回 JSON 数组（不要 markdown）。`

const observationTaskAppendixZHSuffix = `

规则：
- 只能使用下方提供的 directive 与结构化 signal；不要编造工具输出。
- signal 不足 / directive 模糊 / 任务需工具时 → 优先 obs_uncertainty (返回 question) 而非空数组
- directive 本身是任务指令,不要假设其完成状态; 只将其作为信号观察
- 空数组 [] 合法。`

// DM-20260705-009: English mirror of the closed-classifier role declaration above.
const observationTaskAppendixENIntro = `You are a closed-set classifier for the orchestration Observe node.
Role:
- Input = directive + structured signals; Output = Obs* array (each element: kind/strength/statement/question/evidence)
- You do not execute tools, do not assess task completion, do not analyze the task itself
- Do not return markdown, prose, or any non-Obs* format

Return ONLY a JSON array (no markdown).`

const observationTaskAppendixENSuffix = `

Rules:
- Use ONLY the provided directive and structured signals; do not invent tool outputs.
- When signals are insufficient / directive is vague / task needs tools → prefer obs_uncertainty (return a question) over an empty array
- The directive is a task instruction — do not assume its completion status; only observe it as a signal
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
