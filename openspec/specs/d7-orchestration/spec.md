# D7 Orchestration Domain Specification

**Capability:** d7-orchestration
**Domain:** D7
**DSAFT Type:** 核心域 (Core Domain)
**Version:** 3.9.0
**Status:** Canonical — source of truth
**Last Updated:** 2026-06-19
**Domain SoT:** `d7-domain.md`
**Layering Spec:** `openspec/specs/architecture/layering.md`
**Change ID:** devrix-d7-orchestration-domain (DM-20260613-001)
**Demand:** `openspec/changes/devrix-d7-orchestration-domain/demand.md`
**Review R1:** `openspec/changes/devrix-d7-orchestration-domain/review-r1.md`
**Review R2:** `openspec/changes/devrix-d7-orchestration-domain/review-r2.md`

**Archived Changes:** devrix-queryloop-context (2026-06-10, ORCH v2 read model), devrix-wave-scheduler (WaveScheduler), devrix-d7-uncertainty-gaps (2026-06-16, DM-20260616-001, 5 gap fixes)

---

## Overview

D7 编排域回答 **"做什么、按什么顺序做、谁来做、做得怎么样了"**。作为 **横向协调层** 编排 D2（LLM↔Tool 执行原语）与 D4（Agent 委托原语），并向 D1 发布进度事件（D1 仍拥有 ingress）。

**现行实现路径（2026-06-19）：** v2.0 Structure（DM-20260619-005）物理路径与 S 层 1:1 对齐：S2 `sessionorchestrator/`、S3 `wavescheduler/`、S4 `executionflow/{hub,workplan,imsink,bridge}/`、S5 `decisionplanning/`；`coordinator/` 与 `hubspoke/` 保留 type-alias shim。D1 主入口 `sessionorchestrator.Entry.ProcessMessage`（`coordinator.Entry` shim，`bootstrap/wire_coordinator.go::WireD7`）。Intent 四链正交分发不变（CommandHandler / FastPath / OrchestratePath / Skip）。

### S 层博弈角色定义（切法 A — 按用户价值流）

> **基于 `devrix-d7-sa-refine` (DM-20260614-008) + DM-020 D7 Turn 编排上移 (DM-20260614-020)**

| S 层 | 博弈角色 | North Star |
|------|---------|------------|
| D7-S2 | **Screening Mechanism** + **Turn Leader (Stackelberg)** | 用户消息统一入口 + Turn 主循环；S2 = Meta-Orchestrator 跨 S3/S4/S5 |
| D7-S3 | **Mechanism Designer** | 多任务并行执行，冲突避免，上下文隔离 |
| D7-S4 | **Costly Signaler** | 执行进度透明，WorkPlan 可追溯 |
| D7-S5 | **Information Producer** | 把用户 goal 转化为可执行的任务结构 |
| D7-S1 | **State Authority**（非博弈角色） | Task/Plan 持久化与状态机；产"事实"而非"决策" |

| 版本里程碑 | 能力 |
|-----------|------|
| ORCH v1.0 (2026-06-10) | Hub-Spoke 读模型：WorkPlan + ExecutionFlowHub |
| ORCH v1.1 (2026-06-10) | WaveScheduler：DAG 调度、5-slot WorkerPool、ConflictGuard |
| D7 v0.5 (2026-06-13) | DSAFT 域定义、A/F/T 注册表、迁移设计（S3 规划） |
| D7 v1.0 (目标) | 入口上移、D2 瘦身、Task 模型归 D7-S1、S5-P2 分类 |
| D7 v2.1.0 (文档) | Review R1 澄清：三模型、路由矩阵、迁移契约 |
| D7 v2.2.0 (文档) | Review R2：D7-D1 权力分配、HandleInterrupt 顺序、T02c |
| D7 v2.3.0 (2026-06-15) | v1.0 + v1.1 closure：S2 Turn Leader 角色补登 + S1 State Authority 标注；DSAFT 结构 + Scenarios 表 IMPLEMENTED 状态刷新 |

---

## Review R1 澄清摘要（2026-06-14）

完整条文见 `d7-requirements-clarifications.md` §Requirements Clarifications 与 `demand.md`。

| 主题 | 决议 |
|------|------|
| Task 模型 | 三模型职责分离（PlanTask / WaveTaskNode / BackgroundRun），v1.0 不合并存储 |
| S2 vs S3 | 编排路由矩阵：并行 execute 归 S3 Wave，S2 不串行替代 |
| S5 | 分阶段：v1.0 仅 P1(PlanMode)+P2(Classify)；自动拆解 v1.1 |
| 迁移 | `d7.enabled` 默认 true；legacy D1→D2 已退役（DM-20260614-007） |
| 性能 | FastPath 拆 T02a(proxy≤2ms) + T02b(classify≤1ms) + T02c(端到端≤2ms) |
| 配置 | 单一 SoT：`context_engine.tasks.store_dir` |

