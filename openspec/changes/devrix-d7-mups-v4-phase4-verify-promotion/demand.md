# Demand: D7 MUPS v4.3 Phase 4 — Verify 节点升格

**Demand ID:** DM-20260623-002
**Status:** S1_Proposal → S2_Proposal → S3_Design → S4_Implemented → S7_Archived
**Created:** 2026-06-23
**Author:** MUPS v4.3 Phase 4 Verify 节点落地梳理

---

## 1. 需求描述

MUPS v4.3 5 节点管道（Observe → Plan → Execute → Verify → Learn）的前 3 节点（Observe PR-A1, Plan PR-B1, Execute PR-C1/C2）已 S7_Archived（PR #163/#164/#166/#167/#168/#169 联动）。Verify 节点目前仍以分散子集形式存在（doc 17/18 L1+L2 verifier + 内联 string switch），缺少节点级独立抽象。

**核心需求**：把 Verify 节点升格为完整节点级设计（节点级 interface + typed enum + 聚合函数 + 结构化 Evidence + SystemAnomaly wiring），为 Phase 5 Learn 节点的 LP-1 闭环（Observe.Receive prior ← Learn.ReputationStore）准备数据契约。

## 2. 业务背景

Phase 5 Learn 节点强依赖 Verify 节点的 Verdict 数据契约：

- LearningAsset 4 类（SOPAsset / ProtocolAsset / KnowledgeAsset / ConclusionAsset）需要 Evidence 数据作为内容来源
- ReputationEvidence 需要 Verdict.SourceID + Confidence + SystemAnomaly 作为信誉评分输入
- AdaptivePrior Bayesian Update 需要 Verdict.Kind + Confidence 作为先验更新信号

没有完整 Verify 节点抽象，Phase 5 Learn 节点 LP-1 闭环无法落地。

## 3. 关键产物

1. **VerdictKind typed enum**：4 态（Pass/Partial/Indeterminate/Fail）typed enum 替代 string switch
2. **AggregateVerdicts 函数**：4 AggregationStrategy 策略（WeakConjunction/StrongConjunction/Majority/ThresholdByPass）+ 边界处理（空/单元素/同质）
3. **VerdictToExitReason 函数**：4 Verdict → 4 ExitReason 语义映射 + SystemAnomaly 覆盖
4. **14 ExitReason 枚举**：既有 8 个（保持字符串不变，向后兼容）+ 6 新增
5. **VerifyWithRetry G8-1 P0-3 修复**：3 次 parse failure → INDETERMINATE（非 FAIL）
6. **Evidence struct + EvidenceExtractor**：5 字段 + interface 2 方法（Extract + Validate）
7. **SystemAnomaly 异常聚合**：SystemAnomalyAggregator + ObserveNode wiring（CatSystem 异常 → forced UncertaintyCoord.Value=0.95）

## 4. PR 拆分（4 PR × 8 T 点）

- **PR-D1**（D7-S10-A32-T01/T02）：AggregateVerdicts（G3-1）+ VerdictKind typed enum
- **PR-D2**（D7-S10-A33-T03/T04）：VerdictToExitReason + 14 ExitReason + G8-1 修复
- **PR-D3**（D7-S10-A34-T05/T06）：Evidence struct + EvidenceExtractor interface
- **PR-D4**（D7-S10-A35-T07/T08）：SystemAnomaly 异常聚合 + ObserveNode wiring

## 5. 工作量估算

| 项 | 值 |
|----|----|
| 文件数 | 13 新文件 + 4 test 文件 |
| LOC | +2600 / -80 |
| 测试 | 48 tests |
| 风险 | Low/Medium |
| 时间 | 6 天 |

## 6. 关联

- 前置：Phase 2 PR-A1 (Observation 4 类) + PR-B1 (Plan 4 类) + Phase 3 PR-C1 (Artifact) + PR-C2 (Channel)
- 后续：Phase 5 Learn 节点（PR-E1..E5 强依赖本 PR 数据契约）
- 设计稿：doc 45 (D7 Verify 节点详细技术方案) + doc 17 (L2 verifier) + doc 18 (L1 ExitReason)