# Tasks: D7 MUPS v4.3 Phase 2 — Observe + Plan 节点落地

**Change ID:** `devrix-d7-mups-v4-phase2-observe-plan`
**Demand ID:** DM-20260623-001
**Status:** S2_Proposal
**Date:** 2026-06-23
**Author:** MUPS v4.3 落地梳理

---

## PR-RF: PR-A1 Design Review 反馈修复（DM-20260623-001 review fix）

> 本节为 S3-Gate pre-review 决议的落地任务（详见 `design.md` §14 Review Decisions）。
> 所有 5 项代码变更 + 6 个新测试用例必须在 PR-A1 实现阶段同步完成，确保 code 与 design.md 零偏差。

### RF.1 Critical 修复（block PR-A1）

- [ ] **RF.1.1** 改 `internal/layers/orchestration/orchtypes/uncertainty_report.go`
  - `QuantizedIntent` struct: `Kind string` → `Kind IntentKind`（C1 决议）
  - 既有调用点零修改验证：`go build ./...` + `go test ./internal/layers/orchestration/...`
- [ ] **RF.1.2** 改 `internal/layers/orchestration/orchtypes/uncertainty_coord.go`
  - `FromVerifier` 函数：未知 verdict 改 fail-fast：
    ```go
    default:
        return UncertaintyCoord{}, NewUncertaintyCoordInvalidVerdictKindError(verdictKind)
    ```
  - 4 种已知 verdict（pass/partial/indeterminate/fail）行为不变（C3 决议）
- [ ] **RF.1.3** 改 `internal/layers/orchestration/plan/planner.go`（PR-B1 同步）
  - `MatchKind` 签名：`MatchKind(observations []Observation) PlanKind` → `MatchKind(report *orchtypes.UncertaintyReport) PlanKind`
  - 函数体内只读 `report.BusinessObservations`，不读 `report.Observations` 或 `report.SystemObservations`（C2 + W8 决议）
  - 注释显式说明误用 `report.Observations` 会静默降级为 ExplorationPlan

### RF.2 Warning 修复（应在 PR-A1 同步完成）

- [ ] **RF.2.1** 改 `internal/layers/orchestration/orchtypes/observation.go`
  - `MarshalJSON` wire format：`{id, kind, category, strength, payload: {…}, detected_at, source}` 嵌套对象（W1 决议）
  - `UnmarshalJSON` 按 Kind 判别反序列化到对应 Payload 类型
  - `validateFact`：`return ErrObservationPayloadInvalid` → `return fmt.Errorf("orchtypes: FactPayload.Statement empty: %w", ErrObservationPayloadInvalid)`（W2 决议）
- [ ] **RF.2.2** 改 `internal/layers/orchestration/orchtypes/uncertainty_report.go`
  - `Partition` 末尾：`r.Overall = r.ComputeOverallStrength()` → `r.Overall = clamp01Float(r.ComputeOverallStrength(), 0.5)`（W6/I8 决议）
- [ ] **RF.2.3** 合并 `clamp01` + `clamp01Coord` 为 `clamp01Float(v float64, onNaN float64) float64`
  - 改 `internal/layers/orchestration/orchtypes/observation.go` 删除 `clamp01` 旧实现，改用 `clamp01Float`
  - 改 `internal/layers/orchestration/orchtypes/uncertainty_coord.go` 删除 `clamp01Coord` 旧实现，改用 `clamp01Float`
  - onNaN 默认 0.5（与 UncertaintyCoord.Value 冷启动默认值对齐）（W3 决议）

### RF.3 新增测试用例（AC9 验收）

- [ ] **RF.3.1** 在 `orchtypes/observation_test.go` 增 3 个测试
  - `TestObservation_MarshalJSON_WireFormat` — payload 嵌套对象 roundtrip（W1）
  - `TestObservation_ValidateFact_WrappedError` — 验证 `errors.Is(err, ErrObservationPayloadInvalid)` 为 true（W2）
  - `TestClamp01Float_NaN_Fallback` — NaN → onNaN 默认 0.5（W3）
