# D7 Orchestration Domain Specification

**Capability:** d7-orchestration
**Domain:** D7
**DSAFT Type:** 核心域 (Core Domain)
**Version:** 4.22.0
**Status:** Canonical — source of truth
**Last Updated:** 2026-07-01 (devrix-d7-historical-s-cleanup DM-20260701-003: S3 定位澄清 + S7+ 正文迁出 historical-s-mapping.md)
**Domain SoT:** `d7-domain.md`
**Layering Spec:** `openspec/specs/architecture/layering.md`
**Parent Change:** devrix-d7-orchestration-domain (DM-20260613-001)

> **精简设计契约（Lite-Mode）**：本文档只放当前符合代码的设计契约。**过程需求迭代**（174 个完整 Gherkin Scenario 详细文本）**不进入本文件**，留在 `archive/<change-id>/specs/` 各 change 归档目录。详细时间线见 [CHANGELOG.md](CHANGELOG.md)。

---

## Overview

D7 编排域回答 **"做什么、按什么顺序做、谁来做、做得怎么样了"**。作为 **横向协调层** 编排 D2（LLM↔Tool 执行原语）与 D4（Agent 委托原语），并向 D1 发布进度事件（D1 仍拥有 ingress）。

**现行实现路径（v4.21 canonical）**：D7 current S 层固定为 S1-S6：S1 `workmodel/`、S2 `sessionorchestrator/`、S3 `wavescheduler/`、S4 `executionflow/{hub,workplan,imsink,bridge}/`、S5 `plan/` + `decisionplanning/`、S6 `sessionorchestrator/item_pipeline.go` + `mups/` + `escape/` + `interfaces/`。D1 主入口 `sessionorchestrator.Entry.ProcessMessage`（`bootstrap/wire_coordinator.go::WireD7`）。用户消息主链路为 `RunSessionTurnLoop → ItemPipelineRunner → WorkTree`；retired `FastPath` / `OrchestratePath` 仅作历史追溯。

### 核心设计原则

1. **不可变 + 纯函数优先**：值对象（TaskSpec/TaskReport/Verdict）通过 `With*` 返回新副本；Engine 节点间通过 Channel 单向通信
2. **5 节点管道**：Observe → Plan → Execute → Verify → Learn 顺序执行 + LP-1/2/5 闭环（Learn → Observe / Learn → Skill / Plan → Verdict 反查）
3. **入口收敛**：CommandHandler / Skip 是显式控制链；普通用户消息统一进入 `RunSessionTurnLoop → ItemPipelineRunner`；retired FastPath/OrchestratePath 不再作为 current 架构链路
4. **Auto-Close 异步触发**：Verify 完成后异步触发 Learn，不阻塞 channel close（3 层 fail-safe：nil learner / Learn error / channel cancel）
5. **Pessimistic Commit L3 防御**：5 类触发（resource_exhausted / cb_l1 / indeterminate_3x / empty_evidence / manual_abort）+ Rule-based Fallback 4 候选规则
6. **CoW VersionChain 不变性**：所有 Task/Plan/Artifact 走 Copy-on-Write 版本链，避免 in-place mutation
7. **Trace 树全程贯穿**：每跳节点产生 child span；6 prior attributes（α/β/mean/track_mode/classifier_source/injected_at）由 sessionSpan 统一注入

### S 层职责

| S 层 | 名称 | 职责 | 上下游 |
|------|------|------|--------|
| D7-S1 | Work Model | WorkItem CRUD、依赖 DAG、磁盘持久化、PlanMode 状态机 | 写：被 S2/S3 写；读：被 S4 读 |
| D7-S2 | Session Orchestrator | 用户消息主入口、Turn 主循环、Intent 四链分发 | 调用 S1/S3/S4/S5；发布 D7-S4 事件 |
| D7-S3 | Wave Scheduler | TaskGraph DAG 调度、WorkerPool、ConflictGuard、ContextPolicy；**仅** explicit wave / background / delegate 触发，**不是**普通用户消息主链路 | 写 S1 状态；由 Plan/delegate 显式调用 |
| D7-S4 | Execution Flow | FlowEvent 聚合、WorkPlan 快照、IM 广播 | 读 S2/S3 events；写 D1 progress |
| D7-S5 | Decision & Planning | 意图分类、任务拆解、执行器选择、PlanKind 4 类 | 读 D2/D3 信号；驱动 S3 |
| D7-S6 | MUPS Governance | Execute / Verify / Learn / Escape / convergence governance | 读 S1/S5；写 S1 round/verdict/learning state |

---

