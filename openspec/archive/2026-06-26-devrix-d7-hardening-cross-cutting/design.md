# Design: D7 hardening/ 横切包迁移

**Change ID:** `devrix-d7-hardening-cross-cutting`
**Demand ID:** DM-20260626-003
**Priority:** P1
**Sprint:** d7-v6 follow-up
**PR Count:** 1
**Status:** S3_Design → S3-Gate Approved → S4_Implemented → S5_Accepted → S7_Archived
**SoT:** `devrix-d7-six-s-simplification` (DM-20260626-001) design.md §① + acceptance-report.md §7

---

## §0 S3-Gate Review Conclusion

**结论：APPROVED**（无 CRITICAL、无 HIGH、无 MEDIUM）

详见 §9 决策记录。

---

## 1. 根因分析

`devrix-d7-six-s-simplification` (DM-20260626-001) 在 v6.0.0 域升级中把 D7 编排层的 14 S 博弈角色精简为 6 S + 1 横切。9 个 spec 文档（d7-domain.md / design.md 等）已明确声明：

```
6 S + 1 横切:
S1 WorkModel         (State Authority)
S2 SessionOrchestrator (Mediator + Turn Leader + Error Recovery)
S3 WaveScheduler     (Mechanism Designer)
S4 ExecutionFlow+Verify (Costly Signaler + Certifier)
S5 DecisionPlanning+Observe (Info Producer + Quantizer)
S6 MUPS Pipeline     (Pipeline Coordinator + Memory Curator)
横切 Hardening         (Discipline Keeper) → orchestration/hardening/  ← 当前不存在
```

**根因：**
- 9 个 spec 文档 v2.0.0/v4.0.0 落地后，`orchestration/hardening/` 目录从未创建
- PR #215 (DM-20260626-001) + PR #216 (DM-20260626-002) 修复了 6 S 文档语义层 + mups/ 子树物理层
- 但"横切 Discipline Keeper"对应的代码位置仍是散落的：
  - `sessionorchestrator/metrics.go` (61 行 InterruptMetrics)
  - `turn/recovery.go` (133 行 LLM 错误恢复 helpers)
  - `escape/circuit_breaker.go` (420 行 5-Layer CB — 见 §3 Decision 1 不动)

**本次 PR 范围：**
- 物理目录创建 `orchestration/hardening/`
- 迁移 2 类真正横切的关注点：metrics.go + recovery.go（subset）
- 修复 v6.0.0 文档承诺与代码现状的不一致

## 2. 方案设计

### 2.1 物理目录结构

```
orchestration/hardening/   (NEW, Discipline Keeper 横切角色)
├── doc.go                  (NEW, ~30 行说明横切职责)
├── metrics.go              (← sessionorchestrator/metrics.go, package rename)
├── metrics_test.go         (← sessionorchestrator/metrics_test.go, package rename)
├── recovery.go             (← turn/recovery.go subset, package rename)
└── recovery_test.go        (← turn/recovery_test.go subset, package rename)
```

### 2.2 hardening/recovery.go 内容拆分

turn/recovery.go (133 行) 内容拆分为：

**迁到 hardening/recovery.go（package-level 纯函数，跨 S 通用）：**
- `IsContextLengthError(err error) bool` — 纯函数，无 orchestrator 依赖
- `IsOverloadOr5xx(err error) bool` — 纯函数，调用 IsContextLengthError
- `NeedsMaxOutputTokenRecovery(finishReason string) bool` — 纯函数
- `MaxOutputTokensRecoveryMessage` const — LLM output token 恢复提示

**留在 turn/recovery.go（receiver method，紧耦合 *DefaultOrchestrator）：**
- `partialStreamEmit` struct (unexported) — orchestrator.go 内部状态
- `emitStreamRecoveryTombstones(...)` — 调 partialStreamEmit，被 orchestrator.go 调用
- `(o *DefaultOrchestrator) compressMessagesForRecovery(...)` — 调 `o.runCompress`
- `(o *DefaultOrchestrator) invokeStreamWithRecovery(...)` — 调 `o.llm.InvokeStream` + `IsContextLengthError` + `compressMessagesForRecovery`