- [ ] **RF.3.2** 在 `orchtypes/uncertainty_report_test.go` 增 2 个测试
  - `TestUncertaintyReport_Overall_NaN_Fallback` — ComputeOverallStrength 返回 NaN → Overall clamp 到 0.5（W6/I8）
  - `TestUncertaintyReport_QuantizedIntent_KindType` — Kind 字段为 IntentKind 枚举而非 string（C1）
- [ ] **RF.3.3** 在 `orchtypes/uncertainty_coord_test.go` 增 1 个测试
  - `TestUncertaintyCoord_FromVerifier_UnknownKind` — 未知 verdict 返回 `NewUncertaintyCoordInvalidVerdictKindError`（C3）

### RF.4 验收检查（AC10/AC11）

- [ ] **RF.4.1** `go vet ./...` 0 issue（AC10）
- [ ] **RF.4.2** `go test -race ./internal/layers/orchestration/orchtypes/` 全部 PASS（AC9）
- [ ] **RF.4.3** `go test -cover` 覆盖率不低于 PR-A1 现状 72.2%（AC11）
- [ ] **RF.4.4** code change 与 `design.md` §14.4 决议闭环清单逐项核对（零偏差）

### RF.5 文档同步

- [ ] **RF.5.1** `openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/design.md` §14 已落地（✅ 2026-06-23）
- [ ] **RF.5.2** `openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/demand.md` AC1-AC11 已与 §14 决议对齐（✅ 2026-06-23）
- [ ] **RF.5.3** `openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/proposal.md` DM ID + 风险表已更新（✅ 2026-06-23）

### RF.6 S6 Archive 更新（PR-RF 单独归档）

- [ ] PR-RF 单独走 S4 → S4-Gate → S5 → S6 流程（与 PR-A1 合并为一个 PR 或拆为独立 PR，由 reviewer 在 S3-Gate 决议）
- [ ] 建议方案：**拆为独立 PR-RF**（5 项 code change + 6 个测试用例 vs PR-A1 的 4 个核心文件落地，作用域不同，单独走 review 更清晰）
- [ ] 若拆为独立 PR：DM-20260623-001 review fix 作为独立 change 归档到 `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase2-observe-plan-pr-rf/`
- [ ] 若合并 PR：DM-20260623-001 与 PR-A1 共用一个 change 目录，归档时间线延后到 PR-A1 全部落地

---

---

## Phase 0: Setup

- [ ] 创建 change 目录 `openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/`
- [ ] `proposal.md`（S2，已完成）
- [ ] `tasks.md`（S4，本文）
- [ ] `design.md`（S3，详细 Go 代码 + 跨域契约）
- [ ] `specs/d7-orchestration/spec_delta.md`（Gherkin 验收，18 个 ADDED Requirement）
- [ ] `specs/d7-orchestration/t-registry_delta.md`（新增 18 个 P0 T 点）
- [ ] 分支 `feat/devrix-d7-mups-v4-phase2-observe-plan master`
- [ ] `verify-archive.sh` baseline 通过（Phase 1 archive 已存在）

---

## PR-A1: UncertaintyReport + Observation 4 类 + UncertaintyCoord 扩展

### A1.1 Observation struct + 4 类枚举

- [ ] **A1.1.1** 新建文件 `internal/layers/orchestration/orchtypes/observation.go`
  - 定义 `Category` enum（CatBusiness/CatSystem）
  - 定义 `ObservationKind` enum（ObsFact/ObsSignal/ObsDeviation/ObsUncertainty）
  - 定义 `Observation` struct（ID/Kind/Category/Strength/Payload/DetectedAt/Source）
  - 实现 `WithKind()` / `WithStrength()` 等不可变更新方法
- [ ] **A1.1.2** 新建 `orchtypes/observation_test.go`
  - `TestObservation_4Kinds_4Categories` — 4×2 组合互斥
  - `TestObservation_Immutability` — With* 方法返回新副本
  - `TestObservation_Payload_TypeAssertion` — Kind-specific payload 断言

### A1.2 UncertaintyReport 聚合

