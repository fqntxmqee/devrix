# T-Registry Delta: D7-Orchestration — Observe + Plan 节点

**Change ID:** `devrix-d7-mups-v4-phase2-observe-plan`
**Target T-Registry:** `openspec/specs/d7-orchestration/t-registry.md`
**Date:** 2026-06-23
**Status:** S2_Proposal

> **Delta 性质**：ADDED only。新增 18 个 P0 T 点（PRD-A1~PRD-A4 + PRD-B1~PRD-B3），对应 18 个 ADDED Requirement。

---

## ADDED T Points

### T 点编号约定

按 DSAFT 规范（CLAUDE.md），T 点编号 `D{X}-S{X}-A{XX}-T{XX}`：

- **D7** = Domain 7（orchestration）
- **S8** = Subdomain 8（MUPS Phase 2 — Observe + Plan）
- **A15 ~ A24** = Atomic module（按 PR-A1~B3 分配）
- **T01 ~ Tnn** = Test point within module

---

### Module A15: Observation + UncertaintyReport（PR-A1）

#### D7-S8-A15-T01: Observation 4 类 + 2 Category + 不可变

- **优先级**: P0
- **位置**: `internal/layers/orchestration/orchtypes/observation_test.go`
- **覆盖 REQ**: REQ-D7-OBS-001
- **覆盖 AC**: AC1
- **测试用例**:
  - `TestObservation_4Kinds_4Categories` — 4×2 = 8 种组合互斥
  - `TestObservation_Immutability` — WithKind / WithStrength 返回新副本
  - `TestObservation_Payload_TypeAssertion` — Kind-specific payload 断言
  - `TestObservation_Validate_StrengthOutOfRange` — Strength > 1 或 < 0 → Err
  - `TestObservation_Validate_DetectedAtZero` — DetectedAt 为零 → Err
  - `TestObservation_JSON_Roundtrip` — Marshal/Unmarshal 一致

#### D7-S8-A15-T02: UncertaintyReport ComputeOverallStrength 只遍历 BusinessObservations

- **优先级**: P0
- **位置**: `internal/layers/orchestration/orchtypes/uncertainty_report_test.go`
- **覆盖 REQ**: REQ-D7-OBS-002
- **覆盖 AC**: AC2
- **测试用例**:
  - `TestUncertaintyReport_ComputeOverallStrength_BusinessOnly`
  - `TestUncertaintyReport_ComputeOverallStrength_EmptyBusiness_DefaultsHalf`
  - `TestUncertaintyReport_ComputeOverallStrength_IgnoresCatSystem`

#### D7-S8-A15-T03: UncertaintyCoord FromVerifier + Phase 1 兼容

- **优先级**: P0
- **位置**: `internal/layers/orchestration/orchtypes/uncertainty_coord_test.go`（扩展）
- **覆盖 REQ**: REQ-D7-OBS-003
- **覆盖 AC**: AC3
- **测试用例**:
  - `TestUncertaintyCoord_FromVerifier_SetsFromVerifierTrue`
  - `TestUncertaintyCoord_FromVerifier_SourceIsVerifier`
  - `TestUncertaintyCoord_JSON_Phase1_Compatibility` — 旧 JSON 解析正常
  - `TestUncertaintyCoord_JSON_Omitempty_NewFields`

#### D7-S8-A15-T04: UncertaintyReport Partition 不变式

- **优先级**: P0
- **位置**: `internal/layers/orchestration/orchtypes/uncertainty_report_test.go`
- **覆盖 REQ**: REQ-D7-OBS-002
- **覆盖 AC**: AC2
- **测试用例**:
  - `TestUncertaintyReport_PartitionInvariant_BusinessUnionSystemEqualsObservations`
  - `TestUncertaintyReport_PartitionInvariant_Violation_ReturnsError`
  - `TestUncertaintyReport_PartitionInvariant_EmptyObservations`

#### D7-S8-A15-T05: UncertaintyReport FilterByKind 遍历全集

