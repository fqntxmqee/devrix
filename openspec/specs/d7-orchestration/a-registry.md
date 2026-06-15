# D7 Orchestration Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.4.0
**Last Updated:** 2026-06-15
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D7 编排域 A 层活动注册表。代码分布：
- `internal/layers/orchestration/coordinator/`（D7-S2 Session Orchestrator + D7-S5 Intent/Decision）— D7 包命名为 `coordinator`
- `internal/layers/orchestration/wave|flow|workplan|imsink/`（D7-S3 Wave Scheduler + D7-S4 Execution Flow）— 下层原语
- `internal/layers/contextengine/tasks/`（D7-S1 部分 + D7-S5 PlanMode）— 写模型与 PlanMode 仍托管于 D2（v1.1 迁入 coordinator）

**状态图例：** ✅ IMPLEMENTED · 🔶 PARTIAL · ⬜ PLANNED

---

## Legacy 双轨方案（v1.0+）

> 根据 `devrix-d7-sa-refine` (DM-20260614-008) §7 设计决策：
> - **Legacy** — 旧编号冻结追溯，路径：`internal/layers/orchestration/coordinator/`（旧包结构）
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

> Task/Plan 写模型已迁入 `internal/layers/orchestration/coordinator/workmodel.go` + `internal/layers/orchestration/workmodel/` 包（v1.1 closure, layer-delta Phase I/J）。`contextengine/tasks/` 保留为兼容 shim。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S1-A01 | CreateWorkPlan | A-BE | session_id, goal, context | Plan (Task 列表 + DAG) | work_plan.created | ✅ | `coordinator/workmodel.go` + `workmodel/plan_mode.go` |
| D7-S1-A02 | ManageTask | A-BE | task_id, action (create/update/delete/dep) | task_state | task.* | ✅ | `workmodel/task_manager.go` + `task_store.go` |
| D7-S1-A03 | QueryWorkPlan | A-BE | session_id | WorkPlanSnapshot | — | ✅ | `orchestration/flow/hub.go` Snapshot |

### D7-S1 附加活动（PlanMode）

| A ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A04 | EnterPlanMode | A-BE | session_id, goal | plan_state | ✅ | `workmodel/plan_mode.go` |
| D7-S1-A05 | ApprovePlan | A-BE | session_id | []Task | ✅ | `workmodel/plan_mode.go` |
| D7-S1-A06 | ExecutePlanAgent | A-BE | goal, context | PlanResult | ✅ | `workmodel/plan_agent.go` |

---

## D7-S2: Session Orchestrator ✅ LEGACY

> D1 经 `gateway.d7Entry` 路由至 `coordinator.Entry.ProcessMessage`，由 SessionOrchestrator 编排 D2/D4/D7。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S2-A01 | ProcessMessage | A-BE | message, session | events stream | session.orchestrating | ✅ | `orchestration/coordinator/orchestrator.go` |
| D7-S2-A02 | EvaluateIntent | A-BE | message, context | IntentClassification | — | ✅ | `orchestration/coordinator/classifier.go` + `classifier_fallback.go` |
| D7-S2-A03 | HandleInterrupt | A-BE | session_id, reason | — | session.interrupted | ✅ | `orchestration/coordinator/interrupt.go` |

---

## D7-S3: Wave Scheduler ✅ LEGACY

> 升格自 ORCH-S3，完整实现于 `internal/layers/orchestration/wave/`。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S3-A01 | ScheduleWave | A-BE | session_id, task_graph | artifact_list | wave.* | ✅ | `orchestration/wave/scheduler.go` |
| D7-S3-A02 | ResolveWorkerContext | A-BE | task_node, session | resolved_context | — | ✅ | `orchestration/wave/context.go` |
| D7-S3-A03 | GuardConflict | A-BE | candidate, running_tasks | allowed/blocked | — | ✅ | `orchestration/wave/conflict.go` |

---

## D7-S4: Execution Flow ✅ LEGACY

> 升格自 ORCH-S1/S2，实现于 `orchestration/flow/`, `workplan/`, `imsink/`。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S4-A01 | PublishFlowEvent | A-BE | flow_event | — | flow.event_published | ✅ | `orchestration/flow/hub.go` Publish |
| D7-S4-A02 | SnapshotWorkPlan | A-BE | session_id | WorkPlanSnapshot | — | ✅ | `orchestration/flow/hub.go` Snapshot |
| D7-S4-A03 | NotifyGateway | A-BE | event, session | — | — | ✅ | `orchestration/imsink/gateway.go` |

---

## D7-S5: Decision & Planning ✅ LEGACY

