# D7 Orchestration Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 5.0.0
**Last Updated:** 2026-06-26
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `d7-domain.md`

> **v6.0.0 6 S 精简（DM-20260626-001，2026-06-26 落地）：** Canonical S 层 14 → 6 + 1 横切（v6.0.0 博弈角色对齐精简）。MUPS 5 节点管道（Observe/Plan/Execute/Verify/Learn）+ v5 EscapeEngine 完整保留，**A 总数 56 → 49**（S1:4 · S2:7 · S3:4 · S4:9 · S5:8 · S6:15 + Hardening:2）。具体 A 重映射见 §v6.0.0 6 S 精简映射。

---

## Overview

D7 编排域 A 层活动注册表。

> **终态流程 / IntentKind 四链：** 见 `terminal-state-guide.md` §3–§7；**Span↔T Runbook：** 见 `observability-guide.md`。

代码分布：

- `internal/layers/orchestration/sessionorchestrator/`（D7-S2）
- `internal/layers/orchestration/decisionplanning/`（D7-S5）
- `internal/layers/orchestration/wavescheduler/` + `executionflow/`（D7-S3 + D7-S4）
- `internal/layers/orchestration/workmodel/`（D7-S1）
- `internal/layers/orchestration/sessionorchestrator/`（D7-S2-A06/A07 Turn Leader）
- `internal/layers/orchestration/sessionorchestrator/`、`hubspoke/`（1-release legacy shim）

**状态图例：** ✅ IMPLEMENTED · 🔶 PARTIAL · ⬜ PLANNED

---

## Legacy 双轨方案（v1.0+）

> 根据 `devrix-d7-sa-refine` (DM-20260614-008) §7 设计决策：
> - **Legacy** — 旧编号冻结追溯，路径：`internal/layers/orchestration/sessionorchestrator/`（旧包结构）
> - **Canonical** — 新编号按用户价值流，路径：`internal/layers/orchestration/`（新包结构）

### 追溯规则

```
Legacy ID（如 D7-S2-A01-LEGACY）→ 新 Canonical（D7-S2-A01）
Legacy T（如 D7-S2-T01-LEGACY）→ 新 T 映射
```

### 禁止约束

- **禁止** 在 Legacy 语义上新增 T
- **禁止** 在 Legacy 路径下开发新功能
- **强制** 新功能走 Canonical S

---

## D7-S1: Work Model ✅ IMPLEMENTED（v4.3 post-cleanup）

> **WorkItem** 写模型位于 `internal/layers/orchestration/workmodel/{work_tree,workitem,workitem_store,task_manager}.go` + `sessionorchestrator/workmodel.go`。v4.3 post-cleanup（PR #214）已删 `workmodel/task_store.go`（Task flat-view）+ `workitem.go` 内 conversion helpers + `taskStoreAdapter`，**WorkItem 是唯一 canonical 模型**, TaskManager 只是 `Tree()` facade。PlanMode 在 `workmodel/{plan_mode,plan_agent}.go`, PlanAgent 仅服务于 `/plan` CLI 命令入口。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S1-A01 | CreateWorkPlan | A-BE | session_id, goal, context | Plan (**WorkItem** 列表 + DAG) | work_plan.created | ✅ | `sessionorchestrator/workmodel.go` + `workmodel/plan_mode.go` |
| D7-S1-A02 | ManageWorkItem | A-BE | item_id, action (create/update/delete/dep) | workitem_state | workitem.* | ✅ | `workmodel/{work_tree,task_manager}.go` + `workitem_store.go`（DiskWorkItemStore schema v2） |
| D7-S1-A03 | QueryWorkPlan | A-BE | session_id | WorkPlanSnapshot | — | ✅ | `orchestration/executionflow/hub/hub.go` Snapshot |

### D7-S1 附加活动（PlanMode）

| A ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A04 | EnterPlanMode | A-BE | session_id, goal | plan_state | ✅ | `workmodel/plan_mode.go` |
| D7-S1-A05 | ApprovePlan | A-BE | session_id | []WorkItem | ✅ | `workmodel/plan_mode.go` |
| D7-S1-A06 | ExecutePlanAgent | A-BE | goal, context | PlanResult | ✅ | `workmodel/plan_agent.go` |

---

## D7-S2: Session Orchestrator ✅ LEGACY

> D1 经 `gateway.d7Entry` 路由至 `sessionorchestrator.Entry.ProcessMessage`（`coordinator.Entry` shim），由 SessionOrchestrator 编排 D2/D4/D7。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S2-A01 | ProcessMessage | A-BE | message, session | events stream | session.orchestrating | ✅ | `orchestration/sessionorchestrator/orchestrator.go`（**loop_first** default: Skip/Command/Turn；**rule_orchestrate**: 4 独立链 CommandHandler/FastPath/OrchestratePath）|
| D7-S2-A02 | EvaluateIntent | A-BE | message, context | IntentClassification | — | ✅ | `orchestration/decisionplanning/classifier.go` + `classifier_fallback.go` |
| D7-S2-A03 | HandleInterrupt | A-BE | session_id, reason | — | session.interrupted | ✅ | `orchestration/sessionorchestrator/interrupt.go` |
| **D7-S2-A04** | **CommandHandler** | **A-BE** | **message** | **EngineEvent stream** | **—** | **✅** | **`orchestration/sessionorchestrator/command_handler.go` — DM-20260615-004 v1.1; emit 加 select-default + `slog.Warn` 防 consumer stall — DM-20260622-001 A5** |

---

## D7-S3: Wave Scheduler ✅ LEGACY

> 完整实现于 `internal/layers/orchestration/wavescheduler/`。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S3-A01 | ScheduleWave | A-BE | session_id, task_graph | artifact_list | wave.* | ✅ | `orchestration/wavescheduler/scheduler.go` |
| D7-S3-A02 | ResolveWorkerContext | A-BE | task_node, session | resolved_context | — | ✅ | `orchestration/wavescheduler/context.go` |
| D7-S3-A03 | GuardConflict | A-BE | candidate, running_tasks | allowed/blocked | — | ✅ | `orchestration/wavescheduler/conflict.go` |
| **D7-S3-A04** | **HardenScheduler** | **A-BE** | **—** | **—** | **—** | **✅** | **`wavescheduler/scheduler.go::markWaveDone` (release `state.cancels`/`state.handles` on terminal) + `dispatchOne` (atomic `AllowAndRegister`) — DM-20260622-001 A3+A4** |