---

## Review R2 结构层决议（2026-06-14）

完整条文见 `review-r2.md`。

| 主题 | 决议 |
|------|------|
| D7-D1 权力 | D1 ingress owner；D7 routing decision owner；`d7_enabled` 否决权 |
| HandleInterrupt | /stop：Wave→D4→Process→stopped→TaskCancel；正常 Process 结束不杀 Wave |
| OQ-1~4 | 全部闭合（见 review-r1.md §2） |
| D6 advisory | P1 补 `orchestration.d6.validation.*` metric |
| S5 shadow | P1 tail-only LLM classify（规则未命中），为 v1.1 兜底准备 |

---

## DSAFT 结构

| 层级 | ID | 名称 | 说明 | 实现状态 |
|------|-----|------|------|----------|
| D | D7 | Orchestration | 跨域编排协调层 | **IMPLEMENTED**（v1.0 + v1.1 闭环） |
| S | D7-S1 | Work Model | Task/Plan 数据模型与生命周期 | **IMPLEMENTED** → `workmodel/` + `sessionorchestrator/workmodel.go` |
| S | D7-S2 | Session Orchestrator | 用户消息主入口、Turn 主循环、Dispatch | **IMPLEMENTED** → `sessionorchestrator/` + `turn/` |
| S | D7-S3 | Wave Scheduler | DAG 调度、WorkerPool、ConflictGuard | IMPLEMENTED → `wavescheduler/` |
| S | D7-S4 | Execution Flow | FlowEvent 聚合、WorkPlan 快照、IM 广播 | IMPLEMENTED → `executionflow/` |
| S | D7-S5 | Decision & Planning | 意图分类、任务拆解、执行器选择 | **IMPLEMENTED** → `decisionplanning/` |

---

## Scenarios

| ID | Scenario | Responsibility | Status | 代码位置 |
|----|----------|----------------|--------|----------|
| D7-S1 | Work Model | Task CRUD、依赖 DAG、磁盘持久化、PlanMode 状态机 | **IMPLEMENTED** | `workmodel/` + `sessionorchestrator/workmodel.go` |
| D7-S2 | Session Orchestrator | ProcessMessage、FastPath、HandleInterrupt、TurnLoop、InvokeLLM、Dispatch | **IMPLEMENTED** | `sessionorchestrator/` + `turn/` |
| D7-S3 | Wave Scheduler | TaskGraph DAG、5-slot 池、ContextPolicy、ConflictGuard | IMPLEMENTED | `wavescheduler/` |
| D7-S4 | Execution Flow | Hub 双通道发布、WorkPlan 读模型、IM worker_progress、SpokeBridge | IMPLEMENTED | `executionflow/{hub,workplan,imsink,bridge}/` |
| D7-S5 | Decision & Planning | PlanAgent 只读探索、规则+LLM 分类、SynthesizeTaskGraph、SelectExecutor | **IMPLEMENTED** | `decisionplanning/` + `workmodel/plan_*.go` |

---

## Architecture

```
D1 Gateway.RouteInbound
    └── D7-S2 SessionOrchestrator.ProcessMessage    ← v1.0 主入口（wired by wire_coordinator.go::WireD7）
            ├── D7-S2-A02 ClassifyIntent (rule + LLM fallback)
            ├── switch intent.Kind (v1.1.0+ orthogonal dispatch):
            │     ├─ IntentSkip        → close channel
            │     ├─ IntentCommand     → CommandHandler.Handle
            │     │                       ├─ /plan → PlanCLICommands → PlanMode
            │     │                       ├─ /task → CLICommands → TaskManager
            │     │                       └─ /help, /stop → explicit handlers
            │     ├─ IntentFast        → FastPath.Run → TurnOrchestrator → D3 (LLM) + D2 (tools/persist)
            │     └─ IntentOrchestrate → OrchestratePath.Run
            │                            ├─ TaskDecomposer.SynthesizeTaskGraph (D7-S5-A02)
            │                            ├─ WaveScheduler.Start (D7-S3-A01)
            │                            └─ WaveScheduler.WaitForCompletion (D7-S3)
            ├── D7-S2-A06 RunTurnLoop → D7-S2-A07 InvokeLLM → D3 (LLM Gateway)
            │                            → D2 (ContextPreparer / ToolRoundExecutor / SessionPersister)
            ├── D7-S2-A04 DispatchWorker → hubspoke.Dispatcher → D4 Worker / D2 SubQuery
            └── flow.GlobalHub.Publish    ← D7-S4 读模型入口
                    ├── workplan.Service.Apply
                    ├── queue.SessionQueue (delegate-progress)
                    └── imsink.GatewaySink (worker_progress)

D4 Delegate.Service
    └── FlowBridge → flow.GlobalHub.Publish

WaveScheduler (独立调用路径，由 delegate_tools / Plan 触发)
    ├── TaskGraph.ReadyNodes
    ├── WorkerPool.Acquire (cursor=1, claude_code=1, subagent=3)
    ├── ConflictGuard.Allow
    ├── ContextResolver.Resolve (fresh|resume|upstream)
    └── WorkerRunner.Run → ArtifactStore
```

