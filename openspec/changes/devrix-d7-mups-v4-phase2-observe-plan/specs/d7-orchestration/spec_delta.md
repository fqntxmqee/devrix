# Spec Delta: D7-Orchestration — Observe + Plan 节点

**Change ID:** `devrix-d7-mups-v4-phase2-observe-plan`
**Target Spec:** `openspec/specs/d7-orchestration/spec.md`
**Date:** 2026-06-23
**Status:** S2_Proposal

> **Delta 性质**：ADDED only。本 change 不修改现有 Requirement，只新增 18 个 ADDED Requirement，对应 Observe 节点 + Plan 节点的 18 个 Acceptance Criteria。

---

## ADDED Requirements

### REQ-D7-OBS-001: Observation 4 类 + 2 Category

```gherkin
Feature: D7-OBS — Observation 数据契约

  Scenario: Observation 4 类 + Category 2 类 + 不可变
    Given 定义 ObservationKind{ObsFact, ObsSignal, ObsDeviation, ObsUncertainty}
    And 定义 Category{CatBusiness, CatSystem}
    When 调用 Observation.WithKind() / WithStrength()
    Then 返回新 Observation，原实例未修改
    And 4 类 Kind 与 2 类 Category 互相正交（4×2 = 8 种组合）

  Scenario: Observation 字段校验
    Given 一个 Observation 实例
    When 调用 Observation.Validate()
    Then ID 必填，Strength ∈ [0,1]，DetectedAt 非零
    And 任何字段缺失返回 ErrObservation* SentinelError
```

### REQ-D7-OBS-002: UncertaintyReport 聚合 + Partition 不变式

```gherkin
Feature: D7-OBS — UncertaintyReport 聚合

  Scenario: Partition 不变式强制保证
    Given 10 个 Observation（6 个 CatBusiness + 4 个 CatSystem）
    When 调用 NewUncertaintyReport(observations)
    Then 返回 report.BusinessObservations = 6 个 + report.SystemObservations = 4 个
    And 6 + 4 == len(report.Observations) = 10
    And 不变式被破坏时返回 ErrUncertaintyReportPartitionInvariant

  Scenario: ComputeOverallStrength 只遍历 BusinessObservations
    Given 10 个 Observation（6 CatBusiness + 4 CatSystem）
    When 调用 report.ComputeOverallStrength()
    Then 平均 Strength = avg(CatBusiness.Strength)，不包含 CatSystem

  Scenario: FilterByKind 故意遍历全集
    Given 10 个 Observation（混合 Kind）
    When 调用 report.FilterByKind(ObsDeviation)
    Then 返回所有 ObsDeviation（不论 Category）
```

### REQ-D7-OBS-003: UncertaintyCoord Phase 1 增量扩展

```gherkin
Feature: D7-OBS — UncertaintyCoord 扩展

  Scenario: FromVerifier 工厂方法
    Given VerdictKind=Pass, Confidence=0.9, Reason="ok"
    When 调用 UncertaintyCoord.FromVerifier(verdict, confidence, reason)
    Then 返回 UncertaintyCoord{Score: 0.9, Verdict: Pass, FromVerifier: true, Source: SourceVerifier}

  Scenario: Phase 1 JSON 向后兼容
    Given 一个旧版本 JSON（只有 Score/Verdict/Reason/Source 字段）
    When Unmarshal 到新 UncertaintyCoord
    Then FromVerifier=false, SideEffectStatus=""（零值）
    And MarshalJSON 用 omitempty 不写零值字段
```

### REQ-D7-OBS-004: IntentQuantizer 多轮收敛

```gherkin
Feature: D7-OBS — IntentQuantizer

  Scenario: 3 轮正常收敛
    Given 用户输入 "帮我看看这个 PR" + AdaptivePrior=nil
    When IntentQuantizer.Quantize()
    Then Round 1 LLM 自报 Kind=FastPath + Confidence=0.7
    And Round 2 evidence 验证通过
    And Round 3 AdaptivePrior 加权后 Confidence=0.85
    And 返回 IntentPayload{Rounds: 3, Source: SourceClassifier}

  Scenario: 单轮超时兜底
    Given LLMCompleter 单轮响应 > 2000ms
    When IntentQuantizer.Quantize()
    Then Round 1 超时 → Round 2 → Round 3 → 仍超时
    And 返回 ErrIntentUnquantifiable + IntentPayload{Source: SourceAdvisory}
    And Kind 兜底为 IntentOrchestrate（Loop-First fallback）

  Scenario: 高 Confidence 1 轮即收敛
    Given LLM 自报 Confidence=0.9
    When Quantizer 检查收敛
    Then 返回 IntentPayload{Rounds: 1}，不再 round 2/3
```

