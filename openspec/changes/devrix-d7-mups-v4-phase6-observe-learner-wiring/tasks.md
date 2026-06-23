# Tasks: D7 MUPS v4.3 Phase 6 — Observe-Learner 跨域闭环集成

**Change ID:** `devrix-d7-mups-v4-phase6-observe-learner-wiring`
**Demand ID:** DM-20260624-001
**Status:** S3_Tasks → S4_Implemented → S7_Archived
**Created:** 2026-06-24

---

## 总览

| 阶段 | PR | T 点 | 文件 | LOC | 风险 | 状态 |
|------|----|----|------|-----|------|------|
| PR-F1 | Observe 子模块 + WithPrior 变体 | 3 T (D7-S12-A41-T01/T02/T03) | 5 NEW + 1 MODIFIED | +700/-0 | Low | pending |
| PR-F2 | Orchestrator 集成 Learner | 2 T (D7-S12-A42-T04/T05) | 1 NEW + 1 MODIFIED | +200/-30 | Medium | pending |
| PR-F3 | 端到端 LP-1 闭环集成测试 | 1 T (D7-S12-A43-T06) | 1 NEW | +800/-0 | Low | pending |
| **总计** | **3 PR 联动** | **6 T 点** | **7 NEW + 2 MODIFIED** | **+1700/-30** | — | — |

---

## PR-F1: Observer 子模块 + WithPrior 变体（D7-S12-A41）

### F1.1 ObserveRequest 类型

- [ ] **F1.1.1** 新建 `internal/layers/orchestration/orchtypes/observe_request.go`
  - `ObserveRequest{SessionID, Message, Prior *learn.AdaptivePrior}` 结构
  - `NewObserveRequest(sessionID, message string, prior *learn.AdaptivePrior) ObserveRequest` 构造
  - 不可变 + 浅拷贝 + Validate() 校验 sessionID/message 非空
  - `Prior == nil` → caller 用 DefaultDeveloperPrior 兜底
- [ ] **F1.1.2** 新建 `internal/layers/orchestration/orchtypes/observe_request_test.go`
  - TestObserveRequest_New + TestObserveRequest_Validate + TestObserveRequest_NilPrior

### F1.2 IntentQuantizer 子模块（D7-S12-A41-T02）

- [ ] **F1.2.1** 新建 `internal/layers/orchestration/orchtypes/intent_quantizer.go`
  - `IntentQuantizer` struct（无内部状态，concurrency-safe）
  - `Quantize(ctx context.Context, message string) (IntentPayload, error)` baseline
  - `QuantizeWithPrior(ctx context.Context, message string, prior *learn.AdaptivePrior) (IntentPayload, error)` 变体
  - 4-class 意图分类（fact / command / orchestrate / skip）+ confidence 0-100
  - `prior != nil` → confidence *= prior.PriorBeta.Mean()（Beta 均值作为乘数，clamp 到 [0, 100]）
  - `prior == nil` → baseline confidence 不变
- [ ] **F1.2.2** `IntentPayload` 结构（Kind + Confidence + Reason + Optional Class）
- [ ] **F1.2.3** 新建 `internal/layers/orchestration/orchtypes/intent_quantizer_test.go`
  - TestIntentQuantizer_Quantize_Baseline（4-class 各 1 测）
  - TestIntentQuantizer_QuantizeWithPrior_UsePriorBeta（prior Beta(8,3) Mean=0.727 → confidence 调整）
  - TestIntentQuantizer_QuantizeWithPrior_NilPrior_UseBaseline

### F1.3 AnomalyDetector 子模块（D7-S12-A41-T03）

- [ ] **F1.3.1** 新建 `internal/layers/orchestration/orchtypes/anomaly_detector.go`
  - `AnomalyDetector` struct（含 `HistoricalDetector` 子模块）
  - `HistoricalDetector.Detect(ctx context.Context, anomalies []Anomaly) (AnomalyReport, error)` baseline
  - `HistoricalDetector.DetectWithPrior(ctx context.Context, anomalies []Anomaly, prior *learn.AdaptivePrior) (AnomalyReport, error)` 变体
  - 异常严重度评分（0-1）+ 是否触发系统异常
  - `prior != nil` → 阈值 = `0.5 * prior.PriorBeta.Mean()`（Beta 均值越高，异常阈值越高 = 更信任用户 = 更容易放过）
  - `prior == nil` → 阈值 0.5 baseline
