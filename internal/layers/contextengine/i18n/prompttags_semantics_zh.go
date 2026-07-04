package i18n

// prompttagsSemanticsZH maps locale-neutral semantic keys to zh-CN bullet text.
var prompttagsSemanticsZH = map[string]string{
	"plane.guide": "下方 user 帧中 [control] 为编排预算/约束，勿当作待分析业务；[data] 为任务与信号。",

	"observe.node_role": "六节点管道第 1 步 Observe：从结构化 signal 分类 Obs*，不执行工具。",
	"observe.kind.obs_uncertainty.when_use": "范围/目标不清、open question 未闭合时使用",
	"observe.kind.obs_uncertainty.when_not":  "已有强 scope fact 时勿用",
	"observe.kind.obs_fact.when_use":         "directive/signal 直接陈述且有 evidence 时使用",
	"observe.kind.obs_fact.when_not":         "推测或无 signal 支撑时勿用（strength ≤0.85）",
	"observe.kind.obs_signal.when_use":       "结构化 signal 摘要（非 fact 级）",
	"observe.kind.obs_signal.when_not":       "可表达为 uncertainty 时优先 uncertainty",
	"observe.kind.obs_deviation.when_use":    "期望 vs 观测偏差（metric delta）",
	"observe.kind.obs_deviation.when_not":    "无 baseline 时勿用",
	"observe.field.strength.when_use":        "0–1，与 kind 一致；uncertainty 高 → strength 高",
	"observe.field.question.when_use":        "obs_uncertainty 必填；其他可空",
	"observe.field.evidence.when_use":        "仅已有 ID（wi_id、signal 来源），勿编造路径",
	"observe.rule.max_proposals":             "最多 3 条提案（Go enforce）",
	"observe.input.directive.when_use":       "任务指令（data）",
	"observe.input.signal.when_use":          "结构化 signal 行（data）",
	"observe.input.prior_mean.when_use":      "先验均值（control，非业务内容）",
	"observe.input.scope_open_question.when_use": "scope 开放问题（data）",
	"observe.input.incremental_only.when_use":    "增量轮次标记（control）",

	"plan.node_role": "六节点管道第 2 步 Plan：提案 execution_mode 与 deliverable_contract。",
	"plan.output.execution_mode.when_use": "IF uncertainty_mean≥0.45 OR remaining_children>1 needed → decompose；ELIF scope clear → single；ELIF parallel probes → parallel_probe（uncertainty≥0.45 禁 single，Go enforce）",
	"plan.output.deliverable_contract.when_use": "示例：{\"citation\":\"file_line\",\"severity\":\"p0_p1\",\"reject\":[\"planning_meta\"],\"structure\":\"findings_json\"}",
	"plan.output.child_specs.when_use":          "decompose 时每项含 title/directive_suffix/expected_return；最多 2 项（Go enforce）",
	"plan.input.directive.when_use":             "任务指令（data）",
	"plan.input.observation_summary.when_use":   "Obs 摘要（data）",
	"plan.input.uncertainty_mean.when_use":      "不确定性均值（control）；≥0.45 时禁 single",
	"plan.input.remaining_children.when_use":    "剩余子节点预算（control）",

	"execute.node_role": "六节点管道第 3 步 Execute：ReAct + 交付物；Obs taxonomy 已在 Observe 完成。",
	"execute.output.deliverable_contract.when_use": "control · Required when contract applicable · VerifyDeliverableContract",
	"execute.output.findings_json.when_use":        "data · Required when structure=findings_json · findings_json_* verify",
	"execute.output.open_questions.when_use":       "data · Optional · 残余 uncertainty",
	"execute.output.scope_contract.when_use":       "control · Optional update · scope monotonic",
	"execute.output.conclusion.when_use":           "human prose · Optional · 终局先 machine block 后 <conclusion>",
	"execute.output.section.react":                 "ReAct 阶段：tool calls + 短文本",
	"execute.output.section.final":                 "终局回复：先 machine block（contract/findings），后 <conclusion> 人类摘要",
}
