# Proposal: devrix-d4-dsaft-restructuring (DM-20260629-004)

**Change ID:** `devrix-d4-dsaft-restructuring`
**Demand ID:** DM-20260629-004
**Status:** S2_Proposal
**Title:** D4 多智能体域 DSAFT 深度架构重构 — 6 子 Change 联动 (legacy + god-fn + registry + value-flow + span + boundary)
**Template:** `devrix-d3-dsaft-restructuring` proposal.md (DM-20260629-003 S7_Archived)

---

## §0 概述

D4 v1.0.0（2026-06-15 d4-domain.md）已 stable 9 子包 + 4108 LOC + 6 active span ops。但 2026-06-29 DSAFT Review 暴露 5 类债务，对齐 D7/D2/D3 6 子 Change 模板，启动 D4 6 子 Change 联动 refactoring。本 proposal 把 5 类债务映射到 6 子 Change + 1 S7_Archive 收口。

---

## §1 子 Change 提案汇总

| # | 子 Change | PR | T 范围 | 工作量 |
|---|----------|----|----|------|
| **#0** | legacy-cleanup | PR-1 | T01-T05 | 1 PR / 1 天 |
| **#1** | god-fn-split pt1 | PR-2 | T06-T09 | 1 PR / 1 天 |
| **#1** | god-fn-split pt2 | PR-3 | T10-T13 | 1 PR / 1 天 |
| **#2** | registry-sync | PR-4 | T14-T17 | 1 PR / 1 天 |
| **#3** | value-flow-rename | PR-5 | T18-T21 | 1 PR / 1 天 |
| **#4** | span-coverage | PR-6 | T22-T27 | 1 PR / 1 天 |
| **#5** | boundary-decision | PR-7 | T28-T31 | 1 PR / 1 天 |
| **S7_Archive** | S7_Archive | PR-8 | T32-T34 | 1 PR / 1 天 |
| **总计** | **8 PR** | **~34 T** | **8-10 天** |

---

## §2 子 Change #0 legacy-cleanup (PR-1, T01-T05)

**目标**：建立 `orchtypes/` 治理包基础 + inline 确实的过度包装 + 审计 Snapshot 类型。

### T01 建立 `internal/layers/multiagent/orchtypes/` 包

- New dir + `orchtypes/spans.go` (move from `multiagent/spans.go`)
- 6 OpD4_S4_* 在 `internal/layers/observability/instrument/telemetry/names.go` 已存在，**不动**；orchtypes/spans.go 仅承载 coverage.RegisterProvider 钩子
- multiagent/spans.go 删除 (无 caller 副作用 + init() 注册迁移到 orchtypes 包)

### T02 inline WorkerEngine wrapping

- 删除 `run/worker_engine.go` (44 LOC) — 纯 delegating wrapper
- 把 `WorkerEngine` struct 类型 + `NewWorkerEngine()` + `Process()` inline 到 `provision/factory.go` 的 `setEngine` 路径
- `factory.go` L112/L119 `run.NewWorkerEngine(inner, resolved, impl.ID())` → `newInlineWorkerEngine(inner, resolved, impl.ID())` (factory 包内私有函数)
- 保持 contracts.IEngine interface 不变；run/worker_engine_test.go 33 行同步 inline

### T03 清理 ExecutorMetricsSnapshot 类型（仅内部测试用）

- `execute/metrics.go::ExecutorMetricsSnapshot` + Snapshot() 函数保留（atomic counter 是 D5 contract）
- 类型保留 JSON-friendly（`json:"sandbox_exit_failed"`）— **不动**

### T04 清理 ForkerMetricsSnapshot 类型（仅内部测试用）

- `provision/freefork/metrics.go::ForkerMetricsSnapshot` + Snapshot() 函数保留
- 6 字段 JSON schema `forker.schema.json` 是 D5 contract — **不动**

### T05 8 dead exported 确认（实际审计）

- 实际删除 = 0
- `agent.go::Creator` interface — 保留（run.Impl 字段依赖，外部 stub 实现）
- `WorkerEngine` type — 已在 T02 消除
- 2 Snapshot types — 已在 T03/T04 说明保留
- `NewAgentFactory` — 保留（tests/integration + bootstrap/multi_agent.go 使用）
- `EngineBuilder` interface — 保留（factory.go internal 字段依赖）
- 其余 CLIAgentTool/CursorAgentTool/AgentState/Agent — 全部保留（重 caller）
- `multiagent/contracts.go` 47 LOC shim — **暂不退役**，保留向后兼容

---

## §3 子 Change #1 god-fn-split pt1 (PR-2, T06-T09)

### T06 拆 `external/cli_adapter.go` 466 LOC

- → `external/cli_session.go` (session 生命周期 + 工具方法)
- → `external/cli_execute.go` (execute 路径 + streaming)

### T07 cli_session.go (目标 <300 LOC)

- 包含：CLIConfig/CLISession/CLIAgentTool struct + ensureSession + CloseSession + dropSession + closeSession + CleanupBySessionID + Stop + idleSweeper + reapIdle + Info + ExecutionTimeout

### T08 cli_execute.go (目标 <250 LOC)

- 包含：Execute + parseCLIStream (新 helper) + validateCLISession (新 helper) + buildCLIPrompt (新 helper) + handler functions (text/assistant/thinking/tool_call/result)

