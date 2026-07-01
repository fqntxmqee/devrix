# D7 Orchestration Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active · **6 S 精简 (v6.0.0 DM-20260626-001)**
**Version:** 5.5.0
**Last Updated:** 2026-07-01 (devrix-d7-physical-layout-alignment DM-20260701-004 PR-4: §D7-X Cross-S Kernel (orchtypes/) F 段 cross-reference 收敛到 d7-domain.md §North Star Cross-S Kernel 行 + orchtypes/doc.go package 注释升级到 cross-S governance kernel 语义（PR-1 已落地 ## D7-X F 段 6 F + Statistics + Revision History，本 PR 收尾 doc.go + d7-domain.md 文档对齐）；**previous**: devrix-d7-physical-layout-alignment DM-20260701-004 PR-1: ## Hardening F 段 (2 F) + ## D7-X Cross-S Kernel F 段 (6 F) → v5.4.0; **earlier**: devrix-d7-historical-s-cleanup DM-20260701-003 → v5.3.0; **earlier**: devrix-d7-taskcontract-unification-pr-b DM-20260629-008: D7-S18 Pessimistic Commit + Rule-based Fallback 7 F)
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d7-orchestration/a-registry.md`
**Domain SoT:** `d7-domain.md`

> **MUPS v4.3 5 节点管道（2026-06-25 落地）：** 历史 F 层曾扩展至 S1-S14 全部节点。DM-20260701-002 后，current canonical S 固定为 S1-S6：Observe/Plan 归 S5，Execute/Verify/Learn/Escape/Convergence 归 S6；S7-S14 仅作 historical mapping。
>
> **v6.0.0 6 S 精简（DM-20260626-001）：** 14 S → 6 S + 1 横切后 F 层按新 S 重归类。具体 A/F 重映射见 `a-registry.md §v6.0.0 6 S 精简映射`。

---

## Overview

D7 编排域 F 层功能点注册表。代码位置标注**现行路径**；`(planned)` 表示目标 D7 包尚未创建。

**状态图例：** ✅ · 🔶 · ⬜

### Current Path Correction（DM-20260701-002）

| Historical Function Area | Current Canonical Target | Current Runtime Path |
|--------------------------|--------------------------|----------------------|
| Observe / UncertaintyReport | D7-S5 Decision & Planning | `sessionorchestrator/item_observe.go` + `orchtypes/` |
| StrategicPlanProposer | D7-S5 Decision & Planning | `sessionorchestrator/strategic_plan_proposer.go` |
| Execute WorkItem | D7-S6 MUPS Governance | `sessionorchestrator/workitem_executor.go` |
| Verify / Deliverable Gate | D7-S6 MUPS Governance | `sessionorchestrator/deliverable_verify.go` + `item_pipeline.go` |
| Learn | D7-S6 MUPS Governance | `mups/learn/` |
| Rollup / Context Bubble | D7-S1 Work Model + D7-S6 Governance | `workmodel/rollup_gate.go` + `workmodel/context_bubble_apply.go` + `sessionorchestrator/rollup_*` |
| TaskSpec / TaskReport | Contract → D7-S1/D7-S6 | `interfaces/task_spec.go` + `interfaces/task_report.go` |

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
| D7-S2-A01-F02 | RunItemPipeline | F-BE | message, session | events | ✅ | `sessionorchestrator/item_pipeline.go` Run |
| D7-S2-A01-F03 | RunSessionTurnLoop | F-BE | message, session | WorkItem rounds | ✅ | `sessionorchestrator/session_turn_loop.go` |
| D7-S2-A01-F04 | EmitSessionEvents | F-BE | event | — | ✅ | `sessionorchestrator/turn_orchestrator.go` + `EventPublisher` |
| D7-S2-A01-F05 | HandleInterrupt | F-BE | session_id, reason | — | ✅ | `sessionorchestrator/interrupt.go` |

## D7-S2-A02 EvaluateIntent ✅

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S2-A02-F01 | ClassifyByRules | F-BE | message | rules_hint + confidence | ✅ | `decisionplanning/classifier.go` RuleClassifier.Classify |
| D7-S2-A02-F02 | ClassifyByLLM | F-BE | message, rules_hint | llm_classification | 🔶 | `decisionplanning/classifier_fallback.go`（合并 LLM-first 路径） — 物理文件未实现（功能已 inline 到 `classifier.go::ClassifyWithPrior`），DM-20260701-004 PR-2 layout guard 守护 |

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
| D7-S5-A03-F01 | MatchExecutorByTaskType | F-BE | task_type | executor_id | 🔶 | `decisionplanning/executor.go` SelectExecutor — 物理文件未实现，DM-20260701-004 PR-2 layout guard 守护 |
| D7-S5-A03-F02 | CheckExecutorAvailability | F-BE | executor_id | available/busy | 🔶 | `decisionplanning/executor.go` CheckAvailability — 物理文件未实现，DM-20260701-004 PR-2 layout guard 守护 |

---

## Canonical F 层定义（切法 A — 按用户价值流）

> 以下为 `devrix-d7-sa-refine` v1.0 Canonical F 层定义。补充 design.md §5 草案。

### D7-S2 Canonical F 层

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S2-A01-F01 | RouteByIntent | F-BE | IntentClassification | routing_decision | ✅ | `sessionorchestrator/orchestrator.go` ProcessMessage switch |
| D7-S2-A01-F02 | RunItemPipeline | F-BE | message, session | events | ✅ | `sessionorchestrator/item_pipeline.go` Run |
| D7-S2-A01-F03 | RunSessionTurnLoop | F-BE | message, session | WorkItem rounds | ✅ | `sessionorchestrator/session_turn_loop.go` |
| D7-S2-A01-F04 | EmitSessionEvents | F-BE | event | — | ✅ | `sessionorchestrator/orchestrator.go` + `EventPublisher` |

### D7-S5 Canonical F 层

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S5-A01-F01 | ClassifyByRules | F-BE | message | rules_hint | ✅ | `orchestration/decisionplanning/classifier.go` RuleClassifier.Classify |
| D7-S5-A01-F02 | ClassifyByLLM | F-BE | message, rules_hint | llm_classification | 🔶 | `orchestration/decisionplanning/classifier_fallback.go` LLMClassifier.Classify — 物理文件未实现（功能已 inline 到 `classifier.go::ClassifyWithPrior`），DM-20260701-004 PR-2 layout guard 守护 |
| D7-S5-A01-F03 | MergeClassifications | F-BE | rules, llm | final_decision | 🔶 | `orchestration/decisionplanning/classifier_fallback.go` Merge — 物理文件未实现（功能已 inline 到 `classifier.go::ClassifyWithReport`），DM-20260701-004 PR-2 layout guard 守护 |

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

## Historical / Contract F Detail

Former D7-S8–S14 / S18 / S20–S22 F registrations moved to **`historical-s-mapping.md`**. T IDs retain historical A/F prefixes; do not reintroduce as current F headings here.

## Statistics

| Activities with F | Total F Points | Implemented | Planned |
|-------------------|----------------|-------------|---------|
| **deprecated 2 + canonical 73 + v7.0 TaskContract 11 + Hardening 2 + D7-X 6 = 94**（v6.0.0 6 S 精简 + v7.0 PR-A + PR-1 Hardening/D7-X 增量, DM-20260629-007 + DM-20260701-004） | **94** | **92** (canonical 73 + TaskContract 11 + Hardening 2 + D7-X 6 - 2 PLANNED S22) | **2 (D7-S22 PR-B/C)** |

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

## Hardening ✅ Canonical（横切 Discipline Keeper — 物理 location 补登，DM-20260701-004 PR-1）

> **物理 location 锚点：** `internal/layers/orchestration/hardening/`（emitter 集中点 namespace，**不是 owner**）
> **承载职责：** metric 命名 / 并发原子化 / state bound / emitter hardening 等专项 fix（DM-20260622-001 已落地的 5 P0/P1 fix 的 F 层分解）。
> **与 §v6.0.0 6 S 精简映射 - Cross-cutting Hardening 段 关系：** 本段补充物理路径锚点 + 实装 F；§v6.0.0 段保留 14 S → 6 S 重映射视角。两者互补，不重复。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| Hardening-A01-F01 | EmitMetricSpan | F-BE | metric_name, attributes | MetricSpan | ✅ | `orchestration/hardening/emitter.go::EmitMetric`（20+ Emit* span helper 跨节点统一） |
| Hardening-A02-F01 | ConflictGuardAtomic | F-BE | candidate, conflict_scope | allowed/blocked | ✅ | `orchestration/wavescheduler/conflict.go::AllowAndRegisterAtomic`（owner: wavescheduler/，**hardening/ 仅是 namespace**） |

> **横切 Discipline Keeper F 层**：与 A 层对应 — A01 MetricsEmit = F01 EmitMetricSpan（emit helper）；A02 ConcurrencyGuard = F01 ConflictGuardAtomic（hot path atomic call）。ConflictGuard 实际 owner 是 `wavescheduler/`，物理位置与 hardener 命名解耦。

---

## D7-X: Cross-S Kernel（orchtypes/）✅ Canonical — DM-20260701-004 PR-1 新增

> **物理 location 锚点：** `internal/layers/orchestration/orchtypes/`（共享类型 / 哨兵 / 边界决策 / 先验 / 异常检测 / 配置 / LLM 调用契约）
> **承载职责：** 跨 S1-S6 共享 primitives 的 F 层分解（不可变操作 / 类型转换 / 校验函数）。
> **no Go shim, no re-export, 直接 import**：orchtypes/ 是物理 kernel 包。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-X-A01-F01 | NewObservation | F-BE | kind, strength, scope, stakes | Observation | ✅ | `orchtypes/observation.go::NewObservation`（4 ObsKind × 2 Category + sealed Payload 不可变构造） |
| D7-X-A01-F02 | NewUncertaintyReport | F-BE | observations, anomalies | UncertaintyReport | ✅ | `orchtypes/uncertainty_report.go::NewUncertaintyReport`（Partition 不变式 + ComputeOverallStrength 仅遍历 CatBusiness） |
| D7-X-A02-F01 | NewORCHSentinelError | F-BE | code, scope, msg | SentinelError | ✅ | `orchtypes/errors.go::NewORCHSentinelError`（ORCH_* 7100-7113 工厂 + shared/errors.SentinelError 包装） |
| D7-X-A03-F01 | NarrowestSchema | F-BE | inferred, strategic | NarrowerSchema | ✅ | `orchtypes/boundary_decision.go::NarrowestSchema`（单调收窄，LLM 只能收紧不能放宽） |
| D7-X-A04-F01 | BayesianUpdate | F-BE | prior, evidence | Posterior | ✅ | `orchtypes/adaptive_prior_overload.go::BayesianUpdate`（α/β++ + Wilson Score 95% 置信区间 + G8-1 修复 INDETERMINATE parse_failure 不污染） |
| D7-X-A06-F01 | ValidateLLMRequest | F-BE | LLMRequest | (ok, error) | ✅ | `orchtypes/llm_invoker.go::ValidateLLMRequest`（fail-fast 校验 session_id/prompt/model/track_mode 非空） |

> **D7-X F 层与 A 层对应：** A01 DefineCrossSPrimitives = F01/F02 (Observation + UncertaintyReport 2 主件)；A02 DefineSentinelErrors = F01 NewORCHSentinelError；A03 BoundaryDecision = F01 NarrowestSchema；A04 AdaptivePriorInject = F01 BayesianUpdate；A05 SystemAnomalyDetect = （F 层见 executionflow/verify/，跨包 wiring 通过 orchtypes/system_anomaly_wiring.go）；A06 LLMInvokerContract = F01 ValidateLLMRequest。

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
| **5.4.0** | **2026-07-01** | **D7 Physical Layout Alignment（DM-20260701-004）PR-1**：**(1) 新增 ## Hardening F 段**（2 F：Hardening-A01-F01 EmitMetricSpan + Hardening-A02-F01 ConflictGuardAtomic），物理 location 锚点 `orchestration/hardening/`（namespace）+ `orchestration/wavescheduler/`（owner）双归属；**(2) 新增 ## D7-X Cross-S Kernel（orchtypes/）F 段**（6 F：D7-X-A01-F01 NewObservation + D7-X-A01-F02 NewUncertaintyReport + D7-X-A02-F01 NewORCHSentinelError + D7-X-A03-F01 NarrowestSchema + D7-X-A04-F01 BayesianUpdate + D7-X-A06-F01 ValidateLLMRequest），**物理即 kernel，0 shim / 0 alias / 0 reverse import**；**(3) Statistics 更新** 86 → **94 F total**（+8 IMPLEMENTED, +2 canonical Hardening + +6 D7-X），84 → **92 IMPLEMENTED**，PLANNED 保持 2 (D7-S22)；**(4) 0 函数签名变化**（purely additive — 仅追加 ## Hardening F + ## D7-X F 两段 + Statistics + Revision History 行）；**(5) cumulative version bump**：跳过 v5.3.0（该位预留给 devrix-d7-s-layer-normalization DM-20260701-002/003 — S1-S6 canonical 收敛 + S7+ → historical-s-mapping.md 物理拆分），如未来该 PR merge 时检测到 v5.4.0 已合入，则跳过 v5.3.0 步进 |
| **5.5.0** | **2026-07-01** | **D7 Physical Layout Alignment（DM-20260701-004）PR-4**：§D7-X Cross-S Kernel F 段 cross-reference 收敛（PR-1 已落地 6 F 段，本 PR 不重复加 F 行，只做 doc.go package 注释升级 + d7-domain.md §North Star Cross-S Kernel 行收尾）：**(1)** `orchtypes/doc.go` package 注释由 "Package orchtypes holds D7 orchestration shared types (config, intent, process)." 升级为 "Package orchtypes is the cross-S governance kernel of D7 (types, sentinels, intent/observation primitives)."（与 a-registry.md ## D7-X 段 + f-registry.md ## D7-X 段 + code-layout.md §4.2 Cross-S Kernel 行 四处语义对齐）；**(2)** d7-domain.md §North Star 新增 1 行 Cross-S Kernel (orchtypes/) — `types / sentinels / intent primitives / Bayesian / Verdict / Observation / UncertaintyCoord / PlanKind / ChannelKind / ArtifactKind / 14 ExitReason — single source of truth for D7 contract`；**(3)** 0 函数签名变化（purely doc-only 收尾：doc.go 注释 + d7-domain.md 1 行 + Revision History 行）；**(4) cumulative version bump**：跳过 v5.4.1（PR-3 已用 v5.5.0 步进，本 f-registry 也用 v5.5.0 标 PR-4 收尾）。 |
