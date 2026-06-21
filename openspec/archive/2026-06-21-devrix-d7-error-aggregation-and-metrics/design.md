# Design: D7 Error Aggregation & Worktree Metrics

**Change ID:** devrix-d7-error-aggregation-and-metrics
**Demand ID:** DM-20260621-010
**Status:** S3_Design
**Date:** 2026-06-21

---

## 1. Architecture Overview

```
[3 canceler sources]                [3 panic recovery sites]
WaveCanceler ──┐                    Worker panic  ──┐
D4Cancel ──────┼──► InterruptHandler Worker panic ──┼──► metrics
ProcessCancel ─┘    (errors.Join)   TaskMgr panic ──┘    (atomic counter)
       │
       ▼
[Entry.Cancel returns *errors.JoinError]
       │
       ▼
[D1 gateway StopProcess → non-nil err → metric → observability]

[Sandbox Exit failures]
       │
       ▼
[FreeForkerMetrics.SandboxExitFailed.Inc() + slog.Warn]

[Worker panics]
       │
       ▼
[SchedulerMetrics.WorkerPanics.Inc() + slog.Error]

[taskCtx leaks]
       │
       ▼
[completeTask 检测 cancel != nil && ExitCode == 0 → TaskCtxLeaked.Inc()]
```

## 2. Component Design

### 2.1 InterruptMetrics + errors.Join 聚合

```go
// internal/layers/orchestration/sessionorchestrator/metrics.go
package sessionorchestrator

import "sync/atomic"

// InterruptMetrics counts cancel step failures. Designed for
// observability — does NOT block caller (best-effort cancel).
//
// Threading: counters are atomic; safe for concurrent Handle calls.
type InterruptMetrics struct {
    WaveCancelFailed     atomic.Int64
    D4CancelFailed       atomic.Int64
    ProcessCancelFailed  atomic.Int64
    HandleCompleted      atomic.Int64
    HandleErrored        atomic.Int64  // ≥1 step failed
}

// Snapshot returns a consistent point-in-time view.
func (m *InterruptMetrics) Snapshot() InterruptMetricsSnapshot {
    if m == nil { return InterruptMetricsSnapshot{} }
    return InterruptMetricsSnapshot{
        WaveCancelFailed:    m.WaveCancelFailed.Load(),
        D4CancelFailed:      m.D4CancelFailed.Load(),
        ProcessCancelFailed: m.ProcessCancelFailed.Load(),
        HandleCompleted:     m.HandleCompleted.Load(),
        HandleErrored:       m.HandleCancelErrored.Load(),
    }
}

type InterruptMetricsSnapshot struct {
    WaveCancelFailed    int64 `json:"wave_cancel_failed"`
    D4CancelFailed      int64 `json:"d4_cancel_failed"`
    ProcessCancelFailed int64 `json:"process_cancel_failed"`
    HandleCompleted     int64 `json:"handle_completed"`
    HandleErrored       int64 `json:"handle_errored"`
}
```

### 2.2 InterruptHandler 重构

```go
// internal/layers/orchestration/sessionorchestrator/interrupt.go
//
// 重构要点：
// 1. errors.Join 聚合 3 canceler 错误
// 2. 每次失败 metric.Inc + slog.Warn
// 3. 返回 *errors.JoinError 或 nil（成功时仍返回 nil）
//
// 不变量：
// - "stopped" event 始终发送（除非 ctx cancel 太早）
// - best-effort 语义保留：即使 3 步全失败，仍尝试 Sink.Publish
// - 调用方可 errors.Is 检测到 joinErr 即知道"取消失败"

func (h *InterruptHandler) Handle(ctx context.Context, sessionID string) error {
    h.mu.Lock()
    defer h.mu.Unlock()

    var errs []error

    // Step 1: Wave CancelAll
    if h.opts.WaveCanceler != nil {
        if err := h.opts.WaveCanceler(sessionID); err != nil {
            if h.metrics != nil { h.metrics.WaveCancelFailed.Add(1) }
            slog.Warn("orchestrator: HandleInterrupt wave cancel failed",
                "sessionID", sessionID, "err", err)
            errs = append(errs, fmt.Errorf("wave cancel: %w", err))
        }
    }

    // Step 2: D4 delegate cancel — same pattern
    if h.opts.DelegateCanceler != nil {
        if err := h.opts.DelegateCanceler(sessionID); err != nil {
            if h.metrics != nil { h.metrics.D4CancelFailed.Add(1) }
            slog.Warn(...)
            errs = append(errs, fmt.Errorf("d4 cancel: %w", err))
        }
    }

    // Step 3: Process cancel — same pattern
    if h.opts.ProcessCanceler != nil {
        if err := h.opts.ProcessCanceler(sessionID); err != nil {
            if h.metrics != nil { h.metrics.ProcessCancelFailed.Add(1) }
            slog.Warn(...)
            errs = append(errs, fmt.Errorf("process cancel: %w", err))
        }
    }

    // Step 4: emit stopped event (始终尝试)
    if h.opts.Sink != nil {
        ev := &contracts.EngineEvent{Type: "stopped", ...}
        h.opts.Sink.Publish(ctx, ev)
    }

    if len(errs) > 0 {
        if h.metrics != nil { h.metrics.HandleErrored.Add(1) }
        return errors.Join(errs...)
    }
    if h.metrics != nil { h.metrics.HandleCompleted.Add(1) }
    return nil
}
```

