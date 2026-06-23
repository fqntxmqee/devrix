# Demand: D7 MUPS v4.3 Phase 5 — Learn 节点升格 (Learn Promotion)

**Demand ID:** DM-20260623-003
**Status:** S1_Demand → S2_Proposal → S3_Design → S4_Implemented → S7_Archived
**Priority:** P0
**Created:** 2026-06-23
**Author:** MUPS v4.3 Phase 5 Learn 节点落地梳理

---

## 1. 需求描述

MUPS v4.3 5 节点管道（Observe → Plan → Execute → Verify → Learn）前 4 节点已 S7_Archived（Phase 1 Foundation + Phase 2 PR-A1/PR-B1 + Phase 3 PR-C1/PR-C2 + Phase 4 PR-D1..D4）。Learn 节点目前完全没有独立抽象（仅 D6 evolution/reputation 散落定义 SessionReputation），无法形成"沉淀与回写"闭环。

**核心需求**：把 Learn 节点升格为完整节点级设计（节点级 interface + 5 类 LearningAsset + 3 通道记忆 + ReputationEvidence Bayesian Update + AdaptivePrior 闭环回写），闭合 LP-1（Learn 节点的产出注入下一轮 Observe.Quantize 作为先验）。

## 2. 业务背景

Phase 5 Learn 节点是 MUPS 5 节点管道中"让系统从历史中学习"的关键节点，对应 doc 46 §一.1 5 大核心痛点：

1. **没有 Learn 节点**：D7 当前每次会话从零开始，SessionReputation 仅在 doc 25-28 散落定义，没有独立 Learn 抽象
2. **学习无沉淀**：Verdict 产生后即丢弃（除日志外），无法形成可复用资产
3. **声誉无累积**：每次会话都是冷启动，老用户与新用户无差别
4. **先验无回写**：Learn 节点的产出（信誉）无法注入下一轮 Observe，形成闭环
5. **3 通道记忆未分类**：技能 / 反馈 / 调度 3 类记忆散落，没有统一接口

**Learn 节点必须实现 5 大业务目标**（来自 doc 46 §一.1）：

1. **5 类 LearningAsset**：SOPAsset（标准操作流程）/ ProtocolAsset（协议）/ KnowledgeAsset（知识）/ ConclusionAsset（结论）/ PendingAsset（⭐新增，待重试）
2. **3 通道记忆**：SkillMemory（技能沉淀）/ FeedbackMemory（反馈累积）/ ScheduledMemory（调度任务）
3. **ReputationEvidence 跨会话累积**：基于 Bayesian Update（Developer Beta(5,3) / Operator Beta(8,1)）
4. **回写闭环**：ReputationEvidence 注入下一轮 Observe.Quantize 作为先验
5. **冷启动延迟**：≤ 50 次 update 达到稳定（基于 AdaptivePrior）

## 3. 关键产物

1. **LearningAsset 实体 + 5 类 Content**：SOPAsset / ProtocolAsset / KnowledgeAsset / ConclusionAsset / PendingAsset（⭐新增 MVE checkpoint state）
2. **LearningClass 5 类枚举**：从 4 类扩展到 5 类（+LearningPending）
3. **ReputationEvidence + Bayesian Update**：基于 Beta 先验（Developer Beta(5,3) / Operator Beta(8,1)）+ Wilson Score 置信区间
4. **ReputationStore 接口 + 默认实现**：Get/Update/List + 跨 session 持久化
5. **Memory 3 通道接口 + 3 实现**：SkillMemory / FeedbackMemory / ScheduledMemory（LP-2 隔离原则）
6. **AdaptivePrior + 默认先验**：BuildAdaptivePrior(rep, trackMode) → 合并 DefaultPrior + Reputation
7. **Learner 接口 + DefaultLearner**：Learn() + Inject() + ScheduledTick()
8. **AssetBuilder**：从 Verdict + Plan + Artifact 构造 LearningAsset
9. **Learner.Inject LP-1 闭环**：注入到 ObserveNode.Quantize + AnomalyDetector.HistoricalDetector + RuleClassifier

## 4. PR 拆分（5 PR × 13 T 点）

- **PR-E1**（D7-S11-A36-T01/T02/T03）：LearningAsset 5 类 + AssetContent + LearningClass 5 枚举
- **PR-E2**（D7-S11-A37-T04/T05）：ReputationEvidence + Bayesian Update + Wilson Score 置信区间
- **PR-E3**（D7-S11-A38-T06/T07）：AdaptivePrior + DefaultPriors (Developer Beta(5,3) / Operator Beta(8,1))
- **PR-E4**（D7-S11-A39-T08/T09）：Memory 3 通道接口 + 3 实现（SkillMemory + FeedbackMemory + ScheduledMemory）
- **PR-E5**（D7-S11-A40-T10/T11/T12/T13）：Learner 接口 + DefaultLearner + AssetBuilder + ScheduledTick + Observe 节点对接 (IntentQuantizer / HistoricalDetector / RuleClassifier) + LP-1 闭环

## 5. 工作量估算

| 项 | 值 |
|----|----|
| 文件数 | 18 新文件 + 7 test 文件 |
| LOC | +3300 / -50 |
| 测试 | 60 tests（含 BayesianUpdate + Wilson + 5 Asset 类 + 3 Memory + Learner E2E） |
| 风险 | Medium（首次闭环跨节点） |
| 时间 | 7 天 |

## 6. 关联

- **前置**：Phase 2 PR-A1 (Observation 4 类) + PR-B1 (Plan 4 类) + Phase 3 PR-C1 (Artifact) + PR-C2 (Channel) + Phase 4 PR-D1..D4 (Verdict + Evidence + SystemAnomaly)
- **后续**：None（5 节点管道完整闭环，可选 Phase 6 优化 / Phase 7 跨会话追踪）
- **设计稿**：doc 46 (D7 Learn 节点详细技术方案) + doc 25 (AdaptivePrior 来源) + doc 27 (3 通道记忆分类 + 跨会话累积策略) + doc 37 §2.5-6 (LearningAsset + ReputationEvidence 实体定义)

## 7. 不做的事

- ❌ 不引入新持久化后端（PostgreSQL/Redis），Phase 5 默认 in-memory store（Phase 6 可选 Redis）
- ❌ 不实现 LearnNode interface 上移到 SessionOrchestrator（保留为可选中间件，5 节点管道协调保留 D7-S2 SessionOrchestrator 职责）
- ❌ 不影响 Phase 1-4 既有 8 类行为（Foundation / Observe / Plan / Execute / Verify 行为不变）
- ❌ 不引入异步写盘（Phase 5 同步路径 Learn + Inject，异步 ScheduledTick 可与 Learn 并发）
- ❌ 不实现 Learn ↔ D6 Evolution 数据迁移（共存，D6 SessionReputation 保留为 compat shim）
- ❌ 不实现跨 session 资产清理器（dangling reference 标记，由 Phase 6 cleanup worker 处理）