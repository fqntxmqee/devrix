# D7 Orchestration Spec Delta

**Change ID:** devrix-d7-error-aggregation-and-metrics
**Date:** 2026-06-21

---

## D7-S6-A11: HandleInterrupt Error Aggregation

### Capability

`InterruptHandler.Handle` aggregates errors from all 3 canceler steps (Wave / D4 / Process) via `errors.Join`. Returns aggregated error if any step fails; returns `nil` only when all 3 succeed.

### Gherkin

```gherkin
Feature: HandleInterrupt error aggregation

  Scenario: All 3 cancelers succeed
    Given InterruptHandler with 3 successful cancelers
    When Handle is called
    Then return value is nil
    And metric "handle_completed" increments by 1
    And "stopped" event is emitted to sink

  Scenario: 1 canceler fails, 2 succeed
    Given InterruptHandler with WaveCanceler failing
    And DelegateCanceler and ProcessCanceler succeeding
    When Handle is called
    Then return value is non-nil errors.JoinError
    And errors.Is(returnedErr, waveErr) is true
    And errors.Is(returnedErr, d4Err) is false
    And metric "wave_cancel_failed" increments by 1
    And metric "handle_errored" increments by 1
    And "stopped" event is still emitted

  Scenario: All 3 cancelers fail
    Given InterruptHandler with 3 failing cancelers
    When Handle is called
    Then return value is non-nil errors.JoinError containing 3 wrapped errors
    And errors.Is(returnedErr, waveErr) is true
    And errors.Is(returnedErr, d4Err) is true
    And errors.Is(returnedErr, processErr) is true
    And all 3 fail counters increment
    And metric "handle_errored" increments by 1
    And "stopped" event is still emitted (best-effort)

  Scenario: Handle is idempotent
    Given InterruptHandler with active cancelers
    When Handle is called twice for same sessionID
    Then second call is no-op
    And no error is returned
    And metric "handle_completed" or "handle_errored" increments once per call
```

### T Layer

- D7-S6-A11-T01: HandleInterrupt 全部失败 → errors.Join(error1, error2, error3)
- D7-S6-A11-T02: HandleInterrupt 部分失败 → errors.Join 包含失败项
- D7-S6-A11-T03: HandleInterrupt 全部成功 → nil + metric.handle_completed
- D7-S6-A11-T04: HandleInterrupt idempotent — 二次调用 no-op

---

## D7-S6-A12: Worktree Observability

### Capability

Worktree / sandbox / worker panic / taskCtx leak 全部走 metric + slog，不静默吞。5 个 counter 暴露给 D5 observability dashboard。

### Gherkin

```gherkin
Feature: Worktree sandbox exit observability

  Scenario: Sandbox Exit fails
    Given Forker with mock Sandbox.Exit returning error
    When Fork is called
    Then metric "sandbox_exit_failed" increments by 1
    And slog.Warn is logged with path and err
    And Forker.Fork returns aggregated error (if other errors present)

  Scenario: Sandbox Exit succeeds
    Given Forker with mock Sandbox.Exit returning nil
    When Fork is called
    Then metric "sandbox_exit_failed" is unchanged
    And no slog.Warn is logged

Feature: Worker panic observability

  Scenario: Worker goroutine panics
    Given WaveScheduler with mock runner.Run panicking
    When dispatchOne spawns worker
    Then worker recovers via defer
    And metric "worker_panics" increments by 1
    And slog.Error is logged with worker_id and panic value
    And Artifact.Error is set to "worker panic: <panic>"

  Scenario: Worker completes normally
    Given WaveScheduler with mock runner.Run returning nil
    When dispatchOne spawns worker
    Then no panic occurs
    And metric "worker_panics" is unchanged

Feature: taskCtx leak detection

  Scenario: taskCtx not cleaned up after normal completion
    Given WaveScheduler with taskCtx cancel not called
    When completeTask is invoked for that task
    And art.ExitCode == 0
    And art.Error == ""
    Then metric "task_ctx_leaked" increments by 1
    And slog.Warn is logged with session_id and task_id

  Scenario: taskCtx cleaned up via cancel
    Given WaveScheduler with taskCtx cancel called via CancelAll
    When completeTask is invoked for that task
    Then metric "task_ctx_leaked" is unchanged
    And no slog.Warn is logged

  Scenario: taskCtx leak detection false positive rate
    Given 100 normal task completions
    When all completeTask invocations finish
    Then metric "task_ctx_leaked" count < 5 (< 5% false positive rate)

Feature: Wave reentry observability

  Scenario: New wave replaces existing wave for same session
    Given WaveScheduler with existing wave for sessionID
    When Start is called again for same sessionID
    Then existing wave is cancelled via cancelWaveLocked
    And metric "wave_reentry_cancelled" increments by 1
    And slog.Info is logged "wave: reentry — cancelling prior wave"

Feature: Dispatch loop wakeup counter

  Scenario: DispatchLoop runs for 100ms
    Given WaveScheduler with active wave
    When dispatchLoop runs for 100ms (with 20ms ticker)
    Then metric "dispatch_loop_wakeups" >= 5 (100ms / 20ms)
```

