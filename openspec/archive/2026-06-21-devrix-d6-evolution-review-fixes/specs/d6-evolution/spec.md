# D6 Evolution Spec Delta

**Change ID:** devrix-d6-evolution-review-fixes
**Demand ID:** DM-20260621-011
**Date:** 2026-06-21

---

## D6-S11-A02-T09: Verify Invariant 启动期 fail-safe

### Capability

`verify/_invariant.go` ParseStruct 失败时不再 `panic`，改为 `init()` 中 `log.Fatalf` 退出，进程退出码非 0 由 systemd / k8s 接管重启。

### Gherkin

```gherkin
Feature: Verify Invariant fail-safe

  Scenario: ParseStruct 失败 → log.Fatal + 退出码 1
    Given verify invariant struct 标签格式错误
    When init() 调用 ParseStruct
    Then 返回 error 而非 panic
    And init() 调用 log.Fatalf 退出码 1
    And `git grep "panic.*verify invariant"` 0 命中
```

---

## D6-S12-A01: InterventionExecutor 错误聚合与可观测性

### Capability

`InterventionExecutor.terminateAndReroute` 中 `current.Wait(ctx)` 与 `tasks.Fail(taskID, reason)` 失败时走 atomic counter + slog.Warn + errors.Join 三联固化模式，与 D7 error aggregation (DM-20260621-010) 一致。

### Gherkin

```gherkin
Feature: InterventionExecutor silent swallow → errors.Join

  Scenario: Wait 失败 → metric + slog + 错误非 nil
    Given InterventionExecutor + mock agent (waitErr != nil)
    When terminateAndReroute(ctx, session, iv)
    Then 返回 errors.Join 且 errors.Is(err, waitErr) == true
    And metrics.WaitFailed == 1

  Scenario: tasks.Fail 失败 → metric + slog + 错误非 nil
    Given InterventionExecutor + mock tasks (failErr != nil)
    When terminateAndReroute(ctx, session, iv{MilestoneFail: true})
    Then 返回 errors.Join 且 errors.Is(err, failErr) == true
    And metrics.TaskFailFailed == 1

  Scenario: Terminate 失败 + Wait 成功 → 只包含 Terminate error
    Given InterventionExecutor + mock agent (terminateErr != nil, waitErr == nil)
    When terminateAndReroute(ctx, session, iv)
    Then 返回 errors.Join 仅含 terminate error

  Scenario: 三步全成功 → nil
    Given InterventionExecutor + 全成功 mock
    When terminateAndReroute(ctx, session, iv)
    Then 返回 nil
```

---

## D6-S12-A02: Guard 名空间收敛（bridge 完全删除）

### Capability

`internal/layers/evolution/eval/bridge.go` + `internal/layers/evolution/orchestration/bridge.go` 完全删除，spec/code 一致性 100%，外部调用方全部迁移至 `import guard` 直接路径。

### Gherkin

```gherkin
Feature: Bridge 完全删除

  Scenario: bridge.go 文件不存在
    Given PR-B 已合入
    When git ls-files internal/layers/evolution/eval/ internal/layers/evolution/orchestration/
    Then 0 命中

  Scenario: 外部无 bridge import
    Given 全仓 *.go 文件
    When grep "evolution/eval/bridge\|evolution/orchestration/bridge"
    Then 0 命中
```

---

## D6-S12-A03: Orchestration* → Guard* 命名迁移

### Capability

`guard/` 包内 6 处 `Orchestration*` 类型 / 函数 + 6 个 `orch_*` OpenTelemetry 指标重命名为 `Guard*` + `guard_*`，与 spec.md v2.3.0 / v2.4.0 一致。旧名保留 type alias（v2.4 引入，`//go:deprecated`，v2.5 删除）。

### Gherkin

```gherkin
Feature: Orchestration* → Guard* rename

  Scenario: 类型 / 函数重命名完成
    Given PR-B 已合入
    When 全仓 grep "Orchestration" --include="*.go"
    Then 仅命中 alias 定义点（≤ 6 处）
    And alias 定义点均有 //go:deprecated 注释

  Scenario: 指标名重命名完成
    Given PR-B 已合入
    When 全仓 grep "orch_decisions_total\|orch_validations_total\|orch_interventions_total\|orch_judge_latency_seconds\|orch_observer_active\|orch_decisions_by_stage"
    Then 仅命中 metrics.go 中旧名注释 + alias（≤ 6 处）

  Scenario: 旧名调用方编译通过（向后兼容）
    Given 外部代码使用 OrchestrationConfig（type alias）
    When go build ./...
    Then exit 0
    And go vet 提示 OrchestrationConfig 已废弃
```