# Design: D7 Metrics Naming Alignment & Concurrency Hardening

**Change ID:** devrix-d7-metrics-and-concurrency-hardening
**Demand ID:** DM-20260622-001
**Status:** S3_Design
**Date:** 2026-06-22
**Author:** Deep review session (DM-20260621-009 follow-up)

---

## 1. Architecture Overview

### 1.1 修复点全景图

```
              [D7 orchestrator 域]
                       │
    ┌──────────────────┼──────────────────┐
    │                  │                  │
    ▼                  ▼                  ▼

[A1 metric rename]  [A3 state.cancels]  [A4 AllowAndRegister]
scheduler.go:306,309  scheduler.go:549    scheduler.go:283+352
"dispatch_wakeup"     markWaveDone 后       split Allow + Register
   →                  state.cancels = nil   →  原子 AllowAndRegister
"dispatch_loop_                            （dispatchOne 内合并）
 wakeups"
                     [A5 select-default]
                     command_handler.go:100
                     out channel 满时
                       drop + slog.Warn
                            │
                            ▼
                  [D5 observability]
                  spec/code 对齐
                  5/5 counter 可读

[A2 sandbox_exit_failed 归属]
spec.md D7-S6-A12-T01 OBSOLETE
cross-ref D4 multiagent/execute
```

### 1.2 修复维度

| 维度 | 修复点 | 关键文件 |
|------|--------|----------|
| **可观测性** | metric 命名 spec/code 对齐 + 跨域归属澄清 | `wavescheduler/scheduler.go` + spec.md |
| **资源管理** | state.cancels 无界增长 | `wavescheduler/scheduler.go::markWaveDone` |
| **并发安全** | AllowAndRegister TOCTOU 关闭 | `wavescheduler/scheduler.go::dispatchOne` |
| **可用性** | CommandHandler 阻塞 send 加固 | `sessionorchestrator/command_handler.go:90-101` |

---

## 2. Component Design

### 2.1 修复 A1 — metric 命名 rename（4 处 string literal）

#### 2.1.1 改动清单

```diff
// wavescheduler/scheduler.go:306 (dispatchLoop, wakeupCh case)
- s.incMetric("dispatch_wakeup")
+ s.incMetric("dispatch_loop_wakeups")

// wavescheduler/scheduler.go:309 (dispatchLoop, ticker.C case)
- s.incMetric("dispatch_wakeup")
+ s.incMetric("dispatch_loop_wakeups")

// wavescheduler/scheduler.go:414 (dispatchOne, defer recover)
// Worker panic → spec 名 "worker_panics"
- s.incMetric("worker_panic")
+ s.incMetric("worker_panics")

// wavescheduler/scheduler.go:418 (slog.Error field)
- slog.Error("wave: worker panic", "metric", "worker_panic", ...)
+ slog.Error("wave: worker panic", "metric", "worker_panics", ...)
```

#### 2.1.2 不变量

- `incMetric` 内部 switch case（`wavescheduler/metrics.go`）的 string literal **不动**（已是复数）
- `SchedulerMetricsSnapshot` JSON tag **不动**（已是 `dispatch_loop_wakeups` / `worker_panics`）
- 仅 caller-side string 改动 + 对应测试断言

#### 2.1.3 测试更新

```diff
// wavescheduler/scheduler_metrics_test.go
- if got := s.lastIncMetricKey; got != "dispatch_wakeup" {
+ if got := s.lastIncMetricKey; got != "dispatch_loop_wakeups" {
-     t.Errorf("incMetric key = %q, want %q", got, "dispatch_wakeup")
+     t.Errorf("incMetric key = %q, want %q (spec alignment)", got, "dispatch_loop_wakeups")
  }
```

---

### 2.2 修复 A2 — sandbox_exit_failed 跨域归属澄清（spec only）

#### 2.2.1 spec.md 改动

