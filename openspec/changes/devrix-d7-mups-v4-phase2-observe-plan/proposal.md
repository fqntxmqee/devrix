# Proposal: D7 MUPS v4.3 Phase 2 — Observe 节点 + Plan 节点落地

**Change ID:** `devrix-d7-mups-v4-phase2-observe-plan`
**Demand ID:** DM-20260623-001
**Status:** S2_Proposal
**Priority:** P0
**Date:** 2026-06-23
**Author:** MUPS v4.3 落地梳理 (doc 42 §Observe + doc 43 §Plan + doc 47 Phase 2)

---

## 1. Background

MUPS v4.3 5 节点管道（Observe → Plan → Execute → Verify → Learn）的设计稿已完整沉淀（doc 42-46 + doc 47 落地方案）。Phase 1 已落地 4 个 P0 修复点（A15 UncertaintyCoord + A16 AdaptiveThreshold wiring + A17 Verifier scaffold + A18 ExitReason 扩展，DM-20260623-001）。

**Phase 2 是 5 Phase 落地的第二步**，把 [[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/42-d7-observe-node-design|doc 42 Observe 节点]] + [[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/43-d7-plan-node-design|doc 43 Plan 节点]] 两份设计稿同步为 OpenSpec 三件套 + 2 个 delta，按 devrix S1→S6 流程立项。

### 1.1 Phase 1 已落地的契约基础（Phase 2 直接复用）

| Phase 1 资产 | Phase 2 用法 |
|------------|------------|
| `orchtypes/uncertainty_coord.go` UncertaintyCoord | Observe 输出 `UncertaintyReport.Overall`；Plan 输入读 `UncertaintyReport.Overall` |
| `sessionorchestrator/orchestrator.go:279` AdaptiveThreshold wiring | 不变（Phase 2 不动 FastPath 决策） |
| `workmodel/verifier.go` Verifier 4 态 scaffold | 不变（Phase 2 不动 Verify） |
| `turn/orchestrator.go` ExitReason 3 新枚举 | 不变 |

### 1.2 与已有 change 的关系

- **直接前置** `devrix-d7-mups-v4-phase1-foundation` (DM-20260623-001) — UncertaintyCoord 已落地
- **间接前置** `devrix-d7-v2-structure` (DM-20260619-005) — v2.0 物理路径已就绪
- **间接前置** `devrix-d7-uncertainty-gaps` (DM-20260616-001) — 5 gap 已闭环
- **被前置** `devrix-d7-mups-v4-phase3-execute` (DM-20260625-001 候选) — Phase 3 依赖本 Phase 2 的 Plan 类型
- **正交** `devrix-d7-metrics-and-concurrency-hardening` (DM-20260622-001) — D5 Span 可复用，本 change 不冲突

### 1.3 与 tech-debt 的关系

- 本 change 不引入新 tech-debt
- 本 change 不闭合已有 tech-debt（与 Phase 1 同步进入 5 Phase 路线图）

---

## 2. Problem Statement

### Problem 1 (HIGH): Observe 节点缺失统一数据契约

**位置**：
- `internal/layers/orchestration/sessionorchestrator/orchestrator.go:ProcessMessage` 直接走 classifier + IntentKind，**没有 ObserveNode 抽象**
- D5 `EngineEvent.Content` 是 string（无数值层），无 Observation 结构
- 4 类观察（Fact/Signal/Deviation/Intent）散落在 `WorkItem.Uncertainty` + `IntentClassification` + 临时变量中

**根因**：v2.0 结构重构（DM-20260619-005）时只物理路径切分，未抽象 Observe 节点。

**影响**：
- 父 agent 看到 `WorkItem.Uncertainty=0.7` + `IntentClassification.Confidence=70` 不知道两者是否一致
- 异常检测靠 ad-hoc 函数，4 类 Detector（Historical/Structural/LLMClaim/Evidence）未接口化
- 无法为下一轮 Observe 注入 ReputationEvidence 作为先验（LP-1 闭环缺失）
- 业务/系统异常混在一起，LLM 内部 timeout 被误判为业务 deviation（OP-6 缺失）

