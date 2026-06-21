# Delta T-Registry: D7 Metrics Naming Alignment & Concurrency Hardening

**Change ID:** `devrix-d7-metrics-and-concurrency-hardening`
**Affects:** `openspec/specs/d7-orchestration/t-registry.md`
**Demand ID:** DM-20260622-001
**Date:** 2026-06-22

---

## MODIFIED

### D7-S6-A12 Worktree Observability（T01..T05 命名与状态变更）

> **改动**：
> - T01 改为 OBSOLETE（counter 跨域归属 D4）
> - T02/T05 metric 名由单数改为复数（worker_panic → worker_panics）
> - 新增 D7-S6-A12-T05 显式提到 dispatch_loop_wakeups（之前隐含在 B5.1）

| T ID | 改动 | 改前 / 改后 |
|------|------|------------|
| **D7-S6-A12-T01** | status 改 OBSOLETE + 跨域 ref | IMPLEMENTED → **OBSOLETE**（"out of D7 scope, see D4-S6-A12-Txx"） |
| **D7-S6-A12-T02** | metric 名复数化 | "worker_panic" → **"worker_panics"** |
| **D7-S6-A12-T05** | metric 名复数化 | 隐含 "dispatch_wakeup" → **"dispatch_loop_wakeups"**（与 D7-S6-A14-T01 对齐） |

**改后完整条目（嵌入 spec_delta.md）**：

```markdown
| D7-S6-A12-T01 | Sandbox Exit 失败 → metric.Inc("sandbox_exit_failed") + slog.Warn | D7-S6-A12 | `multiagent/execute/worker_test.go::TestSandboxExitFailure_RecordsMetric` | **OBSOLETE** [2026-06-22, see D4-S6-A12-Txx for actual owner] | P0 |

| D7-S6-A12-T02 | Worker panic → metric.Inc("worker_panics") + slog.Error | D7-S6-A12 | `wavescheduler/scheduler_test.go::TestWorkerPanic_RecordsMetric` | IMPLEMENTED | P0 |

| D7-S6-A12-T05 | dispatchLoop wakeup → metric.Inc("dispatch_loop_wakeups")（tick + slot-release） | D7-S6-A12 | `wavescheduler/scheduler_test.go::TestDispatchLoop_WakeupCounter` | IMPLEMENTED | P0 |
```

---

## ADDED

### D7-S6-A14 Metrics Naming Alignment & Concurrency Hardening（6 个新 P0 T 点）

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| **D7-S6-A14-T01** | dispatch_wakeup → dispatch_loop_wakeups rename（spec/code 对齐） | D7-S6-A14 | `wavescheduler/scheduler_metrics_test.go::TestDispatchLoop_WakeupCounter_AlignsSpec` | **PLANNED → IMPLEMENTED**（PR-A 后） | P0 |
| **D7-S6-A14-T02** | worker_panic → worker_panics rename（spec/code 对齐） | D7-S6-A14 | `wavescheduler/scheduler_metrics_test.go::TestWorkerPanic_AlignsSpec` | **PLANNED → IMPLEMENTED**（PR-A 后） | P0 |
| **D7-S6-A14-T03** | sandbox_exit_failed 跨域归属澄清：D7 spec 标注 OBSOLETE，cross-ref D4 | D7-S6-A14 | `openspec/specs/d7-orchestration/spec.md::D7-S6-A12-T01` 标注 | **PLANNED → IMPLEMENTED**（PR-A 后） | P0 |
| **D7-S6-A14-T04** | state.cancels wave 完成时清空（防长会话 leak） | D7-S6-A14 | `wavescheduler/scheduler_test.go::TestStateCancels_NilAfterWaveDone` | **PLANNED → IMPLEMENTED**（PR-A 后） | P0 |
| **D7-S6-A14-T05** | AllowAndRegister 热路径接入（hot path 改用原子调用，关 TOCTOU 窗口） | D7-S6-A14 | `wavescheduler/scheduler_orch_test.go::TestDispatchLoop_UsesAllowAndRegister_OnHotPath` | **PLANNED → IMPLEMENTED**（PR-A 后） | P0 |
| **D7-S6-A14-T06** | command_handler out send select-default 加固（防阻塞） | D7-S6-A14 | `sessionorchestrator/command_handler_test.go::TestCommandHandler_OutChannelFull_DropsEvent` | **PLANNED → IMPLEMENTED**（PR-A 后） | P0 |

---

## 测试位置详细定义

### D7-S6-A14-T01: dispatch_loop_wakeups rename

```go
// wavescheduler/scheduler_metrics_test.go
func TestDispatchLoop_WakeupCounter_AlignsSpec(t *testing.T) {
    s := newTestSchedulerWithMetrics()
    s.dispatchLoop(ctx, 50*time.Millisecond) // 跑 50ms

    snap := s.metrics.Snapshot()
    if snap.DispatchLoopWakeups == 0 {
        t.Fatal("expected DispatchLoopWakeups counter non-zero after 50ms dispatchLoop")
    }

    // 验证 incMetric 调用使用 spec 复数名
    if s.lastIncMetricKey != "dispatch_loop_wakeups" {
        t.Errorf("incMetric key = %q, want %q (spec alignment)",
            s.lastIncMetricKey, "dispatch_loop_wakeups")
    }
}
```

### D7-S6-A14-T02: worker_panics rename