```diff
# openspec/specs/d7-orchestration/spec.md §D7-S6-A12

- ## Scenario: Sandbox Exit 失败 metric 记录
- - GIVEN Forker.Fork 路径触发 sandbox 清理失败
- - WHEN Sandbox.Exit 返回 err
- - THEN metric.Inc("sandbox_exit_failed") + slog.Warn

+ ## Scenario: sandbox_exit_failed 由 D4 multiagent 提供（D7 不重复声明）
+ - GIVEN Forker.Fork 路径触发 sandbox 清理失败
+ - WHEN 实际触发方在 D4 `multiagent/execute/worker.go::recordSandboxExitFailed`
+ - THEN D7 域 spec 不重复声明 `sandbox_exit_failed` 计数器
+ - AND spec.md §D7-S6-A12-T01 标注 OBSOLETE
+ - AND D5 dashboard `sandbox_exit_failed` 唯一来源 = D4 executor metrics
+ - AND cross-ref D4-S6-A12-Txx
```

#### 2.2.2 t-registry 改动

```diff
# openspec/specs/d7-orchestration/t-registry.md

- | D7-S6-A12-T01 | Sandbox Exit 失败 → metric.Inc("sandbox_exit_failed") + slog.Warn | ... | IMPLEMENTED | P0 |
+ | D7-S6-A12-T01 | [OBSOLETE 2026-06-22, see D4-S6-A12-Txx] Sandbox Exit 失败 metric 由 D4 multiagent/execute 提供，D7 spec 不重复声明 | ... | OBSOLETE | P0 |
```

#### 2.2.3 不变量

- D7 代码侧无任何改动
- D4 `multiagent/execute/worker.go::recordSandboxExitFailed` 已存在，**保持不变**
- D5 dashboard 按 counter 名 `sandbox_exit_failed` 过滤，**唯一来源仍是 D4**

---

### 2.3 修复 A3 — state.cancels bound cleanup

#### 2.3.1 问题分析

`wavescheduler/scheduler.go:343, 365` `state.cancels = append(state.cancels, cancel)` 累积 cancel funcs，
但 `cancelWaveLocked`（`:606-617`）完成后**永不清理**。

长会话多 wave 重入时（如 LLM 长任务反复 trigger Plan Mode → wave 重启）：
- 每个 cancel func 持有 `context.WithCancel` 资源
- `tracer.Detach` 注册项累积
- 无 metric 监控 leak 数

#### 2.3.2 设计方案

**清理时机**：`markWaveDone` 函数（`wavescheduler/scheduler.go:549-560`），而非 `cancelWaveLocked`。

**理由**：
- `cancelWaveLocked` 仅 cancel wave 内 cancel funcs，未必触发 wave 完成
- `markWaveDone` 是 wave 终结点（normal completion OR cancellation OR error）
- 一次清理 vs 多次清理，更与"无界增长"语义对齐

#### 2.3.3 代码改动

```diff
// wavescheduler/scheduler.go:549-560 (markWaveDone)
  func (s *Scheduler) markWaveDone(sessionID string) error {
      s.mu.Lock()
      defer s.mu.Unlock()

      state, ok := s.states[sessionID]
      if !ok {
          return nil
      }
      state.status = WaveDone

+     // A3: 清空 state.cancels 防长会话无界增长
+     // cancel funcs 已在 cancelWaveLocked (或自然退出) 时调用过，
+     // slice 使命终结，此处 nil reset 释放 context.CancelFunc 引用
+     state.cancels = nil
+
+     // 同步清理 state.handles map（之前未清理，PR-B 仅追加）
+     state.handles = make(map[string]*taskHandle)
+
      return nil
  }
```

#### 2.3.4 边界场景

| 场景 | state.cancels | state.handles | 行为 |
|------|---------------|---------------|------|
| wave 正常完成 | 非空 → nil | 非空 → 空 | ✅ 安全清理 |
| wave 被 cancel | 已 cancel 但 slice 非空 | 非空 | ✅ 仍 nil reset |
| wave panic 中断 | 可能部分非空 | 部分 | ✅ best-effort 清空 |
| markWaveDone 重复调用 | 已 nil | 已 空 | ✅ idempotent |

#### 2.3.5 不变量

- wave 内 cancel funcs 仍在 `cancelWaveLocked` 时被调用（slice 持有期间已 cancel 过）
- nil reset 不影响正在运行的 task（每个 task 通过 `handles[id].cancel` 持有自己的 cancel）
- 后续 wave 重入会创建新的 `state.cancels = append(...)`

