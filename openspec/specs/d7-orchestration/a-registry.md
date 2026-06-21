# D7 Orchestration Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.8.0
**Last Updated:** 2026-06-22
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `d7-domain.md`

---

## Overview

D7 编排域 A 层活动注册表。

> **终态流程 / IntentKind 四链：** 见 `terminal-state-guide.md` §3–§7；**Span↔T Runbook：** 见 `observability-guide.md`。

代码分布：

- `internal/layers/orchestration/sessionorchestrator/`（D7-S2）
- `internal/layers/orchestration/decisionplanning/`（D7-S5）
- `internal/layers/orchestration/wavescheduler/` + `executionflow/`（D7-S3 + D7-S4）
- `internal/layers/orchestration/workmodel/`（D7-S1）
- `internal/layers/orchestration/turn/`（D7-S2-A06/A07 Turn Leader）
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

## D7-S1: Work Model ✅ LEGACY

> Task/Plan 写模型已迁入 `internal/layers/orchestration/sessionorchestrator/workmodel.go` + `internal/layers/orchestration/workmodel/` 包（v1.1 closure, layer-delta Phase I/J）。`contextengine/tasks/` 保留为兼容 shim。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S1-A01 | CreateWorkPlan | A-BE | session_id, goal, context | Plan (Task 列表 + DAG) | work_plan.created | ✅ | `sessionorchestrator/workmodel.go` + `workmodel/plan_mode.go` |
| D7-S1-A02 | ManageTask | A-BE | task_id, action (create/update/delete/dep) | task_state | task.* | ✅ | `workmodel/task_manager.go` + `task_store.go` |
| D7-S1-A03 | QueryWorkPlan | A-BE | session_id | WorkPlanSnapshot | — | ✅ | `orchestration/executionflow/hub/hub.go` Snapshot |

### D7-S1 附加活动（PlanMode）

| A ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A04 | EnterPlanMode | A-BE | session_id, goal | plan_state | ✅ | `workmodel/plan_mode.go` |
| D7-S1-A05 | ApprovePlan | A-BE | session_id | []Task | ✅ | `workmodel/plan_mode.go` |
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

### D7-S1: Work Model ✅ Canonical

> North Star: Task/Plan **事实与状态机**单一权威
> 博弈角色: **State Authority**

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| D7-S1-A01 | CreateWorkPlan | D7-S1-A01-LEGACY | A-BE | session_id, goal, context | Plan | work_plan.created | ✅ | `sessionorchestrator/workmodel.go` + `workmodel/plan_mode.go` |
| D7-S1-A02 | ManageTask | D7-S1-A02-LEGACY | A-BE | task_id, action | task_state | task.* | ✅ | `workmodel/task_manager.go` + `task_store.go` |
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
| **D7-S2-A06** | **RunTurnLoop** | — | **A-BE** | **session, TurnRequest** | **<-chan EngineEvent** | **turn.{started,completed,failed}** | **✅** | **`orchestration/turn/orchestrator.go`**（DM-020 v1.0-c；wired by `bootstrap/wire_coordinator.go:60`） |
| **D7-S2-A07** | **InvokeLLM** | — | **A-BE** | **LLMInvokeRequest** | **<-chan Chunk** | **llm.{invoked,streaming,completed}** | **✅** | **`orchestration/turn/llm.go`**（DM-020 v1.0-b；wired by `bootstrap/wire_coordinator.go:59`）。**兼作 D2→D3 拆面出口**：`turn.QueryLLMCaller` + `turn.CompressionSummarizer` 由同一 `llmgateway.IGateway` 驱动，单一注入点 `bootstrap/context_engine.go` wired 至 `EngineDeps.QueryLLMCaller` / `EngineDeps.Summarizer`（DM-020 v2.3 拆面闭合） |

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

## Statistics

| Scenarios | Activities | Implemented | Partial | Planned |
|-----------|------------|-------------|---------|---------|
| 6 Canonical (S1–S6) | 25 | 25 | 0 | 0 |
| + Legacy 追溯段 | +20 | — | — | — |

> **v1.0 + v1.1 closure (2026-06-15):** All S-layer activities are now IMPLEMENTED. v2.0-c/f slices (A06/A07 T 层) are still PLANNED at the T level (no test fixtures in `turn/orchestrator_test.go`); the A 层 activities themselves are wired and active in `bootstrap/wire_coordinator.go`.

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
| **3.3.0** | **2026-06-15** | **DM-020 D2→D3 拆面闭合**：D7-S2-A07 InvokeLLM 兼作 D2 拆面出口（`turn.QueryLLMCaller` + `turn.CompressionSummarizer` 由同一 `llmgateway.IGateway` 驱动） |
| **3.5.0** | **2026-06-16** | Canonical 段补登 S1（A01–A06）；24 A 统计；Guides 指针 |
| **3.8.0** | **2026-06-22** | **DM-20260622-001 D7 Metrics & Concurrency Hardening**：(1) 新增 Canonical S6 横切层（D7-S6-A14 HardenMetricsAndConcurrency），承载 5 P0/P1 fix（metric plural 对齐 + state.cancels bound + AllowAndRegister 原子化 + CommandHandler select-default + sandbox_exit_failed 跨域归属澄清）；(2) Legacy S3-A04 HardenScheduler（markWaveDone + dispatchOne 原子化）；(3) Legacy S2-A04 CommandHandler（emit 硬化）。统计 25+20/25+20/0/0 |