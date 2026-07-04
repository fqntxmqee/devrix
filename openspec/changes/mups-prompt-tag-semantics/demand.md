---
demand-id: DM-20260705-001
title: "MUPS 六节点动态提示词 — Observe/Plan/Execute 标签语义与使用场景说明"
source: 产品/架构评审（prompttags v2 后续）
priority: P1
status: OPEN
l1-domain: shared, orchestration
created: 2026-07-05
related:
  - openspec/specs/shared/prompttags.md
  - internal/layers/contextengine/i18n/format_hints_mups.go
  - internal/layers/contextengine/i18n/prompt_dynamic.go
  - internal/layers/contextengine/i18n/workitem_execute.go
  - internal/shared/prompttags/
parent_demands:
  - DM-20260704-004  # prompttags 框架
  - DM-20260704-005  # IO registry + Observe cap
---

# MUPS 六节点动态提示词 — 标签语义与使用场景说明

## 1. 原始描述

> DM-20260704-004/005 已建立 `prompttags` 包与 `MUPSIOCatalog`，Observe / Plan / Execute 的动态 system/user prompt 中已嵌入 JSON schema 行与 envelope tag 语法。实际跑 MUPS 时发现：**标签有形态、缺语义** — 大模型难以判断何时用哪种 `kind`、何时填哪个 envelope 块、user frame 各字段含义是什么，进而影响不确定性拆解与 deliverable 收敛。
>
> 术语：**MUPS 六节点** = Observe → Plan → Execute → Verify → Learn → **Decide**（历史文档常写五节点，Decide/Spawn 为第 6 步）。本需求聚焦 **前三个 LLM 节点** 的动态提示词；Verify/Learn/Decide 为 Go 确定性逻辑，仅作边界引用。

## 2. 问题陈述（现状诊断）

### 2.1 已有能力（不重复建设）

| 能力 | 状态 | 来源 |
|------|------|------|
| envelope / lineframe / wholebody 编码 Profile | ✅ | DM-004 |
| `MUPSIOCatalog` + Observe max-3 enforce | ✅ | DM-005 |
| phase appendix + user frame 组装 | ✅ | D2 materialize + i18n |
| Plan budget / StrategicPlanReject 跨轮反馈 | ✅ 部分 | DM-001/005 |
| Reject feedback loops（parse fail → 下轮 prompt） | ❌ defer | DM-005 P2 |

### 2.2 语义缺口（影响 LLM 判断）

| 节点 | 现状 prompt 内容 | 缺失 |
|------|------------------|------|
| **Observe 输出** | 一行 JSON schema + 3 条短规则 | `obs_fact` / `obs_signal` / `obs_uncertainty` / `obs_deviation` **选用准则**；`strength` / `question` / `evidence` 字段语义；与 user frame `signal` 行的关系 |
| **Observe 输入** | `ObserveUserFrame` key:value 无内联说明 | `prior_mean`、`scope_open_question`、`incremental_only` 对模型意味着什么、应如何响应 |
| **Plan 输出** | schema 行 + deliverable_contract 维度 JSON + 4 条规则 | `execution_mode` 三选一 **决策树**；`child_specs` 与 budget 字段对齐说明；`deliverable_contract` 各 dimension **组合示例**（非仅枚举） |
| **Plan 输入** | `PlanUserFrame` 15+ 字段 | 控制面字段（`remaining_children` 等）vs 数据面字段（`observation_summary`）未区分；`uncertainty_mean` 如何约束 `single` |
| **Execute 输出** | `ExecuteOutputTagDoc` 语法列表 + 1 条 Obs 禁止规则 | 各 envelope tag **何时必填/可选**；数据面（findings）vs 控制面（contract）vs 人类 prose（`<conclusion>`）**分段契约**；tag 与 Verify 维度的对应关系 |
| **Execute 输入** | wiBody 自然语言 + 实例 tag 注入 | 输入 tag（如 `<scope_contract>`）与输出 tag 是否同一 schema；retry 时 `PriorVerifyReason` 与 tag 的分工 |

### 2.3 架构层问题

1. **DocBlock 偏语法、缺语义** — `DocBlockObserveSchema()` 等仅一行 JSON 模板，无「给 LLM 读的字段词典」。
2. **数据面 / 控制面未在 prompt 中显式** — Registry 有分层，动态 prompt 未告诉模型「哪些是任务内容、哪些是格式/预算约束」。
3. **i18n 规则与 machine DocBlock 分裂** — 战术说明在 i18n 常量，schema 在 `prompttags`；缺少统一的 **TagSemanticsRegistry** 或等价物供 prompt 组装与 spec 同步。
4. **六节点职责边界未写入 LLM 可见文本** — Execute 禁止自标 Obs，但未说明 Obs 已在 Observe 完成；Plan 与 Execute 的 scope 单调收紧未表述。

## 3. 目标（草案）

