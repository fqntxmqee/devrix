# Proposal: D7 MUPS v4.3 Phase 6 — Observe-Learner 跨域闭环集成 (Cross-Domain LP-1 Closure)

**Change ID:** `devrix-d7-mups-v4-phase6-observe-learner-wiring`
**Demand ID:** DM-20260624-001
**Status:** S2_Proposal → S3_Design → S4_Implemented → S7_Archived
**Priority:** P0
**Created:** 2026-06-24
**Author:** MUPS v4.3 Phase 6 跨域闭环集成

---

## 1. 背景

MUPS v4.3 5 节点管道（Observe → Plan → Execute → Verify → Learn）Phase 2-5 已全部 S7_Archived。Phase 5 PR-E5 E5.4 (D7-S11-A40-T13) 明确将"Observe 节点跨域 wiring"延期为 Phase 6 集成：

> **T13 PARTIAL**: Observe 节点 QuantizeWithPrior/DetectWithPrior/ClassifyWithPrior + Orchestrator LP-1 时序 wiring 留待 Phase 6 集成。

当前 Learn 节点的 AdaptivePrior 输出还没有真正注入 SessionOrchestrator 的下一个 Observe 周期，LP-1 闭环仅在 in-package 测试中验证（`learner_test.go::TestLP1_ClosedLoop_LearnThenInject`），缺少跨节点端到端集成测试。

doc 35 §三.1 + doc 46 §五.1 明确 AdaptivePrior 传递路径：

```
Verify → Learn.ReputationStore (Bayesian Update)
                                       ↓
                ProcessMessage 入口: Learner.Inject(sessionID)
                                       ↓
                AdaptivePrior.PriorBeta + InjectTargets
                                       ↓
        ┌──────────────────┬──────────────────┐
        ↓                  ↓                  ↓
  IntentQuantizer     AnomalyDetector    RuleClassifier
  .QuantizeWithPrior  .HistoricalDetector.ClassifyWithPrior
                       .DetectWithPrior
```

## 2. 问题陈述（5 Problems）

### P1: Observe 子模块缺失

Phase 5 设计提到 `orchtypes/intent_quantizer.go` 和 `orchtypes/anomaly_detector.go`，但 Go 代码中并不存在。当前 Observe 行为仅靠 `decisionplanning.RuleClassifier` 一处完成，缺少 IntentQuantizer（4-class 意图量化）和 AnomalyDetector（历史异常检测）的独立子模块。

### P2: WithPrior 变体未实现

3 Observer 子模块（IntentQuantizer / HistoricalDetector / RuleClassifier）目前只有 baseline `Classify/Quantize/Detect` 方法，没有 `*WithPrior` 变体。AdaptivePrior.PriorBeta 没有入口点影响 Observe 决策。

### P3: Orchestrator 未注入 Learner

`SessionOrchestrator` 没有 `learner learn.Learner` 字段 + `WithLearner(learn.Learner)` option + ProcessMessage 入口的 `Inject` 调用。当前 Learn 节点虽然存在，但与 Orchestrator 完全解耦。

### P4: ObserveRequest.Prior 字段缺失

没有统一结构把 AdaptivePrior 传给 Observer 子模块。即使加上 Learner 字段和 WithLearner option，也需要一个 ObserveRequest 结构来传递 prior。

### P5: LP-1 闭环 E2E 测试缺失

Phase 5 PR-E5 in-package 测试仅覆盖 `Learn(VerdictPass) × 3 → Alpha=3 → Inject → PriorBeta=Beta(8,3)`，但没有跨节点端到端测试：

- 没有覆盖 5 节点管道（Observe → Plan → Execute → Verify → Learn）完整跑通
- 没有验证 AdaptivePrior 跨 session 累积
- 没有验证 INDETERMINATE → PendingAsset + ScheduledMemory 路径

## 3. 解决方案（3 PR × 6 T 点）

### PR-F1: Observer 子模块 + WithPrior 变体（最小入口）

- **范围**：
  - `orchtypes/intent_quantizer.go` (NEW)：IntentQuantizer struct + 4-class 意图量化 + `Quantize(ctx, message)` + `QuantizeWithPrior(ctx, message, prior *learn.AdaptivePrior)`
  - `orchtypes/anomaly_detector.go` (NEW)：AnomalyDetector struct + HistoricalDetector + `Detect(ctx, anomalies)` + `HistoricalDetector.DetectWithPrior(ctx, anomalies, prior *learn.AdaptivePrior)`
  - `decisionplanning/classifier.go`：RuleClassifier 新增 `ClassifyWithPrior(ctx, message, prior *learn.AdaptivePrior)` 方法
  - `orchtypes/observe_request.go` (NEW)：ObserveRequest{SessionID, Message, Prior} 结构
