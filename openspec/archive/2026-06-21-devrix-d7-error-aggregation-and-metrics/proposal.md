# Proposal: D7 Error Aggregation & Worktree Metrics

**Change ID:** devrix-d7-error-aggregation-and-metrics
**Demand ID:** DM-20260621-010
**Status:** S7_Archived
**Priority:** P1
**Date:** 2026-06-21

---

## 1. Background

2026-06-21 用户发起"D7 编排层深度 review"指令。review 报告（`openspec/changes/d6-deep-review/d6-review-report.md` 同类产出，未独立建档）从 3 个维度扫描 `internal/layers/orchestration/` 全域：

- **代码质量**：A-（4 IntentKind 独立链清晰，但 15+ 改进点）
- **三流链路（思考/任务/总结）**：A-/B+/B（链路清晰但格式不统一）
- **worktree 调用链路**：B/B+/C（工具结果/子任务上下文/异常事件均有隐患）

review 识别出 **5 个 P0/P1 隐患**，全部集中在 **错误处理 + 资源回收 + 可观测性** 这一类公共问题。本 change 是该 review 的立即修复层（"< 1 周"范围），不涉及中期重构。

**与已有 change 的关系**：
- **不重复** `devrix-error-handling-tier1-tier2`（S7_Archived 2026-06-20）—— 那是 sharederrors 公共域的 SanitizeForUser + *Error 类型合并，跨 D1/D2/D3/D7
- **本 change 聚焦 D7 orchestration 域自身**的 HandleInterrupt + worktree/freefork/scheduler 的错误聚合 + metrics 化
- 互补：上游 sharederrors 已统一 sentinel，下游 D7 域内仍有多处 `slog.Warn + nil` 反模式待修

**与 tech-debt 的关系**：
- `openspec/tech-debt/worktree-v2-deferred.md` 中 TD-WT-01（`AdaptiveThreshold` 接入 RunTurn）仍延期 v3.0
- 本 change 不动 AdaptiveThreshold，只补 metrics 化

## 2. Problem Statement

### Problem 1 (P1 — High): `HandleInterrupt` 错误全部 warn + nil

**位置**：`internal/layers/orchestration/sessionorchestrator/interrupt.go:62-78`

```go
// Step 1: Wave CancelAll
if h.opts.WaveCanceler != nil {
    if err := h.opts.WaveCanceler(sessionID); err != nil {
        slog.Warn("orchestrator: HandleInterrupt wave cancel failed", ...)
        // ↑ 错误被吞
    }
}
// Step 2: D4 delegate cancel — 同模式
// Step 3: Process cancel — 同模式
// 最终 return nil（永远不报错）
```

**根因**：
- 设计哲学"best-effort cancel"——但实现走极端，全部 warn+nil
- 调用方（`Entry.Cancel` → D1 gateway StopProcess）无法区分"成功取消"与"3 步全部失败"
- 出问题后无 signal，只能依赖外部 observability（Jaeger trace 末尾 + slog 搜索）

**影响**：
- 生产环境调试"为什么 /stop 没生效"需要 grep 3 处 slog.Warn + 看 trace
- D1 gateway StopProcess 看到 nil → 不打 error metric → 误判"取消成功"

### Problem 2 (P0 — Critical): Worktree sandbox 退出失败被吞掉

**位置**：`internal/layers/multiagent/provision/freefork/forker.go:84, 116`

```go
// 同步模式（forker.go:84）
_ = f.deps.Sandbox.Exit(ctx, sbPath, false)
// 异步模式（forker.go:116）
go func() {
    _ = f.deps.Sandbox.Exit(bgCtx, sbPath, false)
}()
```

**根因**：
- `Sandbox.Exit` 实际是 `os.RemoveAll(sandboxPath)`（`enforce/sandbox/manager.go:48`）
- 如果 `os.RemoveAll` 失败（权限、路径不存在、磁盘满），**sandbox 目录残留**
- 残留不会被 GC，下次 fork 同 slug 时可能撞已有目录
- 没有 metric 监控目录残留数量

**测试断言不足**（`forker_test.go:241-244`）：
```go
// TestFork_FailureMidBatchRollsBack
for _, e := range entries {
    t.Logf("leftover: %s", e.Name())  // ← t.Logf 而非 t.Errorf
}
```
即使 sandbox 残留 100 个，CI 也通过。**违反 "race + 资源" 守门测试原则**。

