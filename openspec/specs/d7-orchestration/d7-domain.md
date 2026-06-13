# D7 Orchestration Domain Specification

**Capability:** d7-orchestration-domain
**Change ID:** devrix-d7-orchestration-domain
**Demand ID:** DM-20260613-001
**Layer:** 7 (Orchestration Domain)
**Version:** 2.0.0
**Status:** Active — IMPLEMENTED (S3/S4) + PLANNED (S1/S2/S5 migration)
**Last Updated:** 2026-06-14
**Implementation Audit:** `layer-delta.md`
**Depends On:** D2-S10 (QueryLoop), D4-S2 (Agent Lifecycle), D4-S10 (Delegate), D1-S1 (Gateway)

---

## Overview

D7 Orchestration Domain 是 DSAFT 架构的第七域，位于 D1-D6 之上，作为跨域编排层。

**域职责**：回答"做什么、按什么顺序做、谁来做、做得怎么样了"。

### 实现状态（2026-06-14 代码审计）

| Scenario | 状态 | 现行代码位置 |
|----------|------|-------------|
| D7-S3 Wave Scheduler | ✅ IMPLEMENTED | `internal/layers/orchestration/wave/` |
| D7-S4 Execution Flow | ✅ IMPLEMENTED | `internal/layers/orchestration/flow/`, `workplan/`, `imsink/` |
| D7-S1 Work Model | 🔶 PARTIAL | `internal/layers/contextengine/tasks/`（写模型仍在 D2） |
| D7-S5 Decision & Planning | 🔶 PARTIAL | PlanMode/PlanAgent 在 D2；分类/拆解未实现 |
| D7-S2 Session Orchestrator | ⬜ PLANNED | D1 仍调用 `D2.Process`（`gateway.go:286`） |
| `internal/layers/d7/` 包 | ⬜ PLANNED | 目录不存在 |

**域边界**：
- D7 **拥有**：WorkPlan 读模型（D7-S4）、Wave DAG 调度（D7-S3）
- D7 **编排**：D2（LLM↔Tool 执行）、D4（Agent 委托）
- D7 **暂托管（D2）**：Task 写模型、PlanMode（目标迁入 D7-S1/S5）
- D7 **不拥有**：会话上下文（D2）、agent 生命周期（D4）、LLM 调用（D3）

**和 D1-D6 的关系（现行 vs 目标）**：
```
【现行】
D1 → D2.Process → delegate_tools → D4 + flow.GlobalHub
ORCH wave/ 由 delegate_tools 独立触发

【目标 D7 v1.0】
D1 → D7.ProcessMessage (替代 D1→D2.Process)
D7 编排 D2 RunQueryLoop + D4 RunAgent
D7 将进度事件发布到 D1（通信层）
D6 → D7 ValidateOrchestration (元决策校验, advisory)
D5 观测 D7 (orchestration.wave.* / orchestration.flow.*)
D3 不直接和 D7 交互
```

| DSAFT ID | 名称 | 来源 | 域类型 |
|----------|------|------|--------|
| D7 | Orchestration Domain | 升格自 ORCH v2 | 核心 |

---

## ADDED Requirements

### Requirement: D7 Domain Identity

D7 MUST exist as a top-level domain package at `internal/layers/d7/` with defined DSAFT S/A/F/T mapping. Domain type MUST be "核心".

**Implementation Status (2026-06-14):** ⬜ PLANNED — `internal/layers/d7/` 不存在；现行能力分布在 `orchestration/` 与 `contextengine/tasks/`。

<!-- T: D7-IDENTITY-T01 (PLANNED) -->

#### Scenario: D7 package exists

- GIVEN the Devrix project structure
- WHEN checking `internal/layers/` directory
- THEN a `d7/` directory exists
- AND its import path is `github.com/devrix/devrix/internal/layers/d7`

#### Scenario: D7 domain registered in DSAFT mapping

- GIVEN the DSAFT methodology document
- WHEN reading the domain mapping table
- THEN D7 is listed with name "编排域" and type "核心"
- AND `openspec/specs/architecture/layering.md` includes D7

---

### Requirement: D7-S1 Work Model

D7-S1 MUST own the unified Task data model as the single source of truth. Task CRUD MUST only go through D7-S1 activities.

**Implementation Status (2026-06-14):** 🔶 PARTIAL — TaskManager 在 `contextengine/tasks/`，WorkPlan 读投影在 `orchestration/`。`CreateWorkPlan` Activity 未实现。

<!-- T: D7-S1-T01 … D7-S1-T06 -->

#### Scenario: Task created through D7-S1

- GIVEN a session and a task goal
- WHEN D7-S1-A02 ManageTask is called with Create action
- THEN a Task is created with unique ID and status "created"
- AND the Task is persisted to durable storage
- AND the Task is queryable via D7-S1-A03 QueryWorkPlan

