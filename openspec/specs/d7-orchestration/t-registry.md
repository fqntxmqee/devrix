# D7 Orchestration Domain — T 层测试点注册表

**Status:** Active
**Version:** 3.8.0
**Last Updated:** 2026-06-22
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `d7-domain.md`
**Spec:** `openspec/specs/d7-orchestration/spec.md`
**Complements:** `terminal-state-guide.md` · `observability-guide.md`
**Change:** 2026-06-20-devrix-context-budget-and-isolation-phase-b (devrix-context-budget-and-isolation / DM-20260620-001-B) — Phase B: AC6 + AC8 + AC9 SubTurn 3-mode dispatch + depth cap (D7-S2-A06-T14/T15/T16/T17); IMPLEMENTED 99→103, P0 70→74. **2026-06-20-devrix-error-handling-tier1-tier2** (DM-20260620-003) — error handling PR-A/PR-B/PR-C: invariant migration to shared/errors (D7-S2-A06-T24), task_manager.Create signature (`(*Task, error)`) (D7-S1-A02-T18), orchestrator.emitError sanitize+code (D7-S2-A02-T18), subagent stream sentinels (D7-S2-A06-T25/T26), retry nil-sentinel (D7-S2-A06-T27), resolveDelegateTaskID `(string, error)` (D7-S1-T19); IMPLEMENTED 109→116, P0 80→83. **2026-06-21-devrix-d7-error-aggregation-and-metrics** (DM-20260621-010) — D7 编排层错误聚合 + worktree 全链路 metrics: interrupt errors.Join aggregation (D7-S6-A11-T01/T02/T03), sandbox cleanup observability (D7-S6-A12-T04/T05/T06), forker errors.Join + 13 callers backward compat (D7-S6-A13-T07); IMPLEMENTED 116→123, P0 83→90. **2026-06-22-devrix-d7-metrics-and-concurrency-hardening** (DM-20260622-001) — D7 编排层 metric 命名 spec/code 对齐 + 并发硬化: dispatch_loop_wakeups / worker_panics 复数化 (D7-S6-A14-T01/T02), sandbox_exit_failed 跨域归属 D4 (D7-S6-A14-T03, D7-S6-A12-T01 OBSOLETE), state.cancels + state.handles markWaveDone 清空 (D7-S6-A14-T04), ConflictGuard hot path AllowAndRegister 原子调用 (D7-S6-A14-T05), CommandHandler emit select-default 防阻塞 (D7-S6-A14-T06); IMPLEMENTED 123→129, P0 90→96.

---

## Overview

D7 T 层测试点注册表。现行测试以 ORCH-S2-T* 注释标注，本文档统一映射为 D7-S*-T* 编号。遗留 ORCH ID 保留在「Legacy ID」列以便追溯。

> **按 S 分组摘要 / P0 Runbook / Trace 树：** 见 `observability-guide.md` §5–§7（本文保留全表登记）。

**状态：** IMPLEMENTED · PARTIAL · PLANNED

---

## D7-S4: Execution Flow

> **v1.1 closure (2026-06-15):** A04/A05 SpokeBridge wired（DM-018）；T 层增补 hubspoke 测试。

| T ID | Legacy ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|-----------|------|----------|-----------|--------|----------|
| D7-S4-T01 | ORCH-S2-T01 | WorkPlan.Snapshot 含 ExecutionFlow + 状态 | D7-S4-A02 | `orchestration/workplan/service_test.go` | IMPLEMENTED | P0 |
| D7-S4-T02 | — | Hub 双通道：WorkPlan + SessionQueue + IM | D7-S4-A01 | `orchestration/executionflow/hub/hub_test.go`；`tests/integration/d7/d7_hub_flow_test.go` | IMPLEMENTED | P0 |
| D7-S4-T03 | D4-S10-T04 | FlowStarted 触发 delegate-progress 入队 | D7-S4-A01-F02 | `orchestration/executionflow/hub/hub_test.go`；`tests/integration/d7/d7_hub_flow_test.go` | IMPLEMENTED | P0 |
| D7-S4-T04 | D4-S10-T07 | Snapshot 含 Task 投影（link_tasks） | D7-S1-A03-F02 | `orchestration/executionflow/hub/hub_test.go` | IMPLEMENTED | P0 |
| D7-S4-T05 | D4-S10-T05 | IMSink 发射 worker_progress 事件 | D7-S4-A03-F01 | `orchestration/imsink/gateway_test.go` | IMPLEMENTED | P0 |
| D7-S4-T06 | — | FlowToolCall 节流（throttle_ms） | D7-S4-A01-F04 | `orchestration/executionflow/hub/hub_test.go` | IMPLEMENTED | P1 |
| **D7-S4-T08** | — | **AgentBridge OnWorkerCompleted success/error** | **D7-S4-A04** | **`hubspoke/hubspoke_test.go::TestAgentBridge_OnWorkerCompleted_{success,error}`** | **IMPLEMENTED** | **P0** |
| **D7-S4-T09** | — | **SubQueryBridge PublishStarted/Completed/Failed** | **D7-S4-A05** | **`hubspoke/hubspoke_test.go::TestSubQueryBridge_Publish{Started,Completed,Failed}`** | **IMPLEMENTED** | **P0** |

---

## D7-S3: Wave Scheduler

| T ID | Legacy ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|-----------|------|----------|-----------|--------|----------|
| D7-S3-T01 | ORCH-S2-T10 | 6 ready subagent + 1 cursor 峰值并发≤5 | D7-S3-A01 | `orchestration/wavescheduler/scheduler_test.go` | IMPLEMENTED | P0 |
| D7-S3-T02 | ORCH-S2-T15 | 槽位释放后 ready Task 立即派发 | D7-S3-A01-F04 | `orchestration/wavescheduler/scheduler_test.go` | IMPLEMENTED | P0 |
| D7-S3-T03 | ORCH-S2-T17 | Plan DAG 仅 ready 节点被派发 | D7-S3-F03 | `orchestration/wavescheduler/scheduler_test.go`, `taskgraph_test.go` | IMPLEMENTED | P0 |
| D7-S3-T04 | ORCH-S2-T11 | upstream policy 收到 artifact，无 Leader 全量 | D7-S3-A02-F02 | `orchestration/wavescheduler/context_test.go`, `scheduler_orch_test.go` | IMPLEMENTED | P0 |
| D7-S3-T05 | ORCH-S2-T12 | fresh policy Messages 仅含 directive | D7-S3-A02-F01 | `orchestration/wavescheduler/context_test.go` | IMPLEMENTED | P0 |
| D7-S3-T06 | ORCH-S2-T13 | 同 conflict_group Task 不并行 | D7-S3-A03-F01 | `orchestration/wavescheduler/scheduler_orch_test.go` | IMPLEMENTED | P0 |
| D7-S3-T07 | ORCH-S2-T16 | cursor + claude-code 并行 file_scope 不交 | D7-S3-A03-F03 | `orchestration/wavescheduler/scheduler_orch_test.go` | IMPLEMENTED | P1 |
| D7-S3-T08 | ORCH-S2-T18 | wave 全完成返回全部 artifacts | D7-S3-A01-F03 | `orchestration/wavescheduler/scheduler_orch_test.go` | IMPLEMENTED | P1 |
| D7-S3-T09 | ORCH-S2-T19 | CancelWorker 槽位释放 status=cancelled | D7-S3-A01-F05 | `orchestration/wavescheduler/scheduler_test.go` | IMPLEMENTED | P0 |
| D7-S3-T10 | ORCH-S2-T20 | CancelAll 5 running 全部 terminal | D7-S3-A01-F05 | `orchestration/wavescheduler/scheduler_test.go` | IMPLEMENTED | P0 |
| D7-S3-T11 | ORCH-S2-T21 | CLI Worker cancel 进程终止 | D7-S3-F06 | `orchestration/wavescheduler/runners/agent_tool_orch_test.go`; `multiagent/external/cli_adapter_test.go` | IMPLEMENTED | P1 |
| **D7-S3-A01-F03-T01** | — | **AllowAndRegister no conflict → registered** | **D7-S3-A01-F03** | **`orchestration/wavescheduler/conflict_test.go::TestAllowAndRegister_NoConflict`** | **IMPLEMENTED** | **P0** |
| **D7-S3-A01-F03-T02** | — | **AllowAndRegister conflict group → blocked** | **D7-S3-A01-F03** | **`orchestration/wavescheduler/conflict_test.go::TestAllowAndRegister_ConflictGroup`** | **IMPLEMENTED** | **P0** |
| **D7-S3-A01-F03-T03** | — | **AllowAndRegister different group → allowed** | **D7-S3-A01-F03** | **`orchestration/wavescheduler/conflict_test.go::TestAllowAndRegister_DifferentGroup`** | **IMPLEMENTED** | **P0** |
| **D7-S3-A01-F03-T04** | — | **AllowAndRegister file scope intersection → blocked** | **D7-S3-A01-F03** | **`orchestration/wavescheduler/conflict_test.go::TestAllowAndRegister_FileScope`** | **IMPLEMENTED** | **P0** |
| **D7-S3-A01-F04-T01** | — | **emit pushes FlowEvent to sink AND channel** | **D7-S3-A01-F04** | **`sessionorchestrator/orchestrate_path.go::emit()`** | **IMPLEMENTED** | **P0** |
| **D7-S3-A01-F04-T02** | — | **emit tolerates nil sink gracefully** | **D7-S3-A01-F04** | **`sessionorchestrator/orchestrate_path.go::emit()`** | **IMPLEMENTED** | **P0** |
| **D7-S3-A01-IT01** | — | **Real WaveScheduler dispatch (3-task DAG)** | **D7-S3-A01** | **`tests/integration/d7/d7_wave_real_test.go::TestIntegration_D7WaveScheduler_RealDispatch`** | **IMPLEMENTED** | **P0** |
| **D7-S3-A01-IT02** | — | **Empty graph no-op** | **D7-S3-A01** | **`tests/integration/d7/d7_wave_real_test.go::TestIntegration_D7WaveScheduler_EmptyGraph`** | **IMPLEMENTED** | **P1** |
| **D7-S3-A01-IT03** | — | **ConflictGuard integration** | **D7-S3-A01** | **`tests/integration/d7/d7_wave_real_test.go::TestIntegration_D7WaveScheduler_ConflictGuard`** | **IMPLEMENTED** | **P0** |

