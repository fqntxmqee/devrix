# D7 Orchestration Domain Specification

**Capability:** d7-orchestration-domain
**Change ID:** devrix-d7-orchestration-domain
**Demand ID:** DM-20260613-001
**Layer:** 7 (Orchestration Domain)
**Version:** 1.0.0
**Status:** S3 — Design
**Depends On:** D2-S10 (QueryLoop), D4-S2 (Agent Lifecycle), D4-S10 (Delegate), D1-S1 (Gateway)

---

## Overview

D7 Orchestration Domain 是 DSAFT 架构的第七域，位于 D1-D6 之上，作为跨域编排层。

**域职责**：回答"做什么、按什么顺序做、谁来做、做得怎么样了"。

**域边界**：
- D7 **拥有**：Task/Plan 数据模型（D7-S1）、编排路线决策（D7-S2/S5）
- D7 **编排**：D2（LLM↔Tool 执行）、D4（Agent 委托）
- D7 **不拥有**：会话上下文（D2）、agent 生命周期（D4）、LLM 调用（D3）

**和 D1-D6 的关系**：
```
D7 编排 D2（执行原语）和 D4（agent 原语）
D7 将进度事件发布到 D1（通信层）
D1 → D7.ProcessMessage (新入口, 替代 D1→D2.Process)
D6 → D7 ValidateOrchestration (元决策校验)
D5 观测 D7 (tracing/metrics)
D3 不直接和 D7 交互
```

| DSAFT ID | 名称 | 来源 | 域类型 |
|----------|------|------|--------|
| D7 | Orchestration Domain | 升格自 ORCH v2 | 核心 |

---

## ADDED Requirements

### Requirement: D7 Domain Identity

D7 MUST exist as a top-level domain package at `internal/layers/d7/` with defined DSAFT S/A/F/T mapping. Domain type MUST be "核心".

<!-- T: D7-IDENTITY-T01, D7-IDENTITY-F01-T01 -->

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

<!-- T: D7-S1-T01, D7-S1-T02, D7-S1-T03 -->

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

<!-- T: D7-S2-T01, D7-S2-T02, D7-S2-T03, D7-S2-T04 -->

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

D7-S3 MUST provide DAG-based multi-agent scheduling, worker pool management, conflict guarding, and context resolution. Behavior MUST remain bit-identical to ORCH-S3 V1.0.0 after migration.

<!-- T: D7-S3-T01, D7-S3-T02 -->

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

D7-S4 MUST aggregate FlowEvent from D2 SubQuery and D4 Delegate into WorkPlan snapshots, and publish to D1 Gateway. Behavior MUST remain bit-identical to ORCH-S1/S2 V1.0.0 after migration.

<!-- T: D7-S4-T01, D7-S4-T02 -->

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

<!-- T: D7-S5-T01, D7-S5-T02, D7-S5-T03, D7-S5-T04 -->

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

<!-- T: D7-THIN-T01, D7-THIN-T02 -->

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
orchestration:
  d7_enabled: true                          # 主开关；false 时回退 V6 行为
  fast_path:
    confidence_threshold: 0.9               # 快速路径置信度阈值
  decision:
    rules_enabled: true                     # 规则分类启用
    llm_fallback: true                      # LLM 分类兜底
  plan:
    max_tasks_per_plan: 20                  # 单 Plan 最大 Task 数
    max_depth: 5                            # DAG 最大深度
  task:
    persistence_enabled: true
    store_dir: "~/.devrix/tasks/"
```

---

## T 层测试点索引

| T ID | 名称 | 归属层次 | 优先级 |
|------|------|----------|--------|
| D7-IDENTITY-T01 | D7 package exists | Project | P0 |
| D7-S1-T01 | Task create and persist | A | P0 |
| D7-S1-T02 | Task lifecycle state machine | F | P0 |
| D7-S1-T03 | Plan with DAG validation | F | P0 |
| D7-S1-T04 | Background task lifecycle | F | P1 |
| D7-S1-T05 | Snapshot includes tasks and flows | A | P1 |
| D7-S1-T06 | Task persistence survives restart | A | P1 |
| D7-S2-T01 | ProcessMessage is D1 entry point | A | P0 |
| D7-S2-T02 | Fast path latency ≤ 2ms | F | P0 |
| D7-S2-T03 | Orchestrate path creates plan | A | P0 |
| D7-S2-T04 | Interrupt cancels active tasks | A | P0 |
| D7-S3-T01 | Wave scheduler bit-identical migration | A | P0 |
| D7-S3-T02 | Worker pool capacity unchanged | F | P0 |
| D7-S4-T01 | Flow event publication unchanged | A | P0 |
| D7-S4-T02 | Event lifecycle kinds | F | P0 |
| D7-S5-T01 | Rule-based classification | F | P0 |
| D7-S5-T02 | LLM fallback classification | F | P0 |
| D7-S5-T03 | Task DAG decomposition | A | P0 |
| D7-S5-T04 | Executor selection | F | P0 |
| D7-S5-T05 | Empty message skip | F | P1 |
| D7-THIN-T01 | Loop fields removed | F | P0 |
| D7-THIN-T02 | Loop line count ≤ 200 | F | P0 |
| D7-THIN-T03 | Loop no D4 import | F | P0 |
| D7-THIN-T04 | Loop Run I/O preserved | A | P0 |
| D7-D1-T01 | D1 calls D7, not D2 | A | P0 |
| D7-D4-T01 | No delegate hooks in D2 loop | A | P0 |
| D7-D6-T01 | D6 validates orchestration decision | A | P1 |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | Initial D7 domain spec: 5 scenarios, 15 activities, 28 function points |
