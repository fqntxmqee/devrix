# D7 turn/ 包合并到 sessionorchestrator/ Spec

**Module:** D7 Orchestration / S2 SessionOrchestrator (Mediator + Turn Leader + Error Recovery)
**Change:** `devrix-d7-6s-package-merge` (DM-20260626-004)
**Status:** S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**Spec Version:** v1.0
**依赖:** devrix-d7-six-s-simplification (DM-20260626-001) + devrix-d7-mups-package-migration (DM-20260626-002) + devrix-d7-hardening-cross-cutting (DM-20260626-003) 全部 S7_Archived

---

## ADDED

### Requirement: sessionorchestrator/ Package Contains turn/ DefaultOrchestrator

`internal/layers/orchestration/sessionorchestrator/orchestrator.go` 必须包含原 `turn/orchestrator.go` (1462 行) 全部内容，类型 `DefaultOrchestrator` + 函数 `NewOrchestrator(deps OrchestratorDeps) *DefaultOrchestrator` + 11 个核心导出 type + 6 个核心导出函数 0 变化。

<!-- T: D7-S2-A50-T01 -->

#### Scenario: sessionorchestrator/orchestrator.go contains DefaultOrchestrator

- GIVEN 原 `internal/layers/orchestration/turn/orchestrator.go` (1462 行) 包含 `DefaultOrchestrator` struct + `NewOrchestrator` 函数 + `OrchestratorDeps` struct + 14 ExitReason 引用
- AND `package turn` 声明在原文件中
- WHEN 执行 `git mv internal/layers/orchestration/turn/orchestrator.go internal/layers/orchestration/sessionorchestrator/orchestrator.go`
- AND `package turn` 改为 `package sessionorchestrator` (1 行 sed)
- THEN `internal/layers/orchestration/sessionorchestrator/orchestrator.go` 存在
- AND 包含 `DefaultOrchestrator` struct (字段 llm + runCompress + obsBridge 等 0 变化)
- AND 包含 `NewOrchestrator(deps OrchestratorDeps) *DefaultOrchestrator` 函数 0 变化
- AND 包含 `OrchestratorDeps` struct 0 变化
- AND `package sessionorchestrator` 声明在新文件中
- AND `internal/layers/orchestration/turn/orchestrator.go` 不存在

#### Scenario: 23 non-conflicting .go files git mv with package rename

- GIVEN 原 `internal/layers/orchestration/turn/` 包含 25 个 .go 文件
- AND 其中 23 个 .go 文件名（不含 `doc.go` + `tracing.go`）与 `sessionorchestrator/` 内文件名无冲突
- WHEN 执行 23 次 `git mv` 把 23 个文件迁移到 `sessionorchestrator/`
- AND 执行 `sed -i '' 's|^package turn$|package sessionorchestrator|'` 替换 23 个文件的 package 声明
- THEN 23 个文件全部位于 `sessionorchestrator/` 内
- AND 每个文件 `head -1` 输出 `package sessionorchestrator`
- AND `internal/layers/orchestration/turn/` 目录物理消失

#### Scenario: 2 same-name files renamed to avoid collision

- GIVEN 原 `turn/doc.go` (17 行, D7-S2-A06/A07 Turn Leader 说明) 与 `sessionorchestrator/doc.go` (17 行, D7-S2 Session Orchestrator 说明) 同名
- AND 原 `turn/tracing.go` (44 行, `(o *DefaultOrchestrator).startSpan` receiver) 与 `sessionorchestrator/tracing.go` (startObsSpan + `(o *SessionOrchestrator).startSpan`) 同名
- WHEN 执行 `git mv turn/doc.go sessionorchestrator/turn_doc.go`
- AND `git mv turn/tracing.go sessionorchestrator/tracing_turn.go`
- AND `package turn` 改为 `package sessionorchestrator` 在 turn_doc.go + tracing_turn.go
- THEN `sessionorchestrator/turn_doc.go` 存在（保留原 turn/doc.go 内容）
- AND `sessionorchestrator/tracing_turn.go` 存在（保留原 turn/tracing.go 的 `(o *DefaultOrchestrator).startSpan` receiver method）
- AND `sessionorchestrator/doc.go` 与 `sessionorchestrator/tracing.go` 0 变化（保留原 sessionorchestrator 内容）

---

### Requirement: Zero Residual Old-Path Imports