### Problem 3 (P1 — High): Forker 并发失败只返回 errs[0]

**位置**：`forker.go:87`

```go
if failed > 0 {
    for _, e := range entries { e.Terminate() }
    for _, sbPath := range spawnedSandboxes {
        _ = f.deps.Sandbox.Exit(...)
    }
    return nil, errs[0]  // ← errs[0] 隐式丢弃 errs[1..n]
}
```

**根因**：
- 并发失败时只报告第一个错误
- 调试时看不到全部并发失败根因（例如 3 个并发 fork 中 1 个 sandbox 失败 + 2 个 factory 失败，只看到第 1 个）

### Problem 4 (P1 — High): `TaskManager.publishCompletion` panic 静默吞

**位置**：`internal/layers/orchestration/workmodel/task_manager.go:218-219`

```go
// 示意
defer func() { _ = recover() }()
// ↑ 比 scheduler.go:390-406 的"defer recover + slog.Error"更差
// 完全无日志、无 metric
```

**根因**：相比 scheduler 的 `defer func() { if r := recover(); r != nil { slog.Error(...); completeTask(...) } }()`，task_manager 走最简实现，**panic 时静默**。

### Problem 5 (P2 — Med): `taskCtx` 脱离 parentCtx 无 leak 检测

**位置**：`wavescheduler/scheduler.go:328`

```go
// Build a per-task context that the scheduler can cancel via CancelWorker.
// Detach cancellation from parentCtx but preserve trace context...
taskCtx, cancel := context.WithCancel(tracer.Detach(parentCtx))
state.graph.SetState(node.ID, StateRunning)
// ...
state.cancels = append(state.cancels, cancel)  // L343 依赖 CancelAll 显式清理
```

**风险**：
- 这是有意设计（保留 trace）
- 但**没有 leak detection**：如果 `CancelAll` 漏调用或 `state.cancels` 漏注册，task 永不结束
- 长期可能积累僵尸 task（持有 sandbox + goroutine）

### Problem 6 (P1 — High): Worker panic 仅 slog.Error，无 metric

**位置**：`wavescheduler/scheduler.go:390-406`

```go
defer func() {
    if r := recover(); r != nil {
        slog.Error("wave: worker panic", "session", sessionID, "task", node.ID, "panic", r)
        s.completeTask(..., Artifact{Error: fmt.Sprintf("worker panic: %v", r), ...})
        // ↑ 无 metric.Inc("worker_panics")
    }
    ...
}()
```

**影响**：D5 observability dashboard 上看不到 worker_panics 趋势，告警无法配。

## 3. Proposed Solution

### 3.1 总体方案

| Phase | 范围 | 工作量 | 风险 |
|-------|------|--------|------|
| **PR-A** | 错误聚合 + Sandbox cleanup hardening | 小 | Low |
| **PR-B** | Worktree 全链路 metrics 化（5+ counter） | 中 | Low |
| **PR-C** | docs + spec sync + t-registry + S6 归档 | 小 | Low |

### 3.2 关键决策

#### Decision 1: HandleInterrupt 错误聚合方式

**选项对比：**

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 仍返回 nil，加 metric 计数 | 最小改动 | 调用方仍无法区分失败 |
| **B. errors.Join 聚合 3 步错误，返回聚合 error（选）** | **调用方可检测；语义清晰** | 调用方需 `errors.Is/As` 适配 |
| C. 返回 struct{ Steps []StepResult } | 信息最丰富 | 签名变更大 |

**选择**: B
**理由**：
- Go 1.20+ `errors.Join` 是标准做法，调用方用 `errors.Is/As` 即可
- 不破坏现有 best-effort 语义（聚合 error 也是 best-effort 的"全部尝试过"）
- D1 gateway StopProcess 看到非 nil err → 打 metric → observability 完整

#### Decision 2: Sandbox Exit 失败处理

**选项对比：**

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 失败返回 err 给上层 | 强制清理 | 调用方多，错误流复杂 |
| **B. 失败 metric.Inc + slog.Warn，保留 _ = 语义（选）** | **不破坏 best-effort；加 observability** | 调用方仍无 error |
| C. 失败时启动后台 retry | 强保证 | 复杂度高 |

**选择**: B
**理由**：
- 短期"立即修复"范围内 B 已足够（resource leak 可被 metric 监控）
- C 是更彻底的修复，但需要后台 cleanup goroutine + 状态机 → 推到未来 change

