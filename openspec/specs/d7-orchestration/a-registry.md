# D7 Orchestration Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D7 编排域 A 层活动注册表。现行代码分布在 `internal/layers/orchestration/`（S3/S4）与 `internal/layers/contextengine/tasks/`（S1/S5 部分）。`internal/layers/d7/` 目标包尚未创建。

**状态图例：** ✅ IMPLEMENTED · 🔶 PARTIAL · ⬜ PLANNED

---

## D7-S1: Work Model 🔶

> 写模型现行托管于 D2 `contextengine/tasks/`，目标迁移至 `internal/layers/d7/workmodel.go`。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S1-A01 | CreateWorkPlan | A-BE | session_id, goal, context | Plan (Task 列表 + DAG) | work_plan.created | ⬜ | `d7/workmodel.go` (planned) |
| D7-S1-A02 | ManageTask | A-BE | task_id, action (create/update/delete/dep) | task_state | task.* | ✅ | `contextengine/tasks/task_manager.go` |
| D7-S1-A03 | QueryWorkPlan | A-BE | session_id | WorkPlanSnapshot | — | ✅ | `orchestration/flow/hub.go` Snapshot |

### D7-S1 附加活动（PlanMode）

| A ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A04 | EnterPlanMode | A-BE | session_id, goal | plan_state | ✅ | `contextengine/tasks/plan_mode.go` |
| D7-S1-A05 | ApprovePlan | A-BE | session_id | []Task | ✅ | `contextengine/tasks/plan_mode.go` |
| D7-S1-A06 | ExecutePlanAgent | A-BE | goal, context | PlanResult | ✅ | `contextengine/tasks/plan_agent.go` |

---

## D7-S2: Session Orchestrator ⬜

> D1 现行仍调用 `D2.ContextEngine.Process`（`gateway.go:286`）。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S2-A01 | ProcessMessage | A-BE | message, session | events stream | session.orchestrating | ⬜ | `d7/orchestrator.go` (planned) |
| D7-S2-A02 | EvaluateIntent | A-BE | message, context | IntentClassification | — | ⬜ | `d7/classifier.go` (planned) |
| D7-S2-A03 | HandleInterrupt | A-BE | session_id, reason | — | session.interrupted | 🔶 | `communication/gateway/gateway.go` StopProcess |

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

> PlanMode/PlanAgent 已实现；意图分类与任务拆解尚未实现。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S5-A01 | ClassifyIntent | A-BE | message, session, context | IntentClassification | — | ⬜ | `d7/classifier.go` (planned) |
| D7-S5-A02 | SynthesizeTaskGraph | A-BE | goal, constraints | []TaskNode | plan.formulated | ⬜ | `d7/decomposer.go` (planned) |
| D7-S5-A03 | SelectExecutor | A-BE | TaskNode, agent_pool | executor_id (D2/D4) | — | ⬜ | `d7/executor.go` (planned) |
| D7-S5-A04 | RunPlanAgent | A-BE | goal, readonly context | PlanResult | plan.generated | ✅ | `contextengine/tasks/plan_agent.go` |

---

## Statistics

| Scenarios | Activities | Implemented | Partial | Planned |
|-----------|------------|-------------|---------|---------|
| 5 | 18 | 10 | 4 | 4 |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始注册表（全 PLANNED 路径） |
| 2.0.0 | 2026-06-14 | 对齐代码：真实路径、实现状态、PlanMode 活动 |