### 2.3 InterruptHandler 构造扩展

```go
// InterruptOptions 新增 Metrics 字段
type InterruptOptions struct {
    WaveCanceler     func(sessionID string) error
    DelegateCanceler func(sessionID string) error
    ProcessCanceler  func(sessionID string) error
    Sink             EventPublisher
    Metrics          *InterruptMetrics  // 可选；nil → no-op
}

func NewInterruptHandler(orch *SessionOrchestrator, opts InterruptOptions) *InterruptHandler {
    return &InterruptHandler{
        opts:    opts,
        orchestrator: orch,
        metrics: opts.Metrics,
    }
}
```

### 2.4 ForkerMetrics + Sandbox Exit 失败处理

```go
// internal/layers/multiagent/provision/freefork/metrics.go
package freefork

import "sync/atomic"

// ForkerMetrics counts fork-related failures.
type ForkerMetrics struct {
    Spawned              atomic.Int64
    SpawnFailed          atomic.Int64
    SandboxEnterFailed   atomic.Int64
    SandboxExitFailed    atomic.Int64  // ★ 新增
    FactoryCreateFailed  atomic.Int64
    RollbackTriggered    atomic.Int64
}

func (m *ForkerMetrics) Snapshot() ForkerMetricsSnapshot { ... }
```

**修改 forkWorker（forker.go:84, 116）**：

```go
// 旧
_ = f.deps.Sandbox.Exit(ctx, sbPath, false)

// 新
if err := f.deps.Sandbox.Exit(ctx, sbPath, false); err != nil {
    if f.metrics != nil { f.metrics.SandboxExitFailed.Add(1) }
    slog.Warn("freefork: sandbox exit failed",
        "session", sessionID, "path", sbPath, "err", err)
}
```

### 2.5 Forker 并发错误聚合

```go
// 修改 fork.go:72-89（DefaultForker.Fork）

// 旧
if failed > 0 {
    for _, e := range entries { e.Terminate() }
    for _, sbPath := range spawnedSandboxes {
        _ = f.deps.Sandbox.Exit(...)
    }
    return nil, errs[0]  // ← errs[0]
}

// 新
if failed > 0 {
    for _, e := range entries { e.Terminate() }
    for _, sbPath := range spawnedSandboxes {
        if exitErr := f.deps.Sandbox.Exit(...); exitErr != nil {
            // 用 channel 收集，不直接写 errs（避免 race）
            rollbackErrs = append(rollbackErrs, fmt.Errorf("sandbox exit %s: %w", sbPath, exitErr))
            if f.metrics != nil { f.metrics.SandboxExitFailed.Add(1) }
        }
    }
    // 合并原 errs + rollbackErrs（如果 errs 为空但 rollback 全失败也返回）
    allErrs := append(errs, rollbackErrs...)
    if len(allErrs) > 0 {
        return nil, errors.Join(allErrs...)
    }
}
return entries, nil
```

### 2.6 SchedulerMetrics 扩展（wavescheduler）

