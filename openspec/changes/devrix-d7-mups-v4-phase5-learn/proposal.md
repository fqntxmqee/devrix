# Proposal: D7 MUPS v4.3 Phase 5 — Learn 节点升格 (Learn Promotion)

**Change ID:** `devrix-d7-mups-v4-phase5-learn`
**Demand ID:** DM-20260623-003
**Status:** S2_Proposal → S3_Design → S4_Implemented → S7_Archived
**Priority:** P0
**Created:** 2026-06-23
**Author:** MUPS v4.3 Phase 5 Learn 节点落地梳理

---

## 1. 背景

MUPS v4.3 5 节点管道（Observe → Plan → Execute → Verify → Learn）前 4 节点已 S7_Archived（Phase 1 Foundation + Phase 2 PR-A1/PR-B1 + Phase 3 PR-C1/PR-C2 + Phase 4 PR-D1..D4）。但 Learn 节点仍以分散子集形式存在：

- D6 `evolution/reputation/` 散落 SessionReputation 骨架（仅信誉，无 LearningAsset）
- D6 `evolution/prior/` 散落 AdaptivePrior（仅 Operator 先验）
- doc 25 §四 / doc 27 §三/四 概念散落（Developer/Operator 默认先验 + 3 通道记忆分类 + 跨会话累积策略）
- 没有任何独立 Learn 抽象、5 类 LearningAsset、3 通道 Memory 接口、ReputationStore 接口、Learner interface

Learn 节点缺失导致 3 大核心痛点：

1. **学习无沉淀**：Phase 4 Verdict 产生后即丢弃（除日志外），无法形成可复用资产
2. **声誉无累积**：每次会话都是冷启动，老用户与新用户无差别（先验无回写 → LP-1 闭环断裂）
3. **3 通道记忆未分类**：技能 / 反馈 / 调度 3 类记忆散落，没有统一接口

doc 46 §一.1 已明确 5 大业务目标 + 5 节点级原则（LP-1 闭环回写 / LP-2 3 通道隔离 / LP-3 Bayesian 更新 / LP-4 可证伪沉淀 / LP-5 跨会话可追溯）。

## 2. 问题陈述（5 Problems）

### P1: Learn 节点抽象完全缺失

D7 当前没有任何独立 Learn 抽象，Phase 4 Verdict 产生后仅写日志，无法沉淀为可复用资产。doc 46 §四.1 要求的 `Learner` interface + `LearningAsset` struct 完全缺失。

### P2: 5 类 LearningAsset 未实现

doc 46 §四.2 要求 5 类 AssetContent（SOPAsset / ProtocolAsset / KnowledgeAsset / ConclusionAsset / **PendingAsset ⭐新增**），其中 PendingAsset 是来自 VerdictIndeterminate 的待重试资产（含 MVE checkpoint state）。当前没有任何 LearningAsset 概念。

### P3: 3 通道记忆未分类

doc 46 §四.4 要求 SkillMemory（结构化 SOP/Protocol）/ FeedbackMemory（自然语言 Knowledge/Conclusion）/ ScheduledMemory（INDETERMINATE 重试）3 通道隔离（LP-2 原则）。当前三者散落，没有统一 Memory 接口。

### P4: ReputationEvidence 无 Bayesian 更新

doc 46 §四.3 要求 ReputationEvidence 跨会话累积采用 Bayesian Update（Beta 先验），不是简单计数。Developer Beta(5,3) / Operator Beta(8,1) 默认先验（doc 25 §四）未实现。

### P5: LP-1 闭环回写断裂

Learn 节点的产出（ReputationEvidence + AdaptivePrior）无法注入下一轮 Observe.Quantize 作为先验。doc 46 §四.5 + §五.1 AdaptivePrior 传递路径明确要求 Orchestrator 持有 prior 注入到 ObserveRequest.Prior 字段，但当前没有 Learn 抽象，无法实现闭环。

## 3. 解决方案（5 PR × 13 T 点）

### PR-E1: LearningAsset 5 类 + AssetContent（最小入口）

- **范围**：LearningAsset struct + LearningClass 5 类枚举（+LearningPending）+ 5 类 AssetContent（SOP/Protocol/Knowledge/Conclusion/Pending）+ AssetContent.Validate() + NewLearningAsset fail-fast
- **T 点**：D7-S11-A36-T01 (LearningAsset struct) / T02 (5 类 AssetContent) / T03 (LearningClass 5 枚举)
- **依赖**：Phase 4 (Verdict 数据契约已就绪)
- **风险**：Low — 纯数据结构 + 接口契约，无现有调用方

### PR-E2: ReputationEvidence + Bayesian Update（信誉引擎）

