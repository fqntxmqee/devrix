# Design: D6 Evolution Review Fixes

**Change ID:** devrix-d6-evolution-review-fixes
**Demand ID:** DM-20260621-011
**Status:** S3_Design
**Date:** 2026-06-21

---

## 1. 设计目标

修复 D6 演化域 2026-06-21 deep review 识别的 Phase 1 阻塞合并问题（1 CRITICAL + 3 HIGH = 4 issue），达成 S4-Gate APPROVED 状态。

**核心设计原则**（与 `devrix-d7-error-aggregation-and-metrics` DM-20260621-010 一致）：

1. **三联固化** —— `atomic.Int64` 计数器 + `slog.Warn` + `errors.Join` 错误聚合
2. **type alias 向后兼容** —— v2.4.0 引入新名 + alias，v2.5.0 删 alias
3. **bridge 一次性清理** —— 不留 shim，参照 `devrix-d5-v2-terminal` (DM-20260619-006) B2a→B2b 模式
4. **log.Fatal 替代 panic** —— 启动期 fatal 走 `log.Fatal`，业务错误走 `SentinelError`

## 2. 架构总览

```
PR-A (low risk):
  verify/_invariant.go:24    →  panic → log.Fatal
  guard/intervention.go:74   →  _, _ = Wait  → metric + slog + errors.Join
  guard/metrics.go           →  + WaitFailed atomic.Int64
  guard/intervention_test.go →  + TestInterventionExecutor_WaitFailure_RecordsMetric
  verify/_invariant_test.go  →  + TestMustParseVerifyInvariants_BadStruct_LogsFatal

PR-B (medium risk, bridge coupling):
  delete: internal/layers/evolution/eval/bridge.go
  delete: internal/layers/evolution/orchestration/bridge.go
  guard/config.go            →  OrchestrationConfig → GuardConfig (+ alias)
  guard/observer.go          →  OrchestrationObserver → GuardObserver (+ alias)
  guard/validator.go         →  RuntimeOrchestrationValidator → RuntimeGuardValidator (+ alias)
  guard/metrics.go           →  orchMetrics → guardMetrics, 6× orch_* → guard_*
  observability/bridge.go    →  同步删除 Orchestration* re-export (如有)

PR-C (docs-only):
  spec.md v2.3.0 → v2.4.0
  design.md v2.2.0 → v2.3.0
  t-registry.md v3.1.0 → v3.2.0 (≥ 5 P0 T 点新增)
  acceptance-report.md
  S6 archive
```

## 3. PR-A 详细设计

### 3.1 H-2 panic → log.Fatal

**位置**：`internal/layers/evolution/verify/_invariant.go`

**Before**:
```go
func mustParseVerifyInvariants() ltllite.InvariantSet {
    set, err := ltllite.ParseStruct(verifyInvariants{})
    if err != nil {
        panic("ltllite: verify invariant parse failed: " + err.Error())
    }
    return set
}

var verifyInvariantSet = mustParseVerifyInvariants()
```

**After**:
```go
// init() runs at package load; on parse failure → log.Fatalf (exit 1).
func init() {
    set, err := parseVerifyInvariants()
    if err != nil {
        log.Fatalf("verify: ltllite invariant parse failed: %v", err)
    }
    verifyInvariantSet = set
}

func parseVerifyInvariants() (ltllite.InvariantSet, error) {
    return ltllite.ParseStruct(verifyInvariants{})
}

var verifyInvariantSet ltllite.InvariantSet
```

**额外步骤（dead code 激活）**：

`_invariant.go` 是死代码 —— Go 工具链忽略 `_` 前缀文件。**重命名为 `invariant.go`** 让 export 真正进入包二进制。

```bash
git mv internal/layers/evolution/verify/_invariant.go internal/layers/evolution/verify/invariant.go
git mv internal/layers/evolution/verify/_invariant_test.go internal/layers/evolution/verify/invariant_test.go
```

**测试**：
```go
// verify/invariant_test.go
func TestParseVerifyInvariants_GoodStruct_Succeeds(t *testing.T) {
    set, err := parseVerifyInvariants()
    if err != nil {
        t.Fatalf("default invariant parse failed: %v", err)
    }
    if len(set.Invariants) == 0 {
        t.Error("expected non-empty invariant set")
    }
}

func TestParseVerifyInvariants_BadStruct_ReturnsError(t *testing.T) {
    type badStruct struct {
        X string `invariant:""`  // 空标签触发 ErrInvalidInvariant
    }
    _, err := ltllite.ParseStruct(badStruct{})
    if err == nil {
        t.Fatal("expected parse error for empty invariant tag")
    }
}

func TestVerifyInvariants_InitSucceeds(t *testing.T) {
    if len(verifyInvariantSet.Invariants) == 0 {
        t.Error("verifyInvariantSet should be populated after init()")
    }
}

func TestCheckVerifyInvariants_NoViolations(t *testing.T) {
    state := ltllite.MapState{ /* all propositions true */ }
    if v := CheckVerifyInvariants(state); len(v) != 0 {
        t.Errorf("expected no violations, got %d", len(v))
    }
}

func TestCheckVerifyInvariants_ViolationDetected(t *testing.T) {
    // ReadOnly invariant: verify_called => no_plan_mutation
    // 故意让 post 为 false, 期望检测到 violation
    state := ltllite.MapState{
        "verify_called": true, "no_plan_mutation": false,
        // ... 其余 true
    }
    if v := CheckVerifyInvariants(state); len(v) == 0 {
        t.Error("expected violation, got 0")
    }
}
```