- **优先级**: P0
- **位置**: `internal/layers/orchestration/orchtypes/uncertainty_report_test.go`
- **覆盖 REQ**: REQ-D7-OBS-002
- **覆盖 AC**: AC2
- **测试用例**:
  - `TestUncertaintyReport_FilterByKind_IncludesCatSystem` — 不按 Category 过滤
  - `TestUncertaintyReport_FilterByKind_Empty` — 返回空切片
  - `TestUncertaintyReport_FilterByKind_AllObservations` — 返回全集当 kind=ALL（自定义）

#### D7-S8-A15-T06: Observation 不可变 + 字段校验

- **优先级**: P0
- **位置**: `internal/layers/orchestration/orchtypes/observation_test.go`
- **覆盖 REQ**: REQ-D7-OBS-001
- **覆盖 AC**: AC1
- **测试用例**:
  - `TestObservation_WithKind_Immutability` — 原实例未变
  - `TestObservation_WithStrength_Panic_OnOutOfRange` — 边界保护
  - `TestObservation_WithStrength_NormalRange`

---

### Module A19: IntentQuantizer 多轮收敛（PR-A2）

#### D7-S8-A19-T01: IntentQuantizer 3 轮收敛

- **优先级**: P0
- **位置**: `internal/layers/orchestration/decisionplanning/intent_quantizer_test.go`
- **覆盖 REQ**: REQ-D7-OBS-004
- **覆盖 AC**: AC4
- **测试用例**:
  - `TestIntentQuantizer_3Rounds_Success`
  - `TestIntentQuantizer_1Round_HighConfidence_FastPath`
  - `TestIntentQuantizer_MaxRounds_Configurable`
  - `TestIntentQuantizer_NilLLMCompleter_PanicSafe`

#### D7-S8-A19-T02: IntentQuantizer 超时兜底

- **优先级**: P0
- **位置**: `internal/layers/orchestration/decisionplanning/intent_quantizer_test.go`
- **覆盖 REQ**: REQ-D7-OBS-004
- **覆盖 AC**: AC4
- **测试用例**:
  - `TestIntentQuantizer_Timeout_Fallback` — PerRoundTimeout 触发 → ErrIntentUnquantifiable
  - `TestIntentQuantizer_Timeout_AdvisorySource`
  - `TestIntentQuantizer_Timeout_OrchestrateFallback`
  - `TestIntentQuantizer_PartialSuccess_MidRound`

#### D7-S8-A19-T03: IntentQuantizer 单轮 P95 ≤ 2s

- **优先级**: P0
- **位置**: `internal/layers/orchestration/decisionplanning/intent_quantizer_test.go`（benchmark）
- **覆盖 REQ**: REQ-D7-OBS-004
- **覆盖 AC**: AC4
- **测试用例**:
  - `BenchmarkIntentQuantizer_SingleRound` — P95 ≤ 2000ms
  - `TestIntentQuantizer_PerRoundTimeout_P95_PerfAssert`

#### D7-S8-A19-T04: IntentQuantizer 兜底 route to IntentOrchestrate

- **优先级**: P0
- **位置**: `internal/layers/orchestration/decisionplanning/intent_quantizer_test.go`
- **覆盖 REQ**: REQ-D7-OBS-004
- **覆盖 AC**: AC4
- **测试用例**:
  - `TestIntentQuantizer_AllRoundsFailed_AdvisoryPayload`
  - `TestIntentQuantizer_AllRoundsFailed_RoutesToOrchestrate`

---

### Module A20: AnomalyDetector 4 实现 + Composite（PR-A3）

#### D7-S8-A20-T01: Composite 4 detector 并行

- **优先级**: P0
- **位置**: `internal/layers/orchestration/observe/anomaly_detector_test.go`
- **覆盖 REQ**: REQ-D7-OBS-005
- **覆盖 AC**: AC5
- **测试用例**:
  - `TestCompositeAnomalyDetector_AllDetect` — 4 detector 全部触发
  - `TestCompositeAnomalyDetector_OrderPreserved` — 返回顺序与 Detectors 一致
  - `TestCompositeAnomalyDetector_Empty_ReturnsEmpty`

