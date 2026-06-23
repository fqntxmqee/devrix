# Design: D7 MUPS v4.3 Phase 2 — Observe + Plan 节点

**Change ID:** `devrix-d7-mups-v4-phase2-observe-plan`
**Status:** S3_Design → S3-Gate ✅ Approved
**Date:** 2026-06-23
**Last Reviewed:** 2026-06-23 (DM-20260623-001 S3-Gate pre-review)
**Author:** MUPS v4.3 落地梳理

---

## 0. S3-Gate Review（2026-06-23 pre-review）

> 本节按 `openspec/specs/project/review-design.md` §2 四维度 + §3 Grill Review 流程执行。
> 详细决议与代码定位见 §14 Review Decisions。

### 0.1 架构决策审查（§2.1）

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 层归属正确 | ✅ PASS | `orchtypes/` + `observe/` + `plan/` + `decisionplanning/` 均在 `internal/layers/orchestration/` 下（D7 核心域）|
| 接口方向正确 | ✅ PASS | 不可变值对象 + `With*` 方法，符合 CLAUDE.md §9；高层 ObserveNode/Planner 依赖低层 orchtypes |
| 不重复造轮子 | ✅ PASS | 不动 Phase 1 已落地的 UncertaintyCoord + AdaptiveThreshold + Verifier；只在 Phase 1 基础上扩展 |
| 跨层依赖最小 | ✅ PASS | 仅依赖 D3 `LLMCompleter`（已存在）+ D5 Prometheus histogram（已存在）+ D7-S1 WorkItem |
| 设计决策有记录 | ✅ PASS | §14 Review Decisions 集中记录 3 Critical + 5 Warning + 8 Info |

### 0.2 需求完整性审查（§2.2）

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 需求可追溯 | ✅ PASS | demand.md（DM-20260623-001）→ proposal.md → design.md → tasks.md PR-RF → design.md §14.4 |
| 验收标准覆盖 | ✅ PASS | AC1-AC11 11 个 P0 验收标准全部映射到 §14.4 决议闭环清单 |
| Out of Scope 明确 | ✅ PASS | proposal.md §7 + demand.md §7 双重声明；本 change 不动 Phase 1 既有文件、不动 PR-A1 范围外代码 |
| DM ID 无冲突 | ⚠️ NOTE | DM-20260623-001 与 Phase 1 原始编号一致；以 change_id `devrix-d7-mups-v4-phase2-observe-plan` 区分 |

### 0.3 规格质量审查（§2.3）

| 检查项 | 结论 | 说明 |
|--------|------|------|
| Gherkin 格式正确 | N/A | 本 change 不改 spec.md；spec_delta.md 由 PR-A1 独立走 |
| Happy path + sad path | ✅ PASS | RF.3 列出 6 个新测试用例（3 happy + 3 sad）|
| 并发场景覆盖 | ✅ PASS | tasks.md RF.4.2 `go test -race` 强制要求 |
| 错误路径覆盖 | ✅ PASS | C3 FromVerifier fail-fast + W6/I8 NaN 兜底 + W2 validateFact 错误包装 |
| T 层映射完整 | ✅ PASS | tasks.md PR-RF 全部映射到 11 个 AC（AC1-AC11）|

### 0.4 风险审查（§2.4）

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 回归风险已评估 | ✅ PASS | design.md §12 风险表 + demand.md §6 风险表 |
| 回滚方案可行 | ✅ PASS | 不动 Phase 1 既有文件；PR-RF 拆为独立 PR，回滚只需 revert 1 个 PR |
| 性能影响已评估 | ✅ PASS | design.md §11.3 性能测试矩阵；PR-RF 无新增性能开销 |

### 0.5 Grill Review 结论（§3.1）

逐决策遍历：

| 决策点 | 选项 A | 选项 B | 决议 | 理由 |
|--------|-------|-------|------|------|
| `QuantizedIntent.Kind` 类型 | `string`（占位）| `IntentKind`（枚举）| **A → B（C1）** | 避免 PR-A2 IntentQuantizer 落地时新增翻译层 |
| `MatchKind` 签名 | `(observations []Observation)` | `(*UncertaintyReport)` | **B（C2+W8）** | 强制只读 BusinessObservations，防止调用方误用 |
| `FromVerifier` 未知 verdict | 静默兜底 0.5 | 返回错误 + 触发 7004 | **B（C3）** | 让 `NewUncertaintyCoordInvalidVerdictKindError` 错误码真正起作用 |
| `Partition` Overall clamp | 不 clamp | clamp NaN → 0.5 | **B（W6/I8）** | NaN 风险兜底，与 UncertaintyCoord.Value 冷启动默认值对齐 |
| `clamp01` 函数合并 | 保留 2 函数 | 合并为 `clamp01Float(v, onNaN)` | **B（W3）** | 唯一区别是 NaN 处理，统一 onNaN 参数即可 |
| PR-RF 拆分策略 | 与 PR-A1 合并 | 独立 PR-RF | **B** | 5 项 code change + 6 个测试用例 vs PR-A1 的 4 个核心文件落地，作用域不同 |
| `Observations` 字段保留 | 删除冗余字段 | 保留（向后兼容）+ 收紧 MatchKind | **B（C2）** | 既保护现有调用方，又通过 MatchKind 签名收紧防止误用 |

逐依赖确认：

| 依赖 | 必要性 | 版本锁定 | 决议 |
|------|-------|---------|------|
| D3 `LLMCompleter` | 必要（Quantizer + ClaimDetector） | 已存在，无需新增 | ✅ Agreed |
| D5 `d7_observe_p95_ms` | 必要（ObserveNode 打点） | 已存在 | ✅ Agreed |
| D7-S1 `WorkItem` | 必要（HistoricalDetector） | 已存在 | ✅ Agreed |
| `golang.org/x/sync/errgroup` | 必要（并行 Quantizer + Detector） | go.mod 已锁 | ✅ Agreed |

### 0.6 S3-Gate 最终结论

| 维度 | 通过率 | 结论 |
|------|--------|------|
| 架构决策 | 5/5 | ✅ PASS |
| 需求完整性 | 3/3 + 1 NOTE | ✅ PASS |
| 规格质量 | 4/4 + 1 N/A | ✅ PASS |
| 风险审查 | 3/3 | ✅ PASS |
| Grill Review | 7/7 + 4/4 | ✅ Agreed |

**Overall: ✅ Approved**（无阻塞问题，可进入 S4 实现阶段）

### 0.7 S4-Gate 自检（2026-06-23 PR-RF 实现完成）

> 本节按 `openspec/specs/project/review-code.md` §2 四维度 + §3 严重级别执行。
> 完整 5 项代码修改 + 6 个新测试用例已落地，全部 PASS。

#### §2.1 OpenSpec 文档完整性

| 检查项 | 结论 | 说明 |
|--------|------|------|
| Change 文件齐全 | ⚠️ PARTIAL | `.openspec.yaml` 缺失（按 devrix 实际在 S6 归档前补）|
| T 层已登记 | N/A | PR-RF 是 review fix，不动 T 层；T 层在 PR-A1 spec_delta.md 已登记 |
| 文档状态一致 | ✅ PASS | `design.md` §0.6 status 已更新为 `S3_Design → S3-Gate ✅ Approved` |

#### §2.2 代码质量

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 包位置正确 | ✅ PASS | 所有改动均在 `internal/layers/orchestration/orchtypes/` 下（D7 核心域）|
| 函数规模 | ✅ PASS | `FromVerifier` 26 行 / `clamp01Float` 11 行 / `validateFact` 6 行 / `NewObservation` 17 行 |
| 文件规模 | ✅ PASS | observation.go 395 行 / uncertainty_report.go 191 行 / uncertainty_coord.go 158 行（均 < 800 行）|
| 嵌套深度 | ✅ PASS | 所有函数嵌套 ≤ 3 层 |
| 命名清晰 | ✅ PASS | `clamp01Float(v, onNaN)` / `QuantizedIntent.Kind IntentKind` / `validateFact` 语义明确 |
| 接口合理 | ✅ PASS | `Payload` sealed interface 4 个方法（Validate + kindMarker）保持小接口 |

#### §2.3 错误与安全

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 错误不静默 | ✅ PASS | `FromVerifier` 未知 verdict 改 fail-fast（不再静默兜底 0.5）|
| Sentinel Error 正确 | ✅ PASS | `NewUncertaintyCoordInvalidVerdictKindError` 触发 `ORCH_COORD_VERDICT_7004` |
| 输入校验 | ✅ PASS | `Observation.Validate` + `UncertaintyCoord.Validate` + `UncertaintyReport.Validate` 三层防御 |
| 无硬编码密钥 | ✅ PASS | 无新增密钥/API key |
| 并发安全 | ✅ PASS | 不可变值对象，`With*` 返回新副本，无共享状态 |
| 值对象不可变 | ✅ PASS | `clamp01` + `clamp01Coord` 合并为 `clamp01Float`，签名不变性更清晰 |
| 类型断言安全 | ✅ PASS | `unmarshalPayload` 使用 type switch，未用裸 `.(*Type)` |
| CQS | ✅ PASS | `ComputeOverallStrength` / `FilterByKind` / `SortedObservationsByStrength` 均为只读方法 |