**关键决策**：
- 使用 `log.Fatalf` 而非 `panic`：符合 D5/D7 规范约定（业务 fatal 走 log）
- 启动期失败进程退出码非 0，systemd / k8s 会重启，避免静默启动后 invariant 缺失
- 移除 `mustXxx` 命名约定（Go 习惯用法之一，本项目遵循"显式 error 返回"原则）
- **`_invariant.go` → `invariant.go` 重命名不可省略** —— 不然 export 永远进不了包二进制

### 3.2 H-3 silent swallow → metric + errors.Join

**位置**：`internal/layers/evolution/guard/intervention.go:74`

**Before**:
```go
func (ie *InterventionExecutor) terminateAndReroute(ctx context.Context, session *types.Session, iv Intervention) error {
    current := ie.agents.SessionAgent(session.SessionID)
    if current != nil {
        if err := current.Terminate(ctx); err != nil {
            slog.Warn("terminate current agent on reroute", "error", err)
        }
        _, _ = current.Wait(ctx)  // ← H-3
    }

    if iv.MilestoneFail || iv.TaskFail {
        taskID := iv.FailReason
        _ = ie.tasks.Fail(taskID, iv.Reason)  // ← 同样静默
    }
    // ...
}
```

**After**:
```go
func (ie *InterventionExecutor) terminateAndReroute(ctx context.Context, session *types.Session, iv Intervention) error {
    var errs []error

    current := ie.agents.SessionAgent(session.SessionID)
    if current != nil {
        if termErr := current.Terminate(ctx); termErr != nil {
            slog.Warn("terminate current agent on reroute", "error", termErr)
            errs = append(errs, fmt.Errorf("terminate: %w", termErr))
        }
        if waitErr := current.Wait(ctx); waitErr != nil {
            ie.metrics.WaitFailed.Add(1)
            slog.Warn("wait current agent failed", "session_id", session.SessionID, "error", waitErr)
            errs = append(errs, fmt.Errorf("wait: %w", waitErr))
        }
    }

    if iv.MilestoneFail || iv.TaskFail {
        taskID := iv.FailReason
        if failErr := ie.tasks.Fail(taskID, iv.Reason); failErr != nil {
            ie.metrics.TaskFailFailed.Add(1)
            slog.Warn("task fail failed", "task_id", taskID, "error", failErr)
            errs = append(errs, fmt.Errorf("task_fail: %w", failErr))
        }
    }

    return errors.Join(errs...)  // ← 与 D7 error aggregation 同模式
}
```

**guard/metrics.go 扩展**：
```go
type orchMetrics struct {
    decisionsTotal     metrics.Counter
    validationsTotal   metrics.Counter
    interventionsTotal metrics.Counter
    judgeLatency       metrics.Histogram
    observerActive     metrics.Gauge
    decisionsByStage   metrics.Counter

    // PR-A 新增（H-3 静默吞错修复）
    WaitFailed        atomic.Int64  // ← 注意：PR-B 整体 rename 为 guardMetrics
    TaskFailFailed    atomic.Int64
}
```

**测试**：
```go
// guard/intervention_test.go
func TestInterventionExecutor_WaitFailure_RecordsMetric(t *testing.T) {
    ie, mockAgent, mockTasks := newTestInterventionExecutor()
    mockAgent.terminateErr = nil
    mockAgent.waitErr = errors.New("agent wait timeout")

    iv := Intervention{Reason: "test reroute"}
    err := ie.terminateAndReroute(context.Background(), testSession(), iv)

    if err == nil {
        t.Fatal("expected non-nil error from Wait failure")
    }
    if !errors.Is(err, mockAgent.waitErr) {
        t.Errorf("expected wrapped wait error, got %v", err)
    }
    if got := ie.metrics.WaitFailed.Load(); got != 1 {
        t.Errorf("WaitFailed counter = %d, want 1", got)
    }
}

func TestInterventionExecutor_TaskFailFailure_RecordsMetric(t *testing.T) {
    // 同上模式，验证 TaskFailFailed 计数器
}
```