#### D7-S8-A20-T02: HistoricalDetector Z-score 计算

- **优先级**: P0
- **位置**: `internal/layers/orchestration/observe/detector_historical_test.go`
- **覆盖 REQ**: REQ-D7-OBS-005
- **覆盖 AC**: AC5
- **测试用例**:
  - `TestHistoricalDetector_ZScore_Above_2_CatBusiness`
  - `TestHistoricalDetector_ZScore_Below_2_NoDeviation`
  - `TestHistoricalDetector_Window_24h_Default`
  - `TestHistoricalDetector_EmptyBaseline_Defaults`

#### D7-S8-A20-T03: LLMClaimDetector 不调 tool

- **优先级**: P0
- **位置**: `internal/layers/orchestration/observe/detector_llm_claim_test.go`
- **覆盖 REQ**: REQ-D7-OBS-005
- **覆盖 AC**: AC6
- **测试用例**:
  - `TestLLMClaimDetector_NoToolAccess` — 验证 AllowedTools=nil
  - `TestLLMClaimDetector_SystemPrompt_DisallowsTools`
  - `TestLLMClaimDetector_ClaimVsEvidence_Diverge`

#### D7-S8-A20-T04: OP-6 业务/系统异常反向校验

- **优先级**: P0
- **位置**: `internal/layers/orchestration/observe/anomaly_detector_test.go`
- **覆盖 REQ**: REQ-D7-OBS-006
- **覆盖 AC**: AC7
- **测试用例**:
  - `TestOP6_CatSystem_Revalidation_ZScoreAbove2_Reclassified`
  - `TestOP6_CatSystem_DetectorSystemHealth_Kept`
  - `TestOP6_CatSystem_ZScoreBelow2_Kept`
  - `TestOP6_LogWarn_OnMisclassify`

---

### Module A21: ObserveNode + ProcessMessage wiring（PR-A4）

#### D7-S8-A21-T01: ObserveNode.All() P95 ≤ 50ms

- **优先级**: P0
- **位置**: `internal/layers/orchestration/observe/observe_node_test.go`
- **覆盖 REQ**: REQ-D7-OBS-007
- **覆盖 AC**: AC8
- **测试用例**:
  - `TestObserveNode_All_P95` — 1000 并发 + percentile 计算
  - `TestObserveNode_All_PrometheusMetric` — d7_observe_p95_ms 打点
  - `TestObserveNode_All_QuantizerAsync_NotInP95`

#### D7-S8-A21-T02: ObserveNode.Receive prior LP-1 闭环

- **优先级**: P0
- **位置**: `internal/layers/orchestration/observe/observe_node_test.go`
- **覆盖 REQ**: REQ-D7-OBS-008
- **覆盖 AC**: AC9
- **测试用例**:
  - `TestObserveNode_All_WithPrior_AdaptivePriorInjected`
  - `TestObserveNode_All_NilPrior_OK`
  - `TestObserveNode_All_PriorInfluencesQuantizer`
  - `TestObserveNode_All_PriorScoreFromReputationStore`

#### D7-S8-A21-T03: Orchestrator:ProcessMessage 集成

- **优先级**: P0
- **位置**: `internal/layers/orchestration/sessionorchestrator/orchestrator_test.go`（扩展）
- **覆盖 REQ**: REQ-D7-INT-001
- **覆盖 AC**: AC16
- **测试用例**:
  - `TestOrchestrator_ProcessMessage_ObservePlanInserted`
  - `TestOrchestrator_ObserveError_Handled`
  - `TestOrchestrator_PlanError_Handled`
  - `TestOrchestrator_DispatchPlan_BackwardCompatible`

---

### Module A22: Plan 4 类 + Planner interface（PR-B1）

#### D7-S8-A22-T01: Plan 4 类 enum + 互斥