- [ ] **F1.3.2** `Anomaly` + `AnomalyReport` 结构（Severity + Category + TriggeredSystemAnomaly）
- [ ] **F1.3.3** 新建 `internal/layers/orchestration/orchtypes/anomaly_detector_test.go`
  - TestAnomalyDetector_HistoricalDetector_Detect_Baseline
  - TestAnomalyDetector_HistoricalDetector_DetectWithPrior（prior Beta(8,3) → threshold=0.364）
  - TestAnomalyDetector_HistoricalDetector_DetectWithPrior_NilPrior

### F1.4 RuleClassifier.ClassifyWithPrior

- [ ] **F1.4.1** 修改 `internal/layers/orchestration/decisionplanning/classifier.go`
  - `RuleClassifier` 新增 `ClassifyWithPrior(ctx context.Context, message string, prior *learn.AdaptivePrior) (orchtypes.IntentClassification, error)`
  - 调用内部 `Classify` 拿到 baseline → 调整 confidence（prior.PriorBeta.Mean() 乘数）
  - 不可变 + 不动 baseline Classify 签名
- [ ] **F1.4.2** 新建 `internal/layers/orchestration/decisionplanning/classifier_with_prior_test.go`
  - TestRuleClassifier_ClassifyWithPrior_UsePriorBeta
  - TestRuleClassifier_ClassifyWithPrior_NilPrior_UseBaseline
  - TestRuleClassifier_ClassifyWithPrior_NoMutation（确保不修改 prior）

### F1.5 PR-F1 收尾

- [ ] **F1.5.1** `go vet ./...` — 0 issue
- [ ] **F1.5.2** `go test -race -count=1 ./internal/layers/orchestration/orchtypes/... ./internal/layers/orchestration/decisionplanning/...` — 12+ tests 100% PASS / 0 race
- [ ] **F1.5.3** coverage ≥ 80% per file

---

## PR-F2: Orchestrator 集成 Learner（D7-S12-A42）

### F2.1 SessionOrchestrator 集成（D7-S12-A42-T04）

- [ ] **F2.1.1** 修改 `internal/layers/orchestration/sessionorchestrator/orchestrator.go`
  - 新增 `learner learn.Learner` 字段
  - 新增 `WithLearner(l learn.Learner) OrchestratorOption`
  - 懒构造 default learner = `learn.NewDefaultLearner(...)`（NewSkillMemory + NewFeedbackMemory + NewScheduledMemory + NewInMemoryReputationStore + NewAssetBuilder）
- [ ] **F2.1.2** 新增 `sessionorchestrator/orchestrator_learner.go`（或者直接在 orchestrator.go 中）
  - `buildObserveRequest(ctx, req orchtypes.ProcessRequest) (orchtypes.ObserveRequest, error)`
    - 调用 `o.learner.Inject(ctx, req.SessionID)` 拿 AdaptivePrior
    - `learner == nil` 或 `Inject` 失败 → 用 `learn.DefaultDeveloperPrior` 兜底（fail-safe）
    - `Inject` 错误 → log + 兜底，不阻塞
- [ ] **F2.1.3** 修改 `ProcessMessage`：
  - 在 `classifySpan := o.startSpan(...)` 之前调用 `o.buildObserveRequest(ctx, req)`
  - 把 `observeReq.Prior` 传给 classifier / 未来 IntentQuantizer
- [ ] **F2.1.4** 修改 `decisionplanning/classifier.go`：
  - `RuleClassifier.Classify` 暂时仍是无 prior 路径
  - 新增 `ClassifyWithPrior` 在 PR-F1 已实现
  - ProcessMessage 中 `o.classifier.Classify` 替换为 `o.classifier.ClassifyWithPrior(ctx, req.Message, observeReq.Prior)`
  - 兼容 nil classifier（fallback to default RuleClassifier + WithPrior 路径）

### F2.2 ObserveRequest 路由（D7-S12-A42-T05）

- [ ] **F2.2.1** 修改 `SessionOrchestrator.shadowClassifier` 路径
  - `ShadowClassifier.Classify` 保持无 prior 路径
  - 但 Orchestrator 在调 shadow 之前先 `buildObserveRequest` → 把 prior 传给 RuleClassifier.ClassifyWithPrior（shadow 内部走 legacy rule 不变）
- [ ] **F2.2.2** 新增 `internal/layers/orchestration/sessionorchestrator/orchestrator_learner_test.go`
  - TestSessionOrchestrator_WithLearner_NilInjector_UseDefaultPrior
  - TestSessionOrchestrator_WithLearner_InjectError_UseDefaultPrior
  - TestSessionOrchestrator_ProcessMessage_InjectBeforeClassify（LP-1 时序约束）
  - TestSessionOrchestrator_ProcessMessage_UsePriorInClassification（prior 影响 confidence）

### F2.3 PR-F2 收尾