- [ ] **A1.2.1** 新建文件 `internal/layers/orchestration/orchtypes/uncertainty_report.go`
  - 定义 `UncertaintyReport` struct（SessionID/Observations/BusinessObservations/SystemObservations/Anomalies/Overall/QuantizedIntent/Prior）
  - 实现 `NewUncertaintyReport(observations []Observation)` 构造函数
  - 实现 `Partition()` 方法按 Category 拆分 BusinessObservations/SystemObservations
  - 实现 `FilterByKind(kind ObservationKind) []Observation`（注意：故意遍历全集，按 Kind 切分不按 Category）
  - 实现 `ComputeOverallStrength() float64` 只遍历 BusinessObservations
  - 实现 `AddObservation(obs Observation)` 不可变方法（返回新 report）
- [ ] **A1.2.2** 新建 `orchtypes/uncertainty_report_test.go`
  - `TestUncertaintyReport_PartitionInvariant` — BusinessObservations ∪ SystemObservations = Observations
  - `TestUncertaintyReport_ComputeOverallStrength_BusinessOnly` — 不污染业务路径
  - `TestUncertaintyReport_FilterByKind` — 故意遍历全集
  - `TestUncertaintyReport_AddObservation_Immutable` — 返回新副本
  - `TestUncertaintyReport_Anomalies_SubsetOfObsDeviation`

### A1.3 UncertaintyCoord 扩展（Phase 1 增量）

- [ ] **A1.3.1** 修改 `internal/layers/orchestration/orchtypes/uncertainty_coord.go`
  - 在 `UncertaintyCoord` struct 增加 2 字段：
    - `FromVerifier bool` — Phase 4 Verifier 输出回写时标记
    - `SideEffectStatus SideEffectStatus` — Phase 3 副作用状态传播（先用 type alias，Phase 3 落地具体 enum）
  - 增加工厂方法 `FromVerifier(verdict VerdictKind, confidence float64, reason string) UncertaintyCoord`
  - 在 `MarshalJSON` / `UnmarshalJSON` 中用 `omitempty` 保证向后兼容
- [ ] **A1.3.2** 在 `orchtypes/uncertainty_coord_test.go` 增补
  - `TestUncertaintyCoord_FromVerifier` — Verifier 输出的 4 态正确投影
  - `TestUncertaintyCoord_JSON_Compatibility_Phase1Fields` — 旧 JSON 仍可解析
  - `TestUncertaintyCoord_JSON_NewFields_OmitEmpty` — 新字段缺省时序列化省略

---

## PR-A2: IntentQuantizer 多轮收敛

### A2.1 IntentQuantizer 主体

- [ ] **A2.1.1** 新建文件 `internal/layers/orchestration/decisionplanning/intent_quantizer.go`
  - 定义 `IntentQuantizer` struct（LLMCompleter/MaxRounds=3/PerRoundTimeoutMs=2000）
  - 定义 `IntentPayload` struct（Kind/Confidence=0.0~1.0/Reason/Rounds/Source）
  - 实现 `Quantize(ctx, message string, prior *AdaptivePrior) (*IntentPayload, error)`
  - 3 轮循环：
    1. LLM 自报 Kind + Reason + Confidence
    2. evidence 客观信号交叉（grep baseline 历史）
    3. AdaptivePrior 加权（**AdaptivePrior 类型先定义在 learn 包，Phase 2 先 stub**）
- [ ] **A2.1.2** 实现兜底 `ErrIntentUnquantifiable`（3 轮仍不收敛）+ route to `IntentOrchestrate`
- [ ] **A2.1.3** 新建 `decisionplanning/intent_quantizer_test.go`
  - `TestIntentQuantizer_3Rounds_Success` — 正常 3 轮收敛
  - `TestIntentQuantizer_1Round_FastPath` — 高 Confidence 1 轮即收敛
  - `TestIntentQuantizer_Timeout_Fallback` — 单轮超时 → ErrIntentUnquantifiable
  - `TestIntentQuantizer_LLMCompleter_NilSafe` — nil receiver / nil LLM 不 panic
  - `TestIntentQuantizer_PerRoundTimeout_P95` — 性能测试单轮 P95 ≤ 2s