```go
// internal/layers/orchestration/wavescheduler/metrics.go

type SchedulerMetrics struct {
    Started                int
    Completed              int
    Failed                 int
    Cancelled              int
    PeakRunning            int
    TotalDispatches        int

    // 新增（PR-B）
    WorkerPanics           atomic.Int64
    TaskCtxLeaked          atomic.Int64
    WaveReentryCancelled   atomic.Int64
    DispatchLoopWakeups    atomic.Int64  // ticker + wakeupCh 总数
}

type SchedulerMetricsSnapshot struct {
    Started                int   `json:"started"`
    // ... existing fields
    WorkerPanics           int64 `json:"worker_panics"`
    TaskCtxLeaked          int64 `json:"task_ctx_leaked"`
    WaveReentryCancelled   int64 `json:"wave_reentry_cancelled"`
    DispatchLoopWakeups    int64 `json:"dispatch_loop_wakeups"`
}

func (s *WaveScheduler) Metrics() SchedulerMetricsSnapshot {
    s.metricsMu.Lock()
    defer s.metricsMu.Unlock()
    return SchedulerMetricsSnapshot{
        Started: s.metrics.Started,
        // ...
        WorkerPanics: s.metrics.WorkerPanics.Load(),
        TaskCtxLeaked: s.metrics.TaskCtxLeaked.Load(),
        WaveReentryCancelled: s.metrics.WaveReentryCancelled.Load(),
        DispatchLoopWakeups: s.metrics.DispatchLoopWakeups.Load(),
    }
}
```

**注意**：现有 `SchedulerMetrics` 用 `int`（非 atomic），需要重构为 atomic.Int64 或保留 int + sync.Mutex。**选择保留 int + sync.Mutex**（已有 metricsMu L59），避免大规模重写。

**实现选择**：
```go
// 用 sync.Mutex 保护整个 struct（已有）
func (s *WaveScheduler) incMetric(field string) {
    s.metricsMu.Lock()
    defer s.metricsMu.Unlock()
    switch field {
    case "started": s.metrics.Started++
    case "completed": s.metrics.Completed++
    case "failed": s.metrics.Failed++
    case "cancelled": s.metrics.Cancelled++
    case "dispatch": s.metrics.TotalDispatches++
    case "worker_panic": s.metrics.WorkerPanics++  // 新增
    case "task_ctx_leaked": s.metrics.TaskCtxLeaked++  // 新增
    case "wave_reentry_cancelled": s.metrics.WaveReentryCancelled++  // 新增
    case "dispatch_wakeup": s.metrics.DispatchLoopWakeups++  // 新增
    }
}
```

### 2.7 Worker panic 修复（scheduler.go:390-406）

```go
// 旧
defer func() {
    if r := recover(); r != nil {
        slog.Error("wave: worker panic", ...)
        s.completeTask(..., Artifact{Error: fmt.Sprintf("worker panic: %v", r), ...})
    }
    ...
}()

// 新（保留 slog.Error + 加 metric）
defer func() {
    if r := recover(); r != nil {
        s.incMetric("worker_panic")
        slog.Error("wave: worker panic",
            "session", sessionID, "task", node.ID, "panic", r,
            "worker_id", workerID,
            "metric", "worker_panics")
        s.completeTask(..., Artifact{Error: fmt.Sprintf("worker panic: %v", r), ...})
    }
    ...
}()
```

### 2.8 taskCtx leak 检测（scheduler.go:444-489 completeTask）

```go
// 修改 completeTask 检查 handle 状态
func (s *WaveScheduler) completeTask(sessionID string, state *schedulerWaveState, taskID string, slotID SlotID, art Artifact) {
    // 检测 taskCtx leak：如果 task 正常完成但 cancel 仍非 nil，
    // 说明 caller 没依赖 CancelAll 清理（潜在 leak）
    state.mu.Lock()
    h, exists := state.handles[taskID]
    state.mu.Unlock()
    if exists && h != nil && h.cancel != nil && art.ExitCode == 0 && art.Error == "" {
        s.incMetric("task_ctx_leaked")
        slog.Warn("wave: taskCtx not cleaned up after normal completion",
            "session", sessionID, "task", taskID, "worker_id", h.taskID)
    }
    // ... 现有逻辑
}
```

**注意**：此检测可能误报（task 自然完成但 cancel func 仍在 map 中）。S5 acceptance 需验证误报率 < 5%。

### 2.9 WaveScheduler.Reentry cancel metric

```go
// scheduler.go:215-218
if hasExisting {
    slog.Info("wave: reentry — cancelling prior wave", "session", sessionID)
    cancelled := s.cancelWaveLocked(existing)
    s.incMetric("wave_reentry_cancelled")
    // 取消任务数通过 cancelled 计数；可扩展 metric 字段
}
```

### 2.10 DispatchLoop wakeup counter

