# Delta: Domain D7 (Orchestration)

**Change ID:** devrix-d7-orchestration-domain → current
**Affects:** orchestration, contextengine/tasks, gateway entry, query loop
**Version:** 2.0.0
**Status:** Active
**Last Updated:** 2026-06-14

---

## Current State Summary

D7 编排域处于 **PARTIAL** 状态：WaveScheduler（D7-S3）与 ExecutionFlowHub（D7-S4）已在 `internal/layers/orchestration/` 完整实现并通过 13 个 T 测试点验证；Task/Plan 写模型（D7-S1）与 PlanMode（D7-S5 部分）仍托管在 `contextengine/tasks/`；Session Orchestrator（D7-S2）与 `internal/layers/d7/` 包尚未创建。D1 主入口仍为 `D2.ContextEngine.Process`。

---

## IMPLEMENTED (现行代码)

### Requirement: D7-S3 Wave Scheduler

`WaveScheduler` MUST provide DAG scheduling with 5-slot WorkerPool, ConflictGuard, and three ContextPolicy modes.

**Package:** `internal/layers/orchestration/wave/`
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

**Package:** `internal/layers/orchestration/flow/hub.go`

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

### Requirement: D7-S1 Task Manager (D2 托管)

`TaskManager` MUST provide in-memory + optional disk-backed Task CRUD per session.

**Package:** `internal/layers/contextengine/tasks/task_manager.go`

#### Scenario: Disk persistence on v2 mode

- GIVEN `tasks.mode=v2` and valid store_dir
- WHEN Task is created and process restarts
- THEN TaskManager.EnsureSession reloads tasks from disk

---

### Requirement: D7-S5 Plan Mode (D2 托管)

PlanMode MUST support `/plan` workflow with read-only PlanAgent exploration.

**Package:** `internal/layers/contextengine/tasks/plan_mode.go`

#### Scenario: Plan mode lifecycle

- GIVEN inactive PlanMode
- WHEN Enter is called with goal
- THEN state becomes active
- AND PlanAgent executes in read-only mode

---

## PARTIAL (实现不完整)

### Requirement: D7-S1 Unified Work Model

**Gap:** Task 写模型在 D2，WorkPlan 读模型在 ORCH；无统一 `Plan` 聚合根与 `CreateWorkPlan` Activity。

| 已有 | 缺失 |
|------|------|
| Task CRUD + 依赖 | Plan 实体与 DAG 校验 Activity |
| DiskStore 持久化 | D7-S1-A01 CreateWorkPlan |
| PlanMode 状态机 | BackgroundTask 注册迁入 D7 |

---

### Requirement: D7-S5 Decision Layer

**Gap:** 仅有 PlanAgent 只读探索，无意图分类与自动任务拆解。

| 已有 | 缺失 |
|------|------|
| PlanAgent + VerificationAgent 设计 | ClassifyIntent 规则+LLM |
| `/plan` CLI 命令 | SynthesizeTaskGraph |
| TaskManager 手动创建 | SelectExecutor (D2/D4 路由) |

---

## PLANNED (D7 v1.0 迁移)

### Requirement: D7-S2 Session Orchestrator

D1 `RouteInbound` MUST call `D7.ProcessMessage` instead of `D2.Process`.

#### Scenario: Entry point migration

- GIVEN `orchestration.d7_enabled=true`
- WHEN D1 Gateway RouteInbound runs
- THEN D7-S2-A01 ProcessMessage is called
- AND D2.Process is NOT called directly by D1

**Current:** `gateway.go:286` calls `g.contextEngine.Process`

---

### Requirement: D2 Thin QueryLoop

`query.Loop.Run` MUST be ≤200 lines with no D4/queue imports.

#### Scenario: Orchestration fields removed

- GIVEN post-migration loop.go
- THEN EnsureParallelAsyncBatch, WaitPendingAsyncBatch, SessionQueue, AfterToolRound MUST NOT exist

**Current:** 414 lines, imports `multiagent/delegate`

---

### Requirement: D7 Package Identity

`internal/layers/d7/` MUST exist as top-level domain package.

**Current:** Directory does not exist; code at `internal/layers/orchestration/`

---

## REMOVED / RETIRED

| 项 | 说明 |
|----|------|
| PEV Plan Engine | 2026-06-13 退役；由 PlanMode + TaskManager 替代 |
| ORCH 作为"纯读模型"定位 | 2026-06-13 升格为 D7 域（部分实现） |

---

## Migration Checklist (D7 v1.0)

| Phase | 内容 | 状态 |
|-------|------|------|
| Phase 1 | D7 域定义 + A/F/T 注册表 | ✅ 文档完成 |
| Phase 2 | D7-S1 Work Model 统一 | ⬜ Task 仍在 D2 |
| Phase 3 | D7-S5 Decision & Planning | ⬜ 仅 PlanMode |
| Phase 4 | D7-S2 Session Orchestrator | ⬜ 入口未上移 |
| Phase 5 | D7-S3/S4 包路径迁移 | ⬜ 仍在 orchestration/ |
| Phase 6 | D2 瘦身 | ⬜ loop.go 414 行 |
| Phase 7 | 回归验收 P0 T 全绿 | ⬜ D7-S2/S5 T 未实现 |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始 D7 delta（设计阶段） |
| 2.0.0 | 2026-06-14 | 对齐代码审计：IMPLEMENTED/PARTIAL/PLANNED 三分 |
