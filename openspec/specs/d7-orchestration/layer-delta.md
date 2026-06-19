# Delta: Domain D7 (Orchestration)

**Change ID:** devrix-d7-orchestration-domain → current
**Affects:** orchestration, contextengine/tasks, gateway entry, query loop
**Version:** 4.0.0
**Status:** Active
**Last Updated:** 2026-06-19

---

## Current State Summary

D7 编排域 **v2.0 Structure 闭环**（DM-20260619-005）：物理路径与 S 层 1:1 对齐——S2 `sessionorchestrator/`、S3 `wavescheduler/`、S4 `executionflow/{hub,workplan,imsink,bridge}/`、S5 `decisionplanning/`；`coordinator/` 与 `hubspoke/` 保留 1-release type-alias shim；`orchtypes/` 承载共享 Config/Intent 类型。WaveScheduler（D7-S3）+ ExecutionFlowHub（D7-S4）完整实现；Session Orchestrator（D7-S2）+ Turn Leader（A06/A07）在 `sessionorchestrator/` + `turn/`；ClassifyIntent/ShadowClassifier/LLM Decomposer（D7-S5）在 `decisionplanning/`；D1 主入口经 `sessionorchestrator.Entry.ProcessMessage`（`coordinator.Entry` shim，`bootstrap/wire_coordinator.go::WireD7`）。WorkTree TD-WT-02/03 部分闭合；t-registry 66/66 IMPLEMENTED。

### S 层博弈角色（切法 A — 按用户价值流）

> **基于 `devrix-d7-sa-refine` (DM-20260614-008)**

| S 层 | 博弈角色 | North Star |
|------|---------|------------|
| D7-S2 | **Screening Mechanism** + **Turn Leader (Stackelberg)** | 用户消息统一入口 + Turn 主循环；S2 = Meta-Orchestrator 跨 S3/S4/S5 |
| D7-S3 | **Mechanism Designer** | 多任务并行执行，冲突避免，上下文隔离 |
| D7-S4 | **Costly Signaler** | 执行进度透明，WorkPlan 可追溯 |
| D7-S5 | **Information Producer** | 把用户 goal 转化为可执行的任务结构 |
| D7-S1 | **State Authority**（非博弈角色） | Task/Plan 持久化与状态机；产"事实"而非"决策" |

---

## IMPLEMENTED (现行代码)

### Requirement: D7-S3 Wave Scheduler

`WaveScheduler` MUST provide DAG scheduling with 5-slot WorkerPool, ConflictGuard, and three ContextPolicy modes.

**Package:** `internal/layers/orchestration/wavescheduler/`
**DSAFT:** D7-S3-A01 ScheduleWave, D7-S3-A02 ResolveWorkerContext, D7-S3-A03 GuardConflict

#### Scenario: Peak concurrency capped at 5

- GIVEN 6 ready subagent tasks and 1 cursor task
- WHEN WaveScheduler runs continuous dispatch
- THEN peak running workers ≤ 5
- AND cursor=1, claude_code=1, subagent=3 caps are respected

#### Scenario: Upstream context receives artifact only

- GIVEN TaskNode with `context_policy=upstream`
- WHEN ContextResolver.Resolve runs
- THEN SystemPrompt includes upstream artifact summary
- AND Messages do NOT contain Leader conversation history

#### Scenario: Fresh context is directive-only

- GIVEN TaskNode with `context_policy=fresh`
- WHEN ContextResolver.Resolve runs
- THEN Messages contain only the user directive message

---

### Requirement: D7-S4 Execution Flow Hub

`Hub` MUST implement `contracts.ExecutionFlowHub` with WorkPlan aggregation and dual-channel fan-out.

**Package:** `internal/layers/orchestration/executionflow/hub/hub.go`

#### Scenario: WorkPlan snapshot includes flows

- GIVEN FlowStarted applied to workplan.Service
- WHEN Hub.Snapshot is called
- THEN ExecutionFlows include status `running` and RecentEvents

#### Scenario: Task projection in snapshot

- GIVEN link_tasks enabled and TaskManager has session tasks
- WHEN Hub.Snapshot is called
- THEN TaskSnapshots include id, subject, status, owner

#### Scenario: IM worker progress emission

- GIVEN im_progress enabled and IMSink wired
- WHEN FlowStarted is published
- THEN GatewaySink emits worker_progress EngineEvent with render=worker_tree

---

### Requirement: D7-S1 Task Manager (已迁入 D7)

`TaskManager` MUST provide in-memory + optional disk-backed Task CRUD per session with state machine guards.

**Package:** `internal/layers/orchestration/workmodel/task_manager.go`
**DSAFT:** D7-S1-A02

#### Scenario: Disk persistence on v2 mode

- GIVEN `tasks.mode=v2` and valid store_dir
- WHEN Task is created and process restarts
- THEN TaskManager.EnsureSession reloads tasks from disk

#### Scenario: Illegal transition rejection

