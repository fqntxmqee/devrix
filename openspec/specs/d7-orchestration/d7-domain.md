# D7 Orchestration Domain Specification

**Capability:** d7-orchestration-domain
**Change ID:** devrix-d7-orchestration-domain
**Demand ID:** DM-20260613-001
**Layer:** 7 (Orchestration Domain)
**Version:** 2.3.0
**Status:** Active — IMPLEMENTED (S3/S4/S2) + PLANNED (S1/S5 migration)
**Last Updated:** 2026-06-15
**Implementation Audit:** `layer-delta.md`
**Demand:** `openspec/changes/devrix-d7-orchestration-domain/demand.md`
**Review R1:** `openspec/changes/devrix-d7-orchestration-domain/review-r1.md`
**Review R2:** `openspec/changes/devrix-d7-orchestration-domain/review-r2.md`
**Depends On:** D2-S10 (QueryLoop), D4-S2 (Agent Lifecycle), D4-S10 (Delegate), D1-S1 (Gateway)

---

## Overview

D7 Orchestration Domain 是 DSAFT 架构的第七域，作为**横向协调层**编排 D2（执行原语）与 D4（委托原语），并向 D1 发布进度事件。D1 仍拥有 ingress，D7 不替代 D1 Gateway。

**域职责**：回答"做什么、按什么顺序做、谁来做、做得怎么样了"。

### 实现状态（2026-06-15 代码审计）

| Scenario | 状态 | 现行代码位置 |
|----------|------|-------------|
| D7-S3 Wave Scheduler | ✅ IMPLEMENTED | `internal/layers/orchestration/wave/` |
| D7-S4 Execution Flow | ✅ IMPLEMENTED | `internal/layers/orchestration/flow/`, `workplan/`, `imsink/` |
| D7-S1 Work Model | 🔶 PARTIAL | `internal/layers/contextengine/tasks/`（写模型仍在 D2） |
| D7-S5 Decision & Planning | 🔶 PARTIAL | PlanMode/PlanAgent 在 D2；分类已实现 |
| D7-S2 Session Orchestrator | ✅ IMPLEMENTED | `internal/layers/orchestration/coordinator/` + `bootstrap/wire_coordinator.go` |
| D7-S5 ClassifyIntent / Shadow | ✅ IMPLEMENTED | `internal/layers/orchestration/coordinator/{classifier,shadow_classifier}.go` |
| D2 Loop 瘦身 | ⬜ IN PROGRESS | `query/loop.go` 414行，需移除编排字段 |

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

## Requirements Clarifications (Review R1, 2026-06-14)

> 本节收录架构 Review 决议，供二次评审。详见 `demand.md` §2 澄清记录。

### Task Model Trinity（三模型职责分离）

v1.0 **不合并**数据结构，通过统一查询入口 `QueryWorkPlan` 聚合：

| 模型 | ID 前缀 | Scenario | 职责 |
|------|---------|----------|------|
| PlanTask | `task_` | D7-S1 | Plan 任务：subject、blocked_by、DiskStore |
| WaveTaskNode | Plan 节点 ID | D7-S3 | DAG 调度：worker_type、context_policy、depends_on |
| BackgroundRun | `bg_` | D7-S1 | SubQuery 异步句柄：output、cancel（目标迁入 D7-S1） |

**映射：** `WaveTaskNode.ID` 可关联 `PlanTask.ID`；`FlowEvent.TaskID` + `link_tasks` 联动 PlanTask 状态。对齐 DM-20260612-011：PlanTask 与 BackgroundRun **分离**，Registry 作 D7-S1 facade。

### Task Status Vocabulary（现行 SoT）

| 需求别名 | 代码 SoT (`TaskStatus`) | 触发 |
|----------|------------------------|------|
| created | `pending` | TaskManager.Create |
| assigned | `pending` + owner≠"" | FlowStarted + link_tasks |
| running | `in_progress` | FlowStarted / UpdateStatus |
| completed | `completed` | FlowCompleted |
| failed | `failed` | FlowFailed |