### 域边界

| D7 拥有 | D7 编排（不拥有） | D7 不拥有 |
|---------|------------------|----------|
| WorkPlan 读模型（D7-S4） | D7 RunTurn / D2 Prepare | 会话上下文（D2） |
| Wave DAG 调度（D7-S3） | D4 Delegate RunAgent | Agent 生命周期（D4） |
| FlowEvent 契约（contracts） | — | LLM 调用（D3） |
| Task/Plan 写模型（D7-S1） | | |

---

## ADDED Requirements

### Requirement: D7-S3 Wave Scheduler

`WaveScheduler` MUST provide DAG-based multi-agent scheduling with fixed WorkerPool capacity, ConflictGuard, and ContextPolicy resolution.

**Priority:** P0  
**Package:** `internal/layers/orchestration/wavescheduler/`  
**T:** D7-S3-T01 … D7-S3-T10

#### Scenario: DAG ready-node dispatch

- GIVEN a TaskGraph with dependency edges
- WHEN `ReadyNodes()` is evaluated
- THEN only nodes whose dependencies are `completed` and self state is `pending` are returned
- AND dispatch order is deterministic (sorted by id)

#### Scenario: Worker pool capacity

- GIVEN default `DefaultPoolCapacity`
- WHEN slots are acquired concurrently
- THEN peak running ≤ 5 (cursor=1, claude_code=1, subagent=3)
- AND slot release triggers immediate re-dispatch (D2 continuous loop)

#### Scenario: Conflict group mutual exclusion

- GIVEN two TaskNodes sharing the same `conflict_group`
- WHEN both are ready
- THEN at most one runs concurrently

#### Scenario: Context policy isolation

- GIVEN `context_policy=fresh`
- WHEN ContextResolver resolves
- THEN Messages contain only the directive (no Leader history)

- GIVEN `context_policy=upstream` and upstream artifact exists
- WHEN ContextResolver resolves
- THEN SystemPrompt includes upstream summary (no Leader history)

---

### Requirement: D7-S4 Execution Flow Hub

`ExecutionFlowHub` MUST aggregate `FlowEvent` from D2 SubQuery and D4 Delegate into WorkPlan snapshots, enqueue Leader delegate-progress, and optionally emit IM worker_progress.

**Priority:** P0  
**Package:** `internal/layers/orchestration/executionflow/hub/hub.go`  
**Contract:** `internal/shared/contracts/execution_flow.go`  
**T:** D7-S4-T01 … D7-S4-T04

#### Scenario: Dual publish on flow event

- GIVEN `execution_flow.enabled=true` with `link_tasks` and `im_progress`
- WHEN Hub.Publish receives FlowStarted
- THEN WorkPlan is updated via `workplan.Service.Apply`
- AND SessionQueue receives delegate-progress for Leader drain
- AND IM sink receives worker_progress when configured

#### Scenario: WorkPlan snapshot

- GIVEN FlowStarted and TaskManager updates for a session
- WHEN Hub.Snapshot is called
- THEN response includes ExecutionFlows with status and RecentEvents
- AND linked Task snapshots reflect owner and in_progress status

#### Scenario: Flow event lifecycle kinds

- GIVEN an active SubQuery or D4 worker
- WHEN FlowEvent is published
- THEN kinds include `started`, `completed`, `failed`, `tool_call`, `iterating`, `joined`
- AND each event is timestamped with actual occurrence time

#### Scenario: Tool call throttle

- GIVEN rapid FlowToolCall events for the same worker
- WHEN throttle window (`tool_summary_throttle_ms`, default 500ms) not elapsed
- THEN duplicate tool_call events are suppressed from publication

---

### Requirement: D7-S1 Work Model ✅ IMPLEMENTED

`TaskManager` MUST provide session-scoped Task CRUD with optional disk persistence and dependency tracking. PlanMode MUST support inactive → active → pending_approval lifecycle.

**Priority:** P0
**Package:** `internal/layers/orchestration/workmodel/` + `sessionorchestrator/workmodel.go`（v1.1 闭环，layer-delta Phase I/J）
**T:** D7-S1-T01 … D7-S1-T08（其中 T06 升 IMPLEMENTED via decomposer_test.go::validateGraph）

#### Scenario: Task create and persist

- GIVEN `tasks.mode=v2` and `store_dir` configured
- WHEN TaskManager.Create is called
- THEN a Task is created with unique ID and status `pending`
- AND the Task is persisted to disk when store is enabled

#### Scenario: Task-flow linkage