### A2.2 IntentPayload 与现有 IntentClassification 兼容

- [ ] **A2.2.1** 在 `decisionplanning/classifier.go` 增加转换函数
  - `func (ip *IntentPayload) ToIntentClassification() IntentClassification` — Phase 2 过渡用，Phase 3 退役
- [ ] **A2.2.2** 测试 `TestIntentPayload_ToIntentClassification_ConfidenceConversion`

---

## PR-A3: AnomalyDetector 4 实现 + Composite

### A3.1 AnomalyDetector interface

- [ ] **A3.1.1** 新建文件 `internal/layers/orchestration/observe/anomaly_detector.go`
  - 定义 `AnomalyDetector` interface（Name/Detect）
  - 定义 `DeviationPayload` struct（DetectorID/Category/ZScore/Expected/Actual/Evidence/SuggestedKind）
  - 定义 `CompositeAnomalyDetector` struct（Detectors []AnomalyDetector）
  - 实现 `NewCompositeAnomalyDetector()` 默认 4 实现
  - 实现 `Composite.Detect(ctx, baseline, current)` 用 `errgroup.Go` 并行
  - 单 detector 失败 → log + 继续（不影响整体）
- [ ] **A3.1.2** 新建 `observe/anomaly_detector_test.go`
  - `TestCompositeAnomalyDetector_AllDetect` — 4 detector 全部触发
  - `TestCompositeAnomalyDetector_SingleFails_ContinueOthers` — 单 detector panic/fail 不影响其他
  - `TestCompositeAnomalyDetector_Empty` — 无 detector 返回空

### A3.2 HistoricalDetector（Z-score）

- [ ] **A3.2.1** 新建文件 `internal/layers/orchestration/observe/detector_historical.go`
  - 定义 `HistoricalDeviationDetector` struct（Window=24h 默认）
  - 实现 `Detect(ctx, baseline, current)` 计算 Z-score
  - Z-score > 2.0 → Category=CatBusiness + SuggestedKind=ObsDeviation
- [ ] **A3.2.2** 测试 `TestHistoricalDetector_ZScore_Above_2` + `TestHistoricalDetector_Window_24h`

### A3.3 StructuralDetector（结构对比）

- [ ] **A3.3.1** 新建文件 `internal/layers/orchestration/observe/detector_structural.go`
  - 实现 D7-S1 WorkItem 结构 diff
- [ ] **A3.3.2** 测试 `TestStructuralDetector_WorkItem_Diff`

### A3.4 LLMClaimDetector（自报 vs 客观）

- [ ] **A3.4.1** 新建文件 `internal/layers/orchestration/observe/detector_llm_claim.go`
  - 定义 `LLMClaimDeviationDetector` struct（LLMCompleter）
  - 实现 `Detect(ctx, claim, evidence)` 调 LLM 对比
  - **关键约束**：调用 LLM 时 `AllowedTools: nil`（避免 detector 调 tool 递归）
- [ ] **A3.4.2** system prompt 强约束：明确"你不能调用任何 tool，只输出 JSON"
- [ ] **A3.4.3** 测试
  - `TestLLMClaimDetector_NoToolAccess` — 验证 AllowedTools=nil
  - `TestLLMClaimDetector_ClaimVsEvidence_Diverge`

### A3.5 EvidenceDetector（多源交叉）

- [ ] **A3.5.1** 新建文件 `internal/layers/orchestration/observe/detector_evidence.go`
  - 多源证据（WorkItem + ToolResult + EngineEvent）交叉
- [ ] **A3.5.2** 测试 `TestEvidenceDetector_MultiSource_Crosscheck`

### A3.6 OP-6 业务/系统异常分离

- [ ] **A3.6.1** 在 `observe/anomaly_detector.go` 加 `RevalidateCategory(payload DeviationPayload) Category`
  - 规则：CatSystem 中 `DetectorID != "system_health"` 且 `ZScore > 2.0` → 重分类 CatBusiness
  - 重分类时发出 `slog.Warn("observe.category_misclassify")`