---

## D7-S4: Execution Flow ✅ LEGACY

> 实现于 `orchestration/executionflow/{hub,workplan,imsink,bridge}/`。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S4-A01 | PublishFlowEvent | A-BE | flow_event | — | flow.event_published | ✅ | `orchestration/executionflow/hub/hub.go` Publish |
| D7-S4-A02 | SnapshotWorkPlan | A-BE | session_id | WorkPlanSnapshot | — | ✅ | `orchestration/executionflow/hub/hub.go` Snapshot |
| D7-S4-A03 | NotifyGateway | A-BE | event, session | — | — | ✅ | `orchestration/executionflow/imsink/gateway.go` |

---

## D7-S5: Decision & Planning ✅ LEGACY

> PlanMode/PlanAgent、ClassifyIntent（规则+LLM fallback）、SynthesizeTaskGraph、SelectExecutor 均已实现（layer-delta Phase H/K/L/M）。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S5-A01 | ClassifyIntent | A-BE | message, session, context | IntentClassification | — | ✅ | `orchestration/decisionplanning/classifier.go` + `classifier_fallback.go` (LLM merge) |
| D7-S5-A02 | SynthesizeTaskGraph | A-BE | goal, constraints, explore_events | []TaskNode | plan.formulated | ✅ | `decisionplanning/decomposer.go` (rule + LLM via SetLLMDecomposer) |
| D7-S5-A03 | SelectExecutor | A-BE | TaskNode, agent_pool | executor_id (D2/D4) | — | ✅ | `decisionplanning/executor.go` |
| D7-S5-A04 | RunPlanAgent | A-BE | goal, readonly context | PlanResult | plan.generated | ✅ | `workmodel/plan_agent.go` |
| D7-S5-A05 | TailShadowClassify | A-BE | message, rule_result | — | metric only | ✅ | `orchestration/decisionplanning/shadow_classifier.go` |

---

## Canonical S 层定义（切法 A — 按用户价值流）

> 以下为 `devrix-d7-sa-refine` v1.0 Canonical 定义。流程见 `terminal-state-guide.md`。

### D7-S1: Work Model ✅ Canonical（v4.3 post-cleanup）

> North Star: **WorkItem 事实与状态机**单一权威（v4.3 起, Task flat-view + TaskStore 全删）
> 博弈角色: **State Authority**

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| D7-S1-A01 | CreateWorkPlan | D7-S1-A01-LEGACY | A-BE | session_id, goal, context | Plan (**WorkItem**) | workitem.created | ✅ | `sessionorchestrator/workmodel.go` + `workmodel/plan_mode.go` |
| D7-S1-A02 | ManageWorkItem | D7-S1-A02-LEGACY | A-BE | item_id, action | workitem_state | workitem.* | ✅ | `workmodel/{work_tree,task_manager}.go` + `workitem_store.go` |
| D7-S1-A03 | QueryWorkPlan | D7-S1-A03-LEGACY | A-BE | session_id | WorkPlanSnapshot | — | ✅ | `orchestration/executionflow/hub/hub.go` Snapshot |
| D7-S1-A04 | EnterPlanMode | — | A-BE | session_id, goal | plan_state | plan.entered | ✅ | `workmodel/plan_mode.go` |
| D7-S1-A05 | ApprovePlan | — | A-BE | session_id | []Task | plan.approved | ✅ | `workmodel/plan_mode.go` |
| D7-S1-A06 | ExecutePlanAgent | D7-S5-A04-LEGACY | A-BE | goal, context | PlanResult | plan.generated | ✅ | `workmodel/plan_agent.go` |

### D7-S2: 会话编排入口 + Turn Leader ✅ Canonical

> North Star: 用户消息统一入口，决定走快速路径还是编排路径；**拥有 LLM 调用权与 Turn 主循环（DM-020）**
> 博弈角色: Screening Mechanism（筛路径）+ **Turn Leader（Stackelberg 先手）**

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| D7-S2-A01 | ProcessMessage | D7-S2-A01-LEGACY | A-BE | message, session | events stream | session.orchestrating | ✅ | `orchestration/sessionorchestrator/orchestrator.go`（loop_first: Turn default；rule_orchestrate: 4 case 正交链）|
| D7-S2-A02 | EvaluateIntent | — | A-BE | message, context | IntentClassification | — | ✅ | `orchestration/decisionplanning/classifier.go` + `classifier_fallback.go` |
| D7-S2-A03 | HandleInterrupt | D7-S2-A03-LEGACY | A-BE | session_id, reason | — | session.interrupted | ✅ | `orchestration/sessionorchestrator/interrupt.go` |
| D7-S2-A04 | DispatchWorker | D4-S10-A01（编排面） | A-BE | leader, worker_spec | spoke_id, executor | task.{delegated,completed,failed} | ✅ | `sessionorchestrator/dispatch.go`（v1.0 路径：`bootstrap/delegate.go` 已 wired） |
| **D7-S2-A06** | **RunTurnLoop** | — | **A-BE** | **session, TurnRequest** | **<-chan EngineEvent** | **turn.{started,completed,failed}** | **✅** | **`orchestration/sessionorchestrator/turn_orchestrator.go`**（DM-020 v1.0-c；wired by `bootstrap/wire_coordinator.go:60`） |
| **D7-S2-A07** | **InvokeLLM** | — | **A-BE** | **LLMInvokeRequest** | **<-chan Chunk** | **llm.{invoked,streaming,completed}** | **✅** | **`orchestration/sessionorchestrator/llm.go`**（DM-020 v1.0-b；wired by `bootstrap/wire_coordinator.go:59`）。**兼作 D2→D3 拆面出口**：`turn.QueryLLMCaller` + `turn.CompressionSummarizer` 由同一 `llmgateway.IGateway` 驱动，单一注入点 `bootstrap/context_engine.go` wired 至 `EngineDeps.QueryLLMCaller` / `EngineDeps.Summarizer`（DM-020 v2.3 拆面闭合） |