- GIVEN `execution_flow.link_tasks=true`
- WHEN Hub.Publish FlowStarted with TaskID
- THEN TaskManager sets owner and status `in_progress`
- AND FlowCompleted/FlowFailed transitions Task to terminal status

---

### Requirement: D7-S5 Plan Mode ✅ IMPLEMENTED

PlanMode MUST support `/plan` command workflow: enter → explore (read-only) → pending_approval → approve/reject.

**Priority:** P1
**Package:** `internal/layers/orchestration/workmodel/{plan_mode,plan_agent}.go`（v1.1 迁入；白名单测试在 `plan_agent_whitelist_test.go` 10 个 AC）
**T:** D7-S5-T01 … D7-S5-T05（含 T04 SynthesizeTaskGraph + T05 SelectExecutor，均已 IMPLEMENTED）
**Design:** `task-planning-design.md`

#### Scenario: Plan mode state machine

- GIVEN inactive PlanMode
- WHEN `/plan <goal>` is invoked
- THEN state transitions to `active`
- AND PlanAgent runs in read-only mode
- AND on completion state becomes `pending_approval`

---

## PLANNED Requirements (D7 v1.0 迁移)

**Status: v1.0 + v1.1 全闭环（2026-06-15）。** 以下条目仅作历史追溯，新功能请遵循 v2.0+ 路线（DM-018 Hub-Spoke + DM-020 Turn Leader 已 wired）。

| Requirement | 目标 | v1.0 / v1.1 状态 |
|-------------|------|------------------|
| D7-S2 ProcessMessage | D1→D7 入口上移 | ✅ IMPLEMENTED |
| D7-S5-P2 ClassifyIntent | 规则 + command-first + LLM fallback | ✅ IMPLEMENTED |
| D7-S5-P3 SynthesizeTaskGraph | 目标拆解为 DAG（规则 + LLM） | ✅ IMPLEMENTED |
| D7-S5 SelectExecutor | D2/D4 执行器选择 | ✅ IMPLEMENTED |
| D2 Thin / QueryLoop removed | loop 已删；D2 无 D4 import | ✅ DM-20260618-010 |
| D7 package identity | `sessionorchestrator/` + `decisionplanning/` + `orchtypes/`；`coordinator/` shim（DM-20260619-005） | ✅ IMPLEMENTED |
| D7 Migration Coexistence | 4 组合回归 | ✅ IMPLEMENTED |
| D7-S2 Turn Leader (DM-020) | A06 RunTurnLoop + A07 InvokeLLM | ✅ IMPLEMENTED（wired by `wire_coordinator.go`） |
| D7-S2 Hub-Spoke (DM-018) | A04 DispatchWorker + A04/A05 SpokeBridge | ✅ IMPLEMENTED（wired by `delegate.go`） |
| D7-S1 WorkModel 迁入 | 写模型从 D2 迁入 D7 | ✅ IMPLEMENTED |

---

### Requirement: PlanAgent Read-Only Sandbox (devrix-d7-uncertainty-gaps)

`PlanAgent` MUST enforce tool call whitelist at runtime, not only via prompt. `ValidateToolCall()` checks against the read-only whitelist and forbidden list, failing closed on unknown tools.

**Priority:** P0
**Package:** `internal/layers/orchestration/workmodel/plan_agent.go`
**T:** D7-S5-A02-F01-T01 … D7-S5-A02-F01-T04

#### Scenario: Allowed tool passes validation

- GIVEN a PlanAgent with the default read-only whitelist
- WHEN `ValidateToolCall` is called with `"read"`
- THEN no error is returned

#### Scenario: Forbidden tool is rejected

- GIVEN a PlanAgent with the default read-only whitelist
- WHEN `ValidateToolCall` is called with `"write"`
- THEN an error is returned containing `"forbidden in plan mode"`

#### Scenario: Unknown tool is rejected

- GIVEN a PlanAgent with the default read-only whitelist
- WHEN `ValidateToolCall` is called with `"unknown_tool"`
- THEN an error is returned containing `"not in the plan mode read-only whitelist"`

#### Scenario: Nil PlanAgent passes through

- GIVEN a nil PlanAgent
- WHEN `ValidateToolCall` is called with `"write"`
- THEN no error is returned (passthrough: no sandbox without PlanAgent)

---

### Requirement: PlanMode LLM Guard (devrix-d7-uncertainty-gaps)

`PlanMode.Enter()` MUST validate LLM availability via `HasLLM()` before entering active state, returning `ErrLLMNotConfigured` immediately instead of failing later during execution.

**Priority:** P0
**Package:** `internal/layers/orchestration/workmodel/plan_mode.go`
**T:** D7-S5-A02-F02-T01 … D7-S5-A02-F02-T02

#### Scenario: Enter with nil LLM returns error

- GIVEN a PlanMode created with nil LLM
- WHEN `Enter` is called
- THEN `ErrLLMNotConfigured` is returned
- AND the PlanMode state remains Inactive

