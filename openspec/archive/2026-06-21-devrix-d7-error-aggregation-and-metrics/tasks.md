# Tasks: D7 Error Aggregation & Worktree Metrics

**Change ID:** devrix-d7-error-aggregation-and-metrics
**Demand ID:** DM-20260621-010
**Status:** S4_Tasks
**Date:** 2026-06-21

---

## Phase 0: Setup（不在 PR-A 内）

- [x] 创建 change 目录 `openspec/changes/devrix-d7-error-aggregation-and-metrics/`
- [x] `.openspec.yaml` 元数据
- [x] `proposal.md`（S2）
- [x] `design.md`（S3）
- [x] `tasks.md`（S4，本文件）
- [x] `specs/d7-orchestration/spec.md`（Gherkin 验收）
- [x] 分支 `feat/devrix-d7-error-aggregation-and-metrics` 从 master 拉出

---

## PR-A: Error Aggregation + Sandbox Cleanup Hardening

### A1. InterruptMetrics struct + Snapshot

- [ ] **A1.1** 新建 `internal/layers/orchestration/sessionorchestrator/metrics.go`
  - 定义 `InterruptMetrics{WaveCancelFailed, D4CancelFailed, ProcessCancelFailed, HandleCompleted, HandleErrored}` 全部 `atomic.Int64`
  - 方法 `Snapshot() InterruptMetricsSnapshot`
- [ ] **A1.2** 新建 `metrics_test.go`
  - `TestInterruptMetrics_Snapshot_AtomicIncrement` 并发 inc + snapshot
  - `TestInterruptMetrics_NilSafe` nil metrics 不 panic
- [ ] **A1.3** 修改 `interrupt.go:32-37` InterruptOptions 加 `Metrics *InterruptMetrics` 字段
- [ ] **A1.4** 修改 `interrupt.go:48-50` NewInterruptHandler 保存 metrics 引用

### A2. InterruptHandler 重构（errors.Join）

- [ ] **A2.1** 修改 `interrupt.go:57-94` 重构 Handle 方法
  - 3 canceler 错误全部 errors.Join 聚合
  - 每步失败：metric.Inc + slog.Warn + `fmt.Errorf("step: %w", err)` 加入 errs slice
  - Step 4 Sink.Publish 始终执行
  - 返回 `errors.Join(errs...)` 或 nil
- [ ] **A2.2** 修改 `orchestrator.go:412-419` `Entry.Cancel` — 无需改签名（已返回 error）
- [ ] **A2.3** 修改 `bootstrap/wire_coordinator.go` 创建 `InterruptMetrics` 实例并注入
- [ ] **A2.4** 新增 `interrupt_test.go::TestHandleInterrupt_AllStepsFail_JoinsErrors`
  - 注入 3 个失败的 canceler
  - 调用 Handle → 返回 err 非 nil
  - 验证 errors.Is 检测 3 个 wrap 错误
  - 验证 metric.Snapshot.HandleErrored = 1
- [ ] **A2.5** 新增 `interrupt_test.go::TestHandleInterrupt_PartialFailure_ReturnsPartialErr`
  - 注入 1 个失败 + 2 个成功
  - 返回 err 非 nil
  - 验证 errors.Is 检测 1 个 wrap 错误
- [ ] **A2.6** 新增 `interrupt_test.go::TestHandleInterrupt_AllSuccess_ReturnsNil`
  - 3 个成功 → 返回 nil
  - 验证 metric.Snapshot.HandleCompleted = 1
- [ ] **A2.7** 新增 `interrupt_test.go::TestHandleInterrupt_StoppedEventEmitted_EvenOnFailure`
  - 3 步全失败 → Sink 仍收到 "stopped" event

### A3. ForkerMetrics struct

- [ ] **A3.1** 新建 `internal/layers/multiagent/provision/freefork/metrics.go`
  - 定义 `ForkerMetrics{Spawned, SpawnFailed, SandboxEnterFailed, SandboxExitFailed, FactoryCreateFailed, RollbackTriggered}` 全部 `atomic.Int64`
  - 方法 `Snapshot() ForkerMetricsSnapshot`