> **D7-S2-A04**（DM-20260614-018）：Hub-Spoke 派发矩阵 + fallback 路由。v1.0 逻辑在 D4 `delegate/service.go`；v2.0 迁 `sessionorchestrator/dispatch.go`。

### D7-S3: Wave 调度 ✅ Canonical

> North Star: 多任务并行执行，冲突避免，上下文隔离
> 博弈角色: Mechanism Designer（定执行规则）

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| D7-S3-A01 | ScheduleWave | D7-S3-A01-LEGACY | A-BE | session_id, task_graph | artifact_list | wave.* | ✅ | `orchestration/wavescheduler/scheduler.go` |
| D7-S3-A02 | ResolveWorkerContext | D7-S3-A02-LEGACY | A-BE | task_node, session | resolved_context | — | ✅ | `orchestration/wavescheduler/context.go` |
| D7-S3-A03 | GuardConflict | D7-S3-A03-LEGACY | A-BE | candidate, running_tasks | allowed/blocked | — | ✅ | `orchestration/wavescheduler/conflict.go` |
| **D7-S3-A04** | **HardenScheduler** | **—** | **A-BE** | **—** | **—** | **—** | **✅** | **`wavescheduler/scheduler.go::markWaveDone` (release state.cancels/handles) + `dispatchOne` (AllowAndRegister atomic) — DM-20260622-001 A3+A4** |

### D7-S4: 执行流 ✅ Canonical

> North Star: 执行进度透明，WorkPlan 可追溯
> 博弈角色: Costly Signaler（向用户广播成本）

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| D7-S4-A01 | PublishFlowEvent | D7-S4-A01-LEGACY | A-BE | flow_event | — | flow.event_published | ✅ | `orchestration/executionflow/hub/hub.go` Publish |
| D7-S4-A02 | SnapshotWorkPlan | D7-S4-A02-LEGACY | A-BE | session_id | WorkPlanSnapshot | — | ✅ | `orchestration/executionflow/hub/hub.go` Snapshot |
| D7-S4-A03 | NotifyGateway | D7-S4-A03-LEGACY | A-BE | event, session | — | — | ✅ | `orchestration/executionflow/imsink/gateway.go` |
| D7-S4-A04 | BridgeAgentSpoke | D4-S10-A02 | A-BE | agent_event, engine_event | — | flow.published | ✅ | `executionflow/bridge/agent_bridge.go`（wired by `bootstrap/delegate.go:52`） |
| D7-S4-A05 | BridgeSubQuerySpoke | D2-S19（Flow 面） | A-BE | subquery_result | flow_event | flow.published | ✅ | `executionflow/bridge/subquery_bridge.go`（wired by `bootstrap/delegate.go`） |

> **D7-S4-A04/A05**（DM-20260614-018）：统一三 Spoke 写侧（D4 Delegate / D2 SubQuery / D7 Wave）→ `ExecutionFlowHub`。

### D7-S5: 决策规划 ✅ Canonical

> North Star: 把用户 goal 转化为可执行的任务结构（结构路径，非内容质量）
> 博弈角色: Information Producer（产私有信息）
> **Explore 输入:** SynthesizeTaskGraph 吸收 Explore Workers（并行 read-only）通过 D7-S4 广播的 FlowEvent

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| D7-S5-A01 | ClassifyIntent | D7-S5-A01-LEGACY | A-BE | message, session, context | IntentClassification | — | ✅ | `orchestration/decisionplanning/classifier.go` + `classifier_fallback.go` |
| D7-S5-A02 | SynthesizeTaskGraph | D7-S5-A02-LEGACY | A-BE | goal, constraints, explore_events | []TaskNode | plan.formulated | ✅ | `decisionplanning/decomposer.go`（rule + LLM via SetLLMDecomposer） |
| D7-S5-A03 | SelectExecutor | — | A-BE | TaskNode, agent_pool | executor_id (D2/D4) | — | ✅ | `decisionplanning/executor.go` |

### D7-S6: Observability & Hardening ✅ Canonical

> North Star: D7 编排层 metric 命名 spec/code 对齐 + 并发硬化，承载 PR-B（DM-20260621-010）落地的 5 个 counter 的 spec 收敛与并发缺陷修复
> 博弈角色: Discipline Keeper（守住 spec 纪律与并发安全）
> **范围：** 横切 S2/S3，不属于 5 个核心 S 的纵向业务流

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| **D7-S6-A14** | **HardenMetricsAndConcurrency** | **—** | **A-BE** | **—** | **—** | **—** | **✅** | **5 fix 一揽子（DM-20260622-001）：(A1) `dispatch_loop_wakeups`/`worker_panics` 复数化与 switch case 双修；(A2) `dispatch_loop_wakeups`/`worker_panics` 命名口径 spec/code 一致；(A3) `markWaveDone` 释放 `state.cancels`/`state.handles` 至 nil/空 map 防跨 wave 累积；(A4) `dispatchOne` 改原子 `AllowAndRegister` 关 TOCTOU 窗口；(A5) `CommandHandler.emit` 改 `select-default` + `slog.Warn` 防 consumer 阻塞；(A6) `sandbox_exit_failed` 跨域归属澄清（D4 实际拥有，D7 spec 标 OBSOLETE）** |

---

### D7-S7: MUPS 5 节点管道入口（横向编排） ✅ Canonical（v4.3 起）

> North Star: 把 **Observe → Plan → Execute → Verify → Learn** 5 节点管道升格为 Canonical 顶层场景，对应 Phase 2-5 OpenSpec（DM-20260623-001/002/003）+ Phase v5 EscapeEngine（DM-20260625-003）
> 博弈角色: Pipeline Coordinator（5 节点 owner + 闭环）
> **与 S1-S6 关系：** S1 是 State Authority，S2-S5 是入口/调度/信号/决策；S7-S14 是 5 节点管道的**纵向自治单元**，彼此通过 DependencyContract 串联（Observe→Plan→Execute→Verify→Learn）。