- [ ] **F2.3.1** `go vet ./...` — 0 issue
- [ ] **F2.3.2** `go test -race -count=1 ./internal/layers/orchestration/sessionorchestrator/... ./internal/layers/orchestration/learn/...` — 既有 tests + 4 new tests 100% PASS / 0 race
- [ ] **F2.3.3** coverage 维持 ≥ 80%

---

## PR-F3: 端到端 LP-1 闭环集成测试（D7-S12-A43）

### F3.1 E2E 集成测试（D7-S12-A43-T06）

- [ ] **F3.1.1** 新建 `tests/integration/d7/learn_observe_closure_test.go`
  - **TestE2E_LP1_ClosedLoop_LearnPassAccumulatePrior**：
    - 第 1 轮：ProcessMessage("hello") → classifier.ClassifyWithPrior(message, nil) → DefaultDeveloperPrior (Beta(5,3))
    - 模拟 Verify → Learn: ReputationStore.Get → BayesianUpdate × 3 (VerdictPass) → Alpha=3
    - 第 2 轮：ProcessMessage("another msg") → classifier.ClassifyWithPrior(message, prior) → PriorBeta=Beta(8,3) → confidence *= 8/(8+3) = 0.727
    - 验证：prior.Mean ≈ 0.727 + RuleClassifier 输出 confidence 调整正确
  - **TestE2E_LP1_ClosedLoop_IndeterminateParseFailure_NoAlphaPollution**：
    - 模拟 Learn(VerdictIndeterminate + "verifier_parse_failure") → α/β 不变 + VerifierFailureCount=1
    - 下一轮 ProcessMessage → Inject → PriorBeta 仍是 Beta(5,3)（冷启动 default）
  - **TestE2E_LP1_ClosedLoop_PendingAssetScheduledMemory**：
    - 模拟 Learn(VerdictIndeterminate + "other_reason") → LearningPending → ScheduledMemory.Store
    - 验证：ScheduledMemory.ListDue 包含新 asset + TriggerAt 默认 ExpiryAt
  - **TestE2E_5NodePipeline_End2End**：
    - 完整 5 节点（Observe → Plan → Execute → Verify → Learn）跑 1 轮
    - 验证：Plan.SourceObservationIDs 反向追溯 + Verdict.SourceArtifactID 反向追溯 + LearningAsset 沉淀 + ReputationStore 更新
- [ ] **F3.1.2** 使用 in-memory mock（不依赖真实 LLM / D2 / D3）
- [ ] **F3.1.3** 集成测试放 `tests/integration/d7/`（已有 d7_entry_test.go 等 precedent）

### F3.2 PR-F3 收尾

- [ ] **F3.2.1** `go test -race -count=1 ./tests/integration/d7/...` — 既有 12+ tests + 4 new E2E tests 100% PASS / 0 race
- [ ] **F3.2.2** 不影响 `go test ./...` 全量

---

## Phase 6: S6 Archive

- [ ] **P6.1** 创建 S6 archive PR（chore(openspec): S6 archive devrix-d7-mups-v4-phase6-observe-learner-wiring）
  - 移动 6 文件 → `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase6-observe-learner-wiring/`
  - 创建 `.openspec.yaml`（含 demand_id + change_id + status）
  - 创建 `acceptance-report.md`（含 6 AC PASS 清单）
- [ ] **P6.2** 同步 `openspec/specs/d7-orchestration/spec.md` v4.5.0 → v4.6.0
  - 更新版本号 + Last Updated
  - 在 Archived Changes 列表追加 Phase 6 entry
- [ ] **P6.3** 同步 `openspec/specs/d7-orchestration/t-registry.md` v3.13.0 → v3.14.0
  - 更新版本号 + Last Updated
  - 在 Change 段追加 Phase 6 entry（6 T 点 IMPLEMENTED，T 168→174, P0 135→141, Scenarios D7-S12 0→3）
- [ ] **P6.4** 同步 `openspec/demand-archive-index.md` 添加 Phase 6 entry
- [ ] **P6.5** 运行 `scripts/verify-archive.sh` — 13/13 PASS, 0 failure
- [ ] **P6.6** 提交 S6 archive PR + squash auto-merge
- [ ] **P6.7** 更新 memory：phase6-s7-archived

---

## 执行顺序（避免循环依赖）

1. **第 1 步**：PR-F1（Observer 子模块 + WithPrior 变体）— 无依赖
2. **第 2 步**：PR-F2（Orchestrator 集成 Learner）— 依赖 PR-F1
3. **第 3 步**：PR-F3（E2E LP-1 闭环）— 依赖 PR-F2
4. **第 4 步**：S6 Archive — 依赖全部 3 PR

**并行机会**：无（强依赖链）

---