- [ ] **A3.2** 新建 `metrics_test.go`
  - `TestForkerMetrics_Snapshot_AtomicIncrement`
  - `TestForkerMetrics_NilSafe`
- [ ] **A3.3** 修改 `DefaultForker` 结构加 `metrics *ForkerMetrics` 字段
- [ ] **A3.4** 修改 `NewDefaultForker(deps DefaultForkerDeps) *DefaultForker` 加 metrics 参数（或可选 setter）

### A4. Sandbox Exit 失败处理（forker.go:84, 116）

- [ ] **A4.1** 修改 `forker.go:84` 同步模式 Exit 失败 → metric.SandboxExitFailed.Inc() + slog.Warn
- [ ] **A4.2** 修改 `forker.go:116` 异步模式 Exit 失败 → 同上（goroutine 内）
- [ ] **A4.3** 新增 `forker_test.go::TestFork_SandboxExitFailure_RecordsMetric`
  - mock Sandbox.Exit 返回 err
  - 调用 Fork
  - 验证 metric.Snapshot.SandboxExitFailed = 1
  - 验证 slog.Warn 输出

### A5. Forker errors.Join 聚合（forker.go:87）

- [ ] **A5.1** 修改 `forker.go:72-89` 重构回滚逻辑
  - rollbackErrs slice 收集 sandbox Exit 错误
  - `allErrs := append(errs, rollbackErrs...)`
  - `return nil, errors.Join(allErrs...)`
- [ ] **A5.2** 处理 errs 为空但 rollbackErrs 非空的情况
  - if `len(allErrs) > 0 { return errors.Join(allErrs...) }`
- [ ] **A5.3** 新增 `forker_test.go::TestFork_AllFailuresJoined`
  - mock 3 个并发失败（factory error + sandbox error + ...）
  - 调用 Fork → 返回 err
  - 验证 errors.Is 检测到全部 wrap 错误（至少 3 个）

### A6. forker_test.go 硬断言补充

- [ ] **A6.1** 修改 `forker_test.go:223-245` `TestFork_FailureMidBatchRollsBack`
  - t.Logf 改 t.Errorf
  - 添加对 metric.Snapshot 的硬断言
- [ ] **A6.2** 加 `t.Helper()` + 明确失败信息

### A7. PR-A 验收

- [ ] **A7.1** `go vet ./...`
- [ ] **A7.2** `go test -race ./internal/layers/orchestration/sessionorchestrator/...`
- [ ] **A7.3** `go test -race ./internal/layers/multiagent/provision/freefork/...`
- [ ] **A7.4** `go test -race ./internal/layers/multiagent/execute/...`
- [ ] **A7.5** 提交 commit `feat(d7): interrupt errors.Join + sandbox cleanup hardening (DM-20260621-010 PR-A)`
- [ ] **A7.6** 创建 PR `devrix-d7-error-aggregation-and-metrics PR-A` → 启用 auto-merge
- [ ] **A7.7** PR merge 后 `git pull --rebase origin master` 再开 PR-B

---

## PR-B: Worktree Full-Chain Metrics

### B1. SchedulerMetrics 扩展

- [ ] **B1.1** 修改 `internal/layers/orchestration/wavescheduler/metrics.go`
  - 新增字段 `WorkerPanics, TaskCtxLeaked, WaveReentryCancelled, DispatchLoopWakeups`（int 类型，受 metricsMu 保护）
- [ ] **B1.2** 修改 `SchedulerMetricsSnapshot` JSON struct 加对应 `json:"..."` tag
- [ ] **B1.3** 修改 `incMetric` switch case 加 `worker_panic, task_ctx_leaked, wave_reentry_cancelled, dispatch_wakeup` 4 个 case
- [ ] **B1.4** 修改 `Metrics()` 方法返回 Snapshot（含新字段）

### B2. Worker panic metric 接入（scheduler.go:390-406）