### REQ-D7-OBS-005: AnomalyDetector 4 实现 + Composite

```gherkin
Feature: D7-OBS — AnomalyDetector

  Scenario: Composite 4 detector 并行触发
    Given 4 个 detector（Historical/Structural/LLMClaim/Evidence）
    When CompositeAnomalyDetector.Detect()
    Then errgroup.Go 并行触发 4 个 detector
    And 返回 4 个 DeviationPayload（按 detector 顺序）

  Scenario: 单 detector 失败不影响整体
    Given LLMClaimDetector 因 LLM timeout 返回 error
    When CompositeAnomalyDetector.Detect()
    Then Historical + Structural + Evidence 仍正常返回
    And 失败的 detector 对应位置为空 DeviationPayload{}
    And slog.Warn("anomaly_detector.failed") 被发出

  Scenario: LLMClaimDetector 不调 tool（避免递归）
    Given LLMClaimDetector 调用 LLMCompleter.CompleteWithOptions
    Then options.AllowedTools = nil（强制）
    And system prompt 明确"你不能调用任何 tool"
```

### REQ-D7-OBS-006: OP-6 业务/系统异常分离

```gherkin
Feature: D7-OBS — OP-6 业务/系统异常分离

  Scenario: CatSystem 误归类被反向校验
    Given DeviationPayload{Category: CatSystem, DetectorID: "historical", ZScore: 2.5}
    When RevalidateCategory(payload) 被调用
    Then 返回 CatBusiness（重分类）
    And slog.Warn("observe.category_misclassify") 被发出

  Scenario: CatSystem 合法保留
    Given DeviationPayload{Category: CatSystem, DetectorID: "system_health", ZScore: 1.5}
    When RevalidateCategory(payload) 被调用
    Then 返回 CatSystem（保留）
```

### REQ-D7-OBS-007: ObserveNode P95 ≤ 50ms

```gherkin
Feature: D7-OBS — ObserveNode 性能

  Scenario: ObserveNode.All() P95 性能约束
    Given 1000 个并发 ObserveRequest
    When 测量 ObserveNode.All() 的延迟分布
    Then P95 ≤ 50ms（除 IntentQuantizer 异步路径）
    And Prometheus 指标 d7_observe_p95_ms 被记录

  Scenario: IntentQuantizer 异步路径不影响 P95
    Given IntentQuantizer 单轮耗时 1500ms
    When ObserveNode.All() 调用
    Then 主路径 P95 ≤ 50ms（IntentQuantizer 不计入）
    And 兜底 IntentPayload 在 IntentQuantizer 失败时使用
```

### REQ-D7-OBS-008: LP-1 闭环（prior 注入）

```gherkin
Feature: D7-OBS — LP-1 ReputationEvidence 闭环

  Scenario: ObserveNode.Receive prior 注入 AdaptivePrior
    Given sessionID "sess_001" 在 ReputationStore 中有 AdaptivePrior{Score: 0.7}
    When ObserveNode.All() 被调用
    Then report.Prior = AdaptivePrior{Score: 0.7}
    And IntentQuantizer 第 3 轮加权时使用 prior.Score
    And 最终 Confidence = claim * (1 - prior) + evidence * prior
```

### REQ-D7-PLN-001: Plan 4 类 + Kind 匹配规则

```gherkin
Feature: D7-PLN — Plan 数据契约

  Scenario: Plan 4 类枚举
    Given 定义 PlanKind{CommitmentPlan, ProtocolPlan, ScenarioPlan, ExplorationPlan}
    When 任意 Plan 实例
    Then Kind 字段为 4 枚举之一
    And 4 类枚举互斥且可序列化（JSON/MsgPack）

  Scenario: Kind 匹配规则
    Given UncertaintyReport 含 ObsFact 主导（5 个 ObsFact + 2 个 ObsSignal）
    When Planner.MatchKind() 被调用
    Then 返回 CommitmentPlan

  Scenario: Kind 匹配兜底
    Given UncertaintyReport 含 ObsUncertainty 主导
    When Planner.MatchKind() 被调用
    Then 返回 ExplorationPlan

  Scenario: Kind 平局优先级
    Given UncertaintyReport 含 5 ObsFact + 5 ObsUncertainty（平局）
    When Planner.MatchKind() 被调用
    Then 返回 ExplorationPlan（Uncertainty > Fact 优先级）
```

### REQ-D7-PLN-002: SourceObservationIDs 血缘字段