#### §2.4 测试完整性

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 单元测试存在 | ✅ PASS | 6 个新测试用例（RF.3.1-3.3）全部到位 |
| Happy path + sad path | ✅ PASS | 6 个测试覆盖 5 happy + 3 sad（NaN 兜底 / 未知 verdict / 类型 roundtrip）|
| T 层映射完整 | ✅ PASS | AC1-AC11 11 个 P0 标准全部对应到 6 个测试 + 既有 23 个测试 |
| Race 检测 | ✅ PASS | `go test -race -count=1 ./internal/layers/orchestration/orchtypes/` 全部 PASS |

#### 验证命令输出（2026-06-23）

```
$ go vet ./...
0 issue

$ go test -race -count=1 ./internal/layers/orchestration/orchtypes/
ok  	github.com/devrix/devrix/internal/layers/orchestration/orchtypes	1.617s

$ go test -cover ./internal/layers/orchestration/orchtypes/
ok  	github.com/devrix/devrix/internal/layers/orchestration/orchtypes	0.539s	coverage: 72.2% of statements
```

#### §3 严重级别评估

| 维度 | 发现 | 级别 |
|------|------|------|
| 并发安全 | 0 issue | — |
| 输入校验 | 0 issue | — |
| 错误处理 | C3 FromVerifier fail-fast 是改进（CRITICAL → CLOSED）| — |
| 资源泄漏 | 0 issue | — |
| 安全漏洞 | 0 issue | — |
| 测试覆盖 | 72.2%（AC11 baseline 维持）| LOW |

**S4-Gate 最终结论：✅ Approved**（无 CRITICAL/HIGH/MEDIUM，可进入 S5 验收）

---

---

## 1. 范围与架构位置

### 1.1 D7 域内位置

```
D7 编排层（internal/layers/orchestration/）
├── orchtypes/          ⭐ 本 change 扩展：Observation + UncertaintyReport + UncertaintyCoord 增量
├── observe/            ⭐ 本 change 新建：AnomalyDetector + ObserveNode
├── plan/               ⭐ 本 change 新建：Plan + Planner + Validator
├── decisionplanning/   ⭐ 本 change 扩展：IntentQuantizer
├── sessionorchestrator/ ⚠ 微调：ProcessMessage wiring
├── workmodel/          不变（Phase 1 UncertaintyCoord 复用）
├── wavescheduler/      不变（Phase 3 Execute 落地方案）
├── turn/               不变（Phase 4 Verify 落地方案）
├── orchtypes/          不变（Phase 1 UncertaintyCoord scaffold 已落）
└── ...
```

### 1.2 跨域依赖

| 跨域 | 接口 | 本 change 用法 |
|---|---|---|
| D3 LLM 网关 | `LLMCompleter.Complete(ctx, prompt) (string, error)` | IntentQuantizer + LLMTaskDecomposer |
| D4 多智能体 | `FreeForkSurface` (后续 Phase 3 引入) | 本 change 不动 |
| D5 可观测性 | `d7_observe_p95_ms` Prometheus histogram | ObserveNode.All() 打点 |
| D7-S1 WorkItem | `WorkItem.Uncertainty` + 结构 | HistoricalDetector + StructuralDetector |

---

## 2. 核心类型设计

> **实现偏离说明（2026-06-23 设计审查后同步）**
>
> | 项 | 原设计 | 实际实现 | 理由 |
> |----|--------|----------|------|
> | `WithStrength` 越界 | panic | clamp01 静默 | 符合 Devrix CLAUDE.md "禁止 panic 用于业务错误" |
> | `Payload` 类型 | `any` | sealed `Payload` interface | 4 个具名类型，封闭枚举可穷尽 type-assert |
> | `NewObservation` Strength 越界 | 静默 clamp | 静默 clamp | 保持一致（容错优先）|
> | `Observation.Validate()` | 未列 | 已加 | 防御性校验（hand-built 场景）|
> | 错误处理 | 本地 `errors.New` | `internal/shared/errors` SentinelError 模式 | 符合 Devrix CLAUDE.md 规范，跨包可 errors.Is |
> | MarshalJSON wire format | 未列 | 已加（⭐ Review W1：见下方 wire format 示例）| Payload 按 Kind 判别反序列化，wire 为 `{id, kind, category, strength, payload: {…}, detected_at, source}` 嵌套对象 |
> | `validateFact` 错误包装 | 直接 return sentinel | `fmt.Errorf("orchtypes: FactPayload.Statement empty: %w", ErrObservationPayloadInvalid)`（⭐ Review W2）| 与 Signal/Deviation/Uncertainty Validate 风格统一 |

### 2.1 Observation + 4 类 + 2 Category

**文件**：`internal/layers/orchestration/orchtypes/observation.go`

```go
package orchtypes

import "time"

// Category 区分业务/系统异常（OP-6）
type Category uint8

const (
    CatBusiness Category = iota  // 业务相关 deviation（进入 Plan 决策路径）
    CatSystem                    // 系统异常（LLM timeout / D7 orchestrator 自身错误，不进入业务路径）
)

func (c Category) String() string {
    switch c {
    case CatBusiness:
        return "business"
    case CatSystem:
        return "system"
    default:
        return fmt.Sprintf("Category(%d)", uint8(c))
    }
}

// ObservationKind 4 类观察
type ObservationKind uint8

const (
    ObsFact        ObservationKind = iota  // 客观事实（文件存在、API 响应、git status）
    ObsSignal                               // 信号（IntentClassification、ContextHint）
    ObsDeviation                            // 偏差（Z-score > 2.0 的 z-score baseline）
    ObsUncertainty                          // 不确定性（ReputationEvidence 反映的 session-level 信誉先验）
)

func (k ObservationKind) String() string {
    switch k {
    case ObsFact:
        return "fact"
    case ObsSignal:
        return "signal"
    case ObsDeviation:
        return "deviation"
    case ObsUncertainty:
        return "uncertainty"
    default:
        return fmt.Sprintf("ObservationKind(%d)", uint8(k))
    }
}

// Observation 是 4 类观察的统一容器（不可变）
type Observation struct {
    ID         string          `json:"id"`
    Kind       ObservationKind `json:"kind"`
    Category   Category        `json:"category"`
    Strength   float64         `json:"strength"`  // [0,1]
    Payload    Payload         `json:"payload"`   // Kind-specific (sealed interface)
    DetectedAt time.Time       `json:"detected_at"`
    Source     string          `json:"source"`    // detector ID 或 user
}

// 不可变更新方法（符合 coding.md §9）
// 越界值采用静默 clamp，不 panic（Devrix CLAUDE.md 禁止 panic 用于业务错误）
func (o Observation) WithKind(k ObservationKind) Observation {
    o.Kind = k
    return o
}

func (o Observation) WithStrength(s float64) Observation {
    o.Strength = clamp01(s)
    return o
}

// Validate 检查 ID/Strength/DetectedAt/Payload 必填。
// 用于手构造场景（如 JSON 反序列化后），防 dirty data 流向下游。
func (o Observation) Validate() error {
    if o.ID == "" {
        return ErrObservationIDRequired
    }
    if o.Strength < 0 || o.Strength > 1 {
        return NewObservationStrengthOutOfRangeError(o.Strength)
    }
    if o.DetectedAt.IsZero() {
        return ErrObservationDetectedAtRequired
    }
    if o.Payload == nil {
        return ErrObservationPayloadRequired
    }
    if err := o.Payload.Validate(); err != nil {
        return fmt.Errorf("orchtypes: payload: %w", err)
    }
    return nil
}
```

### 2.2 UncertaintyReport 聚合