```go
// scheduler.go:232-290 dispatchLoop
func (s *WaveScheduler) dispatchLoop(ctx context.Context, sessionID string, state *schedulerWaveState) {
    ticker := time.NewTicker(20 * time.Millisecond)
    defer ticker.Stop()
    // ...
    for {
        // ... 现有逻辑
        select {
        case <-ctx.Done():
            s.cancelWaveLocked(state)
            s.markWaveDone(state)
            return
        case <-state.wakeupCh:
            s.incMetric("dispatch_wakeup")  // 新增
        case <-ticker.C:
            s.incMetric("dispatch_wakeup")  // 新增
        }
    }
}
```

### 2.11 TaskManager.publishCompletion panic 修复

```go
// internal/layers/orchestration/workmodel/task_manager.go:218-219
// 旧
defer func() { _ = recover() }()

// 新
type TaskManagerMetrics struct {
    PublishCompletionPanics atomic.Int64
}

func (tm *TaskManager) publishCompletion(...) {
    defer func() {
        if r := recover(); r != nil {
            if tm.metrics != nil { tm.metrics.PublishCompletionPanics.Add(1) }
            slog.Error("taskmanager: publishCompletion panic",
                "session", sessionID, "item_id", itemID, "panic", r)
        }
    }()
    // ... 现有逻辑
}
```

## 3. Key Interfaces / Types

### 3.1 InterruptMetrics

```go
// internal/layers/orchestration/sessionorchestrator/metrics.go
type InterruptMetrics struct {
    WaveCancelFailed     atomic.Int64
    D4CancelFailed       atomic.Int64
    ProcessCancelFailed  atomic.Int64
    HandleCompleted      atomic.Int64
    HandleErrored        atomic.Int64
}

type InterruptMetricsSnapshot struct {
    WaveCancelFailed    int64
    D4CancelFailed      int64
    ProcessCancelFailed int64
    HandleCompleted     int64
    HandleErrored       int64
}

func (m *InterruptMetrics) Snapshot() InterruptMetricsSnapshot
```

### 3.2 ForkerMetrics

```go
// internal/layers/multiagent/provision/freefork/metrics.go
type ForkerMetrics struct {
    Spawned              atomic.Int64
    SpawnFailed          atomic.Int64
    SandboxEnterFailed   atomic.Int64
    SandboxExitFailed    atomic.Int64
    FactoryCreateFailed  atomic.Int64
    RollbackTriggered    atomic.Int64
}
```

### 3.3 SchedulerMetrics 扩展

```go
// internal/layers/orchestration/wavescheduler/metrics.go
type SchedulerMetrics struct {
    Started                int
    Completed              int
    Failed                 int
    Cancelled              int
    PeakRunning            int
    TotalDispatches        int
    // 新增字段（PR-B）
    WorkerPanics           int
    TaskCtxLeaked          int
    WaveReentryCancelled   int
    DispatchLoopWakeups    int
}

type SchedulerMetricsSnapshot struct { ... }
```

### 3.4 TaskManagerMetrics

```go
// internal/layers/orchestration/workmodel/task_manager_metrics.go
type TaskManagerMetrics struct {
    PublishCompletionPanics atomic.Int64
}
```

### 3.5 errors.Join 使用

```go
// interrupt.go
return errors.Join(errs...)

// fork.go
return nil, errors.Join(allErrs...)

// 调用方（Entry.Cancel）— 不需要改，已返回 error
```

## 4. Data Flow

### 4.1 HandleInterrupt 新流程

```
[/stop or D1 StopProcess]
       │
       ▼
[Entry.Cancel(ctx, sessionID)]
       │
       ▼
[InterruptHandler.Handle]
       │
       ├─ Step 1: WaveCanceler(sessionID)
       │    ├─ nil → continue
       │    └─ err → metric.WaveCancelFailed.Inc() + slog.Warn + errs = append
       │
       ├─ Step 2: DelegateCanceler(sessionID)
       │    ├─ nil → continue
       │    └─ err → metric.D4CancelFailed.Inc() + slog.Warn + errs = append
       │
       ├─ Step 3: ProcessCanceler(sessionID)
       │    ├─ nil → continue
       │    └─ err → metric.ProcessCancelFailed.Inc() + slog.Warn + errs = append
       │
       ├─ Step 4: Sink.Publish "stopped" event (始终尝试)
       │
       ▼
[if errs > 0]
   metric.HandleErrored.Inc()
   return errors.Join(errs...)  ← ★ 新行为
[else]
   metric.HandleCompleted.Inc()
   return nil
       │
       ▼
[Entry.Cancel returns to D1 gateway]
       │
       ├─ nil → D1 StopProcess "ok" metric
       └─ non-nil → D1 StopProcess "fail" metric + slog.Error + observability alert
```

