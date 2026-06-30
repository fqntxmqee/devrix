# D7 Orchestration Domain Specification

**Capability:** d7-orchestration
**Domain:** D7
**DSAFT Type:** 核心域 (Core Domain)
**Version:** 4.19.0
**Status:** Canonical — source of truth
**Last Updated:** 2026-06-30 (devrix-d7-observe-unified-llm-path DM-20260630-001: SUPERSEDED A74 裸 D3; ADDED A75 Observe LLM via D2 Prepare → 本地化 Obs 附录 → D3)
**Domain SoT:** `d7-domain.md`
**Layering Spec:** `openspec/specs/architecture/layering.md`
**Parent Change:** devrix-d7-orchestration-domain (DM-20260613-001)

> **精简设计契约（Lite-Mode）**：本文档只放当前符合代码的设计契约。**过程需求迭代**（174 个完整 Gherkin Scenario 详细文本）**不进入本文件**，留在 `archive/<change-id>/specs/` 各 change 归档目录。详细时间线见 [CHANGELOG.md](CHANGELOG.md)。

---

## Recent Changes

| Date | Change ID | 摘要 | 归档 |
|------|-----------|------|------|
| 2026-06-30 | devrix-d7-observe-unified-llm-path | S16-A75 Observe LLM D2→D3 (4 Req/4 T) | [archive](../../archive/2026-06-30-devrix-d7-observe-unified-llm-path/) |
| 2026-06-29 | devrix-d7-taskcontract-unification-pr-c | S18 CoW VersionChain + Similarity + Hard Evidence | [archive](../../archive/2026-06-29-devrix-d7-taskcontract-unification-pr-c/) |
| 2026-06-29 | devrix-d7-taskcontract-unification-pr-b | S18-A11/A12 Pessimistic + Rule-based (L3) | [archive](../../archive/2026-06-29-devrix-d7-taskcontract-unification-pr-b/) |
| 2026-06-29 | devrix-d7-taskcontract-unification-pr-a | S20/S21 TaskSpec + TaskReport 契约 | [archive](../../archive/2026-06-29-devrix-d7-taskcontract-unification-pr-a/) |

> 完整时间线见 [CHANGELOG.md](CHANGELOG.md)。

---

## Overview

D7 编排域回答 **"做什么、按什么顺序做、谁来做、做得怎么样了"**。作为 **横向协调层** 编排 D2（LLM↔Tool 执行原语）与 D4（Agent 委托原语），并向 D1 发布进度事件（D1 仍拥有 ingress）。

**现行实现路径（v4.3 post-cleanup）**：v2.0 Structure（DM-20260619-005）+ devrix-d7-dead-files-cleanup 合流后物理路径与 S 层 1:1 对齐：S2 `sessionorchestrator/`、S3 `wavescheduler/`、S4 `executionflow/{hub,workplan,imsink,bridge}/`、S5 `plan/` + `decisionplanning/`。D1 主入口 `sessionorchestrator.Entry.ProcessMessage`（`bootstrap/wire_coordinator.go::WireD7`）。Intent 四链正交分发不变（CommandHandler / FastPath / OrchestratePath / Skip）。详见 `pipeline-architecture.md` §6.3 + §7。

### 核心设计原则

1. **不可变 + 纯函数优先**：值对象（TaskSpec/TaskReport/Verdict）通过 `With*` 返回新副本；Engine 节点间通过 Channel 单向通信
2. **5 节点管道**：Observe → Plan → Execute → Verify → Learn 顺序执行 + LP-1/2/5 闭环（Learn → Observe / Learn → Skill / Plan → Verdict 反查）
3. **意图四链正交分发**：CommandHandler / FastPath / OrchestratePath / Skip 各走各的执行链，不复用
4. **Auto-Close 异步触发**：Verify 完成后异步触发 Learn，不阻塞 channel close（3 层 fail-safe：nil learner / Learn error / channel cancel）
5. **Pessimistic Commit L3 防御**：5 类触发（resource_exhausted / cb_l1 / indeterminate_3x / empty_evidence / manual_abort）+ Rule-based Fallback 4 候选规则
6. **CoW VersionChain 不变性**：所有 Task/Plan/Artifact 走 Copy-on-Write 版本链，避免 in-place mutation
7. **Trace 树全程贯穿**：每跳节点产生 child span；6 prior attributes（α/β/mean/track_mode/classifier_source/injected_at）由 sessionSpan 统一注入

