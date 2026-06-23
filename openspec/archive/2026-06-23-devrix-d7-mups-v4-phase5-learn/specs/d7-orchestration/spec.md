# D7 Orchestration Domain Specification

**Capability:** d7-orchestration
**Domain:** D7
**DSAFT Type:** 核心域 (Core Domain)
**Version:** 4.4.0
**Status:** Canonical — source of truth
**Last Updated:** 2026-06-23
**Domain SoT:** `d7-domain.md`
**Layering Spec:** `openspec/specs/architecture/layering.md`
**Change ID:** devrix-d7-orchestration-domain (DM-20260613-001)
**Demand:** `openspec/changes/devrix-d7-orchestration-domain/demand.md`
**Review R1:** `openspec/changes/devrix-d7-orchestration-domain/review-r1.md`
**Review R2:** `openspec/changes/devrix-d7-orchestration-domain/review-r2.md`

**Archived Changes:** devrix-queryloop-context (2026-06-10, ORCH v2 read model), devrix-wave-scheduler (WaveScheduler), devrix-d7-uncertainty-gaps (2026-06-16, DM-20260616-001, 5 gap fixes), devrix-d7-error-aggregation-and-metrics (2026-06-21, DM-20260621-010, D7-S6 错误聚合 + worktree 全链路 metrics), devrix-d7-mups-v4-phase3-execute (2026-06-23, DM-20260625-001, Phase 3 PR-C1: Execute Artifact 4 类 + SideEffect 5 态 + Artifact struct 5 字段升级), devrix-d7-mups-v4-phase2-observe-plan (2026-06-23, DM-20260623-001, Phase 2 PR-A1 + PR-RF: Observation 4 类 + UncertaintyReport + UncertaintyCoord), devrix-d7-mups-v4-phase2-plan (2026-06-23, DM-20260623-001-PRB1, Phase 2 PR-B1: Plan 4 类 + Planner interface + MatchKind 4 Rules), devrix-d7-mups-v4-phase3-channels (2026-06-23, DM-20260625-001-PRC2, Phase 3 PR-C2: Execute 4 Channel + ChannelRouter), devrix-d7-mups-v4-phase4-verify-promotion (2026-06-23, DM-20260623-002, Phase 4 Verify 节点升格: VerdictKind 4 态 + AggregateVerdicts + VerdictToExitReason + Evidence + SystemAnomaly + 14 ExitReason + G8-1 修复)

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
| **D7-S6** | **Error Aggregation & Metrics** | **HandleInterrupt errors.Join 聚合 + InterruptMetrics; sandbox cleanup observability (freefork + execute); WaveScheduler 4 新 metrics 字段; TaskManager panic counter** | **IMPLEMENTED** | `sessionorchestrator/{interrupt,metrics}.go` + `multiagent/provision/freefork/{forker,metrics}.go` + `multiagent/execute/{worker,metrics}.go` + `wavescheduler/scheduler.go` + `workmodel/task_manager{,_metrics}.go` |
| **D7-S9** | **Execute Artifact Data Contract (PR-C1)** | **ArtifactKind 4 类枚举（StateChangeCert/ResponseRecord/ProbeReport/ExperimentData）+ SideEffectStatus 5 态（None/Unknown/Inflight/Committed/RolledBack）+ wavescheduler.Artifact +5 字段 omitempty 向后兼容 + 跨域类型上提 shared/types 打破 import cycle** | **IMPLEMENTED (PR-C1)** | `internal/shared/types/execute.go` + `orchtypes/artifact_kind_alias.go` + `wavescheduler/types.go` |
| **D7-S8** | **Observation + UncertaintyReport (PR-A1 + PR-RF)** | **Observation 4 类（ObsFact/ObsSignal/ObsDeviation/ObsUncertainty）× 2 Category（CatBusiness/CatSystem）+ sealed Payload interface + UncertaintyReport Partition 不变式 + UncertaintyCoord Phase 2 扩展（FromVerifier/IsColdStart/Equal/With*）+ PR-RF 5 项 review fix（C1 IntentKind enum + C3 FromVerifier fail-fast + W2 fmt.Errorf wrap + W3 clamp01Float 合并 + W6/I8 Partition clamp 末尾）** | **IMPLEMENTED (PR-A1 + PR-RF, A15 模块)** | `internal/layers/orchestration/orchtypes/{observation,uncertainty_report,uncertainty_coord,errors}.go` |

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

### Requirement: D7-S8-A15 Observation + UncertaintyReport + UncertaintyCoord ✅ IMPLEMENTED

MUPS v4.3 Phase 2 Observe 节点的首批落地（PR-A1 + PR-RF）。Observe 节点必须有统一的数据契约：Observation struct + 4 类 `ObservationKind`（ObsFact/ObsSignal/ObsDeviation/ObsUncertainty）× 2 类 `Category`（CatBusiness/CatSystem）+ sealed `Payload` interface + `UncertaintyReport` 聚合 + `UncertaintyCoord` 增量扩展。本 Requirement 不引入 ProcessMessage wiring（PR-A4 范围），仅落地数据契约层。