**修法**：新建 `observe/` 目录 + `orchtypes/observation.go` + `orchtypes/uncertainty_report.go`，把 4 类观察统一为 `Observation` struct，通过 `UncertaintyReport` 聚合；新建 `AnomalyDetector` interface + 4 实现 + Composite。

### Problem 2 (HIGH): IntentQuantizer 多轮收敛缺失

**位置**：
- `internal/layers/orchestration/decisionplanning/classifier.go` 仅 `IntentClassification`（Kind + Confidence + Reason），**无多轮量化**
- LLM 自报 Confidence ≈ 75% 准确率（doc 42 §4.5 数据），但未做交叉验证
- 高不确定场景无法收敛

**根因**：Phase 1 只把 `IntentClassification.Confidence` 通过 `FromConfidence` 投影到 `UncertaintyCoord`，未量化过程本身。

**影响**：
- 单轮 LLM 判定后无法验证（"hi" → FastPath 实际想问复杂问题）
- 跨 session 无法累积 Confidence 校准信号
- Loop-First 路径下 IntentQuantizer 缺失导致首轮 fallback 到 default

**修法**：新建 `decisionplanning/intent_quantizer.go`，3 轮收敛循环（LLM 自报 + 客观 evidence + AdaptivePrior 加权），单轮 ≤ 2s，超时强制 `ErrIntentUnquantifiable`。

### Problem 3 (HIGH): Plan 节点缺失，Execute 无法消费结构化计划

**位置**：
- `internal/layers/orchestration/wavescheduler/scheduler.go` 直接消费 `decisionplanning.Decomposer` 输出的 `[]Task`，**无 Plan 类型**
- `PlanKind` 4 类枚举未定义（Execute 4 类通道需要 PlanKind 决策走哪个通道）
- `SourceObservationIDs` 血缘字段缺失，Verify 无法反向追溯

**根因**：Phase 1 只把 4 类分解约定在 doc 35，未落到代码。

**影响**：
- Execute 节点（Phase 3）无法做 PlanKind 路由（commit/protocol/scenario/exploration 4 类通道）
- 3 项强制约束（PP-1 强度匹配 / PP-2 可证伪性 / PP-3 爆炸半径）无 Validate() 强制
- 跨节点血缘链断裂，Verdict 无法追溯到 Observation

**修法**：新建 `plan/` 目录 + `plan/plan.go`（Plan + PlanKind 4 枚举 + SourceObservationIDs 血缘）+ `plan/plan_validator.go`（PP-1/2/3）+ `plan/planner.go`（Planner interface + DefaultPlanner）。

### Problem 4 (MEDIUM): 业务/系统异常未分离（OP-6 缺失）

**位置**：
- `AnomalyDetector` 4 实现里无 Category 字段，业务 deviation 和 LLM 内部 timeout 混在一起
- Verify 阶段（Phase 4）异常聚合时无法区分"业务失败"vs"环境受限"

**根因**：v2.0 结构重构时未考虑 OP-6 分离。

**影响**：
- Learn 节点（Phase 5）累积 Orchestrator 自身信誉时会污染业务信誉
- 异常归类被滥用，真实业务 deviation 可能被打成 CatSystem 排除

**修法**：在 `Observation` struct 加 `Category` 枚举（CatBusiness/CatSystem），Verify 阶段反向校验（CatSystem 中 `DetectorID != "system_health"` 且 `ZScore > 2.0` → 重分类为 CatBusiness）。

---

## 3. Solution

### 3.1 Observe 节点（D7-S8）

#### 3.1.1 Observation 4 类 + UncertaintyReport 聚合

**新增文件**：
- `internal/layers/orchestration/orchtypes/observation.go`（~200 行）
- `internal/layers/orchestration/orchtypes/uncertainty_report.go`（~250 行）
- `internal/layers/orchestration/orchtypes/observation_test.go`（~150 行）
- `internal/layers/orchestration/orchtypes/uncertainty_report_test.go`（~150 行）

