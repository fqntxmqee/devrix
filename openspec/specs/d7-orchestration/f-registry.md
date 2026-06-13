# D7 Orchestration Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d7-orchestration/a-registry.md`

---

## Overview

D7 编排域 F 层功能点注册表。

---

## D7-S1-A01 CreateWorkPlan

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S1-A01-F01 | SynthesizePlan | F-BE | goal, context | []TaskNode | `d7/workmodel.go` |
| D7-S1-A01-F02 | ValidatePlanDAG | F-BE | []TaskNode | valid/invalid + reason | `d7/workmodel.go` |
| D7-S1-A01-F03 | EstimateTaskScope | F-BE | goal, context | complexity_score | `d7/workmodel.go` |

## D7-S1-A02 ManageTask

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S1-A02-F01 | DispatchTask | F-BE | task_id | execution_id | `d7/executor.go` |
| D7-S1-A02-F02 | TransitionTaskState | F-BE | task_id, from→to | Task | `d7/workmodel.go` |
| D7-S1-A02-F03 | CollectArtifacts | F-BE | task_id | []Artifact | `d7/workmodel.go` |
| D7-S1-A02-F04 | RegisterBackgroundTask | F-BE | session_id, agent_id | task_id | `d7/workmodel.go` |
| D7-S1-A02-F05 | WaitForTask | F-BE | task_id, timeout | terminal_state | `d7/workmodel.go` |
| D7-S1-A02-F06 | CancelTask | F-BE | task_id | — | `d7/workmodel.go` |

## D7-S1-A03 QueryWorkPlan

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S1-A03-F01 | BuildSessionSnapshot | F-BE | session_id | WorkPlanSnapshot | `d7/workmodel.go` |

## D7-S2-A01 ProcessMessage

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S2-A01-F01 | RouteByIntent | F-BE | IntentClassification | routing_decision | `d7/orchestrator.go` |
| D7-S2-A01-F02 | ExecuteFastPath | F-BE | message, session | events | `d7/fastpath.go` |
| D7-S2-A01-F03 | EnterOrchestration | F-BE | message, session | Plan | `d7/orchestrator.go` |
| D7-S2-A01-F04 | OrchestrateSessionLoop | F-BE | plan, ctx | — | `d7/orchestrator.go` |
| D7-S2-A01-F05 | EmitSessionEvents | F-BE | event | — | `d7/orchestrator.go` |

## D7-S2-A02 EvaluateIntent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S2-A02-F01 | ClassifyByRules | F-BE | message | rules_hint + confidence | `d7/classifier.go` |
| D7-S2-A02-F02 | ScoreComplexity | F-BE | message | complexity (simple / medium / complex) | `d7/classifier.go` |

## D7-S2-A03 HandleInterrupt

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S2-A03-F01 | CancelActiveTasks | F-BE | session_id | — | `d7/orchestrator.go` |
| D7-S2-A03-F02 | CleanupSessionState | F-BE | session_id | — | `d7/orchestrator.go` |

## D7-S3-A01 ScheduleWave

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S3-A01-F01 | StartWave | F-BE | session_id, task_graph | — | `d7/wave/scheduler.go` |
| D7-S3-A01-F02 | DispatchWorker | F-BE | task_node, slot | — | `d7/wave/scheduler.go` |
| D7-S3-A01-F03 | WaitForCompletion | F-BE | session_id | []Artifact | `d7/wave/scheduler.go` |

## D7-S3-A02 ResolveWorkerContext

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S3-A02-F01 | ResolveFreshContext | F-BE | task_node, session | *ResolvedContext | `d7/wave/context.go` |
| D7-S3-A02-F02 | ResolveUpstreamContext | F-BE | task_node, artifacts | *ResolvedContext | `d7/wave/context.go` |

## D7-S3-A03 GuardConflict

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S3-A03-F01 | CheckConflict | F-BE | candidate, running | allowed/blocked | `d7/wave/conflict.go` |
| D7-S3-A03-F02 | RegisterTaskGuard | F-BE | task_node | slot_id | `d7/wave/conflict.go` |

## D7-S4-A01 PublishFlowEvent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S4-A01-F01 | PublishEvent | F-BE | ctx, FlowEvent | — | `d7/flow/hub.go` |
| D7-S4-A01-F02 | EnqueueLeaderNotification | F-BE | session_id, event | — | `d7/flow/hub.go` |
| D7-S4-A01-F03 | EmitIMProgress | F-BE | session_id, event | — | `d7/flow/imsink.go` |

## D7-S4-A02 SnapshotWorkPlan

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S4-A02-F01 | BuildExecutionFlowSnapshot | F-BE | session_id | ExecutionFlowSnapshot | `d7/flow/hub.go` |
| D7-S4-A02-F02 | BuildTaskSnapshot | F-BE | task_id | TaskSnapshot | `d7/flow/hub.go` |

## D7-S4-A03 NotifyGateway

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S4-A03-F01 | SendCompleteEvent | F-BE | session_id, usage | — | `d7/flow/imsink.go` |
| D7-S4-A03-F02 | SendWorkerProgress | F-BE | session_id, FlowEvent | — | `d7/flow/imsink.go` |

## D7-S5-A01 ClassifyIntent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S5-A01-F01 | ClassifyByRules | F-BE | message | rules_hint | `d7/classifier.go` |
| D7-S5-A01-F02 | ClassifyByLLM | F-BE | message, rules_hint | llm_classification | `d7/classifier.go` |
| D7-S5-A01-F03 | MergeClassifications | F-BE | rules_hint, llm_result | final_decision + confidence | `d7/classifier.go` |

## D7-S5-A02 SynthesizeTaskGraph

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S5-A02-F01 | DecomposeGoal | F-BE | goal | []sub_goal | `d7/decomposer.go` |
| D7-S5-A02-F02 | BuildDependencyGraph | F-BE | []sub_goal | []TaskNode | `d7/decomposer.go` |
| D7-S5-A02-F03 | EstimateEffort | F-BE | task_node, history | effort_score | `d7/decomposer.go` |
| D7-S5-A02-F04 | ValidateTaskGraph | F-BE | []TaskNode | validation_report | `d7/decomposer.go` |

## D7-S5-A03 SelectExecutor

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D7-S5-A03-F01 | MatchExecutorByTaskType | F-BE | task_type, pool | best_executor + confidence | `d7/executor.go` |
| D7-S5-A03-F02 | CheckExecutorAvailability | F-BE | executor_id | available / busy | `d7/executor.go` |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 15 | 38 |
