# D7 Orchestration Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.1.0
**Last Updated:** 2026-06-19
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d7-orchestration/a-registry.md`
**Domain SoT:** `d7-domain.md`

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

## D7-S1-A01 CreateWorkPlan ✅

> **v1.1 closure:** 写模型已迁入 `sessionorchestrator/workmodel.go` + `workmodel/plan_mode.go`。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A01-F01 | SynthesizePlan | F-BE | goal, context | []TaskNode | ✅ | `sessionorchestrator/workmodel.go` SynthesizePlan |
| D7-S1-A01-F02 | ValidatePlanDAG | F-BE | []TaskNode | valid/invalid | ✅ | `sessionorchestrator/workmodel.go` validateDAG + `decisionplanning/decomposer.go` validateGraph |
| D7-S1-A01-F03 | EstimateTaskScope | F-BE | goal, context | complexity_score | ✅ | `decisionplanning/decomposer.go` decomposeGoal |

## D7-S1-A03 QueryWorkPlan ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A03-F01 | BuildSessionSnapshot | F-BE | session_id | WorkPlanSnapshot | ✅ | `orchestration/executionflow/hub/hub.go` |
| D7-S1-A03-F02 | BuildTaskSnapshots | F-BE | session_id | []TaskSnapshot | ✅ | `orchestration/executionflow/hub/hub.go` taskSnapshots |
| D7-S1-A03-F03 | ApplyFlowEvent | F-BE | FlowEvent | — | ✅ | `orchestration/executionflow/workplan/service.go` Apply |

## D7-S1-A04/A05 PlanMode ✅

> **v1.1 closure:** 迁入 `workmodel/plan_mode.go` + `workmodel/plan_agent.go`。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A04-F01 | EnterPlanMode | F-BE | session_id, goal | — | ✅ | `workmodel/plan_mode.go` Enter |
| D7-S1-A04-F02 | GeneratePlan | F-BE | goal | PlanResult | ✅ | `workmodel/plan_agent.go` GeneratePlan |
| D7-S1-A05-F01 | ApprovePlan | F-BE | session_id | []Task | ✅ | `workmodel/plan_mode.go` Approve |
| D7-S1-A05-F02 | RejectPlan | F-BE | session_id | — | ✅ | `workmodel/plan_mode.go` Reject |

## D7-S2-A01 ProcessMessage ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S2-A01-F01 | RouteByIntent | F-BE | IntentClassification | routing_decision | ✅ | `sessionorchestrator/orchestrator.go` ProcessMessage switch |
| D7-S2-A01-F02 | ExecuteFastPath | F-BE | message, session | events | ✅ | `sessionorchestrator/fastpath.go` Run |
| D7-S2-A01-F03 | EnterOrchestration | F-BE | message, session | Plan | ✅ | `sessionorchestrator/orchestrator.go` orchestrate (delegates to D2/turn) |
| D7-S2-A01-F04 | EmitSessionEvents | F-BE | event | — | ✅ | `sessionorchestrator/orchestrator.go` + `EventPublisher` |
| D7-S2-A01-F05 | HandleInterrupt | F-BE | session_id, reason | — | ✅ | `sessionorchestrator/interrupt.go` |

## D7-S2-A02 EvaluateIntent ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S2-A02-F01 | ClassifyByRules | F-BE | message | rules_hint + confidence | ✅ | `decisionplanning/classifier.go` RuleClassifier.Classify |
| D7-S2-A02-F02 | ClassifyByLLM | F-BE | message, rules_hint | llm_classification | ✅ | `decisionplanning/classifier_fallback.go`（合并 LLM-first 路径） |

## D7-S2-A03 HandleInterrupt 🔶

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S2-A03-F01 | CancelActiveProcess | F-BE | session_id | — | 🔶 | `communication/gateway/gateway.go` StopProcess |
| D7-S2-A03-F02 | CancelWaveWorkers | F-BE | session_id | — | ✅ | `orchestration/wavescheduler/scheduler.go` CancelAll |

## D7-S3-A01 ScheduleWave ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S3-A01-F01 | StartWave | F-BE | session_id, task_graph | — | ✅ | `orchestration/wavescheduler/scheduler.go` Start |
| D7-S3-A01-F02 | DispatchWorker | F-BE | task_node, slot | — | ✅ | `orchestration/wavescheduler/scheduler.go` dispatchOne |
| D7-S3-A01-F03 | WaitForCompletion | F-BE | session_id | []Artifact | ✅ | `orchestration/wavescheduler/scheduler.go` WaitForCompletion |
| D7-S3-A01-F04 | ContinuousRedispatch | F-BE | slot_release | — | ✅ | `orchestration/wavescheduler/scheduler.go` dispatchLoop |
| D7-S3-A01-F05 | CancelWorker | F-BE | task_id | — | ✅ | `orchestration/wavescheduler/scheduler.go` CancelWorker |

## D7-S3-A02 ResolveWorkerContext ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S3-A02-F01 | ResolveFreshContext | F-BE | task_node | ResolvedContext | ✅ | `orchestration/wavescheduler/context.go` resolveFresh |
| D7-S3-A02-F02 | ResolveUpstreamContext | F-BE | task_node, artifacts | ResolvedContext | ✅ | `orchestration/wavescheduler/context.go` resolveUpstream |
| D7-S3-A02-F03 | ResolveResumeContext | F-BE | task_node, sidechain | ResolvedContext | ✅ | `orchestration/wavescheduler/context.go` resolveResume |

## D7-S3-A03 GuardConflict ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S3-A03-F01 | CheckConflict | F-BE | candidate, running | allowed/blocked | ✅ | `orchestration/wavescheduler/conflict.go` Allow |
| D7-S3-A03-F02 | RegisterTaskGuard | F-BE | task_node, slot | — | ✅ | `orchestration/wavescheduler/conflict.go` Register |
| D7-S3-A03-F03 | CheckFileScopeOverlap | F-BE | file_scope[] | bool | ✅ | `orchestration/wavescheduler/conflict.go` pathsOverlap |

## D7-S3 基础设施 ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S3-F01 | AcquireSlot | F-BE | worker_type | SlotID | ✅ | `orchestration/wavescheduler/pool.go` Acquire |
| D7-S3-F02 | ReleaseSlot | F-BE | slot_id | — | ✅ | `orchestration/wavescheduler/pool.go` Release |
| D7-S3-F03 | ReadyNodes | F-BE | — | []TaskNode | ✅ | `orchestration/wavescheduler/taskgraph.go` ReadyNodes |
| D7-S3-F04 | StoreArtifact | F-BE | artifact | — | ✅ | `orchestration/wavescheduler/artifact.go` Put |
| D7-S3-F05 | RunSubAgent | F-BE | WorkerRunSpec | error | ✅ | `orchestration/wavescheduler/runners/subagent.go` |
| D7-S3-F06 | RunAgentTool | F-BE | WorkerRunSpec | error | ✅ | `orchestration/wavescheduler/runners/agent_tool.go` |

## D7-S4-A01 PublishFlowEvent ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S4-A01-F01 | PublishEvent | F-BE | FlowEvent | — | ✅ | `orchestration/executionflow/hub/hub.go` Publish |
| D7-S4-A01-F02 | EnqueueLeaderNotification | F-BE | session_id, event | — | ✅ | `orchestration/executionflow/hub/hub.go` queue.Enqueue |
| D7-S4-A01-F03 | LinkTaskStatus | F-BE | FlowEvent | — | ✅ | `orchestration/executionflow/hub/hub.go` linkTask |
| D7-S4-A01-F04 | ThrottleToolEmit | F-BE | FlowToolCall | bool | ✅ | `orchestration/executionflow/hub/hub.go` allowToolEmit |

## D7-S4-A03 NotifyGateway ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S4-A03-F01 | EmitWorkerProgress | F-BE | FlowEvent | EngineEvent | ✅ | `orchestration/executionflow/imsink/gateway.go` |

## D7-S5-A01 ClassifyIntent ✅

> **v1.1 closure:** LLM-first 路径通过 `decisionplanning/classifier_fallback.go` 实现；rule+LLM merge 在 `Classify` 调用链中。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S5-A01-F01 | ClassifyByRules | F-BE | message | rules_hint | ✅ | `decisionplanning/classifier.go` RuleClassifier.Classify |
| D7-S5-A01-F02 | ClassifyByLLM | F-BE | message, rules_hint | llm_classification | ✅ | `decisionplanning/classifier_fallback.go` LLMClassifier.Classify |
| D7-S5-A01-F03 | MergeClassifications | F-BE | rules, llm | final_decision | ✅ | `decisionplanning/classifier.go` + `classifier_fallback.go` Merge |

## D7-S5-A02 SynthesizeTaskGraph ✅

> **v1.1 closure:** 规则版 + LLM 版（`SetLLMDecomposer`）双路径实装。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S5-A02-F01 | DecomposeGoal | F-BE | goal | []sub_goal | ✅ | `decisionplanning/decomposer.go` decomposeGoal |
| D7-S5-A02-F02 | BuildDependencyGraph | F-BE | []sub_goal | []TaskNode | ✅ | `decisionplanning/decomposer.go` SynthesizeTaskGraph |
| D7-S5-A02-F03 | ValidateTaskGraph | F-BE | []TaskNode | validation_report | ✅ | `decisionplanning/decomposer.go` validateGraph |

## D7-S5-A03 SelectExecutor ✅

> **v1.1 closure:** 规则版 + 黑白名单 worker_type 路由已实装（Phase K）。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S5-A03-F01 | MatchExecutorByTaskType | F-BE | task_type | executor_id | ✅ | `decisionplanning/executor.go` SelectExecutor |
| D7-S5-A03-F02 | CheckExecutorAvailability | F-BE | executor_id | available/busy | ✅ | `decisionplanning/executor.go` CheckAvailability |

---

## Canonical F 层定义（切法 A — 按用户价值流）

> 以下为 `devrix-d7-sa-refine` v1.0 Canonical F 层定义。补充 design.md §5 草案。

### D7-S2 Canonical F 层

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S2-A01-F01 | RouteByIntent | F-BE | IntentClassification | routing_decision | ✅ | `sessionorchestrator/orchestrator.go` ProcessMessage switch |
| D7-S2-A01-F02 | ExecuteFastPath | F-BE | message, session | events | ✅ | `sessionorchestrator/fastpath.go` Run |
| D7-S2-A01-F03 | EnterOrchestration | F-BE | message, session | Plan | ✅ | `sessionorchestrator/orchestrator.go` orchestrate |
| D7-S2-A01-F04 | EmitSessionEvents | F-BE | event | — | ✅ | `sessionorchestrator/orchestrator.go` + `EventPublisher` |

### D7-S5 Canonical F 层

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S5-A01-F01 | ClassifyByRules | F-BE | message | rules_hint | ✅ | `orchestration/decisionplanning/classifier.go` RuleClassifier.Classify |
| D7-S5-A01-F02 | ClassifyByLLM | F-BE | message, rules_hint | llm_classification | ✅ | `orchestration/decisionplanning/classifier_fallback.go` LLMClassifier.Classify |
| D7-S5-A01-F03 | MergeClassifications | F-BE | rules, llm | final_decision | ✅ | `orchestration/decisionplanning/classifier_fallback.go` Merge |

---

## Statistics

| Activities with F | Total F Points | Implemented | Planned |
|-------------------|----------------|-------------|---------|
| 15 + 2 Canonical | 44 + 7 Canonical | 44 + 7 | 0 |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始注册表（全 d7/ 路径） |
| 2.0.0 | 2026-06-14 | 对齐真实代码路径、实现状态、新增 wave 基础设施 F 点 |
| 3.0.0 | 2026-06-14 | Legacy 双轨建立（devrix-d7-sa-refine）；Canonical D7-S2/S5 F 层按 design.md §5 新增 |
| 3.0.1 | 2026-06-15 | **v1.1 closure 同步**：D7-S1-A01/S1-A04/A05 F 层 ✅；D7-S2-A01/A02 F 层 ✅；D7-S5-A01-F02 路径修正（classifier_fallback.go 而非 shadow_classifier.go）；D7-S5-A02/A03 F 层全 ✅；Canonical D7-S2 F 层全 ✅。统计 44+7/44+7/0 |