### 4.2 Sandbox Exit 失败 metric 流程

```
[DefaultForker.Fork / Worker.ExecuteSync/Async]
       │
       ├─ sandbox.Enter() → 路径
       ├─ ... 任务执行
       └─ defer sandbox.Exit(ctx, sbPath, false)
              │
              ├─ nil → continue
              └─ err → metric.SandboxExitFailed.Inc() + slog.Warn
                              │
                              ▼
                     [D5 observability dashboard 暴露告警]
                              │
                              ▼
                     [生产监控：sandbox_exit_failed > 10/h → 触发 SRE 介入]
```

### 4.3 taskCtx leak 检测流程

```
[WaveScheduler.dispatchOne]
       │
       ├─ taskCtx, cancel := context.WithCancel(...)
       ├─ state.handles[node.ID] = handle{cancel: cancel}
       ├─ go runner.Run(taskCtx, spec)
       │       │
       │       ▼
       │   [runner 正常完成]
       │       │
       │       ▼
       │   s.completeTask(...)
       │
       ▼
[completeTask]
       │
       ├─ 检查 handle.cancel 是否仍非 nil
       │    │
       │    ├─ cancel != nil && ExitCode == 0 → metric.TaskCtxLeaked.Inc()
       │    │       │
       │    │       ▼
       │    │   [可能误报：但 S5 acceptance 验证 < 5%]
       │    │
       │    └─ 正常清理 → continue
       │
       └─ state.handles[taskID] 清理（已有逻辑）
```

## 5. File Manifest

### 5.1 新增

| 文件 | 用途 | LoC |
|------|------|-----|
| `internal/layers/orchestration/sessionorchestrator/metrics.go` | `InterruptMetrics` + `Snapshot` | +80 |
| `internal/layers/orchestration/sessionorchestrator/metrics_test.go` | atomic snapshot 单元测试 | +60 |
| `internal/layers/multiagent/provision/freefork/metrics.go` | `ForkerMetrics` | +60 |
| `internal/layers/multiagent/provision/freefork/metrics_test.go` | snapshot 单元测试 | +40 |
| `internal/layers/orchestration/wavescheduler/metrics.go` | 扩展 SchedulerMetrics | +40 |
| `internal/layers/orchestration/wavescheduler/metrics_test.go` | 新增字段测试 | +30 |
| `internal/layers/orchestration/workmodel/task_manager_metrics.go` | `TaskManagerMetrics` | +30 |

### 5.2 修改

| 文件 | 改动 |
|------|------|
| `internal/layers/orchestration/sessionorchestrator/interrupt.go` | errors.Join + metric 集成 |
| `internal/layers/orchestration/sessionorchestrator/interrupt_test.go` | 新增 `TestHandleInterrupt_AllStepsFail_JoinsErrors` |
| `internal/layers/orchestration/sessionorchestrator/orchestrator.go:412-419` | `Entry.Cancel` 调用者适配（无需改签名） |
| `internal/layers/multiagent/provision/freefork/forker.go:84, 87, 116` | sandbox Exit 错误处理 + errs[0] → errors.Join |
| `internal/layers/multiagent/provision/freefork/forker_test.go` | 硬断言 + 新测试 |
| `internal/layers/multiagent/provision/freefork/forker.go:NewDefaultForker` | 加 `*ForkerMetrics` 依赖 |
| `internal/layers/orchestration/wavescheduler/scheduler.go:215-218, 232-290, 390-406, 444-489` | 4 处 metric 接入 |
| `internal/layers/orchestration/workmodel/task_manager.go:218-219` | panic recovery 改 slog.Error + metric |
| `internal/layers/multiagent/execute/worker.go:53, 106` | sandbox Exit metric 接入 |
| `internal/layers/multiagent/provision/freefork/forker_test.go:241-244` | t.Logf → t.Errorf 硬断言 |

### 5.3 删除

- 无