- GIVEN a Task in completed status
- WHEN UpdateStatus to pending is attempted
- THEN ErrIllegalTransition is returned
- AND status is unchanged

---

### Requirement: D7-S5 Plan Mode (已迁入 D7)

PlanMode MUST support `/plan` workflow with read-only PlanAgent exploration.

**Package:** `internal/layers/orchestration/workmodel/plan_mode.go`
**DSAFT:** D7-S1-A04

#### Scenario: Plan mode lifecycle

- GIVEN inactive PlanMode
- WHEN Enter is called with goal
- THEN state becomes active
- AND PlanAgent executes in read-only mode

---

## PARTIAL (实现不完整)

### Requirement: D7-S1 Unified Work Model

**状态:** v1.2 全部完成 ✅。

| 已有 | 状态 |
|------|------|
| Task CRUD + 依赖（workmodel/task_manager.go） | ✅ |
| DiskStore 持久化（workmodel/disk_store.go） | ✅ |
| Plan 实体 + CreateWorkPlan（coordinator/workmodel.go） | ✅ |
| v1.2 LocalWorkModel + BackgroundProvider | ✅ |
| TaskStatus 状态机守卫（IsLegalTransition） | ✅ |
| BackgroundTask 注册与 QueryWorkPlan 可见 | ✅ |

---

### Requirement: D7-S5 Decision Layer

**状态:** v1.1 全部完成。

| 已有 | 状态 |
|------|------|
| ClassifyIntent 规则版（classifier.go） | ✅ |
| ClassifyIntent LLM fallback（classifier_fallback.go） | ✅ |
| SynthesizeTaskGraph 规则版（decomposer.go） | ✅ |
| SynthesizeTaskGraph LLM-based 拆解（decomposer.go） | ✅ |
| SelectExecutor D2/D4 路由（executor.go） | ✅ |

---

## PLANNED (D7 v1.0 迁移)

### Requirement: D7-S2 Session Orchestrator

D1 `RouteInbound` MUST call `D7.ProcessMessage` instead of `D2.Process`.

**Review R1 补充：** OrchestratePath v1.0 不依赖 SynthesizeTaskGraph；并行 execute 委托 S3 Wave。见 `d7-requirements-clarifications.md` Routing Matrix。

#### Scenario: Entry point migration

- GIVEN `orchestration.d7_enabled=true`
- WHEN D1 Gateway RouteInbound runs
- THEN D7-S2-A01 ProcessMessage is called
- AND D2.Process is NOT called directly by D1

**Current:** `gateway.go:286` calls `g.contextEngine.Process`

---

### Requirement: D7 Migration Coexistence

Four-combination regression (`d7_enabled` × `plan.enabled`) MUST pass before defaulting `d7_enabled=true`.

See `d7-requirements-clarifications.md` §Migration Coexistence Contract. T: D7-MIG-T01.

---

### Requirement: D7 Task Model Trinity

Three task representations (PlanTask, WaveTaskNode, BackgroundRun) MUST remain separate in v1.0 with unified QueryWorkPlan. T: D7-S1-T07.

**Current:** PlanTask in D2 tasks/; Wave in orchestration/wave/; Background in query/background.go

---

`query.Loop.Run` MUST be ≤200 lines with no D4/queue imports.

**Current:** ✅ IMPLEMENTED. loop.go 170 行（符合 ≤200 行目标），`LoopHooks` 结构体已删除，4 个编排字段已迁出（`PlanMode`/`TaskManager`/`Orchestration`/`Hub`），无 multiagent/queue imports。D7-THIN-T01/T02 闭环。

---

### Requirement: D7 Package Identity

`internal/layers/orchestration/coordinator/` MUST exist as D7 coordinator sub-package.

**Current:** IMPLEMENTED 2026-06-14 — package `coordinator` lives at `internal/layers/orchestration/coordinator/` (16 files: types/contracts/config/classifier/fastpath/orchestrator/interrupt/workmodel/helpers/shadow_classifier/d6_metrics + tests).

---

## REMOVED / RETIRED

| 项 | 说明 |
|----|------|
| PEV Plan Engine | 2026-06-13 退役；由 PlanMode + TaskManager 替代 |
| ORCH 作为"纯读模型"定位 | 2026-06-13 升格为 D7 域（部分实现） |

---

## v2.0-Structure（DM-20260619-005）— IMPLEMENTED

| Phase | 内容 | 状态 |
|-------|------|------|
| A | 规格同步（design/layer-delta/d7-boundary/code-layout/a-registry） | ✅ |
| B1 | `wave/` → `wavescheduler/` | ✅ |
| B2 | S4 收敛 `executionflow/{hub,workplan,imsink}/` | ✅ |
| B3 | `coordinator` 拆 `sessionorchestrator` + `decisionplanning` + `orchtypes` | ✅ |
| B4 | `hubspoke` 拆 dispatch→S2、bridge→S4 | ✅ |
| C | WorkTree TD-WT-02/03 部分闭合 | ✅ PARTIAL |