### S 层博弈角色定义（切法 A — 按用户价值流）

| S 层 | 博弈角色 | North Star |
|------|---------|------------|
| D7-S2 | **Screening Mechanism** + **Turn Leader (Stackelberg)** | 用户消息统一入口 + Turn 主循环 |
| D7-S3 | **Mechanism Designer** | 多任务并行执行 + 冲突避免 |
| D7-S4 | **Costly Signaler** | 执行进度透明 + WorkPlan 可追溯 |
| D7-S5 | **Information Producer** | 把用户 goal 转化为可执行的任务结构 |
| D7-S1 | **State Authority**（非博弈角色） | **WorkItem** 持久化与状态机 |

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
| A | A1-A99 | 22 个核心活动 | 见 `a-registry.md` |
| F | F1-F999 | 功能点编排 | 见 `f-registry.md` |
| T | T1-T200 | 测试点（T01-T180 IMPLEMENTED，T181-T200 PLANNED） | 见 `t-registry.md` |

**当前计数（v4.19.0）**：D=1, S=6, A=22, F=120, T=180（IMPLEMENTED）。Canonical 5 节点 = Observe-A15 / Plan-A22 / Execute-A25/A26 / Verify-A32 / Learn-A36-A40。

---

## Scenarios

| ID | Scenario | Responsibility | Status | 代码位置 |
|----|----------|----------------|--------|----------|
| D7-S1 | Work Model | **WorkItem** CRUD、依赖 DAG、磁盘持久化（schema v2）、PlanMode 状态机 | **IMPLEMENTED** | `workmodel/work_tree.go` + `workitem.go` + `sessionorchestrator/workmodel.go` |
| D7-S2 | Session Orchestrator | ProcessMessage、FastPath、HandleInterrupt、TurnLoop、InvokeLLM、Dispatch | **IMPLEMENTED** | `sessionorchestrator/` |
| D7-S3 | Wave Scheduler | TaskGraph DAG、5-slot 池、ContextPolicy、ConflictGuard | **IMPLEMENTED** | `wavescheduler/` |
| D7-S4 | Execution Flow | Hub 双通道发布、WorkPlan 读模型、IM worker_progress、SpokeBridge | **IMPLEMENTED** | `executionflow/` |
| D7-S5 | Decision & Planning | PlanAgent 只读探索（/plan CLI）、规则+LLM 分类、PlanKind 4 类 + DefaultPlanner | **IMPLEMENTED** | `plan/` + `decisionplanning/` |
| D7-S6 | Error Aggregation & Metrics | errors.Join 聚合 + InterruptMetrics + 4 新 WaveScheduler metrics 字段 | **IMPLEMENTED** | `sessionorchestrator/{interrupt,metrics}.go` + `wavescheduler/scheduler.go` |
| D7-S8 | Observation + UncertaintyReport | Observation 4 类 + UncertaintyReport Partition + UncertaintyCoord | **IMPLEMENTED** | `orchtypes/{observation,uncertainty_report,uncertainty_coord}.go` |
| D7-S9 | Execute Artifact | ArtifactKind 4 类 + SideEffectStatus 5 态 | **IMPLEMENTED** | `shared/types/execute.go` + `wavescheduler/types.go` |
| D7-S11 | Learn Node | LearningAsset 5 类 + ReputationEvidence + Bayesian + 3 通道 Memory + Learner | **IMPLEMENTED** | `learn/` + `shared/types/learning.go` |
| D7-S12 | Observe-Learner 闭环 | ObserveRequest + IntentQuantizer + AnomalyDetector + buildObserveRequest 3 层 fail-safe | **IMPLEMENTED** | `orchtypes/{observe_request,intent_quantizer,anomaly_detector}.go` |
| D7-S13 | 5 节点运行时闭环 | processAutoClose + synthesizeVerdict 4 规则 + TrackMode + 6 prior attributes | **IMPLEMENTED** | `sessionorchestrator/{autoclose,tracing,orchestrator}.go` |
| D7-S14 | MUPS v5 逃逸 | LoopDepthTracker v2 + EscapeEngine 5 节点 + CircuitBreaker 5 层 + ResumeSession | **IMPLEMENTED** | `escape/` |
| D7-S15 | WorkItem Rollup | Parent Rollup Gate + Root Fallback + Summary/Structured dual bubble | **IMPLEMENTED (Phase 1)** | `workmodel/rollup_gate.go` + `sessionorchestrator/rollup_*` |
| D7-S16 | Layer SubContext | Per-Layer SubContext + ChildDownlink + Observe LLM D2→D3 (A75) | **IMPLEMENTED (Phase 1+2+3 + A75)** | `wavescheduler/context*.go` + `sessionorchestrator/llm_observation_proposer.go` |
| D7-S18 | Pessimistic + Fallback | PessimisticCommitGuard 5 类触发 + 4 候选规则 + CoW VersionChain | **IMPLEMENTED (PR-A/B/C)** | `interfaces/contracts.go` + `escape/fallback.go` + `mups/execute/channel.go` |
| D7-S20 | TaskSpec 下行 | TaskSpec 5 字段 + NewTaskSpec fail-fast + 3 处创建点统一 | **IMPLEMENTED (PR-A 9/11 P0 T)** | `interfaces/task_spec.go` |
| D7-S21 | TaskReport 上行 | TaskReport 7 字段 + Dissent/Blockage/Resource 语义层 + Learn 沉淀 | **IMPLEMENTED (PR-A 9/11 P0 T)** | `interfaces/task_report.go` + `mups/learn/asset/asset_builder.go` |