#### D7-S7 节点间依赖契约

```text
Observe(S8) ── UncertaintyReport ──▶ Plan(S8-PR-B1) ── Plan ──▶ Execute(S9) ── Artifact ──▶ Verify(S10) ── Verdict ──▶ Learn(S11)
   ▲                                                                                                              │
   └──────────────── ReputationEvidence (Bayesian) ←───────────────────────────────────────────────────────────────┘
```

| 节点 | 入口契约 | 出口契约 | 节点间约束 |
|------|---------|---------|----------|
| Observe | SessionID + UserMessage + (可选) AdaptivePrior | UncertaintyReport | 4 类 Observation 必须落 UncertaintyCoord |
| Plan | UncertaintyReport | Plan{ID, Kind, Strength, Steps, FailureCriteria, BlastRadius, SourceObservationIDs} | Plan.SourceObservationIDs 必须可反向追溯 Observation |
| Execute | Plan | Artifact{ID, Kind, Payload, Evidence, SourcePlanID} | Artifact.SourcePlanID 必须可反向追溯 Plan |
| Verify | Artifact + Plan | Verdict{Kind, Evidence, Reason, SourceArtifactID} | Verdict.SourceArtifactID 必须可反向追溯 Artifact |
| Learn | Verdict + Plan + Observation (追溯链) | LearningAsset + ReputationEvidence | ReputationEvidence 必须能注入下一轮 Observe 作先验 |

---

### D7-S8: Observe 节点 ✅ Canonical（Phase 2 PR-A1 + PR-RF, DM-20260623-001）

> North Star: 把"用户消息 + 历史 + 上下文"结构化为 **4 类 Observation**，量化后产出 **UncertaintyReport**（含 UncertaintyCoord）。
> 博弈角色: Information Quantizer（产结构化观察 + 不确定性度量）
> **核心实体：** `Observation{Kind, Strength, Scope, Stakes, SourceIDs}` + `UncertaintyReport{Observations, Overall: UncertaintyCoord, Anomalies: []Anomaly, QuantizedIntent: IntentPayload}`

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S8-A15** | **ObserveQuantize** | **A-BE** | **session_id, message, history, prior?** | **UncertaintyReport** | **observation.recorded** | **✅** | **`orchestration/observe/observe_node.go::Observe`**（Phase 2 PR-A1 S7_Archived + PR-RF 5 review fix）|
| D7-S8-A15-F | AnomalyDetector | F-BE | observations | []Anomaly | — | ✅ | `orchestration/observe/anomaly.go` (Phase 2 PR-A1) |
| D7-S8-A15-F | IntentQuantizer | F-BE | observations + anomaly | IntentPayload | — | ✅ | `orchestration/observe/intent_quantizer.go` (Phase 2 PR-A1 + WithPrior 变体 Phase 6) |

**4 类 Observation 子类型（按 ★ 等级）：**

| ObsKind | 含义 | 强度范围 | Strength 决定 | 备注 |
|---------|------|---------|-------------|------|
| ObsFact | 已确认事实 | ★★-★★★★ | 由 evidence 数量定 | 不可降级 |
| ObsAnomaly | 异常信号 | ★-★★★ | 由 z-score/pattern 定 | 触发 Plan 升格 |
| ObsSignal | 用户/系统信号 | ★-★★★★ | 由来源权威定 | 命令/状态 |
| ObsUser | 用户意图 | ★-★★★ | 由置信度定 | 走 IntentQuantizer |

---

### D7-S9: Execute 节点 ✅ Canonical（Phase 3 PR-C1 + PR-C2, DM-20260625-001）

> North Star: 按 Plan 调度执行，产出 4 类 Artifact；4 通道（同步 / 异步 / 试探 / 探索）走 C2/W8 1:1 映射。
> 博弈角色: Mechanism Designer（执行规则 + 副作用边界）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S9-A25** | **ExecuteArtifact** | **A-BE** | **Plan, session_id** | **Artifact** | **artifact.produced** | **✅** | **`orchestration/execute/executor.go::Execute`**（Phase 3 PR-C1 S7_Archived，跨域类型上提 `shared/types.Artifact`）|
| **D7-S9-A26** | **RouteChannel** | **A-BE** | **Plan.Step, Artifact?** | **ChannelKind (sync/async/probe/explore)** | **—** | **✅** | **`orchestration/execute/channel_router.go::RouteChannel`**（Phase 3 PR-C2 S7_Archived，C2/W8 1:1 映射）|

**4 类 Artifact 子类型：**

| ArtifactKind | 含义 | 配套 Channel | Evidence 字段 |
|--------------|------|-------------|--------------|
| StateChangeCert | 状态变更凭证 | sync | BeforeHash, AfterHash, Actor |
| ResponseRecord | 响应记录 | async | StatusCode, Body, LatencyMs |
| ProbeReport | 试探报告 | probe | Hypothesis, Result, Confidence |
| ExperimentData | 探索数据 | explore | Samples, Statistics, AnomalyScore |

**4 Channel 实现（C2/W8）：**

| Channel | ChannelKind | 失败处理 | 重试 |
|---------|-------------|---------|------|
| SyncChannel | sync | fast-fail | 0 |
| AsyncChannel | async | queue+retry | 3 |
| ProbeChannel | probe | backoff+fallback | 2 |
| ExploreChannel | explore | best-effort | 1 |

---

### D7-S10: Verify 节点 ✅ Canonical（Phase 4, DM-20260623-002）

> North Star: 验证 Artifact 是否满足 Plan.FailureCriteria + 反向追溯 Observation；产出 4 态 Verdict + 14 ExitReason。
> 博弈角色: Certifier（颁发可信判决）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S10-A32** | **VerifyVerdict** | **A-BE** | **Artifact, Plan** | **Verdict** | **verdict.recorded** | **✅** | **`orchestration/verify/verifier.go::Verify`**（Phase 4 S7_Archived）|
| **D7-S10-A33** | **VerdictToExitReason** | **A-BE** | **Verdict, Plan.ObservationStrength** | **ExitReason (14 态)** | **—** | **✅** | **`orchestration/verify/exit_reason.go::MapToExitReason`**（Phase 4 + 14 ExitReason）|
| **D7-S10-A34** | **ExtractEvidence** | **F-BE** | **Artifact, Verdict** | **Evidence** | **—** | **✅** | **`orchestration/verify/evidence.go::ExtractEvidence`** |
| **D7-S10-A35** | **DetectSystemAnomaly** | **F-BE** | **session history** | **SystemAnomaly?** | **—** | **✅** | **`orchestration/verify/system_anomaly.go::Detect`** |

