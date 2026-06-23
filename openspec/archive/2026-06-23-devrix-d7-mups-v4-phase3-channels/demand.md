---
demand-id: DM-20260625-001-PRC2
title: D7 MUPS v4.3 Phase 3 PR-C2（Execute 4 Channel + ChannelRouter + 5 T 点）
priority: P0
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-06-23
parent-change: devrix-d7-mups-v4-phase3-execute
parent-demand: DM-20260625-001
---

# D7 MUPS v4.3 Phase 3 PR-C2 — Execute 4 Channel + ChannelRouter

## 1. 背景

`devrix-d7-mups-v4-phase3-execute` Change (DM-20260625-001) 已完成 Phase 3 PR-C1（Artifact 4 类 + SideEffect 5 态 + 跨域类型上提 shared/types 打破 import cycle），落地在 PR #164 + PR #165。Phase 3 的下一步是 PR-C2：把 [[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/44-d7-execute-node-design|doc 44 Execute 节点]] Channel 抽象 + ChannelRouter 落地。

## 2. 问题陈述

Phase 3 PR-C1 完成了 Execute 节点 Artifact 端的数据契约（ArtifactKind 4 类 + SideEffectStatus 5 态），但 Execute 节点的 Channel 抽象（4 个具体 Channel + 1 个 Router）还是空缺。这导致：

- Phase 3 PR-C2 自身就是 Channel 抽象的入口
- Phase 3 PR-C3 (StrategyDecider) 需要在 ChannelRouter.Route 与 Channel.Execute 之间插桩
- Phase 3 PR-C4 (ToolSpec v3) 与 PR-C7 (Executor + DispatchWorker) 需要 Channel 抽象作为依赖
- PlanKind ↔ ArtifactKind 1:1 映射（C2/W8 决议）没有 ChannelRouter 实现

## 3. PR-C2 目标

1. 新建 `internal/layers/orchestration/execute/` 包
2. Channel interface (Name/Supports/Execute) + ChannelRegistry (PlanKind → Channel 1:1 绑定 + 冲突检测) + ChannelRouter (无状态分发)
3. CommitChannel (CommitmentPlan → ArtifactStateChangeCert, 1-Step 同步 + IdempotencyKey 强制 + 超时 SideEffectInflight)
4. ProtocolChannel (ProtocolPlan → ArtifactResponseRecord, 顺序多步 + 失败 reverse-order rollback)
5. ScenarioChannel (ScenarioPlan → ArtifactProbeReport, 并行探测 + 多数派投票 MaxParallel=5)
6. ExplorationChannel (ExplorationPlan → ArtifactExperimentData, 多 agent 并行 + 优先级排序 + PersistScope 派生 SideEffectStatus)
7. 5 SentinelError + 4 helpers (EXEC_CHANNEL_9001..9004)
8. PlanKind ↔ ArtifactKind 1:1 映射 (C2/W8 决议落地)
9. 5 P0 T 点 + 22 测试 (88.1% 覆盖率)

## 4. 验收标准

- 5 P0 T 点全 IMPLEMENTED (D7-S9-A26-T01..T05)
- 22 个测试 100% PASS (0 race detector warnings)
- 覆盖率 ≥ 80% (实际 88.1%)
- C2/W8 PlanKind ↔ ArtifactKind 1:1 映射已落地
- 4 Channel 共享 ToolRunner interface 解耦 PR-C4 (本地 ToolRunner 隔离)
- 不可变 Channel/ChannelRegistry/ChannelRouter（注册一次只读）

## 5. PR 拆分

| PR | 范围 | 风险 |
|----|------|------|
| PR-C2 | Execute 4 Channel + ChannelRouter + 5 P0 T 点 | Low |

## 6. 不在本次任务范围

- PR-C3 StrategyDecider + RetryPolicy: 待 Phase 3 PR-C3 独立 change
- PR-C4 ToolSpec v3 (10 fields): 待 Phase 3 PR-C4 独立 change（PR-C2 用本地 ToolRunner interface 隔离）
- PR-C5 ExecutionEvidence 结构化: 待 Phase 3 PR-C5 独立 change
- PR-C6 VerifyTrigger wiring: 待 Phase 3 PR-C6 独立 change
- PR-C7 Executor + DispatchWorker v2: 待 Phase 3 PR-C7 独立 change（包装 ChannelRouter + ChannelRegistry）
- Phase 4 Verify ReverseLookup consumer: 待 Phase 4 独立 change
- Phase 5 Learn ExperimentData consumer: 待 Phase 5 独立 change

## 7. 关联

- **前置依赖**: `devrix-d7-mups-v4-phase3-execute` (DM-20260625-001) PR-C1
- **前置依赖**: `devrix-d7-mups-v4-phase2-plan` (DM-20260623-001-PRB1) PR-B1（PlanKind 4 类 + Plan 不可变）
- **后续依赖**: `devrix-d7-mups-v4-phase3-execute` (DM-20260625-001) PR-C3..C7
- **设计稿**: doc 44 (D7 Execute 节点) + doc 37 (Process/Behavior Data Model §2.3)
- **方法论**: doc 35 §三.3 (Execute 节点方法论)