---

## Architecture

> **5 节点管道端到端链路 + LP-1/2/5 闭环 + Auto-Close 异步触发的完整运行时序**，见 `pipeline-architecture.md`（MUPS v4.3 Phase 1-7 全部 S7_Archived 后的端到端总图）。

```
D1 Gateway.RouteInbound
    └── D7-S2 SessionOrchestrator.ProcessMessage    ← v1.0 主入口（wired by wire_coordinator.go::WireD7）
            ├── D7-S2-A02 ClassifyIntent (rule + LLM fallback)
            ├── switch intent.Kind (v1.1.0+ orthogonal dispatch):
            │     ├─ IntentSkip        → close channel
            │     ├─ IntentCommand     → CommandHandler.Handle
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

### 范式 2：Pessimistic Commit L3 防御

#### Scenario: 资源耗尽触发 Pessimistic

- GIVEN a channel execution with `resource_exhausted` signal
- WHEN PessimisticCommitGuard.Evaluate is called
- THEN the channel returns `ApplyPessimisticCommit` with `MVPArtifact` containing `Output` + `RiskWarnings`
- AND `FallbackPolicy=RuleBased` is selected via `most_tests_passed` rule
- AND `ORCH_7110` sentinel error is logged with `ChainHash` for audit

---

## 关键链路口

1. **主入口链**：D1 RouteInbound → D7-S2 ProcessMessage → ClassifyIntent → switch intent → 4 链分发
2. **5 节点管道链**：Observe (S8/S12) → Plan (S5/S22) → Execute (S9/S25/S26) → Verify (S10/A32-A35) → Learn (S11/S36-S40)
3. **D7-D1 反馈链**：D7-S4 Hub.Publish → WorkPlan.Apply → SessionQueue → imsink.GatewaySink (飞书卡片)
4. **跨域消费**：D2 Prepare (V10/V11) → D3 LLM Gateway → D4 Delegate.Service → D5 Span Evidence
5. **Escape 链**：Observe/Plan/Execute/Verify 任一节点失败 → EscapeEngine.Evaluate → ChainedArbitrator (LLM/Rule/Human) → Action 6 类
6. **LP-1 闭环链**：Learn × 3 Pass → Alpha=3 → Round 2 Observe 用 Beta(8,3) 注入（PriorSessionAttrs 6 字段）

---

## 附录：总览

- **当前活跃 Requirement 数**：0（已合入代码，需求态转为代码态）
- **当前活跃 Scenario 数**：0 完整文本（仅 2 个范式示例）
- **历史 Scenario 详细文本**：174 个，分布在 `archive/<change>/specs/` 各 change 目录（详见 CHANGELOG.md）
- **当前 spec 版本**：v4.19.0
- **下一次架构级变更触发**：MUPS v5 5 节点 EscapeEngine 完整接线（PR-V5.7+，T181-T200）