**理由（详见 §3 Decision 2）：**
- 纯函数 IsContextLengthError / IsOverloadOr5xx / NeedsMaxOutputTokenRecovery 没有 receiver，不依赖 *DefaultOrchestrator 内部状态，可独立搬到 hardening/
- receiver method compressMessagesForRecovery / invokeStreamWithRecovery 深度耦合 `*DefaultOrchestrator`（访问 `o.runCompress` + `o.llm` 字段），如果硬搬到 hardening/ 需要：
  - Option A: 重写为 hardening.RecoveryOrchestrator struct + wrap orchestrator deps → 引入 interface 类型 + 重构 orchestrator.go 字段访问
  - Option B: 抽离为 free function 接受 deps 作为参数 → 改 orchestrator.go 字段访问方式
  - Option C: 保留 receiver 在 turn/，纯函数迁 hardening/ → **选这个，最小变动**

### 2.3 sessionorchestrator/metrics.go 整文件迁

`sessionorchestrator/metrics.go` (61 行) 内容整体搬到 `hardening/metrics.go`：
- `package sessionorchestrator` → `package hardening`
- 类型 + 方法 + 测试 0 变化
- `sessionorchestrator/interrupt.go` Metrics 字段类型 `*InterruptMetrics` → `*hardening.InterruptMetrics` + 加 import
- `sessionorchestrator/interrupt_test.go` 加 import hardening
- `sessionorchestrator/metrics_test.go` (本文件) → `hardening/metrics_test.go` 整文件迁

### 2.4 escape/circuit_breaker.go 不动（Decision 1 详见 §3）

420 行 5-Layer CB (L0-L5) + CircuitBreakerSet + baseCircuitBreaker 完整保留在 `escape/` 包。

**依赖关系：**
- `escape/engine.go` line 40: `cbSet *CircuitBreakerSet` 字段
- `escape/engine.go` line 170: `(e *EscapeEngine) CircuitBreakerSet() *CircuitBreakerSet` 暴露
- `escape/loop_depth_tracker.go` line 44: 注释引用 CB
- 5-Layer CB 决策 (Evaluate 返回 EscapeDecision) 喂给 EscapeEngine 决策链

如果硬迁 hardening/，会违反依赖方向（EscapeEngine 应是核心机制，不应 import hardening 横切包）。

## 3. 决策记录

### Decision 1: circuit_breaker.go 保留在 escape/，不迁 hardening/

**结论：** 不动 `escape/circuit_breaker.go` (420 行)。