- [ ] **B2.1** 修改 `scheduler.go:390-406` defer recover 内
  - 在 slog.Error 前加 `s.incMetric("worker_panic")`
- [ ] **B2.2** 新增 `scheduler_test.go::TestWorkerPanic_RecordsMetric`
  - mock runner.Run panic
  - 验证 metric.WorkerPanics = 1
  - 验证 slog.Error 输出

### B3. taskCtx leak 检测（scheduler.go:444-489）

- [ ] **B3.1** 修改 `completeTask` 方法
  - 检查 `state.handles[taskID]` 是否存在且 `h.cancel != nil`
  - 如果 `art.ExitCode == 0 && art.Error == ""` → incMetric("task_ctx_leaked") + slog.Warn
- [ ] **B3.2** 新增 `scheduler_test.go::TestTaskCtxLeak_RecordsMetric`
  - 构造正常完成但 cancel 未清理的 path（mock）
  - 验证 metric.TaskCtxLeaked = 1
- [ ] **B3.3** 新增 `scheduler_test.go::TestTaskCtxLeak_FalsePositiveRate`
  - 跑 100 次正常完成
  - 验证误报数 < 5（< 5%）

### B4. Wave reentry cancel metric（scheduler.go:215-218）

- [ ] **B4.1** 修改 `scheduler.go:215-218` hasExisting 分支
  - 在 `cancelWaveLocked(existing)` 后加 `s.incMetric("wave_reentry_cancelled")`
- [ ] **B4.2** 新增 `scheduler_test.go::TestWaveReentry_RecordsMetric`
  - 同一 session 两次 Start
  - 验证 metric.WaveReentryCancelled = 1

### B5. DispatchLoop wakeup counter（scheduler.go:232-290）

- [ ] **B5.1** 修改 `dispatchLoop` 内 select
  - case `<-state.wakeupCh:` 加 `s.incMetric("dispatch_wakeup")`
  - case `<-ticker.C:` 加 `s.incMetric("dispatch_wakeup")`
- [ ] **B5.2** 新增 `scheduler_test.go::TestDispatchLoop_WakeupCounter`
  - 跑 dispatchLoop 100ms
  - 验证 metric.DispatchLoopWakeups ≥ 5（5 × 20ms = 100ms）

### B6. TaskManager.publishCompletion panic 修复（task_manager.go:218-219）

- [ ] **B6.1** 新建 `internal/layers/orchestration/workmodel/task_manager_metrics.go`
  - `TaskManagerMetrics{PublishCompletionPanics atomic.Int64}`
  - `Snapshot()` 方法
- [ ] **B6.2** 修改 `task_manager.go` TaskManager struct 加 `metrics *TaskManagerMetrics`
- [ ] **B6.3** 修改 `publishCompletion` defer recover
  - 注入 metric.Inc + slog.Error
- [ ] **B6.4** 新增 `task_manager_metrics_test.go::TestPublishCompletionPanic_RecordsMetric`

### B7. Worker.go sandbox Exit metric 接入（worker.go:53, 106）

- [ ] **B7.1** 修改 `worker.go:53, 106` `_ = e.sandbox.Exit(...)` 改 metric.Inc + slog.Warn 模式
- [ ] **B7.2** 新增 `worker_test.go::TestSandboxExitFailure_RecordsMetric`

### B8. PR-B 验收

- [ ] **B8.1** `go vet ./...`
- [ ] **B8.2** `go test -race ./internal/layers/orchestration/wavescheduler/...`
- [ ] **B8.3** `go test -race ./internal/layers/orchestration/workmodel/...`
- [ ] **B8.4** `go test -race ./internal/layers/multiagent/...`
- [ ] **B8.5** 提交 commit `feat(d7): worktree full-chain metrics (DM-20260621-010 PR-B)`
- [ ] **B8.6** 创建 PR `devrix-d7-error-aggregation-and-metrics PR-B` → 启用 auto-merge
- [ ] **B8.7** PR merge 后 `git pull --rebase origin master` 再开 PR-C

