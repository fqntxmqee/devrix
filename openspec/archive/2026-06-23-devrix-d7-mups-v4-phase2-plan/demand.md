---
demand-id: DM-20260623-001-PRB1
title: D7 MUPS v4.3 Phase 2 PR-B1（Plan 4 类 + Planner + MatchKind 4 Rules）
priority: P0
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-06-23
parent-change: devrix-d7-mups-v4-phase2-observe-plan
parent-demand: DM-20260623-001
---

# D7 MUPS v4.3 Phase 2 PR-B1 — Plan Data Contract + Planner

## 1. 背景

`devrix-d7-mups-v4-phase2-observe-plan` Change (DM-20260623-001) 已完成 Phase 2 PR-A1 (Observation 4 类 + UncertaintyReport + UncertaintyCoord) + PR-RF (5 review fix 闭环)，落地在 PR #163 + PR #166。Phase 2 的下一步是 PR-B1：把 [[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/43-d7-plan-node-design|doc 43 Plan 节点]] 数据契约 + Planner interface 落地。

## 2. 问题陈述

Observe 节点已经能产出 `UncertaintyReport`（含 Observations 列表 + 整体 Strength + QuantizedIntent + UncertaintyCoord），但 Plan 节点的 4 类 PlanKind（CommitmentPlan / ProtocolPlan / ScenarioPlan / ExplorationPlan） + Planner interface + MatchKind 分类规则还是空缺。这导致：

- Phase 3 PR-C2 ChannelRouter 无 PlanKind 路由依据
- Phase 4 Verify 反向追溯 Observation 无 Plan.SourceObservationIDs 血缘入口
- C2/W8 MatchKind 签名（`(*UncertaintyReport)`）如果从 PR-A1 stub（`[]Observation`）开始就有错位，PR-C2 不得不破坏式升级 API

## 3. PR-B1 目标

1. 新建 `internal/layers/orchestration/plan/` 包
2. PlanKind 4 类枚举 + snake_case wire format + MarshalJSON/UnmarshalJSON + ParsePlanKind
3. BlastRadius (FileCount/APICallCount/TokenCost/PersistScope 3 态) + FailureCriterion + Step (IdempotencyKey)
4. Plan struct + 不可变 With* 模式 + Validate() (PP-1 强度 + PP-2 可证伪 + PP-3 爆炸半径)
5. ReverseLookupObservations Phase 4 Verify 反向追溯入口
6. Planner interface + DefaultPlanner + PlanInput + MatchKind 4 规则分类器
7. strengthFloor 公式 (0.7 base − 0.1·anomalies + min(observations·0.02, 0.2))
8. 9 SentinelError + 3 helpers (PLAN_KIND_8001 / PLAN_LINEAGE_8002 / PLAN_BLAST_8003)

## 4. 验收标准

- 3 P0 T 点全 IMPLEMENTED (D7-S8-A22-T01..T03)
- 30 个测试 100% PASS (0 race detector warnings)
- 覆盖率 ≥ 80% (实际 93.5%)
- C2/W8 MatchKind 签名收紧为 `(*UncertaintyReport)` 已落地
- Plan.SourceObservationIDs 必填，违反 → ErrPlanSourceObservationIDsRequired (PLAN_LINEAGE_8002)
- 不可变 With* 模式：WithKind/WithStrength/WithFailureCriteria/WithBlastRadius/WithAnomaliesCount 全返回新副本

## 5. PR 拆分

| PR | 范围 | 风险 |
|----|------|------|
| PR-B1 | Plan 4 类 + Planner + MatchKind 4 Rules + Plan.Validate PP-1/2/3 | Low |

## 6. 不在本次任务范围

- PR-A2 IntentQuantizer: 待独立 change
- PR-A3 AnomalyDetector: 待独立 change
- PR-A4 ObserveNode wiring: 待独立 change
- PR-B2 Plan.Validate 细化（Field 可观察性扩展）: 待独立 change
- PR-B3 LLMPlanner: 待独立 change
- Phase 3 Execute 4 Channel (PR-C2): 并行 PR，依赖本 PR 落地
- Phase 4 Verify ReverseLookup consumer: 待 Phase 4 独立 change
- Phase 5 Learn ReputationEvidence consumer: 待 Phase 5 独立 change

## 7. 关联

- **前置依赖**: `devrix-d7-mups-v4-phase2-observe-plan` (DM-20260623-001) PR-A1 + PR-RF
- **后续依赖**: `devrix-d7-mups-v4-phase3-execute` (DM-20260625-001) PR-C2
- **设计稿**: doc 43 (D7 Plan 节点) + doc 37 (Process/Behavior Data Model §2.2)
- **方法论**: doc 35 §三.2 (Plan 节点方法论)