**4 态 VerdictKind：**

| VerdictKind | 触发条件 | 对应 ExitReason 家族 |
|-------------|---------|---------------------|
| ComplianceVerdict | Plan.FailureCriteria 全满足 | natural / succeeded |
| TimelinessVerdict | 时间窗口满足 | resolved_in_window |
| RootCauseVerdict | 反向追溯到 Observation | natural_with_evidence |
| StatisticalVerdict | 概率阈值满足 | statistically_significant |

**14 ExitReason（8 deterministic + 6 verify-driven）：** 详见 `terminal-state-guide.md` §11。

---

### D7-S11: Learn 节点 ✅ Canonical（Phase 5, DM-20260623-003）

> North Star: 把 Verdict + 追溯链沉淀为 **LearningAsset** + **ReputationEvidence**（Bayesian 更新）；下轮 Observe 注入 AdaptivePrior。
> 博弈角色: Memory Curator（记忆资产 + 信誉先验）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S11-A36** | **BuildLearningAsset** | **A-BE** | **Verdict, Plan, Observation (追溯链)** | **LearningAsset** | **asset.stored** | **✅** | **`orchestration/learn/asset_builder.go::Build`**（Phase 5 S7_Archived，4 类 LearningClass）|
| **D7-S11-A37** | **UpdateReputationEvidence** | **A-BE** | **SessionID, Verdict.Kind, Evidence** | **ReputationEvidence** | **reputation.updated** | **✅** | **`orchestration/learn/reputation_store.go::Update`**（Bayesian Beta 更新）|
| **D7-S11-A38** | **BuildAdaptivePrior** | **F-BE** | **ReputationEvidence (历史)** | **AdaptivePrior** | **—** | **✅** | **`orchestration/learn/adaptive_prior.go::Build`**（DefaultDeveloperPrior Beta(5,3) / DefaultOperatorPrior Beta(8,1)）|
| **D7-S11-A39** | **MemoryPersist** | **F-BE** | **LearningAsset** | **—** | **memory.written** | **✅** | **`orchestration/learn/memory.go::Persist`**（3 通道：skill / feedback / scheduled）|
| **D7-S11-A40** | **RunLearner** | **A-BE** | **Verdict + 追溯链** | **[]LearningAsset + ReputationEvidence** | **learner.completed** | **✅** | **`orchestration/learn/learner.go::Learn`** |

**5 类 LearningClass（按 ★）：**

| LearningClass | Kind 字段 | TTL | 注入位置 |
|---------------|-----------|-----|---------|
| SOPAsset | SOP | 90d | skill 通道 → Observe.SkillPrior |
| ProtocolAsset | Protocol | 180d | skill 通道 → Plan.ProtocolHint |
| KnowledgeAsset | Knowledge | 365d | feedback 通道 → Plan.Context |
| ConclusionAsset | Conclusion | 30d | scheduled 通道 → Plan.ConclusionRef |
| ReputationEvidence | (非 asset，meta) | session-bound | 自适应先验 → Observe.Prior |

---

### D7-S12: Observe-Learner 跨域闭环集成 ✅ Canonical（Phase 6, DM-20260624-001）

> North Star: 把 Learn 节点的 ReputationEvidence 注入 Observe 节点，形成 **LP-1 Bayesian reputation 闭环**。
> 博弈角色: Closed-Loop Operator（闭环执行）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S12-A41** | **BuildObserveRequest** | **A-BE** | **SessionID, UserMessage, AdaptivePrior?** | **ObserveRequest** | **—** | **✅** | **`orchestration/observe/observe_node.go::ObserveRequest`**（WithPrior 变体，Phase 6）|
| **D7-S12-A42** | **InjectPriorToSession** | **F-BE** | **ObserveRequest** | **ObserveRequest** | **—** | **✅** | **`orchestration/sessionorchestrator/observe_request.go::buildObserveRequest`**（3 层 fail-safe：prior nil → DefaultPrior → Beta(1,1) uniform）|
| **D7-S12-A43** | **E2ECloseLP1** | **A-BE** | **session_id (跨多轮)** | **ReputationEvidence round-trip** | **loop.closed** | **✅** | **`tests/integration/d7/e2e_lp1_closure_test.go`**（Phase 6 E2E 测试；E2E round-trip Reputation 注入下一轮 Observe）|

---

### D7-S13: Verify 自动闭环 ✅ Canonical（Phase 7, DM-20260625-001）

> North Star: ProcessRequest 终态时若 Verifier 未触发，自动调用 synthesizeVerdict + Auto-Close，避免无限 pending。
> 博弈角色: Auto Closer（兜底回收 pending session）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S13-A47** | **ProcessAutoClose** | **A-BE** | **ProcessRequest, session state** | **Verdict + ExitReason** | **session.auto_closed** | **✅** | **`orchestration/verify/auto_close.go::ProcessAutoClose`**（Phase 7 S7_Archived，3 层 fail-safe）|
| **D7-S13-A48** | **TrackMode** | **F-BE** | **session state** | **TrackMode (full/partial/track-only)** | **—** | **✅** | **`orchestration/verify/track_mode.go::ResolveTrackMode`** |
| **D7-S13-A49** | **EmitSessionSpanPrior** | **F-BE** | **session, prior** | **sessionSpan (6 prior attributes)** | **—** | **✅** | **`orchestration/sessionorchestrator/session_span.go::EmitPrior`**（prior.adaptive_kind / prior.beta_alpha / prior.beta_beta / prior.evidence_count / prior.cycle_count / prior.last_update）|

**4 条 Auto-Close 触发规则：** 详见 `terminal-state-guide.md` §12。

---