---

### 2.4 修复 A4 — AllowAndRegister 热路径接入

#### 2.4.1 问题分析

`wavescheduler/conflict.go:74` 提供原子 `AllowAndRegister(candidate, slotID, running)`：
```go
func (g *ConflictGuard) AllowAndRegister(candidate TaskNode, slotID SlotID, running []RunningTask) bool {
    g.mu.Lock()
    defer g.mu.Unlock()
    // 内部原子完成 Allow + Register
}
```

4 个 P0 T 点 IMPLEMENTED（`conflict_test.go:91-141`），**但 hot path 未使用**。

当前 `wavescheduler/scheduler.go:283 + :352` 仍是 split：
```go
// dispatchLoop (line 283)
if !s.guard.Allow(node, s.guard.Running()) {
    continue  // ← race window starts here
}
// ... 中间数十行 ...
// dispatchOne (line 352) — race window ends here
s.guard.Register(RunningTask{Node: node, SlotID: slotID})
```

**窗口**：在 Allow 通过到 Register 完成之间，另一 goroutine 拿到同一冲突组仍可进入。

#### 2.4.2 设计方案

合并到 `dispatchOne` 内部，单次原子调用：

```diff
// wavescheduler/scheduler.go::dispatchOne

  func (s *Scheduler) dispatchOne(ctx context.Context, node TaskNode) error {
-     // ... prepare taskCtx ...
-     // ... call s.guard.Register(...) at line 352 ...
+     // ... prepare taskCtx ...

+     // A4: 单次原子 AllowAndRegister，关 TOCTOU 窗口
+     slotID := s.pool.AcquireSlot()  // 提前 acquire
+     if !s.guard.AllowAndRegister(node, slotID, s.guard.Running()) {
+         s.pool.ReleaseSlot(slotID)
+         return ErrConflictBlocked
+     }
+     defer s.guard.Unregister(node, slotID)  // 新增 unregister
+
+     // ... dispatch to worker ...
  }
```

**同步修改 dispatchLoop（line 283）**：
```diff
// wavescheduler/scheduler.go::dispatchLoop
-     if !s.guard.Allow(node, s.guard.Running()) {
-         continue
-     }
+     // A4: 移除 dispatchLoop 内的 Allow 检查
+     // 由 dispatchOne 内部 AllowAndRegister 单点原子把关
```

#### 2.4.3 不变量

- `dispatchOne` 失败时（conflict blocked）立即 `pool.ReleaseSlot`
- `defer s.guard.Unregister(...)` 保证 task 完成或失败时清理
- 4 个 `AllowAndRegister` 测试（D7-S3-A01-F03-T01..T04）保持 PASS
- 新增 IT `TestDispatchLoop_UsesAllowAndRegister_OnHotPath` 验证 hot path 仅用原子方法

#### 2.4.4 性能影响

- `AllowAndRegister` 内部仍是单 mutex.Lock（与原 split 两次 mutex 操作相比，**更优**）
- 提前 `pool.AcquireSlot` 改动极小
- `go test -race` 在 100 次并发下应无 race detector 报警

---

### 2.5 修复 A5 — command_handler select-default

#### 2.5.1 问题分析

`sessionorchestrator/command_handler.go:90-101`：
```go
out <- &contracts.EngineEvent{...}   // buffered chan size 32, no select-default
```

**风险**：consumer 持续慢 / 死锁时永久阻塞。

#### 2.5.2 设计方案

```diff
// sessionorchestrator/command_handler.go:90-101

  func (h *CommandHandler) emit(event *contracts.EngineEvent) {
-     h.out <- event
+     // A5: 加 select-default 防 consumer 异常时阻塞
+     select {
+     case h.out <- event:
+         // ok
+     default:
+         slog.Warn("command_handler: out channel full, drop event",
+             "type", event.Type,
+             "session", event.SessionID,
+             "channel_size", cap(h.out),
+         )
+     }
  }
```

#### 2.5.3 不变量

- 正常情况（channel 有空位）行为不变（select 第一个 case 立即命中）
- 异常情况（channel 满）丢弃事件 + slog.Warn，不阻塞 CommandHandler
- 4 个原有 path（`/help` / `/stop` / `/plan` / `/task`）行为兼容

