---
demand-id: DM-20260705-010
title: D7 MUPS 5 节点 frame delta 闭环 — Observe→Plan→Execute LLM I/O 协议显式收敛
priority: P0
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-07-05
---

# D7 MUPS 5 节点 frame delta 闭环

## 1. 背景

D7 MUPS 5 节点管道（Observe → Plan → Execute → Verify → Learn → Decide）的设计原则是"**不确定性驱动渐进消解**"——每一节点的 LLM 调用应比上一节点携带更确定的上下文，逐步把 directive 中的开放变量收敛为可执行的精确指令。

但根据 Jaeger trace `38144cebcf8dda7a123827d96a731bc5`（sess_1783255992426_6000, wi_d0_s0_goal）实测，5 节点之间的 LLM I/O 协议未达成此目标。链路呈现为**5 个独立 LLM 调用拼成的序列**，而非**逐步收敛的 Markov 过程**。

前置依赖：

- DM-20260626-009（D7 Six-S Simplification）：5 节点架构定型
- DM-20260630-013（MUPS PerInvocationEmit）：跨 session 事件隔离
- DM-20260704-006（ResolutionContract + DecideBinding）：Obs→Resolution 闭环（**Plan→Verify→Decide 已闭环，但 Observe→Plan→Execute 三节点之间的 frame delta 未闭合**）
- DM-20260705-003/004（MUPS Go-struct-driven I/O contract + 三节点 prompt 去冗余）：ObservationProposer/StrategicPlanProposer 9 字段契约已落地
- DM-20260705-008（MUPS Strategy 抽象注入）：4 PlanKind × 5 VerdictKind 决策表已驱动 Decide 节点

## 2. 问题陈述

链路诊断（trace `38144cebcf8dda7a123827d96a731bc5`, wi_d0_s0_goal, 7 LLM 调用）：

| 阶段 | LLM 输入关键字段 | 输出 | 状态 |
|------|-----------------|------|------|
| Observe | `directive` **唯一字段**（69 字符） | 3 个 obs_uncertainty（scope/维度/形式都 strength≥0.75） | ❌ 只暴露缺口 |
| Plan | directive + obs_summary + budget 12 字段（928 字符） | execution_mode=decompose + 2 child_specs + scope_in=[d7/plan/] | ⚠️ 借 obs 收窄 |
| Execute #1 | directive + claudeMd + tools（2225 字符）| "先定位 d7 plan 目录" | ❌ 还在猜位置 |
| Execute #2-5 | 同上 + 累积 tool result（3487→7229 tok）| 逐步定位 + 读文件 | ⚠️ 工具反馈收敛 |
| Verify/Learn/Decide | 0 LLM | 确定性 | 0 LLM |

### 根因 #1（断链 A：Observe 输入太薄）

Observe 节点的 LLM user frame 仅含 `directive` 本身（69 字符），LLM 在 24 秒推理时间里**只能基于 directive 文本本身猜**——它看不到：
- 上一轮 Execute 工具调用历史
- Plan 已规划的 scope_in（Plan 还没跑过）
- 历史上同类 directive 的成功路径
- 本 WorkItem 的 SemanticID / Depth / SiblingIndex

→ **Observe 输出是"问题清单"而非"假设路径"**。

### 根因 #2（断链 B：Plan → Execute 信号丢失）

Plan 节点的 LLM system_prompt 要求返回 `execution_mode` + `child_specs` + `deliverable_contract`，但 **Execute 节点的 system_prompt 是固定的"你正在分层工作树中执行一个 WorkItem"**，根本没把这三项作为 frame 字段注入。Execute 5 个 sub-turn 全部用同一份固定 prompt。

→ **LLM 不知道自己是不是 decompose 子任务、期望产出 schema 是什么、哪些不确定性是 Observe 已经标记的**。

### 根因 #3（缺失 delta 回写）

Execute 第 2-5 轮 prompt tokens 从 3487 涨到 7229（+107%），这部分增长**全部来自累积的 tool_result**，**没有任何来自 Observe 或 Plan 的结构化 delta**。也没有 Execute → Observe 的"已收敛度"度量回写，下一轮 Observe 不知道上一轮解决了什么。

→ **链路是 5 个独立调用，不是 1 个收敛过程**。

### 用户报告触发

用户在 2026-07-05 实测 trace 后提出："如果指令都一样，还要子请求干什么呢？" + "看看 Observe→Plan→Execute 链路是否能推动从观察→规划→执行这个链路闭环，目的是将不确定性问题逐步确定性"。

## 3. 验收标准

