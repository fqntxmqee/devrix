# Delta: Domain D7 (Orchestration)

**Change ID:** devrix-d7-orchestration-domain → current
**Affects:** orchestration, contextengine/tasks, gateway entry, query loop
**Version:** 2.6.0
**Status:** Active
**Last Updated:** 2026-06-15

---

## Current State Summary

D7 编排域 **v1.0 核心目标已完成**：WaveScheduler（D7-S3）+ ExecutionFlowHub（D7-S4）在 `internal/layers/orchestration/{wave,flow,workplan,imsink}/` 完整实现；Session Orchestrator（D7-S2）+ ClassifyIntent/ShadowClassifier（D7-S5 A01/A05）在 `internal/layers/orchestration/coordinator/` 落地（package `coordinator`）；D1 主入口通过 `bootstrap/wire_coordinator.go` 的 `WireD7` 函数切换至 `coordinator.Entry.ProcessMessage`（`d7.enabled=true` 激活）。

D2 Loop 已精简（emit.go、executor.go分流，Attachments/Hooks/SessionQueue 保留）；委托工具保持在 `contextengine/delegate_tools.go`（避免循环依赖）。

**v1.1 待完成：**
- Task 写模型迁入 `orchestration/coordinator/`
- SynthesizeTaskGraph + CreateWorkPlan

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

**Review R1 补充：** OrchestratePath v1.0 不依赖 SynthesizeTaskGraph；并行 execute 委托 S3 Wave。见 `d7-domain.md` Routing Matrix。

#### Scenario: Entry point migration

- GIVEN `orchestration.d7_enabled=true`
- WHEN D1 Gateway RouteInbound runs
- THEN D7-S2-A01 ProcessMessage is called
- AND D2.Process is NOT called directly by D1

**Current:** `gateway.go:286` calls `g.contextEngine.Process`

---

### Requirement: D7 Migration Coexistence

Four-combination regression (`d7_enabled` × `plan.enabled`) MUST pass before defaulting `d7_enabled=true`.

See `d7-domain.md` §Migration Coexistence Contract. T: D7-MIG-T01.

---

### Requirement: D7 Task Model Trinity

Three task representations (PlanTask, WaveTaskNode, BackgroundRun) MUST remain separate in v1.0 with unified QueryWorkPlan. T: D7-S1-T07.

**Current:** PlanTask in D2 tasks/; Wave in orchestration/wave/; Background in query/background.go

---

`query.Loop.Run` MUST be ≤200 lines with no D4/queue imports.

#### Scenario: Orchestration fields removed

- GIVEN post-migration loop.go
- THEN EnsureParallelAsyncBatch, WaitPendingAsyncBatch, SessionQueue, AfterToolRound MUST NOT exist

**Current:** Loop.Run ~203 lines (target ≤200), Attachments/Hooks/SessionQueue removed, no multiagent/queue imports. Refactored into loop.go (264 lines) + emit.go (33 lines) + executor.go (81 lines).

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