> PlanMode/PlanAgent、ClassifyIntent（规则+LLM fallback）、SynthesizeTaskGraph、SelectExecutor 均已实现（layer-delta Phase H/K/L/M）。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S5-A01 | ClassifyIntent | A-BE | message, session, context | IntentClassification | — | ✅ | `orchestration/coordinator/classifier.go` + `classifier_fallback.go` (LLM merge) |
| D7-S5-A02 | SynthesizeTaskGraph | A-BE | goal, constraints, explore_events | []TaskNode | plan.formulated | ✅ | `coordinator/decomposer.go` (rule + LLM via SetLLMDecomposer) |
| D7-S5-A03 | SelectExecutor | A-BE | TaskNode, agent_pool | executor_id (D2/D4) | — | ✅ | `coordinator/executor.go` |
| D7-S5-A04 | RunPlanAgent | A-BE | goal, readonly context | PlanResult | plan.generated | ✅ | `workmodel/plan_agent.go` |
| D7-S5-A05 | TailShadowClassify | A-BE | message, rule_result | — | metric only | ✅ | `orchestration/coordinator/shadow_classifier.go` |

---

## Canonical S 层定义（切法 A — 按用户价值流）

> 以下为 `devrix-d7-sa-refine` v1.0 Canonical 定义。按用户价值流划分 S 层，Legacy 双轨可追溯。

### D7-S2: 会话编排入口 + Turn Leader ✅ Canonical

> North Star: 用户消息统一入口，决定走快速路径还是编排路径；**拥有 LLM 调用权与 Turn 主循环（DM-020）**
> 博弈角色: Screening Mechanism（筛路径）+ **Turn Leader（Stackelberg 先手）**

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| D7-S2-A01 | ProcessMessage | D7-S2-A01-LEGACY | A-BE | message, session | events stream | session.orchestrating | ✅ | `orchestration/coordinator/orchestrator.go`（v1.1.0+ 4 case = 4 独立执行链：`CommandHandler` / `FastPath` / `OrchestratePath` + `IntentSkip` 内联 close channel）|
| D7-S2-A02 | EvaluateIntent | — | A-BE | message, context | IntentClassification | — | ✅ | `orchestration/coordinator/classifier.go` + `classifier_fallback.go` |
| D7-S2-A03 | HandleInterrupt | D7-S2-A03-LEGACY | A-BE | session_id, reason | — | session.interrupted | ✅ | `orchestration/coordinator/interrupt.go` |
| D7-S2-A04 | DispatchWorker | D4-S10-A01（编排面） | A-BE | leader, worker_spec | spoke_id, executor | task.{delegated,completed,failed} | ✅ | `hubspoke/dispatch.go`（v1.0 路径：`bootstrap/delegate.go` 已 wired） |
| **D7-S2-A06** | **RunTurnLoop** | — | **A-BE** | **session, TurnRequest** | **<-chan EngineEvent** | **turn.{started,completed,failed}** | **✅** | **`orchestration/turn/orchestrator.go`**（DM-020 v1.0-c；wired by `bootstrap/wire_coordinator.go:60`） |
| **D7-S2-A07** | **InvokeLLM** | — | **A-BE** | **LLMInvokeRequest** | **<-chan Chunk** | **llm.{invoked,streaming,completed}** | **✅** | **`orchestration/turn/llm.go`**（DM-020 v1.0-b；wired by `bootstrap/wire_coordinator.go:59`）。**兼作 D2→D3 拆面出口**：`turn.QueryLLMCaller` + `turn.CompressionSummarizer` 由同一 `llmgateway.IGateway` 驱动，单一注入点 `bootstrap/context_engine.go` wired 至 `EngineDeps.QueryLLMCaller` / `EngineDeps.Summarizer`（DM-020 v2.3 拆面闭合） |

> **D7-S2-A04**（DM-20260614-018）：Hub-Spoke 派发矩阵 + fallback 路由。v1.0 逻辑在 D4 `delegate/service.go`；v2.0 迁 `hubspoke/dispatch.go`。

### D7-S3: Wave 调度 ✅ Canonical

> North Star: 多任务并行执行，冲突避免，上下文隔离
> 博弈角色: Mechanism Designer（定执行规则）

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| D7-S3-A01 | ScheduleWave | D7-S3-A01-LEGACY | A-BE | session_id, task_graph | artifact_list | wave.* | ✅ | `orchestration/wave/scheduler.go` |
| D7-S3-A02 | ResolveWorkerContext | D7-S3-A02-LEGACY | A-BE | task_node, session | resolved_context | — | ✅ | `orchestration/wave/context.go` |
| D7-S3-A03 | GuardConflict | D7-S3-A03-LEGACY | A-BE | candidate, running_tasks | allowed/blocked | — | ✅ | `orchestration/wave/conflict.go` |

### D7-S4: 执行流 ✅ Canonical