- **范围**：ReputationEvidence struct（Alpha/Beta/Mean/Variance/ConfidenceLow/ConfidenceHigh）+ BayesianUpdate(prior, verdict) 函数 + Wilson Score 置信区间 + 跨 session 持久化字段
- **T 点**：D7-S11-A37-T04 (ReputationEvidence struct) / T05 (BayesianUpdate + Wilson)
- **依赖**：PR-E1（LearningAsset 数据结构）
- **风险**：Medium — Bayesian Update 数学需精确（含 G8-1 修复：INDETERMINATE "verifier_parse_failure" 不污染 α/β）

### PR-E3: AdaptivePrior + DefaultPriors（先验工厂）

- **范围**：AdaptivePrior struct + BetaPrior (Alpha/Beta) + DefaultDeveloperPrior (Beta(5,3)) + DefaultOperatorPrior (Beta(8,1)) + BuildAdaptivePrior(rep, trackMode) 函数
- **T 点**：D7-S11-A38-T06 (AdaptivePrior + BetaPrior) / T07 (DefaultPriors + BuildAdaptivePrior)
- **依赖**：PR-E2（ReputationEvidence）
- **风险**：Low — 纯函数 + 默认值，与 Phase 1 MemoryEntry precedent 一致

### PR-E4: Memory 3 通道接口 + 3 实现（记忆通道）

- **范围**：Memory interface (Store/Retrieve/Delete/List) + 3 实现（SkillMemory / FeedbackMemory / ScheduledMemory）+ MemoryChannel 3 枚举 + MemoryFilter + LP-2 隔离校验（ErrAssetClassMismatch）
- **T 点**：D7-S11-A39-T08 (Memory interface + Skill/Feedback 实现) / T09 (ScheduledMemory + ScheduledRetry)
- **依赖**：PR-E1（LearningAsset 数据结构）
- **风险**：Medium — MemoryFilter 边界 + LP-2 隔离检测需充分测试

### PR-E5: Learner interface + DefaultLearner + Observe 闭环（节点级 + 闭环）

- **范围**：Learner interface（Learn/Inject/ScheduledTick）+ DefaultLearner 实现 + AssetBuilder + ScheduledTick 调度器 + **Observe 节点对接（IntentQuantizer + HistoricalDetector + RuleClassifier 接收 AdaptivePrior）** + LP-1 闭环集成测试
- **T 点**：D7-S11-A40-T10 (Learner interface + DefaultLearner) / T11 (AssetBuilder) / T12 (ScheduledTick) / T13 (Observe 对接 + LP-1 闭环 E2E)
- **依赖**：PR-E1/E2/E3/E4（Learn 抽象全栈）
- **风险**：High — 跨节点集成测试，闭环 E2E 涉及 Orchestrator / ObserveNode / Learn 多模块

## 4. 验收标准（14 AC）

| ID | 描述 | 归属 PR | 验证 |
|----|------|---------|------|
| AC1 | LearningAsset struct 12 字段（ID/SessionID/Class/Strength/SourceSessionIDs/SourceVerdictIDs/Content/AssetKey/ContentHash/FailureCriterion/ExpiryAt/CreatedAt/LastUsedAt/UseCount/TraceID）+ 不可变 | PR-E1 | unit test |
| AC2 | LearningClass 5 枚举（SOP/Protocol/Knowledge/Conclusion/Pending）+ LearningUnknown 禁用 + String() | PR-E1 | unit test |
| AC3 | 5 类 AssetContent（SOPAssetContent/ProtocolAssetContent/KnowledgeAssetContent/ConclusionAssetContent/PendingAssetContent 含 MVEState）+ Validate() | PR-E1 | unit test |
| AC4 | ReputationEvidence struct 11 字段（SessionID/TrackMode/Alpha/Beta/Mean/Variance/ConfidenceLow/ConfidenceHigh/LastUpdated/UpdateCount/SourceVerdictIDs/VerifierFailureCount/IndeterminateCount）+ 不可变 | PR-E2 | unit test |
| AC5 | BayesianUpdate 函数 + Wilson Score 置信区间 + G8-1 修复（INDETERMINATE "verifier_parse_failure" 不污染 α/β，仅 VerifierFailureCount++）+ 冷启动除零防御 | PR-E2 | unit test |
| AC6 | AdaptivePrior struct + BetaPrior + DefaultDeveloperPrior (Beta(5,3)) + DefaultOperatorPrior (Beta(8,1)) | PR-E3 | unit test |
| AC7 | BuildAdaptivePrior(rep, trackMode) 合并 DefaultPrior + Reputation（Bayesian 合并）+ InjectTargets 3 个 | PR-E3 | unit test |
| AC8 | Memory interface 4 方法（Store/Retrieve/Delete/List）+ MemoryChannel 3 枚举 + LP-2 隔离（ErrAssetClassMismatch） | PR-E4 | unit test |
| AC9 | SkillMemory（LearningSOP/Protocol 路由）+ FeedbackMemory（LearningKnowledge/Conclusion 路由）+ ScheduledMemory（LearningPending 路由 + ScheduledRetry） | PR-E4 | unit test |
| AC10 | Learner interface 3 方法（Learn/Inject/ScheduledTick）+ LearnRequest struct + DefaultLearner 实现 | PR-E5 | unit test |
| AC11 | AssetBuilder 按 VerdictClass 选 5 类 Content + hashContent 幂等 + AssetKey 格式（`sop:PlanKind:hash`） | PR-E5 | unit test |
| AC12 | ScheduledTick 异步触发 ScheduledMemory 重试 + MaxRetries 边界（耗尽 → FeedbackMemory 警告） | PR-E5 | unit test |
| AC13 | Observe 节点对接（IntentQuantizer / HistoricalDetector / RuleClassifier 接收 AdaptivePrior.PriorBeta 作为先验）+ LP-1 闭环（ProcessMessage 入口前 Learner.Inject 完成） | PR-E5 | integration test |
| AC14 | 5 PR 联动 go vet + go test -race + layer-lint 全绿 + 跨节点集成测试全绿 | 全部 | CI |