v1.0 不强制非法转换拒绝；v1.1 引入 `TransitionTaskState` 校验（D7-S1-T08 PLANNED）。

### Orchestration Routing Matrix（S2 vs S3 分工）

| 路由 | 条件 | 调度者 | 执行者 |
|------|------|--------|--------|
| FastPath | ClassifyIntent=simple, confidence≥threshold | D7-S2 | D2 QueryLoop |
| CommandPath | `/plan` `/task` `/stop` 等 | D7-S2（command-first，优先于 Classify） | 各命令处理器 |
| PlanPath | PlanMode active 或用户 `/plan` | D7-S2 → S5-P1 | PlanAgent → approve → PlanTask |
| SerialExplore | orchestrate + 单步 explore/plan | D7-S2 串行 | D2 QueryLoop（只读工具） |
| WaveExecute | orchestrate + 多 Worker 并行 | **D7-S3** WaveScheduler | D2/D4 via runners |
| BackgroundRun | SubQuery async | D7-S1 | D2 SubQuery（不经 Wave） |

**约束：** D7-S2 **不得**替代 D7-S3 做并行 DAG 调度；S2 OrchestratePath 在需并行时 **委托** `WaveScheduler.Start`。

### S5 Decision Layer — Phased Roadmap

| 阶段 | 能力 | v1.0 范围 |
|------|------|-----------|
| S5-P1 | PlanMode + PlanAgent | ✅ 已实现 |
| S5-P2 | ClassifyIntent（规则 + command-first） | ✅ v1.0 必须 |
| S5-P3 | SynthesizeTaskGraph 自动拆解 | ⬜ v1.1 |
| S5-P4 | auto_detect → PlanMode | ⬜ v1.2 |

v1.0 OrchestratePath：**不依赖** S5-P3；可路由至 PlanMode 或已有 delegate/wave 触发路径。

### Migration Coexistence Contract

| d7_enabled | plan.enabled | 入口 | 编排逻辑位置 |
|------------|--------------|------|-------------|
| false | * | D1→D2.Process | D2（现行） |
| true | false | D1→D7.ProcessMessage | D7 contracts → D2/D4 |
| true | true | D1→D7.ProcessMessage | D7 + PlanMode |

**约束：**
- `d7_enabled=true` 时 D7 **禁止**回退调用含编排逻辑的 D2.Process 主路径
- 迁移窗口 ≤ 2 release；上表 4 组合须全量回归（D7-MIG-T01 PLANNED）
- `d7_enabled` 默认 `false` 直至 acceptance-report P0 全绿

### Performance Acceptance（拆分 WHAT）

| T ID | WHAT | 优先级 |
|------|------|--------|
| D7-S2-T02a | FastPath proxy 在 Classify 完成后额外开销 P99 ≤ 2ms | P0 |
| D7-S2-T02b | 规则 ClassifyIntent（无 LLM）P99 ≤ 1ms | P0 |

简单消息 FastPath **不得**调用 LLM Classify（D7-S5-T06）。

### Configuration SoT

- **唯一** Task 持久化路径：`context_engine.tasks.store_dir`
- `orchestration.task.store_dir` 标记 **DEPRECATED**，实现时不新增

---

## ADDED Requirements

### Requirement: D7 Domain Identity

D7 MUST exist as a top-level domain under `internal/layers/orchestration/coordinator/` with defined DSAFT S/A/F/T mapping. Domain type MUST be "核心".

**Implementation Status (2026-06-14):** ✅ IMPLEMENTED — `internal/layers/orchestration/coordinator/` 已落地（package `coordinator`）；与 `internal/layers/orchestration/{wave,flow,workplan,imsink}/` 协同构成 D7 完整实现。

<!-- T: D7-IDENTITY-T01 (IMPLEMENTED) — covered by repo tree existence check + t-registry -->

#### Scenario: D7 coordinator package exists

- GIVEN the Devrix project structure
- WHEN checking `internal/layers/orchestration/` directory
- THEN a `coordinator/` directory exists
- AND its import path is `github.com/devrix/devrix/internal/layers/orchestration/coordinator`
- AND its package declaration is `coordinator`

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
- THEN a Task is created with unique ID and status `pending` (alias: created)
- AND the Task is persisted to durable storage when `tasks.mode=v2`
- AND the Task is queryable via D7-S1-A03 QueryWorkPlan