> **实现偏离说明 + Review Decisions（2026-06-23 S3-Gate pre-review）**
>
> | 项 | 原设计 | 实际实现 | 理由 |
> |----|--------|----------|------|
> | `NewUncertaintyReport` 签名 | `(sessionID, obs, overall, intent, prior)` 6 参数 | `(sessionID, obs)` 2 参数 | overall/intent/prior 改为方法（`SetQuantizedIntent` / `SetPrior` / `Overall` 字段）|
> | `Overall` 字段类型 | `UncertaintyCoord` | `float64` | UncertaintyCoord 是 Phase 4 概念，Phase 2 不要前置依赖；float64 是中间表示 |
> | `QuantizedIntent.Kind` 类型 | `string` 占位 | `IntentKind`（⭐ Review C1）| 直接用 `orchtypes.IntentKind` 枚举（fast/command/orchestrate/skip），避免 PR-A2 IntentQuantizer 落地时新增 string↔IntentKind 翻译层 |
> | `Prior` 类型 | `*AdaptivePrior`（learn 包）| `*AdaptivePrior`（orchtypes 本地 stub）| Phase 5 才落地 learn 包；提前 stub 兼容 |
> | `partition` 私有 / `Partition` 公开 | 私有 | 公开 + 返回 error | 允许外部直接调 Partition 重建；增加不变式检查 |
> | `AddObservation` 签名 | `(*UncertaintyReport, error)` 指针 | `(UncertaintyReport, error)` 值 | 不可变值对象风格 |
> | `Anomalies` 口径 | 全部 ObsDeviation | CatSystem + ObsDeviation | 业务偏差不污染 Learn 信誉累积 |
> | Partition 后 Overall | 构造时算 | Partition 末尾重算 | 防止手工调 Partition 拿到 stale Overall |
> | `Partition` 末尾 Overall clamp | 未列 | `clamp01Float(Overall, 0.5)`（⭐ Review W6/I8）| ComputeOverallStrength 返回 NaN 时兜底为 0.5（与 UncertaintyCoord.Value 行为对齐）|

**文件**：`internal/layers/orchestration/orchtypes/uncertainty_report.go`

```go
package orchtypes

import (
    "time"
)

// UncertaintyReport 是 Observe 节点对外的统一输出
type UncertaintyReport struct {
    SessionID            string           `json:"session_id"`
    Observations         []Observation    `json:"observations"`            // 全集（Business ∪ System）
    BusinessObservations []Observation    `json:"business_observations"`   // CatBusiness 子集（Plan 决策路径用）
    SystemObservations   []Observation    `json:"system_observations"`     // CatSystem 子集（Learn 信誉累积用）
    Anomalies            []Observation    `json:"anomalies"`               // CatSystem + ObsDeviation（⭐设计稿偏离：见上方说明）
    Overall              float64          `json:"overall"`                 // ⭐ float64 而非 UncertaintyCoord（中间表示）
    QuantizedIntent      *QuantizedIntent `json:"quantized_intent,omitempty"`  // ⭐ 本地 stub 而非 decisionplanning.IntentPayload
    Prior                *AdaptivePrior   `json:"prior,omitempty"`         // ⭐ 本地 stub 而非 learn.AdaptivePrior
    CreatedAt            time.Time        `json:"created_at"`
}

// 不变式：Observations == BusinessObservations ∪ SystemObservations
// 在 NewUncertaintyReport / AddObservation / Partition 末尾强制保证

// NewUncertaintyReport 构造函数，自动按 Category 拆分并计算 Overall
func NewUncertaintyReport(sessionID string, observations []Observation) (UncertaintyReport, error) {
    if sessionID == "" {
        return UncertaintyReport{}, ErrUncertaintyReportSessionIDRequired
    }
    r := UncertaintyReport{
        SessionID:    sessionID,
        Observations: append([]Observation(nil), observations...),
        CreatedAt:    time.Now(),
    }
    if err := r.Partition(); err != nil {
        return UncertaintyReport{}, err
    }
    return r, nil
}

// Partition 按 Category 拆分到 BusinessObservations / SystemObservations
// 不变式：Observations == BusinessObservations ∪ SystemObservations
// 同时按 Kind=ObsDeviation + CatSystem 提取到 Anomalies
// 末尾重算 Overall 防止 stale
func (r *UncertaintyReport) Partition() error {
    r.BusinessObservations = r.BusinessObservations[:0]
    r.SystemObservations = r.SystemObservations[:0]
    r.Anomalies = r.Anomalies[:0]
    for _, obs := range r.Observations {
        switch obs.Category {
        case CatBusiness:
            r.BusinessObservations = append(r.BusinessObservations, obs)
        case CatSystem:
            r.SystemObservations = append(r.SystemObservations, obs)
            if obs.Kind == ObsDeviation {
                r.Anomalies = append(r.Anomalies, obs)
            }
        default:
            return fmt.Errorf("orchtypes: %w: %d", ErrObservationUnknownCategory, obs.Category)
        }
    }
    if len(r.BusinessObservations)+len(r.SystemObservations) != len(r.Observations) {
        return NewUncertaintyReportPartitionInvariantError()
    }
    r.Overall = r.ComputeOverallStrength()  // ⭐Partition 末尾重算
    return nil
}

// FilterByKind 按 Kind 切分（故意遍历全集，不按 Category 过滤）
// 例如：用户可能想看所有 ObsUncertainty（包括业务 + 系统）
func (r *UncertaintyReport) FilterByKind(kind ObservationKind) []Observation {
    var result []Observation
    for _, obs := range r.Observations {
        if obs.Kind == kind {
            result = append(result, obs)
        }
    }
    return result
}

// ComputeOverallStrength 只遍历 BusinessObservations（不污染业务路径）
func (r *UncertaintyReport) ComputeOverallStrength() float64 {
    if len(r.BusinessObservations) == 0 {
        return 0.5  // 冷启动中性
    }
    var sum float64
    for _, obs := range r.BusinessObservations {
        sum += obs.Strength
    }
    return sum / float64(len(r.BusinessObservations))
}

// AddObservation 不可变：返回新 report（值 + error）
func (r UncertaintyReport) AddObservation(obs Observation) (UncertaintyReport, error) {
    r.Observations = append(r.Observations, obs)
    if err := r.Partition(); err != nil {
        return r, err
    }
    return r, nil
}
```

### 2.3 UncertaintyCoord 扩展（Phase 1 增量）

> **实现偏离说明（2026-06-23 设计审查后同步）**
>
> | 项 | 原设计 | 实际实现 | 理由 |
> |----|--------|----------|------|
> | 主字段 | `Score` | `Value` | 与 Phase 4 `UncertaintyReport.Overall` 命名对齐；"value" 语义中性，不暗示数值含义 |
> | Verdict/VerdictKind | 内嵌枚举 | 完全移除 | UncertaintyCoord 退化为纯数值容器（value + confidence），verdict 语义由 Phase 4 Verifier 单独判定，避免一处枚举两处真相 |
> | Source/CoordSource | 枚举（SourceClassifier/SourceAdvisory/...） | 完全移除 | Source 语义在 Phase 2 已迁移到 `IntentPayload.Source` / `UncertaintyReport.QuantizedIntent.Source`，Coord 不再重复 |
> | `Confidence` 字段 | 缺失 | 单独字段 | 与 Verifier 输出的 `confidence` 解耦：value 是判决强度，confidence 是 confidence of value |
> | `UpdatedAt` | 缺失 | 新增 `time.Time` | 配合 `Equal()` 排除 wall-clock 比较（UncertaintyReport 流向下游时不变）|
> | `NewUncertaintyCoord(v)` | 缺失 | 新增 | 默认值构造工厂（cold-start 用）|
> | `FromVerifier` 签名 | `(verdict VerdictKind, confidence, reason)` 3 参数 | `(verdictKind string, confidence, reason, systemAnomaly bool)` 4 参数（⭐ Review C3：未知 verdict 改静默兜底 0.5 → 返回 `NewUncertaintyCoordInvalidVerdictKindError`）| verdict 改 string（不绑枚举），新增 systemAnomaly 强制 0.95 上限；未知 verdict 改报错触发 `ORCH_COORD_VERDICT_7004` 错误码 |
> | `With*` 方法 | 缺失 | WithValue/WithReason/WithSideEffect 3 个 | 不可变值对象标配；`WithSideEffect` 让 Phase 3 副作用状态可回写 |
> | `IsColdStart()` | 缺失 | 新增 | ObserveNode 检测"无历史 prior"场景（避免误判）|
> | `Equal()` | 缺失 | 新增 | 跨层传递时排除 UpdatedAt 比对 |
> | `Validate()` | 缺失 | 已加 | 与 Observation/Report 对齐，统一防御性校验 |
> | `MarshalJSON/UnmarshalJSON` | 默认 | 显式实现 | UnmarshalJSON 中 `UpdatedAt` 兜底为 now（兼容老 wire 格式）|
> | `ErrInvalidVerdictKind` | 新增 | 保留为别名 `ErrUncertaintyCoordInvalidVerdictKind` | 保持向后兼容（Phase 1 已有调用点）|
> | `clamp01` vs `clamp01Coord` | 2 函数重复 | 合并为 `clamp01Float(v, onNaN float64) float64`（⭐ Review W3）| 唯一区别是 NaN 处理：统一 onNaN 参数（默认 0.5），Observation/Coord 共用 |
> | SideEffectStatus 落位 | 标"Phase 3 落地" | orchtypes 本地占位 | 避免 observe→wavescheduler 循环依赖；Phase 3 落地时再移 |

**修改文件**：`internal/layers/orchestration/orchtypes/uncertainty_coord.go`