全仓 `grep -rln "orchestration/turn\""` 与 `grep -rln "turn\.NewOrchestrator"` + `grep -rln "turn\.DefaultOrchestrator"` + `grep -rln "turn\.SubTurnRunner"` 等 11 个核心 type/func 跨包调用必须 0 命中。

<!-- T: D7-S2-A50-T03 -->

#### Scenario: All internal references migrated to sessionorchestrator/

- GIVEN 14 个 importer 文件 (10 bootstrap + 2 decisionplanning + 2 sessionorchestrator/turn_tools) 含 `orchestration/turn"` import path
- AND `sessionorchestrator/turn_tools.go` + `turn_tools_test.go` 内部已 import "turn" 然后调用 `turn.X`
- WHEN 执行 `sed -i '' 's|internal/layers/orchestration/turn"|internal/layers/orchestration/sessionorchestrator"|g'` 替换 14 importer
- AND sessionorchestrator/turn_tools.go 内部 `turn.X` 引用 → `sessionorchestrator.X` (同包 bare name 应自动)
- THEN `grep -rln "orchestration/turn\"" internal/ cmd/` 返回 0 命中
- AND `grep -rln "package turn$" internal/layers/orchestration/` 返回 0 命中 (turn/ 已删)
- AND `grep -rln "turn\.NewOrchestrator\|turn\.DefaultOrchestrator\|turn\.SubTurnRunner\|turn\.GatewayInvoker\|turn\.CompressionSummarizer\|turn\.OrchestratorDeps\|turn\.TurnOrchestrator\|turn\.PreparedTurnAdapter"` 返回 0 命中

#### Scenario: hardening/ receiver methods unchanged

- GIVEN hardening/ 落地 (DM-20260626-003) 后 `turn/recovery.go` 保留 `compressMessagesForRecovery` + `invokeStreamWithRecovery` 两个 receiver methods (类型 `*DefaultOrchestrator`)
- WHEN turn/ → sessionorchestrator/ 物理合并完成
- AND sessionorchestrator/recovery.go 中 `(o *DefaultOrchestrator).compressMessagesForRecovery` + `(o *DefaultOrchestrator).invokeStreamWithRecovery` 0 变化
- THEN sessionorchestrator/recovery.go 仍 import hardening
- AND receiver methods 内部 `hardening.IsContextLengthError` + `hardening.IsOverloadOr5xx` + `hardening.NeedsMaxOutputTokenRecovery` + `hardening.MaxOutputTokensRecoveryMessage` 4 处调用 0 变化
- AND hardening/ 包 0 变化 (Decision 5)

---

### Requirement: Build, Vet, Test All Green

`go build ./...` 0 错误 + `go vet ./...` 0 警告 + `go test ./internal/layers/orchestration/... -race -count=1` 23/23 PASS。

<!-- T: D7-S2-A50-T02 -->

#### Scenario: go build returns 0 errors

- GIVEN 完成 turn/ → sessionorchestrator/ 物理合并 + package 改名 + 14 importer import path 替换
- WHEN 执行 `go build ./...`
- THEN 返回 exit code 0
- AND stdout/stderr 无编译错误
- AND sessionorchestrator/ 包扩展至 ~60 文件 ~15000 行，0 编译错误

#### Scenario: go vet returns 0 warnings

- GIVEN 完成 turn/ → sessionorchestrator/ 物理合并 + package 改名 + 14 importer import path 替换
- WHEN 执行 `go vet ./...`
- THEN 返回 exit code 0
- AND stdout 无 vet 警告

#### Scenario: go test -race passes 23/23 orchestration packages

- GIVEN 完成 turn/ → sessionorchestrator/ 物理合并 + package 改名 + 14 importer import path 替换
- WHEN 执行 `go test ./internal/layers/orchestration/... -race -count=1`
- THEN 返回 23/23 包 PASS（22 baseline + hardening 1 包 = 23，与 hardening 落地后 baseline 持平；turn/ 合并后 sessionorchestrator/ 仍是 1 包，包数不增不减）
- AND 0 race condition detected
- AND sessionorchestrator/ 包内 25 个测试文件（11 迁入 + 14 原）全部 PASS

#### Scenario: LP-1/LP-2/LP-5 paths are unchanged