---

## D7-S1: Work Model

> **v1.1 closure (2026-06-15):** 写模型迁入 `internal/layers/orchestration/workmodel/`。D7-S1-T01..T05 路径从 `contextengine/tasks/` 更新为 `workmodel/`。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| D7-S1-T01 | Task create 生成唯一 ID | D7-S1-A02-F01 | `workmodel/task_manager_test.go::TestTaskManager_Create` | IMPLEMENTED | P0 |
| D7-S1-T02 | Task 依赖 blocked_by 正确 | D7-S1-A02-F03 | `workmodel/task_manager_test.go::TestTaskManager_Dependency` | IMPLEMENTED | P0 |
| D7-S1-T03 | DiskStore v2 持久化恢复 | D7-S1-A02-F05 | `workmodel/disk_store_test.go::TestTaskManager_disk_persist_and_list_consistent`；`tests/integration/d7/d7_workmodel_test.go` | IMPLEMENTED | P0 |
| D7-S1-T04 | ListReadyTasks 仅返回无阻塞任务 | D7-S1-A02-F04 | `workmodel/task_manager_test.go::TestTaskManager_List` | IMPLEMENTED | P1 |
| D7-S1-T05 | FlowEvent link_tasks 状态联动 | D7-S1-A02-F06 | `orchestration/executionflow/hub/hub_test.go` | IMPLEMENTED | P1 |
| D7-S1-T06 | CreateWorkPlan DAG 校验 | D7-S1-A01-F02 | `decisionplanning/decomposer_test.go::TestTaskDecomposer_validateGraph` | IMPLEMENTED | P1 |
| D7-S1-T07 | BackgroundRun 注册与 QueryWorkPlan 可见 | D7-S1 | `sessionorchestrator/entry_test.go`; `contextengine/nested/background_*_test.go` | IMPLEMENTED | P1 |
| D7-S1-T08 | Task 非法状态转换拒绝 | D7-S1-A02-F02 | `workmodel/task_manager_test.go::TestIsLegalTransition`, `TestTaskManager_UpdateStatus_IllegalTransition`, `TestTaskManager_UpdateStatus_LegalTransitions` | IMPLEMENTED | P2 |
| **D7-S1-T09** | **WorkTree EnsureGoal 单 session 单根** | **D7-S1-A02** | **`workmodel/work_tree_test.go`** | **IMPLEMENTED** | **P0** |
| **D7-S1-T10** | **DiskWorkItemStore v2 迁移 + 原子 Save** | **D7-S1-A02-F05** | **`workmodel/work_tree_test.go`; `cross_session_test.go`** | **IMPLEMENTED** | **P0** |
| **D7-S1-T11** | **GetFocus 确定性 tiebreak** | **D7-S1-A02** | **`workmodel/work_tree_test.go::TestWorkTree_GetFocusTiebreak`** | **IMPLEMENTED** | **P1** |
| **D7-S1-T12** | **RunRef terminal → WorkItem status 同步** | **D7-S1-A02** | **`runregistry/spawn_test.go::TestSpawnForWorkItem_SyncTerminal`** | **IMPLEMENTED** | **P1** |
| **D7-S1-T13** | **跨 session FindByItemID** | **D7-S1-A02** | **`workmodel/cross_session_test.go`** | **IMPLEMENTED** | **P2** |
| **D7-S1-T14** | **DecomposeChildren 深度上限** | **D7-S1-A02** | **`workmodel/decompose_test.go::TestDecomposeChildren_DepthLimit`** | **IMPLEMENTED** | **P1** |
| **D7-S1-T15** | **Decompose 24h 频率上限 (5/kind/session)** | **D7-S1-A02** | **`workmodel/decompose_test.go::TestDecomposeChildren_DailyLimit`** | **IMPLEMENTED** | **P1** |
| **D7-S1-T16** | **ResolveHint 高 uncertainty decompose 引导** | **D7-S1-A02** | **`workmodel/decompose_test.go::TestResolveHint_HighUncertainty`** | **IMPLEMENTED** | **P1** |
| **D7-S1-T17** | **RunTurn blocking await running children** | **D7-S1-A02** | **`workmodel/resolve_await_test.go::TestAwaitRunningChildren_BlocksUntilTerminal`** | **IMPLEMENTED** | **P1** |
| **D7-S1-T18** | **TaskManager.Create returns `(*Task, error)` instead of silent nil (DM-20260620-003 PR-C H3)** | **D7-S1-A02-F01** | **`workmodel/task_manager_test.go::TestTaskManager_Create`; `cli_commands.go`; `tool_suite.go`** | **IMPLEMENTED** | **P0** |
| **D7-S1-T19** | **resolveDelegateTaskID returns `(string, error)` so delegate tools surface creation failure** | **D7-S1-A02-F01** | **`delegatetools/delegate_tools.go`; `tests/integration/d7/d7_hub_flow_test.go`** | **IMPLEMENTED** | **P1** |
| **D7-S3-T12** | **OrchestratePath SyncWaveNodes 挂树** | **D7-S3-A01** | **`sessionorchestrator/orchestrate_path.go`; bootstrap wiring** | **IMPLEMENTED** | **P1** |

---

## D7-S5: Decision & Planning