#### 2.5.4 决策点

**为什么丢弃而非阻塞**：
- CommandHandler 是 best-effort UI 反馈（CLI/IM 显示用）
- 阻塞会 hang session，用户无法重连
- 丢失单个事件可接受（有 D5 兜底日志）

---

## 3. Test Design

### 3.1 新增单元测试（6 个 P0 T 点对应）

#### 3.1.1 D7-S6-A14-T01: dispatch_loop_wakeups rename

```go
// wavescheduler/scheduler_metrics_test.go
func TestDispatchLoop_WakeupCounter_AlignsSpec(t *testing.T) {
    // 验证 dispatchLoop 调用 incMetric 时使用 spec 复数名
    spy := &incMetricSpy{}
    s := newTestSchedulerWithSpy(spy)

    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()
    s.dispatchLoop(ctx)

    if spy.countOf("dispatch_loop_wakeups") == 0 {
        t.Errorf("incMetric 'dispatch_loop_wakeups' not called (spec alignment broken)")
    }
    if spy.countOf("dispatch_wakeup") > 0 {
        t.Errorf("incMetric 'dispatch_wakeup' called %d times (stale singular)",
            spy.countOf("dispatch_wakeup"))
    }
}
```

#### 3.1.2 D7-S6-A14-T02: worker_panics rename

```go
func TestWorkerPanic_AlignsSpec(t *testing.T) {
    spy := &incMetricSpy{}
    s := newTestSchedulerWithSpy(spy)

    // mock runner.Run panic
    node := nodeWithPanicRunner()
    s.dispatchOne(context.Background(), node)

    if spy.countOf("worker_panics") != 1 {
        t.Errorf("incMetric 'worker_panics' called %d times, want 1",
            spy.countOf("worker_panics"))
    }
    if spy.countOf("worker_panic") > 0 {
        t.Errorf("incMetric 'worker_panic' called %d times (stale singular)",
            spy.countOf("worker_panic"))
    }
}
```

#### 3.1.3 D7-S6-A14-T03: sandbox_exit_failed 跨域归属（spec only）

**测试策略**：无代码改动，仅 spec.md + t-registry.md 标注验证。

```bash
# 自动化检查
grep -A 2 "D7-S6-A12-T01" openspec/specs/d7-orchestration/t-registry.md | grep -q "OBSOLETE"
```

#### 3.1.4 D7-S6-A14-T04: state.cancels bound

```go
func TestStateCancels_NilAfterWaveDone(t *testing.T) {
    s := newTestScheduler()
    s.Start(ctx, graphWith3Nodes)
    s.WaitForCompletion(ctx)

    if len(s.state.cancels) != 0 {
        t.Errorf("state.cancels len = %d after wave done, want 0 (leak)",
            len(s.state.cancels))
    }
    if len(s.state.handles) != 0 {
        t.Errorf("state.handles len = %d after wave done, want 0",
            len(s.state.handles))
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

#### 3.1.5 D7-S6-A14-T05: AllowAndRegister hot path

```go
func TestDispatchLoop_UsesAllowAndRegister_OnHotPath(t *testing.T) {
    spy := &spyConflictGuard{}
    s := newTestSchedulerWithGuard(spy)

    // 5 ready nodes → 并发 dispatch
    s.Start(ctx, graphWith5ReadyNodes)
    s.WaitForCompletion(ctx)

    if spy.allowAndRegisterCount != 5 {
        t.Errorf("AllowAndRegister called %d times, want 5 (hot path)",
            spy.allowAndRegisterCount)
    }
    if spy.allowCount > 0 || spy.registerCount > 0 {
        t.Errorf("split Allow/Register detected: Allow=%d Register=%d, want 0",
            spy.allowCount, spy.registerCount)
    }
}

// race detector 验证
func TestDispatchLoop_NoRace_Concurrent(t *testing.T) {
    if testing.Short() { t.Skip() }
    for i := 0; i < 100; i++ {
        s := newTestScheduler()
        go s.Start(ctx, graphWith10ReadyNodes)
        s.WaitForCompletion(ctx)
    }
    // 跑：go test -race
}
```

#### 3.1.6 D7-S6-A14-T06: command_handler select-default

```go
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