**核心数据结构**：
```go
type Category int
const (
    CatBusiness Category = iota  // 业务相关 deviation（进入 Plan 决策路径）
    CatSystem                     // 系统异常（LLM timeout / D7 orchestrator 自身错误，不进入业务路径）
)

type ObservationKind int
const (
    ObsFact       ObservationKind = iota  // 客观事实（文件存在、API 响应、git status）
    ObsSignal                              // 信号（IntentClassification、ContextHint）
    ObsDeviation                           // 偏差（Z-score > 2.0 的 z-score baseline）
    ObsUncertainty                         // 不确定性（ReputationEvidence 反映的 session-level 信誉先验）
)

type Observation struct {
    ID         string
    Kind       ObservationKind
    Category   Category
    Strength   float64  // [0,1]，越高 = 越强信号
    Payload    any      // Kind-specific payload
    DetectedAt time.Time
    Source     string   // detector ID 或 user
}

type UncertaintyReport struct {
    SessionID            string
    Observations         []Observation           // 全集（Business ∪ System）
    BusinessObservations []Observation           // CatBusiness 子集（Plan 决策路径用）
    SystemObservations   []Observation           // CatSystem 子集（Learn 信誉累积用）
    Anomalies            []Observation           // Kind=ObsDeviation 子集
    Overall              UncertaintyCoord        // 聚合后的统一坐标系
    QuantizedIntent      IntentPayload           // IntentQuantizer 输出
    Prior                *AdaptivePrior          // LP-1 闭环：上一轮 ReputationEvidence
}
```

**关键不变式**：`Observations == BusinessObservations ∪ SystemObservations`（强制保证）

**ComputeOverallStrength**：只遍历 `BusinessObservations`（不污染业务路径）

#### 3.1.2 UncertaintyCoord 扩展（Phase 1 增量）

**修改文件**：`internal/layers/orchestration/orchtypes/uncertainty_coord.go`

**新增 2 字段**：
```go
type UncertaintyCoord struct {
    Score             float64
    Verdict           VerdictKind
    Reason            string
    Source            CoordSource
    FromVerifier      bool               // ⭐新增：Phase 4 输出回写时标记
    SideEffectStatus  SideEffectStatus   // ⭐新增：Phase 3 副作用状态传播
}

// 新增工厂方法
func FromVerifier(verdict VerdictKind, confidence float64, reason string) UncertaintyCoord
```

#### 3.1.3 AnomalyDetector 4 实现 + Composite

**新增文件**：
- `internal/layers/orchestration/observe/anomaly_detector.go`（~150 行）
- `internal/layers/orchestration/observe/detector_historical.go`（~100 行）
- `internal/layers/orchestration/observe/detector_structural.go`（~100 行）
- `internal/layers/orchestration/observe/detector_llm_claim.go`（~150 行）
- `internal/layers/orchestration/observe/detector_evidence.go`（~100 行）
- `internal/layers/orchestration/observe/anomaly_detector_test.go`（~300 行）

**核心接口**：
```go
type AnomalyDetector interface {
    Name() string
    Detect(ctx context.Context, baseline, current any) (DeviationPayload, error)
}

type DeviationPayload struct {
    DetectorID  string
    Category    Category
    ZScore      float64  // 仅 HistoricalDetector 用
    Expected    any
    Actual      any
    Evidence    string
    SuggestedKind ObservationKind  // ObsDeviation or ObsFact
}

type CompositeAnomalyDetector struct {
    Detectors []AnomalyDetector
}

func (c *CompositeAnomalyDetector) Detect(ctx, baseline, current) ([]DeviationPayload, error) {
    // errgroup.Go 并行 4 个 detector，单个失败不影响整体（continue）
}
```

**关键约束**：
- `LLMClaimDeviationDetector.AllowedTools() = nil`（避免 detector 调 tool 造成递归）
- HistoricalDetector Window=24h 默认（基于 `wavescheduler/history` 历史数据）
- StructuralDetector 对比 D7-S1 WorkItem 结构差异
- EvidenceDetector 多源证据交叉验证

#### 3.1.4 IntentQuantizer 多轮收敛