```go
// wavescheduler/scheduler_metrics_test.go
func TestWorkerPanic_AlignsSpec(t *testing.T) {
    s := newTestSchedulerWithMetrics()
    // mock runner.Run panic
    s.dispatchOne(ctx, nodeWithPanicRunner)

    snap := s.metrics.Snapshot()
    if snap.WorkerPanics != 1 {
        t.Errorf("WorkerPanics = %d, want 1", snap.WorkerPanics)
    }

    // 验证 slog tag 也是复数（不是 worker_panic）
    if s.lastSlogMetricTag != "worker_panics" {
        t.Errorf("slog metric tag = %q, want %q (spec alignment)",
            s.lastSlogMetricTag, "worker_panics")
    }
}
```

### D7-S6-A14-T03: sandbox_exit_failed 跨域归属

```markdown
// openspec/specs/d7-orchestration/spec.md §D7-S6-A12

> D7-S6-A12-T01 [OBSOLETE, 2026-06-22]
> Sandbox Exit 失败 metric 由 D4 multiagent/execute/worker.go::recordSandboxExitFailed 提供
> （见 D4-S6-A12-Txx），D7 wavescheduler 不重复声明该 counter。
> D5 dashboard `sandbox_exit_failed` 唯一来源 = D4 executor metrics。
```

### D7-S6-A14-T04: state.cancels bound

```go
// wavescheduler/scheduler_test.go
func TestStateCancels_NilAfterWaveDone(t *testing.T) {
    s := newTestScheduler()
    s.Start(ctx, graphWith3Nodes)
    s.WaitForCompletion(ctx)

    if len(s.state.cancels) != 0 {
        t.Errorf("state.cancels len = %d after wave done, want 0 (leak)",
            len(s.state.cancels))
    }
}

func TestStateCancels_NoLeakAcrossWaves(t *testing.T) {
    s := newTestScheduler()
    for i := 0; i < 5; i++ {
        s.Start(ctx, graphWith1Node)
        s.WaitForCompletion(ctx)
    }
    if len(s.state.cancels) != 0 {
        t.Errorf("state.cancels len = %d after 5 waves, want 0",
            len(s.state.cancels))
    }
}
```

### D7-S6-A14-T05: AllowAndRegister 热路径

```go
// wavescheduler/scheduler_orch_test.go
func TestDispatchLoop_UsesAllowAndRegister_OnHotPath(t *testing.T) {
    spy := &spyConflictGuard{}
    s := newTestSchedulerWithGuard(spy)
    s.Start(ctx, graphWith5ReadyNodes)  // 5 ready → 并发 dispatch

    // 验证 hot path 只调用 AllowAndRegister 原子方法
    if spy.allowAndRegisterCount != 5 {
        t.Errorf("AllowAndRegister called %d times, want 5 (hot path)",
            spy.allowAndRegisterCount)
    }
    // 验证未走 split Allow+Register 路径
    if spy.allowCount > 0 || spy.registerCount > 0 {
        t.Errorf("split Allow/Register detected: Allow=%d Register=%d, want 0",
            spy.allowCount, spy.registerCount)
    }
}
```

### D7-S6-A14-T06: command_handler select-default

```go
// sessionorchestrator/command_handler_test.go
func TestCommandHandler_OutChannelFull_DropsEvent(t *testing.T) {
    out := make(chan *contracts.EngineEvent, 32)
    // pre-fill channel
    for i := 0; i < 32; i++ {
        out <- &contracts.EngineEvent{Type: "noop"}
    }

    h := &CommandHandler{out: out}
    done := make(chan struct{})
    go func() {
        h.Emit(&contracts.EngineEvent{Type: "stopped", SessionID: "s1"})
        close(done)
    }()

    select {
    case <-done:
        // ok
    case <-time.After(100 * time.Millisecond):
        t.Fatal("Emit blocked >100ms when out channel full (no select-default)")
    }
}
```

---

## 数字汇总

| 项 | 改前 | 改后 | Δ |
|----|------|------|---|
| T 点总数 | 123 | 129（+6 新增） | +6 |
| P0 T 点 | 90 | 96 | +6 |
| IMPLEMENTED T 点 | 123 | 129 | +6 |
| OBSOLETE T 点 | 0 | 1（D7-S6-A12-T01） | +1 |

**实施后统计**：
- D7 T 点：123 → 129（+6 新增）
- D7 P0 T 点：90 → 96（+6 新增）
- 跨域 ref：新增 D4 引用 1 条

---

## 与既有 change 的关联

| Change ID | 关联点 |
|-----------|--------|
| devrix-d7-error-aggregation-and-metrics (S7_Archived) | 本 change 修正其 PR-B 落地的 spec/code 漂移 |
| devrix-d6-evolution-review-fixes (S7_Archived) | 同期姊妹 change，姊妹 PR-B 涉及 orch_* → guard_* 6 metric rename 模式可参考 |
| devrix-d7-certainty-architecture (PLANNED) | 未来 change；本 change 闭合 PR-B 漂移后，deterministic 架构立项的物理依赖更轻 |

---

## 完成 Checklist

- [ ] 6 个新 P0 T 点（PLANNED）注册到 t-registry.md D7-S6-A14 章节
- [ ] D7-S6-A12-T01 status 改 OBSOLETE + cross-ref
- [ ] D7-S6-A12-T02/T05 metric 名复数化
- [ ] PR-A merge 后 6 个 T 点 status 改 IMPLEMENTED
- [ ] 跑 `./scripts/test-acceptance.sh --change devrix-d7-metrics-and-concurrency-hardening` 验证 6/6 PASS