### 3.2 回归测试保证

- `conflict_test.go` 4 个 `AllowAndRegister` 测试保持 PASS（D7-S3-A01-F03-T01..T04）
- `scheduler_test.go` 既有 `TestCancelWorker` / `TestCancelAll` 等 5 个测试保持 PASS
- `command_handler_test.go` 既有 4 个 path 测试（`/help` / `/stop` / `/plan` / `/task`）保持 PASS
- `scheduler_metrics_test.go` 既有 5 个 counter 测试（D7-S6-A12-T01..T05）保持 PASS

### 3.3 覆盖率目标

- `wavescheduler/`: ≥ 80%（既有覆盖率高）
- `sessionorchestrator/`: ≥ 80%
- 整体: ≥ 80%（devrix 最低门槛）

---

## 4. Risk Assessment

### 4.1 回归风险

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| metric rename 破坏 D5 dashboard 过滤 | Med | Med | PR description 显式列变更；D5 团队同步 |
| AllowAndRegister 改造引入死锁 | Low | High | 4 个 P0 T 点已覆盖；新增 IT 100 次并发 |
| state.cancels nil 时机错误导致 ctx 泄漏 | Low | High | 新 unit test 验证 wave 完成 → slice 空 |
| CommandHandler select-default 丢关键事件 | Low | High | 4 path 行为验证 + slog.Warn 留痕 |
| spec.md 跨域 reference 解析失败 | Low | Low | D4 cross-ref + link |

### 4.2 回滚方案

所有改动都是局部增量，可通过 git revert 安全回滚：

```bash
git revert <PR-A merge commit>
```

无 schema 变更，无 DB migration，无配置文件破坏性改动。

### 4.3 性能影响

| 修复 | 性能影响 | 度量 |
|------|---------|------|
| A1 metric rename | 0（纯 string literal） | 无变化 |
| A2 spec only | 0 | 无 |
| A3 state.cancels nil | +~50ns/wave（slice nil reset） | 忽略 |
| A4 AllowAndRegister 原子 | **改善**（原 split 两次 mutex → 一次） | 微优 |
| A5 select-default | -~20ns/event（多一次 select） | 忽略 |

**整体性能**：零回退，部分微优。

---

## 5. Cross-Domain Impact

| 域 | 影响 | 详情 |
|----|------|------|
| **D5 Observability** | 直接 | dashboard filter 名变更；D5 team 同步 |
| **D4 Multi-Agent** | spec 引用 | spec.md cross-ref D4-S6-A12-Txx |
| **D2 Context Engine** | 无 | 不涉及 |
| **D1 Communication** | 间接 | CommandHandler /stop 行为兼容 |
| **D3 LLM Gateway** | 无 | 不涉及 |

---

## 6. Decision Log

### Decision 1: metric 命名修齐 spec（不改 spec 跟代码）

- **Why**: devrix 流程明确 spec 是 "Canonical — source of truth"
- **How**: rename 4 处 string literal + 4 处测试断言
- **Trade-off**: 改动 4-5 处，但保持 spec 权威性

### Decision 2: sandbox_exit_failed 跨域归属 D4

- **Why**: D7 不拥有 D4（spec.md §域边界），counter 由触发源拥有最自然
- **How**: spec.md D7-S6-A12-T01 改 OBSOLETE + cross-ref D4-S6-A12-Txx
- **Trade-off**: 跨域 reference 增加 spec 复杂度，但避免重复 emit

### Decision 3: state.cancels nil 在 markWaveDone

- **Why**: 与 wave 生命周期对齐（一次清理 vs 多次）
- **How**: markWaveDone 函数末尾追加 `state.cancels = nil`
- **Trade-off**: 略多于 cancelWaveLocked 时机，但更安全（任何 wave 终结路径都触发）

### Decision 4: AllowAndRegister 在 dispatchOne 内合并

- **Why**: dispatchOne 是单任务完整生命周期，原子操作语义清晰
- **How**: dispatchLoop 移除 Allow 检查；dispatchOne 内部单次 AllowAndRegister
- **Trade-off**: dispatchOne 函数体略增，但消除了 TOCTOU race window