> **v1.1 closure (2026-06-15):** D7-S5-T04/T05 由 PLANNED 升为 IMPLEMENTED（Phase H/K）。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| D7-S5-T01 | PlanMode inactive→active 转换 | D7-S1-A04-F01 | `workmodel/plan_mode_test.go` 或 `task_manager_test` | IMPLEMENTED | P1 |
| D7-S5-T02 | PlanAgent 只读模式拒绝写操作；工具白名单不含 write/edit/bash | D7-S5-A04 | `workmodel/plan_agent_whitelist_test.go`（10 ACs）；`tests/integration/d7/d7_workmodel_test.go` | IMPLEMENTED | P0 |
| D7-S5-T03 | ClassifyIntent 规则高置信 → simple | D7-S5-A01 | `decisionplanning/classifier_test.go` | IMPLEMENTED | P0 |
| **D7-S5-T04** | **SynthesizeTaskGraph 产出有效 DAG** | **D7-S5-A02** | **`decisionplanning/decomposer_test.go::TestTaskDecomposer_SynthesizeTaskGraph`** | **IMPLEMENTED** | **P1** |
| **D7-S5-T05** | **SelectExecutor explore→D2 execute→D4** | **D7-S5-A03** | **`decisionplanning/executor_test.go::TestExecutorSelector_SelectExecutor`** | **IMPLEMENTED** | **P1** |
| D7-S5-T06 | Command-first：`/plan` 不触发 LLM Classify | D7-S5-A01 | `decisionplanning/{classifier,shadow_classifier}` + `sessionorchestrator/orchestrator_test.go`；`tests/integration/d7/d7_fastpath_test.go` | IMPLEMENTED | P0 |
| D7-S5-T07 | Tail-only LLM classify shadow（rule 未命中时异步 LLM，结果只入 metric） | D7-S5-A05 | `decisionplanning/shadow_classifier_test.go` | IMPLEMENTED | P0 |
| D7-S5-A01-T01 | 规则分类置信度阈值验证（screening 可重复性） | D7-S5-A01 | `decisionplanning/classifier_test.go::TestRuleClassifier_ExactConfidenceValues`, `TestRuleClassifier_ConfidenceDeterminism`, `TestRuleClassifier_ConfidenceRange`; `sessionorchestrator/orchestrator_test.go::TestSessionOrchestrator_FastPathConfidence{Below,Above}Threshold` | IMPLEMENTED | P0 |
| D7-S5-A01-T02 | Command-first 优先于 LLM 分类（用户显式策略优先） | D7-S5-A01 | `decisionplanning/classifier_test.go` | IMPLEMENTED | P0 |
| **D7-S5-A02-T01** | **SynthesizeTaskGraph 吸收 Explore Workers FlowEvent 产出有效 DAG** | **D7-S5-A02** | **`decisionplanning/decomposer_test.go::TestTaskDecomposer_SynthesizeTaskGraph`** | **IMPLEMENTED** | **P1** |
| **D7-S5-A02-T02** | **decomposeGoal 规则版：goal → sub_goal → DAG** | **D7-S5-A02-F01** | **`decisionplanning/decomposer_test.go::TestTaskDecomposer_decomposeGoal`** | **IMPLEMENTED** | **P1** |
| **D7-S5-A03-T01** | **MatchExecutorByTaskType：worker_type → D2/D4** | **D7-S5-A03-F01** | **`decisionplanning/executor_test.go::TestExecutorSelector_MatchExecutorByTaskType`** | **IMPLEMENTED** | **P1** |
| **D7-S5-A03-T02** | **CheckExecutorAvailability：executor 状态查询** | **D7-S5-A03-F02** | **`decisionplanning/executor_test.go::TestExecutorSelector_CheckExecutorAvailability`** | **IMPLEMENTED** | **P1** |
| **D7-S5-A03-T03** | **LLM Decomposer 解析 JSON DAG → wavescheduler.TaskNode（含 7 sub-cases）** | **D7-S5-A03-F03** | **`decisionplanning/llm_decomposer_test.go`（happy / bad JSON / enum coercion / unknown deps / extractJSON 6 case / nil LLM / SynthesizeTaskGraph routing）** | **IMPLEMENTED** | **P1** |
| **D7-S5-A02-F01-T01** | — | **ValidateToolCall: whitelist tool passes** | **D7-S5-A02-F01** | **`workmodel/plan_agent_whitelist_test.go::TestValidateToolCall_Allowed`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F01-T02** | — | **ValidateToolCall: forbidden tool rejected** | **D7-S5-A02-F01** | **`workmodel/plan_agent_whitelist_test.go::TestValidateToolCall_Forbidden`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F01-T03** | — | **ValidateToolCall: unknown tool rejected** | **D7-S5-A02-F01** | **`workmodel/plan_agent_whitelist_test.go::TestValidateToolCall_Unknown`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F01-T04** | — | **ValidateToolCall: nil receiver safe** | **D7-S5-A02-F01** | **`workmodel/plan_agent_whitelist_test.go::TestValidateToolCall_NilReceiver`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F02-T01** | — | **PlanMode.Enter: nil LLM returns ErrLLMNotConfigured** | **D7-S5-A02-F02** | **`workmodel/plan_mode.go::Enter()`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F02-T02** | — | **PlanMode.Enter: valid LLM succeeds** | **D7-S5-A02-F02** | **`workmodel/plan_mode.go::Enter()`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F05-T01** | — | **Config struct: PlanModeApproveGate field removed** | **D7-S5-A02-F05** | **`orchtypes/config.go`, `shared/config/coordinator.go`, `shared/config/loader.go`, `bootstrap/wire_coordinator.go`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F05-T02** | — | **Default config: no PlanModeApproveGate reference** | **D7-S5-A02-F05** | **`orchtypes/config.go::DefaultConfig()`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-IT01** | — | **LLM Decomposer end-to-end (JSON DAG → WaveScheduler)** | **D7-S5-A02** | **`tests/integration/d7/d7_llm_decomposer_test.go::TestIntegration_D7LLMDecomposer_EndToEnd`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-IT02** | — | **LLM Decomposer fallback on invalid JSON** | **D7-S5-A02** | **`tests/integration/d7/d7_llm_decomposer_test.go::TestIntegration_D7LLMDecomposer_FallbackOnInvalidJSON`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-IT03** | — | **LLM Decomposer empty task list** | **D7-S5-A02** | **`tests/integration/d7/d7_llm_decomposer_test.go::TestIntegration_D7LLMDecomposer_EmptyTaskList`** | **IMPLEMENTED** | **P1** |
| **D7-S5-A02-IT04** | — | **LLM Decomposer no JSON in response** | **D7-S5-A02** | **`tests/integration/d7/d7_llm_decomposer_test.go::TestIntegration_D7LLMDecomposer_NoJSONInResponse`** | **IMPLEMENTED** | **P1** |
| **D7-S5-A04-T01** | **turn_adapter.PersistTurn 提交 req.Messages 到 D2 内存（DM-20260617-003 d7-turn-history-persist）** | **D7-S5-A04** | **`internal/bootstrap/turn_adapter_persist_test.go::TestPersistTurn_{WritesMessagesToD2Memory,FullRound,NilEngine,AppendError}`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A04-T02** | **三轮同 session 连续 PersistTurn → Prepare 返回全历史** | **D7-S5-A04** | **`tests/integration/d7/turn_history_persist_test.go::TestTurnHistory_ThreeTurns`** | **IMPLEMENTED** | **P0** |

---

## D7-S6: Error Aggregation & Metrics

> **v3.8 closure (2026-06-21):** `devrix-d7-error-aggregation-and-metrics` (DM-20260621-010) — 取代 `interrupt.go` 三步 cancel 的「all warn + nil」反模式，引入 `errors.Join` 聚合与原子指标；消除 `_ = Sandbox.Exit(...)` 三处 silent swallow；新增 WaveScheduler 4 字段与 TaskManager / Executor metrics 结构。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| **D7-S6-A11-T01** | **HandleInterrupt: 3 步 cancel 全失败 → errors.Join 包含 3 个 wrapped error；errors.Is 命中每个** | **D7-S6-A11** | **`sessionorchestrator/interrupt_test.go::TestHandleInterrupt_AllStepsFail_JoinsErrors`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A11-T02** | **HandleInterrupt: 1 步失败 → 返回非 nil + 仅含失败 step 的 wrapped error** | **D7-S6-A11** | **`sessionorchestrator/interrupt_test.go::TestHandleInterrupt_PartialFailure_ReturnsPartialErr`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A11-T03** | **HandleInterrupt: nil Metrics 仍返回 errors.Join（向后兼容）** | **D7-S6-A11** | **`sessionorchestrator/interrupt_test.go::TestHandleInterrupt_NilMetrics`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A12-T01** | **[OBSOLETE 2026-06-22, see D7-S6-A14-T03 + D4-S6-A12-Txx] Sandbox Exit 失败 metric 由 D4 multiagent/execute 提供，D7 spec 不重复声明** | **D7-S6-A12** | **跨域 reference to D4 executor metrics** | **OBSOLETE** | **P0** |
| **D7-S6-A12-T02** | **Worker panic → SchedulerMetrics.WorkerPanics +1（spec 名 "worker_panics"，DM-20260622-001 A1 后对齐）** | **D7-S6-A12** | **`wavescheduler/scheduler_metrics_test.go::TestWaveScheduler_WorkerPanicsMetric` + `d7_s6_a14_test.go::TestD7S6A14T02_WorkerPanics_SpecAlignedPlural`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A12-T03** | **taskCtx leak → SchedulerMetrics.TaskCtxLeaked +1** | **D7-S6-A12** | **`wavescheduler/scheduler_test.go::TestWaveScheduler_TaskCtxLeakMetric`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A12-T04** | **Forker: sandbox Exit 失败 → SandboxExitFailed 计数器 +1 + slog.Warn（13 调用方兼容）** | **D7-S6-A12** | **`multiagent/provision/freefork/forker_test.go::TestFork_SandboxExitFailure_RecordsMetric`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A12-T05** | **dispatchLoop wakeup → SchedulerMetrics.DispatchLoopWakeups +1（spec 名 "dispatch_loop_wakeups"，DM-20260622-001 A1 后对齐）** | **D7-S6-A12** | **`wavescheduler/scheduler_metrics_test.go::TestWaveScheduler_DispatchLoopWakeupsMetric` + `d7_s6_a14_test.go::TestD7S6A14T01_DispatchLoopWakeups_SpecAlignedPlural`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A12-T06** | **TaskManager.publishCompletion panic → TaskManagerMetrics.PublishCompletionPanics +1 + slog.Error** | **D7-S6-A12** | **`workmodel/task_manager_metrics_test.go::TestTaskManagerMetrics_*`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A13-T07** | **DefaultForker: 多 fork 全失败 → errors.Join 包含每个 fork 的 wrapped error** | **D7-S6-A13** | **`multiagent/provision/freefork/forker_test.go::TestFork_AllFailuresJoined`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A14-T01** | **dispatchLoop wakeup incMetric 名对齐 spec 复数: "dispatch_loop_wakeups"** | **D7-S6-A14** | **`wavescheduler/d7_s6_a14_test.go::TestD7S6A14T01_DispatchLoopWakeups_SpecAlignedPlural`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A14-T02** | **Worker panic incMetric 名对齐 spec 复数: "worker_panics"** | **D7-S6-A14** | **`wavescheduler/d7_s6_a14_test.go::TestD7S6A14T02_WorkerPanics_SpecAlignedPlural`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A14-T03** | **sandbox_exit_failed 跨域归属：spec 标注 OBSOLETE + cross-ref D4-S6-A12-Txx** | **D7-S6-A14** | **spec.md D7-S6-A12-T01 标注 + t-registry 本表** | **IMPLEMENTED** | **P0** |
| **D7-S6-A14-T04** | **state.cancels + state.handles 在 markWaveDone 后清空（防长会话无界增长）** | **D7-S6-A14** | **`wavescheduler/d7_s6_a14_test.go::TestD7S6A14T04_StateCancels_{NilAfterWaveDone,NoLeakAcrossWaves}`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A14-T05** | **dispatchLoop hot path 用 AllowAndRegister 原子调用，关 TOCTOU 窗口** | **D7-S6-A14** | **`wavescheduler/d7_s6_a14_test.go::TestD7S6A14T05_DispatchLoop_HotPathUsesAllowAndRegister`** | **IMPLEMENTED** | **P0** |
| **D7-S6-A14-T06** | **CommandHandler emit 用 select-default 防 consumer 阻塞** | **D7-S6-A14** | **`sessionorchestrator/d7_s6_a14_t06_test.go::TestD7S6A14T06_CommandHandler_OutChannelFull_DropsEvent`** | **IMPLEMENTED** | **P0** |