> North Star: 执行进度透明，WorkPlan 可追溯
> 博弈角色: Costly Signaler（向用户广播成本）

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| D7-S4-A01 | PublishFlowEvent | D7-S4-A01-LEGACY | A-BE | flow_event | — | flow.event_published | ✅ | `orchestration/flow/hub.go` Publish |
| D7-S4-A02 | SnapshotWorkPlan | D7-S4-A02-LEGACY | A-BE | session_id | WorkPlanSnapshot | — | ✅ | `orchestration/flow/hub.go` Snapshot |
| D7-S4-A03 | NotifyGateway | D7-S4-A03-LEGACY | A-BE | event, session | — | — | ✅ | `orchestration/imsink/gateway.go` |
| D7-S4-A04 | BridgeAgentSpoke | D4-S10-A02 | A-BE | agent_event, engine_event | — | flow.published | ✅ | `hubspoke/agent_bridge.go`（wired by `bootstrap/delegate.go:52`） |
| D7-S4-A05 | BridgeSubQuerySpoke | D2-S19（Flow 面） | A-BE | subquery_result | flow_event | flow.published | ✅ | `hubspoke/subquery_bridge.go`（wired by `bootstrap/delegate.go`） |

> **D7-S4-A04/A05**（DM-20260614-018）：统一三 Spoke 写侧（D4 Delegate / D2 SubQuery / D7 Wave）→ `ExecutionFlowHub`。

### D7-S5: 决策规划 🔶 Canonical

> North Star: 把用户 goal 转化为可执行的任务结构（结构路径，非内容质量）
> 博弈角色: Information Producer（产私有信息）
> **Explore 输入:** SynthesizeTaskGraph 吸收 Explore Workers（并行 read-only）通过 D7-S4 广播的 FlowEvent

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| D7-S5-A01 | ClassifyIntent | D7-S5-A01-LEGACY | A-BE | message, session, context | IntentClassification | — | ✅ | `orchestration/coordinator/classifier.go` + `classifier_fallback.go` |
| D7-S5-A02 | SynthesizeTaskGraph | D7-S5-A02-LEGACY | A-BE | goal, constraints, explore_events | []TaskNode | plan.formulated | ✅ | `coordinator/decomposer.go`（rule + LLM via SetLLMDecomposer） |
| D7-S5-A03 | SelectExecutor | — | A-BE | TaskNode, agent_pool | executor_id (D2/D4) | — | ✅ | `coordinator/executor.go` |

---

## Statistics

| Scenarios | Activities | Implemented | Partial | Planned |
|-----------|------------|-------------|---------|---------|
| 5 (Legacy) + 4 (Canonical) | 19 (Legacy) + 16 (Canonical) | 19 + 16 | 0 | 0 |

> **v1.0 + v1.1 closure (2026-06-15):** All S-layer activities are now IMPLEMENTED. v2.0-c/f slices (A06/A07 T 层) are still PLANNED at the T level (no test fixtures in `turn/orchestrator_test.go`); the A 层 activities themselves are wired and active in `bootstrap/wire_coordinator.go`.

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始注册表（全 PLANNED 路径） |
| 2.0.0 | 2026-06-14 | 对齐代码：真实路径、实现状态、PlanMode 活动 |
| 2.1.0 | 2026-06-14 | 包路径迁移 `internal/layers/d7/` → `internal/layers/orchestration/coordinator/`；D7-S2/S5-A01/S5-A05 标记 ✅ |
| 3.0.0 | 2026-06-14 | Hub-Spoke A 增量：D7-S2-A04 DispatchWorker + D7-S4-A04/A05 SpokeBridge（DM-20260614-018） |
| 3.1.0 | 2026-06-15 | Turn Leader A 增量：D7-S2-A06 RunTurnLoop + D7-S2-A07 InvokeLLM（DM-020 v1.0 Registry） |
| 3.2.0 | 2026-06-15 | **v1.0 + v1.1 闭环对齐**：(1) D7-S1 写模型迁入 coordinator/workmodel.go + workmodel/，A01-A06 全 ✅；(2) D7-S2-A02/D7-S2-A04/D7-S2-A06/D7-S2-A07 wired 至 bootstrap；(3) D7-S4-A04/A05 wired 至 hubspoke；(4) D7-S5-A02/A03 规则+LLM 双路径实装。统计 19+16/19+16/0/0 |
| **3.3.0** | **2026-06-15** | **DM-020 D2→D3 拆面闭合**：D7-S2-A07 InvokeLLM 兼作 D2 拆面出口（`turn.QueryLLMCaller` + `turn.CompressionSummarizer` 由同一 `llmgateway.IGateway` 驱动） |
| **3.4.0** | **2026-06-15** | **DM-20260615-004 D7 Intent 路径正交化**：D7-S2-A01 ProcessMessage switch 4 case 改为 4 独立执行链（`CommandHandler` / `FastPath` / `OrchestratePath` + `IntentSkip`）；D7-S2-A01 标注补充 v1.1.0+ orthogonal 形态 |