- **优先级**: P0
- **位置**: `internal/layers/orchestration/plan/plan_test.go`
- **覆盖 REQ**: REQ-D7-PLN-001
- **覆盖 AC**: AC10
- **测试用例**:
  - `TestPlan_4Kinds_EnumExclusive`
  - `TestPlan_Kind_MarshalJSON`
  - `TestPlan_Kind_UnmarshalJSON`

#### D7-S8-A22-T02: SourceObservationIDs 必填 + 血缘

- **优先级**: P0
- **位置**: `internal/layers/orchestration/plan/plan_test.go`
- **覆盖 REQ**: REQ-D7-PLN-002
- **覆盖 AC**: AC11
- **测试用例**:
  - `TestPlan_SourceObservationIDs_Required` — 空时 Validate 失败
  - `TestPlan_SourceObservationIDs_ReverseLookup` — 按 ID 找到 Observation
  - `TestPlan_SourceObservationIDs_DuplicateAllowed`

#### D7-S8-A22-T03: Kind 匹配规则

- **优先级**: P0
- **位置**: `internal/layers/orchestration/plan/plan_kind_test.go`
- **覆盖 REQ**: REQ-D7-PLN-001
- **覆盖 AC**: AC10
- **测试用例**:
  - `TestPlan_MatchKind_4Rules` — 4 种主导 Kind → 4 种 PlanKind
  - `TestPlan_MatchKind_TieBreak_UncertaintyFirst`
  - `TestPlan_MatchKind_EmptyObservations_Fallback`
  - `TestPlan_MatchKind_OnlyCatSystem_Fallback`

---

### Module A23: Plan.Validate() + 3 项强制约束（PR-B2）

#### D7-S8-A23-T01: PP-1 强度匹配

- **优先级**: P0
- **位置**: `internal/layers/orchestration/plan/plan_validator_test.go`
- **覆盖 REQ**: REQ-D7-PLN-003
- **覆盖 AC**: AC12
- **测试用例**:
  - `TestPlan_Validate_PP1_StrengthMismatch`
  - `TestPlan_Validate_PP1_ExactlyMin_OK`
  - `TestPlan_Validate_PP1_NoBusinessObs_NoConstraint`
  - `TestPlan_Validate_PP1_IgnoresCatSystem` — 关键 OP-6 验证

#### D7-S8-A23-T02: PP-2 可证伪性

- **优先级**: P0
- **位置**: `internal/layers/orchestration/plan/plan_validator_test.go`
- **覆盖 REQ**: REQ-D7-PLN-004
- **覆盖 AC**: AC13
- **测试用例**:
  - `TestPlan_Validate_PP2_FailureCriteriaEmpty`
  - `TestPlan_Validate_PP2_OpNotInWhitelist`
  - `TestPlan_Validate_PP2_FieldNotObservable`
  - `TestPlan_Validate_PP2_AllValid_Pass`

#### D7-S8-A23-T03: PP-3 爆炸半径

- **优先级**: P0
- **位置**: `internal/layers/orchestration/plan/plan_validator_test.go`
- **覆盖 REQ**: REQ-D7-PLN-005
- **覆盖 AC**: AC14
- **测试用例**:
  - `TestPlan_Validate_PP3_FileCountExceeded`
  - `TestPlan_Validate_PP3_APICallCountExceeded`
  - `TestPlan_Validate_PP3_BothExceeded_FailFast`
  - `TestPlan_Validate_PP3_BoundaryOK`

---

### Module A24: BlastRadiusCalculator + DefaultPlanner（PR-B3）

#### D7-S8-A24-T01: BlastRadiusCalculator 4 维度

- **优先级**: P0
- **位置**: `internal/layers/orchestration/plan/blast_radius_test.go`
- **覆盖 REQ**: REQ-D7-PLN-006
- **覆盖 AC**: AC15
- **测试用例**:
  - `TestBlastRadius_Calculate_FileCount`
  - `TestBlastRadius_Calculate_APICallCount`
  - `TestBlastRadius_Calculate_TokenCost`
  - `TestBlastRadius_Calculate_PersistScope`
  - `TestBlastRadius_Calculate_EmptyPlan`