- GIVEN 完成 turn/ → sessionorchestrator/ 物理合并 + package 改名 + 14 importer import path 替换
- WHEN 检查 LP-1 (Bayesian reputation) → LP-2 (Memory 3 通道) → LP-5 (Cross-session traceability) 三条核心数据流
- THEN 三条路径全部 0 变化（仅物理迁移）
- AND Phase 6 + Phase 7 集成测试（TestAutoClose_FullLP1Loop + TestIntegration_5NodePipeline_End2End）全部通过

#### Scenario: hardening/ + escape/circuit_breaker.go + autoclose.go unchanged

- GIVEN 完成 turn/ → sessionorchestrator/ 物理合并 + package 改名 + 14 importer import path 替换
- WHEN 执行 `git diff HEAD -- internal/layers/orchestration/hardening/`
- AND `git diff HEAD -- internal/layers/orchestration/escape/circuit_breaker.go`
- AND `git diff HEAD -- internal/layers/orchestration/sessionorchestrator/autoclose.go`
- THEN 三个 diff 全部空（Decision 5 + Decision 1 + Decision 4 验证）

---

## MODIFIED

### Requirement: docs/openspec 同步

`openspec/specs/d7-orchestration/d7-domain.md` v2.1.0 → v2.2.0 + `design.md` v4.1.0 → v4.2.0 + 域 `t-registry.md` v4.3.0 → v4.4.0 (新增 D7-S2-A50-T01..T04) + 根 `t-registry.md` v5.3.0 → v5.4.0 (新增 DM-20260626-004 增量条目)。

<!-- T: D7-S2-A50-T04 -->

#### Scenario: d7-domain.md §① S2 SessionOrchestrator 包路径更新

- GIVEN 原 `openspec/specs/d7-orchestration/d7-domain.md` v2.1.0 §① S2 SessionOrchestrator 章节描述 turn/ + sessionorchestrator/ 双包结构
- WHEN 更新 §① S2 章节为"sessionorchestrator/ 包（扩展至 ~60 文件 ~15000 行，包含 SessionOrchestrator 顶层 + DefaultOrchestrator RunTurn 主循环 + 11 个核心 type + 6 个核心函数）"
- AND 更新版本号 v2.1.0 → v2.2.0
- THEN `d7-domain.md` v2.2.0 包含 S2 SessionOrchestrator 单包物理封装描述

#### Scenario: design.md §① Discipline Keeper 包路径更新

- GIVEN 原 `openspec/specs/d7-orchestration/design.md` v4.1.0 §① S2 SessionOrchestrator 章节描述 turn/ 子包
- WHEN 更新 §① S2 章节描述 turn/ 已合并到 sessionorchestrator/
- AND 更新版本号 v4.1.0 → v4.2.0
- THEN `design.md` v4.2.0 包含 S2 单包描述

#### Scenario: t-registry 同步 D7-S2-A50-T01..T04 IMPLEMENTED

- GIVEN 域 `openspec/specs/d7-orchestration/t-registry.md` v4.3.0 含 D7-S2-A50 PLANNED (4 T)
- AND 根 `openspec/t-registry.md` v5.3.0 含 DM-20260626-004 PLANNED 增量条目
- WHEN 4 T 状态 PLANNED → IMPLEMENTED
- AND 域 t-registry 版本 v4.3.0 → v4.4.0
- AND 根 t-registry 版本 v5.3.0 → v5.4.0
- AND 根 t-registry 新增条目"DM-20260626-004 增量：D7-S2-A50 T01-T04 IMPLEMENTED (turn/ → sessionorchestrator/)"
- THEN t-registry 同步完成

---

## REMOVED

(None — hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go 0 变化，turn/ 物理删除被 git mv 抵消)

---

## 已观察但 NOT IN SCOPE（明确不在本 change 范围）

- ❌ exit_reason.go + verdict_to_exit_reason.go + verdict_to_exit_reason_test.go 临时留 sessionorchestrator/，由 follow-up #4 (devrix-d7-6s-verify-promotion / DM-20260626-005) 从 sessionorchestrator/ promote 到 executionflow/verify/
- ❌ observe/orchtypes → decisionplanning/ 合并 由 follow-up #5 (devrix-d7-6s-observe-merge / DM-20260626-006) 处理
- ❌ bootstrap wire 14 → 6 收敛 由 follow-up #6 (devrix-d7-6s-bootstrap-slim / DM-20260626-007) 处理
- ❌ DefaultOrchestrator + SessionOrchestrator 双 type 命名保留（Decision 3，不改）