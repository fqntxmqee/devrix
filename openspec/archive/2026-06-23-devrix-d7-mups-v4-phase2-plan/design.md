# Design: D7 MUPS v4.3 Phase 2 PR-B1 — Plan Data Contract + Planner

**Change ID:** `devrix-d7-mups-v4-phase2-plan`
**Status:** S3_Design → S4_Implemented → S7_Archived
**Date:** 2026-06-23
**Author:** MUPS v4.3 Phase 2 Plan 节点落地梳理

---

## 0. S3-Gate Review（inherited from Phase 2 observe-plan design）

> PR-B1 复用 Phase 2 observe-plan design §0.6 S3-Gate 5 维度自检 + §0.7 S4-Gate 4 维度自检。
> 本节仅列出本 PR 范围内的调整点。

### 0.1 架构决策审查

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 层归属正确 | ✅ PASS | `internal/layers/orchestration/plan/` 在 D7 核心域下 |
| 跨域类型上提 | ✅ PASS | `PlanKind` 暂居 plan 包；与 `shared/types.ArtifactKind` 同款 snake_case wire format 协同（D5 dashboard 字符串过滤） |
| 不可变模式 | ✅ PASS | Plan struct 不可变；With* 返回新副本（与 Phase 2 PR-A1 Observation 一致） |
| SentinelError 模式 | ✅ PASS | 9 sentinels + 3 helpers；sharederrors.WithCode 包装；与 Phase 1/2 同款 |

### 0.2 数据契约审查

| 检查项 | 结论 | 说明 |
|--------|------|------|
| PlanKind 4 类 enum slot 互斥 | ✅ PASS | uint8=1/2/3/4；wire format 4 个不同 snake_case 字符串 |
| Plan 不可变 + 防御性拷贝 | ✅ PASS | `NewPlan` 防御性拷贝 `sourceObservationIDs` 切片；外部 mutation 不影响 Plan 内字段 |
| PP-1/PP-2/PP-3 强制 | ✅ PASS | Validate() 强制 PP-1 强度范围 + PP-2 FailureCriteria 非空 + Op 白名单 + PP-3 BlastRadius 阈值 |
| SourceObservationIDs 必填 | ✅ PASS | 空 → ErrPlanSourceObservationIDsRequired (PLAN_LINEAGE_8002) |
| MatchKind 签名收紧 | ✅ PASS | C2/W8 决议：`(*UncertaintyReport)` 已落地（`MatchKind(quantizedKind string, stepCount, anomaliesCount int)` 接受报告字段，避免 PR-C2 再次破坏 API） |

### 0.3 接口契约审查

| 检查项 | 结论 | 说明 |
|--------|------|------|
| Planner interface | ✅ PASS | `Plan(ctx, PlanInput) (*Plan, error)`；PlanInput 含 SessionID + UncertaintyReport + Step templates |
| DefaultPlanner 实现 | ✅ PASS | 规则引擎；strengthFloor 公式 + MatchKind 4 规则 + Validate 强制 + BlastRadius 透传 |
| ReverseLookupObservations | ✅ PASS | Phase 4 Verify 反向追溯入口；按 ID 集合求交；重复 ID 不产生重复结果；空输入边界 |

## 1. 数据模型

### 1.1 PlanKind 枚举 (4 类)

```go
type PlanKind uint8

const (
    KindUnset         PlanKind = 0
    CommitmentPlan    PlanKind = 1
    ProtocolPlan      PlanKind = 2
    ScenarioPlan      PlanKind = 3
    ExplorationPlan   PlanKind = 4
)
```

wire format snake_case：`commitment_plan` / `protocol_plan` / `scenario_plan` / `exploration_plan`。

### 1.2 BlastRadius

```go
type BlastRadius struct {
    FileCount    int
    APICallCount int
    TokenCost    int
    PersistScope PersistScope
}

type PersistScope uint8

const (
    PersistUnset      PersistScope = 0
    PersistTransient  PersistScope = 1
    PersistSession    PersistScope = 2
    PersistPermanent  PersistScope = 3
)
```

### 1.3 FailureCriterion + Step

```go
type FailureCriterion struct {
    Field string
    Op    string  // whitelist: eq/ne/gt/lt/contains
    Value any
}

type Step struct {
    ID              string
    Directive       string
    ToolName        string
    ToolArgs        map[string]any
    IdempotencyKey  string  // required for side-effecting tools
    EstimatedTokens int
}
```

### 1.4 Plan struct（不可变）

```go
type Plan struct {
    id                    string
    sessionID             string
    kind                  PlanKind
    strength              float64           // ∈ [0,1]
    steps                 []Step            // 1+
    failureCriteria       []FailureCriterion
    blastRadius           BlastRadius
    sourceObservationIDs  []string          // 1+, lineage
    anomaliesCount        int
}
```

不可变 + 防御性拷贝 + 5 With* 方法 + Validate() + ValidateWithOpts(opts) + ReverseLookupObservations(lookup)。

## 2. Planner + MatchKind

### 2.1 MatchKind 4 规则分类器

```go
func MatchKind(quantizedKind string, stepCount, anomaliesCount int) PlanKind {
    // Rule 1: uncertainty-first
    if quantizedKind == "intent_orchestrate" || anomaliesCount >= 3 {
        return ExplorationPlan
    }
    // Rule 2: 1 step
    if stepCount == 1 {
        return CommitmentPlan
    }
    // Rule 3: command or few steps
    if quantizedKind == "intent_command" || stepCount <= 3 {
        return ProtocolPlan
    }
    // Rule 4: default
    return ScenarioPlan
}
```