> 配套 P1：WaveScheduler `WorkerPanics` / `TaskCtxLeaked` / `WaveReentryCancelled` / `DispatchLoopWakeups` 4 字段为 `wavescheduler/scheduler_metrics_test.go` 7 单元 + 端到端测试覆盖（panickingRunner / reentry / wakeup ticker）；`TestFork_Metrics_*` 3 场景覆盖 SandboxEnterFailed / FactoryCreateFailed / RollbackTriggered 触发路径。

---

## D7-S2: Session Orchestrator

> **v1.1 closure (2026-06-15):** D7-S2-A04 DispatchWorker wired（Phase DM-018）；D7-S2-A06/A07 wired（Phase DM-020）。T 层增补 hubspoke 测试。

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority |
|------|------|--------|-----------|--------|----------|
| D7-S2-T01 | ProcessMessage 为 D1 主入口 | D7-S2-A01 | `sessionorchestrator/orchestrator_test.go`；`tests/integration/d7/d7_fastpath_test.go` | IMPLEMENTED | P0 |
| D7-S2-T02a | FastPath proxy 开销 P99 ≤ 2ms（Classify 后） | D7-S2-A01-F02 | `sessionorchestrator/orchestrator_test.go` | IMPLEMENTED | P0 |
| D7-S2-T02b | 规则 ClassifyIntent P99 ≤ 1ms | D7-S2-A02 | `decisionplanning/classifier_test.go` | IMPLEMENTED | P0 |
| D7-S2-T02c | FastPath 端到端 P99 ≤ 2ms（command-first 全栈） | D7-S2-A01-F02 | `sessionorchestrator/orchestrator_test.go`；`tests/integration/d7/d7_fastpath_test.go` | IMPLEMENTED | P0 |
| D7-S2-T03 | OrchestratePath 按路由矩阵（v1.1.0+ 显式调 SynthesizeTaskGraph + WaveScheduler） | D7-S2-A01-F03 | `sessionorchestrator/orchestrate_path_test.go` (5 AC) | IMPLEMENTED | P0 |
| D7-S2-T04 | HandleInterrupt：Wave→D4→Process→stopped→TaskCancel | D7-S2-A03 | `sessionorchestrator/orchestrator_test.go`；`tests/integration/d7/d7_interrupt_test.go` | IMPLEMENTED | P0 |
| D7-S2-T05 | HandleInterrupt 幂等 | D7-S2-A03 | `sessionorchestrator/orchestrator_test.go` | IMPLEMENTED | P0 |
| D7-S2-A01-T03 | 禁止在 Worker terminal FlowEvent 前伪造 Task 进度（anti-fabrication commitment） | D7-S2-A01 | `sessionorchestrator/orchestrator_test.go::TestSessionOrchestrator_AntiFabrication_NoSyntheticProgress` | IMPLEMENTED | P0 |
| D7-S2-A01-T04 | IntentCommand 显式分发到 PlanCLI/CLICommands（v1.1.0+ orthogonal） | D7-S2-A01 | `sessionorchestrator/command_handler_test.go` (3 AC) | IMPLEMENTED | P0 |
| D7-S2-A01-T05 | IntentOrchestrate 走 SynthesizeTaskGraph + WaveScheduler（v1.1.0+ orthogonal） | D7-S2-A01 | `sessionorchestrator/orchestrate_path_test.go` (5 AC) | IMPLEMENTED | P0 |
| D7-S2-A01-T06 | IntentFast 保持 FastPath（v1.1.0+ orthogonal, 不回归） | D7-S2-A01 | `sessionorchestrator/orchestrator_test.go::TestSessionOrchestrator_ProcessMessage_FastPath` | IMPLEMENTED | P0 |
| D7-S2-A03-T01 | HandleInterrupt 中断顺序正确（可中断性承诺） | D7-S2-A03 | `sessionorchestrator/orchestrator_test.go`；`tests/integration/d7/d7_interrupt_test.go` | IMPLEMENTED | P0 |
| **D7-S2-A04-T01** | **DispatchWorker D4 enabled with leader** | **D7-S2-A04** | **`hubspoke/hubspoke_test.go::TestDispatcher_Dispatch_D4_enabled_withLeader`** | **IMPLEMENTED** | **P0** |
| **D7-S2-A04-T02** | **DispatchWorker D4 disabled falls back to D2 SubQuery** | **D7-S2-A04** | **`hubspoke/hubspoke_test.go::TestDispatcher_Dispatch_D4_disabled_fallsToD2`** | **IMPLEMENTED** | **P0** |
| **D7-S2-A04-T03** | **DispatchWorker async mode** | **D7-S2-A04** | **`hubspoke/hubspoke_test.go::TestDispatcher_Dispatch_D4_async`** | **IMPLEMENTED** | **P1** |
| **D7-S2-A03-F06-T01** | — | **LLMFallbackClassifier Deprecated marker** | **D7-S2-A03-F06** | **`decisionplanning/classifier_fallback.go`** | **IMPLEMENTED** | **P1** |
| **D7-S2-A03-F06-T02** | — | **ExecutorSelector Deprecated marker** | **D7-S2-A03-F06** | **`decisionplanning/executor.go`** | **IMPLEMENTED** | **P1** |
| **D7-S2-A06-IT01** | — | **Multi-turn tool conversation (2 LLM rounds)** | **D7-S2-A06** | **`tests/integration/d7/d7_multiturn_test.go::TestIntegration_D7FastPath_MultiTurnToolConversation`** | **IMPLEMENTED** | **P0** |
| **D7-S2-A06-IT02** | — | **MaxTurns cap enforcement** | **D7-S2-A06** | **`tests/integration/d7/d7_multiturn_test.go::TestIntegration_D7FastPath_MaxTurnsCap`** | **IMPLEMENTED** | **P1** |
| **D7-S2-A06-IT03** | — | **StopProcess during slow Turn** | **D7-S2-A06** | **`tests/integration/d7/d7_multiturn_test.go::TestIntegration_D7FastPath_ContextCancellation`** | **IMPLEMENTED** | **P1** |

### Turn Adapter LTL-Lite Hook (DM-20260618-007)

