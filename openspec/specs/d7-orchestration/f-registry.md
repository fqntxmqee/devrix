# D7 Orchestration Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d7-orchestration/a-registry.md`

---

## Overview

D7 编排域 F 层功能点注册表。代码位置标注**现行路径**；`(planned)` 表示目标 D7 包尚未创建。

**状态图例：** ✅ · 🔶 · ⬜

---

## D7-S1-A02 ManageTask ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A02-F01 | CreateTask | F-BE | subject, description | Task | ✅ | `contextengine/tasks/task_manager.go` |
| D7-S1-A02-F02 | UpdateTaskStatus | F-BE | task_id, status | Task | ✅ | `contextengine/tasks/task_manager.go` |
| D7-S1-A02-F03 | AddDependency | F-BE | task_id, blocked_by | — | ✅ | `contextengine/tasks/task_manager.go` |
| D7-S1-A02-F04 | ListReadyTasks | F-BE | session_id | []Task | ✅ | `contextengine/tasks/task_manager.go` |
| D7-S1-A02-F05 | PersistToDisk | F-BE | session_id | — | ✅ | `contextengine/tasks/disk_store.go` |
| D7-S1-A02-F06 | SetOwner | F-BE | task_id, worker_id | — | ✅ | `contextengine/tasks/task_manager.go` |

## D7-S1-A01 CreateWorkPlan ⬜

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A01-F01 | SynthesizePlan | F-BE | goal, context | []TaskNode | ⬜ | `d7/workmodel.go` (planned) |
| D7-S1-A01-F02 | ValidatePlanDAG | F-BE | []TaskNode | valid/invalid | ⬜ | `d7/workmodel.go` (planned) |
| D7-S1-A01-F03 | EstimateTaskScope | F-BE | goal, context | complexity_score | ⬜ | `d7/workmodel.go` (planned) |

## D7-S1-A03 QueryWorkPlan ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A03-F01 | BuildSessionSnapshot | F-BE | session_id | WorkPlanSnapshot | ✅ | `orchestration/flow/hub.go` |
| D7-S1-A03-F02 | BuildTaskSnapshots | F-BE | session_id | []TaskSnapshot | ✅ | `orchestration/flow/hub.go` taskSnapshots |
| D7-S1-A03-F03 | ApplyFlowEvent | F-BE | FlowEvent | — | ✅ | `orchestration/workplan/service.go` Apply |

## D7-S1-A04/A05 PlanMode ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A04-F01 | EnterPlanMode | F-BE | session_id, goal | — | ✅ | `contextengine/tasks/plan_mode.go` |
| D7-S1-A04-F02 | GeneratePlan | F-BE | goal | PlanResult | ✅ | `contextengine/tasks/plan_agent.go` |
| D7-S1-A05-F01 | ApprovePlan | F-BE | session_id | []Task | ✅ | `contextengine/tasks/plan_mode.go` |
| D7-S1-A05-F02 | RejectPlan | F-BE | session_id | — | ✅ | `contextengine/tasks/plan_mode.go` |

## D7-S2-A01 ProcessMessage ⬜

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S2-A01-F01 | RouteByIntent | F-BE | IntentClassification | routing_decision | ⬜ | `d7/orchestrator.go` (planned) |
| D7-S2-A01-F02 | ExecuteFastPath | F-BE | message, session | events | ⬜ | `d7/fastpath.go` (planned) |
| D7-S2-A01-F03 | EnterOrchestration | F-BE | message, session | Plan | ⬜ | `d7/orchestrator.go` (planned) |
| D7-S2-A01-F04 | OrchestrateSessionLoop | F-BE | plan, ctx | — | ⬜ | `d7/orchestrator.go` (planned) |
| D7-S2-A01-F05 | EmitSessionEvents | F-BE | event | — | ⬜ | `d7/orchestrator.go` (planned) |