**新增文件**：
- `internal/layers/orchestration/decisionplanning/intent_quantizer.go`（~300 行）
- `internal/layers/orchestration/decisionplanning/intent_quantizer_test.go`（~200 行）

**核心逻辑**：
```go
type IntentQuantizer struct {
    LLMCompleter LLMCompleter  // D3 跨域
    MaxRounds    int           // 默认 3
    PerRoundTimeoutMs int      // 默认 2000
}

type IntentPayload struct {
    Kind       IntentKind
    Confidence float64      // [0, 1]，不再是 int [0,100]
    Reason     string
    Rounds     int          // 实际收敛轮数（1-3）
    Source     CoordSource  // SourceClassifier / SourceAdvisory（兜底）
}

func (q *IntentQuantizer) Quantize(ctx context.Context, message string, prior *AdaptivePrior) (*IntentPayload, error) {
    // 3 轮循环：
    //   1. LLM 自报 Kind + Reason + Confidence
    //   2. evidence 客观信号交叉（grep baseline 历史）
    //   3. AdaptivePrior 加权 → 最终 Confidence
    // 超时 → ErrIntentUnquantifiable，route to IntentOrchestrate（Loop-First 兜底）
}
```

#### 3.1.5 ObserveNode + ProcessMessage wiring

**新增文件**：
- `internal/layers/orchestration/observe/observe_node.go`（~250 行）
- `internal/layers/orchestration/observe/observe_node_test.go`（~200 行）

**修改文件**：
- `internal/layers/orchestration/sessionorchestrator/orchestrator.go:ProcessMessage`（在 classifier 之后插入 ObserveNode.All()，~30 行新增）
- `cmd/devrix/main.go`（DI 注入 ObserveNode + CompositeAnomalyDetector + IntentQuantizer，~10 行）

**核心接口**：
```go
type ObserveRequest struct {
    SessionID         string
    UserMessage       string
    IntentKind        IntentKind  // 来自 classifier 的初判
    Prior             *AdaptivePrior  // LP-1 闭环
}

type ObserveNode interface {
    All(ctx context.Context, req ObserveRequest) (*UncertaintyReport, error)
}

type DefaultObserveNode struct {
    Quantizer      *IntentQuantizer
    AnomalyDetectors *CompositeAnomalyDetector
    BaselineStore  BaselineStore  // D7-S3 WaveScheduler 历史
}

func (n *DefaultObserveNode) All(ctx, req) (*UncertaintyReport, error) {
    // errgroup.Go 并行：
    //   - Quantizer.Quantize(message, prior)
    //   - CompositeAnomalyDetector.Detect(baseline, current)
    // 聚合为 UncertaintyReport
    // P95 ≤ 50ms（除 IntentQuantizer 异步路径）
}
```

### 3.2 Plan 节点（D7-S8）

#### 3.2.1 Plan 4 类 + SourceObservationIDs 血缘

**新增目录**：`internal/layers/orchestration/plan/`

**新增文件**：
- `plan/plan.go`（~300 行）
- `plan/plan_kind.go`（~80 行）
- `plan/plan_test.go`（~200 行）

**核心数据结构**：
```go
type PlanKind int
const (
    CommitmentPlan   PlanKind = iota  // 1 Step 直接执行（commit channel）
    ProtocolPlan                       // 多 Step 顺序协议（protocol channel）
    ScenarioPlan                       // 并行试探（scenario channel，并行 max=5）
    ExplorationPlan                    // 多 agent 并行探索（exploration channel，FreeFork 可选）
)

type Plan struct {
    ID                    string
    Kind                  PlanKind
    Strength              float64  // [0, 1]，由 Plan.Validate() 强制 ≤ min(BusinessObservations.Strength)
    Steps                 []PlanStep
    FailureCriteria       []FailureCriterion
    BlastRadius           BlastRadius
    SourceObservationIDs  []string  // ⭐血缘字段，反向追溯到 UncertaintyReport.Observations[].ID
    AnomaliesCount        int       // OP-4 衍生：Plan 触发的异常数（Learn 用）
    CreatedAt             time.Time
}

type PlanStep struct {
    Index          int
    ToolName       string
    Parameters     string         // JSON
    IdempotencyKey string         // ⭐必填当 Tool 有 SideEffect（Phase 3 集成时校验）
    RetryPolicy    *RetryPolicy   // 可选 per-step
}

type FailureCriterion struct {
    Field   string  // "exit_code" / "diff_hash" / "api_status" / ...
    Op      string  // "eq" / "ne" / "lt" / "gt" / "contains"
    Value   any
}

type BlastRadius struct {
    FileCount    int      // 影响文件数
    APICallCount int      // 外部 API 调用次数
    TokenCost    int      // 估算 token 消耗
    PersistScope string   // "session" / "user" / "global"
}

func (p *Plan) Validate(observations []Observation) error {
    // PP-1: p.Strength ≤ min(BusinessObservations.Strength)
    // PP-2: FailureCriteria 非空 + 可观测
    // PP-3: BlastRadius.FileCount ≤ 50 (configurable)
}
```