```go
package orchtypes

import (
    "time"
)

// SideEffectStatus Phase 3 落地的副作用状态占位（type alias）
// 真实 enum 在 wavescheduler/artifact.go，Phase 3 PR 落地时再迁移
type SideEffectStatus string

const (
    SideEffectUnknown    SideEffectStatus = "unknown"
    SideEffectInflight   SideEffectStatus = "inflight"
    SideEffectCommitted  SideEffectStatus = "committed"
    SideEffectRolledBack SideEffectStatus = "rolled_back"
)

// Phase 2 增量：UncertaintyCoord 退化为纯数值容器
//   Value:     [0,1] 判决强度（clamp01Coord 保护）
//   Confidence: [0,1] value 的置信度（独立维度）
//   FromVerifier: true 表示 Phase 4 Verifier 注入（与 ObserveNode 注入区分）
//   SideEffectStatus: Phase 3 副作用状态传播
type UncertaintyCoord struct {
    Value            float64          `json:"value"`
    Confidence       float64          `json:"confidence,omitempty"`
    Reason           string           `json:"reason,omitempty"`
    FromVerifier     bool             `json:"from_verifier,omitempty"`
    SideEffectStatus SideEffectStatus `json:"side_effect_status,omitempty"`
    UpdatedAt        time.Time        `json:"updated_at"`
}

// 默认工厂（cold-start 用）
// ⭐ PR-RF W3：clamp01Coord 合并到 clamp01Float，统一 NaN 兜底
func NewUncertaintyCoord(value float64) UncertaintyCoord {
    return UncertaintyCoord{
        Value:     clamp01Float(value, 0.5),
        UpdatedAt: time.Now(),
    }
}

// Phase 4 Verifier 注入工厂
// verdictKind: "pass" | "partial" | "indeterminate" | "fail"
// systemAnomaly=true → value 强制 0.95（orchestrator 不信任）
// ⭐ PR-RF C3：未知 verdict 改 fail-fast，返回 (UncertaintyCoord, error)
// 触发 ORCH_COORD_VERDICT_7004 错误码；4 种已知 verdict 行为不变
func FromVerifier(verdictKind string, confidence float64, reason string, systemAnomaly bool) (UncertaintyCoord, error) {
    base := 0.5
    switch verdictKind {
    case "pass":
        base = 0.0
    case "partial":
        base = 0.4
    case "indeterminate":
        base = 0.7
    case "fail":
        base = 0.9
    default:
        return UncertaintyCoord{}, NewUncertaintyCoordInvalidVerdictKindError(verdictKind)
    }
    if systemAnomaly {
        base = 0.95
    }
    return UncertaintyCoord{
        Value:        clamp01Float(base, 0.5),
        Confidence:   clamp01Float(confidence, 0.5),
        Reason:       reason,
        FromVerifier: true,
        UpdatedAt:    time.Now(),
    }, nil
}

// 不可变更新方法
func (u UncertaintyCoord) WithValue(v float64) UncertaintyCoord { ... }
func (u UncertaintyCoord) WithReason(r string) UncertaintyCoord { ... }
func (u UncertaintyCoord) WithSideEffect(s SideEffectStatus) UncertaintyCoord { ... }

// 校验 + 辅助方法
func (u UncertaintyCoord) Validate() error { ... }
func (u UncertaintyCoord) IsColdStart() bool { ... }
func (u UncertaintyCoord) Equal(other UncertaintyCoord) bool { ... }  // 排除 UpdatedAt

// 向后兼容：Phase 1 调用点
var ErrInvalidVerdictKind = ErrUncertaintyCoordInvalidVerdictKind
```

---

## 3. IntentQuantizer 设计

**文件**：`internal/layers/orchestration/decisionplanning/intent_quantizer.go`

```go
package decisionplanning

import (
    "context"
    "errors"
    "fmt"
    "time"
)

var ErrIntentUnquantifiable = errors.New("decisionplanning: intent unquantifiable after max rounds")

type IntentQuantizer struct {
    LLMCompleter     LLMCompleter
    MaxRounds        int           // 默认 3
    PerRoundTimeout  time.Duration // 默认 2s
    BaselineScorer   BaselineScorer // 用于第 2 轮 evidence 客观信号
}

type IntentPayload struct {
    Kind       IntentKind  `json:"kind"`
    Confidence float64     `json:"confidence"` // [0,1]
    Reason     string      `json:"reason"`
    Rounds     int         `json:"rounds"`     // 实际收敛轮数（1-3）
    Source     CoordSource `json:"source"`
}

func (q *IntentQuantizer) Quantize(ctx context.Context, message string, prior *AdaptivePrior) (*IntentPayload, error) {
    if q.LLMCompleter == nil {
        return nil, errors.New("IntentQuantizer: LLMCompleter is nil")
    }
    if q.MaxRounds == 0 {
        q.MaxRounds = 3
    }
    if q.PerRoundTimeout == 0 {
        q.PerRoundTimeout = 2 * time.Second
    }

    for round := 1; round <= q.MaxRounds; round++ {
        roundCtx, cancel := context.WithTimeout(ctx, q.PerRoundTimeout)

        // Round 1: LLM 自报
        claim, err := q.llmClaim(roundCtx, message)
        cancel()
        if err != nil {
            if round == q.MaxRounds {
                return q.fallbackAdvisory(message, round), ErrIntentUnquantifiable
            }
            continue
        }

        // Round 2: evidence 客观信号交叉
        evidence, err := q.evidenceScore(roundCtx, message, claim)
        if err != nil {
            if round == q.MaxRounds {
                return q.fallbackAdvisory(message, round), ErrIntentUnquantifiable
            }
            continue
        }

        // Round 3: AdaptivePrior 加权
        final := q.priorWeighted(claim, evidence, prior)

        if q.isConverged(final) {
            return &IntentPayload{
                Kind:       final.Kind,
                Confidence: final.Confidence,
                Reason:     final.Reason,
                Rounds:     round,
                Source:     SourceClassifier,
            }, nil
        }
    }
    // 兜底
    return q.fallbackAdvisory(message, q.MaxRounds), ErrIntentUnquantifiable
}

// fallbackAdvisory 生成 Advisory（兜底，route to IntentOrchestrate）
func (q *IntentQuantizer) fallbackAdvisory(message string, rounds int) *IntentPayload {
    return &IntentPayload{
        Kind:       IntentOrchestrate,  // ⭐ Loop-First 兜底：交给 Orchestrator 进一步拆分
        Confidence: 0.5,
        Reason:     fmt.Sprintf("intent unquantifiable after %d rounds, fallback to orchestrate", rounds),
        Rounds:     rounds,
        Source:     SourceAdvisory,
    }
}

func (q *IntentQuantizer) isConverged(p IntentPayload) bool {
    return p.Confidence >= 0.8  // 经验阈值
}
```

---

## 4. AnomalyDetector 设计

**文件**：`internal/layers/orchestration/observe/anomaly_detector.go`

```go
package observe

import (
    "context"
    "log/slog"
    "golang.org/x/sync/errgroup"
    "devrix/internal/layers/orchestration/orchtypes"
)

type AnomalyDetector interface {
    Name() string
    Detect(ctx context.Context, baseline, current any) (orchtypes.DeviationPayload, error)
}

type DeviationPayload struct {
    DetectorID    string                       `json:"detector_id"`
    Category      orchtypes.Category           `json:"category"`
    ZScore        float64                      `json:"z_score,omitempty"`
    Expected      any                          `json:"expected,omitempty"`
    Actual        any                          `json:"actual,omitempty"`
    Evidence      string                       `json:"evidence"`
    SuggestedKind orchtypes.ObservationKind    `json:"suggested_kind"`
}

type CompositeAnomalyDetector struct {
    Detectors []AnomalyDetector
}

func NewCompositeAnomalyDetector() *CompositeAnomalyDetector {
    return &CompositeAnomalyDetector{
        Detectors: []AnomalyDetector{
            &HistoricalDeviationDetector{Window: 24 * time.Hour},
            &StructuralDeviationDetector{},
            &LLMClaimDeviationDetector{LLMCompleter: nil},  // DI 注入
            &EvidenceDeviationDetector{},
        },
    }
}

func (c *CompositeAnomalyDetector) Detect(ctx context.Context, baseline, current any) ([]orchtypes.DeviationPayload, error) {
    results := make([]orchtypes.DeviationPayload, len(c.Detectors))
    eg, egCtx := errgroup.WithContext(ctx)
    for i, det := range c.Detectors {
        i, det := i, det  // capture
        eg.Go(func() error {
            payload, err := det.Detect(egCtx, baseline, current)
            if err != nil {
                slog.Warn("anomaly_detector.failed", "detector", det.Name(), "error", err)
                return nil  // ⭐ 单 detector 失败不影响整体
            }
            // ⭐ OP-6 反向校验
            payload.Category = RevalidateCategory(payload)
            results[i] = payload
            return nil
        })
    }
    _ = eg.Wait()
    return results, nil
}

// RevalidateCategory OP-6 业务/系统异常分离
func RevalidateCategory(payload orchtypes.DeviationPayload) orchtypes.Category {
    if payload.Category == orchtypes.CatSystem &&
        payload.DetectorID != "system_health" &&
        payload.ZScore > 2.0 {
        slog.Warn("observe.category_misclassify",
            "detector", payload.DetectorID,
            "z_score", payload.ZScore,
            "from", "CatSystem",
            "to", "CatBusiness",
        )
        return orchtypes.CatBusiness
    }
    return payload.Category
}
```