#### Scenario: Task lifecycle state machine

- GIVEN a Task in status `pending`
- WHEN D7-S1-A02 ManageTask sets owner via FlowStarted (link_tasks)
- THEN the Task owner field is updated (alias: assigned)
- AND status becomes `in_progress` (alias: running)
- AND v1.0 does NOT reject invalid transitions (deferred to v1.1 D7-S1-T08)

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
- WHEN D7-S1 registers a BackgroundRun (`bg_*` prefix)
- THEN a task ID is returned
- AND the run status is `running`
- AND on completion the status transitions to terminal (`completed` / `failed` / `cancelled`)
- AND the result is available via task_output tool and QueryWorkPlan Flow projection
- AND v1.0 implementation may remain in `query/background.go` with D7-S1 facade (D7-S1-T07)

#### Scenario: Task persistence survives restart

- GIVEN tasks persisted via D7-S1-A02
- WHEN the process restarts
- THEN all persisted tasks are recoverable
- AND their status reflects the last known state before restart

---

### Requirement: D7-S2 Session Orchestrator

D7-S2-A01 ProcessMessage MUST replace D1→D2.Process as the primary request entry point. It MUST support fast-path (direct D2 proxy) and orchestrate-path (multi-step plan).

**Implementation Status (2026-06-14, updated 2026-06-15):** ✅ IMPLEMENTED — D1 `gateway.go:325` 已有 D7 条件分支；`bootstrap/wire_coordinator.go` 提供 `WireD7` bootstrap 函数；`SetOrchestrationEntry` 切换路由。

<!-- T: D7-S2-T01 … D7-S2-T04 (IMPLEMENTED) -->

#### Scenario: ProcessMessage is the entry point

- GIVEN a user message arrives at D1 Gateway
- WHEN D1 routes the message
- THEN D1 calls `D7-S2-A01 ProcessMessage` (not `D2.Process`)
- AND a channel of EngineEvent is returned
- AND at least one `text` or `thinking` event is emitted before `complete`

#### Scenario: Fast path routes directly to D2

- GIVEN a simple user message (e.g. "hello", "what time is it")
- WHEN D7-S2-A02 EvaluateIntent returns "simple" with confidence ≥ 90% (rules only, no LLM)
- WHEN D7-S2-A01 routes through fast path
- THEN D2.RunQueryLoop is called directly
- AND no Plan or Wave Task is created
- AND FastPath proxy overhead P99 ≤ 2ms after classify (D7-S2-T02a)
- AND rule classify P99 ≤ 1ms (D7-S2-T02b)

#### Scenario: Orchestrate path routes per matrix

- GIVEN a complex user message (e.g. "explore the module and refactor it")
- WHEN D7-S2-A02 EvaluateIntent returns "orchestrate" with confidence < 90%
- WHEN D7-S2-A01 routes through orchestrate path
- THEN v1.0 MUST NOT require SynthesizeTaskGraph (S5-P3 deferred)
- AND v1.0 routes to PlanMode entry OR existing Wave/delegate trigger per Routing Matrix
- AND parallel multi-worker execute MUST delegate to D7-S3 WaveScheduler (not S2 serial loop)

#### Scenario: Command-first routing takes precedence

- GIVEN a user message starting with `/plan`, `/task`, or `/stop`
- WHEN D7-S2-A01 ProcessMessage runs
- THEN command handler is invoked before ClassifyIntent
- AND LLM classification is NOT invoked for routing (D7-S5-T06)

#### Scenario: Interrupt handler cancels active orchestration (/stop)

- GIVEN a user sends /stop during active orchestration
- WHEN D7-S2-A03 HandleInterrupt is called
- THEN sub-capabilities execute in order:
  1. `WaveScheduler.CancelAll(sessionID)` — explicit; Wave task ctx is detached from Process (`wave/scheduler.go`)
  2. D4 active delegate workers for the session are cancelled
  3. D2 Process context is cancelled (`gateway.StopProcess`)
  4. a `stopped` EngineEvent is emitted to D1
  5. TaskCancel propagates to WorkerCancel for any in-flight worker handles