### Decision 5: CommandHandler 阻塞 send 用 select-default（丢而非阻塞）

- **Why**: CommandHandler 是 best-effort UI 反馈，阻塞 hang session 不可接受
- **How**: select { case out <- event: default: slog.Warn }
- **Trade-off**: 极端情况丢事件，但有 slog.Warn 留痕 + D5 兜底

---

## 7. Grill Review（自检）

| 检查项 | 结论 |
|--------|------|
| 层归属正确 | ✅ 全部在 D7 wavescheduler / sessionorchestrator |
| 接口方向正确 | ✅ 不引入新接口，纯内部改动 |
| 不重复造轮子 | ✅ 复用既有 incMetric / AllowAndRegister / InterruptMetrics 模式 |
| 跨层依赖最小 | ✅ 仅 spec 跨域 ref（D4） |
| 需求可追溯 | ✅ DM-20260622-001 → proposal.md → design.md → spec_delta.md |
| 验收标准覆盖 | ✅ 6 个 P0 T 点对应 6 个 Scenario |
| Out of Scope 明确 | ✅ 8 项明确不做 |
| DM ID 无冲突 | ✅ 2026-06-22 系列无重名 |
| Gherkin 格式正确 | ✅ GIVEN/WHEN/THEN 完整 |
| Happy/Sad path 覆盖 | ✅ Happy: 正常 dispatch + 正常 emit；Sad: channel 满 + panic + conflict |
| 并发场景覆盖 | ✅ T05 100 次并发 + race detector |
| 错误路径覆盖 | ✅ CancelAll / panic / conflict blocked |
| T 层映射完整 | ✅ 6 个 T 点全映射 |
| 回归风险已评估 | ✅ §4.1 表 |
| 回滚方案可行 | ✅ git revert |
| 性能影响已评估 | ✅ §4.3 表 |

**Grill Review 结论**：**Approved**（无阻塞问题）

---

## 8. S3-Gate 自审

| 检查项（review-design.md §2） | 结论 |
|------------------------------|------|
| 层归属和接口方向正确 | ✅ §1 + §5 |
| 不重复现有能力 | ✅ §2 全部复用既有 |
| demand → proposal → design → specs 追溯链完整 | ✅ DM-20260622-001 → proposal.md → design.md → spec_delta.md + t-registry_delta.md |
| 所有 P0 验收标准有对应 Scenario | ✅ 6 P0 T 点对应 6 Scenario |
| Happy path 和 sad path 均有 Scenario | ✅ spec_delta.md §ADDED D7-S6-A14 4 Scenario |
| 回归风险已评估 | ✅ §4.1 |
| Grill Review 结论已记录 | ✅ §7 |
| Review 结论明确 | ✅ §7 Approved |

**S3-Gate 状态**：**Approved with Suggestions**

**建议（非强制）**：
1. `state.handles` 同步清理逻辑可在 follow-up PR 加 metric（`state_handles_cumulative`）便于 D5 监控
2. CommandHandler select-default 可考虑加 metric `command_events_dropped` 计数（PR-A 不做，留 follow-up）

---

## 9. Out of Scope（重申）

- `coordinator/aliases.go` 退役（130 行 shim + 12 bootstrap import）—— 独立 change
- `turn/orchestrator.go` 1349 行拆分 —— 独立 change
- UncertaintyCoord + AdaptiveThreshold + VERDICT 协议 —— 独立 change
- 三流格式统一 —— spec.md:210 defer
- DispatchLoop 唤醒语义切分（tick vs slot-release）—— P3 优化，独立 change
- worktree_slug 与 Worktree 命名差异 —— 已确认是 stable API contract

---

## 10. References

- 上次深度 review：`~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-deep-review-2026-06-21.md`
- 本次深度 review 综合报告（同 session 输出）
- 上次 change S7 归档：`~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-error-aggregation-s7-archived.md`
- 确定性架构 P0 遗留：`~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-certainty-architecture.md`
- devrix 流程规范：`openspec/specs/project/master.md`
- devrix 设计评审规范：`openspec/specs/project/review-design.md`
- devrix D7 域规范：`openspec/specs/d7-orchestration/spec.md`