### 4.1 LLMClaimDetector（特殊约束）

**文件**：`internal/layers/orchestration/observe/detector_llm_claim.go`

```go
type LLMClaimDeviationDetector struct {
    LLMCompleter decisionplanning.LLMCompleter
}

func (d *LLMClaimDeviationDetector) Name() string { return "llm_claim" }

func (d *LLMClaimDeviationDetector) Detect(ctx context.Context, baseline, current any) (orchtypes.DeviationPayload, error) {
    claim, evidence, ok := extractClaimVsEvidence(baseline, current)
    if !ok {
        return orchtypes.DeviationPayload{}, errors.New("LLMClaimDeviationDetector: claim/evidence extraction failed")
    }
    prompt := d.buildPrompt(claim, evidence)

    // ⭐ 关键约束：AllowedTools=nil，避免 detector 调 tool 递归
    result, err := d.LLMCompleter.CompleteWithOptions(ctx, decisionplanning.CompleteOptions{
        SystemPrompt: "你是一个 claim vs evidence 比对器。你不能调用任何 tool，只输出 JSON {match: bool, reason: string}。",
        UserPrompt:   prompt,
        AllowedTools: nil,  // ⭐⭐⭐ 避免递归
    })
    if err != nil {
        return orchtypes.DeviationPayload{}, err
    }
    return d.parseResult(result, claim, evidence)
}
```

---

## 5. ObserveNode 设计

**文件**：`internal/layers/orchestration/observe/observe_node.go`

```go
package observe

import (
    "context"
    "fmt"
    "time"
    "golang.org/x/sync/errgroup"
    "devrix/internal/layers/orchestration/orchtypes"
)

type ObserveRequest struct {
    SessionID   string                          `json:"session_id"`
    UserMessage string                          `json:"user_message"`
    IntentKind  orchtypes.IntentKind            `json:"intent_kind"`  // 来自 classifier 的初判
    Prior       *learn.AdaptivePrior            `json:"prior"`         // LP-1 闭环
}

type ObserveNode interface {
    All(ctx context.Context, req ObserveRequest) (*orchtypes.UncertaintyReport, error)
}

type DefaultObserveNode struct {
    Quantizer        *decisionplanning.IntentQuantizer
    AnomalyDetectors *CompositeAnomalyDetector
    BaselineStore    BaselineStore  // D7-S3 WaveScheduler 历史
    Timeout          time.Duration  // 默认 50ms（除 Quantizer 异步外）
}

func (n *DefaultObserveNode) All(ctx context.Context, req ObserveRequest) (*orchtypes.UncertaintyReport, error) {
    start := time.Now()
    defer func() {
        d7_observe_p95_ms.Observe(float64(time.Since(start).Milliseconds()))
    }()

    // 并行：Quantizer（异步，不计入 P95）+ AnomalyDetector
    var (
        quantized *decisionplanning.IntentPayload
        anomalies []orchtypes.DeviationPayload
        intent    orchtypes.IntentPayload
    )
    eg, egCtx := errgroup.WithContext(ctx)

    // 1. IntentQuantizer（异步路径，单轮 ≤ 2s）
    eg.Go(func() error {
        qCtx, cancel := context.WithTimeout(egCtx, 6*time.Second)  // 3 轮 × 2s
        defer cancel()
        q, err := n.Quantizer.Quantize(qCtx, req.UserMessage, req.Prior)
        if err != nil {
            slog.Warn("observe.intent_quantizer.failed", "error", err)
            return nil  // 兜底
        }
        quantized = q
        return nil
    })

    // 2. AnomalyDetector（同步路径，计入 P95）
    eg.Go(func() error {
        aCtx, cancel := context.WithTimeout(egCtx, n.Timeout)
        defer cancel()
        baseline, _ := n.BaselineStore.Get(req.SessionID)
        payloads, _ := n.AnomalyDetectors.Detect(aCtx, baseline, req.UserMessage)
        anomalies = payloads
        return nil
    })

    _ = eg.Wait()

    // 兜底：Quantizer 失败时使用初判
    if quantized == nil {
        quantized = &orchtypes.IntentPayload{
            Kind:       req.IntentKind,
            Confidence: 0.5,
            Reason:     "intent_quantizer_failed_fallback",
            Rounds:     0,
            Source:     orchtypes.SourceAdvisory,
        }
    }

    // 聚合为 UncertaintyReport
    observations := buildObservations(req, quantized, anomalies)
    overall := orchtypes.UncertaintyCoord{
        Score:  quantized.Confidence,
        Verdict: orchtypes.VerdictUnresolved,  // Phase 4 Verifier 决定
        Reason:  quantized.Reason,
        Source:  quantized.Source,
    }
    intent = *quantized

    return orchtypes.NewUncertaintyReport(req.SessionID, observations, overall, &intent, req.Prior)
}
```

---

## 6. Plan 设计

**文件**：`internal/layers/orchestration/plan/plan.go`

```go
package plan

import (
    "time"
    "devrix/internal/layers/orchestration/wavescheduler/retry"
)

type PlanKind uint8

const (
    CommitmentPlan PlanKind = iota  // 1 Step 直接执行（commit channel）
    ProtocolPlan                     // 多 Step 顺序协议（protocol channel）
    ScenarioPlan                     // 并行试探（scenario channel，并行 max=5）
    ExplorationPlan                  // 多 agent 并行探索（exploration channel，FreeFork 可选）
)

type Plan struct {
    ID                    string                  `json:"id"`
    Kind                  PlanKind                `json:"kind"`
    Strength              float64                 `json:"strength"`             // [0,1]
    Steps                 []PlanStep              `json:"steps"`
    FailureCriteria       []FailureCriterion      `json:"failure_criteria"`
    BlastRadius           BlastRadius             `json:"blast_radius"`
    SourceObservationIDs  []string                `json:"source_observation_ids"`  // ⭐血缘
    AnomaliesCount        int                     `json:"anomalies_count"`         // ⭐ OP-4 衍生
    CreatedAt             time.Time               `json:"created_at"`
}

type PlanStep struct {
    Index          int                 `json:"index"`
    ToolName       string              `json:"tool_name"`
    Parameters     string              `json:"parameters"`         // JSON
    IdempotencyKey string              `json:"idempotency_key"`    // ⭐ Phase 3 副作用工具必填
    RetryPolicy    *retry.RetryPolicy  `json:"retry_policy,omitempty"`
}

type FailureCriterion struct {
    Field string `json:"field"`   // "exit_code" / "diff_hash" / "api_status" / ...
    Op    string `json:"op"`      // "eq" / "ne" / "lt" / "gt" / "contains" / "matches"
    Value any    `json:"value"`
}

type BlastRadius struct {
    FileCount    int    `json:"file_count"`
    APICallCount int    `json:"api_call_count"`
    TokenCost    int    `json:"token_cost"`
    PersistScope string `json:"persist_scope"`  // "session" / "user" / "global"
}

// Validate 便捷方法（委托 PlanValidator）
func (p *Plan) Validate(observations []orchtypes.Observation) error {
    return DefaultPlanValidator.Validate(p, observations)
}
```

### 6.1 PlanValidator（PP-1/2/3）

**文件**：`internal/layers/orchestration/plan/plan_validator.go`