```gherkin
Feature: D7-PLN — 血缘链

  Scenario: Plan.SourceObservationIDs 必填
    Given 一个 Plan 实例
    When 调用 Plan.Validate()
    Then len(plan.SourceObservationIDs) >= 1
    And 空时返回 ErrPlanSourceObservationIDsRequired

  Scenario: 血缘反向追溯
    Given Plan.SourceObservationIDs = ["obs_001", "obs_002"]
    When 调用 UncertaintyReport.Observations[?]
    Then 可按 ID 找到原始 Observation
    And Verify 阶段可用此血缘反向追溯到 Observation
```

### REQ-D7-PLN-003: PP-1 强度匹配

```gherkin
Feature: D7-PLN — PP-1 强度匹配

  Scenario: Plan.Strength ≤ min(BusinessObs.Strength)
    Given BusinessObservations Strengths = [0.9, 0.7, 0.8]（min = 0.7）
    And Plan.Strength = 0.8
    When Plan.Validate()
    Then 返回 ErrPlanStrengthMismatch（0.8 > 0.7）

  Scenario: Plan.Strength 合规
    Given BusinessObservations Strengths = [0.9, 0.7, 0.8]（min = 0.7）
    And Plan.Strength = 0.6
    When Plan.Validate()
    Then 通过 PP-1 检查

  Scenario: PP-1 不污染系统异常
    Given SystemObservations Strengths = [0.1, 0.1]（低）
    And BusinessObservations Strengths = [0.9, 0.7, 0.8]
    When Plan.Validate()
    Then min 计算只遍历 BusinessObservations = 0.7
    And SystemObservations 不影响 PP-1 校验
```

### REQ-D7-PLN-004: PP-2 可证伪性

```gherkin
Feature: D7-PLN — PP-2 可证伪性

  Scenario: FailureCriteria 非空
    Given Plan.FailureCriteria = []
    When Plan.Validate()
    Then 返回 ErrPlanFailureCriteriaEmpty

  Scenario: FailureCriteria Op 白名单
    Given Plan.FailureCriteria = [{Field: "exit_code", Op: "regex", Value: "..."}]
    When Plan.Validate()
    Then 返回 ErrPlanFailureCriteriaInvalidOp（"regex" 不在白名单）

  Scenario: FailureCriteria Field 可观测
    Given Plan.FailureCriteria = [{Field: "unknown_field", Op: "eq", Value: "..."}]
    When Plan.Validate()
    Then 返回 ErrPlanFailureCriteriaInvalidOp（"unknown_field" 不可观测）
```

### REQ-D7-PLN-005: PP-3 爆炸半径

```gherkin
Feature: D7-PLN — PP-3 爆炸半径

  Scenario: FileCount 超阈值
    Given Plan.BlastRadius.FileCount = 100
    And config.PlanMaxBlastRadiusFileCount = 50
    When Plan.Validate()
    Then 返回 ErrPlanBlastRadiusExceeded

  Scenario: APICallCount 超阈值
    Given Plan.BlastRadius.APICallCount = 30
    And config.PlanMaxBlastRadiusAPICallCount = 20
    When Plan.Validate()
    Then 返回 ErrPlanBlastRadiusExceeded

  Scenario: BlastRadius 合规
    Given Plan.BlastRadius.FileCount = 30, APICallCount = 10
    When Plan.Validate()
    Then 通过 PP-3 检查
```

### REQ-D7-PLN-006: Planner 全链路

```gherkin
Feature: D7-PLN — DefaultPlanner

  Scenario: Planner.Plan 完整流程
    Given UncertaintyReport{Observations: [...], QuantizedIntent: FastPath, Prior: nil}
    When DefaultPlanner.Plan()
    Then Step 1: MatchKind() 返回 PlanKind
    And Step 2: LLMDecomposer.Decompose() 生成 Steps
    And Step 3: BlastRadiusCalculator.Calculate() 估算 BlastRadius
    And Step 4: Validator.Validate() 强制 PP-1/2/3
    And Step 5: SourceObservationIDs 从 report.BusinessObservations 填充
    And 返回 *Plan 实例

  Scenario: Validator 失败立即 fail-fast
    Given Plan 触发 PP-3 失败
    When DefaultPlanner.Plan()
    Then 返回 error，Plan 实例被丢弃（不返回）

  Scenario: Strength 推导公式
    Given BusinessObservations Strengths = [0.9, 0.7, 0.8]（min = 0.7）
    And LLMDecomposer 生成 3 个 Steps
    When DefaultPlanner.Plan()
    Then Plan.Strength = min(LLM_strength=0.7, min_obs=0.7, floor=0.5) = 0.5
    And 实际取 StrengthFloor 作为下限
```

### REQ-D7-PLN-007: AnomaliesCount 字段