| ID | 标准 | 优先级 | 度量方式 |
|----|------|--------|----------|
| AC1 | Observe 输出新增 `prior_artifact_summary` 字段，承接上一轮 Execute 收敛度量（首轮为空） | P0 | Jaeger span tag + Go test |
| AC2 | Observe 输出新增 `known_gaps` 字段，等价于 Plan 已规划的 scope_in / child_specs[].directive_suffix（首轮为空） | P0 | Jaeger span tag + Go test |
| AC3 | Plan 输出的 `execution_mode` + `child_specs[]` + `deliverable_contract` 注入 Execute 节点的 system_prompt frame | P0 | prompt tag 对比 + Go test |
| AC4 | Execute 每个 sub-turn 结束后 emit `convergence_metric` span（含 uncertainty_reduction_rate + observed_gaps_closed_count） | P0 | Jaeger span + Go test |
| AC5 | 端到端 trace 验证：Observe→Plan→Execute 链路 LLM 帧 delta 显式可见（Jaeger span tag 含 frame delta fields） | P0 | 真实 trace 重放对比 |
| AC6 | 0 行为变化场景：5 节点重构 M1-M5 已落地的 LLM frame 契约 0 修改 | P1 | 现有 70+ 测试 0 修改 PASS |
| AC7 | LLM frame delta 闭合验证：trace 上 Observe→Plan→Execute LLM 调用 prompt size 单调不增（允许 ±5% 噪声） | P1 | 真实 trace 统计 |
| AC8 | 三方博弈论 review：S3-Gate 前发起 codex+cursor 三方共识 review | P0 | PR 评论 / 合入前讨论 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖前置 | DM-20260705-008 (M3 Strategy) 已落地 → WorkItemExecContext.Strategy 字段可复用承载 Plan frame delta |
| 依赖前置 | DM-20260705-003 (M1) FrameObserveUser 9 字段契约已就位 → 在 9 字段之外加 prior_artifact_summary/known_gaps |
| 依赖前置 | DM-20260705-009 (Observe closed-classifier) → Observe 输出形态约束为封闭式 JSON 数组，delta 字段也是数组元素 |
| 依赖前置 | DM-20260704-006 (ResolutionContract) → Execute 输出 ResolutionClaim[] 可复用承载 convergence_metric |
| 约束 | Execute system_prompt 当前长度 2586 字符 → 增量注入 plan_frame_delta 须 ≤ 200 字符以避免 LLM context 稀释 |
| 约束 | 不破坏已有 PlanKind × VerdictKind 决策表（DM-20260705-008 M3 落地）|
| 约束 | LLM frame delta 字段必须 schema-first 形态（machine-readable JSON），不允许 prose 注入 |

## 5. 变更范围

### 新增

- `internal/layers/orchestration/interfaces/mups_frame_delta.go` — FrameDelta struct 定义
- `internal/layers/orchestration/sessionorchestrator/observe_frame_delta.go` — Observe 输出 prior_artifact_summary + known_gaps 字段
- `internal/layers/orchestration/sessionorchestrator/execute_plan_frame_inject.go` — Execute system_prompt 注入 Plan frame delta
- `internal/layers/orchestration/sessionorchestrator/convergence_metric.go` — Execute sub-turn convergence_metric 计算 + emit
- `openspec/specs/d7-orchestration/specs/d7-mups-frame-delta-delta.md` — spec delta
- `openspec/specs/d7-orchestration/d7-t-registry.md` 新增段 D7-S5-A111 + D7-S9-A112 + D7-S9-A113

### 修改

- `internal/layers/orchestration/sessionorchestrator/observation_proposer.go` — Observe frame 增加 2 个字段
- `internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go` — frame spec 扩展
- `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go` — Plan 输出可序列化 delta
- `internal/layers/orchestration/sessionorchestrator/item_pipeline.go` — Plan→Execute frame delta 注入点 + convergence_metric emit
- `openspec/specs/d7-orchestration/spec.md` — 5 节点管道 I/O 协议段新增 frame delta 描述
- `openspec/specs/d7-orchestration/CHANGELOG.md` — 顶部条目

### 不变更

- Verify / Learn / Decide 节点 LLM I/O 协议（已是 deterministic）
- 5 节点重构 M1-M5 已落地的 LLM frame 契约（frame delta 在原 frame 之外增量注入）
- DM-20260704-006 ResolutionContract 数据契约
- DM-20260705-008 Strategy 决策表
- 三层 fail-safe / Pessimistic Commit L3 防御

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| Execute system_prompt 增量注入 > 200 字符，稀释 LLM context | Plan frame delta 被 LLM 忽略 | frame delta 走"摘要 + schema hash"双轨，摘要 ≤ 80 字符 |
| convergence_metric 计算引入额外 LLM 调用 | 5 节点延迟翻倍 | convergence_metric 走 deterministic 计算（工具结果 diff + claim 数），0 LLM |
| Observe prior_artifact_summary 字段破坏封闭式分类器定位（DM-20260705-009）| Observe 退化为开放生成 | prior_artifact_summary 字段定义为 `obs_fact` kind，由 classifier 自然吸收 |
| Plan→Execute 注入链破坏 PlanKind 决策表 | DM-20260705-008 行为回退 | frame delta 注入走 `WorkItemExecContext.Strategy` 旁路，不进决策表 |

## 7. 关联

- 父 Change: devrix-d7-orchestration-domain (DM-20260613-001)
- 兄弟 Change:
  - devrix-d7-uncertainty-resolution-traceability (DM-20260704-006) — Obs→Verify→Decide 闭环
  - d7-mups-strategy-injection (DM-20260705-008) — Strategy 抽象
  - mups-go-struct-driven (DM-20260705-003) — Frame 9 字段契约
  - mups-node-prompt-dedup (DM-20260705-004) — Prompt 净化
  - d7-observe-closed-classifier-prompt (DM-20260705-009) — 封闭式分类器定位
- 关联 trace: `38144cebcf8dda7a123827d96a731bc5` (sess_1783255992426_6000, wi_d0_s0_goal, wi_d1_s0_explore)