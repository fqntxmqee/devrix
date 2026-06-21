# Delta Spec: D7 Metrics Naming Alignment & Concurrency Hardening

**Change ID:** `devrix-d7-metrics-and-concurrency-hardening`
**Affects:** `internal/layers/orchestration/wavescheduler/` (scheduler.go + conflict.go + scheduler_metrics_test.go)
**Affects:** `internal/layers/orchestration/sessionorchestrator/` (command_handler.go)
**Affects:** `openspec/specs/d7-orchestration/spec.md` (D7-S6-A12 命名澄清 + D7-S6-A14 新增)
**Affects:** `openspec/specs/d7-orchestration/t-registry.md` (D7-S6-A14-T01..T06 新增 + D7-S6-A12-T01 OBSOLETE)
**Demand ID:** DM-20260622-001
**Date:** 2026-06-22

---

## MODIFIED

### Requirement: D7-S6-A12 Worktree Observability (命名澄清)

> **改动**：metric 名由单数改为复数，与 spec D7-S6-A12-T01..T05 保持一致；
> **`sandbox_exit_failed` 跨域归属澄清**：该 counter 由 D4 `multiagent/execute/worker.go` 提供，
> D7 wavescheduler 不重复声明。D7 域 spec 该条标注 OBSOLETE，cross-ref 到 D4 对应 T 点。

**Priority:** P0
**Package:** `internal/layers/orchestration/wavescheduler/` + `internal/layers/multiagent/execute/`

#### Scenario: dispatchLoop wakeup counter 与 spec 名对齐

- GIVEN `wavescheduler/scheduler.go::dispatchLoop` 在 `<-state.wakeupCh` 和 `<-ticker.C` 两条 case
- WHEN 任一 case 被触发
- THEN 调用 `s.incMetric("dispatch_loop_wakeups")`（**复数**，与 spec 名一致）
- AND 测试 `scheduler_metrics_test.go` 断言使用 `"dispatch_loop_wakeups"`
- AND D5 dashboard 按 `dispatch_loop_wakeups` 名过滤可看到计数

#### Scenario: Worker panic counter 与 spec 名对齐

- GIVEN `wavescheduler/scheduler.go::dispatchOne` 内 `defer recover` 检测 worker panic
- WHEN panic 被 recover
- THEN 调用 `s.incMetric("worker_panics")`（**复数**，与 spec 名一致）
- AND slog.Error 字段 `"metric": "worker_panics"`（**复数**）
- AND 测试 `scheduler_metrics_test.go` 断言使用 `"worker_panics"`

#### Scenario: sandbox_exit_failed 由 D4 提供（D7 不重复声明）

- GIVEN Forker.Fork 路径触发 sandbox 清理失败
- WHEN 实际 incMetric 触发方在 `multiagent/execute/worker.go`（D4 域）
- THEN D7 域 spec 不重复声明 `sandbox_exit_failed` 计数器
- AND spec.md §D7-S6-A12-T01 标注 OBSOLETE + cross-ref D4-S6-A12-Txx
- AND D5 dashboard `sandbox_exit_failed` 仅由 D4 域唯一来源 emit

---

## ADDED

### Requirement: D7-S6-A14 Metrics Naming Alignment & Concurrency Hardening

D7 编排层在 DM-20260621-010 PR-B 已落地 5 个 worktree counter 的基础上，
闭合遗留的 4 类 P0/P1 问题：spec/code 命名漂移、TOCTOU 热路径未启用原子调用、
`state.cancels` 无界增长、`command_handler` 阻塞 send。

**Priority:** P1
**Package:** `internal/layers/orchestration/wavescheduler/` + `internal/layers/orchestration/sessionorchestrator/`
**T:** D7-S6-A14-T01 ... D7-S6-A14-T06

#### Scenario: state.cancels wave 完成时清空（防长会话 leak）

- GIVEN WaveScheduler 调度某 wave 并注册 N 个 cancel funcs 到 `state.cancels`
- WHEN wave 完成（`markWaveDone` 触发）
- THEN `state.cancels` 被置为 nil（或 len == 0）
- AND `state.handles` 同步清理（无残留 task handle）
- AND 同一 session 后续 wave 重入不会累积 cancel funcs

#### Scenario: ConflictGuard 热路径使用 AllowAndRegister