## D7-S2-A02 EvaluateIntent ⬜

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S2-A02-F01 | ClassifyByRules | F-BE | message | rules_hint + confidence | ⬜ | `d7/classifier.go` (planned) |
| D7-S2-A02-F02 | ScoreComplexity | F-BE | message | complexity | ⬜ | `d7/classifier.go` (planned) |

## D7-S2-A03 HandleInterrupt 🔶

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S2-A03-F01 | CancelActiveProcess | F-BE | session_id | — | 🔶 | `communication/gateway/gateway.go` StopProcess |
| D7-S2-A03-F02 | CancelWaveWorkers | F-BE | session_id | — | ✅ | `orchestration/wave/scheduler.go` CancelAll |

## D7-S3-A01 ScheduleWave ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S3-A01-F01 | StartWave | F-BE | session_id, task_graph | — | ✅ | `orchestration/wave/scheduler.go` Start |
| D7-S3-A01-F02 | DispatchWorker | F-BE | task_node, slot | — | ✅ | `orchestration/wave/scheduler.go` dispatchOne |
| D7-S3-A01-F03 | WaitForCompletion | F-BE | session_id | []Artifact | ✅ | `orchestration/wave/scheduler.go` WaitForCompletion |
| D7-S3-A01-F04 | ContinuousRedispatch | F-BE | slot_release | — | ✅ | `orchestration/wave/scheduler.go` dispatchLoop |
| D7-S3-A01-F05 | CancelWorker | F-BE | task_id | — | ✅ | `orchestration/wave/scheduler.go` CancelWorker |

## D7-S3-A02 ResolveWorkerContext ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S3-A02-F01 | ResolveFreshContext | F-BE | task_node | ResolvedContext | ✅ | `orchestration/wave/context.go` resolveFresh |
| D7-S3-A02-F02 | ResolveUpstreamContext | F-BE | task_node, artifacts | ResolvedContext | ✅ | `orchestration/wave/context.go` resolveUpstream |
| D7-S3-A02-F03 | ResolveResumeContext | F-BE | task_node, sidechain | ResolvedContext | ✅ | `orchestration/wave/context.go` resolveResume |

## D7-S3-A03 GuardConflict ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S3-A03-F01 | CheckConflict | F-BE | candidate, running | allowed/blocked | ✅ | `orchestration/wave/conflict.go` Allow |
| D7-S3-A03-F02 | RegisterTaskGuard | F-BE | task_node, slot | — | ✅ | `orchestration/wave/conflict.go` Register |
| D7-S3-A03-F03 | CheckFileScopeOverlap | F-BE | file_scope[] | bool | ✅ | `orchestration/wave/conflict.go` pathsOverlap |

## D7-S3 基础设施 ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S3-F01 | AcquireSlot | F-BE | worker_type | SlotID | ✅ | `orchestration/wave/pool.go` Acquire |
| D7-S3-F02 | ReleaseSlot | F-BE | slot_id | — | ✅ | `orchestration/wave/pool.go` Release |
| D7-S3-F03 | ReadyNodes | F-BE | — | []TaskNode | ✅ | `orchestration/wave/taskgraph.go` ReadyNodes |
| D7-S3-F04 | StoreArtifact | F-BE | artifact | — | ✅ | `orchestration/wave/artifact.go` Put |
| D7-S3-F05 | RunSubAgent | F-BE | WorkerRunSpec | error | ✅ | `orchestration/wave/runners/subagent.go` |
| D7-S3-F06 | RunAgentTool | F-BE | WorkerRunSpec | error | ✅ | `orchestration/wave/runners/agent_tool.go` |