**Change:** devrix-tools-terminal-architecture (DM-20260618-007) — LTL-Lite runtime check + CI lint + turn_adapter HookRegistry (PERMISSION-GATE-1-T01/T02/T03) + BackgroundTaskSurface ToolEventStream (D7-S2-A08-T01)

### Context Budget Phase A — Turn Loop Integration (DM-20260620-001)

> **devrix-context-budget-and-isolation (DM-20260620-001) — Phase A 落地。**
> AC1+AC2+AC4 turn loop 集成（D2-S17-A05/S17-A06/S15-A08 helpers 消费方）：
> tool result cap + assistant fold + per-iter audit + bootstrap wiring。
> D2 t-registry 持有 helper 自身的 T 点（T01-T05/T01-T03/T01-T05）；
> 本表持有 turn loop 集成 + bootstrap 接线 T 点（T11-T13）。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| **D7-S2-A06-T11** | **AC1+AC2 turn loop integration: tool result cap + assistant fold wired into RunTurn** | **D7-S2-A06** | **`orchestration/turn/orchestrator_toolcap_test.go::TestOrchestrator_BuildToolResultMsgWithCap_*`, `TestOrchestrator_BuildAssistantToolCallMsgFolded_*`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D7-S2-A06-T12** | **AC4 per-iter audit + proactive fold + span attrs + slog** | **D7-S2-A06** | **`orchestration/turn/orchestrator_toolcap_test.go::TestOrchestrator_RunTokenAudit_*`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D7-S2-A06-T13** | **WireD7 bootstrap constructs ToolResultStore** | **D7-S2-A06** | **`bootstrap/wire_coordinator.go::NewOrchestrator(OrchestratorDeps{ToolResultStore: …})`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |

### Context Budget Phase B — SubTurn 3-Mode Dispatch (DM-20260620-001-B)

> **devrix-context-budget-and-isolation (DM-20260620-001-B) — Phase B 落地（待 B.5 验证 + S6 归档）。**
> AC6+AC8+AC9+AC10+AC11a SubTurn 3-mode 派发（brief/fork/full）：
> `SubTurnRunner` 按 `req.Mode` 选 `applyMode` 分支；empty → `SubagentConfig.DefaultMode`；
> `LegacyMode` 覆盖 `DefaultMode`；`Depth >= MaxDepth` 拒绝；
> fork mode 走 `conversation.BuildForkedMessages` 保证 prefix byte-level stable
> （cache anchor for future Anthropic `cache_control`）；
> `delegate_*` / `free_fork` LLM tool schema 暴露 `mode` 字段。
> D4 t-registry 持有 schema 侧的 T 点（T01/T02）；
> D2 t-registry 持有 BuildForkedMessages 自身的 T 点（T06/T07/T08）；
> 本表持有 SubTurnRunner 派发 + depth + default-mode T 点（T14/T15/T16/T17）。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| **D7-S2-A06-T14** | **AC6 brief mode drops parent history: LLM sees only last user message** | **D7-S2-A06** | **`orchestration/turn/subturn_test.go::TestSubTurnRunner_BriefMode_PreloadedMessagesNil`** | **IMPLEMENTED (DM-20260620-001-B)** | **P0** |
| **D7-S2-A06-T15** | **AC8+AC11a fork mode = BuildForkedMessages (cache-friendly prefix) + full mode = legacy parity** | **D7-S2-A06** | **`orchestration/turn/subturn_test.go::TestSubTurnRunner_ForkMode_DispatchesAsFork`, `TestSubTurnRunner_FullMode_BackwardCompat`, `TestSubTurnRunner_FullMode_EquivalentToLegacy`, `TestSubTurnRunner_FullMode_EmptyParent`; `subturn_fork_test.go::TestSubTurnRunner_ForkSiblingPrefixStable`, `TestSubTurnRunner_ForkPrefix_ContainsPlaceholder`** | **IMPLEMENTED (DM-20260620-001-B)** | **P0** |
| **D7-S2-A06-T16** | **AC9 depth limit: `Depth >= MaxDepth` rejected before LLM call; `Depth = MaxDepth-1` allowed** | **D7-S2-A06** | **`orchestration/turn/subturn_test.go::TestSubTurnRunner_DepthLimit_{Equals,Exceeds,BoundaryAtMaxMinus1}`** | **IMPLEMENTED (DM-20260620-001-B)** | **P0** |
| **D7-S2-A06-T17** | **AC6 default mode: empty `req.Mode` → `SubagentConfig.DefaultMode`; `LegacyMode` overrides `DefaultMode`; invalid mode rejected** | **D7-S2-A06** | **`orchestration/turn/subturn_test.go::TestSubTurnRunner_DefaultModeFromConfig`, `TestSubTurnRunner_DefaultModeBrief`, `TestSubTurnRunner_InvalidModeRejected`** | **IMPLEMENTED (DM-20260620-001-B)** | **P0** |

### Context Budget Phase C — Nested Branch Budget Injection (DM-20260620-002)

> **devrix-context-budget-phase-c-nested (DM-20260620-002) — Phase C 落地。**
> `runLoop` nested branch (`orchestrator.go:221-268`) skips `o.context.Prepare`,
> leaving `prepared.MaxContextTokens=0` and making every Phase A budget control
> (runTokenAudit + ShouldFoldProactively + tool result cap + budgetTracker) a
> no-op. The fix threads `maxContextTokens` from `SubTurnRequest` →
> `TurnRequest` → nested-branch read, with fallback to `o.maxContextTokens`
> (Phase A wiring) for legacy callers.
>
> Bug → fix: 4-parallel deep-review sub-agents (e.g. "review D1/D2/D3/D7" after
> 10 tool rounds each) accumulated ~80K-char oversized read_file results, blew
> past the LLM context window, and were rejected. After Phase C, the audit
> fires on the nested branch and the largest assistant message is folded
> (80000→1186 chars).
>
> D2 t-registry holds `enforce.Run` pass-through T 点（T09/T10）。
> D7 t-registry holds TurnRequest + nested-branch read + integration
> verification T 点（T18/T19/T20/T21/T22/T23）。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| **D7-S2-A06-T18** | **AC1 `TurnRequest.MaxContextTokens` 字段添加 + 注释（nested 分支可显式注入 budget）** | **D7-S2-A06** | **`orchestration/turn/contracts.go::TurnRequest.MaxContextTokens`** | **IMPLEMENTED (DM-20260620-002)** | **P0** |
| **D7-S2-A06-T19** | **AC1 `runLoop` nested 分支读取 `req.MaxContextTokens`，fallback `o.maxContextTokens`** | **D7-S2-A06** | **`orchestration/turn/orchestrator.go:271-274`** | **IMPLEMENTED (DM-20260620-002)** | **P0** |
| **D7-S2-A06-T20** | **AC1 `SubTurnRunner.Cfg.MaxContextTokens` + `bootstrap/wire_coordinator.go` 注入全局 config** | **D7-S2-A06** | **`orchestration/turn/subturn.go::SubTurnConfig.MaxContextTokens`, `bootstrap/wire_coordinator.go:179` (NewSubTurnRunner 调用)** | **IMPLEMENTED (DM-20260620-002)** | **P0** |
| **D7-S2-A06-T21** | **AC1 nested-branch 显式注入路径：80K assistant + 96K system + 32K budget → audit 触发 + fold 80000→1186** | **D7-S2-A06** | **`orchestration/turn/orchestrator_test.go::TestOrchestrator_RunTurn_NestedBranch_BudgetInjection_DM_20260620_002`** | **IMPLEMENTED (DM-20260620-002)** | **P0** |
| **D7-S2-A06-T22** | **AC1 nested-branch fallback 路径：req=0 → `o.maxContextTokens`（Phase A wiring 32000）audit 仍触发** | **D7-S2-A06** | **`orchestration/turn/orchestrator_test.go::TestOrchestrator_RunTurn_NestedBranch_FallbackToDeps_PhaseA_AC1_DM_20260620_002`** | **IMPLEMENTED (DM-20260620-002)** | **P0** |
| **D7-S2-A06-T23** | **AC2 4-parallel deep review 端到端：4 路 `SubQuery.Run` 并行（80K+96K+32K）全部完成，capture adapter 验证 max 消息 1186 chars (folded)** | **D7-S2-A06** | **`tests/integration/d7/d7_nested_budget_test.go::TestIntegration_D7NestedBudget_4ParallelDeepReview`** | **IMPLEMENTED (DM-20260620-002)** | **P0** |

### Error Handling Tier 1+2 (DM-20260620-003)

