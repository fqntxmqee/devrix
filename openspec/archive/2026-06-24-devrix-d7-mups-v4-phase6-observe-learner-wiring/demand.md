# Demand: D7 MUPS v4.3 Phase 6 — Observe-Learner 跨域闭环集成 (Cross-Domain LP-1 Closure)

**Demand ID:** DM-20260624-001
**Status:** S1_Demand → S2_Proposal → S3_Design → S4_Implemented → S7_Archived
**Priority:** P0
**Created:** 2026-06-24
**Author:** MUPS v4.3 Phase 6 跨域闭环集成

---

## 1. 需求描述

MUPS v4.3 5 节点管道（Observe → Plan → Execute → Verify → Learn）Phase 2-5 已全部 S7_Archived。Phase 5 PR-E5 E5.4 (T13 PARTIAL) 明确将"Observe 节点跨域 wiring"延期到 Phase 6 集成。当前 Learn 节点的 AdaptivePrior 输出还没有真正注入 Orchestrator 的下一个 Observe 周期，LP-1 闭环仅在 in-package 测试中验证，缺少端到端集成测试。

**核心需求**：把 Learn 节点的产出（AdaptivePrior）真正注入 SessionOrchestrator.ProcessMessage 入口前的 Observe 阶段，建立 LP-1 跨节点闭环，并提供端到端集成测试覆盖。

## 2. 业务背景

Phase 5 S7_Archived 报告 (DM-20260623-003) 明确列出后续工作：

- **Phase 6 集成（T13 PARTIAL 续）**：
  - `Orchestrator.ProcessMessage` 在 ObserveNode.All() 之前调用 `Learner.Inject(ctx, sessionID)`
  - `IntentQuantizer.QuantizeWithPrior` / `AnomalyDetector.HistoricalDetector.DetectWithPrior` / `RuleClassifier.ClassifyWithPrior` 跨域 wiring
  - `tests/integration/d7/learn_observe_closure_test.go` 端到端 5 节点管道 LP-1 闭环集成测试

当前痛点（5 个核心缺失）：

1. **Observe 子模块缺失**：Phase 5 设计提到 `orchtypes/intent_quantizer.go` 和 `orchtypes/anomaly_detector.go`，但 Go 代码中并不存在
2. **WithPrior 变体未实现**：3 Observer 子模块（IntentQuantizer / HistoricalDetector / RuleClassifier）只有 baseline `Classify/Quantize/Detect` 方法，没有 `*WithPrior` 变体
3. **Orchestrator 未注入 Learner**：`SessionOrchestrator` 没有 `learner` 字段 + `WithLearner` option + `Inject` 调用
4. **ObserveRequest.Prior 字段缺失**：没有统一结构把 AdaptivePrior 传给 Observer 子模块
5. **LP-1 闭环 E2E 测试缺失**：仅 in-package `learner_test.go` 验证，缺少跨节点端到端集成测试

## 3. 关键产物

1. **ObserveRequest 结构**：在 `orchtypes` 中新增 `ObserveRequest{SessionID, Message, Prior *learn.AdaptivePrior}`，作为 Observer 子模块统一输入
2. **IntentQuantizer 子模块**：在 `orchtypes/intent_quantizer.go` 中实现 4-class 意图量化 + `QuantizeWithPrior(ctx, req, prior)` 变体
3. **AnomalyDetector 子模块**：在 `orchtypes/anomaly_detector.go` 中实现历史异常检测 + `HistoricalDetector.DetectWithPrior(ctx, anomalies, prior)` 变体
4. **RuleClassifier.ClassifyWithPrior**：在 `decisionplanning` 中给 RuleClassifier 新增 `ClassifyWithPrior(ctx, message, prior)` 方法（prior 影响 confidence）
5. **SessionOrchestrator 集成**：`WithLearner(learn.Learner)` option + ProcessMessage 入口 `Learner.Inject(ctx, sessionID)` + `ObserveRequest.Prior` 注入
6. **端到端集成测试**：`tests/integration/d7/learn_observe_closure_test.go` 覆盖 5 节点管道 LP-1 闭环（VerdictPass → Learn → ReputationStore → 下一轮 ProcessMessage → Inject → Observe 使用 prior.PriorBeta）

## 4. PR 拆分（3 PR × ~6 T 点）

- **PR-F1**（D7-S12-A41-T01/T02/T03）：ObserveRequest + IntentQuantizer + AnomalyDetector + 3 WithPrior 变体
- **PR-F2**（D7-S12-A42-T04/T05）：Orchestrator 集成 Learner（WithLearner + ProcessMessage 入口 Inject + ObserveRequest.Prior 注入）
- **PR-F3**（D7-S12-A43-T06）：端到端 LP-1 闭环集成测试（5 节点管道 + AdaptivePrior 跨 session 累积）

## 5. 工作量估算

| 项 | 值 |
|----|----|
| 文件数 | 9 新文件 + 5 test 文件 |
| LOC | +1800 / -100 |
| 测试 | 35 tests（含 3 WithPrior unit + Orchestrator wiring + E2E LP-1 closure） |
| 风险 | Medium（跨包耦合 + 引导 5 节点管道） |
| 时间 | 4 天 |

## 6. 关联

- **前置**：Phase 5 (DM-20260623-003) Learn 节点 + 5 节点管道数据契约
- **后续**：None（5 节点管道完整闭环；可选 Phase 7 跨会话追踪 InMemoryReputationStore → D2 ContextEngine-backed 实现）
- **设计稿**：doc 35 §三.1 (Observe 节点) + doc 46 §五.1 (AdaptivePrior 传递路径) + doc 37 §2.1-2.6 (5 节点数据模型)

## 7. 不做的事

- ❌ 不引入新的 LLM 模型（IntentQuantizer + AnomalyDetector 仍以规则 + 历史统计为主，可选 LLM 注入为 v1.x）
- ❌ 不实现 InMemoryReputationStore → D2 ContextEngine-backed 持久化（Phase 7 可选）
- ❌ 不实现 Learn ↔ D6 Evolution 数据迁移（共存）
- ❌ 不重写 Phase 1-5 既有代码（仅在 SessionOrchestrator 中增加 Learner 字段 + Inject 调用，其余方法签名不变）
- ❌ 不引入异步 Inject（注入路径与 ProcessMessage 同步，prior 缺失时 fail-safe 用 DefaultDeveloperPrior）
- ❌ 不实现 SessionReputation 跨 session 聚合（Phase 7 范围）
