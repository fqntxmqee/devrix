---
demand-id: DM-20260705-009
title: "Observe 节点封闭式分类器定位强化 — system_prompt 让 LLM 不再困惑"
source: 用户报告 + 上轮 LLM 诊断
priority: P1
status: S1_Proposal
l1-domain: orchestration, context-engine
created: 2026-07-05
related:
  - openspec/specs/d7-orchestration/spec.md
  - openspec/specs/d7-orchestration/pipeline-architecture.md
  - openspec/specs/d7-orchestration/observe-structbind.md
  - internal/layers/orchestration/sessionorchestrator/observation_proposer.go
  - internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go
  - internal/layers/contextengine/i18n/format_hints_mups.go
  - internal/layers/contextengine/i18n/prompttags_semantics_zh.go
  - internal/layers/contextengine/i18n/prompttags_semantics_en.go
parent_demands:
  - DM-20260705-001  # tag semantics layer
  - DM-20260705-002  # parse reject feedback
  - DM-20260705-003  # mups-go-struct-driven (M1 Observe)
---

# Observe 节点封闭式分类器定位强化

## 1. 背景

MUPS 5 节点重构(M1+M2+M3+M4+M5)已于 2026-07-05 全流程归档完成。Observe 节点在 M1 阶段走 go-struct-driven 设计,9 字段 + `[data]/[control]` 包裹 + i18n guide header 已稳定。

用户报告 Observe 节点 LLM 调用出现 3 个症状:

1. **"已经修改了用户语义"** — 用户原始 directive 被 `[data] directive:` 包裹
2. **"对应的动态提示词也不对"** — system_prompt 与任务不匹配
3. **"大模型返回了错误的格式"** — LLM 返 markdown / 单文本 / 空数组

## 2. 问题陈述

### 2.1 现状(已诊断)

**Observe 节点当前 system_prompt 模板**(`format_hints_mups.go::observationTaskAppendixZH`):

```
你是编排 Observe 节点的观察提案助手。仅返回 JSON 数组(不要 markdown)。
六节点管道第 1 步 Observe: 从结构化 signal 分类 Obs*, 不执行工具。
语义: ... 4 kinds, strength, question, evidence, max_proposals
每个元素: {"kind":...,"strength":...,"statement":...,"question":...,"evidence":[...]}

规则:
- 只能使用下方提供的 directive 与结构化 signal; 不要编造工具输出。
- 空数组 [] 合法。
```

**典型现场**(用户提供截图,`wi_65d7819c`):

- directive: "review d7 领域 plan目录下代码"
- 仅有 work_item_id + directive + prior_mean,**无 signal / scope / evidence**
- user task 是开放式执行任务("review 代码")
- LLM 面对 "review" 任务,既不能执行工具(被禁止),又不能编造工具输出(被禁止)
- LLM 不知"我是分类器"还是"我是分析器",因此:
  - 返 markdown 代码块
  - 返单个 review 文本
  - 返空数组 (认为没 signal)
  - 返错误 kind (认为 directive 本身就是 fact)

### 2.2 已有能力(不重复建设)

| 能力 | 状态 | 路径 |
|------|------|------|
| `ObserveSignalInput` 9 字段 + `pt` struct tag | ✅ M1 | `observation_proposer.go:25-58` |
| i18n guide header(`[control]/[data]` 区分) | ✅ M1 | `prompttags_semantics_zh.go:5` |
| 4 alias 解析(`obs_fact/fact` 等) | ✅ | `llm_observation_proposer.go:108-122` |
| `prior_parse_reject` 反馈链路 | ✅ DM-20260705-002 | `parse_reject_format.go` |
| `[]` 空数组合法 | ✅ | `format_hints_mups.go:31` |

### 2.3 缺口(Gap)

| 缺口 | 影响 | 性质 |
|------|------|------|
| system_prompt 缺"封闭式分类器"定位声明 | LLM 不知"我是分类器" | **是 Gap** |
| directive 模糊/signal 不足时未引导 obs_uncertainty 优先 | LLM 返空数组/错 kind | **是 Gap** |
| "不要编造工具输出"措辞可优化(被误读为"不要给出结论") | LLM 退缩返空 | **是 Gap** |
| M1 9 字段契约、guide header、parse 链路 | (无 Gap) | 已稳定 |

### 2.4 目标行为

修复后,当用户任务"review d7 领域 plan目录下代码"被送入 Observe 节点时:

1. LLM 看到 system_prompt 明确"**你是封闭式分类器;输入 = directive + signal;输出 = Obs* 数组**"
2. LLM 看到"**signal 不足时,优先 obs_uncertainty (返回问题而非空数组)**"
3. LLM 看到"**directive 本身是任务指令,不要假设其完成状态;只将其作为信号观察**"
4. LLM 正确返回: `[{kind:"obs_uncertainty", strength:0.7, question:"需要先 review 哪些 plan 文件以确定 scope?", evidence:["wi_65d7819c"]}]`

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `ObservationTaskAppendix(LocaleZH)` 包含"封闭式分类器"措辞,LLM 明确知道"输入=directive+signal, 输出=Obs* 数组" | P0 |
| AC2 | `ObservationTaskAppendix(LocaleZH)` 包含"signal 不足 → 优先 obs_uncertainty"引导 | P0 |
| AC3 | `ObservationTaskAppendix(LocaleEN)` 同步 AC1+AC2 英文版 | P0 |
| AC4 | `prompttags_semantics_{zh,en}.go::observe.node_role` 同步改写 | P1 |
| AC5 | 现有 8 测试(`llm_observation_proposer_test.go` 3 + `observation_proposer_test.go` 5)0 修改 PASS | P0 |
| AC6 | 新增 golden snapshot 测试覆盖"开放式 directive + 无 signal"场景,LLM 返回 obs_uncertainty 引导存在 | P0 |
| AC7 | 现有 M1 9 字段契约、i18n guide header、4 alias 解析、`prior_parse_reject` 反馈链路 0 修改 | P0 |
| AC8 | 覆盖率 ≥ 80% (P0 T 100% PASS) | P0 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | M1 go-struct-driven 9 字段契约(DM-20260705-003,已合入 master) |
| 依赖 | i18n guide header(DM-20260705-001,已合入 master) |
| 依赖 | `prior_parse_reject` 反馈链路(DM-20260705-002,已合入 master) |
| 约束 | **不能修改** `ObserveSignalInput` struct 的 9 字段契约 |
| 约束 | **不能修改** i18n guide header 措辞 |
| 约束 | **不能修改** 4 alias 解析(`obs_fact/fact` 等) |
| 约束 | **不能回退** tag-driven 设计 |
| 约束 | 文件 < 800 行,函数 < 50 行 |

## 5. 变更范围

### 新增

| 文件 | 用途 |
|------|------|
| `internal/layers/contextengine/i18n/format_hints_mups_observer_test.go` (NEW) | golden snapshot 覆盖"封闭式分类器"措辞 |
| `internal/layers/orchestration/sessionorchestrator/observation_closed_classifier_test.go` (NEW) | 集成测试覆盖 4 alias + 信号不足 → obs_uncertainty |

### 修改

| 文件 | 变更 |
|------|------|
| `internal/layers/contextengine/i18n/format_hints_mups.go` | `observationTaskAppendixZHIntro/ENIntro` + `observationTaskAppendixZHSuffix/ENSuffix` 措辞强化 |
| `internal/layers/contextengine/i18n/prompttags_semantics_zh.go` | `observe.node_role` 措辞同步 |
| `internal/layers/contextengine/i18n/prompttags_semantics_en.go` | `observe.node_role` 措辞同步 |

### 不变更

- `ObserveSignalInput` struct 9 字段契约
- i18n guide header (`plane.guide` 措辞)
- 4 alias 解析 (`obs_fact/fact` 等)
- `parseObservationProposalsJSON` 容错逻辑
- `prior_parse_reject` 反馈链路

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| system_prompt 措辞改动影响现有 8 测试 | Med | 8 测试只验证"包含某 marker"或"包含 obs_uncertainty 关键词",不验证整段措辞,改动安全 |
| i18n 双语同步遗漏 | Low | en/zh 同步改,加双语测试覆盖 |
| 用户期望"directive 原文不包裹" | Low | M1 9 字段契约 + i18n guide header 是设计契约,需求文档已说明 |
| golden snapshot 漂移 | Low | 现有 golden 文件不动,新增 golden 覆盖新场景 |

## 7. Out of Scope

- **不修改** 9 字段契约 / i18n guide header / parse 链路 (M1 锁定)
- **不复活** ChannelRouter 4 文件 (DM-20260626-009 decommissioned)
- **不修改** LLM invocation 路径 (D3 LLMGateway 不动)
- **不修改** PlanKind 路由 (M3 已闭环)
- **不修改** user frame wrapper 形式 (M1 契约)
- **不引入** 任务类型感知(commit/review/explore)的 system_prompt 分支 (过大范围,推迟到下个 change)

## 8. 检查清单

- [x] DM ID 已分配 (DM-20260705-009) 无冲突
- [x] demand.md 包含背景、问题、验收标准、范围
- [x] 至少 1 个 P0 验收标准 (AC1/AC2/AC3/AC5/AC6/AC7/AC8 都是 P0)
- [x] Out of Scope 已明确声明
- [x] DSAFT 域标注正确 (orchestration + context-engine)