#### Decision 3: 5 个 metrics 加到 D5 observability

**选项对比：**

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 新建独立 metrics 包 | 解耦清晰 | 新增包，过度设计 |
| **B. 扩展现有 `SchedulerMetrics` + `ValidationMetrics` 结构（选）** | **复用现有 observability 接入点** | 改动 schema |
| C. 走 OpenTelemetry counter | 工业标准 | devrix 当前未全量接入 OTel |

**选择**: B
**理由**：
- devrix 当前 metrics 走 `SchedulerMetrics`（s.go:18）+ `ValidationMetrics`（4-counter pass/fail/timeout/error）
- 在现有结构加字段 + 在 D5 observability 加 span attribute 双写，零额外包
- OTel 接入可在未来 change 统一做（参考 devrix-error-handling-tier1-tier2 的 *_test.go metric 模式）

### 3.3 实施顺序

| PR | 范围 | 风险 | 依赖 | 估算 LoC |
|----|------|------|------|----------|
| **PR-A** | Problem 1+3 错误聚合；Problem 2 sandbox cleanup hardening | Low | — | +180 / -30 |
| **PR-B** | Problem 4+5+6 metrics 化；forker_test.go 硬断言 | Low | — | +200 / -40 |
| **PR-C** | docs/spec sync + t-registry 新增 + S6 归档 | Low | PR-A/B | +120 / 0 |

**总计**：3 PR × 1 周交付；改动 ~500 LoC / 20 文件

## 4. Success Metrics

| 指标 | 当前 | 目标 | 度量方式 |
|------|------|------|----------|
| HandleInterrupt 返回 nil 比例 | 100% 即使 3 步全失败 | 0% 当任一步失败 | unit test + 生产 metric |
| Sandbox Exit 失败可观测性 | 0 个 metric | 1+ counter | grep `sandbox_exit_failed` |
| Worker panic 可观测性 | 0 个 metric | 1+ counter | grep `worker_panics` |
| taskCtx leak 可观测性 | 0 个 metric | 1+ counter | grep `task_ctx_leaked` |
| Forker 并发错误完整度 | 仅 errs[0] | errors.Join 全错误 | unit test `TestFork_AllFailuresJoined` |
| forker_test 硬断言 | 0 处 `t.Errorf` for cleanup | ≥3 处 | grep `t.Errorf.*leftover` |
| P0 T 层测试通过率 | — | 100% | `./scripts/test-unit.sh` + acceptance |

## 5. Implementation Plan

### 5.1 PR-A — Error aggregation + Sandbox cleanup hardening

**核心文件**：

| 文件 | 改动 | LoC 预估 |
|------|------|----------|
| `internal/layers/orchestration/sessionorchestrator/interrupt.go` | 改 `errors.Join` 聚合 3 canceler 错误 | +25 / -10 |
| `internal/layers/orchestration/sessionorchestrator/interrupt_test.go` | 新增 `TestHandleInterrupt_AllStepsFail_JoinsErrors` | +60 |
| `internal/layers/multiagent/provision/freefork/forker.go:84, 116` | sandbox Exit 失败改 metric + slog.Warn | +20 / -5 |
| `internal/layers/multiagent/provision/freefork/forker.go:87` | errs[0] → errors.Join | +15 / -5 |
| `internal/layers/multiagent/provision/freefork/forker_test.go` | 新增 `TestFork_AllFailuresJoined` + `TestFork_SandboxExitFailure_RecordsMetric` | +60 |
| `internal/layers/multiagent/provision/freefork/metrics.go` (新) | `ForkerMetrics{SandboxExitFailed, FactoryFailed}` 结构 | +30 |
| `internal/layers/orchestration/sessionorchestrator/metrics.go` (新) | `InterruptMetrics{WaveCancelFailed, D4CancelFailed, ProcessCancelFailed}` 结构 | +30 |

**PR-A 合计**：7 文件 +210 LoC

### 5.2 PR-B — Worktree full-chain metrics

**核心文件**：

