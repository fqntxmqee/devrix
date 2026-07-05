---
demand-id: DM-20260705-002
title: "MUPS parse/budget reject — 结构化反馈注入下一轮 user frame"
source: DM-005 P2 defer + DM-20260705-001 归档 tasks P2
priority: P1
status: S4_COMPLETE
l1-domain: shared, orchestration
created: 2026-07-05
related:
  - openspec/specs/shared/prompttags.md
  - internal/layers/orchestration/sessionorchestrator/item_pipeline.go
  - internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go
  - internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go
parent_demands:
  - DM-20260704-005  # prompttags v2 IO registry (P2 defer)
  - DM-20260705-001  # tag semantics (archived)
---

# MUPS parse/budget reject — 结构化反馈注入下一轮 user frame

## 1. 原始描述

> DM-20260704-005 / DM-20260705-001 已将 Observe/Plan/Execute 的 **标签语法 + 语义 appendix** 补齐，但 **parse/budget reject 的跨轮反馈** 仍为 P2 defer。现状：LLM 输出不符合 wholebody/envelope 契约时，Observe 静默丢弃、Plan 回退 Go DefaultPlanner、budget reject 仅写入 `SpawnRationale` 并在 Execute retry 侧回灌——**Observe/Plan 下一轮 user frame 看不到结构化 reject**，模型容易重复同类错误。

## 2. 问题陈述（现状诊断）

### 2.1 已有能力（不重复建设）

| 能力 | 状态 | 路径 |
|------|------|------|
| StrategicPlanReject（budget/uncertainty gate） | ✅ 部分 | `applyBudgetCap` / `applySingleModeUncertaintyGate` |
| reject → SpawnRationale | ✅ | `formatStrategicPlanReject` → `item_pipeline.go` |
| Execute inline retry + PriorVerifyReason | ✅ | `workitem_executor.go` |
| machineSpawnFeedback（Execute retry 子集） | ✅ | `deliverable_execute.go` |
| Observe/Plan 语义 appendix + user frame | ✅ | DM-20260705-001 |

### 2.2 缺口

| 节点 | 失败模式 | 当前行为 | 缺失 |
|------|----------|----------|------|
| **Observe** | wholebody JSON parse / validate fail | `mergeProposedObservations` 吞错，确定性 Obs 继续 | 下一轮 Observe user frame **无** `prior_parse_reject` |
| **Plan** | JSON parse fail | 回退 `DefaultPlanner`，pipeline 继续 | 下一轮 Plan user frame **无** parse reject |
| **Plan** | StrategicPlanReject | 写入 `SpawnRationale`；Execute retry 可见 | **Plan/Observe 下一轮** user frame 未注入 budget reject |
| **Execute** | Verify fail | PriorVerifyReason + PriorDeliverableRetryHint | ✅ 已有（本需求仅对齐模式，不改 Execute） |

### 2.3 目标行为（草案）

1. D7 在 parse/budget reject 时产出 **机器可读 reject record**（phase、code、field、max_allowed、snippet）。
2. D7 将 reject record 持久化到 round / WorkItem 状态，供下一轮读取。
3. D2 通过 **lineframe 新字段**（如 `prior_parse_reject` 或等价 tag）注入 Observe/Plan user frame；i18n 语义表补充 when-use。
4. **不在此需求做** 同一轮 LLM format-hint 重试（Escape `parseWithRetry` 模式另议）。

## 3. 非目标

- Verify / Learn / Decide（Go-only，无 LLM user frame）
- 修改 SpawnPolicy 规则本身
- 重写 prompttags registry 或 devrix_core

## 4. 澄清记录

### Q1: reject 注入哪些节点？
**A**: Observe 与 Plan 的 **下一轮 user frame**；Execute 已有 PriorVerifyReason，仅保持模式一致。 — 2026-07-05

### Q2: 与 SpawnRationale 关系？
**A**: `SpawnRationale` 保留给人读/Decide；user frame 注入 **结构化单行/JSON linefield**，避免 prose 漂移（对齐 `machineSpawnFeedback` 思路）。 — 2026-07-05

### Q3: 同一轮是否 retry LLM？
**A**: **否**（P2 scope）；仅跨轮 feedback。同轮 retry 另开 demand。 — 2026-07-05

## 5. L1-L5 映射（草案）

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | shared | prompttags | 已有 |
| L1 | orchestration | MUPS 编排 | 已有 |
| L2 | L2-ORCH-MUPS | 六节点 WorkItem 管道 | 已有 |
| L3-BE | D7-S5 | Observe LLM 提案 | 扩展 |
| L3-BE | D7-S5 | Plan 战略提案 | 扩展 |
| L4-BE | shared-A98 | ParseRejectRecord + lineframe 字段 | **新增（草案）** |
| L4-BE | D7-S5-A98 | reject 捕获与跨轮持久化 | **新增（草案）** |
| L4-BE | D2-S15-A98 | user frame reject 注入 + i18n | **新增（草案）** |
| L5 | L5-MUPS-REJ-01 | Observe parse fail → 下轮 user frame 含 prior_parse_reject | 草拟 P0 |
| L5 | L5-MUPS-REJ-02 | Plan JSON parse fail → 下轮 Plan frame 含 reject | 草拟 P0 |
| L5 | L5-MUPS-REJ-03 | StrategicPlanReject → 下轮 Plan frame 含 budget reject | 草拟 P0 |
| L5 | L5-MUPS-REJ-04 | Execute retry 仍仅 machineSpawnFeedback，不重复注入 Plan frame | 草拟 P1 |

## 6. 验收标准（草案）

- P0：Observe/Plan parse 或 budget reject 后，**下一轮**对应 proposer 的 user prompt 含结构化 reject 字段（golden/snapshot 测试）。
- P0：reject 字段在 `TagSemanticsRegistry` / i18n 有 when-use 说明（zh/en）。
- P1：token 增量记录在 acceptance-report；无 D7 战术散文新增。

## 7. 规划状态

- [x] S1 `demand.md`
- [x] S3 `proposal.md` / `design.md` / `tasks.md` / delta spec
- [x] S4 开发
- [ ] S5–S7 验收 / 交付 / 归档