#### D7-S8-A24-T02: DefaultPlanner 全链路

- **优先级**: P0
- **位置**: `internal/layers/orchestration/plan/planner_test.go`
- **覆盖 REQ**: REQ-D7-PLN-006
- **覆盖 AC**: AC15
- **测试用例**:
  - `TestDefaultPlanner_Plan_Commitment` — ObsFact → CommitmentPlan
  - `TestDefaultPlanner_Plan_StrengthCapped` — min(LLM, obs, floor)
  - `TestDefaultPlanner_Plan_SourceObservationIDs_Populated`
  - `TestDefaultPlanner_Plan_ValidationFailure_ReturnsError`
  - `TestDefaultPlanner_Plan_AnomaliesCount_Populated`

#### D7-S8-A24-T03: AnomaliesCount 字段

- **优先级**: P0
- **位置**: `internal/layers/orchestration/plan/planner_test.go`
- **覆盖 REQ**: REQ-D7-PLN-007
- **覆盖 AC**: AC17
- **测试用例**:
  - `TestPlan_AnomaliesCount_FromReport`
  - `TestPlan_AnomaliesCount_ZeroWhenNoAnomalies`
  - `TestPlan_AnomaliesCount_PropagatesToArtifact`（Phase 3 衔接点）

#### D7-S8-A24-T04: SourceObservationIDs 反向追溯链集成

- **优先级**: P0
- **位置**: `tests/integration/d7/source_observation_chain_test.go`
- **覆盖 REQ**: REQ-D7-INT-002
- **覆盖 AC**: AC18
- **测试用例**:
  - `TestSourceObservationChain_PlanToObservation_ReverseLookup`
  - `TestSourceObservationChain_ArtifactInheritsPlanSource`（Phase 3 占位）
  - `TestSourceObservationChain_FullChainEnd2End`

---

## T 点统计

| Module | T 点数 | PR | 覆盖 AC |
|--------|--------|-----|---------|
| A15 Observation + UncertaintyReport | 6 | PR-A1 | AC1, AC2, AC3 |
| A19 IntentQuantizer | 4 | PR-A2 | AC4 |
| A20 AnomalyDetector | 4 | PR-A3 | AC5, AC6, AC7 |
| A21 ObserveNode + Wiring | 3 | PR-A4 | AC8, AC9, AC16 |
| A22 Plan 4 类 + Planner interface | 3 | PR-B1 | AC10, AC11 |
| A23 Plan.Validate() + 3 项强制约束 | 3 | PR-B2 | AC12, AC13, AC14 |
| A24 BlastRadiusCalculator + DefaultPlanner | 4 | PR-B3 | AC15, AC17, AC18 |
| **合计** | **18 + 9 子用例** | **6 PR** | **18 AC** |

注：T 点数 = 18（每个 module 1 个或多个 T 编号），子用例数 = ~49（每个 T 编号下含多个 test 用例）

---

## 落地后状态

PR 全部合入 master 后：

1. `openspec/specs/d7-orchestration/t-registry.md` 中新增 18 个 T 编号条目
2. 每个 T 点对应 1-N 个 Go 测试函数
3. S4-Gate 通过：`go test -race -cover ./internal/layers/orchestration/...` 覆盖率 ≥ 80%
4. S5-Gate 通过：18 个 T 编号全部 P0 PASS
5. S6 归档：`openspec/archive/2026-06-25-devrix-d7-mups-v4-phase2-observe-plan/t-registry.md`（PLANNED → IMPLEMENTED）

---

## Cross-references

- 设计稿：`openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/design.md` §11 测试矩阵
- 任务清单：`openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/tasks.md`
- Spec Delta：`openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/specs/d7-orchestration/spec_delta.md`
- 提案：`openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/proposal.md`
- 目标 T-Registry：`openspec/specs/d7-orchestration/t-registry.md`（Phase 1 已 IMPLEMENTED 部分保留）