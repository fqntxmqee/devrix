# D7 Orchestration Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D7 编排域 A 层活动注册表。升格自 ORCH v2 读模型包，负责任务模型、会话编排、波浪调度、执行流、决策与规划。

---

## D7-S1: Work Model

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D7-S1-A01 | CreateWorkPlan | A-BE | session_id, goal, context | Plan (Task 列表 + DAG) | work_plan.created | `d7/workmodel.go` |
| D7-S1-A02 | ManageTask | A-BE | task_id, action (create/assign/complete/cancel/tick) | task_state | task.{assigned,running,completed,failed,cancelled} | `d7/workmodel.go` |
| D7-S1-A03 | QueryWorkPlan | A-BE | session_id | WorkPlanSnapshot | — | `d7/workmodel.go` |

## D7-S2: Session Orchestrator

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D7-S2-A01 | ProcessMessage | A-BE | message, session | events stream | session.orchestrating | `d7/orchestrator.go` |
| D7-S2-A02 | EvaluateIntent | A-BE | message, context | IntentClassification | — | `d7/classifier.go` |
| D7-S2-A03 | HandleInterrupt | A-BE | session_id, reason | — | session.interrupted | `d7/orchestrator.go` |

## D7-S3: Wave Scheduler

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D7-S3-A01 | ScheduleWave | A-BE | session_id, task_graph | artifact_list | wave.{started,completed,failed} | `d7/wave/scheduler.go` |
| D7-S3-A02 | ResolveWorkerContext | A-BE | task_node, session | resolved_context | — | `d7/wave/context.go` |
| D7-S3-A03 | GuardConflict | A-BE | candidate, running_tasks | allowed/blocked | — | `d7/wave/conflict.go` |

## D7-S4: Execution Flow

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D7-S4-A01 | PublishFlowEvent | A-BE | flow_event | — | flow.event_published | `d7/flow/hub.go` |
| D7-S4-A02 | SnapshotWorkPlan | A-BE | session_id | flow_snapshot | — | `d7/flow/hub.go` |
| D7-S4-A03 | NotifyGateway | A-BE | event, session | — | — | `d7/flow/imsink.go` |

## D7-S5: Decision & Planning

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D7-S5-A01 | ClassifyIntent | A-BE | message, session, context | IntentClassification | — | `d7/classifier.go` |
| D7-S5-A02 | SynthesizeTaskGraph | A-BE | goal, constraints | []TaskNode | plan.formulated | `d7/decomposer.go` |
| D7-S5-A03 | SelectExecutor | A-BE | TaskNode, agent_pool | executor_id (D2/D4) | — | `d7/executor.go` |

---

## Statistics

| Scenarios | Activities |
|-----------|------------|
| 5 | 15 |