#### Scenario: Task lifecycle state machine

- GIVEN a Task in status "created"
- WHEN D7-S1-A02 ManageTask transitions to "assigned"
- THEN the Task status is updated
- AND validation rejects invalid transitions (e.g. "created" → "completed" without "running")

#### Scenario: Plan contains DAG of tasks

- GIVEN a set of TaskSpecs with dependencies
- WHEN D7-S1-A01 CreateWorkPlan is called
- THEN a Plan is created with valid DAG
- AND dependency validation rejects cycles
- AND each Task in the Plan has the correct `Dependencies` field

#### Scenario: Work Plan snapshot query

- GIVEN a session with active tasks and completed tasks
- WHEN D7-S1-A03 QueryWorkPlan is called
- THEN a WorkPlanSnapshot is returned
- AND the snapshot includes all tasks with current status
- AND the snapshot includes execution flow events

#### Scenario: Background task registration

- GIVEN a session and a sub-query specification
- WHEN D7-S1-A02 ManageTask registers a background task
- THEN a task ID is returned
- AND the task status is "running"
- AND on completion the task status transitions to "completed"
- AND the result is available via QueryWorkPlan

#### Scenario: Task persistence survives restart

- GIVEN tasks persisted via D7-S1-A02
- WHEN the process restarts
- THEN all persisted tasks are recoverable
- AND their status reflects the last known state before restart

---

### Requirement: D7-S2 Session Orchestrator

D7-S2-A01 ProcessMessage MUST replace D1→D2.Process as the primary request entry point. It MUST support fast-path (direct D2 proxy) and orchestrate-path (multi-step plan).

**Implementation Status (2026-06-14):** ⬜ PLANNED — D1 `gateway.go:286` 仍调用 `contextEngine.Process`。

<!-- T: D7-S2-T01 … D7-S2-T04 (PLANNED) -->

#### Scenario: ProcessMessage is the entry point

- GIVEN a user message arrives at D1 Gateway
- WHEN D1 routes the message
- THEN D1 calls `D7-S2-A01 ProcessMessage` (not `D2.Process`)
- AND a channel of EngineEvent is returned
- AND at least one `text` or `thinking` event is emitted before `complete`

#### Scenario: Fast path routes directly to D2

- GIVEN a simple user message (e.g. "hello", "what time is it")
- WHEN D7-S2-A02 EvaluateIntent returns "simple" with confidence ≥ 90%
- WHEN D7-S2-A01 routes through fast path
- THEN D2.RunQueryLoop is called directly
- AND no Plan or Task is created
- AND total added latency is ≤ 2ms compared to direct D2.Process call

#### Scenario: Orchestrate path creates Plan

- GIVEN a complex user message (e.g. "explore the module and refactor it")
- WHEN D7-S2-A02 EvaluateIntent returns "complex" or confidence < 90%
- WHEN D7-S2-A01 routes through orchestrate path
- THEN D7-S5-A02 SynthesizeTaskGraph is called
- AND D7-S1-A01 CreateWorkPlan creates a Plan
- AND tasks are dispatched sequentially with dependency ordering

#### Scenario: Interrupt handler cancels active orchestration

- GIVEN a user sends /stop during active orchestration
- WHEN D7-S2-A03 HandleInterrupt is called
- THEN all running tasks for the session are cancelled
- AND D2 RunQueryLoop contexts are cancelled
- AND D4 agent runs are cancelled
- AND a "stopped" event is emitted

#### Scenario: Interrupt idempotency

- GIVEN no active tasks for a session
- WHEN D7-S2-A03 HandleInterrupt is called
- THEN no error is returned
- AND cleanup is idempotent

---

### Requirement: D7-S3 Wave Scheduler (升格自 ORCH-S3)

D7-S3 MUST provide DAG-based multi-agent scheduling, worker pool management, conflict guarding, and context resolution.

**Implementation Status (2026-06-14):** ✅ IMPLEMENTED at `internal/layers/orchestration/wave/`. 12/11 T 测试点 IMPLEMENTED（D7-S3-T01…T10, T11 PARTIAL）。

<!-- T: D7-S3-T01 … D7-S3-T11 -->

#### Scenario: DAG scheduling unchanged after migration

- GIVEN a task graph that previously ran through ORCH-S3 ScheduleWave
- WHEN D7-S3-A01 ScheduleWave runs with the same input
- THEN execution order is identical to ORCH-S3
- AND output artifacts are identical

#### Scenario: Worker pool capacity preserved

- GIVEN D7-S3-A01 ScheduleWave
- WHEN worker slots are acquired
- THEN capacity limits match ORCH-S3 defaults: cursor=1, claude_code=1, subagent=3
- AND slot release hooks fire on completion

---

### Requirement: D7-S4 Execution Flow (升格自 ORCH-S1/S2)