| 文件 | 改动 | LoC 预估 |
|------|------|----------|
| `internal/layers/orchestration/wavescheduler/scheduler.go:390-406` | worker panic → metric.Inc + slog.Error | +15 / -5 |
| `internal/layers/orchestration/wavescheduler/scheduler.go:444-489` | completeTask 检测 task_ctx_leaked | +20 / -5 |
| `internal/layers/orchestration/wavescheduler/scheduler.go:215-218` | Reentry cancel count → metric | +10 / 0 |
| `internal/layers/orchestration/wavescheduler/scheduler.go:232-290` | dispatchLoop wakeup counter | +10 / 0 |
| `internal/layers/orchestration/wavescheduler/metrics.go` (扩展) | `SchedulerMetrics{WorkerPanics, TaskCtxLeaked, WaveReentryCancelled, DispatchLoopWakeups}` | +30 |
| `internal/layers/orchestration/workmodel/task_manager.go:218-219` | panic 改 slog.Error + metric | +10 / -3 |
| `internal/layers/orchestration/workmodel/task_manager_metrics.go` (新) | `TaskManagerMetrics{PublishCompletionPanics}` | +20 |
| `internal/layers/multiagent/execute/worker.go:53, 106` | sandbox Exit 失败 metric 接入 | +15 / -5 |
| `internal/layers/multiagent/provision/freefork/forker_test.go:223-245` | t.Logf → t.Errorf 硬断言 | +30 / -10 |

**PR-B 合计**：9 文件 +170 LoC

### 5.3 PR-C — Docs + spec sync + t-registry + S6 archive

| 文件 | 改动 |
|------|------|
| `openspec/specs/d7-orchestration/spec.md` | 新增 §D7-S6-A11/12/13 三节（错误聚合 + worktree observability） |
| `openspec/specs/d7-orchestration/t-registry.md` | 新增 7 个 P0 T 点（D7-S6-A11-T01..A13-T01） |
| `openspec/specs/d7-orchestration/design.md` | §Worktree observability 章节补充 |
| `openspec/changes/devrix-d7-error-aggregation-and-metrics/` → `openspec/archive/2026-06-21-devrix-d7-error-aggregation-and-metrics/` | S6 归档 |

**PR-C 合计**：4 文件 +120 LoC

## 6. Risks & Mitigations

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| HandleInterrupt 签名变更破坏调用方（`Entry.Cancel`） | Low | High | 单人项目；编译期可见；test 覆盖 |
| `errors.Join` 在 Go < 1.20 不可用 | Low | High | devrix Go 版本 ≥ 1.21（grep `go.mod`） |
| Metrics 结构扩展破坏现有 JSON 序列化 | Low | Low | 内部 struct，外部 API 不暴露 |
| Worker panic metric 在 race 模式下 counter 竞争 | Low | Low | 加 atomic.Int64 或 sync.Mutex（参考 SchedulerMetrics.metricsMu） |
| Sandbox Exit 失败导致后台 goroutine 残留 | Low | Med | PR-A 不解决（推到未来 change），但 metric 暴露风险 |

## 7. Out of Scope

- 不重构 `coordinator/aliases.go` legacy shim（建议独立 change）
- 不重构 `Dispatcher.Dispatch` 孤儿模块（建议独立 change）
- 不动 `summarizeArtifacts` metadata 增强（建议独立 change）
- 不统一三流格式（FastPath vs OrchestratePath）（建议独立 change）
- 不实现 `UncertaintyCoord`（已规划独立 P0）
- 不接入 `AdaptiveThreshold` 到 RunTurn（TD-WT-01 P2 延期 v3.0）
- 不重构 `turn/orchestrator.go` 1349 行超大文件（建议独立 change）
- 不做 OpenTelemetry 接入（devrix 当前 metrics 走自有 schema）
- 不动 wave/scheduler.go 的 `taskCtx` 脱离 parentCtx 设计（有意）
- 不动 `ConflictGuard` TOCTOU 注释自承（已知 + 不在本次 review 修复层）

## 8. References

- 完整 review 报告：见本 change 同 session 输出（`openspec/changes/d6-deep-review/d6-review-report.md` 同类产出）
- `devrix-error-handling-tier1-tier2`（S7_Archived 2026-06-20）—— sharederrors 公共域修复
- `devrix-tool-surface-phase2-full-pr64`（PR #64 merged）—— 12→0 global loop 关闭
- `devrix-context-budget-phase-a-pr128`（PR #128 merged）—— Phase A 5/5 AC
- `openspec/tech-debt/worktree-v2-deferred.md` —— TD-WT-01 P2 延期
- `openspec/specs/d7-orchestration/spec.md` —— D7 域规范
- `openspec/specs/d7-orchestration/design.md` —— D7 域设计
- `openspec/specs/d7-orchestration/t-registry.md` —— D7 域 T 层注册表