**理由：**
- 原 v6.0.0 plan §1 §3 列出 3 文件（metrics.go + circuit_breaker.go + error_recovery.go）
- 但 `circuit_breaker.go` 的语义实质是 **EscapeEngine 核心机制**而非横切观测：
  - `engine.go` 直接持有 `cbSet *CircuitBreakerSet` 字段
  - CB.Evaluate 返回 EscapeDecision 喂给 EscapeEngine 决策链
  - 13 类失败降级矩阵依赖 CB + LoopDepthTracker + LoopBudget 协同
  - V5 EscapeEngine DM-20260625-003 (PR #198 + PR-V5.6 #200) 已 S7_Archived
- 迁 hardening/ 违反依赖方向：EscapeEngine 是核心机制，hardening 才是横切观测
- 与 design.md §7 SoT + doc 38 §21 SoT 不符
- 未来如需"CB 状态对外 observability 暴露"，可在 hardening/metrics.go 新增 wrapper（通过 EscapeEngine 注入），不搬 CB 本身

**附议：** 本次仅迁 metrics.go + recovery.go（subset），符合 v6.0.0 plan 精神（横切关注点收口），但**对 plan 提到的 3 文件做了 scope 收紧**，原因明确文档化在 §2.4 + Decision 1。

### Decision 2: recovery.go receiver methods 保留在 turn/，纯函数迁 hardening/

**结论：** `compressMessagesForRecovery` 和 `invokeStreamWithRecovery` 两个 receiver method 留在 `turn/recovery.go`，不迁 hardening/。

**理由：**
- 这两个 method 是 `*DefaultOrchestrator` 方法，深度耦合：
  - `compressMessagesForRecovery` 调 `o.runCompress(ctx, req, &CompressHint{...})` — runCompress 是 *DefaultOrchestrator 私有方法
  - `invokeStreamWithRecovery` 调 `o.llm.InvokeStream(ctx, invokeReq)` + `o.compressMessagesForRecovery` + `IsContextLengthError` — llm 字段 + 同包方法
- 如果硬迁 hardening/：
  - Option A: hardening.RecoveryOrchestrator struct + wrap orchestrator deps → 引入新接口类型
  - Option B: 抽离为 free function → 改 orchestrator.go 字段访问
  - Option C: 保留 receiver 在 turn/ → **选这个，0 重构**
- 纯函数 IsContextLengthError / IsOverloadOr5xx / NeedsMaxOutputTokenRecovery + MaxOutputTokensRecoveryMessage 常量 **不依赖 *DefaultOrchestrator**，可独立搬到 hardening/

**后续 orchestrator.go 改动：**
- 加 `import "github.com/devrix/devrix/internal/layers/orchestration/hardening"` (1 行)
- `NeedsMaxOutputTokenRecovery(finishReason)` → `hardening.NeedsMaxOutputTokenRecovery(finishReason)` (1 处)
- `MaxOutputTokensRecoveryMessage` → `hardening.MaxOutputTokensRecoveryMessage` (1 处)
- `partialStreamEmit` + `emitStreamRecoveryTombstones` 不变（同 package）
- `IsContextLengthError` 在 `compressMessagesForRecovery` / `invokeStreamWithRecovery` 内部调用，需要 hardening 导入。但 turn/recovery.go 仍可在 hardening 包内调用 hardening.IsContextLengthError（无需前缀因为 receiver method 内部），或加 hardening. 前缀（更明确）。

**最终 turn/recovery.go 内容（精简后）：**
```go
package turn

import (
    "context"
    "github.com/devrix/devrix/internal/layers/llmgateway"
    "github.com/devrix/devrix/internal/layers/orchestration/hardening"  // NEW
    "github.com/devrix/devrix/internal/shared/contracts"
    "github.com/devrix/devrix/internal/shared/types"
)

type partialStreamEmit struct { ... }  // KEEP

func emitStreamRecoveryTombstones(...) { ... }  // KEEP

func (o *DefaultOrchestrator) compressMessagesForRecovery(ctx context.Context, req TurnRequest, messages []types.Message) []types.Message {
    if o == nil || len(messages) == 0 { return messages }
    result := o.runCompress(ctx, req, &CompressHint{...})
    if result.Summary == "" { return messages }
    return []types.Message{{...}}
}

const maxOutputTokenRecoveryAttempts = 3  // KEEP (used in orchestrator.go loop bound)

func (o *DefaultOrchestrator) invokeStreamWithRecovery(
    ctx context.Context,
    req TurnRequest,
    invokeReq LLMInvokeRequest,
) (<-chan llmgateway.Chunk, error) {
    ch, err := o.llm.InvokeStream(ctx, invokeReq)
    if err == nil || !hardening.IsContextLengthError(err) {  // 加 hardening. 前缀
        return ch, err
    }
    compressed := o.compressMessagesForRecovery(ctx, req, invokeReq.Messages)
    ...
}
```

### Decision 3: hardening/ 使用全新 package 名（不复用 escape / turn / sessionorchestrator）

**结论：** hardening 包使用全新 `package hardening` 声明，与原 metrics.go / recovery.go 旧 package 声明不同。

**理由：**
- hardening 是新横切角色，与 mups migration 的"保留 package execute / package learn"策略相反
- hardening 跨多 S 调用（sessionorchestrator.interrupt + turn.orchestrator + future），新名字明确归属
- 与 v6.0.0 文档"横切 Discipline Keeper"命名一致
- bootstrap wire 14 → 6 收口（follow-up #6）需要明确"hardening" 命名空间

**对应改动：**
- metrics.go: `package sessionorchestrator` → `package hardening`（1 行）
- recovery.go (turn/ 拆分后): `package turn` → `package hardening`（1 行）

### Decision 4: hardening/recovery_test.go 子集拆分 + recoveryStubLLM 留在 turn/

**结论：** 
- hardening/recovery_test.go 包含 3 个测试：TestIsContextLengthError, TestIsOverloadOr5xx, TestNeedsMaxOutputTokenRecovery
- turn/recovery_test.go 保留 3 个测试：TestInvokeStreamWithRecovery_CompressesOn413, TestEmitStreamRecoveryTombstones, TestRunTurn_MaxOutputTokensRecovery_TombstoneAndRetry
- turn/recovery_test.go 保留 recoveryStubLLM (本地 stub)

**理由：**
- 3 个纯函数测试零依赖 turn 包，可独立 hardening 包测试
- 3 个 orchestrator 耦合测试依赖 turn/ stubs (NewOrchestrator + runCompress + llm field)，留在 turn/ 避免 stubs 重复

## 4. 关键接口变化

### 4.1 hardening/metrics.go 公开 API

```go
package hardening

// InterruptMetrics counts cancel step failures across all 3 cancellers
// (was: SessionOrchestrator.Metrics field).
type InterruptMetrics struct {
    handleCancels int64  // unexported
    stateCancels  int64
    workCancels   int64
    handlerFails  int64
    ...
}

// Snapshot returns an atomic snapshot (nil-safe).
func (m *InterruptMetrics) Snapshot() InterruptMetricsSnapshot

// InterruptMetricsSnapshot is the JSON-friendly view.
type InterruptMetricsSnapshot struct {
    HandleCancels int64 `json:"handle_cancels"`
    StateCancels  int64 `json:"state_cancels"`
    WorkCancels   int64 `json:"work_cancels"`
    ...
}

// TotalCancelFailures returns sum of all 3 cancellers' failure counts.
func (m *InterruptMetrics) TotalCancelFailures() int64
```

### 4.2 hardening/recovery.go 公开 API

```go
package hardening

// IsContextLengthError reports prompt-too-long failures (TD-QL-01, D7 path).
func IsContextLengthError(err error) bool

// IsOverloadOr5xx reports overload / 5xx / rate-limit errors suitable for
// gateway-level fallback retry (TD-QL-03 — handled in llmgateway.Stream).
func IsOverloadOr5xx(err error) bool

// NeedsMaxOutputTokenRecovery reports finish_reason=length truncation (TD-QL-02).
func NeedsMaxOutputTokenRecovery(finishReason string) bool

// MaxOutputTokensRecoveryMessage is injected when the provider stops with
// finish_reason=length so the model can continue without orphan UI chunks.
const MaxOutputTokensRecoveryMessage = "Your previous response was truncated..."
```

### 4.3 turn/orchestrator.go 调用点改动

```go
// 改动前（orchestrator.go line 628, 638）
if !NeedsMaxOutputTokenRecovery(finishReason) {
    ...
}
...
st.messages = append(st.messages, types.Message{
    SessionID: req.SessionID,
    Role:      types.MessageRoleUser,
    Content:   MaxOutputTokensRecoveryMessage,
})

// 改动后
if !hardening.NeedsMaxOutputTokenRecovery(finishReason) {
    ...
}
...
st.messages = append(st.messages, types.Message{
    SessionID: req.SessionID,
    Role:      types.MessageRoleUser,
    Content:   hardening.MaxOutputTokensRecoveryMessage,
})
```

### 4.4 sessionorchestrator/interrupt.go 调用点改动

```go
// 改动前（interrupt.go line 42）
type Interrupt struct {
    ...
    Metrics *InterruptMetrics  // 同 package
    ...
}

// 改动后
import "github.com/devrix/devrix/internal/layers/orchestration/hardening"

type Interrupt struct {
    ...
    Metrics *hardening.InterruptMetrics  // 跨 package
    ...
}
```

## 5. 数据流分析

### 5.1 hardening/metrics.go 数据流（无变化）

```
SessionOrchestrator.interrupt.Metrics (was: *sessionorchestrator.InterruptMetrics)
                ↓ (*hardening.InterruptMetrics)
hardening/metrics.go::InterruptMetrics.Snapshot()
                ↓
InterruptMetricsSnapshot (JSON view, fields不变)
                ↓
D5 observability (后续 PR 推)
```

### 5.2 hardening/recovery.go 数据流（receiver 跨包调用）

```
turn/orchestrator.go::RunTurn
    ↓
turn/orchestrator.go (调用 hardening.NeedsMaxOutputTokenRecovery)
    ↓
hardening/recovery.go::NeedsMaxOutputTokenRecovery (纯函数)
    ↓
返回 bool → turn/orchestrator.go 决定是否进入 recovery loop
```

```
turn/orchestrator.go::RunTurn (streamRecovery loop)
    ↓
turn/orchestrator.go (调用 hardening.MaxOutputTokensRecoveryMessage)
    ↓
hardening/recovery.go::MaxOutputTokensRecoveryMessage (常量)
    ↓
拼接到 st.messages
```

```
turn/orchestrator.go::invokeStreamWithRecovery (receiver method)
    ↓
调 hardening.IsContextLengthError(err)
    ↓
返回 bool → 决定是否触发 compress 重试
```

## 6. 文件清单（最终）

### 新建文件 (1)
| 路径 | 说明 |
|------|------|
| `internal/layers/orchestration/hardening/doc.go` | ~30 行说明横切职责（Discipline Keeper）|

### git mv 迁移文件 (4)
| 原路径 | 新路径 | 说明 |
|--------|--------|------|
| `internal/layers/orchestration/sessionorchestrator/metrics.go` | `internal/layers/orchestration/hardening/metrics.go` | package rename + InterruptMetrics 不变 |
| `internal/layers/orchestration/sessionorchestrator/metrics_test.go` | `internal/layers/orchestration/hardening/metrics_test.go` | 同上 |
| `internal/layers/orchestration/turn/recovery.go` (subset) | `internal/layers/orchestration/hardening/recovery.go` | 仅 4 纯函数 + 1 const 迁出 |
| `internal/layers/orchestration/turn/recovery_test.go` (subset) | `internal/layers/orchestration/hardening/recovery_test.go` | 仅 3 纯函数测试迁出 |

### 改动文件 (4)
| 路径 | 改动 |
|------|------|
| `internal/layers/orchestration/sessionorchestrator/interrupt.go` | 加 import hardening + Metrics 字段类型 `*InterruptMetrics` → `*hardening.InterruptMetrics` |
| `internal/layers/orchestration/sessionorchestrator/interrupt_test.go` | 加 import hardening |
| `internal/layers/orchestration/turn/orchestrator.go` | 加 import hardening + 2 处 `hardening.` 前缀调用 |
| `internal/layers/orchestration/turn/recovery.go` (KEEP 部分) | 加 import hardening + 1 处 `hardening.IsContextLengthError` 调用 |

### 不改文件 (1)
| 路径 | 说明 |
|------|------|
| `internal/layers/orchestration/escape/circuit_breaker.go` | V5 EscapeEngine 核心机制 (Decision 1) |

### 文档同步 (4)
| 路径 | 改动 |
|------|------|
| `openspec/specs/d7-orchestration/d7-domain.md` v2.0.0 → v2.1.0 | §① Cross-cutting Hardening 章节包路径描述更新 |
| `openspec/specs/d7-orchestration/design.md` v4.0.0 → v4.1.0 | §① Discipline Keeper 包路径描述更新 |
| `openspec/specs/d7-orchestration/t-registry.md` v4.2.0 → v4.3.0 | 新增 4 P0 T (D7-S7-A01-T01..T04) |
| `openspec/t-registry.md` (root) v5.2.0 → v5.3.0 | 新增 DM-20260626-003 增量条目 + D7 行更新 |

**diff 预估：**
- 创建 1 文件 (doc.go ~30 行)
- git mv 4 文件（rename 100%，内容变化：仅 package 声明 + receiver 调整）
- 改动 4 文件（~10 行 import + 类型引用）
- 不改 1 文件 (circuit_breaker.go)
- 文档同步 4 文件（~30 行描述更新）
- **总计：~70 行净变化**

## 7. 回归风险评估

### 7.1 风险清单

| 风险 | 等级 | 缓解 |
|------|------|------|
| interrupt.go Metrics 字段类型跨包引用 | 中 | 中断点：`*hardening.InterruptMetrics` + interrupt.go 加 import |
| orchestrator.go 2 处 hardening 引用 | 低 | hardening 同包 import，加 `hardening.` 前缀即可 |
| recovery_test.go 拆分 2 文件可能漏 import | 低 | Step 2 同步拆 + Step 3 测试验证 |
| hardening 包新创建，CI 镜像缓存 | 低 | 删旧目录后强制 re-build |
| circuit_breaker.go 不动可能让 review 疑惑 | 低 | Decision 1 + §2.4 明确解释 |
| LP-1/LP-2/LP-5 行为漂移 | 极低 | 0 函数逻辑变化，仅物理迁移 |

### 7.2 测试覆盖验证

**Step 3 必跑：**
- `go build ./...` → 0 错误
- `go vet ./...` → 0 警告
- `go test ./internal/layers/orchestration/... -race -count=1` → 23/23 PASS（含 hardening 1 新包）
- LP-1 (Bayesian reputation) / LP-2 (Memory 3 通道) / LP-5 (Cross-session traceability) 集成测试 → 100% 兼容

## 8. 回滚方案

如果 S5 验收发现不可逆问题：
1. revert 整个 PR (1 commit)
2. master 退回 PR-1 commit 前状态
3. v6.0.0 文档承诺与代码现状的不一致继续存在
4. 重新评估（可能 scope 拆分更细或推迟）

**回滚成本：低**（单 PR + 单 squash commit）

## 9. 相关决策

### 9.1 与 DM-20260626-001 (devrix-d7-six-s-simplification) 关系
- 6 S + 1 横切文档已 S7_Archived (PR #215)
- hardening/ 横切包本次首次代码侧落地
- 后续 follow-up #3-#6 不依赖本次硬化包（本次 scope 自包含）

### 9.2 与 DM-20260626-002 (devrix-d7-mups-package-migration) 关系
- mups 子树物理迁移已 S7_Archived (PR #216)
- 本次硬化迁移与 mups 平行（不同物理目录）
- 模式一致：物理目录迁移 + import path 替换 + 0 函数逻辑变化

### 9.3 与后续 follow-up PR 关系
- follow-up #3 (6s-package-merge): turn/ + autoclose → sessionorchestrator/ — 与本次正交
- follow-up #4 (6s-verify-promotion): exit_reason + observe/verify → executionflow/verify/ — 与本次正交
- follow-up #5 (6s-observe-merge): observe/orchtypes → decisionplanning/ — 与本次正交
- follow-up #6 (6s-bootstrap-slim): wire 14 → 6 — **依赖 hardening/ 已落地**
- 本次 = follow-up #2 of 6

### 9.4 Change ID
`devrix-d7-hardening-cross-cutting` (DM-20260626-003)