D7-S4 MUST aggregate FlowEvent from D2 SubQuery and D4 Delegate into WorkPlan snapshots, and publish to D1 Gateway.

**Implementation Status (2026-06-14):** ✅ IMPLEMENTED at `orchestration/flow/hub.go`, `workplan/service.go`, `imsink/gateway.go`. 7 T 测试点 IMPLEMENTED。

<!-- T: D7-S4-T01 … D7-S4-T07 -->

#### Scenario: Event publication unchanged

- GIVEN FlowEvent published through ORCH-S2 PublishFlow
- WHEN D7-S4-A01 PublishFlowEvent receives the same event
- THEN WorkPlan is updated identically
- AND SessionQueue delegate-progress is enqueued
- AND IM worker_progress is emitted (if configured)

#### Scenario: Flow event lifecycle kinds

- GIVEN an active SubQuery or D4 worker
- WHEN FlowEvent is published
- THEN event kinds include FlowStarted, FlowCompleted, FlowFailed, FlowToolCall, FlowIterating
- AND each event is timestamped with actual occurrence time

---

### Requirement: D7-S5 Decision & Planning

D7-S5 MUST provide structured intent classification and task decomposition. Classification MUST use a layered approach: rules first, LLM fallback, merged result.

**Implementation Status (2026-06-14):** 🔶 PARTIAL — PlanMode/PlanAgent 已实现（`/plan` 工作流）；ClassifyIntent/SynthesizeTaskGraph/SelectExecutor 未实现。

<!-- T: D7-S5-T01 … D7-S5-T05 -->

#### Scenario: Rule-based classification

- GIVEN a user message matching a known pattern (e.g. "hello", "thanks", contains "delegate_explore")
- WHEN D7-S5-A01 ClassifyIntent runs
- THEN classification result returns high confidence (≥ 90%)
- AND LLM classification is NOT invoked

#### Scenario: LLM-based classification fallback

- GIVEN a user message with no matching rules (e.g. "investigate auth module latency and propose fix")
- WHEN D7-S5-A01 ClassifyIntent runs with no rules matched
- THEN rule-based result has low confidence (< 90%)
- AND LLM classification is invoked
- AND merged result combines both signals

#### Scenario: Task decomposition produces valid DAG

- GIVEN a complex goal and session context
- WHEN D7-S5-A02 SynthesizeTaskGraph runs
- THEN output is a non-empty list of TaskSpecs
- AND TaskSpec dependencies form a valid, acyclic DAG
- AND each TaskSpec has a defined type (explore | plan | execute | background)

#### Scenario: Executor selection by task type

- GIVEN a TaskSpec with type "explore"
- WHEN D7-S5-A03 SelectExecutor runs
- THEN the selected executor is D2 (LLM↔Tool loop with read-only tools)

- GIVEN a TaskSpec with type "execute" and requires parallel workers
- WHEN D7-S5-A03 SelectExecutor runs
- THEN the selected executor is D4 (agent delegation)

#### Scenario: Empty message classification

- GIVEN an empty user message
- WHEN D7-S5-A01 ClassifyIntent runs
- THEN classification returns "skip"
- AND no LLM classification is invoked

---

### Requirement: D2 Thin — Pure Query Loop

After D7 migration, `query.Loop.Run` MUST only handle LLM↔Tool interaction. All orchestration-side responsibilities MUST be removed from Loop.

**Implementation Status (2026-06-14):** ⬜ NOT STARTED — `loop.go` 414 行，仍 import `multiagent/delegate`。

<!-- T: D7-THIN-T01 … D7-THIN-T04 (PLANNED) -->

#### Scenario: Orchestration fields removed from Loop

- GIVEN the query.Loop struct after migration
- WHEN inspecting its fields
- THEN the following fields MUST NOT exist: Attachments, EnsureParallelAsyncBatch, WaitPendingAsyncBatch, SessionQueue, Hooks.AfterToolRound, Hooks.BeforeComplete
- AND LLM, Tools, Permission, Compress MUST remain

#### Scenario: Loop.Run line count

- GIVEN query/loop.go after D7 migration
- WHEN counting lines in the Run method (loop body only)
- THEN Run MUST be ≤ 200 lines (down from ~430)

#### Scenario: Loop no longer imports D4

- GIVEN query/loop.go after migration
- WHEN checking imports
- THEN `multiagent/delegate` is NOT imported
- AND `contextengine/queue` is NOT imported

#### Scenario: Loop.Run input/output unchanged

- GIVEN a valid set of messages, tools, and system prompt
- WHEN Loop.Run is called with the same inputs before and after migration
- THEN the returned Result (messages, usage, turnCount) is identical
- AND the order of LLM calls and tool executions is preserved

---

### Requirement: D7-D1 Contract

D1 Gateway MUST route inbound messages to D7 ProcessMessage instead of D2 Process.