**Priority:** P0
**Package:** `internal/layers/orchestration/orchtypes/{observation,uncertainty_report,uncertainty_coord,errors}.go`
**T:** D7-S8-A15-T01 … D7-S8-A15-T06
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase2-observe-plan/`

#### Scenario: Observation 4 类 × 2 Category + 不可变

- GIVEN 定义 `ObservationKind{ObsFact, ObsSignal, ObsDeviation, ObsUncertainty}` 与 `Category{CatBusiness, CatSystem}`
- WHEN 调用 `Observation.WithKind()` / `WithStrength()`
- THEN 返回新 Observation 实例，原实例未被修改
- AND 4 类 Kind 与 2 类 Category 正交（4×2 = 8 种组合，每种绑定 sealed Payload concrete type）
- AND `Strength ∈ [0, 1]` 边界保护（越界 panic via `clamp01Float(v, onNaN)`）
- AND `DetectedAt` 非零（零值 → `ErrObservationDetectedAtZero`）
- AND `MarshalJSON` wire format 为嵌套对象 `{"id": ..., "kind": ..., "category": ..., "strength": ..., "detected_at": ..., "payload": {...}}`

#### Scenario: UncertaintyReport Partition 不变式 + ComputeOverallStrength 仅遍历 CatBusiness

- GIVEN 10 个 Observation（6 CatBusiness + 4 CatSystem）
- WHEN 调用 `NewUncertaintyReport(observations)`
- THEN `report.BusinessObservations` 含 6 个，`report.SystemObservations` 含 4 个
- AND 6 + 4 == `len(report.Observations)` == 10
- AND 不变式被破坏时返回 `ErrUncertaintyReportPartitionInvariant`
- AND `report.ComputeOverallStrength()` 仅遍历 `BusinessObservations`，返回 avg(CatBusiness.Strength)
- AND CatBusiness 为空时 defaults 0.5（避免 NaN）

#### Scenario: UncertaintyCoord Phase 2 扩展（FromVerifier + Phase 1 兼容）

- GIVEN `VerdictKind=Pass, Confidence=0.9, Reason="ok"`
- WHEN 调用 `UncertaintyCoord.FromVerifier(verdict, confidence, reason)`
- THEN 返回 `UncertaintyCoord{Score: 0.9, Verdict: Pass, FromVerifier: true, Source: SourceVerifier, Reason: "ok"}`

- GIVEN 一个旧版本 JSON（仅有 `Value/UpdatedAt` 字段，Phase 1 wire format）
- WHEN `Unmarshal` 到新 `UncertaintyCoord`
- THEN `FromVerifier=false`（零值）+ `SideEffectStatus=""`（零值）
- AND `MarshalJSON` 使用 `omitempty` 不写零值字段，保持 Phase 1 调用方零修改

- GIVEN 一个未知 `VerdictKind` 值（不在白名单）
- WHEN 调用 `UncertaintyCoord.FromVerifier(unknown, ...)`
- THEN fail-fast 返回 `NewUncertaintyCoordInvalidVerdictKindError` + 错误码 `ORCH_COORD_VERDICT_7004`
- AND 不静默兜底（不返回零值 Coord）

#### Scenario: UncertaintyReport Anomalies subset + FilterByKind 跨 Category

- GIVEN 10 个 Observation 含 3 个 `ObsDeviation`（不论 Category）
- WHEN 调用 `report.Anomalies`
- THEN 返回 3 个 `ObsDeviation` Observation
- AND 调用 `report.FilterByKind(ObsDeviation)` 返回同样的 3 个（FilterByKind 故意遍历全集）

#### Scenario: Observation 字段校验 + 错误码

- GIVEN `FactPayload{Statement: ""}` 触发的 `validateFact` 失败
- WHEN 调用 `Observation.Validate()`
- THEN 返回 `fmt.Errorf("orchtypes: FactPayload.Statement empty: %w", ErrObservationPayloadInvalid)`（W2 包装）
- AND 错误可被 `errors.Is` 判定为 `ErrObservationPayloadInvalid`

#### Scenario: PR-RF 5 项 review fix 闭环

- **C1**: `QuantizedIntent.Kind` 从 `string` 改为 `IntentKind` enum（消除 PR-A2 translation shim）
- **C3**: `FromVerifier` 未知 verdict fail-fast + `ORCH_COORD_VERDICT_7004` 错误码
- **W2**: `validateFact` 改 `fmt.Errorf("orchtypes: FactPayload.Statement empty: %w", ErrObservationPayloadInvalid)` 包装
- **W3**: `clamp01` + `clamp01Coord` 合并为 `clamp01Float(v, onNaN)` 单函数
- **W6/I8**: `Partition` 末尾 `r.Overall = clamp01Float(r.ComputeOverallStrength(), 0.5)`（NaN safe）

注：C2/W8（`MatchKind` 签名收紧为 `(*UncertaintyReport)`）落点为后续 PR-B1，本 Requirement 不涉及。

---

### Requirement: D7-S9-A25 Execute Artifact Data Contract ✅ IMPLEMENTED

Phase 3 Execute 节点的最小风险入口（PR-C1）。提供跨域共享的 Artifact 数据契约：
ArtifactKind 4 类枚举 + SideEffectStatus 5 态 + SideEffectDetail 结构 + Artifact struct 5 字段升级。
本 Requirement 不引入执行链路变更（PR-C2..C7 范围），仅落地数据契约层。

**Priority:** P0
**Package:** `internal/shared/types/` + `internal/layers/orchestration/orchtypes/` + `internal/layers/orchestration/wavescheduler/`
**T:** D7-S9-A25-T01 … D7-S9-A25-T04
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase3-execute/`