为 Observe / Plan / Execute 的动态提示词补充 **机器可组装、人类/模型可读的标签语义层**，使 LLM 能：

1. 按场景选对 **wholebody 字段 / envelope tag / lineframe 字段**；
2. 理解 **数据面载荷** 与 **控制面约束** 的分工；
3. 输出更可被 Go Verify/Spawn 解析的结构，减少 silent drop 与 inline 无效重试。

**非目标（本需求不做或后置）**：

- 同一轮 LLM parse 失败后的 format-hint 重试（DM-005 P2，可另开或作为 P2 子项）；
- Verify / Learn / Decide 节点 prompt（无 LLM）；
- 修改 MUPS 状态机或 SpawnPolicy 规则本身。

## 4. 澄清记录

### Q1: 六节点 vs 五节点命名？
**A**: 规格与 prompt 统一使用 **六节点**（含 Decide）；Decide 为 Go SpawnPolicy，本需求仅在语义 doc 中标注边界，不改 Decide 实现。 — 2026-07-05

### Q2: 语义写在哪里？
**A**: 优先 **集中注册**（`prompttags` 或 i18n 语义表）→ D2 `BuildPhaseAppendix` / user frame 组装时注入；**禁止**在 orchestration Go 散落战术散文（D7 编排层规约）。 — 2026-07-05

### Q3: 与 DM-004/005 关系？
**A**: DM-004/005 解决 **形态与 enforce**；本需求解决 **语义与场景说明**，是 prompttags 系列的 P1 文档/组装层补全。 — 2026-07-05

## 5. L1-L5 映射（草案）

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | shared | 跨域 prompttags | 已有 |
| L1 | orchestration | MUPS 编排 | 已有 |
| L2 | L2-ORCH-MUPS | MUPS 六节点 WorkItem 管道 | 已有 |
| L3-BE | D7-S5 | Observe 节点 LLM 提案 | 已有 |
| L3-BE | D7-S5 | Plan 节点战略提案 | 已有 |
| L3-BE | D7-S6 | Execute 节点 ReAct | 已有 |
| L4-BE | D2-S15-A97 | Phase prompt 语义 appendix 组装 | **新增（草案）** |
| L4-BE | shared-A97 | TagSemanticsRegistry / DocBlock 语义扩展 | **新增（草案）** |
| L5 | L5-MUPS-TAG-01 | Observe kind 选用 — 给定 scope 不清 → obs_uncertainty | 草拟 P0 |
| L5 | L5-MUPS-TAG-02 | Plan execution_mode — 高 U → 不得 single | 草拟 P0 |
| L5 | L5-MUPS-TAG-03 | Execute 输出 — findings_json 结构下 envelope 必填集 | 草拟 P0 |
| L5 | L5-MUPS-TAG-04 | Prompt golden — 三节点 system+user 含语义段 hash 稳定 | 草拟 P1 |
| L5 | L5-MUPS-TAG-05 | 数据面/控制面 — user frame 字段带 role=control/data 注释 | 草拟 P1 |

## 6. 范围

### In Scope

- Tag 语义注册表设计（字段定义、使用场景、正反例一行、所属 plane）
- Observe / Plan / Execute **动态 prompt 增量**（appendix + user frame 内联说明，locale zh/en）
- `openspec/specs/shared/prompttags.md` 语义章节 delta
- Golden / snapshot 测试（prompt 字节稳定）

### Out of Scope

- P2 reject feedback loops 实现
- devrix_core 静态 base 重写
- 新 envelope tag 类型（除非语义梳理暴露必须新增）

## 7. 验收标准（草案）

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | 每个 Observe `obs_*` kind 有 **When to use / When not** 说明并出现在 phase appendix | P0 |
| AC2 | Plan `execution_mode` + `deliverable_contract` 有 **决策说明 + ≥1 合法示例** | P0 |
| AC3 | Execute 输出 tag 标明 **必填/可选/人类 prose** 三分 | P0 |
| AC4 | Observe/Plan user frame 关键控制面字段有 **一行语义**（如 `uncertainty_mean`） | P1 |
| AC5 | 语义源单点维护；i18n 与 `prompttags` 无 duplicate 战术常量 | P1 |
| AC6 | zh/en prompt golden 测试 PASS | P1 |

## 8. 风险与约束

- **Token 膨胀**：语义说明需 concise（表格/单行 bullet），避免 appendix 翻倍。
- **战术硬编码**：Go orchestration 禁止长散文；语义进 registry/i18n/materialize。
- **与 enforce 对齐**：prompt 声明须已被 Go 侧 enforce 或标注「advisory only」。

## 9. 后续（S3 再展开）

- `proposal.md` — Capabilities: TagSemanticsRegistry, PhaseSemanticAppendix, UserFrameFieldDoc
- `design.md` — 数据面/控制面 prompt 分区、六节点 I/O 矩阵、与 Verify 维度映射
- `tasks.md` — 映射 L4/L5/T 点