**uncertainty-first tie-break**: Rule 1 永远先判，避免被 stepCount/anomalyCount 覆盖。

### 2.2 strengthFloor 公式

```
strengthFloor(anomaliesCount, observationCount) = 0.7 - 0.1·anomalies + min(observationCount·0.02, 0.2)
```

边界：
- `anomaliesCount=0`, `observationCount=100` → 0.9（base + 0.2 ceiling）
- `anomaliesCount=3`, `observationCount=0` → 0.4
- `anomaliesCount=0`, `observationCount=10` → 0.9

## 3. Validate() 强制 PP-1/2/3

```go
func (p *Plan) Validate() error {
    if p.kind == KindUnset { return NewPlanKindUnsetError() }
    if len(p.sourceObservationIDs) == 0 { return NewPlanSourceObservationIDsRequiredError() }
    if len(p.steps) == 0 { return ErrPlanStepsEmpty }
    if p.strength < 0 || p.strength > 1 { return ErrPlanStrengthOutOfRange }
    if len(p.failureCriteria) == 0 { return ErrPlanFailureCriteriaEmpty }
    // Op whitelist + Field observability
    for _, fc := range p.failureCriteria {
        if !isValidOp(fc.Op) { return ... }
        if !isObservableField(fc.Field) { return ... }
    }
    // BlastRadius thresholds
    if p.blastRadius.FileCount > MaxFileCount { return NewPlanBlastRadiusExceededError("FileCount", ...) }
    if p.blastRadius.APICallCount > MaxAPICallCount { ... }
    if p.blastRadius.TokenCost > MaxTokenCost { ... }
    if !p.blastRadius.PersistScope.Valid() { return ErrPlanPersistScopeInvalid }
    return nil
}
```

## 4. ReverseLookupObservations

```go
type ObservationLookup interface {
    GetByID(ctx context.Context, id string) (Observation, error)
}

func (p *Plan) ReverseLookupObservations(ctx context.Context, lookup ObservationLookup) ([]Observation, error) {
    if lookup == nil { return nil, nil }
    index := make(map[string]struct{}, len(p.sourceObservationIDs))
    for _, id := range p.sourceObservationIDs { index[id] = struct{}{} }
    
    seen := make(map[string]struct{})
    var result []Observation
    for _, id := range p.sourceObservationIDs {
        if _, dup := seen[id]; dup { continue }
        obs, err := lookup.GetByID(ctx, id)
        if err != nil { continue }
        if _, ok := index[obs.GetID()]; !ok { continue }
        seen[obs.GetID()] = struct{}{}
        result = append(result, obs)
    }
    return result, nil
}
```

## 5. 错误层（9 sentinels + 3 helpers）

| Sentinel | Code | 触发条件 |
|----------|------|---------|
| `ErrPlanKindUnset` | PLAN_KIND_8001 | `p.kind == KindUnset` |
| `ErrPlanSourceObservationIDsRequired` | PLAN_LINEAGE_8002 | `len(p.sourceObservationIDs) == 0` |
| `ErrPlanBlastRadiusExceeded` | PLAN_BLAST_8003 | FileCount / APICallCount / TokenCost 超阈值 |
| `ErrPlanStrengthOutOfRange` | — | `p.strength ∉ [0,1]` |
| `ErrPlanStepsEmpty` | — | `len(p.steps) == 0` |
| `ErrPlanFailureCriteriaEmpty` | — | `len(p.failureCriteria) == 0` |
| `ErrPlanFailureCriterionInvalidOp` | — | Op ∉ {eq, ne, gt, lt, contains} |
| `ErrPlanFailureCriterionInvalidField` | — | Field 不可观察 |
| `ErrPlanPersistScopeInvalid` | — | PersistScope == PersistUnset 或未知值 |

3 helpers：
- `NewPlanKindUnsetError()` → wrap ErrPlanKindUnset + code
- `NewPlanSourceObservationIDsRequiredError()` → wrap + code
- `NewPlanBlastRadiusExceededError(field, actual, max)` → wrap + code + 字段信息

## 6. 跨节点依赖

### 6.1 上游契约（从 Observe 接收）

- `UncertaintyReport.QuantizedIntent.Kind` (string) → `MatchKind` 第一参数
- `UncertaintyReport.Anomalies` ([]Anomaly) → `MatchKind` 第三参数
- `UncertaintyReport.Observations` ([]Observation) → `DefaultPlanner.Plan()` 构造 Plan.SourceObservationIDs
- `UncertaintyReport.ComputeOverallStrength()` → `DefaultPlanner.Plan()` 校准 Plan.Strength

### 6.2 下游契约（向 Execute 交付）

- `Plan.Kind` → ChannelRouter.Route 1:1 路由
- `Plan.Steps[].ToolName` / `ToolArgs` / `IdempotencyKey` → Channel.Execute 顺序 / 并行执行
- `Plan.BlastRadius` → ExplorationChannel 派生 `SideEffectStatus`
- `Plan.SourceObservationIDs` → Artifact 端可追溯（Phase 4 Verify 反向追溯入口）
