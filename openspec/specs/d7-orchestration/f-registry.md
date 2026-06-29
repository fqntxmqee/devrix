# D7 Orchestration Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active · **6 S 精简 (v6.0.0 DM-20260626-001)**
**Version:** 5.1.0
**Last Updated:** 2026-06-29 (taskcontract-unification-pr-a DM-20260629-007: 新增 11 个 F: D7-S20/F01-F06 + D7-S21/F01-F03 + D7-S22/F01-F02 PR-A 通讯契约)
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d7-orchestration/a-registry.md`
**Domain SoT:** `d7-domain.md`

> **MUPS v4.3 5 节点管道（2026-06-25 落地）：** Canonical F 层扩展至 S1-S14 全部节点的 F 功能点登记，共 **75** 个 F 点全部 IMPLEMENTED（deprecated 2 + canonical 73）。具体 F 层见 §8-§14（Observe/Execute/Verify/Learn/跨域集成/Verify Auto-Close/EscapeEngine）。
>
> **v6.0.0 6 S 精简（DM-20260626-001）：** 14 S → 6 S + 1 横切后 F 层按新 S 重归类。具体 A/F 重映射见 `a-registry.md §v6.0.0 6 S 精简映射`。

---

## Overview

D7 编排域 F 层功能点注册表。代码位置标注**现行路径**；`(planned)` 表示目标 D7 包尚未创建。

**状态图例：** ✅ · 🔶 · ⬜

---

## D7-S1-A02 ManageTask ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A02-F01 | CreateTask | F-BE | subject, description | Task | ✅ | `workmodel/task_manager.go` |
| D7-S1-A02-F02 | UpdateTaskStatus | F-BE | task_id, status | Task | ✅ | `workmodel/task_manager.go` |
| D7-S1-A02-F03 | AddDependency | F-BE | task_id, blocked_by | — | ✅ | `workmodel/task_manager.go` |
| D7-S1-A02-F04 | ListReadyTasks | F-BE | session_id | []Task | ✅ | `workmodel/task_manager.go` |
| D7-S1-A02-F05 | PersistToDisk | F-BE | session_id | — | ✅ | `workmodel/workitem_store.go` |
| D7-S1-A02-F06 | SetOwner | F-BE | task_id, worker_id | — | ✅ | `workmodel/task_manager.go` |

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
| D7-S2-A01-F01 | RouteByIntent | F-BE | IntentClassification | routing_decision | ✅ | `sessionorchestrator/turn_orchestrator.go` ProcessMessage switch |
| D7-S2-A01-F02 | ExecuteFastPath | F-BE | message, session | events | ✅ | `sessionorchestrator/fastpath.go` Run |
| D7-S2-A01-F03 | EnterOrchestration | F-BE | message, session | Plan | ✅ | `sessionorchestrator/turn_orchestrator.go` orchestrate (delegates to D2/turn) |
| D7-S2-A01-F04 | EmitSessionEvents | F-BE | event | — | ✅ | `sessionorchestrator/turn_orchestrator.go` + `EventPublisher` |
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
| D7-S3-A03-F01 | CheckConflict | F-BE | candidate, running | allowed/blocked | ✅ | `orchestration/wavescheduler/conflict.go` Allow（legacy，DM-20260622-001 A4 标记 deprecated） |
| D7-S3-A03-F02 | RegisterTaskGuard | F-BE | task_node, slot | — | ✅ | `orchestration/wavescheduler/conflict.go` Register（legacy，DM-20260622-001 A4 标记 deprecated） |
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

> **v1.1 closure:** LLM-first 路径合并入 `decisionplanning/classifier.go`；rule+LLM merge 在 `ClassifyWithPrior` 调用链中（DM-20260624-001 Phase 6 PR-F1 收口）。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S5-A01-F01 | ClassifyByRules | F-BE | message | rules_hint | ✅ | `decisionplanning/classifier.go` RuleClassifier.Classify |
| D7-S5-A01-F02 | ClassifyByLLM | F-BE | message, rules_hint | llm_classification | ✅ | `decisionplanning/classifier.go` RuleClassifier.ClassifyWithPrior (LLM-first 路径合并) |
| D7-S5-A01-F03 | MergeClassifications | F-BE | rules, llm | final_decision | ✅ | `decisionplanning/classifier.go` ClassifyWithPrior merge chain |

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

## D7-S6-A14 HardenMetricsAndConcurrency ✅ (DM-20260622-001)

> 5 P0/P1 fix 一揽子 F 层登记。横切 S2/S3，不属于纵向业务流。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S6-A14-F01** | **AllowAndRegisterAtomic** | **F-BE** | **candidate, slot, running** | **bool** | **✅** | **`orchestration/wavescheduler/conflict.go::AllowAndRegister` (atomic) — 替代原 `Allow` + `Register` 拆分，消除 TOCTOU 窗口** |
| **D7-S6-A14-F02** | **MarkWaveDoneRelease** | **F-BE** | **state** | **—** | **✅** | **`orchestration/wavescheduler/scheduler.go::markWaveDone` — wave terminal 时将 `state.cancels = nil` + `state.handles = make(map)`，防跨 wave 重入累积** |
| **D7-S6-A14-F03** | **EmitSelectDefault** | **F-BE** | **event** | **—** | **✅** | **`orchestration/sessionorchestrator/command_handler.go::emit` 闭包 — `out <- ev` 包 `select { case out <- ev: default: slog.Warn(...) }`，防 consumer stall 永久阻塞** |
| **D7-S6-A14-F04** | **IncMetricPlural** | **F-BE** | **field** | **—** | **✅** | **`orchestration/wavescheduler/scheduler.go::incMetric` switch case 复数化：`worker_panics` / `dispatch_loop_wakeups`（与 spec 一致）；调用方同步改 plural** |

---

## D7-S8-A15 ObserveQuantize ✅ (Phase 2 PR-A1 + PR-RF, DM-20260623-001)

> Observe 节点 F 层登记：4 类 Observation 量化 + UncertaintyReport 产出 + UncertaintyCoord 不确定性度量。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S8-A15-F01** | **ClassifyObsKind** | **F-BE** | **raw signal** | **ObsKind (fact/anomaly/signal/user)** | **✅** | **`orchestration/observe/observation.go::ClassifyObsKind`** |
| **D7-S8-A15-F02** | **ScoreObsStrength** | **F-BE** | **observation + evidence** | **strength (★-★★★★)** | **✅** | **`orchestration/observe/observation.go::ScoreStrength`** |
| **D7-S8-A15-F03** | **DetectAnomaly** | **F-BE** | **[]observation** | **[]Anomaly** | **✅** | **`orchestration/observe/anomaly.go::Detect`**（PR-A1）|
| **D7-S8-A15-F04** | **QuantizeIntent** | **F-BE** | **[]observation + anomalies** | **IntentPayload** | **✅** | **`orchestration/observe/intent_quantizer.go::Quantize`**（PR-A1 + WithPrior 变体 Phase 6）|
| **D7-S8-A15-F05** | **BuildUncertaintyCoord** | **F-BE** | **[]observation** | **UncertaintyCoord** | **✅** | **`orchestration/observe/uncertainty_coord.go::Build`**（PR-A1）|
| **D7-S8-A15-F06** | **BuildUncertaintyReport** | **F-BE** | **observations, coord, anomalies, intent** | **UncertaintyReport** | **✅** | **`orchestration/observe/uncertainty_report.go::Build`**（PR-A1）|

---

## D7-S9-A25/A26 Execute ✅ (Phase 3 PR-C1 + PR-C2, DM-20260625-001)

> Execute 节点 F 层登记：4 类 Artifact 数据契约 + 4 Channel 路由 + C2/W8 1:1 映射。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S9-A25-F01** | **BuildArtifact** | **F-BE** | **Plan, ExecutionResult** | **Artifact (4 子类多态)** | **✅** | **`orchestration/execute/artifact.go::Build`**（PR-C1，跨域类型上提 `shared/types.Artifact`）|
| **D7-S9-A25-F02** | **ResolveArtifactKind** | **F-BE** | **Plan.Step.Kind** | **ArtifactKind (state_change/response/probe/experiment)** | **✅** | **`orchestration/execute/artifact.go::ResolveKind`** |
| **D7-S9-A25-F03** | **ExtractEvidence** | **F-BE** | **ExecutionResult** | **Evidence (与 Plan.FailureCriteria 对齐)** | **✅** | **`orchestration/execute/evidence.go::Extract`** |
| **D7-S9-A26-F01** | **RouteChannelKind** | **F-BE** | **Plan.Step** | **ChannelKind (sync/async/probe/explore)** | **✅** | **`orchestration/execute/channel_router.go::RouteChannel`**（PR-C2）|
| **D7-S9-A26-F02** | **DispatchSync** | **F-BE** | **Plan.Step** | **ExecutionResult (synchronous)** | **✅** | **`orchestration/execute/channel_sync.go::Dispatch`** |
| **D7-S9-A26-F03** | **DispatchAsync** | **F-BE** | **Plan.Step** | **<-chan ExecutionResult** | **✅** | **`orchestration/execute/channel_async.go::Dispatch`**（3 retry + queue）|
| **D7-S9-A26-F04** | **DispatchProbe** | **F-BE** | **Plan.Step** | **ExecutionResult (probe)** | **✅** | **`orchestration/execute/channel_probe.go::Dispatch`**（2 retry + backoff）|
| **D7-S9-A26-F05** | **DispatchExplore** | **F-BE** | **Plan.Step** | **ExecutionResult (explore)** | **✅** | **`orchestration/execute/channel_explore.go::Dispatch`**（1 retry best-effort）|

---

## D7-S10-A32..A35 Verify ✅ (Phase 4, DM-20260623-002)

> Verify 节点 F 层登记：4 态 Verdict + 14 ExitReason + Evidence + SystemAnomaly。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S10-A32-F01** | **ExtractVerdict** | **F-BE** | **Artifact + Plan** | **VerdictKind (compliance/timeliness/root_cause/statistical)** | **✅** | **`orchestration/verify/verifier.go::extractVerdict`** |
| **D7-S10-A32-F02** | **AggregateVerdicts** | **F-BE** | **[]Verdict** | **Verdict (聚合)** | **✅** | **`orchestration/verify/verdict.go::AggregateVerdicts`** |
| **D7-S10-A32-F03** | **VerifyWithRetry** | **F-BE** | **Artifact + Plan** | **Verdict (3 次重试兜底)** | **✅** | **`orchestration/verify/verifier.go::VerifyWithRetry`** |
| **D7-S10-A33-F01** | **MapVerdictToExitReason** | **F-BE** | **Verdict + Plan.ObservationStrength** | **ExitReason (14 态)** | **✅** | **`orchestration/verify/exit_reason.go::MapToExitReason`** |
| **D7-S10-A33-F02** | **IsDeterministicReason** | **F-BE** | **ExitReason** | **bool (8 deterministic vs 6 verify-driven)** | **✅** | **`orchestration/verify/exit_reason.go::IsDeterministic`** |
| **D7-S10-A34-F01** | **ExtractEvidence** | **F-BE** | **Artifact + Verdict** | **Evidence (含 SourceObservationIDs 追溯链)** | **✅** | **`orchestration/verify/evidence.go::ExtractEvidence`** |
| **D7-S10-A34-F02** | **ValidateEvidenceCompleteness** | **F-BE** | **Evidence + Plan.FailureCriteria** | **bool** | **✅** | **`orchestration/verify/evidence.go::ValidateCompleteness`** |
| **D7-S10-A35-F01** | **DetectSystemAnomaly** | **F-BE** | **session history** | **SystemAnomaly?** | **✅** | **`orchestration/verify/system_anomaly.go::Detect`** |
| **D7-S10-A35-F02** | **ClassifyAnomalySeverity** | **F-BE** | **SystemAnomaly** | **Severity (warn/error/critical)** | **✅** | **`orchestration/verify/system_anomaly.go::ClassifySeverity`** |

---

## D7-S11-A36..A40 Learn ✅ (Phase 5, DM-20260623-003)

> Learn 节点 F 层登记：4 类 LearningAsset + ReputationEvidence Bayesian 更新 + AdaptivePrior + Memory 3 通道。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S11-A36-F01** | **BuildAssetContent** | **F-BE** | **Verdict + Plan + Observation (追溯链)** | **AssetContent** | **✅** | **`orchestration/learn/asset_builder.go::BuildContent`** |
| **D7-S11-A36-F02** | **ClassifyLearningClass** | **F-BE** | **Verdict.Kind** | **LearningClass (SOP/Protocol/Knowledge/Conclusion)** | **✅** | **`orchestration/learn/asset_builder.go::ClassifyClass`** |
| **D7-S11-A36-F03** | **AssignAssetTTL** | **F-BE** | **LearningClass** | **TTL (90d/180d/365d/30d)** | **✅** | **`orchestration/learn/asset_builder.go::AssignTTL`** |
| **D7-S11-A37-F01** | **BayesianUpdate** | **F-BE** | **prior Beta(α,β) + Verdict.Kind** | **posterior Beta(α',β')** | **✅** | **`orchestration/learn/reputation_store.go::BayesianUpdate`**（PR-v4.5 合并原 bayesian_update.go）|
| **D7-S11-A37-F02** | **StoreReputationEvidence** | **F-BE** | **SessionID + Verdict + Evidence** | **ReputationEvidence** | **✅** | **`orchestration/learn/reputation_store.go::Store`** |
| **D7-S11-A37-F03** | **LoadReputationHistory** | **F-BE** | **SessionID** | **[]ReputationEvidence** | **✅** | **`orchestration/learn/reputation_store.go::LoadHistory`** |
| **D7-S11-A38-F01** | **BuildAdaptivePrior** | **F-BE** | **[]ReputationEvidence** | **AdaptivePrior** | **✅** | **`orchestration/learn/adaptive_prior.go::Build`** |
| **D7-S11-A38-F02** | **DefaultDeveloperPrior** | **F-BE** | **—** | **AdaptivePrior Beta(5,3)** | **✅** | **`orchestration/learn/adaptive_prior.go::DefaultDeveloperPrior`** |
| **D7-S11-A38-F03** | **DefaultOperatorPrior** | **F-BE** | **—** | **AdaptivePrior Beta(8,1)** | **✅** | **`orchestration/learn/adaptive_prior.go::DefaultOperatorPrior`** |
| **D7-S11-A39-F01** | **PersistToSkillChannel** | **F-BE** | **LearningAsset** | **—** | **✅** | **`orchestration/learn/memory.go::PersistSkill`** |
| **D7-S11-A39-F02** | **PersistToFeedbackChannel** | **F-BE** | **LearningAsset** | **—** | **✅** | **`orchestration/learn/memory.go::PersistFeedback`** |
| **D7-S11-A39-F03** | **PersistToScheduledChannel** | **F-BE** | **LearningAsset** | **—** | **✅** | **`orchestration/learn/memory.go::PersistScheduled`** |
| **D7-S11-A40-F01** | **RunLearner** | **F-BE** | **Verdict + 追溯链** | **[]LearningAsset + ReputationEvidence** | **✅** | **`orchestration/learn/learner.go::Learn`** |
| **D7-S11-A40-F02** | **DispatchToMemoryChannel** | **F-BE** | **LearningAsset** | **ChannelKind (skill/feedback/scheduled)** | **✅** | **`orchestration/learn/learner.go::DispatchChannel`** |

---

## D7-S12-A41..A43 Observe-Learner 跨域闭环 ✅ (Phase 6, DM-20260624-001)

> Observe-Learner 跨域闭环 F 层登记：WithPrior 变体 + 3 层 fail-safe + E2E round-trip。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S12-A41-F01** | **BuildObserveRequestWithPrior** | **F-BE** | **SessionID + UserMessage + AdaptivePrior** | **ObserveRequest (含 prior)** | **✅** | **`orchestration/observe/observe_node.go::ObserveRequestWithPrior`**（Phase 6 WithPrior 变体）|
| **D7-S12-A41-F02** | **InjectPriorToIntentQuantizer** | **F-BE** | **ObserveRequest** | **IntentPayload (prior-weighted)** | **✅** | **`orchestration/observe/intent_quantizer.go::QuantizeWithPrior`** |
| **D7-S12-A42-F01** | **ResolvePriorLayer1** | **F-BE** | **SessionID** | **AdaptivePrior (历史)** | **✅** | **`orchestration/sessionorchestrator/observe_request.go::resolvePrior` — Layer 1: 从 ReputationStore Load** |
| **D7-S12-A42-F02** | **ResolvePriorLayer2** | **F-BE** | **AdaptivePrior?** | **AdaptivePrior** | **✅** | **`orchestration/sessionorchestrator/observe_request.go::resolvePrior` — Layer 2: nil → DefaultDeveloperPrior Beta(5,3)** |
| **D7-S12-A42-F03** | **ResolvePriorLayer3** | **F-BE** | **AdaptivePrior?** | **AdaptivePrior** | **✅** | **`orchestration/sessionorchestrator/observe_request.go::resolvePrior` — Layer 3: 仍 nil → Beta(1,1) uniform** |
| **D7-S12-A43-F01** | **E2ECloseLP1RoundTrip** | **F-BE** | **session_id (跨多轮)** | **ReputationEvidence round-trip** | **✅** | **`tests/integration/d7/e2e_lp1_closure_test.go`**（Phase 6 E2E）|

---

## D7-S13-A47..A49 Verify Auto-Close ✅ (Phase 7, DM-20260625-001)

> Verify 自动闭环 F 层登记：processAutoClose + TrackMode + sessionSpan 6 prior attributes。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S13-A47-F01** | **ProcessAutoClose** | **F-BE** | **ProcessRequest + session state** | **Verdict + ExitReason** | **✅** | **`orchestration/verify/auto_close.go::ProcessAutoClose`**（3 层 fail-safe）|
| **D7-S13-A47-F02** | **SynthesizeVerdict** | **F-BE** | **session state + Plan** | **Verdict (default compliance)** | **✅** | **`orchestration/verify/auto_close.go::SynthesizeVerdict`** |
| **D7-S13-A48-F01** | **ResolveTrackMode** | **F-BE** | **session state** | **TrackMode (full/partial/track-only)** | **✅** | **`orchestration/verify/track_mode.go::ResolveTrackMode`** |
| **D7-S13-A48-F02** | **ShouldAutoClose** | **F-BE** | **session state + last activity** | **bool** | **✅** | **`orchestration/verify/auto_close.go::ShouldAutoClose`**（4 条触发规则）|
| **D7-S13-A49-F01** | **EmitSessionSpanPrior** | **F-BE** | **session + prior** | **sessionSpan (6 prior attributes)** | **✅** | **`orchestration/sessionorchestrator/session_span.go::EmitPrior`**（prior.adaptive_kind / beta_alpha / beta_beta / evidence_count / cycle_count / last_update）|

---

## D7-S14-A50..A52 EscapeEngine + ResumeSession ✅ (Phase v5, DM-20260625-003 + DM-20260625-004)

> EscapeEngine + ResumeSession F 层登记：5 层 CircuitBreaker + 3 决策路由 + sessionSpan 3 resume attributes。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S14-A50-F01** | **TriggerEscape** | **F-BE** | **session_id + signal** | **EscapeTrigger** | **✅** | **`orchestration/escape/engine.go::Trigger`** |
| **D7-S14-A50-F02** | **ResolveCircuitLevel** | **F-BE** | **signal + history** | **CircuitLevel (L0..L5)** | **✅** | **`orchestration/escape/circuit_breaker.go::ResolveLevel`** |
| **D7-S14-A50-F03** | **ApplyCircuitBreaker** | **F-BE** | **CircuitLevel + plan** | **plan (modified)** | **✅** | **`orchestration/escape/circuit_breaker.go::Apply`** |
| **D7-S14-A50-F04** | **LiftEscape** | **F-BE** | **session_id** | **—** | **✅** | **`orchestration/escape/engine.go::Lift`** |
| **D7-S14-A51-F01** | **ApplyResumeSessionLayer1** | **F-BE** | **session_id + user_choice** | **ResumeDecision** | **✅** | **`orchestration/sessionorchestrator/resume.go::applyResumeSession` Layer 1** |
| **D7-S14-A51-F02** | **ApplyResumeSessionLayer2** | **F-BE** | **session_id + prior state** | **ResumeDecision** | **✅** | **`orchestration/sessionorchestrator/resume.go::applyResumeSession` Layer 2 (fall through 兜底)** |
| **D7-S14-A51-F03** | **ApplyResumeSessionLayer3** | **F-BE** | **session_id** | **AbortDecision (with audit)** | **✅** | **`orchestration/sessionorchestrator/resume.go::applyResumeSession` Layer 3 (AbortWithAudit)** |
| **D7-S14-A51-F04** | **RouteResumeDecision** | **F-BE** | **user_choice** | **DecisionKind (A fall through / B user_accept→ForceExit / C user_cancel→Abort)** | **✅** | **`orchestration/sessionorchestrator/resume.go::routeResumeDecision`** |
| **D7-S14-A52-F01** | **EmitSessionSpanResume** | **F-BE** | **session + decision** | **sessionSpan (3 resume attributes)** | **✅** | **`orchestration/sessionorchestrator/session_span.go::EmitResume`**（resume.decision / circuit_level / user_choice）|

---

## D7-S18: Pessimistic Commit + Rule-based Fallback F 层 ✅ (PR-B, DM-20260629-008)

> **L3 防御运行时层:** PR-B 落地 PessimisticCommitGuard interface + 5 类触发条件 (resource_exhausted / cb_l1 / indeterminate_3x / empty_evidence / manual_abort) + 3 FallbackPolicy 路径 (Pessimistic / RuleBased / Abort) + 4 候选规则 (most_tests_passed / compiled_clean / min_cost / min_uncertainty, default min_uncertainty). Feature Flag `D7_PESSIMISTIC_COMMIT_ENABLED` 默认 disabled, 0 行为变更.

### D7-S18-A11 Pessimistic Commit

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S18-A11-F01** | **EvaluatePessimistic** | **F-BE** | **spec, report, budget** | **(ok bool, blockedReason string, err error)** | **✅** | **`orchestration/interfaces/contracts.go::PessimisticCommitGuard.Evaluate` + `orchestration/escape/fallback.go::DefaultPessimisticCommitGuard.Evaluate`** (5 类触发检测) |
| **D7-S18-A11-F02** | **ResolveFallback** | **F-BE** | **report** | **(policy FallbackPolicy, ruleName string)** | **✅** | **`orchestration/escape/fallback.go::DefaultPessimisticCommitGuard.ResolveFallback`** (3 路径: Pessimistic/RuleBased/Abort, Blockage.Source=policy_override 解析) |
| **D7-S18-A11-F03** | **BuildMVPArtifact** | **F-BE** | **report, blockedReason** | **MVPArtifact** | **✅** | **`orchestration/escape/fallback.go::DefaultPessimisticCommitGuard.BuildMVPArtifact`** (Output/RiskWarnings/Trigger/ChainHash FNV-1a) |
| **D7-S18-A11-F04** | **NotifyPessimisticHook** | **F-BE** | **engine, spec, report** | **(*TaskReport, error)** | **✅** | **`orchestration/escape/engine.go::EscapeEngine.NotifyPessimistic`** (5 层 fail-safe: nil guard / nil report / Evaluate error → fall-open / blocked → MVPArtifact 注入) |
| **D7-S18-A11-F05** | **CheckResourceExhausted** | **F-BE** | **used, budget, reserve** | **bool** | **✅** | **`orchestration/interfaces/convergence_budget.go::RemainingBelowReserve`** (资源耗尽触发检测, reserve 默认 10% budget) |

### D7-S18-A12 Rule-based Fallback

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S18-A12-F01** | **ParseFallbackRuleName** | **F-BE** | **string** | **(name, recognized)** | **✅** | **`orchestration/interfaces/fallback_policy.go::ParseFallbackRuleName`** (空 → 默认 / 4 候选 round-trip / 未知 → 默认 + recognized=false) |
| **D7-S18-A12-F02** | **ValidateFallbackPolicy** | **F-BE** | **FallbackPolicy** | **bool** | **✅** | **`orchestration/interfaces/fallback_policy.go::FallbackPolicy.Valid + ValidNonLegacy`** (3 态 / 2 non-legacy) |

---

## D7-S20-A01 TaskSpec 下行契约 F 层 ✅ (PR-A, DM-20260629-007)

> **物理位置：** `orchestration/interfaces/task_spec.go`。pure types 原则（0 import D7 子包）。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S20-A01-F01** | **NewTaskSpec** | **F-BE** | **session_id, plan, channel, work_item, trace_id** | **TaskSpec** | **✅** | **`orchestration/interfaces/task_spec.go::NewTaskSpec`（fail-fast session_id + TraceID format `ts_<8 hex>`）** |
| **D7-S20-A01-F02** | **ValidateTaskSpec** | **F-BE** | **TaskSpec** | **error** | **✅** | **`orchestration/interfaces/task_spec.go::Validate`**（happy path + empty session_id + channel unknown + trace_id 格式校验）|
| **D7-S20-A01-F03** | **WithTaskSpecFields** | **F-BE** | **TaskSpec, Plan/Channel/WorkItem** | **TaskSpec (immutable)** | **✅** | **`orchestration/interfaces/task_spec.go::WithPlan + WithChannel + WithWorkItem`（3 不可变 builder，浅拷贝 `c := *s` 返回新副本）**|

## D7-S20-A02 TaskReport 上行契约 F 层 ✅ (PR-A, DM-20260629-007)

> **物理位置：** `orchestration/interfaces/task_report.go`。pure types 原则。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S20-A02-F01** | **NewTaskReport** | **F-BE** | **session_id, channel, verdict, trace_id** | **TaskReport** | **✅** | **`orchestration/interfaces/task_report.go::NewTaskSpec`（fail-fast session_id + trace_id 格式）** |
| **D7-S20-A02-F02** | **ValidateTaskReport** | **F-BE** | **TaskReport** | **error** | **✅** | **`orchestration/interfaces/task_report.go::Validate`** |
| **D7-S20-A02-F03** | **WithTaskReportFields** | **F-BE** | **TaskReport, Verdict/Resource/Blockage** | **TaskReport (immutable)** | **✅** | **`orchestration/interfaces/task_report.go::WithVerdict + WithResource + WithBlockage`（3 不可变 builder）** |
| **D7-S20-A02-F04** | **AppendDissent** | **F-BE** | **TaskReport, Dissent** | **TaskReport (immutable)** | **✅** | **`orchestration/interfaces/task_report.go::AppendDissent`（top-3 截断 + summary hash 懒计算）**|

## D7-S21-A01/A02/A03 字段语义 F 层 ✅ (PR-A, DM-20260629-007)

> **物理位置：** `orchestration/interfaces/task_report.go` + `task_spec.go` + 内部 helpers。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S21-A01-F01** | **HashDissentSummary** | **F-BE** | **summary string** | **hash 8 hex prefix** | **✅** | **`orchestration/interfaces/task_report.go::hashSummary`（fnv64a → fmt.Sprintf("%08x", h)[:8]）** |
| **D7-S21-A01-F02** | **TopNTuncateDissent** | **F-BE** | **[]Dissent, n int** | **[]Dissent (≤ n)** | **✅** | **`orchestration/interfaces/task_report.go::AppendDissent` 内嵌（默认 n=3，silent truncate 不警告）** |
| **D7-S21-A02-F01** | **ClassifyBlockageKind** | **F-BE** | **failure error + Plan context** | **BlockageKind (permission/resource/contract)** | **✅** | **`orchestration/interfaces/task_spec.go::WithBlockage` 内嵌分类器（403/IAM deny → permission；OOM/disk/quota → resource；其他 → contract）** |
| **D7-S21-A03-F01** | **ExtractResource** | **F-BE** | **ExecutionResult + 上下文 (token accounting + Start/End time + ReAct iter count)** | **Resource (token/time/step)** | **✅** | **`orchestration/interfaces/task_report.go::WithResource` 内嵌抽取器（直接读 execution metadata）** |

## D7-S22 TaskContract PR-B 通讯契约预留位 ✅ (DESIGN ONLY, DM-20260629-006)

> **物理位置：** `orchestration/interfaces/contracts.go`（PLANNED，留给 PR-B Pessimistic Commit / PR-C CoW VersionChain）
>
> **PR-A 不实现**，仅登记 F 层接口签名作为契约锚点；PR-B + PR-C 在不破坏 `interfaces` 包 pure types 原则下扩展。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S22-F01** | **PessimisticCommitGuard** | **F-BE** | **TaskSpec + TaskReport (diff)** | **ok/blocked** | **⬜ PLANNED (PR-B)** | **`orchestration/interfaces/contracts.go::PessimisticCommitGuard`（防 false success commit，先 mark pessimistic state 再 verify）** |
| **D7-S22-F02** | **CoWVersionChain** | **F-BE** | **TaskSpec vN** | **TaskSpec vN+1 (with prev_version_id)** | **⬜ PLANNED (PR-C)** | **`orchestration/interfaces/contracts.go::CoWVersionChain`（每次 spec 变 → 生成新 version_id，引用前驱用于反向追溯）** |

---

## Statistics

| Activities with F | Total F Points | Implemented | Planned |
|-------------------|----------------|-------------|---------|
| **deprecated 2 + canonical 73 + v7.0 TaskContract 11 = 86**（v6.0.0 6 S 精简 + v7.0 PR-A 增量, DM-20260629-007） | **86** | **84** (canonical 73 + TaskContract 11 - 2 PLANNED S22) | **2 (D7-S22 PR-B/C)** |

---

## ValueFlow Semantic 映射（v6.0.0 + v2.5.1 同步）

> **ValueFlow Alias per S**（定义见 `d7-domain.md` §North Star）：
> - **S1 WorkModel** = Multi-Step Task Coordination
> - **S2 SessionOrchestrator** = Turn-Based Conversation
> - **S3 WaveScheduler** = Parallel Worktree Execution
> - **S4 ExecutionFlow + Verify** = Trustworthy Conclusion Delivery
> - **S5 DecisionPlanning + Observe** = Intent + Uncertainty Quantization
> - **S6 MUPS Pipeline** = Learn from Outcome
> - **Cross-cutting Hardening** = (Discipline Keeper)

| F ID | Name | ValueFlow Semantic（用户动作语义） |
|------|------|-----------------------------------|
| **D7-S1-A02-F01** | CreateTask | 用户/系统创建 WorkItem 节点 |
| D7-S1-A02-F02 | ValidateTaskTransition | 验证状态机转移合法性 |
| D7-S1-A02-F03 | ComputeBlockedBy | 计算任务依赖阻塞 |
| D7-S1-A02-F04 | ListReadyTasks | 列出可执行任务 |
| D7-S1-A02-F05 | PersistToDiskStore | 持久化到磁盘 v2 schema |
| D7-S1-A02-F06 | LinkTaskToFlow | 关联任务到 FlowEvent |
| D7-S1-A01-F02 | ValidateGraph | 校验 DAG 合法性 |
| **D7-S2-A01-F01** | BuildObserveRequest | 构建 Observe 请求（含 prior） |
| **D7-S2-A01-F02** | FastPathProxy | 快速路径代理（< 2ms P99） |
| **D7-S2-A01-F03** | OrchestratePath | 编排路径（5 节点管道入口） |
| **D7-S2-A01-F04** | EmitFlowThrottled | 节流广播 FlowEvent |
| **D7-S2-A02-F01** | RuleClassifyIntent | 规则意图分类（P99 < 1ms） |
| **D7-S2-A02-F02** | LLMClassifyIntent | LLM 意图分类（fallback） |
| **D7-S2-A03-F01** | WaveCancel | 取消运行中的 Wave |
| **D7-S2-A03-F02** | D4Cancel | 取消 D4 Worker |
| **D7-S2-A03-F03** | ProcessCancel | 取消 Process 主循环 |
| **D7-S2-A03-F06** | DeprecatedMarker | 标记 deprecated 函数（迁移期保留） |
| **D7-S3-A01-F01** | BuildTaskGraph | 构建任务 DAG |
| **D7-S3-A01-F02** | DispatchTask | 派发单个任务 |
| **D7-S3-A01-F03** | CollectArtifacts | 收集产物 |
| **D7-S3-A01-F04** | SlotRelease | 释放 worker 槽位 |
| **D7-S3-A01-F05** | CancelWorker | 取消 worker |
| **D7-S3-A02-F01** | BuildContextPolicy | 构建上下文策略 |
| **D7-S3-A02-F02** | ResolveContext | 解析 worker 上下文 |
| **D7-S3-A03-F01** | AllowAndRegister | 原子检查冲突 + 注册（防 TOCTOU） |
| **D7-S3-A03-F02** | GuardConflict | 冲突守卫（已被 F01 原子化） |
| **D7-S3-A03-F03** | FileScopeIntersect | 文件作用域冲突检查 |
| **D7-S4-A01-F01** | PublishFlowEventInternal | 内部广播 FlowEvent |
| **D7-S4-A01-F02** | EnqueueProgress | 入队 worker 进度 |
| **D7-S4-A01-F04** | ThrottleFlowToolCall | 节流工具调用事件 |
| **D7-S4-A03-F01** | EmitWorkerProgress | 发射 worker 进度到 IM |
| **D7-S5-A01-F01** | ClassifyIntent | 分类意图（Command-first） |
| **D7-S5-A01-F02** | MergeLLMResult | 合并 LLM 分类结果 |
| **D7-S5-A01-F03** | ShadowClassify | 影子分类（观测用） |
| **D7-S5-A02-F01** | ValidateTaskGraph | 校验任务图 |
| **D7-S5-A02-F02** | PlanLLMFallback | Plan 生成的 LLM fallback |
| **D7-S5-A03-F01** | SelectD2OrD4 | 选 D2 SubQuery 或 D4 Worker |
| **D7-S5-A04-F01** | BuildPlanResult | 构建 PlanAgent 结果 |
| **D7-S5-A05-F01** | TailShadowLog | tail shadow 日志 |
| **D7-S6-A14-F01** | AllowAndRegisterAtomic | 原子化 AllowAndRegister |
| **D7-S6-A14-F02** | MarkWaveDoneRelease | markWaveDone 释放 state.cancels/handles |
| **D7-S6-A14-F03** | EmitSelectDefault | emit 加 select-default 防 stall |
| **D7-S6-A14-F04** | IncMetricPlural | metric 复数命名对齐 |
| **D7-S8-A15-F01** | ClassifyObsKind | 4 类 Observation 分类 |
| **D7-S8-A15-F02** | ScoreObsStrength | 评分 Obs 强度 [0,1] |
| **D7-S8-A15-F03** | DetectAnomaly | 检测异常信号 |
| **D7-S8-A15-F04** | QuantizeIntent | 量化意图 |
| **D7-S8-A15-F05** | BuildUncertaintyCoord | 构建 UncertaintyCoord |
| **D7-S8-A15-F06** | BuildUncertaintyReport | 构建 UncertaintyReport |
| **D7-S9-A25-F01** | BuildArtifact | 构建 4 类 Artifact |
| **D7-S9-A25-F02** | ResolveArtifactKind | 解析 ArtifactKind |
| **D7-S9-A25-F03** | ExtractArtifactEvidence | 提取 Artifact 证据 |
| **D7-S9-A25-F04** | AssignSideEffectStatus | 派生 SideEffectStatus |
| **D7-S9-A26-F01** | RouteChannelKind | 路由 4 Channel |
| **D7-S9-A26-F02** | DispatchCommit | 1-Step 同步派发 |
| **D7-S9-A26-F03** | DispatchProtocol | 顺序多步派发 |
| **D7-S9-A26-F04** | DispatchScenario | 5 并行探测派发 |
| **D7-S9-A26-F05** | DispatchExploration | 3 多 agent 探索派发 |
| **D7-S10-A32-F01** | ExtractVerdict | 提取 Verdict 4 态 |
| **D7-S10-A32-F02** | AggregateVerdicts | 聚合多个 Verdict |
| **D7-S10-A32-F03** | VerifyWithRetry | 验证 + 重试 |
| **D7-S10-A33-F01** | MapVerdictToExitReason | 映射到 14 ExitReason |
| **D7-S10-A33-F02** | IsDeterministicReason | 判定 deterministic 原因 |
| **D7-S10-A34-F01** | ExtractEvidence | 提取 Evidence |
| **D7-S10-A34-F02** | ValidateEvidenceCompleteness | 验证证据完整性 |
| **D7-S10-A35-F01** | DetectSystemAnomaly | 检测系统异常 |
| **D7-S10-A35-F02** | ClassifyAnomalySeverity | 分类异常严重度 |
| **D7-S11-A36-F01** | BuildAssetContent | 构建 LearningAsset 内容 |
| **D7-S11-A36-F02** | ClassifyLearningClass | 分类 5 类 LearningClass |
| **D7-S11-A36-F03** | AssignAssetTTL | 分配 TTL（90/180/365/30 天） |
| **D7-S11-A37-F01** | BayesianUpdate | Bayesian Beta 更新 |
| **D7-S11-A37-F02** | StoreReputationEvidence | 存信誉证据 |
| **D7-S11-A37-F03** | LoadReputationEvidence | 读信誉证据 |
| **D7-S11-A38-F01** | BuildAdaptivePrior | 构建 AdaptivePrior |
| **D7-S11-A38-F02** | DefaultDeveloperPrior | 默认 developer Beta(5,3) |
| **D7-S11-A38-F03** | DefaultOperatorPrior | 默认 operator Beta(8,1) |
| **D7-S11-A39-F01** | MemoryPersistSkill | 持久化 skill 通道 |
| **D7-S11-A39-F02** | MemoryPersistFeedback | 持久化 feedback 通道 |
| **D7-S11-A39-F03** | MemoryPersistScheduled | 持久化 scheduled 通道 |
| **D7-S11-A40-F01** | RunLearner | 跑学习者主循环 |
| **D7-S11-A40-F02** | DispatchToMemoryChannel | 派发到对应记忆通道 |
| **D7-S12-A41-F01** | BuildObserveRequestWithPrior | 构建带 prior 的 ObserveRequest |
| **D7-S12-A42-F01** | InjectPriorToIntentQuantizer | 注入 prior 到意图量化 |
| **D7-S12-A42-F02** | FailSafePriorFallback | prior nil → Default → uniform |
| **D7-S12-A42-F03** | FailSafeObserverFallback | observer nil → 跳过 prior |
| **D7-S12-A42-F04** | FailSafeChannelCancel | channel 取消时返回空 |
| **D7-S12-A43-F01** | E2ECloseLP1RoundTrip | E2E LP-1 闭环 round-trip |
| **D7-S13-A47-F01** | ProcessAutoClose | channel 关闭时自动收口 |
| **D7-S13-A47-F02** | SynthesizeVerdict | 合成 Verdict（complete/error/tombstone） |
| **D7-S13-A48-F01** | ResolveTrackMode | 解析 TrackMode 3-tier |
| **D7-S13-A48-F02** | ShouldAutoClose | 4 规则判定 auto-close |
| **D7-S13-A49-F01** | EmitSessionSpanPrior | emit 6 prior attributes |
| **D7-S14-A50-F01** | TriggerEscape | 触发 EscapeEngine |
| **D7-S14-A50-F02** | ResolveCircuitLevel | 解析 CircuitBreaker 5 层 |
| **D7-S14-A50-F03** | ApplyCircuitBreaker | 应用 CircuitBreaker |
| **D7-S14-A50-F04** | LiftEscape | 解除 Escape 状态 |
| **D7-S14-A51-F01** | ResumeSession3Layer | 3 层 fail-safe resume |
| **D7-S14-A51-F02** | RouteResumeDecision | 3 决策路由 (A/B/C) |
| **D7-S14-A51-F03** | ResumeErrorFailsafe | resume error 兜底 |
| **D7-S14-A52-F01** | EmitSessionSpanResume | emit 3 resume attributes |

> **ValueFlow Semantic 列**（75 F 全覆盖）：用户动作语义视角；与 6 S 博弈角色 + 4.3 MUPS 5 节点管道 三视角互补。

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始注册表（全 d7/ 路径） |
| 2.0.0 | 2026-06-14 | 对齐真实代码路径、实现状态、新增 wave 基础设施 F 点 |
| 3.0.0 | 2026-06-14 | Legacy 双轨建立（devrix-d7-sa-refine）；Canonical D7-S2/S5 F 层按 design.md §5 新增 |
| 3.0.1 | 2026-06-15 | **v1.1 closure 同步**：D7-S1-A01/S1-A04/A05 F 层 ✅；D7-S2-A01/A02 F 层 ✅；D7-S5-A01-F02 路径修正（classifier_fallback.go 而非 shadow_classifier.go）；D7-S5-A02/A03 F 层全 ✅；Canonical D7-S2 F 层全 ✅。统计 44+7/44+7/0 |
| **3.2.0** | **2026-06-22** | **DM-20260622-001 D7 Metrics & Concurrency Hardening**：新增 D7-S6-A14 横切 F 层 4 项（AllowAndRegisterAtomic + MarkWaveDoneRelease + EmitSelectDefault + IncMetricPlural）；D7-S3-A03-F01/F02 legacy 标记 deprecated（hot path 已切 AllowAndRegisterAtomic）。统计 48+7/48+7/0 |
| **4.0.0** | **2026-06-25** | **MUPS v4.3 5 节点管道 + v5 EscapeEngine F 层补全（DM-20260623-001/002/003 + DM-20260624-001 + DM-20260625-001/003/004）**：新增 7 段共 27 F 点。D7-S8-A15 6 F（ClassifyObsKind + ScoreObsStrength + DetectAnomaly + QuantizeIntent + BuildUncertaintyCoord + BuildUncertaintyReport）；D7-S9-A25/A26 8 F（BuildArtifact + ResolveArtifactKind + ExtractEvidence + RouteChannelKind + 4 Dispatch）；D7-S10-A32..A35 9 F（ExtractVerdict + AggregateVerdicts + VerifyWithRetry + MapVerdictToExitReason + IsDeterministicReason + ExtractEvidence + ValidateEvidenceCompleteness + DetectSystemAnomaly + ClassifyAnomalySeverity）；D7-S11-A36..A40 14 F（BuildAssetContent + ClassifyLearningClass + AssignAssetTTL + BayesianUpdate + Store/LoadReputationEvidence + BuildAdaptivePrior + DefaultDeveloper/OperatorPrior + 3 MemoryPersist + RunLearner + DispatchToMemoryChannel）；D7-S12-A41..A43 6 F（BuildObserveRequestWithPrior + InjectPriorToIntentQuantizer + 3 Layer fail-safe + E2ECloseLP1RoundTrip）；D7-S13-A47..A49 5 F（ProcessAutoClose + SynthesizeVerdict + ResolveTrackMode + ShouldAutoClose + EmitSessionSpanPrior）；D7-S14-A50..A52 9 F（TriggerEscape + ResolveCircuitLevel + ApplyCircuitBreaker + LiftEscape + 3 Layer resume + RouteResumeDecision + EmitSessionSpanResume）。统计 68+7/68+7/0 |
| **5.0.0** | **2026-06-26** | **6 S 精简（DM-20260626-001）**：14 S → **6 S + 1 横切**（State Authority / Mediator+Turn Leader+Error Recovery / Mechanism Designer / Costly Signaler+Certifier / Information Producer+Quantizer / Pipeline Coordinator+Memory Curator / 横切 Discipline Keeper）；F 层按新 S 重归类（F 总数 75 → 68，Legacy 41 + Canonical 27；S 编号变化不增减 F 点）；新增 Status 标注 `6 S 精简 (v6.0.0 DM-20260626-001)`；Statistics 表 Activities with F 加注 v6.0.0。具体 A/F 重映射见 `a-registry.md §v6.0.0 6 S 精简映射`。 |
| **5.1.0** | **2026-06-29** | **v7.0 TaskContract 统一 PR-A（DM-20260629-007）**：**(1) 新增 11 个 F**（D7-S20-A01/F01-F03 TaskSpec 下行契约 + D7-S20-A02/F01-F04 TaskReport 上行契约 + D7-S21-A01/F01-F02 Dissent + D7-S21-A02-F01 Blockage 分类 + D7-S21-A03-F01 Resource 抽取）；(2) **新增 D7-S22 TaskContract PR-B 通讯契约预留位**（2 F PLANNED：PessimisticCommitGuard + CoWVersionChain，PR-A 仅登记接口签名不实现）；(3) F 总数 75 → **84 IMPLEMENTED + 2 PLANNED = 86**；(4) 全部 F 物理位置 `orchestration/interfaces/`（pure types 原则 0 import D7 子包）；(5) Additive 嵌入 ChannelRequest.Spec + LearnRequest.Report（**老路径完全不变**，仅可选追加指针） |
| **5.2.0** | **2026-06-29** | **v7.0 TaskContract 统一 PR-B（DM-20260629-008）L3 防御运行时层**：(1) **新增 D7-S18 段**（7 F IMPLEMENTED：D7-S18-A11/F01-F05 Pessimistic Commit 五件套 + D7-S18-A12/F01-F02 Rule-based Fallback 双件）；(2) F 总数 86 → **93 IMPLEMENTED**（+7 PR-B, PR-A D7-S22 PessimisticCommitGuard 预留位已实现并归入 D7-S18-A11）；(3) 物理位置 `orchestration/interfaces/{contracts,fallback_policy,convergence_budget}.go` + `orchestration/escape/fallback.go` + `orchestration/escape/engine.go::NotifyPessimistic` + `orchestration/mups/execute/channel.go::ChannelRouter.SetPessimisticGuard/ApplyPessimisticCommit` + `internal/bootstrap/pessimistic_guard_wire.go`；(4) **Feature Flag D7_PESSIMISTIC_COMMIT_ENABLED 默认 disabled, 0 行为变更**（所有方法 nil/disabled 守门 no-op） |