#### Scenario: Enter with valid LLM succeeds

- GIVEN a PlanMode created with a valid LLMCompleter
- WHEN `Enter` is called
- THEN no error is returned
- AND the PlanMode state is Active

---

### Requirement: ConflictGuard Atomic Allow+Register (devrix-d7-uncertainty-gaps)

`ConflictGuard.AllowAndRegister()` MUST atomically check conflict and register a task, eliminating the TOCTOU window between `Allow()` and `Register()`. Returns `true` if registered, `false` if conflict prevents registration.

**Priority:** P0
**Package:** `internal/layers/orchestration/wavescheduler/conflict.go`
**T:** D7-S3-A01-F03-T01 … D7-S3-A01-F03-T04

#### Scenario: AllowAndRegister succeeds when no conflict

- GIVEN an empty ConflictGuard
- WHEN `AllowAndRegister` is called with a TaskNode in group `"A"`
- THEN the call returns true
- AND the task is registered in the guard

#### Scenario: AllowAndRegister blocks on conflict group

- GIVEN a ConflictGuard with a running task in group `"A"`
- WHEN `AllowAndRegister` is called with another TaskNode in group `"A"`
- THEN the call returns false
- AND the second task is NOT registered

#### Scenario: AllowAndRegister allows different groups

- GIVEN a ConflictGuard with a running task in group `"A"`
- WHEN `AllowAndRegister` is called with a TaskNode in group `"B"`
- THEN the call returns true
- AND both tasks are registered

#### Scenario: AllowAndRegister blocks on file scope intersection

- GIVEN a ConflictGuard with a running write task scoped to `"src/auth/**"`
- WHEN `AllowAndRegister` is called with a write TaskNode scoped to `"src/auth/login.go"`
- THEN the call returns false

---

### Requirement: OrchestratePath FlowEvent Sink (devrix-d7-uncertainty-gaps)

`emit()` MUST push FlowEvent to the EventPublisher sink for IM/WebSocket notifications, while also writing to the caller channel. Both paths respect context cancellation; nil sink is gracefully tolerated.

**Priority:** P0
**Package:** `internal/layers/orchestration/sessionorchestrator/orchestrate_path.go`
**T:** D7-S3-A01-F04-T01 … D7-S3-A01-F04-T02

#### Scenario: emit pushes to sink when available

- GIVEN an OrchestratePath with a non-nil EventPublisher sink
- AND a WorkerEvent with Type `"text"` and Content `"task_1 done"`
- WHEN `emit` is called
- THEN `sink.Publish` is called with the corresponding EngineEvent
- AND the event is also written to the out channel

#### Scenario: emit tolerates nil sink

- GIVEN an OrchestratePath with a nil EventPublisher sink
- WHEN `emit` is called
- THEN no panic occurs
- AND the event is written to the out channel

---

### Requirement: Dead Code Markers (devrix-d7-uncertainty-gaps)

`LLMFallbackClassifier` and `ExecutorSelector` MUST carry `Deprecated:` comments documenting they are deferred to v1.1, so future readers understand they are intentionally dead code rather than bugs.

**Priority:** P1
**Package:** `internal/layers/orchestration/decisionplanning/classifier_fallback.go`, `internal/layers/orchestration/decisionplanning/executor.go`
**T:** D7-S2-A03-F06-T01 … D7-S2-A03-F06-T02

#### Scenario: LLMFallbackClassifier has Deprecated marker

- GIVEN the `classifier_fallback.go` file
- THEN the file contains a `Deprecated:` comment
- AND the existing tests still pass

#### Scenario: ExecutorSelector has Deprecated marker

- GIVEN the `executor.go` file
- THEN the file contains a `Deprecated:` comment
- AND the existing tests still pass

### Requirement: D2 QueryLoop Legacy Path Removal (devrix-d2-queryloop-dismantle)

D2 `query.Loop`, `QueryLLMCaller`, and `d2_query_loop_legacy_invocations_total` MUST NOT exist. All LLM↔Tool loops MUST run through D7 `RunTurn` / `SubTurn`. Supersedes DM-20260617-001 Z0 deprecation.

**Priority**: P0  
**T mapping**: D7-S2-A06-T09, `contextengine/queryloop_removed_test.go`

#### Scenario: Production default uses D7 only

- GIVEN default devrix configuration
- WHEN a session completes a full turn
- THEN D7 RunTurn handles all LLM↔Tool iterations
- AND `grep QueryLLMCaller internal/` returns zero production hits

<!-- T: D7-S2-A06-T09 -->

---

## REMOVED Requirements

### Requirement: PlanModeApproveGate Config (devrix-d7-uncertainty-gaps)

The `PlanModeApproveGate` config field has been removed across all config layers — Approve/Reject is driven by explicit CLI commands, not an extra config switch.

