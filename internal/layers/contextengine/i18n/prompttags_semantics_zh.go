package i18n

import "github.com/devrix/devrix/internal/shared/prompttags"

// semanticGlossaryZH maps SemanticCondition machine codes to zh-CN labels.
var semanticGlossaryZH = map[prompttags.SemanticCondition]string{
	prompttags.CondScopeUnclear:       "范围/目标不清、open question 未闭合时使用",
	prompttags.CondStrongFactExists:   "已有强 scope fact 时勿用",
	prompttags.CondSignalBacked:       "directive/signal 直接陈述且有 evidence 时使用",
	prompttags.CondNoSpeculation:      "推测或无 signal 支撑时勿用（strength ≤0.85）",
	prompttags.CondStructuredSignal:   "结构化 signal 摘要（非 fact 级）",
	prompttags.CondPreferUncertainty:  "可表达为 uncertainty 时优先 uncertainty",
	prompttags.CondMetricDelta:        "期望 vs 观测偏差（metric delta）",
	prompttags.CondNoBaseline:         "无 baseline 时勿用",
	prompttags.CondStrengthAlignedKind: "0–1，与 kind 一致；uncertainty 高 → strength 高",
	prompttags.CondRequiredForObsUncertainty: "obs_uncertainty 必填；其他可空",
	prompttags.CondEvidenceExistingIDsOnly:   "仅已有 ID（wi_id、signal 来源），勿编造路径",
	prompttags.CondMaxProposalsThree:         "最多 3 条提案（Go enforce）",

	prompttags.CondExecutionModeDecisionTree: "若 uncertainty_mean≥0.45 或 remaining_children>1 需拆子项 → decompose；否则 scope 已清晰 → single；需并行探针 → parallel_probe（uncertainty≥0.45 禁 single，Go 强制）",
	prompttags.CondDeliverableContractExample: `示例：{"citation":"file_line","severity":"p0_p1","reject":["planning_meta"],"structure":"findings_json"}`,
	prompttags.CondChildSpecsDecomposeMax2:    "decompose 时每项含 title/directive_suffix/expected_return；最多 2 项（Go 强制）",

	prompttags.CondRequiredWhenContract:       "control · 适用交付物契约时必填 · VerifyDeliverableContract 校验",
	prompttags.CondRequiredWhenFindingsJSON:   "data · structure=findings_json 时必填 · findings_json 校验",
	prompttags.CondOptionalResidualQuestions:  "data · 可选 · 残余 open question",
	prompttags.CondOptionalScopeUpdate:        "control · 可选更新 · scope 不可回扩",
	prompttags.CondOptionalConclusionProse:    "人类可读 prose · 可选 · 终局先输出 machine block，再写 <conclusion>",

	prompttags.CondTaskDirective:            "任务指令（data）",
	prompttags.CondPriorParseRejectFeedback: "上一轮 parse/budget 失败反馈（control）；按 code/field 修正输出",
	prompttags.CondStructuredSignals:        "结构化 signal 行（data）",
	prompttags.CondPriorMeanControl:         "先验均值（control，非业务内容）",
	prompttags.CondScopeOpenQuestions:       "scope 开放问题（data）",
	prompttags.CondIncrementalRound:         "增量轮次标记（control）",
	prompttags.CondScopeGoal:                "scope 目标陈述（data）",
	prompttags.CondWorkItemIdentifier:       "WorkItem ID（control，仅标识）",
	prompttags.CondPriorObservationIDs:      "上一轮 Obs ID（control，增量上下文）",
	prompttags.CondObservationSummary:       "Obs 摘要（data）",
	prompttags.CondUncertaintyMeanControl:   "不确定性均值（control）；≥0.45 时禁 single",
	prompttags.CondRemainingChildrenBudget:  "剩余子节点预算（control）",
	prompttags.CondObservationIDs:           "上一轮 Obs IDs (data, 增量上下文)",
	prompttags.CondDepthControl:             "当前工作项深度 (control)",
	prompttags.CondMaxDepthControl:          "允许的最大深度 (control)",

	// DM-20260705-010 Phase 2 T9: Observe→Plan frame delta inject.
	prompttags.CondPriorArtifactSummary:     "上一轮 artifact 摘要（data）；≤80 字符；承载前一轮 Execute 收敛上下文",
	prompttags.CondKnownGaps:                "已知未闭合 gap ID（data）；机读 JSON 数组；桥接 Observe→Plan",
	prompttags.CondExistingChildrenControl:  "已存在非临时子节点数 (control)",
	prompttags.CondMaxChildrenControl:       "允许的最大子节点数 (control)",
	prompttags.CondDecomposeUsedToday:       "24h 内已用 decompose 次数 (control)",
	prompttags.CondRemainingDaily:           "今日 decompose 剩余预算 (control)",
	prompttags.CondMaxDailyControl:          "每日 decompose 上限 (control)",
	prompttags.CondMaxItersControl:          "单 WorkItem ReAct 迭代上限 (control)",
	prompttags.CondParentScopeInControl:     "父级 scope in 路径 (control, 子集约束)",

	// ResolutionContract (DM-20260704-006, RC-1) — Plan user frame fields.
	prompttags.CondResolutionStrategies: "resolution_strategies (data)：每个 ObsUncertainty 一条，绑定 obs_id + planned_tool + success_criterion；需 sibling 调查时挂 sub_worktree",
	prompttags.CondResolutionClaims:     "resolution_claims (data, 跨轮反馈)：上一轮 Execute 输出的 per-obs_id 答案+confidence+evidence",
}

// semanticNodeRoleZH maps node-role keys to one-line zh-CN descriptions.
var semanticNodeRoleZH = map[string]string{
	"observe.node_role":              "六节点管道第 1 步 Observe：封闭式分类器；输入=directive+signal，输出=Obs* 数组，不执行工具、不评估任务本身。",
	"plan.node_role":                 "六节点管道第 2 步 Plan：提案 execution_mode 与 deliverable_contract。",
	"execute.node_role":              "六节点管道第 3 步 Execute：ReAct + 交付物；Obs 分类已在 Observe 完成。",
	"execute.output.section.react":   "ReAct 阶段：tool call + 短文本",
	"execute.output.section.final":   "终局回复：先 machine block（contract/findings），后 <conclusion> 人类摘要",
}

const semanticPlaneGuideZH = "下方 user 帧中 [control] 为编排预算/约束，勿当作待分析业务；[data] 为任务与信号。"