## DSAFT 结构

| 层级 | ID | 名称 | 物理路径 |
|------|-----|------|----------|
| D | D7 | Orchestration | `internal/layers/orchestration/` |
| S | D7-S1 | Work Model | `workmodel/` + `sessionorchestrator/workmodel.go` |
| S | D7-S2 | Session Orchestrator | `sessionorchestrator/` |
| S | D7-S3 | Wave Scheduler | `wavescheduler/` |
| S | D7-S4 | Execution Flow | `executionflow/{hub,workplan,imsink,bridge}/` |
| S | D7-S5 | Decision & Planning | `plan/` + `decisionplanning/` |
| S | D7-S6 | MUPS Governance | `sessionorchestrator/` + `mups/` + `escape/` + `interfaces/` |
| A | A1-A99 | 22 个核心活动 | 见 `a-registry.md` |
| F | F1-F999 | 功能点编排 | 见 `f-registry.md` |
| T | T1-T200 | 测试点（T01-T180 IMPLEMENTED，T181-T200 PLANNED） | 见 `t-registry.md` |

**当前计数（v4.21.0）**：D=1, S=6 (canonical), A=22 current + historical mapped, F=120+, T=188+（IMPLEMENTED）。Canonical 5 节点 = S5 Observe/Plan + S6 Execute/Verify/Learn，不再作为独立 S 层扩张。

---

## Scenarios

| ID | Scenario | Responsibility | Status | 代码位置 |
|----|----------|----------------|--------|----------|
| D7-S1 | Work Model | **WorkItem** CRUD、依赖 DAG、磁盘持久化（schema v2）、PlanMode 状态机 | **IMPLEMENTED** | `workmodel/work_tree.go` + `workitem.go` + `sessionorchestrator/workmodel.go` |
| D7-S2 | Session Orchestrator | ProcessMessage、FastPath、TurnLoop、Session Deliverable Gate（ExtractSessionDeliverable + LastTextQualityGate） | **IMPLEMENTED** | `sessionorchestrator/` |
| D7-S3 | Wave Scheduler | TaskGraph DAG、5-slot 池、ContextPolicy、ConflictGuard；**explicit wave/background 调度**，非 ItemPipeline 主链路 | **IMPLEMENTED** | `wavescheduler/` |
| D7-S4 | Execution Flow | Hub 双通道发布、WorkPlan 读模型、IM worker_progress、SpokeBridge | **IMPLEMENTED** | `executionflow/` |
| D7-S5 | Decision & Planning | PlanAgent、规则+LLM 分类、PlanKind 4 类 + DefaultPlanner + **StrategicPlanProposer (A76)** | **IMPLEMENTED** | `plan/` + `decisionplanning/` + `sessionorchestrator/strategic_plan_proposer.go` |
| D7-S6 | MUPS Governance | Execute / Verify / Learn / Escape / Pessimistic / convergence governance；承载 D7-S6 横切 hardening | **IMPLEMENTED** | `sessionorchestrator/item_pipeline.go` + `workitem_executor.go` + `deliverable_verify.go` + `mups/` + `escape/` + `interfaces/` |

### Historical / Contract Mapping（非 current S）

> 完整 A/F 正文与 v6.0.0 14→6 重映射见 **`historical-s-mapping.md`**。下表为 current 追溯索引。

| Historical ID | Current Target | Meaning |
|---------------|----------------|---------|
| D7-S7 | D7-S6-A01 | MUPS 5 节点管道入口 |
| D7-S8 | D7-S5-A06 | Observe + UncertaintyReport |
| D7-S9 | D7-S6-A02 | Execute Artifact / WorkItemExecutor |
| D7-S10 | D7-S6-A03 | Verify Verdict / Deliverable Gate |
| D7-S11 | D7-S6-A04 | Learn Node / Reputation / Memory |
| D7-S12 | D7-S6-A05 | Observe-Learner 闭环 |
| D7-S13 | D7-S2-A07 + D7-S6-A03 | AutoClose / session completion |
| D7-S14 | D7-S6-A06 | MUPS v5 EscapeEngine |
| D7-S15 | D7-S1-A07 | WorkItem Rollup |
| D7-S16 | D7-S1-A08 + D7-S5-A07 | Layer SubContext / ScopeContract / StrategicPlanProposer |
| D7-S18 | D7-S6-A07 | Pessimistic + Fallback |
| D7-S20 | Contract → D7-S1/D7-S6 | TaskSpec 下行契约 |
| D7-S21 | Contract → D7-S1/D7-S6 | TaskReport 上行契约 |

---