#### Scenario: ArtifactKind 4 类枚举 + snake_case wire format

- GIVEN 定义 `shared/types.ArtifactKind{StateChangeCert, ResponseRecord, ProbeReport, ExperimentData}`
- WHEN 任意 Artifact 实例
- THEN `Kind` 字段为 4 枚举之一（uint8 0-3）
- AND `String()` 输出 snake_case wire format：`"state_change_cert"` / `"response_record"` / `"probe_report"` / `"experiment_data"`
- AND `MarshalJSON()` 输出字符串（不是数字），便于 D5 dashboard 字符串过滤
- AND `UnmarshalJSON()` 接收字符串，未知值 fail-fast 返回带 kind 名称的 error（不静默兜底）

#### Scenario: SideEffectStatus 5 态 + IsTerminal/NeedsAttention 派生

- GIVEN 定义 `shared/types.SideEffectStatus{None, Unknown, Inflight, Committed, RolledBack}`（string alias）
- WHEN 任意 Artifact / UncertaintyCoord 实例
- THEN `SideEffectStatus` 字段为 5 枚举之一（字符串值 `"none"` / `"unknown"` / `"inflight"` / `"committed"` / `"rolled_back"`）
- AND `IsTerminal()` 返回 true 当 status ∈ {None, Committed, RolledBack}
- AND `NeedsAttention()` 返回 true 当 status ∈ {Unknown, Inflight}
- AND `orchtypes.SideEffectStatus = types.SideEffectStatus` type alias，Phase 2 UncertaintyCoord 调用方零修改

#### Scenario: wavescheduler.Artifact struct 5 字段升级（v2 JSON 向后兼容）