## 4. PR-B 详细设计

### 4.1 bridge.go 删除

**删除文件**：
- `internal/layers/evolution/eval/bridge.go`（旧的 v1.0 → v2.0 桥接，11 个 alias 之一）
- `internal/layers/evolution/orchestration/bridge.go`（同上）

**迁移检查**：
```bash
# 全仓 grep 确认外部 import 路径
git grep "evolution/eval/bridge" -- ':!*.md' ':!.openspec.yaml'
git grep "evolution/orchestration/bridge" -- ':!*.md' ':!.openspec.yaml'
# 期望：0 命中（除要删除的 bridge.go 自身）
```

**删除后调整**：
- `internal/layers/observability/bridge.go` 同步检查 —— 若有 Orchestration* re-export 需同步删除
- 任何外部调用方改为直接 `import "internal/layers/evolution/guard"`（PR-B rename 后）

### 4.2 Orchestration* → Guard* 重命名

**完整 rename 映射表**：

| 旧名（v2.0-v2.3） | 新名（v2.4） | 文件 |
|---|---|---|
| `OrchestrationConfig` | `GuardConfig` | `guard/config.go` |
| `OrchestrationObserver` | `GuardObserver` | `guard/observer.go` |
| `RuntimeOrchestrationValidator` | `RuntimeGuardValidator` | `guard/validator.go` |
| `NewOrchestrationObserver` | `NewGuardObserver` | `guard/observer.go` |
| `NewRuntimeOrchestrationValidator` | `NewRuntimeGuardValidator` | `guard/validator.go` |
| `orchMetrics` struct | `guardMetrics` struct | `guard/metrics.go` |

**type alias 向后兼容模式**（v2.4 引入，v2.5 删除）：

```go
// guard/config.go
package guard

import (
    "github.com/devrix/devrix/internal/layers/evolution/guard/config"
)

type GuardConfig = config.GuardConfig

// Deprecated: use GuardConfig. v2.5.0 将删除。
//go:deprecated
type OrchestrationConfig = config.GuardConfig
```

**关键决策**：
- type alias（`type X = Y`）而非 type def（`type X Y`），保证方法集完全一致
- `//go:deprecated` 注释让 `go vet` / IDE 在外部调用旧名时给出告警
- v2.5.0 删除前需要再次全仓 grep 0 命中旧名

### 4.3 orch_* → guard_* 指标重命名

**6 个指标名映射**：

| 旧指标名（v2.0-v2.3） | 新指标名（v2.4） |
|---|---|
| `orch_decisions_total` | `guard_decisions_total` |
| `orch_validations_total` | `guard_validations_total` |
| `orch_interventions_total` | `guard_interventions_total` |
| `orch_judge_latency_seconds` | `guard_judge_latency_seconds` |
| `orch_observer_active` | `guard_observer_active` |
| `orch_decisions_by_stage` | `guard_decisions_by_stage` |

**兼容性策略**：
- OpenTelemetry 指标名是数据契约，重命名 = 数据丢失
- 短期（v2.4）：仅新名注册，旧名保留 1 个版本作为迁移期
- 中期（v2.5）：删除旧名注册
- 文档同步：dashboard JSON / alert rules 同步更新

### 4.4 PR-B 风险控制

1. **全仓 grep 验证**：
   ```bash
   git grep -n "Orchestration\|orch_" -- '*.go' ':!*.md' ':!openspec/changes/*'
   # 期望：仅命中 type alias 定义点 + comments + 文档
   ```

2. **编译验证**：
   ```bash
   go vet ./...
   go build ./...
   go test -race ./internal/layers/evolution/...
   ```

3. **CI 阻断**：
   - 添加 `scripts/check-orch-rename.sh`：grep Orchestration 在 `*.go` 中仅允许命中 alias 定义点
   - 任何 PR 命中 Orchestration 即 fail

## 5. PR-C 详细设计

### 5.1 spec.md v2.3.0 → v2.4.0

**新增章节**：
- §目录结构：删除 eval/ + orchestration/ + bridge 章节
- §DSAFT 结构：S12 韧性域新增 A02/A03 需求项
- §Revision History：新增 "v2.4.0 命名迁移"

**新增需求项**：