### T09 验证

- cli_session.go <300 LOC ✓
- cli_execute.go <250 LOC ✓
- `go test ./internal/layers/multiagent/external/... -race -count=1` PASS
- t-registry T 编号归属不变

---

## §4 子 Change #1 god-fn-split pt2 (PR-3, T10-T13)

### T10 拆 `external/cursor_adapter.go` 410 LOC

- → `external/cursor_session.go`
- → `external/cursor_execute.go`

### T11 cursor_session.go (目标 <300 LOC)

- 包含：CursorConfig/CursorAgentTool struct + readLoop + CleanupBySessionID + Stop + CloseSession + Info + ExecutionTimeout

### T12 cursor_execute.go (目标 <200 LOC)

- 包含：Execute + handleSystem + handleAssistant + handleThinking + handleToolCall + handleResult + buildCursorRequest (新 helper) + parseCursorStream (新 helper) + emitCursorEvent + formatCursorToolCallLabel + cursorToolCallDetail + truncateCursorDetail

### T13 验证

- cursor_session.go <300 LOC ✓
- cursor_execute.go <200 LOC ✓
- `go test ./internal/layers/multiagent/external/... -race -count=1` PASS
- t-registry T 编号归属不变

---

## §5 子 Change #2 registry-sync (PR-4, T14-T17)

### T14 18 F 路径全替换

参考 `design.md §物理路径映射表` 18 处全替换 — 见 `tasks.md §T14` 详细。

### T15 Historical S 沉 archive

- `openspec/specs/d4-multi-agent/{a,f,t}-registry.md` Legacy S 章节 (D4-S1~S10) 整段下沉
- 新建 `openspec/archive/2026-06-15-devrix-d4-sa-refine/legacy-s1-s10.md` 含 frozen index + 迁移路径表
- d4-domain.md 删除 D4-S1~S10 章节

### T16 d4-domain.md 物理路径表与 code 100% 对齐

- `d4-domain.md §Layer Mapping` 全 18 F 行对齐 code 路径

### T17 同步 `d7-boundary.md` 中 D4 边界描述

- Out of Scope + 委派契约同步

---

## §6 子 Change #3 value-flow-rename (PR-5, T18-T21)

### T18 d4-domain.md §North Star 加 ValueFlow Alias 列

5 S + 1 横切 = 6 alias (T18 详细列表见 `demand.md §1.3`)。

### T19 a-registry 加 ValueFlow Alias block

### T20 f-registry 加 ValueFlow Alias block

### T21 t-registry + layer-delta.md §Canonical S 加 ValueFlow Alias 表

---

## §7 子 Change #4 span-coverage (PR-6, T22-T27)

### T22 `internal/layers/multiagent/orchtypes/events.go` NEW

7 EngineEvent 常量化（与 `boundary_decision.go` 并列治理包）。

### T23 同步 consumer bridge

- `internal/layers/orchestration/executionflow/bridge/agent_bridge.go` L142-154 case switch 走 `orchtypes.EventAgent*` const
- `internal/layers/evolution/guard/observer.go` L52-86 case switch 同

### T24 t-registry 77 T 行加 Span Evidence 列

### T25 `scripts/d4-span-coverage.sh` (NEW, ~80 lines)

- awk 解析 t-registry §2-§9 表格 + grep 6 OpD4_S4_* + 7 EventAgent*
- 守门：effective 覆盖率 ≥80%

### T26 + T27 验证 + diff

- 60+/77 T 映射
- 0 raw 字面量残留

---

## §8 子 Change #5 boundary-decision (PR-7, T28-T31)

### T28-T31 3 boundary debt 全部 RESOLVED

- `internal/layers/multiagent/orchtypes/boundary_decision.go` NEW (~30 lines)
- `boundary_decision_test.go` NEW — 3 单测（Exist + VersionFormat + Unique）
- d4-domain.md §Boundary Debt Decisions 章节登记 3 row + 治理常量位置

---

## §9 S7_Archive (PR-8, T32-T34)

### T32-T34 6 artifacts 复制 + verify-archive 12/12 + d4-domain v1.5.0

---

## §10 关键设计决策

1. **P5 门禁处理**：建立 orchtypes/ 基础 + inline 实际过度包装 + 0 aggressive deletion（实际"dead code"远比声称少）
2. **Historical S 沉 archive**：复用 `openspec/archive/2026-06-15-devrix-d4-sa-refine/` dir
3. **contracts.go 退役策略**：spec v2.0-e 标记，PR-4/5 渐进迁移；**PR-1 不强删**避免破坏 sessionagents/manager.go 等 5+ 测试
4. **Span 覆盖率目标**：≥80% effective（60+/77 T 映射到 6 OpD4_S4_* + 7 EventAgent* const）
5. **ValueFlow Alias 命名**：`D4_` 前缀 + Suffix 用 `_Agent` / `_Worker` / `_Tool`
6. **7 EngineEvent 常量化策略**：新增 `orchtypes/events.go`，consumer 端（agent_bridge.go + observer.go）走 const switch
7. **boundary_decision 治理常量**：`boundary-debt:{name}-v{major}.{minor}` 命名空间，3 decision 全部 RESOLVED

---

**END of Proposal**