- [ ] **A3.6.2** 测试 `TestOP6_CatSystem_Revalidation_ZScoreAbove2_Reclassified`

---

## PR-A4: ObserveNode + ProcessMessage wiring

### A4.1 ObserveNode interface

- [ ] **A4.1.1** 新建文件 `internal/layers/orchestration/observe/observe_node.go`
  - 定义 `ObserveRequest` struct（SessionID/UserMessage/IntentKind/Prior）
  - 定义 `ObserveNode` interface（All）
  - 定义 `DefaultObserveNode` struct（Quantizer/AnomalyDetectors/BaselineStore）
  - 实现 `DefaultObserveNode.All(ctx, req) (*UncertaintyReport, error)`
  - 内部 `errgroup.Go` 并行：Quantizer + AnomalyDetector
  - P95 ≤ 50ms（除 IntentQuantizer 异步路径）
- [ ] **A4.1.2** 新建 `observe/observe_node_test.go`
  - `TestObserveNode_All_P95` — 性能测试
  - `TestObserveNode_All_WithPrior` — LP-1 闭环（prior 注入 AdaptivePrior）
  - `TestObserveNode_All_QuantizerFailure_Fallback` — Quantizer 失败但 AnomalyDetector 仍返回

### A4.2 ProcessMessage wiring

- [ ] **A4.2.1** 修改 `internal/layers/orchestration/sessionorchestrator/orchestrator.go:ProcessMessage`
  - 在 classifier 之后（约 line 295）插入：
    ```go
    observeReq := observe.ObserveRequest{
        SessionID:   req.SessionID,
        UserMessage: req.Message,
        IntentKind:  intent.Kind,
        Prior:       o.reputationStore.GetPrior(req.SessionID),  // LP-1 闭环
    }
    report, err := o.observeNode.All(ctx, observeReq)
    if err != nil {
        return o.handleObserveError(err, req)
    }
    plan, err := o.planner.Plan(ctx, report)
    ```
  - 严格位置：classifier 之后，dispatcher 之前
- [ ] **A4.2.2** 修改 `internal/layers/orchestration/cmd/devrix/main.go`
  - DI 注入 ObserveNode + CompositeAnomalyDetector + IntentQuantizer
  - baseline store 初始化（从 `workmodel/cross_session.go:SessionReputation` 复用）

### A4.3 T 编号 D7-S8-A21 注册

- [ ] **A4.3.1** 在 `specs/d7-orchestration/t-registry_delta.md` 新增
  - `D7-S8-A21-T01` ObserveNode.All() P95 ≤ 50ms
  - `D4.3.2` 在测试中加 d7_observe_p95_ms Prometheus 指标

---

## PR-B1: Plan 4 类 + Planner interface

### B1.1 Plan struct

- [ ] **B1.1.1** 新建目录 `internal/layers/orchestration/plan/`
- [ ] **B1.1.2** 新建文件 `internal/layers/orchestration/plan/plan_kind.go`
  - 定义 `PlanKind` enum（CommitmentPlan/ProtocolPlan/ScenarioPlan/ExplorationPlan）
  - 实现 `String()` + `MarshalJSON` + `UnmarshalJSON`
- [ ] **B1.1.3** 新建文件 `internal/layers/orchestration/plan/plan.go`
  - 定义 `Plan` struct（ID/Kind/Strength/Steps/FailureCriteria/BlastRadius/SourceObservationIDs/AnomaliesCount/CreatedAt）
  - 定义 `PlanStep` struct（Index/ToolName/Parameters/IdempotencyKey/RetryPolicy）
  - 定义 `FailureCriterion` struct（Field/Op/Value）
  - 定义 `BlastRadius` struct（FileCount/APICallCount/TokenCost/PersistScope）
  - 实现 `WithKind()` / `WithSteps()` 等不可变更新方法
- [ ] **B1.1.4** 新建 `plan/plan_test.go`
  - `TestPlan_4Kinds` — 4 枚举互斥
  - `TestPlan_SourceObservationIDs_Required` — 必填校验
  - `TestPlan_AnomaliesCount_FieldExists`
  - `TestPlan_Immutability`