- GIVEN WaveScheduler dispatchLoop 处理 ready node
- WHEN `dispatchOne` 内执行冲突检查
- THEN 调用 `s.guard.AllowAndRegister(node, slotID, s.guard.Running())` 单次原子调用
- AND **不**调用 split `Allow(node, running)` + `Register(...)` 序列
- AND `go test -race` 在 100 次并发 dispatch 下无 race detector 报警

#### Scenario: CommandHandler out channel 满时丢事件不阻塞

- GIVEN `sessionorchestrator/command_handler.go` 的 out channel 已满（buffer 32）
- WHEN CommandHandler 试图 emit 新 EngineEvent
- THEN 使用 `select { case out <- event: default: slog.Warn(...) }` 模式
- AND emit 操作在 < 100ms 内返回（不阻塞）
- AND slog.Warn 记录被丢弃事件（type + sessionID）

#### Scenario: 5 个 counter 命名与 spec 100% 对齐

- GIVEN D7 域 metrics 全部 incMetric 调用
- WHEN `grep -rn 'incMetric(' internal/layers/orchestration/wavescheduler/`
- THEN 5 个 counter 名（dispatch_loop_wakeups / worker_panics / task_ctx_leaked / wave_reentry_cancelled / completed/failed/cancelled）与 spec D7-S6-A12-T01..T05 完全一致
- AND D5 dashboard 按 spec 名过滤 5/5 可见

#### Scenario: D7-S6-A14 PR-A 测试覆盖

- GIVEN PR-A 完成 5 个修复点
- WHEN 跑 `go test -race ./internal/layers/orchestration/wavescheduler/...` + `sessionorchestrator/...`
- THEN 6 个新 P0 T 点全部 PASS（D7-S6-A14-T01..T06）
- AND 覆盖率 ≥80%

---

## REMOVED

### Requirement: D7-S6-A12-T01 sandbox_exit_failed（D7 域 OBSOLETE）

> **移除原因**：该 counter 实际由 D4 `multiagent/execute/worker.go::recordSandboxExitFailed` 提供，
> 不属于 D7 wavescheduler 域职责。spec 该条改为 OBSOLETE，cross-ref 到 D4 域对应 T 点。
> 历史记录保留以追溯 PR-B (DM-20260621-010) 决策。

**原内容（保留作历史）**：
> D7-S6-A12-T01: Sandbox Exit 失败 → metric.Inc("sandbox_exit_failed") + slog.Warn

**替换为**：
> D7-S6-A12-T01 [OBSOLETE, 2026-06-22, see D4-S6-A12-Txx for actual owner]
> — 该 counter 由 D4 multiagent/execute/worker.go 提供，D7 spec 不重复声明。

---

## 跨域 Reference

| Counter 名 | 实际归属 | 触发文件 | 跨域 spec ref |
|-----------|---------|---------|--------------|
| `sandbox_exit_failed` | D4 | `internal/layers/multiagent/execute/worker.go:54-61` | D4-S6-A12-Txx |
| `dispatch_loop_wakeups` | D7 | `internal/layers/orchestration/wavescheduler/scheduler.go:306, 309` | D7-S6-A14-T01 |
| `worker_panics` | D7 | `internal/layers/orchestration/wavescheduler/scheduler.go:414` | D7-S6-A14-T02 |
| `task_ctx_leaked` | D7 | `internal/layers/orchestration/wavescheduler/scheduler.go:509` | D7-S6-A12-T03 (已 IMPLEMENTED) |
| `wave_reentry_cancelled` | D7 | `internal/layers/orchestration/wavescheduler/scheduler.go:237` | D7-S6-A12-T04 (已 IMPLEMENTED) |
| `completed` / `failed` / `cancelled` | D7 | `wavescheduler/scheduler.go:490-494` | 既有 T 点 |

---

## 与既有 spec 章节的关系

| 章节 | 关系 | 改动 |
|------|------|------|
| §D7-S6-A11 HandleInterrupt Error Aggregation | 不变 | — |
| §D7-S6-A12 Worktree Observability | **MODIFIED** | T01 OBSOLETE + T02/T05 metric 名复数化 |
| §D7-S6-A13 Forker Batch Error Surfacing | 不变 | — |
| §D7-S6-A14 Metrics Naming Alignment & Concurrency Hardening | **ADDED** | 本 change 新增 |