**Priority:** P0
**Packages:** `internal/layers/orchestration/orchtypes/config.go`, `internal/shared/config/coordinator.go`, `internal/shared/config/loader.go`, `internal/bootstrap/wire_coordinator.go`
**T:** D7-S5-A02-F05-T01 … D7-S5-A02-F05-T02

#### Scenario: Config struct no longer contains the field

- GIVEN the Config struct definition
- THEN `PlanModeApproveGate` field does not exist

#### Scenario: Default config compiles without it

- GIVEN the `DefaultConfig` function
- THEN no reference to `PlanModeApproveGate` exists

---

## Configuration

```yaml
context_engine:
  execution_flow:
    enabled: false              # 默认关闭，bootstrap 显式启用
    link_tasks: true
    im_progress: true
    tool_summary_throttle_ms: 500
    event_buffer_size: 32
  tasks:
    mode: v2                  # v1=todo, v2=task
    store_dir: "~/.devrix/tasks/"
  plan:
    enabled: false              # 显式 /plan 启用
    auto_detect: false

# D7 v1.0 规划配置（未实现）
orchestration:
  d7_enabled: true              # false 时 WireD7 失败，进程不启动（DM-007）
  routing_mode: loop_first      # loop_first (default) | rule_orchestrate (legacy ingress)
  fast_path:
    confidence_threshold: 0.9
  plan:
    max_tasks_per_plan: 20
    max_depth: 5
```

### Loop-First Routing (DM-20260616-002)

When `coordinator.routing_mode` is `loop_first` (default):

- Ingress: Skip | Command | Turn — no ingress-level `OrchestratePath`
- Wave/Plan: tool-gated inside Turn via `delegate_wave` / `enter_plan_mode`
- ShadowClassifier: continues tail-only observation on legacy orchestrate tail without changing routing
- Metrics: `orchestration.tool.delegate_wave`, route span label `turn` for loop_first messages

When `routing_mode=rule_orchestrate`, DM-20260615-004 ingress behavior is preserved (FastPathThreshold downgrade).

---

## Guides（互补，非登记 SoT）

- **领域 SoT**: `d7-domain.md` — North Star、Out of Scope、文档索引
- **终态架构**: `terminal-state-guide.md` — IntentKind 四链、跨域时序、路由矩阵
- **可观测性**: `observability-guide.md` — Span↔T、Trace 树、FastPath SLA、P0 Runbook
- **澄清归档**: `d7-requirements-clarifications.md` — Review R1/R2 完整条文

---

## Cross-Domain Contracts

