# D7 Cross-cutting Hardening — hardening/ Package Migration Spec

**Module:** D7 Orchestration / S7 Cross-cutting (Discipline Keeper)
**Change:** `devrix-d7-hardening-cross-cutting` (DM-20260626-003)
**Status:** S3_Design
**Spec Version:** v1.0
**依赖:** devrix-d7-six-s-simplification (DM-20260626-001) S7_Archived + devrix-d7-mups-package-migration (DM-20260626-002) S7_Archived

---

## ADDED

### Requirement: hardening/metrics Package Directory Exists

`internal/layers/orchestration/hardening/` 目录必须存在，包含原 `sessionorchestrator/metrics.go` + `metrics_test.go` 全部内容，`package hardening` 声明（取代原 `package sessionorchestrator`）。

<!-- T: D7-S7-A01-T01 -->

#### Scenario: hardening/metrics directory contains InterruptMetrics + Snapshot

- GIVEN 原 `internal/layers/orchestration/sessionorchestrator/metrics.go` 包含 `InterruptMetrics` struct + `Snapshot()` + `TotalCancelFailures()` 方法
- AND `package sessionorchestrator` 声明在原文件中
- WHEN 执行 `git mv internal/layers/orchestration/sessionorchestrator/metrics.go internal/layers/orchestration/hardening/metrics.go`
- AND `git mv internal/layers/orchestration/sessionorchestrator/metrics_test.go internal/layers/orchestration/hardening/metrics_test.go`
- AND `package sessionorchestrator` 改为 `package hardening` (1 行修改)
- THEN `internal/layers/orchestration/hardening/metrics.go` 存在
- AND `internal/layers/orchestration/hardening/metrics_test.go` 存在
- AND `package hardening` 声明在两个文件中
- AND `InterruptMetrics` 类型 + 5 个字段 + 3 个方法 (`Snapshot` + `TotalCancelFailures`) 0 变化
- AND `git log --follow` 能追溯到原文件的 commit 历史

---

### Requirement: hardening/recovery Package Directory Exists

`internal/layers/orchestration/hardening/recovery.go` 目录必须存在，包含 turn/recovery.go subset（4 纯函数 + 1 常量），`package hardening` 声明。

<!-- T: D7-S7-A02-T02 -->

#### Scenario: hardening/recovery directory contains 4 pure functions + 1 const

- GIVEN 原 `internal/layers/orchestration/turn/recovery.go` 包含 7 项内容（4 纯函数 + 2 receiver methods + 1 struct + 2 const）
- WHEN 拆分 4 纯函数 + 1 const 迁 hardening/recovery.go:
  - `IsContextLengthError(err error) bool`
  - `IsOverloadOr5xx(err error) bool` (calls hardening.IsContextLengthError)
  - `NeedsMaxOutputTokenRecovery(finishReason string) bool`
  - `MaxOutputTokensRecoveryMessage` const
- AND 保留 3 项在 turn/recovery.go (Decision 2):
  - `partialStreamEmit` struct (unexported)
  - `emitStreamRecoveryTombstones(...)` function
  - `compressMessagesForRecovery(...)` + `invokeStreamWithRecovery(...)` receiver methods on `*DefaultOrchestrator`
- THEN `internal/layers/orchestration/hardening/recovery.go` 存在
- AND 包含 4 纯函数 + 1 const，函数体 0 变化（仅包名 + hardening.IsContextLengthError 内部引用变化）
- AND `package hardening` 声明
- AND `internal/layers/orchestration/turn/recovery.go` 仍存在，保留 3 项内容

---

### Requirement: Zero Residual Old-Path Imports

全仓 `grep -rln "sessionorchestrator\.InterruptMetrics"` 与 `grep -rln "turn\.IsContextLengthError"` + `grep -rln "turn\.NeedsMaxOutputTokenRecovery"` 必须 0 命中。

<!-- T: D7-S7-A01-T03 -->

#### Scenario: All internal references migrated to hardening/

- GIVEN metrics.go 迁 hardening 后，`sessionorchestrator/interrupt.go` Metrics 字段类型从 `*InterruptMetrics` 改为 `*hardening.InterruptMetrics`
- AND recovery.go 拆 hardening 后，`turn/orchestrator.go` 2 处引用从 `NeedsMaxOutputTokenRecovery` / `MaxOutputTokensRecoveryMessage` 改为 `hardening.NeedsMaxOutputTokenRecovery` / `hardening.MaxOutputTokensRecoveryMessage`
- AND `turn/recovery.go` 内 `IsContextLengthError(err)` 调用从同包变为 `hardening.IsContextLengthError(err)`
- WHEN 完成所有 import path + package prefix 替换
- THEN `grep -rln "sessionorchestrator\.InterruptMetrics" internal/ cmd/` 返回 0 命中
- AND `grep -rln "turn\.IsContextLengthError" internal/ cmd/` 返回 0 命中
- AND `grep -rln "sessionorchestrator/metrics" internal/ cmd/` 返回 0 命中
- AND `grep -rln "turn/recovery" internal/ cmd/` 返回 0 命中

---

### Requirement: Build, Vet, Test All Green

`go build ./...` 0 错误 + `go vet ./...` 0 警告 + `go test ./internal/layers/orchestration/... -race -count=1` 23/23 PASS。

<!-- T: D7-S7-A01-T04 -->

#### Scenario: go build returns 0 errors

- GIVEN 完成 hardening/ 物理目录创建 + 4 文件 git mv + import path 替换
- WHEN 执行 `go build ./...`
- THEN 返回 exit code 0
- AND stdout/stderr 无编译错误

#### Scenario: go vet returns 0 warnings

- GIVEN 完成 hardening/ 物理目录创建 + 4 文件 git mv + import path 替换
- WHEN 执行 `go vet ./...`
- THEN 返回 exit code 0
- AND stdout 无 vet 警告

#### Scenario: go test -race passes 23/23 orchestration packages

- GIVEN 完成 hardening/ 物理目录创建 + 4 文件 git mv + import path 替换
- WHEN 执行 `go test ./internal/layers/orchestration/... -race -count=1`
- THEN 返回 23/23 包 PASS（原 22 + 新 hardening 1 包）
- AND 0 race condition detected
- AND hardening/ 包内 4 测试全部 PASS (TestInterruptMetrics_Snapshot_AtomicIncrement + TestInterruptMetrics_NilSafe + TestInterruptMetrics_TotalCancelFailures + TestInterruptMetrics_Snapshot_AllFields)

#### Scenario: LP-1/LP-2/LP-5 paths are unchanged

- GIVEN 完成 hardening/ 物理目录创建 + 4 文件 git mv + import path 替换
- WHEN 检查 LP-1 (Bayesian reputation) → LP-2 (Memory 3 通道) → LP-5 (Cross-session traceability) 三条核心数据流
- THEN 三条路径全部 0 变化
- AND Phase 6 + Phase 7 集成测试（TestAutoClose_FullLP1Loop + TestIntegration_5NodePipeline_End2End）全部通过

## MODIFIED

(None — hardening/ 是 NEW 包，所有变更属于 ADDED)

## REMOVED

(None — escape/circuit_breaker.go 0 变化（Decision 1）)