---

## PR-C: Docs + Spec Sync + t-registry + S6 Archive

### C1. spec.md 更新

- [ ] **C1.1** 修改 `openspec/specs/d7-orchestration/spec.md`
  - 新增 §D7-S6-A11 `HandleInterrupt Error Aggregation`（Gherkin spec）
  - 新增 §D7-S6-A12 `Worktree Observability`
  - 新增 §D7-S6-A13 `Forker Batch Error Surfacing`

### C2. t-registry 更新

- [ ] **C2.1** 修改 `openspec/specs/d7-orchestration/t-registry.md`
  - 新增 D7-S6-A11-T01: HandleInterrupt 全部失败 → errors.Join(error1, error2, error3)
  - 新增 D7-S6-A12-T01: Sandbox Exit 失败 → metric.Inc + slog.Warn
  - 新增 D7-S6-A12-T02: Worker panic → metric.Inc + slog.Error
  - 新增 D7-S6-A12-T03: taskCtx leak → metric.Inc
  - 新增 D7-S6-A13-T01: Forker 3 个并发失败 → errors.Join

### C3. design.md 更新

- [ ] **C3.1** 修改 `openspec/specs/d7-orchestration/design.md`
  - §Worktree observability 章节补充 metrics 接入图

### C4. S5 验收

- [ ] **C4.1** 跑全量 `go test -race ./...` 确保 PR-A + PR-B 改动无回归
- [ ] **C4.2** 验证覆盖率：`./scripts/test-unit.sh` ≥ 80%
- [ ] **C4.3** 跑 `./scripts/verify-archive.sh devrix-d7-error-aggregation-and-metrics`
- [ ] **C4.4** 写 `acceptance-report.md` 总结 7 P0 T 点 PASS 状态

### C5. S6 归档

- [ ] **C5.1** 提交 commit `docs(d7): spec sync + t-registry + acceptance report (DM-20260621-010 PR-C)`
- [ ] **C5.2** 创建 PR `devrix-d7-error-aggregation-and-metrics PR-C (archive)` → 启用 auto-merge
- [ ] **C5.3** PR merge 后执行归档脚本：
  - `mv openspec/changes/devrix-d7-error-aggregation-and-metrics openspec/archive/2026-06-21-devrix-d7-error-aggregation-and-metrics`
- [ ] **C5.4** 更新 `openspec/t-registry.md` 根索引（如果存在 D7-S6-* 条目）
- [ ] **C5.5** 通知用户验收完成

---

## 总览

| Phase | PR | 任务数 | 文件数 | LoC 预估 | 风险 |
|-------|----|----|--------|----------|------|
| PR-A | Error Aggregation + Sandbox Cleanup | ~25 | 7 | +210 / -30 | Low |
| PR-B | Worktree Full-Chain Metrics | ~25 | 9 | +170 / -40 | Low |
| PR-C | Docs + Spec + t-registry + Archive | ~10 | 4 | +120 / 0 | Low |
| **总计** | **3 PR × 1 周** | **~60 任务** | **~20 文件** | **+500 / -70** | **Low** |

**关键测试点**（7 P0 T，跨 3 PR）：
- D7-S6-A11-T01: HandleInterrupt errors.Join（PR-A）
- D7-S6-A12-T01: Sandbox Exit metric（PR-A）
- D7-S6-A12-T02: Worker panic metric（PR-B）
- D7-S6-A12-T03: taskCtx leak metric（PR-B）
- D7-S6-A13-T01: Forker errors.Join（PR-A）
- + 隐式 2 个（metrics snapshot nil-safe / atomic 验证）

**关键交付**：
- 3 PR × 1 周 = **2026-06-28 前可完成**
- ~500 LoC 改动，~20 文件
- 不破坏现有 API（errors.Join 兼容；metrics struct 扩展字段）
- S6 归档后 D7 域 metrics 覆盖率达 100%（worker panic / sandbox exit / taskCtx leak / wave reentry / dispatch wakeup）