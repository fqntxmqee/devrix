# D7 Orchestration Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 5.6.0
**Last Updated:** 2026-07-05 (mups-verify-table-driven DM-20260705-005: D7-S10-A101 verify_decision_table kernel + 3 verify 函数走表驱动化活动登记 → v5.6.0; **previous**: devrix-d7-physical-layout-alignment DM-20260701-004 PR-3: D7-S6 段新增 D7-S6-A03 PlanValidate + D7-S6-A04 PlanGenerate 行 + 新增 ## D7-S5 plan/ ↔ decisionplanning/ 双登记说明段 + S5 段加 cross-reference → v5.5.0)
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `d7-domain.md`

> **Current canonical（DM-20260701-002）：** D7 current S 层固定为 **S1-S6**。S7-S14 是 MUPS 历史节点拆分，S20/S21 是 TaskContract contract assets，均不再作为 current canonical S。历史 ID 保留在 mapping 段用于追溯，不重编号历史 T。

---

## Overview

D7 编排域 A 层活动注册表。

> **终态流程 / IntentKind 四链：** 见 `terminal-state-guide.md` §3–§7；**Span↔T Runbook：** 见 `observability-guide.md`。

代码分布（current runtime）：

- `internal/layers/orchestration/sessionorchestrator/`（D7-S2）
- `internal/layers/orchestration/decisionplanning/`（D7-S5）
- `internal/layers/orchestration/wavescheduler/` + `executionflow/`（D7-S3 + D7-S4）
- `internal/layers/orchestration/workmodel/`（D7-S1）
- `internal/layers/orchestration/sessionorchestrator/`（D7-S2-A06/A07 Turn Leader）
- `internal/layers/orchestration/sessionorchestrator/`、`mups/`、`escape/`、`interfaces/`（D7-S6 MUPS Governance）
- `internal/layers/orchestration/hubspoke/`（historical shim only, not current S）

**状态图例：** ✅ IMPLEMENTED · 🔶 PARTIAL · ⬜ PLANNED

---

## D7-S1: Work Model ✅ IMPLEMENTED（v4.3 post-cleanup）

> **WorkItem** 写模型位于 `internal/layers/orchestration/workmodel/{work_tree,workitem,workitem_store,task_manager}.go` + `sessionorchestrator/workmodel.go`。v4.3 post-cleanup（PR #214）已删 `workmodel/task_store.go`（Task flat-view）+ `workitem.go` 内 conversion helpers + `taskStoreAdapter`，**WorkItem 是唯一 canonical 模型**, TaskManager 只是 `Tree()` facade。PlanMode 在 `workmodel/{plan_mode,plan_agent}.go`, PlanAgent 仅服务于 `/plan` CLI 命令入口。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-S1-A01 | CreateWorkPlan | A-BE | session_id, goal, context | Plan (**WorkItem** 列表 + DAG) | work_plan.created | ✅ | `sessionorchestrator/workmodel.go` + `workmodel/plan_mode.go` |
| D7-S1-A02 | ManageWorkItem | A-BE | item_id, action (create/update/delete/dep) | workitem_state | workitem.* | ✅ | `workmodel/{work_tree,task_manager}.go` + `workitem_store.go`（DiskWorkItemStore schema v2） |
| D7-S1-A03 | QueryWorkPlan | A-BE | session_id | WorkPlanSnapshot | — | ✅ | `orchestration/executionflow/hub/hub.go` Snapshot |

### D7-S1 附加活动（PlanMode）

| A ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| D7-S1-A04 | EnterPlanMode | A-BE | session_id, goal | plan_state | ✅ | `workmodel/plan_mode.go` |
| D7-S1-A05 | ApprovePlan | A-BE | session_id | []WorkItem | ✅ | `workmodel/plan_mode.go` |
| D7-S1-A06 | ExecutePlanAgent | A-BE | goal, context | PlanResult | ✅ | `workmodel/plan_agent.go` |

---

## Canonical S 层定义（切法 A — 按用户价值流）

> 以下为 `devrix-d7-sa-refine` v1.0 Canonical 定义。流程见 `terminal-state-guide.md`。

### D7-S1: Work Model ✅ Canonical（v4.3 post-cleanup）

> North Star: **WorkItem 事实与状态机**单一权威（v4.3 起, Task flat-view + TaskStore 全删）
> 博弈角色: **State Authority**

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| D7-S1-A01 | CreateWorkPlan | D7-S1-A01-LEGACY | A-BE | session_id, goal, context | Plan (**WorkItem**) | workitem.created | ✅ | `sessionorchestrator/workmodel.go` + `workmodel/plan_mode.go` |
| D7-S1-A02 | ManageWorkItem | D7-S1-A02-LEGACY | A-BE | item_id, action | workitem_state | workitem.* | ✅ | `workmodel/{work_tree,task_manager}.go` + `workitem_store.go` |
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
| **D7-S2-A06** | **RunTurnLoop** | — | **A-BE** | **session, TurnRequest** | **<-chan EngineEvent** | **turn.{started,completed,failed}** | **✅** | **`orchestration/sessionorchestrator/turn_orchestrator.go`**（DM-020 v1.0-c；wired by `bootstrap/wire_coordinator.go:60`） |
| **D7-S2-A07** | **InvokeLLM** | — | **A-BE** | **LLMInvokeRequest** | **<-chan Chunk** | **llm.{invoked,streaming,completed}** | **✅** | **`orchestration/sessionorchestrator/llm.go`**（DM-020 v1.0-b；wired by `bootstrap/wire_coordinator.go:59`）。**兼作 D2→D3 拆面出口**：`turn.QueryLLMCaller` + `turn.CompressionSummarizer` 由同一 `llmgateway.IGateway` 驱动，单一注入点 `bootstrap/context_engine.go` wired 至 `EngineDeps.QueryLLMCaller` / `EngineDeps.Summarizer`（DM-020 v2.3 拆面闭合） |

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
| D7-S4-A05 | BridgeSubQuerySpoke | D2-S19（Flow 面） | A-BE | subquery_result | flow_event | flow.published | 🔶 | `executionflow/bridge/subquery_bridge.go`（wired by `bootstrap/delegate.go`） — 物理文件未实现，DM-20260701-004 PR-2 layout guard 守护 |

> **D7-S4-A04/A05**（DM-20260614-018）：统一三 Spoke 写侧（D4 Delegate / D2 SubQuery / D7 Wave）→ `ExecutionFlowHub`。

### D7-S5: 决策规划 ✅ Canonical

> North Star: 把用户 goal 转化为可执行的任务结构（结构路径，非内容质量）
> 博弈角色: Information Producer（产私有信息）
> **Explore 输入:** SynthesizeTaskGraph 吸收 Explore Workers（并行 read-only）通过 D7-S4 广播的 FlowEvent

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| D7-S5-A01 | ClassifyIntent | D7-S5-A01-LEGACY | A-BE | message, session, context | IntentClassification | — | ✅ | `orchestration/decisionplanning/classifier.go` + `classifier_fallback.go` |
| D7-S5-A02 | SynthesizeTaskGraph | D7-S5-A02-LEGACY | A-BE | goal, constraints, explore_events | []TaskNode | plan.formulated | ✅ | `decisionplanning/decomposer.go`（rule + LLM via SetLLMDecomposer） |
| D7-S5-A03 | SelectExecutor | — | A-BE | TaskNode, agent_pool | executor_id (D2/D4) | — | 🔶 | `decisionplanning/executor.go` — 物理文件未实现，DM-20260701-004 PR-2 layout guard 守护 |

> **D7-S5 双登记提示**：D7-S5 主路径为 `decisionplanning/`，但 `plan/` 是 **S5 sub-registration carve-out**（doc-only dual registration），详见下方 `## D7-S5 plan/ ↔ decisionplanning/ 双登记说明` 段。`D7-S6-A04 PlanGenerate` 物理路径在 `plan/planner.go`，但属于 S6 治理 Activity（见 S6 表）。

### D7-S6: MUPS Governance + Observability & Hardening ✅ Canonical

> North Star: MUPS Execute / Verify / Learn / Escape / convergence governance 与 D7 编排层 hardening 统一归口；S6 是 governance overlay，不要求新增单独 scenario 目录。
> 博弈角色: Discipline Keeper（守住 spec 纪律与并发安全）
> **范围：** 横切 S2/S3，不属于 5 个核心 S 的纵向业务流

| A ID | Name | Legacy ID | Type | Input | Output | State Change | Status | Code Location |
|------|------|-----------|------|-------|--------|--------------|--------|---------------|
| **D7-S6-A03** | **PlanValidate** | **D7-S6-A03-LEGACY** | **A-BE** | **Plan, ConstraintSet** | **[]PlanValidationError** | **plan.validated** | **✅** | **`decisionplanning/plan_mode.go::Validate`**（S6 治理 Activity，物理路径在 S5 `decisionplanning/`，符合 spec.md §S5 carve-out Note） |
| **D7-S6-A04** | **PlanGenerate** | **D7-S6-A04-LEGACY** | **A-BE** | **goal, observation, rules** | **Plan** | **plan.generated** | **✅** | **`plan/planner.go::DefaultPlanner.Generate`**（S6 治理 Activity，物理路径在 S5 `plan/`，符合 spec.md §S5 carve-out Note + design §④ S5 sub-registration carve-out，doc-only 双登记：`plan/` ↔ `decisionplanning/` 共存，**0 shim / 0 alias / 0 git mv**）|
| **D7-S6-A14** | **HardenMetricsAndConcurrency** | **—** | **A-BE** | **—** | **—** | **—** | **✅** | **5 fix 一揽子（DM-20260622-001）：(A1) `dispatch_loop_wakeups`/`worker_panics` 复数化与 switch case 双修；(A2) `dispatch_loop_wakeups`/`worker_panics` 命名口径 spec/code 一致；(A3) `markWaveDone` 释放 `state.cancels`/`state.handles` 至 nil/空 map 防跨 wave 累积；(A4) `dispatchOne` 改原子 `AllowAndRegister` 关 TOCTOU 窗口；(A5) `CommandHandler.emit` 改 `select-default` + `slog.Warn` 防 consumer 阻塞；(A6) `sandbox_exit_failed` 跨域归属澄清（D4 实际拥有，D7 spec 标 OBSOLETE）** |

---

## D7-S5 `plan/` ↔ `decisionplanning/` 双登记说明（DM-20260701-004 PR-3）

> **决策记录（design §④ Q1 选 B：doc-only）**：`plan/` 与 `decisionplanning/` 物理共存，**避免 43 importer 改动**。
> 
> **Doc-only dual-registration 定义（design §④ S5 sub-registration carve-out）**：
> - `plan/`（物理独立子包，5 prod + 1 test 共 6 .go 文件）：`plan.go` / `plan_struct.go` / `planner.go` / `blast_radius.go` / `errors.go` + `plan_test.go`
> - `decisionplanning/`（S5 主路径，8 prod + 8 test 共 16 .go 文件）：`classifier.go` / `classifier_fallback.go` / `decomposer.go` / `llm_decomposer.go` / `plan_mode.go` / `similarity_gate.go` / `filter.go` / `filter_adapter.go` + 8 对应 `_test.go`
> - 两个包各自承担不同职责，但都属于 S5 Decision & Planning 范畴
> - **0 物理 shim / 0 alias / 0 git mv** — 仅在 a/f-registry 双登记索引
> - a-registry: `D7-S5-A02 SynthesizeTaskGraph`（decisionplanning/）+ `D7-S6-A04 PlanGenerate`（plan/）双登记路径
> - code-layout.md §4.2 line 102：`D7-S5 sub | Plan Generation | plan | orchestration/plan/ | ✅ doc-only 双登记`

---

## Historical / Contract Detail

Former D7-S7–S14 MUPS node sections, D7-S18 pessimistic runtime, D7-S20/S21 TaskContract tables, and v6.0.0 14→6 S remap live in **`historical-s-mapping.md`**. Current A registration above (S1–S6) is authoritative.

## ValueFlow Semantic 映射（v6.0.0 + v2.5.1 同步）

> **ValueFlow Alias per S**（定义见 `d7-domain.md` §North Star）：
> - **S1 WorkModel** = Multi-Step Task Coordination
> - **S2 SessionOrchestrator** = Turn-Based Conversation
> - **S3 WaveScheduler** = Parallel Worktree Execution
> - **S4 ExecutionFlow + Verify** = Trustworthy Conclusion Delivery
> - **S5 DecisionPlanning + Observe** = Intent + Uncertainty Quantization
> - **S6 MUPS Pipeline** = Learn from Outcome
> - **Cross-cutting Hardening** = (Discipline Keeper)

| A ID | Name | ValueFlow Semantic（用户动作语义） |
|------|------|-----------------------------------|
| S1-A01 | CreateWorkPlan | 用户给出目标 → 拆解为多步任务 DAG |
| S1-A02 | ManageWorkItem | 用户/系统增删改查 WorkItem 节点 |
| S1-A03 | QueryWorkPlan | 用户/系统查询当前任务计划状态 |
| S1-A04 | ExecutePlanAgent | 用户/系统执行规划好的多步任务 |
| S2-A01 | ProcessMessage | 用户发送消息 → 进入 D7 编排对话 |
| S2-A02 | HandleInterrupt | 用户/系统中断当前编排流程 |
| S2-A03 | CommandHandler | 用户发 slash 命令（/plan, /worktree, /help, /stop） |
| S2-A04 | DispatchWorker | D7 派发任务给 D4 Worker |
| S2-A05 | RunTurnLoop | 多轮 LLM 对话主循环 |
| S2-A06 | InvokeLLM | 调用 LLM 生成回复 |
| S2-A07 | AutoClose + Resume + Escape + PriorBuild | 会话自动收口 + 用户恢复会话 + 紧急逃生 + 先验注入 |
| S3-A01 | ScheduleWave | 调度一波并行 Worker 任务 |
| S3-A02 | ResolveWorkerContext | 为 Worker 解析执行上下文 |
| S3-A03 | GuardConflict | 防止并行任务冲突（file scope / group） |
| S3-A04 | HardenScheduler | 调度器并发硬化（atomic + state bound） |
| S4-A01 | PublishFlowEvent | 广播执行进度事件给用户 |
| S4-A02 | SnapshotWorkPlan | 快照当前任务计划 |
| S4-A03 | NotifyGateway | 通知 IM 网关推送卡片 |
| S4-A04 | BridgeAgentSpoke | D4 Agent 进度接入执行流 |
| S4-A05 | BridgeSubQuerySpoke | D2 SubQuery 结果接入执行流 |
| S4-A06 | VerifyVerdict | 验证产物是否满足完成标准 |
| S4-A07 | VerdictToExitReason | 把验证结果映射为退出原因 |
| S4-A08 | AggregateVerdicts | 聚合多个验证结果 |
| S4-A09 | DetectSystemAnomaly | 检测系统异常信号 |
| S5-A01 | ClassifyIntent | 分类用户意图（Command/Fast/Orchestrate/Skip） |
| S5-A02 | SynthesizeTaskGraph | 合成可执行任务图 |
| S5-A03 | SelectExecutor | 选 D2/D4 执行器 |
| S5-A04 | EvaluateIntent | 评估意图（带上下文） |
| S5-A05 | TailShadowClassify | 影子分类（异步观测） |
| S5-A06 | ObserveQuantize | 把消息+历史量化为 4 类观察 |
| S5-A07 | PlanGenerate | 生成 4 类执行计划 |
| S5-A08 | PriorLoad | 加载历史信誉先验 |
| S6-A01 | ExecuteArtifact | 按计划执行产出 4 类产物 |
| S6-A02 | RouteChannel | 选 4 通道（sync/async/probe/explore） |
| S6-A03 | ChannelDispatch | 通道路由派发 |
| S6-A04 | ToolCall | 调用工具 |
| S6-A05 | RetryPolicy | 工具调用重试策略 |
| S6-A06 | BuildLearningAsset | 构建 5 类学习资产 |
| S6-A07 | UpdateReputationEvidence | 更新 Bayesian 信誉证据 |
| S6-A08 | BuildAdaptivePrior | 构建自适应先验 |
| S6-A09 | MemoryPersist | 持久化到 3 通道记忆（skill/feedback/scheduled） |
| S6-A10 | RunLearner | 跑学习者主循环 |
| S6-A11 | FeedbackPersist | 持久化反馈通道 |
| S6-A12 | ScheduledPersist | 持久化计划通道 |
| S6-A13 | CrossSessionLearning | 跨 session 学习 |
| S6-A14 | ObserveLearnerLoop | 观察-学习闭环 LP-1 |
| S6-A15 | AutoClose | 终态自动收口 |
| Hardening-A01 | HardenMetricsAndConcurrency | metric 命名对齐 + 并发硬化 |
| Hardening-A02 | HardenCircuitBreakerMonitor | CircuitBreaker 监控硬化 |

> **ValueFlow Semantic 列**（49 A 全覆盖）：用户动作语义视角；与 6 S 博弈角色 + 4.3 MUPS 5 节点管道 三视角互补。`legacy_harness` 域内 0 A。

---

## Hardening ✅ Canonical（横切 Discipline Keeper — 物理 location 补登，DM-20260701-004 PR-1）

> **物理 location 锚点：** `internal/layers/orchestration/hardening/`（emitter 集中点 namespace，**不是 owner**）
> **承载职责：** metric 命名 / 并发原子化 / state bound / emitter hardening 等专项 fix（DM-20260622-001 已落地的 5 P0/P1 fix）。
> **与 §v6.0.0 6 S 精简映射 - Cross-cutting Hardening 段 关系：** 本段补充物理路径锚点 + 实装 A；§v6.0.0 段保留 14 S → 6 S 重映射视角。两者互补，不重复。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| Hardening-A01 | MetricsEmit | A-BE | metric_name, attributes | MetricSpan | metric.recorded | ✅ | `orchestration/hardening/emitter.go`（20+ Emit* span helper 跨节点） |
| Hardening-A02 | ConcurrencyGuard | A-BE | candidate, conflict_scope | allowed/blocked | — | ✅ | `orchestration/wavescheduler/conflict.go` + `orchestration/wavescheduler/scheduler.go`（ConflictGuard AllowAndRegister 原子调用,**owner: wavescheduler/**，`hardening/` 仅是 namespace） |

> **横切 Discipline Keeper**：横切 S2/S3/S6；ConflictGuard 实际 owner 是 `wavescheduler/`，物理位置与 hardener 命名解耦。`hardening/` 仅是 namespace（emitter 集中点），不是 owner；并发原子化的实施 owner 是 `wavescheduler/scheduler.go::dispatchOne`。后续 v1.1 评估：在 hardening/ 增加 re-export 透明层。

---

## D7-X: Cross-S Kernel（orchtypes/）✅ Canonical — DM-20260701-004 PR-1 新增

> **物理 location 锚点：** `internal/layers/orchestration/orchtypes/`（共享类型 / 哨兵 / 边界决策 / 先验 / 异常检测 / 配置 / LLM 调用契约）
> **承载职责：** 跨 S1-S6 共享 primitives，被 S5 (intent) + S6 (types) + S1-S6 各层直接 import。
> **no Go shim, no re-export, 直接 import**：orchtypes/ 是物理 kernel 包，不通过 alias 暴露。

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| D7-X-A01 | DefineCrossSPrimitives | A-BE | schema | SharedType | type.registered | ✅ | `orchtypes/`（根包 — Observation / UncertaintyReport / UncertaintyCoord / Verdict / LearningAsset / ReputationEvidence / AdaptivePrior / PlanKind / ChannelKind / ArtifactKind / SideEffectStatus / ExitReason 等核心类型） |
| D7-X-A02 | DefineSentinelErrors | A-BE | error_code, scope | SentinelError | error.registered | ✅ | `orchtypes/errors.go`（ORCH_* 7100-7113 + ErrORCHTaskSpecEmpty/TaskSpecChannelUnknown/TaskReportEmpty/TaskReportVerdictEmpty/TaskContractTraceInvalid + ErrORCHPessimisticTriggered/EmptyMVP/FallbackRuleInvalid/FallbackAbortTimeout + ErrChannelCtxCancelled 等） |
| D7-X-A03 | BoundaryDecision | A-BE | boundary_request | BoundaryDecision | boundary.recorded | ✅ | `orchtypes/boundary_decision.go`（SchemaMonotonicNarrowing + BoundaryReason + BoundaryAction enum） |
| D7-X-A04 | AdaptivePriorInject | A-BE | session_id, track_mode | AdaptivePrior | prior.injected | ✅ | `orchtypes/adaptive_prior_overload.go`（WithPrior 变体 + DefaultDeveloperPrior Beta(5,3) + DefaultOperatorPrior Beta(8,1) + BuildAdaptivePrior Bayesian 合并公式 + EffectivePrior 兜底） |
| D7-X-A05 | SystemAnomalyDetect | A-BE | observation, history | SystemAnomaly | anomaly.detected | ✅ | `orchtypes/system_anomaly_wiring.go`（跨包 wiring 到 executionflow/verify，SystemAnomaly + AnomalyReport + HistoricalDetector baseline + DetectWithPrior threshold = 0.5 × Mean） |
| D7-X-A06 | LLMInvokerContract | A-BE | LLMRequest | LLMResponse | llm.invoked | ✅ | `orchtypes/llm_invoker.go`（IGateway 接口 + LLMInvokeRequest/Chunk/Response 不可变 + D2→D3 拆面契约） |

> **Cross-S Kernel 设计原则：**
> - **物理即 kernel**：`orchtypes/` 是真实物理包（不是 alias 也不是 shim）
> - **0 reverse import**：orchtypes/ 不 import D7 任何子包（仅依赖 shared/errors + shared/types）
> - **S-level 透明**：S1-S6 任意 A 可直接 import orchtypes/，无需中间层
> - **演进路径**：后续 v1.1 评估 — 把 D7-X-A01 子类型按 S 维度再分文件（observation.go / uncertainty.go / verdict.go ...）但保持单包

---


## D7-S9-A50: ToolChannel 路由 (per-EmissionClass termination)

**承诺 C6：** 为 Execute 节点 4 ToolChannel 提供 per-EmissionClass termination 路由 + LTL-Lite L4–L6 invariant 挂载，根治 LLM 自我循环（demand RC-1）。

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| **D7-S9-A50-T01** | **ToolChannel interface** | **A-BE** | **—** | **ToolChannel** | **—** | **`orchestration/mups/execute/toolchannel/channel.go`** (ToolChannel interface) | **—** |
| **D7-S9-A50-T02** | **ToolChannelRouter** | **A-BE** | **Mode, 4 channels** | **Router** | **per-session state** | **`orchestration/mups/execute/toolchannel/channel.go`** (Router) | **—** |
| **D7-S9-A50-T03** | **ProbeToolChannel Bounded(n)** | **A-BE** | **ToolCall, State** | **(ok, err)** | **state.IterationsUsed++** | **`orchestration/mups/execute/toolchannel/probe.go`** (ProbeToolChannel.Accept) | **—** |
| **D7-S9-A50-T04** | **ProbeToolChannel hard reject** | **A-BE** | **iter>=bound** | **ErrProbeToolChannelBoundExceeded** | **—** | **`orchestration/mups/execute/toolchannel/probe.go`** (Accept) | **—** |
| **D7-S9-A50-T05** | **PromptPressure 3-stage** | **A-BE** | **state, task_kind** | **— (warning emitted)** | **—** | **`orchestration/mups/execute/toolchannel/probe.go`** (InjectPromptPressure) | **—** |
| **D7-S9-A50-T06** | **Fact/Action/Experiment Channels** | **A-BE** | **ToolCall, State** | **ChannelOutcome** | **—** | **`orchestration/mups/execute/toolchannel/{fact,action,experiment}.go`** | **—** |
| **D7-S9-A50-T07** | **Shadow mode** | **A-BE** | **Mode=Shadow** | **(false, nil)** | **wouldRejectCount++** | **`orchestration/mups/execute/toolchannel/channel.go`** (Router.Route) | **—** |
| **D7-S9-A50-T08** | **L0-L3 cross-check** | **A-BE** | **3 CC rules** | **—** | **—** | **`orchestration/mups/execute/toolchannel/probe.go`** (Accept CC-1) | **—** |

> **Code 锚点**: 本 change `devrix-mups-tool-classification-and-channel-autonomy` (DM-20260701-007) Phase B 落地。

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始注册表（全 PLANNED 路径） |
| 2.0.0 | 2026-06-14 | 对齐代码：真实路径、实现状态、PlanMode 活动 |
| 2.1.0 | 2026-06-14 | 包路径迁移 `internal/layers/d7/` → `internal/layers/orchestration/sessionorchestrator/`；D7-S2/S5-A01/S5-A05 标记 ✅ |
| 3.0.0 | 2026-06-14 | Hub-Spoke A 增量：D7-S2-A04 DispatchWorker + D7-S4-A04/A05 SpokeBridge（DM-20260614-018） |
| 3.1.0 | 2026-06-15 | Turn Leader A 增量：D7-S2-A06 RunTurnLoop + D7-S2-A07 InvokeLLM（DM-020 v1.0 Registry） |
| 3.2.0 | 2026-06-15 | **v1.0 + v1.1 闭环对齐**：(1) D7-S1 写模型迁入 coordinator/workmodel.go + workmodel/，A01-A06 全 ✅；(2) D7-S2-A02/D7-S2-A04/D7-S2-A06/D7-S2-A07 wired 至 bootstrap；(3) D7-S4-A04/A05 wired 至 hubspoke；(4) D7-S5-A02/A03 规则+LLM 双路径实装。统计 19+16/19+16/0/0 |
| 3.3.0 | 2026-06-15 | DM-020 D2→D3 拆面闭合：D7-S2-A07 InvokeLLM 兼作 D2 拆面出口（`turn.QueryLLMCaller` + `turn.CompressionSummarizer` 由同一 `llmgateway.IGateway` 驱动） |
| 3.5.0 | 2026-06-16 | Canonical 段补登 S1（A01–A06）；24 A 统计；Guides 指针 |
| 3.8.0 | 2026-06-22 | DM-20260622-001 D7 Metrics & Concurrency Hardening：新增 Canonical S6 横切层（D7-S6-A14 HardenMetricsAndConcurrency），承载 5 P0/P1 fix；Legacy S3-A04 HardenScheduler；Legacy S2-A04 CommandHandler（emit 硬化）。统计 25+20/25+20/0/0 |
| 4.0.0 | 2026-06-25 | MUPS v4.3 5 节点管道 + v5 EscapeEngine 落地（DM-20260623-001/002/003 + DM-20260624-001 + DM-20260625-001/003/004）：Canonical S 扩展至 S1-S14（14 个 S 层）。新增 7 段 + 31 A 活动。统计 56+20/56+20/0/0 |
| **5.0.0** | **2026-06-26** | **6 S 博弈角色对齐精简**（DM-20260626-001）：(1) 14 S → **6 S + 1 横切**（State Authority / Mediator+Turn Leader+Error Recovery / Mechanism Designer / Costly Signaler+Certifier / Information Producer+Quantizer / Pipeline Coordinator+Memory Curator / 横切 Discipline Keeper）；(2) A 活动 **56 → 49**（S1:4 · S2:7 · S3:4 · S4:9 · S5:8 · S6:15 + Hardening:2）；(3) **新增 §v6.0.0 6 S 精简映射**（14 S → 6 S 完整映射表 + 6 S 完整 A 清单 49 A + 14 S 冗余合并依据 4 类）；(4) 7 Legacy A 全部并入 Canonical（不再保留独立 Legacy 段）；(5) MUPS 5 节点挂载：Observe+Plan 归 S5，Execute+Learn 归 S6，Verify 归 S4，AutoClose+Resume+Escape入口 归 S2 |
| **5.1.0** | **2026-06-29** | **v7.0 TaskContract 统一 PR-A（DM-20260629-007）**：**(1) 新增 D7-S20/S21 TaskContract 段**（6 A：D7-S20-A01 BuildTaskSpec + D7-S20-A02 BuildTaskReport + D7-S20-A03 SyncTaskContractSpec + D7-S21-A01 RecordDissent + D7-S21-A02 ClassifyBlockage + D7-S21-A03 ExtractResource），物理包 `orchestration/interfaces/` 7 NEW 文件；(2) **新增 5 个 ORCH_* SentinelError**（7100-7104：TaskSpecEmpty / TaskSpecChannelUnknown / TaskReportEmpty / TaskReportVerdictEmpty / TaskContractTraceInvalid）；(3) A 活动 **49 → 55**（6 v7.0 A 增量）；(4) 4-Layer × 3-Phase 设计框架落地：本 PR 完成 L1 接口层 + L2 字段语义层 + L4 spec 同步 9/11 P0 T IMPLEMENTED（L3 防御运行时层留给 PR-B + PR-C）；(5) Dissent top-3 截断 + summary hash + Learn 沉淀；Blockage 3 类 kind（permission/resource/contract）；Resource token/time/step 三件套；Additive 嵌入 ChannelRequest.Spec + LearnRequest.Report；(6) **0 函数签名变化**（pure types 原则，interfaces 0 import D7 任何子包） |
| **5.2.0** | **2026-06-29** | **v7.0 TaskContract 统一 PR-B（DM-20260629-008）L3 防御运行时层**：(1) **新增 D7-S18 Pessimistic Commit + Rule-based Fallback 段**（2 A：D7-S18-A11 EvaluatePessimistic + D7-S18-A12 ResolveRuleFallback），A 活动 55 → 57；(2) **新增 3 NEW interfaces/ 文件**（contracts.go PessimisticCommitGuard interface + 4 ORCH_* 错误码 7110-7113 + fallback_policy.go FallbackPolicyRuleNames + ParseFallbackRuleName + convergence_budget.go NewConvergenceBudget + With* + Validate + RemainingBelowReserve + ToFields），物理包 `orchestration/interfaces/` 共 10 文件；(3) **新增 escape/fallback.go**（~310 LOC，DefaultPessimisticCommitGuard 5 类触发 + 3 FallbackPolicy 路径 + buildChainHash FNV-1a 非加密 digest）；(4) **escape/engine.go +NotifyPessimistic + SetPessimisticGuard/PessimisticGuard + 4 unit tests**；(5) **mups/execute/channel.go +ChannelRouter.SetPessimisticGuard + ApplyPessimisticCommit**；(6) **bootstrap/pessimistic_guard_wire.go NEW**（PessimisticCommitEnabled + PessimisticRuleStrategy + NewPessimisticCommitGuardFromEnv + 7 env tests）；(7) **circuit_breaker_test.go +3 L1 Pessimistic 联动测试**（L1 trips StateOpen + 60s 持久窗口 + L1-only reason 含 "l1" 路由 hint）；(8) **6/7 T 点 IMPLEMENTED**（T05 Span/Metric 完整 wire 留 PR-C，本 PR 仅 slog.Info 占位）；(9) **Feature Flag D7_PESSIMISTIC_COMMIT_ENABLED 默认 disabled, 0 行为变更** |
| **5.4.0** | **2026-07-01** | **D7 Physical Layout Alignment（DM-20260701-004）PR-1**：**(1) 新增 ## Hardening 段**（2 A：Hardening-A01 MetricsEmit + Hardening-A02 ConcurrencyGuard），物理 location 锚点 `orchestration/hardening/`（namespace）+ `orchestration/wavescheduler/`（owner）双归属；**(2) 新增 ## D7-X Cross-S Kernel（orchtypes/）段**（6 A：D7-X-A01 DefineCrossSPrimitives + D7-X-A02 DefineSentinelErrors + D7-X-A03 BoundaryDecision + D7-X-A04 AdaptivePriorInject + D7-X-A05 SystemAnomalyDetect + D7-X-A06 LLMInvokerContract），**物理即 kernel，0 shim / 0 alias / 0 reverse import**；**(3) 与 §v6.0.0 6 S 精简映射 - Cross-cutting Hardening 段互补**（本段补物理路径锚点，§v6.0.0 段保留 14 S → 6 S 重映射视角）；**(4) 0 函数签名变化**（purely additive — 仅追加 ## Hardening + ## D7-X 两段，§v6.0.0 段与 S1-S6 现有 Canonical 段不动）；**(5) cumulative version bump**：跳过 v5.3.0（该位预留给 devrix-d7-s-layer-normalization DM-20260701-002/003 — S1-S6 canonical 收敛 + S7+ → historical-s-mapping.md 物理拆分），如未来该 PR merge 时检测到 v5.4.0 已合入，则跳过 v5.3.0 步进 |
| **5.5.0** | **2026-07-01** | **D7 Physical Layout Alignment（DM-20260701-004）PR-3**: **(1) D7-S6 段新增 D7-S6-A03 PlanValidate + D7-S6-A04 PlanGenerate 行**：A03 Code Location `decisionplanning/plan_mode.go::Validate`，A04 Code Location `plan/planner.go::DefaultPlanner.Generate`（S6 治理 Activity 物理路径在 S5，符合 spec.md §S5 carve-out Note 与 design §④ S5 sub-registration carve-out）；**(2) 新增 ## D7-S5 plan/ ↔ decisionplanning/ 双登记说明 段**：plan/（5 prod + 1 test 共 6 .go）+ decisionplanning/（8 prod + 8 test 共 16 .go）职责分工 + 0 shim / 0 alias / 0 git mv doc-only dual-registration；**(3) D7-S5 段加 cross-reference 提示**：指向下方双登记说明段 + D7-S6-A04 的物理路径；**(4) 0 函数签名变化 / 0 物理路径变化**（purely additive — 仅 a-registry 内部新增 2 行 + 1 cross-reference 段 + 1 双登记说明段）；**(5) T 层覆盖 D7-PL-T07**（plan/ 归属 S5 在 code-layout + a-registry 双登记）。 |


---

## D7-S10-A101: verify_decision_table kernel (M4, DM-20260705-005)

> **mups-verify-table-driven (DM-20260705-005) — MUPS 5 节点重构总图 M4 落地.**

**A 定位 (3 个 A)**: 决策表 kernel (verify_decision_table.go) + 3 verify 函数活动化

| A ID | 名称 | 角色 | Code Location | Notes |
|------|------|------|---------------|-------|
| **D7-S10-A101-F01** | **`verifyContext` 不可变结构体 (art/item/pl/contract/stats/id 6 字段)** | **A-BE** | **`sessionorchestrator/verify_decision_table.go:30-40`** (verifyContext struct) | detector 只读 ctx; 不可变 value type 保证并发安全 |
| **D7-S10-A101-F02** | **`VerdictTemplate` / `VerdictTrigger` / `VerifyDecisionTable` 3 struct + `applyDecisionTable` 顺序遍历** | **A-BE** | **`sessionorchestrator/verify_decision_table.go:42-110`** (3 struct + applyDecisionTable + buildVerdictFromTemplate + defaultVerdict) | 第一个 fired trigger 返回; 都不 fire → defaultVerdict (Pass 0.9); SourceID 仅在 ctx.id != "" 时注入 |
| **D7-S10-A101-F03** | **12 detector 命名函数 (5 artifact + 3 workItem overlay + 3 rollup + 1 rollup catch-all)** | **A-BE** | **`sessionorchestrator/verify_decision_table.go:130-300`** (detectXxx functions) | 命名清晰; 单一职责; 单元测试覆盖 fire true/false |
| **D7-S10-A101-F04** | **2 包级决策表 var (`artifactDecisionTable` 5 trigger + `rollupDecisionTable` 4 trigger)** | **A-BE** | **`sessionorchestrator/verify_decision_table.go:310-440`** (artifactDecisionTable + rollupDecisionTable) | 顺序显式声明; trigger 顺序是隐性契约 |
| **D7-S10-A101-F05** | **3 verify 函数走表驱动 (`verifyArtifact` / `verifyArtifactForWorkItemWithContract` / `verifyRollupArtifact`)** | **A-BE** | **`sessionorchestrator/{item_verify.go:30-110, rollup_verify.go:10-17}`** (3 verify 重构) | 49→9 行 + 54→35 行 + 47→9 行 = -95 行; 0 行为变化 (17 现有测试 0 修改 + 3 byte-equivalent 测试 17 组合 PASS) |
| **D7-S10-A101-F06** | **`_legacy_test.go` 旧实现保留 (build tag `legacy_verify`)** | **A-BE** | **`sessionorchestrator/verify_legacy_test.go`** (verifyArtifactLegacy + verifyArtifactForWorkItemWithContractLegacy + verifyRollupArtifactLegacy + verdictEqual + 3 byte-equivalent 测试) | 仅在 `-tags legacy_verify` 时编译; 下个 change (`mups-cleanup-legacy`) 删除 |

**A-门禁**:
- `internal/layers/orchestration/sessionorchestrator/verify_decision_table.go` 339 行 (含 12 detector + 2 decision table + applyDecisionTable + helpers + comments)
- 28 新测试 + 3 byte-equivalent 测试 (legacy_verify build tag) 全 PASS
- 17 现有测试 (item_verify + deliverable_verify + item_pipeline_rollup) 0 修改 PASS
- 0 行为变化; 决策表 trigger 顺序显式声明 (5 artifact + 4 rollup = 9 explicit)