### D7-S14: EscapeEngine + ResumeSession ✅ Canonical（Phase v5, DM-20260625-003 + DM-20260625-004）

> North Star: 当 Observe/Plan/Execute/Verify 任一节点 stall/error 时，触发 **5 层 CircuitBreaker**（L0..L5）；用户 `/resume` 后 **3 决策路由**（A fall through / B user_accept→ForceExit / C user_cancel→AbortWithAudit）。
> 博弈角色: Escape Operator（紧急逃生 + 受控恢复）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S14-A50** | **RunEscapeEngine** | **A-BE** | **session_id, signal** | **EscapeDecision** | **escape.{triggered,lifted}** | **✅** | **`orchestration/escape/engine.go::Run`**（Phase v5 PR-V5.0 S7_Archived，5 层 CircuitBreaker L0..L5）|
| **D7-S14-A51** | **ApplyResumeSession** | **A-BE** | **session_id, user_choice** | **ResumeDecision** | **session.{resumed,force_exited,aborted}** | **✅** | **`orchestration/sessionorchestrator/resume.go::applyResumeSession`**（PR-V5.6 S7_Archived + review fix PR-V5.6-rf，3 层 fail-safe + 3 决策路由）|
| **D7-S14-A52** | **EmitSessionSpanResume** | **F-BE** | **session, decision** | **sessionSpan (3 resume attributes)** | **—** | **✅** | **`orchestration/sessionorchestrator/session_span.go::EmitResume`**（resume.decision / resume.circuit_level / resume.user_choice）|

**5 层 CircuitBreaker：**

| Level | 触发条件 | 行为 |
|-------|---------|------|
| L0 | 正常 | observe → plan → execute → verify → learn |
| L1 | 单节点 1 次 error | retry once |
| L2 | 单节点 3 次 error | 切换 fallback path |
| L3 | 跨节点 2 次 stall | 缩窄 plan 范围 |
| L4 | 跨节点 5 次 stall | pause + ask user |
| L5 | 跨节点 10 次 stall | hard escape → abort + audit |

---

## Statistics

| Scenarios | Activities | Implemented | Partial | Planned |
|-----------|------------|-------------|---------|---------|
| **6 Canonical (S1–S6) + 1 横切 (Hardening)** | **49** | **49** | **0** | **0** |
| + Legacy 追溯段（已并入 Canonical） | +0 | — | — | — |

> **v1.0 + v1.1 closure (2026-06-15):** All S-layer activities are now IMPLEMENTED. v2.0-c/f slices (A06/A07 T 层) are still PLANNED at the T level (no test fixtures in `turn/orchestrator_test.go`); the A 层 activities themselves are wired and active in `bootstrap/wire_coordinator.go`.
>
> **v4.3 MUPS closure (2026-06-25):** Canonical S 扩展至 14 个（S1-S14），MUPS 5 节点管道（Observe/Plan/Execute/Verify/Learn）+ 跨域集成 + Verify Auto-Close + EscapeEngine 全部 IMPLEMENTED。56 A 活动覆盖：S1(6) + S2(7) + S3(4) + S4(5) + S5(4) + S6(1) + S7(1,5 节点门面) + S8(1,Observe) + S9(2,Execute) + S10(4,Verify) + S11(5,Learn) + S12(3,Observe-Learner 闭环) + S13(3,Verify 自动闭环) + S14(3,EscapeEngine)。
>
> **v6.0.0 6 S closure (DM-20260626-001，2026-06-26):** Canonical S **14 → 6 + 1 横切**，A 活动 **56 → 49**。博弈角色对齐：State Authority / Mediator+Turn Leader+Error Recovery / Mechanism Designer / Costly Signaler+Certifier / Information Producer+Quantizer / Pipeline Coordinator+Memory Curator / 横切 Discipline Keeper。MUPS 5 节点管道挂载：Observe+Plan 归 S5，Execute+Learn 归 S6，Verify 归 S4，ResumeSession+EscapeEngine 入口 归 S2，AutoClose 归 S2。7 个 Legacy A 全部并入 Canonical S。详见 §v6.0.0 6 S 精简映射。

---

## v6.0.0 6 S 精简映射（DM-20260626-001）

> 14 S → 6 S + 1 横切后，A 活动重映射如下。**所有原有 A 活动均保留并归入新 S**，只是 S 编号变化；7 个 Legacy A 全部并入 Canonical S（不再保留独立 Legacy 段）。

### 14 S → 6 S 映射表

| 原 S | 原 A 数 | 新 S | 博弈角色 | 新 A 编号（示例） | 变化 |
|------|--------|------|----------|------------------|------|
| S1 Work Model | 6 | **S1 WorkModel** | State Authority | S1-A01..A04 | A01-A03 保留；A04-A06（PlanMode/PlanAgent）下放到 S1-A04 |
| S2 Session Orchestrator | 7 | **S2 SessionOrchestrator** | Mediator + Turn Leader + Error Recovery | S2-A01..A07 | A01-A04/A06/A07 保留；CommandHandler 升级为 A03 |
| S3 Wave Scheduler | 4 | **S3 WaveScheduler** | Mechanism Designer | S3-A01..A04 | 不变 |
| S4 Execution Flow | 5 | **S4 ExecutionFlow + Verify** | Costly Signaler + Certifier | S4-A01..A09 | A01-A05 保留；A06-A09 来自原 S10（Verify）|
| S5 Decision & Planning | 4 | **S5 DecisionPlanning + Observe** | Information Producer + Quantizer | S5-A01..A08 | A01-A04 保留；A05-A08 来自原 S8（Observe/Plan）|
| S6 Observability & Hardening | 1 | **Cross-cutting: Hardening** | Discipline Keeper | Hardening-A01..A02 | 不占 S 位；A14 拆为 2 A |
| S7 MUPS Pipeline | 1 | **（并入 S2 + S4 + S5 + S6）** | — | — | S7 角色拆分到 6 S（Pipeline Coord 归 S6）|
| S8 Observe | 1 | **S5** | Information Quantizer | S5-A06（ObserveQuantize）+ S5-A07（PlanGenerate）| 节点归属调整 |
| S9 Execute | 2 | **S6 MUPS Pipeline** | Pipeline Coordinator | S6-A01（ExecuteArtifact）+ S6-A02（RouteChannel）| 节点归属调整 |
| S10 Verify | 4 | **S4** | Certifier | S4-A06..A09 | 节点归属调整 |
| S11 Learn | 5 | **S6** | Memory Curator | S6-A03..A07（LearningAsset / Reputation / Memory）| 节点归属调整 |
| S12 Observe-Learner 闭环 | 3 | **S2 + S5** | Closed-Loop Operator | S2-A07（buildObserveRequest）+ S5-A08（PriorLoad）| 拆分 |
| S13 Verify 自动闭环 | 3 | **S2 + S4** | Auto Closer | S2-A07（AutoClose）+ S4-A08（sessionSpan Prior）| 拆分 |
| S14 EscapeEngine | 3 | **S2** | Escape Operator | S2-A08（EscapeDispatch）+ S2-A09（ResumeSession）| 节点入口归 S2；Engine 物理独立 |
| **总计** | **49** | **6 S + 1 横切** | — | **49** | -7（去 Legacy 段重复 + S7 拆分精简）|