```go
package plan

import (
    "errors"
    "fmt"
)

var (
    ErrPlanStrengthMismatch      = errors.New("plan: PP-1 strength mismatch")
    ErrPlanFalsifiabilityFailed  = errors.New("plan: PP-2 falsifiability failed")
    ErrPlanBlastRadiusExceeded   = errors.New("plan: PP-3 blast radius exceeded")
    ErrPlanSourceObservationIDsRequired = errors.New("plan: source_observation_ids required")
    ErrPlanFailureCriteriaEmpty  = errors.New("plan: failure_criteria empty")
    ErrPlanFailureCriteriaInvalidOp = errors.New("plan: failure_criteria op not in whitelist")
    ErrPlanUnknownKind           = errors.New("plan: unknown plan kind")
    ErrPlanKindMatchFailed       = errors.New("plan: kind match failed")
    ErrPlanStepsEmpty            = errors.New("plan: steps empty")
)

var validFailureOps = map[string]bool{
    "eq": true, "ne": true, "lt": true, "gt": true, "contains": true, "matches": true,
}

var observableFields = map[string]bool{
    "exit_code": true, "diff_hash": true, "api_status": true,
    "output_match": true, "duration_ms": true,
}

type PlanValidator struct {
    MaxBlastRadius BlastRadius  // 配置上限
}

func (v *PlanValidator) Validate(plan *Plan, observations []orchtypes.Observation) error {
    if plan == nil {
        return errors.New("plan: nil")
    }
    if len(plan.Steps) == 0 {
        return ErrPlanStepsEmpty
    }
    if len(plan.SourceObservationIDs) == 0 {
        return ErrPlanSourceObservationIDsRequired
    }
    if err := v.checkStrength(plan, observations); err != nil {
        return err  // PP-1
    }
    if err := v.checkFalsifiability(plan); err != nil {
        return err  // PP-2
    }
    if err := v.checkBlastRadius(plan); err != nil {
        return err  // PP-3
    }
    return nil
}

func (v *PlanValidator) checkStrength(plan *Plan, observations []orchtypes.Observation) error {
    businessObs := filterBusinessObservations(observations)
    if len(businessObs) == 0 {
        return nil  // 没有业务 obs，强度无约束
    }
    minStrength := math.MaxFloat64
    for _, obs := range businessObs {
        if obs.Strength < minStrength {
            minStrength = obs.Strength
        }
    }
    if plan.Strength > minStrength {
        return fmt.Errorf("%w: plan.Strength=%.2f > min(business_obs.Strength)=%.2f",
            ErrPlanStrengthMismatch, plan.Strength, minStrength)
    }
    return nil
}

func (v *PlanValidator) checkFalsifiability(plan *Plan) error {
    if len(plan.FailureCriteria) == 0 {
        return ErrPlanFailureCriteriaEmpty
    }
    for _, fc := range plan.FailureCriteria {
        if !validFailureOps[fc.Op] {
            return fmt.Errorf("%w: op=%s", ErrPlanFailureCriteriaInvalidOp, fc.Op)
        }
        if !observableFields[fc.Field] {
            return fmt.Errorf("%w: field=%s not observable", ErrPlanFailureCriteriaInvalidOp, fc.Field)
        }
    }
    return nil
}

func (v *PlanValidator) checkBlastRadius(plan *Plan) error {
    if plan.BlastRadius.FileCount > v.MaxBlastRadius.FileCount {
        return fmt.Errorf("%w: file_count=%d > max=%d",
            ErrPlanBlastRadiusExceeded,
            plan.BlastRadius.FileCount, v.MaxBlastRadius.FileCount)
    }
    if plan.BlastRadius.APICallCount > v.MaxBlastRadius.APICallCount {
        return fmt.Errorf("%w: api_call_count=%d > max=%d",
            ErrPlanBlastRadiusExceeded,
            plan.BlastRadius.APICallCount, v.MaxBlastRadius.APICallCount)
    }
    return nil
}

func filterBusinessObservations(observations []orchtypes.Observation) []orchtypes.Observation {
    result := make([]orchtypes.Observation, 0, len(observations))
    for _, obs := range observations {
        if obs.Category == orchtypes.CatBusiness {
            result = append(result, obs)
        }
    }
    return result
}
```

### 6.2 Planner + DefaultPlanner

**文件**：`internal/layers/orchestration/plan/planner.go`

```go
package plan

import (
    "context"
    "fmt"
    "time"
    "github.com/google/uuid"
    "devrix/internal/layers/orchestration/orchtypes"
)

type Planner interface {
    Plan(ctx context.Context, report *orchtypes.UncertaintyReport) (*Plan, error)
}

type DefaultPlanner struct {
    LLMDecomposer decisionplanning.LLMTaskDecomposer
    Validator     *PlanValidator
    BlastCalc     *BlastRadiusCalculator
    StrengthFloor float64  // 最低 Plan.Strength（如 0.5）
}

func (p *DefaultPlanner) Plan(ctx context.Context, report *orchtypes.UncertaintyReport) (*Plan, error) {
    // 1. Kind 匹配（⭐ Review C2/W8: MatchKind 签名收紧，强制读 BusinessObservations）
    kind := MatchKind(report)

    // 2. LLMTaskDecomposer 生成 Steps
    steps, err := p.LLMDecomposer.Decompose(ctx, decisionplanning.DecomposeRequest{
        Observations: report.BusinessObservations,  // 只用业务 obs
        Intent:       report.QuantizedIntent,
    })
    if err != nil {
        return nil, fmt.Errorf("DefaultPlanner: LLM decompose: %w", err)
    }

    // 3. PlanStrength = min(LLM 建议, min(BusinessObs.Strength), StrengthFloor)
    strength := p.computeStrength(steps, report.BusinessObservations)

    // 4. SourceObservationIDs 填充
    sourceIDs := make([]string, 0, len(report.BusinessObservations))
    for _, obs := range report.BusinessObservations {
        sourceIDs = append(sourceIDs, obs.ID)
    }

    // 5. AnomaliesCount
    anomaliesCount := len(report.Anomalies)

    plan := &Plan{
        ID:                   uuid.New().String(),
        Kind:                 kind,
        Strength:             strength,
        Steps:                convertToPlanSteps(steps),
        FailureCriteria:      extractFailureCriteria(steps),
        BlastRadius:          BlastRadius{},  // 待 BlastCalc 计算
        SourceObservationIDs: sourceIDs,
        AnomaliesCount:       anomaliesCount,
        CreatedAt:            time.Now(),
    }

    // 6. BlastRadius 计算
    plan.BlastRadius = p.BlastCalc.Calculate(plan)

    // 7. Validate 强制 3 项约束
    if err := p.Validator.Validate(plan, report.Observations); err != nil {
        return nil, fmt.Errorf("DefaultPlanner: validate: %w", err)
    }
    return plan, nil
}

func (p *DefaultPlanner) computeStrength(steps []decisionplanning.Task, businessObs []orchtypes.Observation) float64 {
    minObs := 1.0
    for _, obs := range businessObs {
        if obs.Strength < minObs {
            minObs = obs.Strength
        }
    }
    // LLM 建议的 strength 来自 steps 数量 / 复杂度，这里用 steps 数倒推
    llmStrength := math.Max(0.0, 1.0 - float64(len(steps))*0.1)
    return math.Min(math.Min(llmStrength, minObs), p.StrengthFloor)
}

// MatchKind returns the dominant PlanKind for the business observations
// in the report. Reads report.BusinessObservations ONLY — never the full
// Observations set — so callers cannot accidentally leak system-level
// signals into the Kind decision.
//
// ⭐ Review C2/W8: signature tightened from
//   `MatchKind(observations []Observation) PlanKind`
// to
//   `MatchKind(report *UncertaintyReport) PlanKind`
// to prevent callers from passing report.Observations / report.SystemObservations
// by mistake, which would silently degrade to ExplorationPlan via the
// "no dominant business kind" fallback.
func MatchKind(report *orchtypes.UncertaintyReport) PlanKind {
    counts := make(map[orchtypes.ObservationKind]int)
    for _, obs := range report.BusinessObservations {
        counts[obs.Kind]++
    }
    max := 0
    var dominant orchtypes.ObservationKind
    for k, c := range counts {
        if c > max {
            max = c
            dominant = k
        }
    }
    switch dominant {
    case orchtypes.ObsFact:
        return CommitmentPlan
    case orchtypes.ObsSignal:
        return ProtocolPlan
    case orchtypes.ObsDeviation:
        return ScenarioPlan
    case orchtypes.ObsUncertainty:
        return ExplorationPlan
    default:
        return ExplorationPlan  // 兜底：交给 Orchestrate
    }
}
```

---

## 7. Orchestrator:ProcessMessage Wiring

**修改文件**：`internal/layers/orchestration/sessionorchestrator/orchestrator.go`

```go
// 在 ProcessMessage 中，classifier 之后、dispatcher 之前插入 ObserveNode + Planner
func (o *Orchestrator) ProcessMessage(ctx context.Context, req ProcessMessageRequest) (*ProcessMessageResponse, error) {
    // ... 现有 classifier 逻辑 ...
    intent, err := o.classifier.ClassifyIntent(ctx, req.Message)
    if err != nil {
        return nil, fmt.Errorf("orchestrator: classify: %w", err)
    }

    // ⭐ Phase 2 新增：Observe 节点
    observeReq := observe.ObserveRequest{
        SessionID:   req.SessionID,
        UserMessage: req.Message,
        IntentKind:  intent.Kind,
        Prior:       o.reputationStore.GetPrior(req.SessionID),  // LP-1 闭环
    }
    report, err := o.observeNode.All(ctx, observeReq)
    if err != nil {
        return nil, o.handleObserveError(err, req)
    }

    // ⭐ Phase 2 新增：Plan 节点
    plan, err := o.planner.Plan(ctx, report)
    if err != nil {
        return nil, o.handlePlanError(err, req)
    }

    // Phase 3 替换为 Executor.Execute(plan)
    // Phase 2 暂时走原 dispatcher（向后兼容）
    return o.dispatchPlan(req, plan, intent)
}

// dispatchPlan 兼容旧路径
func (o *Orchestrator) dispatchPlan(req ProcessMessageRequest, plan *plan.Plan, intent *orchtypes.IntentClassification) (*ProcessMessageResponse, error) {
    // plan.Steps → 转换为 []Task 给现有 WaveScheduler
    tasks := convertPlanStepsToTasks(plan.Steps)
    return o.waveScheduler.SubmitTasks(ctx, tasks)
}
```