**Shim（1 release）：** `coordinator/aliases.go`、`hubspoke/aliases.go` 重导出类型与构造函数；新代码应直接 import 目标包。

---

## HISTORICAL（PLANNED 已闭合）

以下条目在 v1.x 标为 PLANNED/PARTIAL，v2.0 Structure 后归入 HISTORICAL：

| 项 | 原状态 | 现态 |
|----|--------|------|
| D7-S2/S5 同包 `coordinator/` | PLANNED 拆分 | ✅ 已拆至 sessionorchestrator + decisionplanning |
| `wave/` / `flow/` Legacy 路径 | 迁移中 | ✅ 已迁至 wavescheduler / executionflow |
| hubspoke 整包 | 待拆分 | ✅ dispatch→S2，bridge→S4 |
| TaskNode 独立 SoT | PARTIAL | 🔶 TD-WT-02 投影化（TD-WT-01/04/05/06 defer） |

---

## Migration Checklist (D7 v1.0 — Review R1)

| Phase | 内容 | 状态 |
|-------|------|------|
| 1 / A | 域定义 + Review R1 澄清（demand/review-r1/tasks） | ✅ |
| 5 | D7-S3/S4 实现（ORCH 代码） | ✅ |
| B | D7 骨架 + contracts + re-export + bootstrap | ✅ |
| C | S5-P2 Classify + S2 ProcessMessage | ✅ |
| D | 入口切换（WireD7 + SetOrchestrationEntry） | ✅ |
| E | D2 Loop 精简（emit/executor分流，Loop.Run 196行） | ✅ |
| F | S3/S4 包路径迁移 | ✅ |
| G | 回归 + D7-MIG-T01 四组合矩阵 | ✅ |
| H (v1.1) | S5-P3 SynthesizeTaskGraph + CreateWorkPlan | ✅ DONE |
| I (v1.1) | Task 写模型迁入 coordinator | ✅ DONE |
| J (v1.1) | DiskStore 持久化迁入 D7 | ✅ DONE |
| K (v1.1) | SelectExecutor D2/D4 路由 | ✅ DONE |
| L (v1.1) | ClassifyIntent LLM fallback | ✅ DONE |
| M (v1.1) | SynthesizeTaskGraph LLM-based 拆解 | ✅ DONE |
| N (v1.2 + v2.0) | Task 状态机守卫 + 置信度阈值 + Turn Leader + LLM Invoker | ✅ DONE |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始 D7 delta（设计阶段） |
| 2.0.0 | 2026-06-14 | 对齐代码审计：IMPLEMENTED/PARTIAL/PLANNED 三分 |
| 2.1.0 | 2026-06-14 | Review R1 决议同步：路由矩阵、三模型、迁移契约 |
| 2.2.0 | 2026-06-15 | WireD7 bootstrap 实现完成；D2 Loop 瘦身待进行 |
| 2.3.0 | 2026-06-14 | Task 写模型迁入 coordinator (Phase I 完成)；CreateWorkPlan 基础版 (Phase H 进行中) |
| 2.4.0 | 2026-06-14 | SynthesizeTaskGraph 规则版实现 (D7-S5-A02)；Phase H 全部完成 |
| 2.5.0 | 2026-06-14 | v1.1 全部完成：DiskStore 持久化、SelectExecutor、LLM fallback、LLM-based 拆解 |
| 2.6.0 | 2026-06-15 | DM-020 Turn Leader wired + Hub-Spoke SoT + D2 Thin 进行中 |
| 3.0.0 | 2026-06-16 | **v1.2 + v2.0-b/c/f 全部闭环** |
| **4.0.0** | **2026-06-19** | **v2.0 Structure（DM-20260619-005）**：物理路径对齐 S 层；coordinator/hubspoke shim；WorkTree TD-WT-02/03 部分闭合 |

---

## Docs Sync（2026-06-16）

领域规格同步（`openspec/specs/d7-orchestration/`，无代码变更）：

| 新增/更新 | 文件 | 说明 |
|----------|------|------|
| REWRITE | `d7-domain.md` v1.0.0 | 薄领域 SoT（对齐 D1 `*-domain.md` 模式） |
| ADDED | `terminal-state-guide.md` | IntentKind 四链、A→F 编排树、跨域时序 |
| RENAMED | `d7-requirements-clarifications.md` | 原厚 `d7-domain.md`（Review R1/R2 + 历史 Gherkin） |
| UPDATED | `spec.md` v3.1.0 | Domain SoT 指针；域边界 LLM 产权修正 |
| UPDATED | `a-registry.md` v3.5.0 | Canonical 段补登 S1；24 A 统计 |
| UPDATED | `design.md` / `t-registry.md` / `d3-boundary.md` | 文档索引交叉引用 |
| UPDATED | `dsaft-architecture.md` | 收敛为 Stub v2.0.0 |
| ADDED | `observability-guide.md` | Span↔T、Trace 树、Hub Flow、P0 Runbook |
| UPDATED | `span-registry.md` | Guides 指针 |
