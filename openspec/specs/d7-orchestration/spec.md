# D7 Orchestration Domain Specification

**Capability:** d7-orchestration
**Domain:** D7
**DSAFT Type:** 核心域 (Core Domain)
**Version:** 2.3.0
**Status:** Canonical — source of truth
**Last Updated:** 2026-06-15
**Layering Spec:** `openspec/specs/architecture/layering.md`
**Change ID:** devrix-d7-orchestration-domain (DM-20260613-001)
**Demand:** `openspec/changes/devrix-d7-orchestration-domain/demand.md`
**Review R1:** `openspec/changes/devrix-d7-orchestration-domain/review-r1.md`
**Review R2:** `openspec/changes/devrix-d7-orchestration-domain/review-r2.md`

**Archived Changes:** devrix-queryloop-context (2026-06-10, ORCH v2 read model), devrix-wave-scheduler (WaveScheduler)

---

## Overview

D7 编排域回答 **"做什么、按什么顺序做、谁来做、做得怎么样了"**。作为 **横向协调层** 编排 D2（LLM↔Tool 执行原语）与 D4（Agent 委托原语），并向 D1 发布进度事件（D1 仍拥有 ingress）。

**现行实现路径（2026-06-15）：** v1.0 + v1.1 全部闭环（layer-delta Phase A–N）。Session Orchestrator（D7-S2 A01–A07, 含 Turn Leader A06/A07）+ ClassifyIntent/SynthesizeTaskGraph/SelectExecutor（D7-S5 A01–A03）+ WorkModel + PlanMode（D7-S1 + D7-S5 A04）位于 `internal/layers/orchestration/{coordinator,workmodel,turn,hubspoke}/`；Wave/Flow/IMSink（D7-S3/S4）位于 `internal/layers/orchestration/{wave,flow,workplan,imsink}/`。D1 主入口已切换至 `coordinator.Entry.ProcessMessage`（经 `d7_enabled` 路由开关，`bootstrap/wire_coordinator.go::WireD7` 完成所有 wiring）。

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

完整条文见 `d7-domain.md` §Requirements Clarifications 与 `demand.md`。

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
| S | D7-S1 | Work Model | Task/Plan 数据模型与生命周期 | **IMPLEMENTED** → `coordinator/workmodel.go` + `orchestration/workmodel/` |
| S | D7-S2 | Session Orchestrator | 用户消息主入口、Turn 主循环、Dispatch | **IMPLEMENTED** → `coordinator/` + `turn/` + `hubspoke/` |
| S | D7-S3 | Wave Scheduler | DAG 调度、WorkerPool、ConflictGuard | IMPLEMENTED → `orchestration/wave/` |
| S | D7-S4 | Execution Flow | FlowEvent 聚合、WorkPlan 快照、IM 广播 | IMPLEMENTED → `orchestration/flow/` + `hubspoke/` |
| S | D7-S5 | Decision & Planning | 意图分类、任务拆解、执行器选择 | **IMPLEMENTED** → `coordinator/{classifier,classifier_fallback,decomposer,executor}.go` |

---

## Scenarios

| ID | Scenario | Responsibility | Status | 代码位置 |
|----|----------|----------------|--------|----------|
| D7-S1 | Work Model | Task CRUD、依赖 DAG、磁盘持久化、PlanMode 状态机 | **IMPLEMENTED** | `orchestration/workmodel/` + `coordinator/workmodel.go` |
| D7-S2 | Session Orchestrator | ProcessMessage、FastPath、HandleInterrupt、TurnLoop、InvokeLLM、Dispatch | **IMPLEMENTED** | `orchestration/coordinator/` + `turn/` + `hubspoke/` |
| D7-S3 | Wave Scheduler | TaskGraph DAG、5-slot 池、ContextPolicy、ConflictGuard | IMPLEMENTED | `orchestration/wave/` |
| D7-S4 | Execution Flow | Hub 双通道发布、WorkPlan 读模型、IM worker_progress、SpokeBridge | IMPLEMENTED | `orchestration/flow/`, `workplan/`, `imsink/`, `hubspoke/` |
| D7-S5 | Decision & Planning | PlanAgent 只读探索、规则+LLM 分类、SynthesizeTaskGraph、SelectExecutor | **IMPLEMENTED** | `coordinator/{classifier,classifier_fallback,decomposer,executor,shadow_classifier}.go` + `workmodel/plan_*.go` |

---

## Architecture

```
D1 Gateway.RouteInbound
    └── D7-S2 SessionOrchestrator.ProcessMessage    ← v1.0 主入口（wired by wire_coordinator.go::WireD7）
            ├── D7-S2-A02 ClassifyIntent (rule + LLM fallback)
            ├── D7-S2-A06 RunTurnLoop → D7-S2-A07 InvokeLLM → D3 (LLM Gateway)
            │                            → D2 (ContextPreparer / ToolRoundExecutor / SessionPersister)
            ├── D7-S2-A01-F02 FastPath → D2.RunQueryLoop (legacy path)
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
| WorkPlan 读模型（D7-S4） | D2 RunQueryLoop | 会话上下文（D2） |
| Wave DAG 调度（D7-S3） | D4 Delegate RunAgent | Agent 生命周期（D4） |
| FlowEvent 契约（contracts） | Task 写模型（暂在 D2） | LLM 调用（D3） |

---

## ADDED Requirements

### Requirement: D7-S3 Wave Scheduler

`WaveScheduler` MUST provide DAG-based multi-agent scheduling with fixed WorkerPool capacity, ConflictGuard, and ContextPolicy resolution.

**Priority:** P0  
**Package:** `internal/layers/orchestration/wave/`  
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
**Package:** `internal/layers/orchestration/flow/hub.go`  
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
**Package:** `internal/layers/orchestration/workmodel/` + `coordinator/workmodel.go`（v1.1 闭环，layer-delta Phase I/J）
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
| D2 Thin QueryLoop | loop.go ≤200 行、无 D4 import | ✅ IMPLEMENTED |
| D7 package identity | `internal/layers/orchestration/coordinator/` (package `coordinator`) | ✅ IMPLEMENTED |
| D7 Migration Coexistence | 4 组合回归 | ✅ IMPLEMENTED |
| D7-S2 Turn Leader (DM-020) | A06 RunTurnLoop + A07 InvokeLLM | ✅ IMPLEMENTED（wired by `wire_coordinator.go`） |
| D7-S2 Hub-Spoke (DM-018) | A04 DispatchWorker + A04/A05 SpokeBridge | ✅ IMPLEMENTED（wired by `delegate.go`） |
| D7-S1 WorkModel 迁入 | 写模型从 D2 迁入 D7 | ✅ IMPLEMENTED |

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
  fast_path:
    confidence_threshold: 0.9
  plan:
    max_tasks_per_plan: 20
    max_depth: 5
```

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