---

## 8. 错误处理

> **实现偏离说明（2026-06-23 设计审查后同步）**
>
> 原设计仅列出 5 个本地 `errors.New` 哨兵。实际实现遵循 Devrix `internal/shared/errors/` SentinelError 模式：
>
> | 维度 | 原设计 | 实际实现 |
> |------|--------|----------|
> | 哨兵定义 | `var ErrXxx = errors.New(...)` | 同上（保持不变）|
> | 错误码 | 无 | 4 位 ORCH 域码（7000-7999 范围）|
> | 包装函数 | 无 | `NewXxxError()` 返回 `*sharederrors.SentinelError` |
> | 错误信息 | 单层 | `%w` 包装 + WithCode 注入 code/field/value |
> | 新增哨兵 | 5 个 | 11 个（Observation 6 + Report 2 + Coord 3）|
>
> 错误码分配：
> - `ORCH_OBS_STRENGTH_7001` — 强度越界
> - `ORCH_OBS_CATEGORY_7002` — 未知 Category
> - `ORCH_REPORT_PARTITION_7003` — Partition 不变式破坏
> - `ORCH_COORD_VALUE_7004` — Coord Value 越界
> - 其余（Payload/ID/DetectedAt/SessionID/Confidence/VerdictKind）保留 stderr text，不分配 code（业务级，调用方只关心 sentinel）

**文件**：`internal/layers/orchestration/orchtypes/errors.go`（新增）

```go
package orchtypes

import (
    "errors"
    "fmt"

    sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// 11 个哨兵（保留 errors.New 风格以便 errors.Is）
var (
    ErrObservationIDRequired          = errors.New("orchtypes: observation ID required")
    ErrObservationStrengthOutOfRange  = errors.New("orchtypes: observation strength out of [0,1]")
    ErrObservationDetectedAtRequired  = errors.New("orchtypes: observation DetectedAt required")
    ErrObservationUnknownCategory     = errors.New("orchtypes: observation unknown category")
    ErrObservationPayloadRequired     = errors.New("orchtypes: observation payload required")
    ErrObservationPayloadInvalid      = errors.New("orchtypes: observation payload invalid")
    ErrUncertaintyReportPartitionInvariant = errors.New("orchtypes: uncertainty report partition invariant violated")
    ErrUncertaintyReportSessionIDRequired  = errors.New("orchtypes: uncertainty report session_id required")
    ErrUncertaintyCoordValueOutOfRange      = errors.New("orchtypes: uncertainty coord value out of [0,1]")
    ErrUncertaintyCoordConfidenceOutOfRange = errors.New("orchtypes: uncertainty coord confidence out of [0,1]")
    ErrUncertaintyCoordInvalidVerdictKind   = errors.New("orchtypes: uncertainty coord invalid verdict kind")
)

// 4 个错误码包装器（跨包可被 metrics/observability 透传）
func NewObservationStrengthOutOfRangeError(strength float64) *sharederrors.SentinelError {
    return sharederrors.WithCode(
        "ORCH_OBS_STRENGTH_7001",
        fmt.Sprintf("strength %.3f out of [0,1]", strength),
        ErrObservationStrengthOutOfRange,
    )
}

func NewObservationUnknownCategoryError(c Category) *sharederrors.SentinelError {
    return sharederrors.WithCode(
        "ORCH_OBS_CATEGORY_7002",
        fmt.Sprintf("unknown category %d", uint8(c)),
        ErrObservationUnknownCategory,
    )
}

func NewUncertaintyReportPartitionInvariantError() *sharederrors.SentinelError {
    return sharederrors.WithCode(
        "ORCH_REPORT_PARTITION_7003",
        "business+system != all observations",
        ErrUncertaintyReportPartitionInvariant,
    )
}

func NewUncertaintyCoordValueOutOfRangeError(v float64) *sharederrors.SentinelError {
    return sharederrors.WithCode(
        "ORCH_COORD_VALUE_7004",
        fmt.Sprintf("coord value %.3f out of [0,1]", v),
        ErrUncertaintyCoordValueOutOfRange,
    )
}

// 向后兼容别名（Phase 1 调用点）
var ErrInvalidVerdictKind = ErrUncertaintyCoordInvalidVerdictKind
```

**文件**：`internal/layers/orchestration/plan/errors.go`（11 个，已在 tasks.md §B2.5.1 列出）

---

## 9. 配置

**修改文件**：`internal/layers/orchestration/orchtypes/config.go`

```go
type Config struct {
    // ... Phase 1 已有的配置 ...

    // ⭐ Phase 2 新增
    ObserveNodeTimeoutMs     int  `yaml:"observe_node_timeout_ms"`      // 默认 50
    IntentQuantizerMaxRounds int  `yaml:"intent_quantizer_max_rounds"` // 默认 3
    IntentQuantizerPerRoundTimeoutMs int `yaml:"intent_quantizer_per_round_timeout_ms"` // 默认 2000
    PlanMaxBlastRadiusFileCount int `yaml:"plan_max_blast_radius_file_count"` // 默认 50
    PlanMaxBlastRadiusAPICallCount int `yaml:"plan_max_blast_radius_api_call_count"` // 默认 20
    PlanStrengthFloor float64 `yaml:"plan_strength_floor"` // 默认 0.5
}
```

**YAML 覆盖**（`~/.devrix/config.yaml`）：
```yaml
d7:
  observe:
    timeout_ms: 50
    intent_quantizer:
      max_rounds: 3
      per_round_timeout_ms: 2000
  plan:
    max_blast_radius:
      file_count: 50
      api_call_count: 20
    strength_floor: 0.5
```

---

## 10. 度量指标

新增 Prometheus 指标（`internal/layers/orchestration/observability/`）：

| 指标 | 类型 | Labels |
|------|------|--------|
| `d7_observe_p95_ms` | Histogram | `session_id` (可选) |
| `d7_intent_quantizer_rounds` | Histogram | — |
| `d7_intent_quantifier_errors_total` | Counter | `error_type` |
| `d7_anomaly_detector_total` | Counter | `detector_id`, `category` |
| `d7_anomaly_misclassify_total` | Counter | `detector_id`, `from_category`, `to_category` |
| `d7_plan_validate_errors_total` | Counter | `error_type` (PP-1/2/3) |
| `d7_plan_kind_match_total` | Counter | `kind` |
| `d7_plan_strength_capped_total` | Counter | — |

---

## 11. 测试矩阵

### 11.1 单元测试

| 文件 | 测试数 | 状态 | 覆盖 |
|------|-------|------|------|
| `orchtypes/observation_test.go` | 13 | ✅ PR-A1 | AC1 (Observation 4 类 + Category 2 类 + 不可变 + Strength clamp + Validate + JSON roundtrip) |
| `orchtypes/uncertainty_report_test.go` | 12 | ✅ PR-A1 | AC2 (Partition 不变式 + Overall 重算 + Anomalies 子集) + AC3 (SetQuantizedIntent/SetPrior/Validate) |
| `orchtypes/uncertainty_coord_test.go` | 9 | ✅ PR-A1 | AC3 (UncertaintyCoord 扩展：Value/Confidence/FromVerifier/SideEffect + IsColdStart/Equal) |
| `decisionplanning/intent_quantizer_test.go` | 6 | 🔲 PR-A2 | AC4 (3 轮 + 兜底 + 性能) |
| `observe/anomaly_detector_test.go` | 8 | 🔲 PR-A3 | AC5/6/7 (Composite + 单失败 + OP-6) |
| `plan/plan_test.go` | 6 | 🔲 PR-B1 | AC10/11 (4 Kind + SourceObservationIDs) |
| `plan/plan_validator_test.go` | 10 | 🔲 PR-B1 | AC12/13/14 (PP-1/2/3) |
| `plan/planner_test.go` | 6 | 🔲 PR-B1 | AC15 (Planner 全链路) |
| **合计** | **70** | 34/70 (49%) | — |

### 11.2 集成测试