**Kind 匹配规则**：
- `ObsFact` 主导 → `CommitmentPlan`
- `ObsSignal` 主导 → `ProtocolPlan`
- `ObsDeviation` 主导 → `ScenarioPlan`
- `ObsUncertainty` 主导 → `ExplorationPlan`

#### 3.2.2 Plan.Validate() + 3 项强制约束

**新增文件**：
- `plan/plan_validator.go`（~250 行）
- `plan/strength_match.go`（~100 行）
- `plan/plan_validator_test.go`（~250 行）

**PP-1 强度匹配**：
```go
func ValidateStrength(plan *Plan, businessObs []Observation) error {
    minStrength := math.MaxFloat64
    for _, obs := range businessObs {
        if obs.Strength < minStrength {
            minStrength = obs.Strength
        }
    }
    if plan.Strength > minStrength {
        return fmt.Errorf("%w: plan.Strength=%.2f > min(obs.Strength)=%.2f",
            ErrPlanStrengthMismatch, plan.Strength, minStrength)
    }
    return nil
}
```

**PP-2 可证伪性**：所有 `FailureCriterion.Op` 必须在 `["eq", "ne", "lt", "gt", "contains", "matches"]` 白名单内；每个 `Field` 必须可从 ExecutionEvidence 提取。

**PP-3 爆炸半径**：`BlastRadius.FileCount > 50` 或 `APICallCount > 20` → `ErrBlastRadiusExceeded`（立即 fail-fast）。

#### 3.2.3 Planner.Default + BlastRadiusCalculator

**新增文件**：
- `plan/blast_radius.go`（~150 行）
- `plan/planner.go`（~200 行：Planner interface + DefaultPlanner）
- `plan/planner_test.go`（~200 行）

**核心接口**：
```go
type Planner interface {
    Plan(ctx context.Context, report *UncertaintyReport) (*Plan, error)
}

type DefaultPlanner struct {
    LLMDecomposer  LLMTaskDecomposer  // D3 跨域
    Validator      *PlanValidator
    BlastCalc      *BlastRadiusCalculator
}

func (p *DefaultPlanner) Plan(ctx, report) (*Plan, error) {
    // 1. Kind 匹配（基于 Observations 主类）
    // 2. LLMTaskDecomposer 生成 Steps
    // 3. BlastRadiusCalculator 估算
    // 4. Plan.Validate() 强制 3 项约束
    // 5. SourceObservationIDs 填充
}
```

### 3.3 跨域接口契约

| 跨域依赖 | 接口 | 实现位置 | 本 change 是否动 |
|---|---|---|---|
| `LLMCompleter` | `func Complete(ctx, prompt) (string, error)` | D3 LLM 网关 | **不动**（IntentQuantizer + LLMDecomposer 复用） |
| `AdaptiveThreshold` | `ThresholdFor(sessionID) float64` | `workmodel/uncertainty.go` | **不动**（Phase 1 已落地） |
| `BaselineStore` | `Get(sessionID) (Baseline, error)` | D7-S3 WaveScheduler 历史 | **不动**（仅消费） |
| `CoordSource` | enum | `orchtypes/uncertainty_coord.go` | **微调**：增 `SourceOrchestrator`（Observe 内部聚合用） |