### 6 S 完整 A 清单（49 A）

#### D7-S1 WorkModel — State Authority（4 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| S1-A01 | CreateWorkPlan | D7-S1-A01 | `sessionorchestrator/workmodel.go` + `workmodel/plan_mode.go` |
| S1-A02 | ManageWorkItem | D7-S1-A02 | `workmodel/{work_tree,task_manager}.go` |
| S1-A03 | QueryWorkPlan | D7-S1-A03 | `executionflow/hub/hub.go` Snapshot |
| S1-A04 | ExecutePlanAgent | D7-S1-A06（原 PlanMode A04-A05 已并入 S2 CommandHandler）| `workmodel/plan_agent.go` |

#### D7-S2 SessionOrchestrator — Mediator + Turn Leader + Error Recovery（7 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| S2-A01 | ProcessMessage | D7-S2-A01 | `sessionorchestrator/orchestrator.go` |
| S2-A02 | HandleInterrupt | D7-S2-A03 | `sessionorchestrator/interrupt.go` |
| S2-A03 | CommandHandler | D7-S2-A04-LEGACY | `sessionorchestrator/command_handler.go` |
| S2-A04 | DispatchWorker | D7-S2-A04 | `sessionorchestrator/dispatch.go` |
| S2-A05 | RunTurnLoop | D7-S2-A06 | `turn/orchestrator.go` |
| S2-A06 | InvokeLLM | D7-S2-A07 | `turn/llm.go` |
| S2-A07 | AutoClose + Resume + Escape + PriorBuild | D7-S13-A47 + D7-S14-A50/A51 + D7-S12-A42 | `sessionorchestrator/{autoclose,resume,observe_request}.go` + `escape/engine.go`（**入口归 S2，Engine 物理独立**）|

#### D7-S3 WaveScheduler — Mechanism Designer（4 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| S3-A01 | ScheduleWave | D7-S3-A01 | `wavescheduler/scheduler.go` |
| S3-A02 | ResolveWorkerContext | D7-S3-A02 | `wavescheduler/context.go` |
| S3-A03 | GuardConflict | D7-S3-A03 | `wavescheduler/conflict.go` |
| S3-A04 | HardenScheduler | D7-S3-A04 | `wavescheduler/scheduler.go::markWaveDone` |

#### D7-S4 ExecutionFlow + Verify — Costly Signaler + Certifier（9 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| S4-A01 | PublishFlowEvent | D7-S4-A01 | `executionflow/hub/hub.go` |
| S4-A02 | SnapshotWorkPlan | D7-S4-A02 | `executionflow/hub/hub.go` |
| S4-A03 | NotifyGateway | D7-S4-A03 | `executionflow/imsink/gateway.go` |
| S4-A04 | BridgeAgentSpoke | D7-S4-A04 | `executionflow/bridge/agent_bridge.go` |
| S4-A05 | BridgeSubQuerySpoke | D7-S4-A05 | `executionflow/bridge/subquery_bridge.go` |
| S4-A06 | VerifyVerdict | D7-S10-A32 | `verify/verifier.go` |
| S4-A07 | VerdictToExitReason | D7-S10-A33 | `verify/exit_reason.go` |
| S4-A08 | AggregateVerdicts | D7-S10-A34 | `verify/aggregate.go` |
| S4-A09 | DetectSystemAnomaly | D7-S10-A35 | `verify/system_anomaly.go` |

#### D7-S5 DecisionPlanning + Observe — Information Producer + Quantizer（8 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| S5-A01 | ClassifyIntent | D7-S5-A01 | `decisionplanning/classifier.go` |
| S5-A02 | SynthesizeTaskGraph | D7-S5-A02 | `decisionplanning/decomposer.go` |
| S5-A03 | SelectExecutor | D7-S5-A03 | `decisionplanning/executor.go` |
| S5-A04 | EvaluateIntent | D7-S2-A02 | `decisionplanning/classifier.go`（从 S2 移入）|
| S5-A05 | TailShadowClassify | D7-S5-A05 | `decisionplanning/shadow_classifier.go` |
| S5-A06 | ObserveQuantize | D7-S8-A15 | `observe/observe_node.go::Observe` |
| S5-A07 | PlanGenerate | D7-S8-A22 | `observe/plan/planner.go::Plan` |
| S5-A08 | PriorLoad | D7-S12-A41 | `observe/observe_node.go::ObserveRequest`（WithPrior 变体）|