```markdown
### D6-S12-A02: Guard 名空间收敛（v2.4 命名迁移）

**来源**：DM-20260621-011 PR-B
**关联**：D6-S11-A02 (verify invariant), D6-S12-A01 (intervention metrics)

#### 场景
- `internal/layers/evolution/guard/` 内 0 处 `Orchestration*`（除 type alias 定义点）
- 6 个指标 `orch_*` → `guard_*` 全量重命名
- `eval/bridge.go` + `orchestration/bridge.go` 完全删除

#### Gherkin
```gherkin
Given D6 spec v2.4.0 引入 Guard* 命名
When PR-B 完成 Orchestration* → Guard* rename + bridge deletion
Then 全仓 grep `Orchestration` 仅命中 alias 定义点（≤ 6 处）
And 全仓 grep `orch_` 仅命中 metrics.go 中旧名注释（≤ 6 处）
And eval/bridge.go + orchestration/bridge.go `git ls-files` 0 命中
```

### D6-S12-A03: Verify Invariant 启动期 fail-safe

**来源**：DM-20260621-011 PR-A
**关联**：D6-S11-A02 (verify invariant parse)

#### 场景
- `verify/_invariant.go` 不再使用 `panic`，ParseStruct 失败走 `log.Fatalf`
- 启动期失败进程退出码非 0，systemd 重启接管

#### Gherkin
```gherkin
Given verify invariant struct 标签格式错误
When init() 调用 ParseStruct
Then 返回 error 而非 panic
And init() 调用 log.Fatalf 退出码 1
And `git grep "panic.*verify invariant"` 0 命中
```
```

### 5.2 t-registry.md v3.1.0 → v3.2.0

**新增 T 点**（≥ 5 P0）：

| T ID | 描述 | Test 位置 | Priority |
|------|------|-----------|----------|
| D6-S11-A02-T09 | verify/_invariant.go ParseStruct 失败不再 panic | `verify/_invariant_test.go` | P0 |
| D6-S12-A01-T01 | intervention.go Wait 失败 metric.Inc + slog.Warn + errors.Join | `guard/intervention_test.go` | P0 |
| D6-S12-A01-T02 | intervention.go tasks.Fail 失败 metric.Inc + slog.Warn | `guard/intervention_test.go` | P0 |
| D6-S12-A02-T01 | bridge.go 完全删除（git ls-files 0 命中） | `tests/integration/d6_bridge_absence_test.go` | P0 |
| D6-S12-A03-T01 | guard/ 内 0 处 `Orchestration*`（除 alias 定义点） | `tests/integration/d6_rename_test.go` | P0 |
| D6-S12-A03-T02 | 6 个指标 `orch_*` → `guard_*` 重命名与 spec v2.4.0 一致 | `guard/metrics_test.go` | P0 |

### 5.3 design.md v2.2.0 → v2.3.0

**删除**：
- §目录结构 中 `bridge.go` 章节（已删）
- §命名规范 中 v1.0 → v2.0 迁移路径（v2.4 补充新路径）

**新增**：
- §命名规范 "v2.0 → v2.4 命名迁移路径"
- §错误处理 "三联固化：atomic counter + slog + errors.Join"（与 D7 对齐）

### 5.4 acceptance-report.md

按 S5 验收模板，含：
- 13 个 AC 全 PASS（与 proposal.md AC1-AC8 对应 + 5 个子项）
- 6 个新 P0 T 点全 IMPLEMENTED
- go vet + go test -race + layer-lint 全绿
- D5 spans P95 复跑不退化

## 6. 跨 PR 依赖

```
PR-A (low risk)
   ↓ verify/intervention 单测就绪
PR-B (medium risk, 依赖 PR-A)
   ↓ rename + bridge deletion 完成
PR-C (docs + S6 archive)
```

**不能并行**：PR-B 依赖 PR-A 完成 metrics struct 扩展，否则 rename 时会漏 WaitFailed 字段。
**可并行**：PR-C 文档工作可与 PR-A/B 同时进行（仅在 PR-A/B 完成后合入）。

## 7. 兼容性矩阵

| 调用方 | v2.4 新 API | v2.4 旧 API (alias) | v2.5+ 旧 API |
|--------|-------------|---------------------|--------------|
| `OrchestrationConfig` | ✓ | ✓ `//go:deprecated` | ✗ 编译失败 |
| `GuardConfig` | ✓ | n/a | ✓ |
| `OrchestrationObserver` | ✓ | ✓ `//go:deprecated` | ✗ 编译失败 |
| `GuardObserver` | ✓ | n/a | ✓ |
| `RuntimeOrchestrationValidator` | ✓ | ✓ `//go:deprecated` | ✗ 编译失败 |
| `RuntimeGuardValidator` | ✓ | n/a | ✓ |
| `orch_decisions_total` 指标 | n/a | ✓ 注册中（v2.4 迁移期） | ✗ 不注册 |
| `guard_decisions_total` 指标 | ✓ 注册 | n/a | ✓ |

## 8. 参考

- 完整 review 报告：`openspec/changes/d6-deep-review/d6-review-report.md`
- 姊妹 change：`openspec/archive/2026-06-21-devrix-d7-error-aggregation-and-metrics/`（PR 拆分模式 + S6 归档流程）
- 模板 change：`openspec/archive/2026-06-19-devrix-d5-v2-terminal/`（bridge 清债 + 物理路径归位）