---

## 4. Acceptance Criteria

| AC | 描述 | 验证方式 |
|---|---|---|
| **AC1** | Observation struct + 4 类（Fact/Signal/Deviation/Uncertainty）+ Category 2 类 + Strength 字段 + 不可变 | 单测 `orchtypes/observation_test.go` ≥ 6 个用例 |
| **AC2** | UncertaintyReport 聚合：BusinessObservations ∪ SystemObservations = Observations（不变式） | 单测 `TestUncertaintyReport_PartitionInvariant` |
| **AC3** | UncertaintyCoord 扩展 FromVerifier + SideEffectStatus 字段（Phase 1 增量） | 单测 `TestUncertaintyCoord_FromVerifier` |
| **AC4** | IntentQuantizer 3 轮收敛 + 单轮 ≤ 2s + 兜底 ErrIntentUnquantifiable | 单测 + 性能测试 `intent_quantizer_test.go` |
| **AC5** | AnomalyDetector 4 实现 + Composite errgroup 并行 + 单 detector 失败不影响整体 | 单测 `observe/anomaly_detector_test.go` ≥ 4 个用例 |
| **AC6** | LLMClaimDeviationDetector.AllowedTools()=nil（避免 detector 调 tool 递归） | 单测 `TestLLMClaimDetector_NoToolAccess` |
| **AC7** | OP-6 业务/系统异常分离：CatSystem 中 DetectorID != "system_health" 且 ZScore > 2.0 → 重分类 CatBusiness | 单测 `TestOP6_CatSystem_Revalidation` |
| **AC8** | ObserveNode.All() P95 ≤ 50ms（除 IntentQuantizer 异步路径） | 性能测试 + `d7_observe_p95_ms` 指标 |
| **AC9** | ObserveNode.Receive(prior ReputationEvidence) LP-1 闭环（C3 fix） | 集成测试 `tests/integration/d7/observe_with_prior_test.go` |
| **AC10** | Plan 4 类（Commitment/Protocol/Scenario/Exploration）+ Kind 匹配规则 | 单测 `plan/plan_kind_test.go` |
| **AC11** | Plan.SourceObservationIDs 必填 + 反向追溯到 Observations[].ID | 单测 `TestPlan_SourceObservationIDs_Required` |
| **AC12** | PP-1 强度匹配：Plan.Strength ≤ min(BusinessObservations.Strength) | 单测 `TestPlan_Validate_PP1_StrengthMismatch` |
| **AC13** | PP-2 可证伪性：FailureCriteria 非空 + Op 在白名单 + Field 可观测 | 单测 `TestPlan_Validate_PP2_Falsifiability` |
| **AC14** | PP-3 爆炸半径：BlastRadius.FileCount > 50 → ErrBlastRadiusExceeded | 单测 `TestPlan_Validate_PP3_BlastRadiusExceeded` |
| **AC15** | Planner.Plan(ctx, report) 产出 Plan 经过 Validate() + SourceObservationIDs 填充 | 集成测试 `tests/integration/d7/planner_pipeline_test.go` |
| **AC16** | Orchestrator:ProcessMessage 在 classifier 之后插入 ObserveNode.All() | `grep -n "ObserveNode.All" sessionorchestrator/orchestrator.go` ≥ 1 命中 |
| **AC17** | Plan.AnomaliesCount 字段（OP-4 衍生，Phase 5 Learn 用）| 单测 + Verify 阶段消费 |
| **AC18** | 全链路集成：ProcessMessage → ObserveNode.All → Planner.Plan → Plan.Validate 通过 | 集成测试 `tests/integration/d7/observe_plan_pipeline_test.go` |

---

## 5. Out of Scope

明确**不在 Phase 2** 内的事项：