#### D7-S6 MUPS Pipeline — Pipeline Coordinator + Memory Curator（15 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| S6-A01 | ExecuteArtifact | D7-S9-A25 | `execute/executor.go::Execute` |
| S6-A02 | RouteChannel | D7-S9-A26 | `execute/channel_router.go::RouteChannel` |
| S6-A03 | ChannelDispatch | （新 P0 span）⭐ | `mups/channel/router.go::Select`（v6.0.0 新增）|
| S6-A04 | ToolCall | （隐含在 S6-A01）| `execute/tool_call.go`（独立抽出）|
| S6-A05 | RetryPolicy | （隐含在 S6-A01）| `execute/retry.go`（独立抽出）|
| S6-A06 | BuildLearningAsset | D7-S11-A36 | `learn/asset_builder.go::Build` |
| S6-A07 | UpdateReputationEvidence | D7-S11-A37 | `learn/reputation_store.go::Update` |
| S6-A08 | BuildAdaptivePrior | D7-S11-A38 | `learn/adaptive_prior.go::Build` |
| S6-A09 | MemoryPersist | D7-S11-A39（新 P0 span）⭐ | `learn/memory.go::Persist`（3 通道统一）|
| S6-A10 | RunLearner | D7-S11-A40 | `learn/learner.go::Learn` |
| S6-A11 | FeedbackPersist | （从 MemoryPersist 拆出）| `learn/feedback.go` |
| S6-A12 | ScheduledPersist | （从 MemoryPersist 拆出）| `learn/scheduled.go` |
| S6-A13 | CrossSessionLearning | （隐含在 S6-A10）| `learn/cross_session.go`（独立抽出）|
| S6-A14 | ObserveLearnerLoop | D7-S12-A43 | `learn/observer_loop.go::Loop`（E2E LP-1）|
| S6-A15 | AutoClose | D7-S13-A47 | `verify/auto_close.go::ProcessAutoClose`（S6 治理下的兜底）|

#### Cross-cutting Hardening — Discipline Keeper（2 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| Hardening-A01 | HardenMetricsAndConcurrency | D7-S6-A14 | `hardening/metrics.go` + `hardening/concurrency.go`（**拆 2 A**）|
| Hardening-A02 | HardenCircuitBreakerMonitor | D7-S6-A14（部分）| `hardening/circuit_breaker.go`（**从 escape 拆分**）|

### 14 S → 6 S 合并依据（v6.0.0）

| 14 S 冗余类型 | 案例 | 6 S 解决方案 |
|---------------|------|--------------|
| 角色重合 | S4 + S9 都叫 "Costly Signaler" | S4 吸收 S9 验证角色（Verify 节点）；S9 Execute 改归 S6 Pipeline |
| 代码同址 | S7 = S2 自身；S13 = S2 内部文件；S12 散落在 S2 内 | S7/S12/S13 全部并入 S2；Engine 物理独立但入口归 S2 |
| 跨切不该独立成 S | S6 Hardening 是观测基础设施 | Hardening 改为 cross-cutting（不占 S 位）|
| 粒度过细 | S5 决策 + S8 Observe Quantize 都属"信息生产+量化" | S5 + S8 合并为 S5（Information Producer + Quantizer）|

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始注册表（全 PLANNED 路径） |
| 2.0.0 | 2026-06-14 | 对齐代码：真实路径、实现状态、PlanMode 活动 |
| 2.1.0 | 2026-06-14 | 包路径迁移 `internal/layers/d7/` → `internal/layers/orchestration/sessionorchestrator/`；D7-S2/S5-A01/S5-A05 标记 ✅ |
| 3.0.0 | 2026-06-14 | Hub-Spoke A 增量：D7-S2-A04 DispatchWorker + D7-S4-A04/A05 SpokeBridge（DM-20260614-018） |
| 3.1.0 | 2026-06-15 | Turn Leader A 增量：D7-S2-A06 RunTurnLoop + D7-S2-A07 InvokeLLM（DM-020 v1.0 Registry） |
| 3.2.0 | 2026-06-15 | **v1.0 + v1.1 闭环对齐**：(1) D7-S1 写模型迁入 coordinator/workmodel.go + workmodel/，A01-A06 全 ✅；(2) D7-S2-A02/D7-S2-A04/D7-S2-A06/D7-S2-A07 wired 至 bootstrap；(3) D7-S4-A04/A05 wired 至 hubspoke；(4) D7-S5-A02/A03 规则+LLM 双路径实装。统计 19+16/19+16/0/0 |
| 3.3.0 | 2026-06-15 | DM-020 D2→D3 拆面闭合：D7-S2-A07 InvokeLLM 兼作 D2 拆面出口（`turn.QueryLLMCaller` + `turn.CompressionSummarizer` 由同一 `llmgateway.IGateway` 驱动） |
| 3.5.0 | 2026-06-16 | Canonical 段补登 S1（A01–A06）；24 A 统计；Guides 指针 |
| 3.8.0 | 2026-06-22 | DM-20260622-001 D7 Metrics & Concurrency Hardening：新增 Canonical S6 横切层（D7-S6-A14 HardenMetricsAndConcurrency），承载 5 P0/P1 fix；Legacy S3-A04 HardenScheduler；Legacy S2-A04 CommandHandler（emit 硬化）。统计 25+20/25+20/0/0 |
| 4.0.0 | 2026-06-25 | MUPS v4.3 5 节点管道 + v5 EscapeEngine 落地（DM-20260623-001/002/003 + DM-20260624-001 + DM-20260625-001/003/004）：Canonical S 扩展至 S1-S14（14 个 S 层）。新增 7 段 + 31 A 活动。统计 56+20/56+20/0/0 |
| **5.0.0** | **2026-06-26** | **6 S 博弈角色对齐精简**（DM-20260626-001）：(1) 14 S → **6 S + 1 横切**（State Authority / Mediator+Turn Leader+Error Recovery / Mechanism Designer / Costly Signaler+Certifier / Information Producer+Quantizer / Pipeline Coordinator+Memory Curator / 横切 Discipline Keeper）；(2) A 活动 **56 → 49**（S1:4 · S2:7 · S3:4 · S4:9 · S5:8 · S6:15 + Hardening:2）；(3) **新增 §v6.0.0 6 S 精简映射**（14 S → 6 S 完整映射表 + 6 S 完整 A 清单 49 A + 14 S 冗余合并依据 4 类）；(4) 7 Legacy A 全部并入 Canonical（不再保留独立 Legacy 段）；(5) MUPS 5 节点挂载：Observe+Plan 归 S5，Execute+Learn 归 S6，Verify 归 S4，AutoClose+Resume+Escape入口 归 S2 |