> **devrix-error-handling-tier1-tier2 (DM-20260620-003) — 错误处理 PR-A (Tier 1) + PR-B (C2 type merge) + PR-C (H3+M1..M4) 落地。**
>
> **Tier 1 (PR-A)**: `SanitizeForUser` redacts API keys/tokens/paths before IM render;
> `emitError` signature gains variadic `code ...string` for `error_code` metadata;
> `subturn.go` adds `ErrSubagentStreamError` + `ErrSubagentStreamClosed` (codes
> AGT_STREAM_5013/5014) so callers retain error type information;
> `retry.go:91` nil-sentinel fix wraps a real `errors.New(...)` cause instead of `nil`.
>
> **Tier 2 (PR-B)**: `LLMError` becomes a type alias for `*SentinelError`;
> all factories return `*SentinelError`; `SentinelError.Error()` falls back to
> inner Err then Code (preserving LLMError's permissive semantics); `migrate.go`
> provides build-time guard + deprecated helpers.
>
> **Tier 2 (PR-C)**: `TaskManager.Create` returns `(*Task, error)` (silent
> fallback fix); `turn_adapter.ErrInvariantViolation` migrated to
> `sharederrors.ErrInvariantViolation` (code AGT_INVARIANT_5013) with deprecated
> alias; `classifyAndWrap` + `Gateway.classify` take ctx so downstream spans
> can read cached Classification via `ClassifyResultFromCtx`;
> `Observability.Shutdown` uses `errors.Join` + `%w` so callers retain typed chain;
> `decisionplanning.LLMFallbackClassifier.Classify` logs `slog.Warn` when LLM
> classify fails (was silent).
>
> Cross-cutting docs: `docs/error-handling.md`. Shared spec lives at
> `internal/shared/errors/` (no `shared-errors` D-domain — cross-cutting per
> `openspec/specs/architecture/cross-domain-boundaries.md`).

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| **D7-S2-A06-T24** | **AC6 `turn_adapter.ErrInvariantViolation` migrated to `sharederrors.ErrInvariantViolation` (code AGT_INVARIANT_5013); `Prepare` wraps via `NewInvariantViolationError`; legacy alias kept** | **D7-S2-A06** | **`internal/layers/orchestration/turn_adapter/ltl_hook_test.go::TestHookRegistry_Prepare_*` (still match via alias)** | **IMPLEMENTED (DM-20260620-003)** | **P1** |
| **D7-S2-A06-T25** | **AC2 `subturn.go:collectSubTurnResult` error case: when event has `error_code` metadata, wrap via `derrors.WithCode(code, ...)`; otherwise fall back to `NewSubagentStreamError`** | **D7-S2-A06** | **`internal/shared/errors/subturn.go`; `internal/layers/orchestration/turn/subturn.go`** | **IMPLEMENTED (DM-20260620-003)** | **P1** |
| **D7-S2-A06-T26** | **AC2 `subturn.go:collectSubTurnResult` channel-closed-without-complete branch returns `NewSubagentStreamClosedError()` (code AGT_STREAM_5014)** | **D7-S2-A06** | **`internal/shared/errors/subturn.go::NewSubagentStreamClosedError`** | **IMPLEMENTED (DM-20260620-003)** | **P1** |
| **D7-S2-A06-T27** | **AC2/H3 `protect/retry.go:91` nil-sentinel fix: defensive fallback wraps `errors.New("retry loop completed without recording an error: ...")` instead of `nil`** | **D7-S2-A06** | **`internal/layers/llmgateway/protect/retry.go`** | **IMPLEMENTED (DM-20260620-003)** | **P0** |
| **D7-S2-A02-T18** | **AC1 `orchestrator.emitError` variadic `code ...string` adds `Metadata["error_code"]`; all 5 call sites pass `SanitizeForUser(err)` + `ErrorCode(err)`** | **D7-S2-A02** | **`internal/layers/orchestration/turn/orchestrator.go::emitError` + call sites (256, 292, 371, 428, 581)** | **IMPLEMENTED (DM-20260620-003)** | **P0** |

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| **PERMISSION-GATE-1-T01** | LTL-Lite runtime check (ltllite.Check + HookRegistry.Prepare) | turn_adapter | `internal/layers/orchestration/turn_adapter/ltl_hook_test.go::TestHookRegistry_Prepare_*` | **IMPLEMENTED** | **P0** |
| **PERMISSION-GATE-1-T02** | CI lint 静态校验 (ci-lint-invariant 扫描 _invariant.go) | tools/ | `tools/ci-lint-invariant/main_test.go` | **IMPLEMENTED** | **P0** |
| **PERMISSION-GATE-1-T03** | turn_adapter HookRegistry Prepare/BeforeExecute 定向重检 | turn_adapter | `internal/layers/orchestration/turn_adapter/ltl_hook_test.go::TestHookRegistry_BeforeExecute_*` | **IMPLEMENTED** | **P0** |
| **D7-S2-A08-T01** | ToolEventStream context 推送 + BackgroundTaskSurface 集成 | turn | `internal/layers/orchestration/turn/tool_stream_test.go` | **IMPLEMENTED** | **P0** |

### Loop-First Routing L5 (DM-20260616-002)

| L5 ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|-------|------|----------|-----------|--------|----------|
| **D7-S2-L5-01** | 问候语 Turn 不触发 Wave（无 plan_formed/wave_started） | D7-S2-A01 | `tests/integration/d7/d7_loop_first_test.go::TestIntegration_D7LoopFirst_GreetingNoWave`；`decisionplanning/classifier_test.go::TestRuleClassifier_Classify_LoopFirstDefault` | IMPLEMENTED | P0 |
| **D7-S2-L5-02** | delegate_wave tool 门控 OrchestratePath | D7-S2-F02 | `sessionorchestrator/turn_tools_test.go`；`tests/integration/d7/d7_loop_first_test.go::TestIntegration_D7LoopFirst_DelegateWaveTool` | IMPLEMENTED | P0 |
| **D7-S2-L5-03** | Slash 命令零 LLM | D7-S2-A01 | `tests/integration/d7/d7_orthogonal_dispatch_test.go::TestIntegration_D7ProcessMessage_CommandBypassesLLM` | IMPLEMENTED | P0 |
| **D7-S2-L5-04** | EngineEvent 单投递（无 sink mirror） | D7-S2-F03 | `sessionorchestrator/orchestrator_test.go`；`capture/agent_route.go` | IMPLEMENTED | P0 |
| **D7-S2-L5-05** | enter_plan_mode tool | D7-S2-F02 | `sessionorchestrator/turn_tools_test.go` | IMPLEMENTED | P1 |
| **D7-S2-L5-06** | rule_orchestrate 回滚（threshold 降级） | D7-S2-F01 | `orchtypes/routing_test.go`；`sessionorchestrator/orchestrator_test.go` | IMPLEMENTED | P1 |

---

## Cross-Domain (D7 契约)

| T ID | 描述 | 归属 | Test 位置 | Status | Priority |
|------|------|------|-----------|--------|----------|
| D7-D1-T01 | D1 调用 D7 而非 D2（d7_enabled） | D7-S2 | `tests/integration/d7/d7_entry_test.go`（WireD7 全栈）；`sessionorchestrator/entry_test.go` | IMPLEMENTED | P0 |
| D7-D4-T01 | D2 enforce 无 delegate hooks | D7-S2 | `internal/lint/layer/d2_thin_test.go` | IMPLEMENTED | P0 |
| D7-D6-T01 | D6 校验编排决策（advisory）+ `orchestration.d6.validation.{pass,fail,timeout,error}` metric | D7-S5 | `internal/layers/orchestration/sessionorchestrator/validation_metrics_test.go` | IMPLEMENTED | P1 |
| D7-D6-T02 | D6 校验超时 50ms 视为 pass | D7-S5 | `internal/layers/orchestration/sessionorchestrator/entry_test.go` | IMPLEMENTED | P2 |
| D7-D6-T03 | 4 counter 注入 + result.Pass 分流 | D7-S5 | `internal/layers/orchestration/sessionorchestrator/validation_metrics_test.go` | IMPLEMENTED | P0 |
| D7-D6-T04 | timeout_rate > 5% 触发 AlertHook（5min 滑窗） | D7-S5 | `internal/layers/orchestration/sessionorchestrator/validation_metrics_test.go` | IMPLEMENTED | P0 |
| D7-D6-T05 | panic-recovered 计入 error 路径 | D7-S2 | `internal/layers/orchestration/sessionorchestrator/validation_metrics_test.go` | IMPLEMENTED | P0 |
| D7-D6-T06 | nil validator 与 nil metrics 都降级 no-op | D7-S2 | `internal/layers/orchestration/sessionorchestrator/validation_metrics_test.go` | IMPLEMENTED | P0 |
| D7-MIG-T01 | D7-only ingress × plan.enabled 组合回归 | D7-S2 | `tests/integration/d7/d7_entry_test.go::TestIntegration_D7Entry_PlanModeStillUsesD7Path`；`coordinator_matrix_test.go` | IMPLEMENTED | P0 |
| D7-THIN-T01 | D2 contextengine 无 orchestration import | D2 瘦身 | `internal/lint/layer/d2_thin_test.go` | IMPLEMENTED | P0 |
| D7-THIN-T02 | ~~loop.go Run ≤200 行~~ | D2 瘦身 | **REMOVED**（`query/loop.go` 已删，DM-20260618-010） | REMOVED | P0 |

---

## D1 集成（IM 渲染）

| T ID | Legacy ID | 描述 | 归属 | Test 位置 | Status | Priority |
|------|-----------|------|------|-----------|--------|----------|
| D7-S4-T07 | ORCH-S2-T14 | 每 Task 独立双区块 IM 卡流式 | D1-S8 + D7-S4 | `communication/channel/adapters/feishu_worker_card_test.go` | IMPLEMENTED | P0 |

---

## D7-S2 Turn Leader（DM-020 v1.0 Registry）

> **v3.0 closure (2026-06-15):** v2.0-b/c/f 全部闭环。A06-T01..T04 + A07-T01..T02 全部 IMPLEMENTED。

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority |
|------|------|--------|-----------|--------|----------|
| D7-S2-A06-T01 | FastPath turn D2 then D3 in order | D7-S2-A06 | `orchestration/turn/orchestrator_test.go::TestOrchestrator_RunTurn_SingleTurn_NoTools` | IMPLEMENTED | P0 |
| D7-S2-A06-T02 | Cancel propagates to D3 stream and D2 tools | D7-S2-A06 | `orchestration/turn/orchestrator_test.go::TestOrchestrator_RunTurn_CancelBetweenTurns`, `TestOrchestrator_RunTurn_CancelBeforeLLM` | IMPLEMENTED | P0 |
| D7-S2-A06-T03 | Multi-turn tool_use loops under D7 | D7-S2-A06 | `orchestration/turn/orchestrator_test.go::TestOrchestrator_RunTurn_MultiTurn_ToolLoop` | IMPLEMENTED | P0 |
| D7-S2-A06-T04 | SubQuery nested turn uses same orchestrator | D7-S2-A06 | `orchestration/turn/orchestrator_test.go::TestOrchestrator_RunTurn_{SubQueryScope,SameOrchestratorForMainAndSubQuery}` | IMPLEMENTED | P0 |
| D7-S2-A07-T01 | Breaker open with no fallback returns error | D7-S2-A07 | `orchestration/turn/orchestrator_test.go::TestOrchestrator_RunTurn_LLMInvokeError`; `orchestration/turn/llm_test.go::TestGatewayInvoker_InvokeStream_BreakerOpen` | IMPLEMENTED | P0 |
| D7-S2-A07-T02 | StreamChat timeout propagates as EngineEvent | D7-S2-A07 | `orchestration/turn/llm_test.go::TestGatewayInvoker_InvokeStream_{ContextCanceled,ContextDeadlineExceeded}`, `TestOrchestrator_RunTurn_StreamTimeout_EngineEvent` | IMPLEMENTED | P0 |
| **D7-S2-A06-T09** | **D7 RunTurn never touches removed D2 QueryLoop** | **D7-S2-A06** | **`orchestration/turn/loop_legacy_test.go::TestOrchestrator_RunTurn_MainPathOnly`** | **IMPLEMENTED** | **P0** |
| **D7-S2-A06-T10** | **~~D2.QueryLoop legacy counter~~ REMOVED (DM-20260618-010)** | **D7-S2-A06** | **`contextengine/queryloop_removed_test.go::TestD2_NoQueryLoopProductionReferences`** | **IMPLEMENTED** | **P0** |

### Legacy T 映射（DM-020 — v1.0 Registry，v2.0 实施）

> v1.0：**不改**现有测试 `// T:` 注释。下表供追溯与新测试登记。

| Legacy T ID | Canonical T ID | Canonical S | 域 | 描述 |
|-------------|----------------|-------------|-----|------|
| D2-S16-A01-T01 | D7-S2-A06-T01 | S2 Turn | D7 | FastPath turn D2 then D3 |
| D2-S16-A01-T02 | D7-S2-A06-T02 | S2 Turn cancel | D7 | Cancel propagates |
| D2-S16-A01-T03 | D2-THIN-T01 | import lint | D2 | D2→D3 import 禁止 |
| D2-S10-A01-T34 | D7-S2-A06-T03 | multi-turn loop | D7 | Multi-turn tool_use |
| D2-S10-A01-T35~T42 | D2-S15/S18/S19-T* | 按机制拆分 | D2 | 保留 D2 域内 |
| （新增） | D7-S2-A07-T01 | RouteModel+Stream | D7 | Breaker sad path |
| （新增） | D7-S2-A07-T02 | StreamChat timeout | D7 | Timeout propagate |
| （新增） | D7-S2-A06-T04 | SubQuery nested | D7 | Nested turn |
| （新增） | D2-S15-A01-T10 | CompressHint no LLM | D2 | D2 不调 LLM |

---

## Statistics

| Total | IMPLEMENTED | PARTIAL | PLANNED | P0 |
|-------|-------------|---------|---------|-----|
| 123 | 123 | 0 | 0 | 90 |

### 按 Scenario

| Scenario | Total | IMPLEMENTED | PLANNED |
|----------|-------|-------------|---------|
| D7-S1 | 8 | 8 | 0 |
| D7-S2 | 36 | 36 | 0 |
| D7-S3 | 20 | 20 | 0 |
| D7-S4 | 9 | 9 | 0 |
| D7-S5 | 28 | 28 | 0 |
| **D7-S6** | **7** | **7** | **0** |
| 契约/迁移 | 8 | 8 | 0 |

> **v3.0 closure (2026-06-15):** v1.2 + v2.0-b/c/f 全部闭环。D7-S1-T08 (state machine), D7-S5-A01-T01 (confidence threshold), D7-S2-A06-T01..T04 (turn leader), D7-S2-A07-T01..T02 (LLM invoker) 全部 IMPLEMENTED。IMPLEMENTED 58→66，PLANNED 9→0。全部 T 点闭环。
>
> **v3.1 closure (2026-06-16):** **devrix-d7-uncertainty-gaps (DM-20260616-001) 归档**：+26 T 点全部 IMPLEMENTED（PlanAgent runtime gate 4 + PlanMode LLM guard 2 + ConflictGuard TOCTOU 4 + FlowEvent sink 2 + PlanModeApproveGate removal 2 + dead code markers 2 + 积分测试 10）。IMPLEMENTED 66→92，P0 44→63。
>
> **v3.2 closure (2026-06-17):** **devrix-d7-turn-history-persist (DM-20260617-003) 归档**：+2 T 点 IMPLEMENTED（D7-S5-A04-T01/T02 turn adapter persist + 3-轮集成）。IMPLEMENTED 94→96，P0 65→67。
>
> **v3.6 closure (2026-06-20):** **devrix-context-budget-and-isolation (DM-20260620-001) Phase A 归档**：+3 T 点 IMPLEMENTED（D7-S2-A06-T11 turn loop 集成 AC1+AC2 + T12 AC4 per-iter audit + T13 bootstrap 接线）。IMPLEMENTED 96→99，P0 67→70。D2 域内另 +13 T 点（D2-S17-A05 T01-T05 + D2-S17-A06 T01-T03 + D2-S15-A08 T01-T05）见 d2 t-registry。
>
> **v3.7 closure (2026-06-20):** **devrix-context-budget-and-isolation (DM-20260620-001-B) Phase B 归档**：+4 T 点 IMPLEMENTED（D7-S2-A06-T14 brief mode PreloadedMessages=nil + T15 fork/full mode parity + T16 depth limit + T17 default mode resolution chain）。IMPLEMENTED 99→103，P0 70→74。配套 D2 域 +3 T 点（D2-S15-A08 T06-T08 BuildForkedMessages byte-level prefix stability）见 d2 t-registry；D4 域 +2 T 点（D4-S14-A07 T01-T02 mode field schema）见 d4 t-registry。AC12 D5 spans 22-step replay P95=21707 ≤ 40K（Phase A baseline 51K）。
>
> **v3.8 closure (2026-06-20):** **devrix-context-budget-phase-c-nested (DM-20260620-002) Phase C 归档**：+6 T 点 IMPLEMENTED（D7-S2-A06-T18 TurnRequest.MaxContextTokens 字段 + T19 nested 分支读取 + T20 SubTurnRunner Cfg + bootstrap 注入 + T21 显式注入单测 + T22 fallback 单测 + T23 4-parallel integration）。IMPLEMENTED 103→109，P0 74→80。配套 D2 域 +2 T 点（D2-S15-A08 T09 SubTurnRequest→TurnRequest propagation + T10 SubQueryParams→SubTurnRequest pass-through）见 d2 t-registry。AC2 4-parallel deep-review sub-agents 全绿（capture adapter 验证 max 消息 1186 chars folded from 80000）。D7TestStack 同步修复 deepseek DefaultModel / ModelRouting 默认空值（unblocks 所有 D7 integration test）。
>
> **v3.9 closure (2026-06-20):** **devrix-error-handling-tier1-tier2 (DM-20260620-003) 归档**：+7 T 点 IMPLEMENTED（D7-S1-T18 TaskManager.Create `(*Task, error)` + D7-S1-T19 resolveDelegateTaskID `(string, error)` + D7-S2-A06-T24 turn_adapter invariant migration to sharederrors + T25 subturn error_code wrap + T26 subturn channel-closed sentinel + T27 retry nil-sentinel defensive wrap + D7-S2-A02-T18 orchestrator.emitError variadic code）。IMPLEMENTED 109→116，P0 80→83。配套 D3 域 +1 T 点（D3-S3-A01-T16 retry nil-sentinel）见 d3 t-registry；D5 域 +1 T 点（D5-S23-A06-T03 Observability.Shutdown errors.Join）见 d5 t-registry。Tier 1 (PR-A #141) + Tier 2 C2 (PR-B #142) + Tier 2 H3+M1..M4 (PR-C #143) 全部 merged；跨切面 `docs/error-handling.md` v1.0 落地。

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始（仅 ORCH-S2-T* 遗留 ID） |
| 2.0.0 | 2026-06-14 | D7-S*-T* 统一编号、Legacy 映射、S1/S5/契约 T 点补全 |
| 2.1.0 | 2026-06-14 | Review R1：T02 拆分、T06/T07、MIG-T01、v1.0/v1.1 范围标注 |
| 2.2.0 | 2026-06-14 | Review R2：T02c 端到端 SLA、T04 中断顺序、D7-D6-T01 metric、S5-T02 白名单 |
| 2.3.0 | 2026-06-14 | DM-20260614-005：D7-S5-T03 / T06 闭环（端到端测试 + CommandFirst=false 回归） |
| 2.4.0 | 2026-06-14 | devrix-d7-sa-refine (DM-20260614-008)：T03 anti-fabrication、D7-S5-A01-T01/T02、S5-A02-T01 新增 |
| 2.5.0 | 2026-06-15 | DM-020 v1.0 Registry：D7-S2-A06/A07 T 点（6 个 PLANNED）+ Legacy T 映射表 |
| 2.6.0 | 2026-06-15 | **v1.1 closure 同步**：(1) D7-S1 T01-T05 路径迁入 workmodel/；(2) D7-S1-T06 升 IMPLEMENTED（decomposer_test.go::validateGraph）；(3) D7-S5-T04/T05 升 IMPLEMENTED；(4) D7-S5-A02-T01/T02 + A03-T01/T02 增补；(5) D7-S2-A04-T01/T02/T03 Dispatcher；(6) D7-S4-T08/T09 SpokeBridge。总 55→67，IMPLEMENTED 40→53 |
| 2.7.0 | 2026-06-15 | **D7-S5-A03-T03 LLM Decomposer 闭环**：`decisionplanning/llm_decomposer_test.go` 7 T sub-cases（happy path / bad JSON / enum coercion / unknown deps / extractJSON / nil LLM / SynthesizeTaskGraph routing）；D7-S5 总数 14→21，IMPLEMENTED 11→18 |
| 2.8.0 | 2026-06-15 | **D2 Thin + CLI Worker + BackgroundRun 闭环**：(1) D7-D4-T01/D7-THIN-T01/D7-THIN-T02 PLANNED→IMPLEMENTED；(2) D7-S3-T11 PARTIAL→IMPLEMENTED（SIGTERM/SIGKILL 测试）；(3) D7-S1-T07 PARTIAL→IMPLEMENTED（LocalWorkModel.SetBackgroundProvider + GlobalBackgroundRegistry 初始化）；IMPLEMENTED 53→58，PARTIAL 1→0，PLANNED 13→9 |
| 3.0.0 | 2026-06-15 | **v1.2 + v2.0-b/c/f 全部闭环**：(1) D7-S1-T08 state machine guard + test（IsLegalTransition 24 transition + 4 journey）；(2) D7-S5-A01-T01 confidence threshold verification + FastPathThreshold gating；(3) D7-S2-A06-T01..T04 turn leader 全部 IMPLEMENTED（含 SubQuery nested turn）；(4) D7-S2-A07-T01/T02 LLM invoker breaker/timeout 测试（llm_test.go 9 tests）；IMPLEMENTED 58→66，PLANNED 9→1 |
| **3.1.0** | **2026-06-16** | **devrix-d7-uncertainty-gaps (DM-20260616-001) 归档**：(1) D7-S3 +9 T 点（ConflictGuard TOCTOU 4 + FlowEvent sink 2 + WaveScheduler 积分 3）；(2) D7-S5 +12 T 点（PlanAgent runtime gate 4 + PlanMode LLM guard 2 + PlanModeApproveGate removal 2 + LLM Decomposer 积分 4）；(3) D7-S2 +5 T 点（dead code markers 2 + multi-turn 积分 3）。IMPLEMENTED 66→92，P0 44→63 |
| **3.2.0** | **2026-06-16** | **devrix-d7-loop-first-routing (DM-20260616-002) 归档**：Loop-First L5 登记 D7-S2-L5-01..06（6 P0/P1） |
| **3.3.0** | **2026-06-17** | **devrix-queryloop-legacy-decommission (DM-20260617-001)**：(1) D7-S2-A06-T09 登记（orchestrator 不触 D2.QueryLoop.Run）；(2) D7-S2-A06-T10 登记（Run() 必增 metric）。IMPLEMENTED 92→94 |
| **3.5.0** | **2026-06-19** | **devrix-d7-v2-structure 路径同步**：T 表 Code Location 列对齐 sessionorchestrator/decisionplanning/wavescheduler/executionflow/orchtypes |
| **3.6.0** | **2026-06-20** | **2026-06-20-devrix-context-budget-and-isolation (devrix-context-budget-and-isolation / DM-20260620-001) Phase A 归档**：D7-S2-A06 +3 T 点（T11 turn loop 集成 AC1+AC2 + T12 AC4 per-iter audit + T13 bootstrap 接线）。IMPLEMENTED 96→99，P0 67→70 |
| **3.7.0** | **2026-06-20** | **2026-06-20-devrix-context-budget-and-isolation-phase-b (devrix-context-budget-and-isolation / DM-20260620-001-B) Phase B 归档**：D7-S2-A06 +4 T 点（T14 brief mode PreloadedMessages=nil + T15 fork/full mode parity + T16 depth limit + T17 default mode resolution chain）。IMPLEMENTED 99→103，P0 70→74 |
| **3.8.0** | **2026-06-20** | **2026-06-20-devrix-context-budget-phase-c-nested (devrix-context-budget-phase-c-nested / DM-20260620-002) Phase C 归档**：D7-S2-A06 +6 T 点（T18 TurnRequest.MaxContextTokens 字段 + T19 nested 分支读取 + T20 SubTurnRunner Cfg + bootstrap 注入 + T21 显式注入单测 + T22 fallback 单测 + T23 4-parallel integration）。IMPLEMENTED 103→109，P0 74→80。D7TestStack 同步修复 deepseek DefaultModel / ModelRouting 默认空值。 |
| **3.9.0** | **2026-06-20** | **devrix-error-handling-tier1-tier2 (DM-20260620-003) 归档**：D7-S1 +2 T 点 (T18 TaskManager.Create `(*Task, error)` + T19 resolveDelegateTaskID `(string, error)`) + D7-S2-A06 +4 T 点 (T24 invariant migration + T25 subturn error_code wrap + T26 channel-closed sentinel + T27 retry nil-sentinel) + D7-S2-A02 +1 T 点 (T18 emitError variadic code)。IMPLEMENTED 109→116, P0 80→83。 |
| **3.4.0** | **2026-06-19** | **devrix-d7-v2-structure (DM-20260619-005)**：T ID 不变（66/66 IMPLEMENTED）；测试文件随实现迁移 |