<!-- T: D7-D1-T01 -->

#### Scenario: D1 calls D7 for all messages

- GIVEN a running Devrix instance with D7 enabled
- WHEN any user message arrives at D1 Gateway RouteInbound
- THEN D7-S2-A01 ProcessMessage is called (not D2.Process)
- AND D2.Process is NOT called directly by D1

#### Scenario: Feature flag fallback

- GIVEN `orchestration.d7_enabled=false` (feature flag)
- WHEN D1 Gateway RouteInbound runs
- THEN the legacy path D1→D2.Process is used
- AND behavior matches pre-D7 V6

---

### Requirement: D7-D4 Contract

D7 MUST orchestrate D4 agent operations directly, NOT through D2 loop hooks.

<!-- T: D7-D4-T01 -->

#### Scenario: No delegate hooks in D2 loop

- GIVEN D7 is enabled
- WHEN inspecting D2 query.Loop hooks
- THEN AfterToolRound is nil
- AND EnsureParallelAsyncBatch is nil
- AND WaitPendingAsyncBatch is nil

#### Scenario: D7 directly calls D4 delegate

- GIVEN an orchestration path requiring a delegate worker
- WHEN D7-S5-A03 SelectExecutor selects D4
- THEN D7 calls D4 delegate service directly (not through D2)
- AND the delegate result is returned to D7 for artifact collection

---

### Requirement: D7-D6 Contract

D6 MAY validate orchestration decisions made by D7.

<!-- T: D7-D6-T01 -->

#### Scenario: D6 validates orchestration decision

- GIVEN D7-S5-A01 ClassifyIntent produces a classification
- WHEN D6-S4-A01 ValidateOrchestration is invoked
- THEN validation returns pass/fail with reason
- AND D7 MAY proceed or adjust based on validation result

#### Scenario: D6 validation is advisory

- GIVEN D6 validation returns "fail" for a D7 orchestration decision
- WHEN D7 continues with the decision
- THEN D7 does NOT panic or crash
- AND D6 validation failure is logged

---

## MODIFIED Requirements

### Requirement: D1 Gateway Entry Point

D1-S1-A02 RouteInbound MUST support routing to D7 (preferred) or D2 (legacy fallback).

<!-- T: D1-S1-T01 (modified) -->

#### Scenario: RouteInbound routes to D7

- GIVEN D7 is enabled
- WHEN RouteInbound processes a message
- THEN it extracts session and message
- AND calls `D7.ProcessMessage`
- AND D2.Process is NOT called

#### Scenario: RouteInbound fallback to D2

- GIVEN `orchestration.d7_enabled=false`
- WHEN RouteInbound processes a message
- THEN it calls `D2.Process` as before
- AND D7.ProcessMessage is NOT called

### Requirement: D2 Context Engine Process

D2.Process MUST remain available for backward compatibility but SHOULD NOT be the primary entry point when D7 is active.

<!-- T: D2-CTX-T01 (modified) -->

#### Scenario: D2.Process backward compatible

- GIVEN D7 is enabled
- WHEN D2.Process is called directly (bypassing D7)
- THEN it produces identical results as before migration
- AND session context is managed correctly

---

## REMOVED Requirements

(None)

---

## Configuration

```yaml
# 现行配置（已实现）
context_engine:
  execution_flow:
    enabled: false              # 默认关闭
    link_tasks: true
    im_progress: true
    tool_summary_throttle_ms: 500
    event_buffer_size: 32
  tasks:
    mode: v2
    store_dir: "~/.devrix/tasks/"
  plan:
    enabled: false
    auto_detect: false

# D7 v1.0 规划配置（未实现）
orchestration:
  d7_enabled: false             # false 时保持 D1→D2.Process
  fast_path:
    confidence_threshold: 0.9
  decision:
    rules_enabled: true
    llm_fallback: true
  plan:
    max_tasks_per_plan: 20
    max_depth: 5
  task:
    persistence_enabled: true
    store_dir: "~/.devrix/tasks/"
```

---

## T 层测试点索引

完整注册表见 `t-registry.md`。摘要：

| 范围 | IMPLEMENTED | PLANNED | P0 |
|------|-------------|---------|-----|
| D7-S3 Wave | 10 | 1 (PARTIAL) | 8 |
| D7-S4 Flow | 7 | 0 | 6 |
| D7-S1 Work | 5 | 1 | 3 |
| D7-S5 Decision | 1 | 4 | 3 |
| D7-S2 Orchestrator | 0 | 4 | 4 |
| 契约/瘦身 | 0 | 5 | 4 |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | Initial D7 domain spec: 5 scenarios, 15 activities, 28 function points |
| 2.0.0 | 2026-06-14 | 代码审计对齐：实现状态标注、配置同步、T 层索引指向 t-registry.md |