- GIVEN `wavescheduler.Artifact` 扩展 5 字段：`Kind` / `SourcePlanID` / `AnomaliesCount` / `SideEffectStatus` / `SideEffectDetail`
- WHEN 任意 Artifact 实例序列化
- THEN 5 字段全部带 `omitempty` JSON tag
- AND zero value（Kind=0/SourcePlanID=""/AnomaliesCount=0/SideEffectStatus=""/*SideEffectDetail=nil）不出现在 JSON 输出
- AND v2 调用方（仅写 v2 字段）序列化结果与升级前**字节相同**
- AND Unmarshal 接收 v2 JSON（5 字段缺失）不报错，零值默认到 `Kind=0 (StateChangeCert)` / `SideEffectStatus="" (None)` 等

#### Scenario: 跨域类型上提 shared/types 打破 import cycle

- GIVEN orchtypes → workmodel → wavescheduler 单向依赖链
- WHEN Artifact.SideEffectStatus 需要与 UncertaintyCoord.SideEffectStatus 同类型（跨域 wire format 统一）
- AND 直接 wavescheduler → orchtypes 双向引用会破环
- THEN 把 `ArtifactKind` + `SideEffectStatus` + `SideEffectDetail` 上提到 `internal/shared/types/execute.go`（Phase 1 `MemoryEntry` precedent）
- AND `orchtypes` 提供 type alias + const re-export（`type SideEffectStatus = types.SideEffectStatus`）保持 Phase 2 调用方零修改
- AND `shared/types` → orchtypes 单向依赖，无 cycle

---

### Requirement: D7-S8-A22 Plan Data Contract + Planner (Phase 2 PR-B1) ✅ IMPLEMENTED

MUPS v4.3 Phase 2 Plan 节点的数据契约层（PR-B1）。提供 Plan struct + 4 类 `PlanKind` 枚举 + Planner interface + DefaultPlanner.MatchKind 4 规则分类器。本 Requirement 落地后：
- Phase 3 PR-C2 ChannelRouter 直接消费 `plan.Plan` 进行 PlanKind → Channel 路由；
- Phase 4 Verify 通过 `Plan.SourceObservationIDs` 反向追溯 `Observation`；
- Phase 5 Learn 通过 `Plan.AnomaliesCount` 调整 ReputationEvidence。

**Priority:** P0
**Package:** `internal/layers/orchestration/plan/`
**T:** D7-S8-A22-T01 … D7-S8-A22-T03
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase2-plan/`

#### Scenario: PlanKind 4 类枚举 + snake_case wire format

- GIVEN 定义 `plan.PlanKind{CommitmentPlan, ProtocolPlan, ScenarioPlan, ExplorationPlan}`（uint8 1-4）
- AND 零值 `KindUnset` 显式区分（避免与任意 4 命名值混淆）
- WHEN 任意 Plan 实例
- THEN `Kind` 字段为 4 命名枚举之一（`IsKnown()` 返回 true）
- AND `String()` 输出 snake_case wire format：`"commitment_plan"` / `"protocol_plan"` / `"scenario_plan"` / `"exploration_plan"`
- AND `MarshalJSON()` 输出字符串（KindUnset omitempty 返回 null，未知值返回 error），便于 D5 dashboard 字符串过滤
- AND `UnmarshalJSON()` 接收字符串，未知值 fail-fast 返回带 kind 名称的 error（不静默兜底到 KindUnset）
- AND `ParsePlanKind(s)` CLI 反向解析（case-insensitive + trim whitespace）

#### Scenario: Plan.SourceObservationIDs 必填 + Phase 4 Verify 反向追溯入口

- GIVEN Plan 是 Phase 2 Observe 节点产出的可调度结构
- WHEN 任意 Plan 进入 ChannelRouter（Phase 3 PR-C2）或 Verifier（Phase 4）
- THEN `Plan.SourceObservationIDs` 必填（空 → `Validate()` 失败，返回 `ErrPlanSourceObservationIDsRequired` + 错误码 `PLAN_LINEAGE_8002`）
- AND `NewPlan` 防御性拷贝外部 slice（外部 mutation 不影响 Plan 内字段）
- AND `Plan.ReverseLookupObservations([]ObservationLookup)` 按 ID 集合求交，返回 `ObservationLookup` 子集（Phase 4 Verify 反向追溯入口）
- AND 重复 ID 在 SourceObservationIDs 中不产生重复结果（defensive copy 保证）
- AND 空输入（nil / empty）返回 nil（边界保护）

#### Scenario: MatchKind 4 规则分类器 + uncertainty-first tie-break

- GIVEN `MatchKind(quantizedKind string, stepCount int, anomaliesCount int) PlanKind`
- WHEN 4 规则按优先级求值：
  - **Rule 1**: `quantizedKind == "intent_orchestrate"` OR `anomaliesCount >= 3` → `ExplorationPlan`
  - **Rule 2**: `stepCount == 1` → `CommitmentPlan`
  - **Rule 3**: `quantizedKind == "intent_command"` OR `stepCount <= 3` → `ProtocolPlan`
  - **Rule 4**: 默认 → `ScenarioPlan`
- THEN 不确定性优先 tie-break：Rule 1 始终优于 Rule 2/3（exploration 比 commitment 更可回滚）
- AND `DefaultPlanner.Plan(input PlanInput)` 走完整集成：
  - 空 `ObservationIDs` fast-fail 返回 `ErrPlanSourceObservationIDsRequired`
  - 强度公式 `strengthFloor(anomalies, observations) = 0.7 − 0.1·anomalies + min(observations·0.02, 0.2)`（cap [0, 1]）
  - `Validate()` 失败透传（`ErrPlanFailureCriteriaEmpty` + `PLAN_PP2_EMPTY_8020` 等）
  - `BlastRadius` 透传到输出 Plan
  - `AnomaliesCount` 透传（Phase 4 Verify 用以关联 Observation anomalies）

---

### Requirement: D7-S9-A26 Execute 4 Channel + ChannelRouter (Phase 3 PR-C2) ✅ IMPLEMENTED

MUPS v4.3 Phase 3 Execute 节点的 4 类执行通道 + ChannelRouter（PR-C2）。Channel interface 把 4 PlanKind 路由到对应的 Execute 策略：
- `CommitChannel` (CommitmentPlan) → 1-Step 同步 ToolRunner 调用 → `ArtifactStateChangeCert`
- `ProtocolChannel` (ProtocolPlan) → 顺序多步 + 失败 reverse-order rollback → `ArtifactResponseRecord`
- `ScenarioChannel` (ScenarioPlan) → 并行探测 (MaxParallel=5) + 多数派投票 → `ArtifactProbeReport`
- `ExplorationChannel` (ExplorationPlan) → 多 agent 并行 + 优先级排序 + PersistScope 派生 → `ArtifactExperimentData`

ChannelRouter 走 `Plan.Kind` → `ChannelRegistry.Get(kind)` → `Channel.Execute` 三段式分发；ToolRunner 是本地 interface（PR-C2 隔离 PR-C4 ToolSpec v3 + PR-C7 DefaultExecutor）。

**Priority:** P0
**Package:** `internal/layers/orchestration/execute/`
**T:** D7-S9-A26-T01 … D7-S9-A26-T05
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase3-channels/`

#### Scenario: ChannelRegistry 1:1 绑定 + ChannelRouter 4 PlanKind 路由

- GIVEN 4 Channel 实现（Commit/Protocol/Scenario/Exploration）+ ChannelRegistry
- WHEN Register 全部 4 Channel
- THEN `ChannelRegistry.Len() == 4` 且 0 冲突（PlanKind → Channel 1:1）
- AND `ChannelRegistry.Get(planKind)` 返回对应 Channel；未注册时返回 `ErrChannelNotFound` + 错误码 `EXEC_CHANNEL_9001`
- AND 重复 Register 同一 PlanKind 返回 `ErrChannelUnsupported` + 错误码 `EXEC_CHANNEL_9002`（wiring conflict）

- GIVEN ChannelRouter 装载 4 Channel
- WHEN `Route(ctx, plan, ChannelRequest{SessionID: ...})` 4 种 PlanKind 各跑一次
- THEN 返回 4 类 ArtifactKind 1:1 对应：
  - `CommitmentPlan` → `ArtifactStateChangeCert`（PR-C1 §1 wire format）
  - `ProtocolPlan` → `ArtifactResponseRecord`
  - `ScenarioPlan` → `ArtifactProbeReport`
  - `ExplorationPlan` → `ArtifactExperimentData`
- AND `Artifact.SourcePlanID == plan.ID`（反向追溯）
- AND `Artifact.SessionID == req.SessionID`（线程绑定）
- AND `Route(ctx, nil, _)` 返回 `ErrChannelPlanNil`；`Route(ctx, plan{Kind: KindUnset}, _)` 返回 `ErrChannelUnsupported`（defensive checks）

#### Scenario: CommitChannel 1-Step 同步 + IdempotencyKey 强制

- GIVEN CommitChannel 装载 ToolRunner
- WHEN 单 Step Plan（`len(Steps) == 1`）+ IdempotencyKey 非空
- AND ToolRunner 返回 `ToolResult{ExitCode: 0}`
- THEN Artifact 字段：
  - `Kind == ArtifactStateChangeCert`
  - `SideEffectStatus == SideEffectCommitted`
  - `SideEffectDetail.IdempotencyKey == step.IdempotencyKey`
  - `SideEffectDetail.ConfirmedAt > SentAt`（确认时间戳）

- WHEN ToolRunner 返回 `context.DeadlineExceeded`（内或外 ctx）
- THEN `SideEffectStatus == SideEffectInflight`（PR-C3 StrategyDecider 路由到 `StrategyAskNow`）

- WHEN 多 Step Plan 或 Step 缺 IdempotencyKey
- THEN 返回 `ErrChannelStepCountMismatch` + 错误码 `EXEC_CHANNEL_9003`（fail-fast，避免无幂等性的 side effect）

#### Scenario: ProtocolChannel 顺序多步 + reverse-order rollback

- GIVEN ProtocolChannel 装载 ToolRunner
- WHEN 3 步 Plan（login → fetch → parse）全部成功
- THEN Artifact 字段：
  - `Kind == ArtifactResponseRecord`
  - `SideEffectStatus == SideEffectCommitted`
  - `Summary` 含 `step_0` / `step_1` / `step_2` 三个 step 输出（顺序）

- WHEN 第 2 步（fetch）失败
- THEN 已执行的 step 1（login）被 reverse-order rollback（ToolRunner 收到 `__rollback: true` 标记）
- AND Artifact `SideEffectStatus == SideEffectRolledBack`
- AND Runner 调用次数 = 3（login + fetch-failed + login-rollback）

- WHEN `len(Steps) == 0`
- THEN 返回 `ErrChannelStepCountMismatch`（protocol 必须有 ≥1 步）

#### Scenario: ScenarioChannel 并行探测 + 多数派投票

- GIVEN ScenarioChannel 装载 ToolRunner + `MaxParallel=5`
- WHEN 5 个 probe Step 并行执行
- THEN 至少 2 个 probe 同时 in-flight（验证 parallelism，避免串行退化）
- AND Artifact `Kind == ArtifactProbeReport`
- AND `SideEffectStatus == SideEffectNone`（read-only 探测，无 side effect）

- GIVEN 5 个 probe 中 3 个成功 / 2 个失败
- WHEN 调用 `Execute`
- THEN `Summary` 含 `"3/5 probes succeeded"` + threshold `> len/2 = 2`
- AND `ExitCode == 0`（多数派 pass）

- WHEN 5 个 probe 中 2 个成功 / 3 个失败（多数派 reject）
- THEN `ExitCode == 1` + 错误返回（`ErrChannelStepCountMismatch` 复用）

#### Scenario: ExplorationChannel 多 agent + 优先级排序 + PersistScope 派生

- GIVEN ExplorationChannel 装载 ToolRunner + `MaxParallel=3`
- WHEN 3 个 Step 并行执行
- THEN Artifact `Kind == ArtifactExperimentData`
- AND `SideEffectStatus` 由 `Plan.BlastRadius.PersistScope` 派生：
  - `PersistTransient` → `SideEffectNone`（read-only 沙箱）
  - `PersistSession` / `PersistPermanent` → `SideEffectCommitted`
  - 未知 scope → `SideEffectUnknown`（Phase 4 必须 verify）
- AND 容忍部分失败（free-fork 语义）：1/2 success 不触发 error
- AND 优先级排序：(1) 无 error → (2) `duration` 短 → (3) `EstimatedTokens` 少
- AND `Summary` 含 `"top_result: <winning_tool>"` + `"<n>/<m> succeeded"`

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

### Requirement: SubTurn 3-Mode Context Isolation (devrix-context-budget-phase-b)

`SubTurnRunner.RunSubTurn` (D7-S2-A06) MUST dispatch sub-agent context
inheritance by `req.Mode` and reject requests that exceed the configured
recursion depth. Three modes align with the clawcode 3-mode pattern:

- `brief` (default) — `PreloadedMessages=nil`; child starts with a fresh
  history. Saves tokens at the cost of parent context visibility.
- `fork` — `PreloadedMessages=BuildForkedMessages(parent, directive)`;
  byte-level stable prefix shared across sibling fork children (prompt
  cache friendly). Includes the synthesized `"Fork started — processing
  in background"` placeholder.
- `full` — `PreloadedMessages=messagesWithoutLastUser(parent)`; legacy
  behavior, opt-in only for callers that need full parent history (D5
  evaluation).

When `req.Mode` is empty, `SubTurnRunner` falls back to
`SubagentConfig.DefaultMode` (or `LegacyMode` when set). When
`req.Depth >= SubagentConfig.MaxDepth`, the runner MUST return
`ErrSubagentDepthExceeded` (code `AGT_DEPTH_5011`) WITHOUT invoking the
LLM. Supersedes Phase A `SubTurnRunner` legacy default.

**Priority**: P0  
**T mapping**: D7-S2-A06-T14 … T17, `internal/layers/orchestration/turn/subturn_test.go`

#### Scenario: brief mode drops parent history

- GIVEN a parent session with N messages and `req.Mode=brief`
- WHEN `SubTurnRunner.RunSubTurn` is called
- THEN the LLM sees exactly 1 message (the last user / directive)
- AND `PreloadedMessages` is nil

<!-- T: D7-S2-A06-T14 -->

#### Scenario: fork mode produces cache-friendly prefix

- GIVEN a parent session with assistant + tool_result messages
- WHEN `req.Mode=fork` and 10 sibling children are spawned in parallel
- THEN each child sees the same byte-level identical prefix
- AND the prefix contains the literal `"Fork started — processing in background"`

<!-- T: D7-S2-A06-T15 -->

#### Scenario: full mode preserves legacy behavior

- GIVEN `req.Mode=full`
- WHEN `SubTurnRunner.RunSubTurn` is called
- THEN `PreloadedMessages` equals `messagesWithoutLastUser(req.Messages)`
- AND the result is byte-equal to the pre-Phase-B legacy dispatch

<!-- T: D7-S2-A06-T15 (D2-S15-A08-T07 boundary) -->

#### Scenario: empty Mode falls back to DefaultMode

- GIVEN `req.Mode=""` and `Cfg.DefaultMode="brief"`
- WHEN `SubTurnRunner.RunSubTurn` is called
- THEN the runner dispatches as if `Mode=brief`

<!-- T: D7-S2-A06-T17 -->

#### Scenario: LegacyMode override wins over DefaultMode

- GIVEN `Cfg.DefaultMode="brief"` and `Cfg.LegacyMode="full"`
- WHEN `req.Mode=""` is sent
- THEN the runner dispatches as `Mode=full` (legacy back-compat)

<!-- T: D7-S2-A06-T17 -->

#### Scenario: invalid Mode is rejected before LLM call

- GIVEN `req.Mode="unknown"`
- WHEN `SubTurnRunner.RunSubTurn` is called
- THEN the runner returns `ErrSubagentInvalidMode` (code `AGT_DEPTH_5012`)
- AND the LLM is not invoked

#### Scenario: Depth at MaxDepth is rejected

- GIVEN `Cfg.MaxDepth=3` and `req.Depth=3`
- WHEN `SubTurnRunner.RunSubTurn` is called
- THEN the runner returns `ErrSubagentDepthExceeded` (code `AGT_DEPTH_5011`)
- AND the error message hints to retry with `mode=brief`
- AND the LLM is not invoked

<!-- T: D7-S2-A06-T16 -->

#### Scenario: Depth at MaxDepth-1 is allowed

- GIVEN `Cfg.MaxDepth=3` and `req.Depth=2`
- WHEN `SubTurnRunner.RunSubTurn` is called
- THEN the LLM is invoked normally

<!-- T: D7-S2-A06-T16 -->

---

### Requirement: HandleInterrupt errors.Join Aggregation (devrix-d7-error-aggregation-and-metrics)

`InterruptHandler.Handle` MUST aggregate errors from the 3 cancel steps (Wave → D4 → Process) via `errors.Join` instead of the previous "warn all + return nil" anti-pattern. The "stopped" EngineEvent emission remains best-effort regardless of cancel outcomes.

**Priority:** P0
**Package:** `internal/layers/orchestration/sessionorchestrator/`
**T:** D7-S6-A11-T01, T02, T03

#### Scenario: All 3 cancel steps fail

- GIVEN WaveCanceler / DelegateCanceler / ProcessCanceler all return non-nil errors
- WHEN `InterruptHandler.Handle(ctx, sessionID)` is called
- THEN the returned error is `errors.Join(waveErr, d4Err, procErr)`
- AND `errors.Is(returned, waveErr)` is true for each underlying error
- AND `InterruptMetrics.Snapshot.WaveCancelFailed == 1`
- AND `InterruptMetrics.Snapshot.D4CancelFailed == 1`
- AND `InterruptMetrics.Snapshot.ProcessCancelFailed == 1`
- AND `InterruptMetrics.Snapshot.HandleErrored == 1`
- AND the "stopped" EngineEvent is still emitted best-effort

#### Scenario: Partial failure

- GIVEN 1 of 3 cancelers fails (others return nil)
- WHEN `Handle` is called
- THEN the returned error contains only the failed step's wrapped error
- AND `InterruptMetrics.HandleErrored == 1`

#### Scenario: Backward-compat with nil Metrics

- GIVEN `InterruptOptions{Metrics: nil}` (pre-PR-A callers)
- WHEN `Handle` is called
- THEN `errors.Join` is still returned (no panic, no metric increments)
- AND the "stopped" event is still emitted if Sink is non-nil

### Requirement: Sandbox Cleanup Observability (devrix-d7-error-aggregation-and-metrics)

All `Sandbox.Exit` call sites across `freefork.Forker`, `execute.Executor`, and `wavescheduler.WorkerRunner` MUST surface Exit failures via atomic counter + `slog.Warn`, replacing the previous `_ = Sandbox.Exit(...)` silent-swallow pattern. **Note (2026-06-22):** the `sandbox_exit_failed` counter is owned by D4 (`multiagent/execute/worker.go::recordSandboxExitFailed`); the D7 wavescheduler does NOT emit it. See D7-S6-A14-T03 for the cross-domain clarification.

**Priority:** P0
**Package:** `internal/layers/multiagent/provision/freefork/` + `internal/layers/multiagent/execute/` + `internal/layers/orchestration/wavescheduler/`
**T:** D7-S6-A12-T01 (OBSOLETE — see D7-S6-A14-T03), D7-S6-A12-T02, T03, T04, T05, T06

#### Scenario: Forker sandbox Exit failure

- GIVEN `DefaultForker.WithMetrics(&ForkerMetrics{})` is set
- AND a rollback path triggers `Sandbox.Exit` that returns an error
- WHEN the rollback completes
- THEN `ForkerMetrics.Snapshot.SandboxExitFailed == 1`
- AND `slog.Warn("freefork: sandbox exit failed", ...)` is emitted

#### Scenario: Executor sandbox Exit failure

- GIVEN `Executor.WithMetrics(&ExecutorMetrics{})` is set
- WHEN `ExecuteSync` / `ExecuteAsync` / `forkWorker` cleanup paths trigger `Sandbox.Exit` failures
- THEN `ExecutorMetrics.Snapshot.SandboxExitFailed` is incremented (3 potential sites)
- AND each failure emits `slog.Warn("execute: sandbox exit failed", where=..., sessionID=..., ...)` with `where` distinguishing the call site

#### Scenario: TaskManager.publishCompletion panic

- GIVEN `notify.GlobalBus().Publish` (or a panicking subscriber) panics during `publishCompletion`
- WHEN the deferred recover catches the panic
- THEN `TaskManagerMetrics.Snapshot.PublishCompletionPanics.Add(1)` is incremented
- AND `slog.Error("taskmanager: publishCompletion panic", session=..., item_id=..., panic=..., metric="publish_completion_panics")` is emitted
- AND the publishCompletion goroutine does not crash the process

### Requirement: Forker errors.Join Aggregation + Backward Compatibility (devrix-d7-error-aggregation-and-metrics)

`DefaultForker.Fork` MUST aggregate errors from N concurrent spawn failures via `errors.Join`, replacing the previous `return nil, errs[0]` pattern that dropped N-1 errors. The `WithMetrics` setter MUST be backward-compatible with the 13 existing `NewDefaultForker` callers that don't explicitly wire metrics.

**Priority:** P0
**Package:** `internal/layers/multiagent/provision/freefork/`
**T:** D7-S6-A13-T07

#### Scenario: All N forks fail

- GIVEN N ForkRequests with a stub factory that fails every Create
- WHEN `Fork(ctx, parent, reqs)` is called
- THEN the returned error is `errors.Join(err1, err2, ..., errN)`
- AND each fork's name appears in the joined error message
- AND `errors.Is(returned, factoryErr)` is reachable

#### Scenario: Backward-compat with nil metrics

- GIVEN `NewDefaultForker(deps)` without `WithMetrics` call
- WHEN `Fork` is called
- THEN the call does not panic
- AND errors.Join aggregation still works (zero overhead)

#### Scenario: Hard assertion on leftover sandbox

- GIVEN a failed fork batch where rollback cleans up sandbox dirs
- WHEN the test inspects `wt.base` for leftover subdirectories
- THEN `t.Errorf` fires on any leftover (was `t.Logf` pre-PR-A, masking latent cleanup failures)

---

### Requirement: Metrics Naming Alignment & Concurrency Hardening (devrix-d7-metrics-and-concurrency-hardening)

D7-S6-A14 closes the remaining P0/P1 follow-ups from DM-20260621-010 (PR-B worktree
metrics) and the DM-20260621-009 deep review. It (1) aligns metric names with the
spec text so D5 dashboards can filter by name, (2) hands `sandbox_exit_failed`
ownership to its real emitter in D4, (3) bounds `state.cancels` and
`state.handles` so long-lived sessions with repeated wave re-entries do not
leak context-cancel references, (4) routes the dispatchLoop hot path through
`ConflictGuard.AllowAndRegister` to eliminate the TOCTOU window, and (5) hardens
`CommandHandler` against consumer stalls via `select-default` emit.

**Priority:** P1
**Package:** `internal/layers/orchestration/wavescheduler/` + `internal/layers/orchestration/sessionorchestrator/`
**T:** D7-S6-A14-T01 … D7-S6-A14-T06

#### Scenario: dispatch_loop_wakeups spec-aligned plural

- GIVEN `WaveScheduler.dispatchLoop` is running
- WHEN either `<-state.wakeupCh` or `<-ticker.C` fires
- THEN `s.incMetric("dispatch_loop_wakeups")` is invoked (plural, matches spec)
- AND D5 dashboards keyed on `dispatch_loop_wakeups` observe the count

#### Scenario: worker_panics spec-aligned plural

- GIVEN a worker goroutine inside `dispatchOne`
- WHEN the deferred recover catches a panic
- THEN `s.incMetric("worker_panics")` is invoked (plural, matches spec)
- AND `slog.Error(..., "metric", "worker_panics")` carries the matching key

#### Scenario: sandbox_exit_failed owned by D4 (cross-domain reference)

- GIVEN `multiagent/execute/worker.go::recordSandboxExitFailed` is the only emitter
- WHEN `sandbox_exit_failed` appears on the D5 dashboard
- THEN the counter comes from `ExecutorMetrics.SandboxExitFailed` (D4-S6-*)
- AND the D7 `SchedulerMetrics` struct does NOT carry a `sandbox_exit_failed`
  field (D7-S6-A12-T01 is OBSOLETE)
- AND `openspec/specs/d7-orchestration/t-registry.md` references D4-S6-A12-Txx
  for the actual owner

#### Scenario: state.cancels released after wave completion

- GIVEN a wave with N dispatched tasks
- WHEN `markWaveDone` runs (any terminal path: all-complete, ctx-cancel,
  or zero ready nodes)
- THEN `state.cancels` is `nil` (length 0)
- AND `state.handles` is an empty map
- AND a follow-up wave re-entry on the same session starts with empty bookkeeping

#### Scenario: ConflictGuard hot path uses AllowAndRegister

- GIVEN `WaveScheduler.dispatchLoop` iterating over ready nodes
- WHEN `dispatchOne` is called for each candidate
- THEN the conflict check is the atomic `AllowAndRegister` (single mutex
  acquisition covering both the conflict scan and the slot registration)
- AND `dispatchLoop` no longer pre-checks `guard.Allow` (which would leave
  a TOCTOU window between Allow and Register)
- AND the `go test -race` run for `TestD7S6A14T05_DispatchLoop_HotPathUsesAllowAndRegister`
  completes without race-detector findings

#### Scenario: CommandHandler emit drops events when consumer is wedged

- GIVEN `CommandHandler.Handle` returning a buffered channel (cap=4)
- WHEN the consumer fails to drain within the buffer capacity
- THEN `emit` uses `select { case out <- ev: default: slog.Warn(...) }`
- AND the Handle goroutine returns within bounded latency
- AND a `slog.Warn("command_handler: out channel full, drop event", ...)` is emitted

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
| **4.0.0** | **2026-06-21** | **devrix-d7-error-aggregation-and-metrics (DM-20260621-010)**：(1) D7-S6 新增 Scenario「Error Aggregation & Metrics」覆盖 HandleInterrupt errors.Join + sandbox cleanup observability + Forker errors.Join；(2) ADDED Requirements：D7-S6-A11 (interrupt errors.Join 3 步 cancel 聚合) + D7-S6-A12 (sandbox cleanup observability freefork+execute+taskmanager) + D7-S6-A13 (forker errors.Join + 13 调用方 backward compat)；(3) Scenarios 表新增 D7-S6 行；(4) Archived Changes 表新增 DM-20260621-010 引用 |
| **4.1.0** | **2026-06-23** | **devrix-d7-mups-v4-phase3-execute (DM-20260625-001) Phase 3 PR-C1**：(1) ADDED Requirement D7-S9-A25 (Execute Artifact Data Contract): ArtifactKind 4 类枚举 + SideEffectStatus 5 态 + SideEffectDetail + wavescheduler.Artifact +5 字段 (Kind/SourcePlanID/AnomaliesCount/SideEffectStatus/SideEffectDetail) omitempty 向后兼容；(2) 跨域类型上提 shared/types/execute.go 打破 orchtypes→workmodel→wavescheduler 潜在 import cycle（Phase 1 MemoryEntry precedent）；(3) 4 个新 P0 T 点 D7-S9-A25-T01..T04 IMPLEMENTED；(4) t-registry v3.8.0→v3.9.0 (T 129→133, P0 96→100)；(5) Archived Changes 新增 DM-20260625-001 引用 |
| **4.2.0** | **2026-06-23** | **devrix-d7-mups-v4-phase2-observe-plan (DM-20260623-001) Phase 2 PR-A1 + PR-RF**：(1) ADDED Requirement D7-S8-A15 (Observation + UncertaintyReport + UncertaintyCoord): Observation 4 类 ObservationKind × 2 Category + sealed Payload interface + UncertaintyReport Partition 不变式 + ComputeOverallStrength 仅遍历 CatBusiness + UncertaintyCoord Phase 2 扩展 (FromVerifier/IsColdStart/Equal/With*) + 11 SentinelError + 4 错误码 (7001-7004)；(2) PR-RF 5 项 review fix 闭环（C1 IntentKind enum + C3 FromVerifier fail-fast + W2 fmt.Errorf wrap + W3 clamp01Float 合并 + W6/I8 Partition clamp 末尾）；(3) 6 个新 P0 T 点 D7-S8-A15-T01..T06 IMPLEMENTED；(4) t-registry v3.9.0→v3.10.0 (T 133→139, P0 100→106)；(5) Scenarios 表新增 D7-S8 行（Observation + UncertaintyReport PR-A1 + PR-RF, A15 模块）；(6) Archived Changes 新增 DM-20260623-001 引用 |

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