| 任务 | 落点 |
|---|---|
| Execute 节点（4 类通道 + StrategyDecider + Tool Spec v3 扩展）| Phase 3 |
| Verify 节点升格（AggregateVerdicts + VerdictToExitReason 区分 + ExitReason 12 枚举）| Phase 4 |
| Learn 节点（LearningAsset 4 类 + ReputationEvidence + AdaptivePrior 持久层）| Phase 5 |
| ToolSpec v3（10 字段扩展：IsAsync/IsIdempotent/IsRetryable/IsCompensable + 补偿契约）| Phase 3（Execute 落地方案中）|
| IntentOrchestrate fallback 真实路由（Phase 2 仅 type 定义，路由实现 Phase 3）| Phase 3 |
| LoopFirst=true 时的 Observe 跳过策略 | Phase 5+（设计意图暂不动）|
| AnomalyDetector 历史 baseline 持久化（D7-S3 history store 扩展）| Phase 3+（当前读 SessionReputation 内存缓存）|

---

## 6. Risk Assessment

| 风险 | 等级 | 缓解 |
|---|---|---|
| `Observations` 不变式被破坏（外部直接 append）| 中 | 不可变值对象 + 单测强制保证 |
| `IntentQuantizer` LLM 调用超时影响 ObserveNode P95 | 中 | 异步路径 + PerRoundTimeoutMs=2000 + 兜底 route to Orchestrate |
| AnomalyDetector 4 实现中 LLMClaimDetector 调 tool 递归 | 低 | AllowedTools()=nil + system prompt 强约束 |
| Plan.Validate() 与现有 decisionplanning.Decomposer 输出冲突 | 中 | Plan 与 Decomposer 共存（Decomposer 仍输出 []Task，但 Planner 包一层输出 Plan）；Phase 3 后逐步退役 Decomposer |
| `Strength` 字段语义混淆（与 Confidence 区别）| 中 | doc 43 §4.6 文档化 + 单测明确边界 |
| ObserveNode.All() 在 ProcessMessage 中插入位置错误 | 中 | 严格在 classifier 之后 + integration test 验证 |
| UncertaintyCoord Phase 1 已落地 JSON 兼容被 Phase 2 扩展破坏 | 低 | 新增字段用 `omitempty` + UnmarshalJSON 容错 |

---

## 7. Workload Estimation

| 子任务 | 工作量 |
|---|---|
| Observation + UncertaintyReport 类型 + 不变式（PR-A1） | 1.5 天 |
| UncertaintyCoord 扩展 FromVerifier + SideEffectStatus（PR-A1 增量） | 0.3 天 |
| IntentQuantizer 3 轮收敛（PR-A2）| 1.5 天 |
| AnomalyDetector 4 实现 + Composite（PR-A3）| 2.0 天 |
| ObserveNode + ProcessMessage wiring（PR-A4）| 1.0 天 |
| Plan 4 类 + Planner interface（PR-B1）| 1.5 天 |
| Plan.Validate() + 3 项强制约束（PR-B2）| 1.5 天 |
| BlastRadiusCalculator + DefaultPlanner（PR-B3）| 1.0 天 |
| Spec + T Registry delta 撰写 | 0.5 天 |
| 集成测试 + 灰度 rollout | 1.0 天 |
| **合计** | **~10.8 天（取整 10 天）** |

---

## 8. Cross-References

- 详细设计：[[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/42-d7-observe-node-design|doc 42 Observe 节点]]（六段式 + 依赖契约 A/B/C）
- 详细设计：[[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/43-d7-plan-node-design|doc 43 Plan 节点]]（六段式 + 依赖契约 A/B/C）
- 落地全景：[[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/47-d7-mups-v4-phase2-5-openspec-landing-plan|doc 47 Phase 2-5 落地方案]]（§3 Phase 2 详解）
- 数据契约：doc 37 §2.1 Observation + §2.2 Plan + §2.5 UncertaintyReport
- Phase 1：DM-20260623-001 UncertaintyCoord + AdaptiveThreshold wiring + Verifier scaffold + ExitReason
- Phase 3：DM-20260625-001（候选）Execute 节点 — 消费 Plan
- Phase 5：DM-20260627-001（候选）Learn 节点 — 消费 Plan.AnomaliesCount + ReputationEvidence