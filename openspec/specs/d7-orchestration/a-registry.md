# D7 Orchestration Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 2.1.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D7 编排域 A 层活动注册表。代码分布：
- `internal/layers/orchestration/coordinator/`（D7-S2 Session Orchestrator + D7-S5 Intent/Decision）— D7 包命名为 `coordinator`
- `internal/layers/orchestration/wave|flow|workplan|imsink/`（D7-S3 Wave Scheduler + D7-S4 Execution Flow）— 下层原语
- `internal/layers/contextengine/tasks/`（D7-S1 部分 + D7-S5 PlanMode）— 写模型与 PlanMode 仍托管于 D2（v1.1 迁入 coordinator）

**状态图例：** ✅ IMPLEMENTED · 🔶 PARTIAL · ⬜ PLANNED

---

## D7-S1: Work Model 🔶

> 写模型现行托管于 D2 `contextengine/tasks/`，v1.1 迁移至 `internal/layers/orchestration/coordinator/workmodel.go`。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S1-A01 | CreateWorkPlan | A-BE | session_id, goal, context | Plan (Task 列表 + DAG) | work_plan.created | ⬜ | `coordinator/workmodel.go` (planned v1.1) |
| D7-S1-A02 | ManageTask | A-BE | task_id, action (create/update/delete/dep) | task_state | task.* | ✅ | `contextengine/tasks/task_manager.go` |
| D7-S1-A03 | QueryWorkPlan | A-BE | session_id | WorkPlanSnapshot | — | ✅ | `orchestration/flow/hub.go` Snapshot |

### D7-S1 附加活动（PlanMode）

| A ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A04 | EnterPlanMode | A-BE | session_id, goal | plan_state | ✅ | `contextengine/tasks/plan_mode.go` |
| D7-S1-A05 | ApprovePlan | A-BE | session_id | []Task | ✅ | `contextengine/tasks/plan_mode.go` |
| D7-S1-A06 | ExecutePlanAgent | A-BE | goal, context | PlanResult | ✅ | `contextengine/tasks/plan_agent.go` |

---

## D7-S2: Session Orchestrator ✅

> D1 经 `gateway.d7Entry` 路由至 `coordinator.Entry.ProcessMessage`，由 SessionOrchestrator 编排 D2/D4/D7。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S2-A01 | ProcessMessage | A-BE | message, session | events stream | session.orchestrating | ✅ | `orchestration/coordinator/orchestrator.go` |
| D7-S2-A02 | EvaluateIntent | A-BE | message, context | IntentClassification | — | ✅ | `orchestration/coordinator/classifier.go` |
| D7-S2-A03 | HandleInterrupt | A-BE | session_id, reason | — | session.interrupted | ✅ | `orchestration/coordinator/interrupt.go` |

---

## D7-S3: Wave Scheduler ✅

> 升格自 ORCH-S3，完整实现于 `internal/layers/orchestration/wave/`。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S3-A01 | ScheduleWave | A-BE | session_id, task_graph | artifact_list | wave.* | ✅ | `orchestration/wave/scheduler.go` |
| D7-S3-A02 | ResolveWorkerContext | A-BE | task_node, session | resolved_context | — | ✅ | `orchestration/wave/context.go` |
| D7-S3-A03 | GuardConflict | A-BE | candidate, running_tasks | allowed/blocked | — | ✅ | `orchestration/wave/conflict.go` |

---

## D7-S4: Execution Flow ✅

> 升格自 ORCH-S1/S2，实现于 `orchestration/flow/`, `workplan/`, `imsink/`。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S4-A01 | PublishFlowEvent | A-BE | flow_event | — | flow.event_published | ✅ | `orchestration/flow/hub.go` Publish |
| D7-S4-A02 | SnapshotWorkPlan | A-BE | session_id | WorkPlanSnapshot | — | ✅ | `orchestration/flow/hub.go` Snapshot |
| D7-S4-A03 | NotifyGateway | A-BE | event, session | — | — | ✅ | `orchestration/imsink/gateway.go` |

---

## D7-S5: Decision & Planning 🔶

> PlanMode/PlanAgent 已实现；ClassifyIntent/Rule 分类已实现（coordinator）；SynthesizeTaskGraph/SelectExecutor 推迟至 v1.1。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S5-A01 | ClassifyIntent | A-BE | message, session, context | IntentClassification | — | ✅ | `orchestration/coordinator/classifier.go` |
| D7-S5-A02 | SynthesizeTaskGraph | A-BE | goal, constraints | []TaskNode | plan.formulated | ⬜ | `coordinator/decomposer.go` (planned v1.1) |
| D7-S5-A03 | SelectExecutor | A-BE | TaskNode, agent_pool | executor_id (D2/D4) | — | ⬜ | `coordinator/executor.go` (planned v1.1) |
| D7-S5-A04 | RunPlanAgent | A-BE | goal, readonly context | PlanResult | plan.generated | ✅ | `contextengine/tasks/plan_agent.go` |
| D7-S5-A05 | TailShadowClassify | A-BE | message, rule_result | — | metric only | ✅ | `orchestration/coordinator/shadow_classifier.go` |

---

## Statistics

| Scenarios | Activities | Implemented | Partial | Planned |
|-----------|------------|-------------|---------|---------|
| 5 | 19 | 14 | 1 | 4 |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始注册表（全 PLANNED 路径） |
| 2.0.0 | 2026-06-14 | 对齐代码：真实路径、实现状态、PlanMode 活动 |
| 2.1.0 | 2026-06-14 | 包路径迁移 `internal/layers/d7/` → `internal/layers/orchestration/coordinator/`；D7-S2/S5-A01/S5-A05 标记 ✅ |