```gherkin
Feature: D7-PLN — OP-4 AnomaliesCount 衍生

  Scenario: Plan.AnomaliesCount 填充
    Given UncertaintyReport.Anomalies = [3 个 ObsDeviation]
    When DefaultPlanner.Plan()
    Then Plan.AnomaliesCount = 3

  Scenario: AnomaliesCount 用于 Learn 信誉累积
    Given Plan.AnomaliesCount = 3 写入 Verdict
    When Phase 5 Learner.Learn()
    Then ReputationEvidence 更新时考虑异常数
```

### REQ-D7-INT-001: Orchestrator:ProcessMessage wiring

```gherkin
Feature: D7-INT — Orchestrator wiring

  Scenario: ObserveNode + Planner 插入 ProcessMessage
    Given ProcessMessage 请求
    When ProcessMessage() 被调用
    Then 步骤 1: classifier.ClassifyIntent() 输出 intent
    And 步骤 2: observeNode.All() 输出 report ⭐ 新增
    And 步骤 3: planner.Plan() 输出 plan ⭐ 新增
    And 步骤 4: dispatchPlan() 兼容旧路径（plan.Steps → []Task）

  Scenario: ObserveNode.All() 失败处理
    Given ObserveNode.All() 返回 error
    When ProcessMessage()
    Then 调用 handleObserveError() 兜底
    And 不进入 planner.Plan() 阶段

  Scenario: Planner.Plan() 失败处理
    Given Planner.Plan() 返回 ErrPlan*（PP-1/2/3 失败）
    When ProcessMessage()
    Then 调用 handlePlanError() 兜底
    And Plan 实例被丢弃
```

### REQ-D7-INT-002: SourceObservationIDs 反向追溯链

```gherkin
Feature: D7-INT — 血缘链可追溯

  Scenario: Plan → Observation 反向追溯
    Given Plan.SourceObservationIDs = ["obs_001", "obs_002"]
    When 反向追溯时
    Then UncertaintyReport.Observations[?].ID == "obs_001"
    And UncertaintyReport.Observations[?].ID == "obs_002"

  Scenario: Phase 3 Artifact.SourcePlanID 衔接
    Given Plan.ID = "plan_001"
    When Phase 3 Execute 节点生成 Artifact
    Then Artifact.SourcePlanID = "plan_001"
    And 可通过 SourceObservationIDs → Plan → Observation 完整追溯

  Scenario: Phase 4 Verdict.SourceArtifactID 衔接
    Given Artifact.ID = "artifact_001"
    When Phase 4 Verifier 生成 Verdict
    Then Verdict.SourceArtifactID = "artifact_001"
    And 形成完整 4 节点追溯链：Plan → Artifact → Verdict → Learn
```

---

## REMOVED Requirements

（无 — 本 change 不删除现有 Requirement）

## MODIFIED Requirements

（无 — 本 change 不修改现有 Requirement，只新增）

---

## 关联表

| ADDED Req | 对应 AC | 对应 T 点 |
|----------|--------|----------|
| REQ-D7-OBS-001 | AC1 | D7-S8-A15-T01, T06 |
| REQ-D7-OBS-002 | AC2 | D7-S8-A15-T02, T04, T05 |
| REQ-D7-OBS-003 | AC3 | D7-S8-A15-T03 |
| REQ-D7-OBS-004 | AC4 | D7-S8-A19-T01, T02, T03, T04 |
| REQ-D7-OBS-005 | AC5, AC6 | D7-S8-A20-T01, T02, T03 |
| REQ-D7-OBS-006 | AC7 | D7-S8-A20-T04 |
| REQ-D7-OBS-007 | AC8 | D7-S8-A21-T01 |
| REQ-D7-OBS-008 | AC9 | D7-S8-A21-T02 |
| REQ-D7-PLN-001 | AC10 | D7-S8-A22-T01, T03 |
| REQ-D7-PLN-002 | AC11 | D7-S8-A22-T02 |
| REQ-D7-PLN-003 | AC12 | D7-S8-A23-T01 |
| REQ-D7-PLN-004 | AC13 | D7-S8-A23-T02 |
| REQ-D7-PLN-005 | AC14 | D7-S8-A23-T03 |
| REQ-D7-PLN-006 | AC15 | D7-S8-A24-T01, T02 |
| REQ-D7-PLN-007 | AC17 | D7-S8-A24-T03 |
| REQ-D7-INT-001 | AC16 | D7-S8-A21-T03 |
| REQ-D7-INT-002 | AC18 | D7-S8-A24-T04 |

---

## Cross-references

- 详细设计：`openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/design.md` §2-§7
- 任务清单：`openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/tasks.md`
- 提案：`openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/proposal.md`
- T 点登记：`openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/specs/d7-orchestration/t-registry_delta.md`
- 目标 Spec：`openspec/specs/d7-orchestration/spec.md`（PLANNED 状态，本 delta 落地后变 IMPLEMENTED）