## 5. 工作量估算

| PR | 文件数 | LOC | 测试 | 风险 |
|----|--------|------|------|------|
| PR-E1 | 4 + 1 test | +700/-0 | 14 | Low |
| PR-E2 | 2 + 1 test | +500/-0 | 10 | Medium |
| PR-E3 | 2 + 1 test | +350/-0 | 8 | Low |
| PR-E4 | 4 + 1 test | +600/-0 | 12 | Medium |
| PR-E5 | 6 + 3 test | +1150/-50 | 16 | High |
| **总计** | **18 + 7 test** | **+3300/-50** | **60** | **7 天** |

## 6. 不做的事

- ❌ 不引入新持久化后端（PostgreSQL/Redis），Phase 5 默认 in-memory store
- ❌ 不实现 LearnNode interface 上移到 SessionOrchestrator（保留为可选中间件）
- ❌ 不影响 Phase 1-4 既有 8 类行为（Foundation / Observe / Plan / Execute / Verify 不变）
- ❌ 不引入异步写盘（同步路径 Learn + Inject）
- ❌ 不实现 Learn ↔ D6 Evolution 数据迁移（共存，D6 SessionReputation 保留为 compat shim）
- ❌ 不实现跨 session 资产清理器（dangling reference 标记，由 Phase 6 cleanup worker 处理）

## 7. 关联

- **前置**：Phase 2 PR-A1 (Observation 4 类) + PR-B1 (Plan 4 类 + SourceObservationIDs) + Phase 3 PR-C1 (Artifact 4 类 + SourcePlanID) + PR-C2 (Channel) + Phase 4 PR-D1..D4 (Verdict + Evidence + SystemAnomaly + 14 ExitReason)
- **后续**：None（5 节点管道完整闭环）
- **设计稿**：doc 46 (D7 Learn 节点详细技术方案 1124 行) + doc 25 (AdaptivePrior 来源) + doc 27 (3 通道记忆分类) + doc 37 §2.5-6 (LearningAsset + ReputationEvidence 实体定义) + doc 35 §三.5 (5 节点管道方法论)

## 8. 风险与缓解

| 风险 | 等级 | 缓解 |
|------|------|------|
| Bayesian Update 数学不精确 | High | Wilson Score 公式参考 doc 46 §4.3 实现；G8-1 修复点 INDETERMINATE "verifier_parse_failure" 不污染 α/β 显式短路 |
| LP-1 闭环跨节点集成测试复杂 | High | 5 PR 严格依赖链单向：PR-E1 → PR-E2 + PR-E4；PR-E3 依赖 PR-E2；PR-E5 依赖全部前 4 PR |
| 5 类 AssetContent Validate() 边界不一致 | Medium | 统一 `ErrAssetIncomplete` sentinel + 每个 Content.Validate() 必填字段 fail-fast |
| Memory 3 通道隔离检测（ErrAssetClassMismatch）误判 | Medium | MemoryFilter 显式 Class 校验 + 路由前断言 |
| AdaptivePrior 冷启动 α=β=0 除零 | Medium | BayesianUpdate 显式 `if total == 0` 分支，保持冷启动默认值（Developer Beta(5,3) → 0.625） |
| AssetBuilder 5 类 Content 路由错误 | Medium | switch case 5 类枚举 + zero value LearningUnknown fallback 返回 nil + ErrAssetBuildFailed |
| PendingAsset MVE checkpoint state 字段传递 | Medium | PendingAssetContent.MVEState *execute.MVEState 指针（nullable）+ Question 非空校验 |
| Phase 5 跨节点 PR 落地长链路 | Low | 5 PR 严格依赖链单向，PR-E1/E2 可并行启动，PR-E5 必须最后落 |