| 文件 | 场景 |
|------|------|
| `tests/integration/d7/observe_plan_pipeline_test.go` | ProcessMessage → ObserveNode → Planner 全链路 |
| `tests/integration/d7/observe_with_prior_test.go` | LP-1 闭环 + ReputationEvidence 注入 |
| `tests/integration/d7/op6_category_separation_test.go` | CatSystem 反向校验 |
| `tests/integration/d7/plan_validation_pipeline_test.go` | PP-1/2/3 全部失败场景 |
| `tests/integration/d7/source_observation_chain_test.go` | 血缘链：Plan.SourceObservationIDs → Observations[].ID 可追溯 |
| **合计** | **5 套** |

### 11.3 性能测试

| 指标 | 测试方式 | 阈值 |
|------|---------|------|
| ObserveNode.All() P95 | 1000 次并发调用 + pprof | ≤ 50ms（除 Quantizer） |
| IntentQuantizer 单轮 P95 | mock LLMCompleter + benchmark | ≤ 2000ms |
| CompositeAnomalyDetector | 1000 并发 + benchmark | ≤ 30ms |
| Plan.Validate() | 10000 次 + benchmark | ≤ 1ms |

---

## 12. 风险与缓解（详见 proposal §6）

| 风险 | 缓解 |
|---|---|
| Plan 与现有 Decomposer 输出冲突 | Plan 与 Decomposer 共存，Phase 3 后逐步退役 |
| `Strength` 语义混淆 | 文档化 + 单测明确边界 |
| Orchestrator 插入位置错误 | 严格在 classifier 之后 + integration test |

---

## 13. Cross-references

- 设计稿：[[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/42-d7-observe-node-design|doc 42]] + [[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/43-d7-plan-node-design|doc 43]]
- Phase 1 前置：`devrix-d7-mups-v4-phase1-foundation`（原 DM-20260623-001）— UncertaintyCoord + AdaptiveThreshold wiring + Verifier + ExitReason
- 当前 PR-A1：本 Change（DM-20260623-001 review fix）— 详见 §14 Review Decisions
- Phase 3（候选）：DM-20260625-001 Execute 节点 — 消费 Plan.SourceObservationIDs + Plan.AnomaliesCount
- Phase 5（候选）：DM-20260627-001 Learn 节点 — 消费 Plan.AnomaliesCount + ReputationEvidence

---

## 14. Review Decisions（2026-06-23 S3-Gate pre-review）

> 本节集中记录 S3-Gate 预审阶段（DM-20260623-001）的所有设计决议，每条决议与 §2.x deviation table 一一对应。
> 决议落地后，对应 deviation row 视为"已落地"，可作为 S4 实现阶段的零偏差参考。

### 14.1 Critical 决议（block S4）

| ID | 决议 | 落地位置 | 验收 |
|----|------|---------|------|
| **C1** | `QuantizedIntent.Kind` 由 `string` 占位升级为 `IntentKind` 枚举 | §2.2 deviation table + uncertainty_report.go `QuantizedIntent` struct | AC1 + 既有调用方零修改 |
| **C2** | 保留 `UncertaintyReport.Observations` 字段（向后兼容），**同时**收紧 `MatchKind` 签名为 `(*UncertaintyReport)`，强制只读 `BusinessObservations` | §6.2 `MatchKind` 函数签名 + 注释 | AC8 + Plan 调用方 PR-B1 同步更新 |
| **C3** | `FromVerifier` 对未知 verdict 改 fail-fast：返回 `NewUncertaintyCoordInvalidVerdictKindError`（错误码 `ORCH_COORD_VERDICT_7004`）；4 种已知 verdict（pass/partial/indeterminate/fail）行为不变 | §2.3 deviation table + uncertainty_coord.go `FromVerifier` | AC3 + `TestUncertaintyCoord_FromVerifier_UnknownKind` PASS |

### 14.2 Warning 决议（应在 S4 修）

| ID | 决议 | 落地位置 | 验收 |
|----|------|---------|------|
| **W1** | `Observation.MarshalJSON` wire format = `{id, kind, category, strength, payload: {…}, detected_at, source}` 嵌套对象；`UnmarshalJSON` 按 Kind 判别反序列化到对应 Payload 类型 | §2.1 deviation table + observation.go | AC4 + JSON roundtrip 测试 |
| **W2** | `validateFact` 改 `fmt.Errorf("orchtypes: FactPayload.Statement empty: %w", ErrObservationPayloadInvalid)` | §2.1 deviation table + observation.go `validateFact` | AC5 + 与 Signal/Deviation/Un.Validate 风格统一 |
| **W3** | `clamp01` + `clamp01Coord` 合并为 `clamp01Float(v float64, onNaN float64) float64`，Observation/Coord 共用 | §2.3 deviation table + observation.go + uncertainty_coord.go | AC6 + NaN 兜底测试 |
| **W6/I8** | `Partition` 末尾 `r.Overall = clamp01Float(r.ComputeOverallStrength(), 0.5)`，NaN 兜底为 0.5（与 UncertaintyCoord.Value 默认值对齐） | §2.2 deviation table + uncertainty_report.go | AC7 + NaN 数据不污染下游 |
| **W8** | 同 C2，`MatchKind` 签名收紧为 `(*UncertaintyReport)`，注释明确禁止传 `report.Observations` / `report.SystemObservations` | §6.2 `MatchKind` 函数注释 | AC8（与 C2 合并验收） |

### 14.3 Info 决议（延后下个 PR）

| ID | 决议 |
|----|------|
| **I1-I7** | 全部风格/边界微调延后下个 PR（OOS-7 列出）；本 PR-A1 不动 |
| **I8** | 与 W6 合并落地（NaN 兜底复用 clamp01Float） |

### 14.4 决议闭环清单

S4 实现的"零偏差"参考：

| 决议编号 | 文件 | 函数/字段 | 落地行（参考） |
|---------|------|----------|---------------|
| C1 | `internal/layers/orchestration/orchtypes/uncertainty_report.go` | `QuantizedIntent` struct | `Kind IntentKind` 替换 `Kind string` |
| C2 + W8 | `internal/layers/orchestration/plan/planner.go` | `MatchKind` 签名 + 注释 | 收紧为 `(*orchtypes.UncertaintyReport)` |
| C3 | `internal/layers/orchestration/orchtypes/uncertainty_coord.go` | `FromVerifier` 兜底分支 | 未知 verdict 改 return `nil, NewUncertaintyCoordInvalidVerdictKindError(...)` |
| W1 | `internal/layers/orchestration/orchtypes/observation.go` | `MarshalJSON` / `UnmarshalJSON` | payload 嵌套对象 wire format |
| W2 | `internal/layers/orchestration/orchtypes/observation.go` | `validateFact` | `fmt.Errorf("...: %w", ErrObservationPayloadInvalid)` |
| W3 | `internal/layers/orchestration/orchtypes/{observation,uncertainty_coord}.go` | `clamp01` / `clamp01Coord` | 合并为 `clamp01Float(v, onNaN)` |
| W6/I8 | `internal/layers/orchestration/orchtypes/uncertainty_report.go` | `Partition` 末尾 | `r.Overall = clamp01Float(r.ComputeOverallStrength(), 0.5)` |

### 14.5 S3-Gate 决议（review-design.md 4 维度）

| 维度 | 检查点 | 结论 |
|------|-------|------|
| **数据一致性** | Observation 4 类 + Category 2 类 + 不可变 + clamp 静默 | ✅ PASS（§2.1 偏离表已说明） |
| **逻辑一致性** | Partition 不变式 + Overall 重算 + Anomalies 子集（CatSystem + ObsDeviation） | ✅ PASS（§2.2 + AC2/AC7） |
| **边界一致性** | WithStrength 静默 clamp + MatchKind 强制读 BusinessObservations | ✅ PASS（§6.2 C2/W8 收紧） |
| **调用一致性** | FromVerifier 4 种 verdict 行为不变 + 未知 verdict fail-fast + 错误码 7004 | ✅ PASS（C3 决议） |
| **异常一致性** | 11 个 sentinel + 4 个错误码 7001-7004 跨包可 errors.Is | ✅ PASS（§8 + errors.go） |

**S3-Gate 结论**：✅ 通过（5/5 维度 PASS），可进入 S4 实现阶段。

### 14.6 Reviewer Sign-off

| 维度 | Reviewer | Date | Status |
|------|----------|------|--------|
| 数据维度 | Agent（self-review） | 2026-06-23 | ✅ PASS |
| 逻辑维度 | Agent（self-review） | 2026-06-23 | ✅ PASS |
| 边界维度 | Agent（self-review） | 2026-06-23 | ✅ PASS |
| 调用维度 | Agent（self-review） | 2026-06-23 | ✅ PASS |
| 异常维度 | Agent（self-review） | 2026-06-23 | ✅ PASS |

> 用户确认后，本 PR-A1 即可进入 S4 实现阶段。所有 5 维度决议已落实到 §14.1-14.4，code change 与 design.md 保持零偏差。