| 契约 | 方向 | 接口 | 状态 |
|------|------|------|------|
| ExecutionFlowHub | D2/D4 → D7-S4 | `contracts.ExecutionFlowHub` | IMPLEMENTED |
| FlowBridge | D4 → Hub | `multiagent/delegate` FlowBridge | IMPLEMENTED |
| delegate_tools | D2 → D4 + Hub | `contextengine/delegate_tools.go` | IMPLEMENTED（目标由 D7 编排） |
| WorkPlanSnapshot | D7 → D2 Leader | `Hub.Snapshot` → delegate_status tool | IMPLEMENTED |
| D1 entry | D1 → D7 | `ProcessMessage` | IMPLEMENTED |
| **DM-020 LLMCaller 拆面** | **D7 → D2** | `contracts.LLMCaller` ← `turn.QueryLLMCaller` | **IMPLEMENTED** |
| **DM-020 Summarizer 拆面** | **D7 → D2** | `contracts.Summarizer` ← `turn.CompressionSummarizer` | **IMPLEMENTED** |
| **DM-20260617-008 Tool 端到端链路** | **D7 → D2** | `turn_adapter.ExecuteRound` (派发闸口) | **IMPLEMENTED** — 完整链路 SoT 在 `openspec/specs/d2-context-engine/spec.md` §"Tool Call End-to-End Flow" (Chain A/B/C + 5 surface 派发表 + 跨域拓扑) |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-10 | ORCH v2 read model spec (DM-20260610-012) |
| 1.0.0 | 2026-06-13 | D7 domain spec draft (DM-20260613-001, S3 design) |
| 2.0.0 | 2026-06-14 | 对齐最新代码：实现状态标注、DSAFT 结构、T 层映射、配置同步 |
| 2.1.0 | 2026-06-14 | Review R1 澄清写入 spec 摘要，指向 demand.md / d7-domain.md |
| 2.2.0 | 2026-06-14 | Review R2：D7-D1 权力分配、HandleInterrupt 顺序、OQ 闭合 |
| 2.3.0 | 2026-06-15 | **v1.0 + v1.1 闭环**：(1) S2 Turn Leader (DM-020) + Meta-Orchestrator 标注；(2) S1 State Authority 标注；(3) DSAFT 结构 + Scenarios 表 5/5 S 层 IMPLEMENTED；(4) Architecture 图更新至 D7-S2 主入口；(5) D7-S1 WorkModel Requirement 状态刷新（Partial → IMPLEMENTED）；(6) PLANNED Requirements 表全 ✅ |
| **2.4.0** | **2026-06-15** | **DM-020 D2→D3 拆面闭合**：(1) `shared/contracts/llm_facade.go` 新增 `LLMCaller` + `Summarizer` 拆面契约；(2) `turn.QueryLLMCaller` + `turn.CompressionSummarizer` 实现并由 `bootstrap/context_engine.go` 单一注入点 wired 至 `EngineDeps.QueryLLMCaller` / `EngineDeps.Summarizer`；(3) D2 production wiring 零 D3 import；(4) Cross-Domain Contracts 表新增两行 DM-020 拆面 IMPLEMENTED |
| **2.5.0** | **2026-06-15** | **DM-20260615-004 D7 Intent 路径正交化**：(1) `coordinator.command_handler.go` 新增（IntentCommand 显式分发到 PlanCLI/CLICommands，零 LLM 成本）；(2) `coordinator.orchestrate_path.go` 新增（IntentOrchestrate 显式调 `TaskDecomposer.SynthesizeTaskGraph` + `WaveScheduler.Start` + `WaitForCompletion`）；(3) `coordinator.orchestrator.go::ProcessMessage` switch 4 case 改为 4 独立执行链，删除 v1.0 `handleCommand` / `orchestrate` 占位实现；(4) Architecture 图更新至 v1.1.0+ orthogonal 形态 |
| **2.7.0** | **2026-06-15** | **D7 Real-Closure Spec Sync**：(1) 实现状态表 4 cell 更新（D7-S1 WorkModel、D7-S5 PlanMode、D7-S2-A06 RunTurnLoop、D7-S2-A07 InvokeLLM 全部 IMPLEMENTED）；(2) 域边界移除 "Task 写模型（暂在 D2）"；(3) D2 Loop 最终状态 sync（loop.go ≤200 行，LoopHooks 已删除）|
| **2.9.0** | **2026-06-15** | **D2 Loop 瘦身闭环**：(1) `query/loop.go` 239 行→170 行（符合 ≤200 行目标）；(2) `LoopHooks` 结构体删除，4 个编排字段迁出（`PlanMode`/`TaskManager`/`Orchestration`/`Hub`）；(3) D7-D4-T01 / D7-THIN-T01/T02 T 点闭环 |
| **2.10.0** | **2026-06-15** | **D7-S5 LLM Decomposer 闭环**：(1) `coordinator/llm_decomposer.go` 新增（LLM 增强任务合成，JSON DAG → wave.TaskNode）；(2) `coordinator/llm_decomposer_test.go` 7 T sub-cases（happy/bad JSON/enum coercion/unknown deps/extractJSON/nil LLM/routing）；(3) `WithLLMDecomposer` option wired into SessionOrchestrator |
| **3.0.0** | **2026-06-16** | **v1.2 + v2.0-b/c/f 全部闭环**：(1) D7-S1-T08 Task 状态机守卫；(2) D7-S5-A01-T01 置信度阈值；(3) D7-S2-A06/A07 Turn Leader；(4) t-registry 66/66 IMPLEMENTED |
| **3.1.0** | **2026-06-16** | 薄 `d7-domain.md` + `terminal-state-guide.md`；澄清迁至 `d7-requirements-clarifications.md`；域边界 LLM 产权修正 |
| **3.4.0** | **2026-06-16** | **devrix-d7-loop-first-routing (DM-20260616-002)**：(1) `routing_mode=loop_first` 默认 ingress → Turn；(2) `delegate_wave` / `enter_plan_mode` tool 门控 Wave/Plan；(3) EngineEvent 单投递路径；(4) `rule_orchestrate` 回滚；(5) L5-01..06 登记 |
| **3.3.0** | **2026-06-16** | **devrix-d7-uncertainty-gaps (DM-20260616-001) 归档**：(1) PlanAgent 运行时门控 Gherkin scenarios（4 T 点）；(2) PlanMode LLM 守卫（2 T 点）；(3) ConflictGuard 原子 Allow+Register（4 T 点）；(4) OrchestratePath FlowEvent sink 恢复（2 T 点）；(5) PlanModeApproveGate 死配置移除（2 T 点）；(6) 死代码 Deprecated 标记（2 T 点） |
| **3.2.0** | **2026-06-16** | `observability-guide.md`；`dsaft-architecture.md` Stub；Guides 索引 |
| **3.5.0** | **2026-06-17** | **devrix-queryloop-legacy-decommission (DM-20260617-001)**：(1) ADDED Requirements：D2 QueryLoop Legacy Path Decommission（loopFirst=true 主路径护栏 + 拆面 adapter 零调用 + legacy metric 暴露 + CLI 警告 + D2-S10 spec.md LEGACY 标记）；(2) 6 个 Gherkin Scenario 覆盖 AC1-AC7；(3) T09/T10 + T04/T05 注册 |
| **3.6.0** | **2026-06-17** | **devrix-tool-surface-phase2-full (DM-20260617-008) 工具调用链路登记**：(1) Cross-Domain Contracts 表新增 DM-20260617-008 行（指 D2 spec §"Tool Call End-to-End Flow" 为完整链路 SoT）；(2) 端到端 Chain A/B/C 视图（3 链 7 surface 5 domain 拓扑）由 D2 spec 持有, D7 通过本表反查 |
| **3.7.0** | **2026-06-17** | **devrix-unified-work-tree (DM-20260617-009)**：(1) ADDED WorkItem + WorkTree 统一工作语义；(2) WorkTree ⊥ RunRegistry 分离（`run_ref` 外键）；(3) todo_write→checklist ephemeral 子节点 + sc.Todos 投影；(4) Wave OrchestratePath SyncWaveNodes；(5) legacy TaskManager 适配器；(6) 跨 session 只读查询 baseline |
| **3.8.0** | **2026-06-18** | **devrix-unified-work-tree v1.5–v2.0 闭环 (PR #85–#87)**：(1) 统一工具 alias task_write/spawn/await；(2) RunTurn decompose + ResolveHint + depth/daily limits；(3) RunTurn blocking await (`ResolveAwaiter`)；(4) v2.1+ defer → `openspec/tech-debt/worktree-v2-deferred.md` |
| **3.9.0** | **2026-06-19** | **devrix-d7-v2-structure (DM-20260619-005)**：(1) S 层物理路径对齐 `code-layout.md` §4.2；(2) coordinator→sessionorchestrator+decisionplanning+orchtypes；(3) wave→wavescheduler、S4→executionflow；(4) hubspoke dispatch/bridge 拆分；(5) WorkTree TD-WT-02/03 部分闭合 |

---

## Unified Work Tree (DM-20260617-009)

> **Archived:** `openspec/archive/2026-06-17-devrix-unified-work-tree/`

### Requirement: WorkItem Unified Work Unit Model

The orchestration layer SHALL represent all work semantics as `WorkItem` nodes in a per-session tree owned by D7 `WorkTree`.

Each WorkItem MUST have: `id`, optional `parent_id`, `kind`, `status`, `title`, `directive`, optional `uncertainty`, `policy`, dependency edges, optional `run_ref`, and `ephemeral` flag.

#### Scenario: Session root goal creation
- GIVEN a new session with no work items
- WHEN the first user message is processed
- THEN exactly one `kind=goal` root WorkItem exists with the user directive as `directive`

#### Scenario: Child work item under goal
- GIVEN an existing session goal WorkItem
- WHEN `delegate_implement` is invoked without an explicit work item id
- THEN a new `kind=implement` WorkItem is created with `parent_id` set to the goal id

### Requirement: WorkTree and RunRegistry Separation

Work semantics (What) and execution handles (How) MUST remain separate stores. `WorkItem.run_ref` links to RunRegistry entries; terminal callbacks update WorkItem status and bubble parent re-evaluation.

### Requirement: Legacy TaskManager Compatibility Adapter

`TaskManager` delegates to `WorkTree` internally while preserving flat `Task` API for `task_create`, `task_get`, `task_list`, and `/task` CLI commands.

### Requirement: Wave Scheduler Reads WorkTree (v1.1)

`OrchestratePath` SHALL call `TaskManager.SyncWaveNodes` after `SynthesizeTaskGraph`, writing implement subtrees before Wave dispatch.

### Deprecated (v2.0 target → v2.1 tech-debt)

> 详见 `openspec/tech-debt/worktree-v2-deferred.md` (TD-WT-02, TD-WT-03)

- Session-scratch `sc.Todos` as authoritative checklist (demoted to read projection via WorkTree)
- Independent persistent wave task graph as work semantics SoT

### Requirement: RunTurn Resolve and Decompose (v1.5–v2.0)

`DefaultOrchestrator.RunTurn` SHALL inject focus context via `FocusHintProvider`, MAY block-await running focus children via `ResolveAwaiter`, and SHALL guide decompose via `ResolveHint` when uncertainty exceeds threshold.

#### Scenario: Blocking await before LLM loop
- GIVEN a focus WorkItem with in-progress children that have `run_ref`
- WHEN RunTurn starts
- THEN `ResolveAwaiter` blocks until children reach terminal or timeout
- AND a `resolve` engine event is emitted with await summary

#### Scenario: High uncertainty decompose guidance
- GIVEN a focus WorkItem with uncertainty ≥ threshold and decomposable kind
- WHEN RunTurn injects focus hint
- THEN ResolveHint advises `task_write mode=decompose`
- AND `DecomposeChildren` enforces max depth, max children, and daily limit