## 6. Regression Risk Assessment

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| `errors.Join` 在 Go < 1.20 不可用 | None | — | devrix `go.mod` 已 ≥ 1.21（grep 验证） |
| InterruptOptions 新增字段破坏现有调用 | None | — | 字段为可选；现有 NewInterruptHandler 调用照常工作 |
| ForkerMetrics 接入 NewDefaultForker 改签名 | Low | Med | 同步改所有调用方（bootstrap / 测试） |
| SchedulerMetrics int→atomic 重构破坏现有 test | Low | Low | 保留 int + sync.Mutex 方案（已有 metricsMu） |
| taskCtx leak 误报（task 正常完成但 cancel 仍 map 中） | Med | Low | S5 acceptance 验证 < 5%；可后续增加清理逻辑 |
| D5 observability dashboard 未及时更新 | Low | Low | D5 接入是手动 work（与本 PR 并行跟踪） |
| forker_test 硬断言暴露 latent bug | Med | Med | CI 全绿即安全；bug 修复可在 S4 follow-up |

## 7. Rollback Plan

- **PR-A**：`git revert` 即可；metric struct 可保留（无副作用）；errors.Join 是 string-compatible
- **PR-B**：`git revert` 即可；SchedulerMetrics 字段加回旧字段（不删）
- **PR-C**：`git revert` 即可；spec/t-registry 是文档，无代码副作用

每个 PR 单独 squash + revert 独立。

## 8. Verification

```bash
# PR-A
go test -race ./internal/layers/orchestration/sessionorchestrator/...  # interrupt_test
go test -race ./internal/layers/multiagent/provision/freefork/...      # forker_test
go test -race ./internal/layers/multiagent/execute/...                 # worker_test

# PR-B
go test -race ./internal/layers/orchestration/wavescheduler/...        # scheduler_test (含 metrics)
go test -race ./internal/layers/orchestration/workmodel/...            # task_manager_test

# PR-C
./scripts/verify-archive.sh devrix-d7-error-aggregation-and-metrics
```

**手工验证（用户视角）**：

PR-A 验收：
1. 启动 devrix，对运行中的 session 发送 /stop
2. 模拟 WaveCanceler 注入失败 → 检查返回 err 非 nil + InterruptMetrics.Snapshot.WaveCancelFailed = 1
3. 注入 3 步全失败 → 检查返回 errors.Join(error1, error2, error3) 且 D1 gateway StopProcess 打 fail metric
4. 注入 sandbox Exit 失败 → ForkerMetrics.Snapshot.SandboxExitFailed = 1 + slog.Warn 输出

PR-B 验收：
1. 模拟 worker panic → SchedulerMetrics.Snapshot.WorkerPanics = 1
2. 模拟 taskCtx leak（构造正常完成但 cancel 不清理的 path）→ TaskCtxLeaked = 1
3. Reentry wave → WaveReentryCancelled = 1
4. DispatchLoop 运行 5 秒 → DispatchLoopWakeups 数量符合预期（≥ 250 = 5s / 20ms）

## 9. S3-Gate Self-Check

按 `review-design.md` §2 四个维度：

- [x] **层归属正确**：D7 域内改动（orchestrator / wavescheduler / workmodel / multiagent-freefork）；D5 observability 仅做 metrics 接线，不跨层依赖
- [x] **接口方向正确**：metrics struct 由低层提供，scheduler/orchestrator 调用；D1 gateway 仍只依赖 contracts，不感知 metrics
- [x] **不重复造轮子**：复用 `errors.Join`（Go 1.20+ 标准库）；复用现有 `SchedulerMetrics.metricsMu` 模式；不复造 metric schema
- [x] **跨层依赖最小**：metrics.go 是 orchestrator / wavescheduler / workmodel 包内文件，零跨包依赖
- [x] **设计决策有记录**：Decision 1/2/3 在 proposal.md
- [x] **demand → proposal → design → specs 链路完整**：openspec/changes/devrix-d7-error-aggregation-and-metrics/
- [x] **验收标准覆盖**：7 P0 T 点 + 3 PR，每 PR 至少 2 P0 T
- [x] **Out of Scope 明确**：10 项明确排除项
- [x] **DM ID 无冲突**：DM-20260621-010 是当日第 10 号（与 007/008/009 hotfix 独立）
- [x] **Gherkin 格式正确**：specs/d7-orchestration/spec.md 用 Given/When/Then
- [x] **Happy + sad path 均有 Scenario**：成功取消 / 部分失败 / 全部失败 三场景
- [x] **T 层映射完整**：7 P0 T 点（D7-S6-A11/12/13）映射到 t-registry
- [x] **回归风险已评估**：6 风险项 + 缓解
- [x] **回滚方案可行**：3 PR 独立 revert

**S3-Gate 结论**: Ready for S4 Implementation