- AND this order MUST NOT rely on Process context propagation to Wave (Wave survives normal Process cancel by design)

#### Scenario: Normal Process end does not cancel Wave

- GIVEN a leader Process context ends without `/stop`
- WHEN Wave workers are in-flight with detached task context
- THEN Wave workers continue until completion or explicit `CancelAll`
- AND only HandleInterrupt (`/stop`) triggers step 1 above

#### Scenario: Interrupt idempotency

- GIVEN no active tasks or workers for a session
- WHEN D7-S2-A03 HandleInterrupt is called
- THEN no error is returned
- AND cleanup is idempotent (D7-S2-T05)

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

> **Scope:** S5-P3 — **v1.1 only**；v1.0 使用 PlanMode 或手动 Task 创建替代。

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

**Implementation Status (2026-06-14, updated 2026-06-15):** ⬜ IN PROGRESS — `loop.go` 414行含编排字段（Attachments/Hooks/SessionQueue）待移除，目标≤200行。`delegate_tools.go`含`multiagent/delegate` import待迁移。

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

**Power allocation (Review R2):**

| Role | Ownership |
|------|-----------|
| D1 | ingress owner — `RouteInbound` invocation |
| D7 | routing decision owner — FastPath vs OrchestratePath |
| D1 | final veto — `orchestration.d7_enabled=false` restores legacy path |
| D7 | FastPath SLA — P99 ≤2ms (T02a+T02b+T02c) |

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
- AND validation call timeout defaults to 50ms; timeout treated as pass (D7-D6-T02)

---

### Requirement: D7 Migration Coexistence

During D7 rollout, dual-entry behavior MUST be explicitly defined and regression-tested.

<!-- T: D7-MIG-T01 -->

#### Scenario: Four-combination regression matrix

- GIVEN combinations of `d7_enabled` × `plan.enabled`
- WHEN RouteInbound processes a representative message set
- THEN each combination produces documented routing per Migration Coexistence Contract
- AND `d7_enabled=false` is bit-identical to pre-migration behavior

---

### Requirement: D7 Task Model Trinity

D7-S1 and D7-S3 MUST document and preserve three task representations with explicit mapping; v1.0 MUST NOT silently merge storage.

<!-- T: D7-S1-T07 -->

#### Scenario: QueryWorkPlan aggregates all representations

- GIVEN a session with PlanTasks, active Wave workers, and BackgroundRuns
- WHEN D7-S1-A03 QueryWorkPlan is called
- THEN WorkPlanSnapshot includes TaskSnapshots (PlanTask) and ExecutionFlows (workers)
- AND BackgroundRun progress is visible via FlowEvent or task_output

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

# D7 v1.0 规划配置（未实现；task.store_dir DEPRECATED）
orchestration:
  d7_enabled: false             # false 时保持 D1→D2.Process
  fast_path:
    confidence_threshold: 0.9
  decision:
    rules_enabled: true
    llm_fallback: false         # v1.1；v1.0 仅规则+command-first
  plan:
    max_tasks_per_plan: 20
    max_depth: 5
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
| D7-S2 Orchestrator | 0 | 7 | 7 |
| 契约/瘦身/迁移 | 0 | 6 | 4 |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | Initial D7 domain spec: 5 scenarios, 15 activities, 28 function points |
| 2.0.0 | 2026-06-14 | 代码审计对齐：实现状态标注、配置同步、T 层索引指向 t-registry.md |
| 2.1.0 | 2026-06-14 | Review R1：三模型、路由矩阵、S5 分阶段、迁移契约、性能指标拆分 |
| 2.2.0 | 2026-06-14 | Review R2：D7-D1 权力分配、HandleInterrupt 顺序、T02c、D6 metric |
| 2.3.0 | 2026-06-15 | WireCoordinator bootstrap 实现完成；D2 Loop 瘦身进行中 |