### B1.2 Kind 匹配规则

- [ ] **B1.2.1** 在 `plan/plan_kind.go` 实现 `MatchKind(observations []Observation) PlanKind`
  - ObsFact 主导 → CommitmentPlan
  - ObsSignal 主导 → ProtocolPlan
  - ObsDeviation 主导 → ScenarioPlan
  - ObsUncertainty 主导 → ExplorationPlan
  - 平局时优先级：Uncertainty > Deviation > Signal > Fact
- [ ] **B1.2.2** 测试 `TestPlan_MatchKind_4Rules` + `TestPlan_MatchKind_TieBreak`

---

## PR-B2: Plan.Validate() + 3 项强制约束

### B2.1 PlanValidator

- [ ] **B2.1.1** 新建文件 `internal/layers/orchestration/plan/plan_validator.go`
  - 定义 `PlanValidator` struct（MaxBlastRadius config）
  - 实现 `Validate(plan *Plan, observations []Observation) error`
  - 顺序执行 3 项检查：
    1. `checkStrength(plan, observations)` — PP-1
    2. `checkFalsifiability(plan)` — PP-2
    3. `checkBlastRadius(plan)` — PP-3
  - 任一失败立即返回（fail-fast）
- [ ] **B2.1.2** 在 `plan/plan.go` 增加 `func (p *Plan) Validate(observations []Observation) error` 便捷方法

### B2.2 PP-1 StrengthMatch

- [ ] **B2.2.1** 新建文件 `internal/layers/orchestration/plan/strength_match.go`
  - 实现 `ValidateStrength(plan, businessObs []Observation) error`
  - `plan.Strength ≤ min(businessObs.Strength)`（只遍历 BusinessObservations，不污染）
- [ ] **B2.2.2** 测试 `TestPlan_Validate_PP1_StrengthMismatch`

### B2.3 PP-2 可证伪性

- [ ] **B2.3.1** 在 `plan/plan_validator.go` 实现 `checkFalsifiability(plan)`
  - `FailureCriteria` 非空
  - `Op` 在白名单 `["eq", "ne", "lt", "gt", "contains", "matches"]`
  - `Field` 在 ExecutionEvidence 可观测字段集（先用常量定义，Phase 3 Execute 落地后扩展）
- [ ] **B2.3.2** 测试 `TestPlan_Validate_PP2_Falsifiability` + `TestPlan_Validate_PP2_EmptyFailureCriteria`

### B2.4 PP-3 爆炸半径

- [ ] **B2.4.1** 在 `plan/plan_validator.go` 实现 `checkBlastRadius(plan)`
  - `FileCount > MaxBlastRadius.FileCount` → `ErrBlastRadiusExceeded`
  - `APICallCount > MaxBlastRadius.APICallCount` → 同上
- [ ] **B2.4.2** 测试 `TestPlan_Validate_PP3_BlastRadiusExceeded`

### B2.5 SentinelError

- [ ] **B2.5.1** 在 `plan/errors.go` 定义 11 个 SentinelError（含 PP-1/2/3 + SourceObservationIDs 缺失 + PlanKind 未知 + Kind 匹配失败等）
- [ ] **B2.5.2** 测试每个 SentinelError 触发路径

---

## PR-B3: BlastRadiusCalculator + DefaultPlanner

### B3.1 BlastRadiusCalculator

- [ ] **B3.1.1** 新建文件 `internal/layers/orchestration/plan/blast_radius.go`
  - 定义 `BlastRadiusCalculator` struct
  - 实现 `Calculate(plan *Plan) BlastRadius`
  - 估算规则：
    - FileCount = Steps 中 file 类 tool × avg_files_per_call
    - APICallCount = Steps 中 http 类 tool count
    - TokenCost = Steps 总字符数 × 系数
    - PersistScope = 默认 "session"，涉及 user/global 数据时升级
- [ ] **B3.1.2** 测试 `TestBlastRadius_Calculate_4Dimensions`