### T Layer

- D7-S6-A12-T01: Sandbox Exit 失败 → metric.Inc("sandbox_exit_failed") + slog.Warn
- D7-S6-A12-T02: Worker panic → metric.Inc("worker_panics") + slog.Error
- D7-S6-A12-T03: taskCtx leak → metric.Inc("task_ctx_leaked") + slog.Warn
- D7-S6-A12-T04: Wave reentry → metric.Inc("wave_reentry_cancelled")
- D7-S6-A12-T05: DispatchLoop wakeup → metric.Inc("dispatch_loop_wakeups")
- D7-S6-A12-T06: TaskManager publishCompletion panic → metric.Inc + slog.Error
- D7-S6-A12-T07: Worker.go sandbox Exit metric 接入

---

## D7-S6-A13: Forker Batch Error Surfacing

### Capability

`DefaultForker.Fork` aggregates all concurrent failures via `errors.Join` instead of returning only `errs[0]`. Includes both original spawn errors and rollback errors.

### Gherkin

```gherkin
Feature: Forker batch error aggregation

  Scenario: 3 concurrent spawn failures
    Given Forker with 3 fork requests
    And mock Factory returning error for all 3
    When Fork is called
    Then return value is non-nil errors.JoinError containing all 3 wrapped errors
    And errors.Is(returnedErr, factoryErr1) is true
    And errors.Is(returnedErr, factoryErr2) is true
    And errors.Is(returnedErr, factoryErr3) is true

  Scenario: Spawn errors + rollback errors combined
    Given Forker with 2 spawn failures
    And mock Sandbox.Exit failing during rollback
    When Fork is called
    Then return value is errors.JoinError containing both spawn and rollback errors
    And total wrap count == 4 (2 spawn + 2 rollback)

  Scenario: All spawns succeed but rollback fails
    Given Forker with 3 successful spawns
    But mid-batch failure triggers rollback
    And Sandbox.Exit returns error during rollback
    When Fork is called
    Then return value is non-nil errors.JoinError
    And no spawn errors wrapped (only rollback errors)

  Scenario: All operations succeed
    Given Forker with all successful operations
    When Fork is called
    Then return value is nil
    And no error is logged
```

### T Layer

- D7-S6-A13-T01: Forker 3 个并发失败 → errors.Join(err1, err2, err3) 而非 errs[0]
- D7-S6-A13-T02: Spawn errors + rollback errors 合并 → errors.Join 包含全部
- D7-S6-A13-T03: Rollback 失败时仍返回 errors.Join

---

## Out of Scope（明确排除）

- 不重构 `coordinator/aliases.go` legacy shim
- 不重构 `Dispatcher.Dispatch` 孤儿模块
- 不动 `summarizeArtifacts` metadata 增强
- 不统一三流格式（FastPath vs OrchestratePath）
- 不实现 `UncertaintyCoord`
- 不接入 `AdaptiveThreshold` 到 RunTurn（TD-WT-01 P2 延期 v3.0）
- 不重构 `turn/orchestrator.go` 1349 行超大文件
- 不做 OpenTelemetry 接入
- 不动 `taskCtx` 脱离 parentCtx 设计

## 关联 Change

- `devrix-error-handling-tier1-tier2`（S7_Archived 2026-06-20）—— sharederrors 公共域修复
- `devrix-tool-surface-phase2-full-pr64` —— 12→0 global loop 关闭
- `devrix-context-budget-phase-a-pr128` —— Phase A 5/5 AC