## Architecture

> **5 节点管道端到端链路 + LP-1/2/5 闭环 + Auto-Close 异步触发的完整运行时序**，见 `pipeline-architecture.md`（MUPS v4.3 Phase 1-7 全部 S7_Archived 后的端到端总图）。

```
D1 Gateway.RouteInbound
    └── D7-S2 SessionOrchestrator.ProcessMessage    ← 主入口（wired by wire_coordinator.go::WireD7）
            ├── D7-S2-A02 ClassifyIntent (rule + LLM fallback)
            ├── switch intent.Kind:
            │     ├─ IntentSkip        → close channel
            │     ├─ IntentCommand     → CommandHandler.Handle
            │     └─ IntentFast / IntentOrchestrate / default
            │           → RunSessionTurnLoop → ItemPipelineRunner (D7-S6 MUPS Governance)
            │                 ├─ Observe / Plan (D7-S5)
            │                 ├─ Execute / Verify / Learn (D7-S6)
            │                 └─ WorkTree rollup (D7-S1)
            ├── D7-S2-A06 RunTurnLoop → D7-S2-A07 InvokeLLM → D3 (LLM Gateway)
            │                            → D2 (ContextPreparer / ToolRoundExecutor / SessionPersister)
            ├── D7-S2-A04 DispatchWorker → sessionorchestrator/dispatch.go → D4 Worker / D2 SubQuery
            └── flow.GlobalHub.Publish    ← D7-S4 读模型入口
                    ├── workplan.Service.Apply
                    ├── queue.SessionQueue (delegate-progress)
                    └── imsink.GatewaySink (worker_progress)

WaveScheduler (独立 explicit 路径 — D7-S3；由 delegate_tools / Plan / background job 触发，不经 ProcessMessage 主链路)
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

## 关键 Scenario 范式

> **1-2 个典型 Gherkin 示例**作为 canonical 范式。完整 174 个 Scenario 分布在 `archive/<change>/specs/` 各 change 目录。

### 范式 1：DAG 调度（核心调度语义）

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

### 范式 2：MUPS 交付收敛（Deliverable Gate + LLM 战略提案）

#### Scenario: Session complete prefers rollup deliverable

- GIVEN Goal rollup `ArtifactSummary` contains P0/P1 review with file:line citations
- AND the last processed child WorkItem summary is an exploration transition phrase
- WHEN `RunSessionTurnLoop` terminates
- THEN `complete.Content` SHALL be the rollup deliverable (via `ExtractSessionDeliverable`)
- AND NOT the child's transition text

#### Scenario: Partial without deliverable stays open

- GIVEN `VerdictPartial` AND `DeliverableStatus=incomplete` (e.g. `stop_reason=max_iters` without file:line)
- WHEN `ApplyPipelineRound` + `StatusAfterSpawnNone`
- THEN `TaskStatus` SHALL remain `InProgress` (not `Completed`)

### 范式 3：Pessimistic Commit L3 防御

#### Scenario: 资源耗尽触发 Pessimistic

- GIVEN a channel execution with `resource_exhausted` signal
- WHEN PessimisticCommitGuard.Evaluate is called
- THEN the channel returns `ApplyPessimisticCommit` with `MVPArtifact` containing `Output` + `RiskWarnings`
- AND `FallbackPolicy=RuleBased` is selected via `most_tests_passed` rule
- AND `ORCH_7110` sentinel error is logged with `ChainHash` for audit

---

## 关键链路口

1. **主入口链**：D1 RouteInbound → D7-S2 ProcessMessage → ClassifyIntent → Command / TurnLoop → ItemPipelineRunner
2. **5 节点管道链**：Observe/Plan (S5) → Execute/Verify/Learn (S6) → WorkTree rollup (S1)
3. **Wave 调度链（S3，独立）**：delegate_tools / Plan / background → WaveScheduler.Start → WorkerPool → ArtifactStore
4. **D7-D1 反馈链**：D7-S4 Hub.Publish → WorkPlan.Apply → SessionQueue → imsink.GatewaySink (飞书卡片)
5. **跨域消费**：D2 Prepare (V10/V11) → D3 LLM Gateway → D4 Delegate.Service → D5 Span Evidence
6. **Escape 链**：Observe/Plan/Execute/Verify 任一节点失败 → EscapeEngine.Evaluate → ChainedArbitrator (LLM/Rule/Human) → Action 6 类
7. **LP-1 闭环链**：Learn × 3 Pass → Alpha=3 → Round 2 Observe 用 Beta(8,3) 注入（PriorSessionAttrs 6 字段）