### B3.2 Planner interface

- [ ] **B3.2.1** 新建文件 `internal/layers/orchestration/plan/planner.go`
  - 定义 `Planner` interface（Plan）
  - 定义 `DefaultPlanner` struct（LLMDecomposer/Validator/BlastCalc）
  - 实现 `DefaultPlanner.Plan(ctx, report) (*Plan, error)`
  - 流程：
    1. Kind 匹配
    2. LLMTaskDecomposer 生成 Steps（**复用** Phase 1 的 decisionplanning.LLMDecomposer）
    3. BlastRadiusCalculator 估算
    4. Plan.Validate() 强制 3 项约束
    5. SourceObservationIDs 填充（从 report.Observations）
- [ ] **B3.2.2** 新建 `plan/planner_test.go`
  - `TestDefaultPlanner_Plan_Commitment` — ObsFact → CommitmentPlan
  - `TestDefaultPlanner_Plan_StrengthCapped` — Strength ≤ min(BusinessObs.Strength)
  - `TestDefaultPlanner_Plan_SourceObservationIDs_Populated`
  - `TestDefaultPlanner_Plan_ValidationFailure_ReturnsError`

---

## PR-A4 (扩展): Orchestrator:ProcessMessage 完整 wiring

> **PR-A4 已包含 ObserveNode wiring，本节扩展 Plan wiring**

- [ ] **B3.3.1** 修改 `internal/layers/orchestration/sessionorchestrator/orchestrator.go`
  - 在 ObserveNode.All() 之后插入：
    ```go
    plan, err := o.planner.Plan(ctx, report)
    if err != nil {
        return o.handlePlanError(err, req)
    }
    // Phase 3 替换为 Executor.Execute(plan)
    // Phase 2 暂时走原 dispatcher（向后兼容）
    return o.dispatchPlan(req, plan, intent)
    ```
- [ ] **B3.3.2** `dispatchPlan()` 兼容旧路径：plan.Steps → 转换为 `[]Task` 给现有 WaveScheduler
- [ ] **B3.3.3** 测试 `TestOrchestrator_ObservePlan_Integration`（E2E）

---

## PR 全量集成测试

- [ ] **INT-1** 新建 `tests/integration/d7/observe_plan_pipeline_test.go`
  - E2E: ProcessMessage → ObserveNode.All → Planner.Plan → Plan.Validate 通过
- [ ] **INT-2** 新建 `tests/integration/d7/observe_with_prior_test.go`
  - LP-1 闭环：sessionID 有 prior ReputationEvidence → ObserveNode.Receive 注入
- [ ] **INT-3** 新建 `tests/integration/d7/op6_category_separation_test.go`
  - CatSystem 异常被反向校验重分类
- [ ] **INT-4** 新建 `tests/integration/d7/plan_validation_pipeline_test.go`
  - PP-1/2/3 全部失败场景

---

## S6 Archive（Phase 2 落地后）

### Phase 2 review fix（PR-RF，DM-20260623-001）

- [ ] PR-RF 单独走 S4 → S4-Gate → S5 → S6 流程（与 PR-A1 拆为独立 PR，作用域清晰）
- [ ] 归档到 `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase2-observe-plan-pr-rf/`
- [ ] `scripts/verify-archive.sh` 全部通过（仅 PR-RF 范围）
- [ ] PR-RF 不动 spec.md / t-registry.md（仅 design.md §14 同步）

### Phase 2 主链路（PR-A1 → PR-B3）

- [ ] PR-A1 → PR-A2 → PR-A3 → PR-A4 → PR-B1 → PR-B2 → PR-B3 顺序合入 master（squash auto-merge）
- [ ] 归档到 `openspec/archive/2026-06-25-devrix-d7-mups-v4-phase2-observe-plan/`
- [ ] `scripts/verify-archive.sh` 全部通过
- [ ] 更新 `openspec/specs/d7-orchestration/spec.md` + `t-registry.md`（PLANNED → IMPLEMENTED）
- [ ] 关闭 Change（DM-20260623-001 状态：✅ S7_Archived）