## D7-S4-A01 PublishFlowEvent ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S4-A01-F01 | PublishEvent | F-BE | FlowEvent | — | ✅ | `orchestration/flow/hub.go` Publish |
| D7-S4-A01-F02 | EnqueueLeaderNotification | F-BE | session_id, event | — | ✅ | `orchestration/flow/hub.go` queue.Enqueue |
| D7-S4-A01-F03 | LinkTaskStatus | F-BE | FlowEvent | — | ✅ | `orchestration/flow/hub.go` linkTask |
| D7-S4-A01-F04 | ThrottleToolEmit | F-BE | FlowToolCall | bool | ✅ | `orchestration/flow/hub.go` allowToolEmit |

## D7-S4-A03 NotifyGateway ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S4-A03-F01 | EmitWorkerProgress | F-BE | FlowEvent | EngineEvent | ✅ | `orchestration/imsink/gateway.go` |

## D7-S5-A01 ClassifyIntent ⬜

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S5-A01-F01 | ClassifyByRules | F-BE | message | rules_hint | ⬜ | `d7/classifier.go` (planned) |
| D7-S5-A01-F02 | ClassifyByLLM | F-BE | message, rules_hint | llm_classification | ⬜ | `d7/classifier.go` (planned) |
| D7-S5-A01-F03 | MergeClassifications | F-BE | rules, llm | final_decision | ⬜ | `d7/classifier.go` (planned) |

## D7-S5-A02 SynthesizeTaskGraph ⬜

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S5-A02-F01 | DecomposeGoal | F-BE | goal | []sub_goal | ⬜ | `d7/decomposer.go` (planned) |
| D7-S5-A02-F02 | BuildDependencyGraph | F-BE | []sub_goal | []TaskNode | ⬜ | `d7/decomposer.go` (planned) |
| D7-S5-A02-F03 | ValidateTaskGraph | F-BE | []TaskNode | validation_report | ⬜ | `d7/decomposer.go` (planned) |

## D7-S5-A03 SelectExecutor ⬜

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S5-A03-F01 | MatchExecutorByTaskType | F-BE | task_type | executor_id | ⬜ | `d7/executor.go` (planned) |
| D7-S5-A03-F02 | CheckExecutorAvailability | F-BE | executor_id | available/busy | ⬜ | `d7/executor.go` (planned) |

---

## Canonical F 层定义（切法 A — 按用户价值流）

> 以下为 `devrix-d7-sa-refine` v1.0 Canonical F 层定义。补充 design.md §5 草案。

### D7-S2 Canonical F 层

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S2-A01-F01 | RouteByIntent | F-BE | IntentClassification | routing_decision | ⬜ | `d7/orchestrator.go` (planned) |
| D7-S2-A01-F02 | ExecuteFastPath | F-BE | message, session | events | ⬜ | `d7/fastpath.go` (planned) |
| D7-S2-A01-F03 | EnterOrchestration | F-BE | message, session | Plan | ⬜ | `d7/orchestrator.go` (planned) |
| D7-S2-A01-F04 | EmitSessionEvents | F-BE | event | — | ⬜ | `d7/orchestrator.go` (planned) |

### D7-S5 Canonical F 层

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S5-A01-F01 | ClassifyByRules | F-BE | message | rules_hint | ✅ | `orchestration/coordinator/classifier.go` |
| D7-S5-A01-F02 | ClassifyByLLM | F-BE | message, rules_hint | llm_classification | ✅ | `orchestration/coordinator/shadow_classifier.go` |
| D7-S5-A01-F03 | MergeClassifications | F-BE | rules, llm | final_decision | ✅ | `orchestration/coordinator/classifier.go` |

---

## Statistics

| Activities with F | Total F Points | Implemented | Planned |
|-------------------|----------------|-------------|---------|
| 15 + 2 Canonical | 44 + 7 Canonical | 30 + 3 | 14 + 4 |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始注册表（全 d7/ 路径） |
| 2.0.0 | 2026-06-14 | 对齐真实代码路径、实现状态、新增 wave 基础设施 F 点 |
| 3.0.0 | 2026-06-14 | Legacy 双轨建立（devrix-d7-sa-refine）；Canonical D7-S2/S5 F 层按 design.md §5 新增 |