- **T 点**：D7-S12-A41-T01 (ObserveRequest) / T02 (IntentQuantizer + QuantizeWithPrior) / T03 (AnomalyDetector + DetectWithPrior)
- **依赖**：Phase 5 (AdaptivePrior 已就绪)
- **风险**：Low — 新增子模块 + WithPrior 变体，不动 baseline 路径

### PR-F2: Orchestrator 集成 Learner

- **范围**：
  - `sessionorchestrator/orchestrator.go`：新增 `learner learn.Learner` 字段 + `WithLearner(learn.Learner)` option + ProcessMessage 入口 `o.learner.Inject(ctx, req.SessionID)` → 包装为 `OrchestratorObserveRequest{SessionID, Message, Prior}` → 传给 classifier
  - `sessionorchestrator/observe_request.go` (NEW)：OrchestratorObserveRequest 类型（含 fail-safe DefaultDeveloperPrior）
  - 给 RuleClassifier / IntentQuantizer 注入 prior 路径
- **T 点**：D7-S12-A42-T04 (WithLearner option) / T05 (ProcessMessage 入口 Inject + ObserveRequest.Prior 注入)
- **依赖**：PR-F1
- **风险**：Medium — 首次跨包 wiring sessionorchestrator ↔ learn

### PR-F3: 端到端 LP-1 闭环集成测试

- **范围**：
  - `tests/integration/d7/learn_observe_closure_test.go` (NEW)：覆盖 5 节点管道完整跑通 + AdaptivePrior 跨 session 累积
  - 验证：VerdictPass × 3 → ReputationStore.Alpha=3 → 下一轮 ProcessMessage → Inject → PriorBeta=Beta(8,3) → RuleClassifier.ClassifyWithPrior 使用 prior
  - 验证：INDETERMINATE + verifier_parse_failure → α/β 不污染 + VerifierFailureCount=1
  - 验证：PendingAsset 路径（LearningPending → ScheduledMemory）
  - 验证：5 节点管道（Observe → Plan → Execute → Verify → Learn → 下一轮 Observe）端到端 1 轮
- **T 点**：D7-S12-A43-T06 (E2E LP-1 closure)
- **依赖**：PR-F2
- **风险**：Low — 仅测试，不动生产代码

## 4. 跨域一致性

| 检查项 | 结论 |
|--------|------|
| AdaptivePrior 跨包共享 | ✅ Phase 5 `learn.AdaptivePrior` + orchtypes 不需要 import learn（用 interface{} 或上提） |
| 既有 baseline 路径不变 | ✅ RuleClassifier.Classify / IntentQuantizer.Quantize / AnomalyDetector.Detect 签名不变 |
| 冷启动兜底 | ✅ prior == nil → DefaultDeveloperPrior (Beta(5,3)) |
| 失败模式 | ✅ Learner.Inject 失败 → log + 用 DefaultDeveloperPrior，不阻塞 ProcessMessage |
| Layer lint | ✅ learn/orchtypes/decisionplanning/sessionorchestrator 4 层依赖合法 |
| Import cycle | ✅ orchtypes 不依赖 learn；用 interface 注入 + AdaptivePrior 上提到 shared/types 必要时不引 |

## 5. 关联

- **前置**：Phase 1 Foundation (DM-20260620-001) + Phase 2 PR-A1 (DM-20260623-001 Observation) + PR-B1 (DM-20260623-001-PRB1 Plan) + Phase 3 PR-C1 (DM-20260625-001 Artifact) + PR-C2 (DM-20260625-001-PRC2 Channel) + Phase 4 PR-D1..D4 (DM-20260623-002 Verdict) + Phase 5 (DM-20260623-003 Learn 节点)
- **后续**：None（5 节点管道完整闭环；可选 Phase 7 跨会话追踪 InMemoryReputationStore → D2 ContextEngine-backed + SessionReputation 跨 session 聚合）
- **设计稿**：doc 35 §三.1 (Observe 节点) + doc 46 §五.1 (AdaptivePrior 传递路径) + doc 37 §2.1-2.6 (5 节点数据模型)

## 6. 不做的事

- ❌ 不引入新的 LLM 模型（IntentQuantizer + AnomalyDetector 仍以规则 + 历史统计为主，可选 LLM 注入为 v1.x）
- ❌ 不实现 InMemoryReputationStore → D2 ContextEngine-backed 持久化（Phase 7 可选）
- ❌ 不实现 Learn ↔ D6 Evolution 数据迁移（共存）
- ❌ 不重写 Phase 1-5 既有代码（仅在 SessionOrchestrator 中增加 Learner 字段 + Inject 调用，其余方法签名不变）
- ❌ 不引入异步 Inject（注入路径与 ProcessMessage 同步，prior 缺失时 fail-safe 用 DefaultDeveloperPrior）
- ❌ 不实现 SessionReputation 跨 session 聚合（Phase 7 范围）
