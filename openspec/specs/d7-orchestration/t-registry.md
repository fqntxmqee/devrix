# D7 Orchestration Domain — T 层测试点注册表

**Status:** Active
**Version:** 3.5.0
**Last Updated:** 2026-06-17
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `d7-domain.md`
**Spec:** `openspec/specs/d7-orchestration/spec.md`
**Complements:** `terminal-state-guide.md` · `observability-guide.md`

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
| D7-S4-T02 | — | Hub 双通道：WorkPlan + SessionQueue + IM | D7-S4-A01 | `orchestration/flow/hub_test.go`；`tests/integration/d7/d7_hub_flow_test.go` | IMPLEMENTED | P0 |
| D7-S4-T03 | D4-S10-T04 | FlowStarted 触发 delegate-progress 入队 | D7-S4-A01-F02 | `orchestration/flow/hub_test.go`；`tests/integration/d7/d7_hub_flow_test.go` | IMPLEMENTED | P0 |
| D7-S4-T04 | D4-S10-T07 | Snapshot 含 Task 投影（link_tasks） | D7-S1-A03-F02 | `orchestration/flow/hub_test.go` | IMPLEMENTED | P0 |
| D7-S4-T05 | D4-S10-T05 | IMSink 发射 worker_progress 事件 | D7-S4-A03-F01 | `orchestration/imsink/gateway_test.go` | IMPLEMENTED | P0 |
| D7-S4-T06 | — | FlowToolCall 节流（throttle_ms） | D7-S4-A01-F04 | `orchestration/flow/hub_test.go` | IMPLEMENTED | P1 |
| **D7-S4-T08** | — | **AgentBridge OnWorkerCompleted success/error** | **D7-S4-A04** | **`hubspoke/hubspoke_test.go::TestAgentBridge_OnWorkerCompleted_{success,error}`** | **IMPLEMENTED** | **P0** |
| **D7-S4-T09** | — | **SubQueryBridge PublishStarted/Completed/Failed** | **D7-S4-A05** | **`hubspoke/hubspoke_test.go::TestSubQueryBridge_Publish{Started,Completed,Failed}`** | **IMPLEMENTED** | **P0** |

---

## D7-S3: Wave Scheduler

| T ID | Legacy ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|-----------|------|----------|-----------|--------|----------|
| D7-S3-T01 | ORCH-S2-T10 | 6 ready subagent + 1 cursor 峰值并发≤5 | D7-S3-A01 | `orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| D7-S3-T02 | ORCH-S2-T15 | 槽位释放后 ready Task 立即派发 | D7-S3-A01-F04 | `orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| D7-S3-T03 | ORCH-S2-T17 | Plan DAG 仅 ready 节点被派发 | D7-S3-F03 | `orchestration/wave/scheduler_test.go`, `taskgraph_test.go` | IMPLEMENTED | P0 |
| D7-S3-T04 | ORCH-S2-T11 | upstream policy 收到 artifact，无 Leader 全量 | D7-S3-A02-F02 | `orchestration/wave/context_test.go`, `scheduler_orch_test.go` | IMPLEMENTED | P0 |
| D7-S3-T05 | ORCH-S2-T12 | fresh policy Messages 仅含 directive | D7-S3-A02-F01 | `orchestration/wave/context_test.go` | IMPLEMENTED | P0 |
| D7-S3-T06 | ORCH-S2-T13 | 同 conflict_group Task 不并行 | D7-S3-A03-F01 | `orchestration/wave/scheduler_orch_test.go` | IMPLEMENTED | P0 |
| D7-S3-T07 | ORCH-S2-T16 | cursor + claude-code 并行 file_scope 不交 | D7-S3-A03-F03 | `orchestration/wave/scheduler_orch_test.go` | IMPLEMENTED | P1 |
| D7-S3-T08 | ORCH-S2-T18 | wave 全完成返回全部 artifacts | D7-S3-A01-F03 | `orchestration/wave/scheduler_orch_test.go` | IMPLEMENTED | P1 |
| D7-S3-T09 | ORCH-S2-T19 | CancelWorker 槽位释放 status=cancelled | D7-S3-A01-F05 | `orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| D7-S3-T10 | ORCH-S2-T20 | CancelAll 5 running 全部 terminal | D7-S3-A01-F05 | `orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| D7-S3-T11 | ORCH-S2-T21 | CLI Worker cancel 进程终止 | D7-S3-F06 | `orchestration/wave/runners/agent_tool_orch_test.go`; `multiagent/external/cli_adapter_test.go` | IMPLEMENTED | P1 |
| **D7-S3-A01-F03-T01** | — | **AllowAndRegister no conflict → registered** | **D7-S3-A01-F03** | **`orchestration/wave/conflict_test.go::TestAllowAndRegister_NoConflict`** | **IMPLEMENTED** | **P0** |
| **D7-S3-A01-F03-T02** | — | **AllowAndRegister conflict group → blocked** | **D7-S3-A01-F03** | **`orchestration/wave/conflict_test.go::TestAllowAndRegister_ConflictGroup`** | **IMPLEMENTED** | **P0** |
| **D7-S3-A01-F03-T03** | — | **AllowAndRegister different group → allowed** | **D7-S3-A01-F03** | **`orchestration/wave/conflict_test.go::TestAllowAndRegister_DifferentGroup`** | **IMPLEMENTED** | **P0** |
| **D7-S3-A01-F03-T04** | — | **AllowAndRegister file scope intersection → blocked** | **D7-S3-A01-F03** | **`orchestration/wave/conflict_test.go::TestAllowAndRegister_FileScope`** | **IMPLEMENTED** | **P0** |
| **D7-S3-A01-F04-T01** | — | **emit pushes FlowEvent to sink AND channel** | **D7-S3-A01-F04** | **`coordinator/orchestrate_path.go::emit()`** | **IMPLEMENTED** | **P0** |
| **D7-S3-A01-F04-T02** | — | **emit tolerates nil sink gracefully** | **D7-S3-A01-F04** | **`coordinator/orchestrate_path.go::emit()`** | **IMPLEMENTED** | **P0** |
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
| D7-S1-T05 | FlowEvent link_tasks 状态联动 | D7-S1-A02-F06 | `orchestration/flow/hub_test.go` | IMPLEMENTED | P1 |
| D7-S1-T06 | CreateWorkPlan DAG 校验 | D7-S1-A01-F02 | `coordinator/decomposer_test.go::TestTaskDecomposer_validateGraph` | IMPLEMENTED | P1 |
| D7-S1-T07 | BackgroundRun 注册与 QueryWorkPlan 可见 | D7-S1 | `coordinator/entry_test.go`; `contextengine/nested/background_*_test.go` | IMPLEMENTED | P1 |
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
| **D7-S3-T12** | **OrchestratePath SyncWaveNodes 挂树** | **D7-S3-A01** | **`coordinator/orchestrate_path.go`; bootstrap wiring** | **IMPLEMENTED** | **P1** |

---

## D7-S5: Decision & Planning

> **v1.1 closure (2026-06-15):** D7-S5-T04/T05 由 PLANNED 升为 IMPLEMENTED（Phase H/K）。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| D7-S5-T01 | PlanMode inactive→active 转换 | D7-S1-A04-F01 | `workmodel/plan_mode_test.go` 或 `task_manager_test` | IMPLEMENTED | P1 |
| D7-S5-T02 | PlanAgent 只读模式拒绝写操作；工具白名单不含 write/edit/bash | D7-S5-A04 | `workmodel/plan_agent_whitelist_test.go`（10 ACs）；`tests/integration/d7/d7_workmodel_test.go` | IMPLEMENTED | P0 |
| D7-S5-T03 | ClassifyIntent 规则高置信 → simple | D7-S5-A01 | `coordinator/classifier_test.go` | IMPLEMENTED | P0 |
| **D7-S5-T04** | **SynthesizeTaskGraph 产出有效 DAG** | **D7-S5-A02** | **`coordinator/decomposer_test.go::TestTaskDecomposer_SynthesizeTaskGraph`** | **IMPLEMENTED** | **P1** |
| **D7-S5-T05** | **SelectExecutor explore→D2 execute→D4** | **D7-S5-A03** | **`coordinator/executor_test.go::TestExecutorSelector_SelectExecutor`** | **IMPLEMENTED** | **P1** |
| D7-S5-T06 | Command-first：`/plan` 不触发 LLM Classify | D7-S5-A01 | `coordinator/{classifier,shadow_classifier,orchestrator}_test.go`；`tests/integration/d7/d7_fastpath_test.go` | IMPLEMENTED | P0 |
| D7-S5-T07 | Tail-only LLM classify shadow（rule 未命中时异步 LLM，结果只入 metric） | D7-S5-A05 | `coordinator/shadow_classifier_test.go` | IMPLEMENTED | P0 |
| D7-S5-A01-T01 | 规则分类置信度阈值验证（screening 可重复性） | D7-S5-A01 | `coordinator/classifier_test.go::TestRuleClassifier_ExactConfidenceValues`, `TestRuleClassifier_ConfidenceDeterminism`, `TestRuleClassifier_ConfidenceRange`; `coordinator/orchestrator_test.go::TestSessionOrchestrator_FastPathConfidence{Below,Above}Threshold` | IMPLEMENTED | P0 |
| D7-S5-A01-T02 | Command-first 优先于 LLM 分类（用户显式策略优先） | D7-S5-A01 | `coordinator/classifier_test.go` | IMPLEMENTED | P0 |
| **D7-S5-A02-T01** | **SynthesizeTaskGraph 吸收 Explore Workers FlowEvent 产出有效 DAG** | **D7-S5-A02** | **`coordinator/decomposer_test.go::TestTaskDecomposer_SynthesizeTaskGraph`** | **IMPLEMENTED** | **P1** |
| **D7-S5-A02-T02** | **decomposeGoal 规则版：goal → sub_goal → DAG** | **D7-S5-A02-F01** | **`coordinator/decomposer_test.go::TestTaskDecomposer_decomposeGoal`** | **IMPLEMENTED** | **P1** |
| **D7-S5-A03-T01** | **MatchExecutorByTaskType：worker_type → D2/D4** | **D7-S5-A03-F01** | **`coordinator/executor_test.go::TestExecutorSelector_MatchExecutorByTaskType`** | **IMPLEMENTED** | **P1** |
| **D7-S5-A03-T02** | **CheckExecutorAvailability：executor 状态查询** | **D7-S5-A03-F02** | **`coordinator/executor_test.go::TestExecutorSelector_CheckExecutorAvailability`** | **IMPLEMENTED** | **P1** |
| **D7-S5-A03-T03** | **LLM Decomposer 解析 JSON DAG → wave.TaskNode（含 7 sub-cases）** | **D7-S5-A03-F03** | **`coordinator/llm_decomposer_test.go`（happy / bad JSON / enum coercion / unknown deps / extractJSON 6 case / nil LLM / SynthesizeTaskGraph routing）** | **IMPLEMENTED** | **P1** |
| **D7-S5-A02-F01-T01** | — | **ValidateToolCall: whitelist tool passes** | **D7-S5-A02-F01** | **`workmodel/plan_agent_whitelist_test.go::TestValidateToolCall_Allowed`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F01-T02** | — | **ValidateToolCall: forbidden tool rejected** | **D7-S5-A02-F01** | **`workmodel/plan_agent_whitelist_test.go::TestValidateToolCall_Forbidden`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F01-T03** | — | **ValidateToolCall: unknown tool rejected** | **D7-S5-A02-F01** | **`workmodel/plan_agent_whitelist_test.go::TestValidateToolCall_Unknown`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F01-T04** | — | **ValidateToolCall: nil receiver safe** | **D7-S5-A02-F01** | **`workmodel/plan_agent_whitelist_test.go::TestValidateToolCall_NilReceiver`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F02-T01** | — | **PlanMode.Enter: nil LLM returns ErrLLMNotConfigured** | **D7-S5-A02-F02** | **`workmodel/plan_mode.go::Enter()`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F02-T02** | — | **PlanMode.Enter: valid LLM succeeds** | **D7-S5-A02-F02** | **`workmodel/plan_mode.go::Enter()`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F05-T01** | — | **Config struct: PlanModeApproveGate field removed** | **D7-S5-A02-F05** | **`coordinator/config.go`, `shared/config/coordinator.go`, `shared/config/loader.go`, `bootstrap/wire_coordinator.go`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-F05-T02** | — | **Default config: no PlanModeApproveGate reference** | **D7-S5-A02-F05** | **`coordinator/config.go::DefaultConfig()`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-IT01** | — | **LLM Decomposer end-to-end (JSON DAG → WaveScheduler)** | **D7-S5-A02** | **`tests/integration/d7/d7_llm_decomposer_test.go::TestIntegration_D7LLMDecomposer_EndToEnd`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-IT02** | — | **LLM Decomposer fallback on invalid JSON** | **D7-S5-A02** | **`tests/integration/d7/d7_llm_decomposer_test.go::TestIntegration_D7LLMDecomposer_FallbackOnInvalidJSON`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A02-IT03** | — | **LLM Decomposer empty task list** | **D7-S5-A02** | **`tests/integration/d7/d7_llm_decomposer_test.go::TestIntegration_D7LLMDecomposer_EmptyTaskList`** | **IMPLEMENTED** | **P1** |
| **D7-S5-A02-IT04** | — | **LLM Decomposer no JSON in response** | **D7-S5-A02** | **`tests/integration/d7/d7_llm_decomposer_test.go::TestIntegration_D7LLMDecomposer_NoJSONInResponse`** | **IMPLEMENTED** | **P1** |
| **D7-S5-A04-T01** | **turn_adapter.PersistTurn 提交 req.Messages 到 D2 内存（DM-20260617-003 d7-turn-history-persist）** | **D7-S5-A04** | **`internal/bootstrap/turn_adapter_persist_test.go::TestPersistTurn_{WritesMessagesToD2Memory,FullRound,NilEngine,AppendError}`** | **IMPLEMENTED** | **P0** |
| **D7-S5-A04-T02** | **三轮同 session 连续 PersistTurn → Prepare 返回全历史** | **D7-S5-A04** | **`tests/integration/d7/turn_history_persist_test.go::TestTurnHistory_ThreeTurns`** | **IMPLEMENTED** | **P0** |

---

## D7-S2: Session Orchestrator

> **v1.1 closure (2026-06-15):** D7-S2-A04 DispatchWorker wired（Phase DM-018）；D7-S2-A06/A07 wired（Phase DM-020）。T 层增补 hubspoke 测试。

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority |
|------|------|--------|-----------|--------|----------|
| D7-S2-T01 | ProcessMessage 为 D1 主入口 | D7-S2-A01 | `coordinator/orchestrator_test.go`；`tests/integration/d7/d7_fastpath_test.go` | IMPLEMENTED | P0 |
| D7-S2-T02a | FastPath proxy 开销 P99 ≤ 2ms（Classify 后） | D7-S2-A01-F02 | `coordinator/orchestrator_test.go` | IMPLEMENTED | P0 |
| D7-S2-T02b | 规则 ClassifyIntent P99 ≤ 1ms | D7-S2-A02 | `coordinator/classifier_test.go` | IMPLEMENTED | P0 |
| D7-S2-T02c | FastPath 端到端 P99 ≤ 2ms（command-first 全栈） | D7-S2-A01-F02 | `coordinator/orchestrator_test.go`；`tests/integration/d7/d7_fastpath_test.go` | IMPLEMENTED | P0 |
| D7-S2-T03 | OrchestratePath 按路由矩阵（v1.1.0+ 显式调 SynthesizeTaskGraph + WaveScheduler） | D7-S2-A01-F03 | `coordinator/orchestrate_path_test.go` (5 AC) | IMPLEMENTED | P0 |
| D7-S2-T04 | HandleInterrupt：Wave→D4→Process→stopped→TaskCancel | D7-S2-A03 | `coordinator/orchestrator_test.go`；`tests/integration/d7/d7_interrupt_test.go` | IMPLEMENTED | P0 |
| D7-S2-T05 | HandleInterrupt 幂等 | D7-S2-A03 | `coordinator/orchestrator_test.go` | IMPLEMENTED | P0 |
| D7-S2-A01-T03 | 禁止在 Worker terminal FlowEvent 前伪造 Task 进度（anti-fabrication commitment） | D7-S2-A01 | `coordinator/orchestrator_test.go::TestSessionOrchestrator_AntiFabrication_NoSyntheticProgress` | IMPLEMENTED | P0 |
| D7-S2-A01-T04 | IntentCommand 显式分发到 PlanCLI/CLICommands（v1.1.0+ orthogonal） | D7-S2-A01 | `coordinator/command_handler_test.go` (3 AC) | IMPLEMENTED | P0 |
| D7-S2-A01-T05 | IntentOrchestrate 走 SynthesizeTaskGraph + WaveScheduler（v1.1.0+ orthogonal） | D7-S2-A01 | `coordinator/orchestrate_path_test.go` (5 AC) | IMPLEMENTED | P0 |
| D7-S2-A01-T06 | IntentFast 保持 FastPath（v1.1.0+ orthogonal, 不回归） | D7-S2-A01 | `coordinator/orchestrator_test.go::TestSessionOrchestrator_ProcessMessage_FastPath` | IMPLEMENTED | P0 |
| D7-S2-A03-T01 | HandleInterrupt 中断顺序正确（可中断性承诺） | D7-S2-A03 | `coordinator/orchestrator_test.go`；`tests/integration/d7/d7_interrupt_test.go` | IMPLEMENTED | P0 |
| **D7-S2-A04-T01** | **DispatchWorker D4 enabled with leader** | **D7-S2-A04** | **`hubspoke/hubspoke_test.go::TestDispatcher_Dispatch_D4_enabled_withLeader`** | **IMPLEMENTED** | **P0** |
| **D7-S2-A04-T02** | **DispatchWorker D4 disabled falls back to D2 SubQuery** | **D7-S2-A04** | **`hubspoke/hubspoke_test.go::TestDispatcher_Dispatch_D4_disabled_fallsToD2`** | **IMPLEMENTED** | **P0** |
| **D7-S2-A04-T03** | **DispatchWorker async mode** | **D7-S2-A04** | **`hubspoke/hubspoke_test.go::TestDispatcher_Dispatch_D4_async`** | **IMPLEMENTED** | **P1** |
| **D7-S2-A03-F06-T01** | — | **LLMFallbackClassifier Deprecated marker** | **D7-S2-A03-F06** | **`coordinator/classifier_fallback.go`** | **IMPLEMENTED** | **P1** |
| **D7-S2-A03-F06-T02** | — | **ExecutorSelector Deprecated marker** | **D7-S2-A03-F06** | **`coordinator/executor.go`** | **IMPLEMENTED** | **P1** |
| **D7-S2-A06-IT01** | — | **Multi-turn tool conversation (2 LLM rounds)** | **D7-S2-A06** | **`tests/integration/d7/d7_multiturn_test.go::TestIntegration_D7FastPath_MultiTurnToolConversation`** | **IMPLEMENTED** | **P0** |
| **D7-S2-A06-IT02** | — | **MaxTurns cap enforcement** | **D7-S2-A06** | **`tests/integration/d7/d7_multiturn_test.go::TestIntegration_D7FastPath_MaxTurnsCap`** | **IMPLEMENTED** | **P1** |
| **D7-S2-A06-IT03** | — | **StopProcess during slow Turn** | **D7-S2-A06** | **`tests/integration/d7/d7_multiturn_test.go::TestIntegration_D7FastPath_ContextCancellation`** | **IMPLEMENTED** | **P1** |

### Turn Adapter LTL-Lite Hook (DM-20260618-007)

**Change:** devrix-tools-terminal-architecture (DM-20260618-007) — LTL-Lite runtime check + CI lint + turn_adapter HookRegistry (PERMISSION-GATE-1-T01/T02/T03) + BackgroundTaskSurface ToolEventStream (D7-S2-A08-T01)

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| **PERMISSION-GATE-1-T01** | LTL-Lite runtime check (ltllite.Check + HookRegistry.Prepare) | turn_adapter | `internal/layers/orchestration/turn_adapter/ltl_hook_test.go::TestHookRegistry_Prepare_*` | **IMPLEMENTED** | **P0** |
| **PERMISSION-GATE-1-T02** | CI lint 静态校验 (ci-lint-invariant 扫描 _invariant.go) | tools/ | `tools/ci-lint-invariant/main_test.go` | **IMPLEMENTED** | **P0** |
| **PERMISSION-GATE-1-T03** | turn_adapter HookRegistry Prepare/BeforeExecute 定向重检 | turn_adapter | `internal/layers/orchestration/turn_adapter/ltl_hook_test.go::TestHookRegistry_BeforeExecute_*` | **IMPLEMENTED** | **P0** |
| **D7-S2-A08-T01** | ToolEventStream context 推送 + BackgroundTaskSurface 集成 | turn | `internal/layers/orchestration/turn/tool_stream_test.go` | **IMPLEMENTED** | **P0** |

### Loop-First Routing L5 (DM-20260616-002)

| L5 ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|-------|------|----------|-----------|--------|----------|
| **D7-S2-L5-01** | 问候语 Turn 不触发 Wave（无 plan_formed/wave_started） | D7-S2-A01 | `tests/integration/d7/d7_loop_first_test.go::TestIntegration_D7LoopFirst_GreetingNoWave`；`coordinator/classifier_test.go::TestRuleClassifier_Classify_LoopFirstDefault` | IMPLEMENTED | P0 |
| **D7-S2-L5-02** | delegate_wave tool 门控 OrchestratePath | D7-S2-F02 | `coordinator/turn_tools_test.go`；`tests/integration/d7/d7_loop_first_test.go::TestIntegration_D7LoopFirst_DelegateWaveTool` | IMPLEMENTED | P0 |
| **D7-S2-L5-03** | Slash 命令零 LLM | D7-S2-A01 | `tests/integration/d7/d7_orthogonal_dispatch_test.go::TestIntegration_D7ProcessMessage_CommandBypassesLLM` | IMPLEMENTED | P0 |
| **D7-S2-L5-04** | EngineEvent 单投递（无 sink mirror） | D7-S2-F03 | `coordinator/orchestrator_test.go`；`capture/agent_route.go` | IMPLEMENTED | P0 |
| **D7-S2-L5-05** | enter_plan_mode tool | D7-S2-F02 | `coordinator/turn_tools_test.go` | IMPLEMENTED | P1 |
| **D7-S2-L5-06** | rule_orchestrate 回滚（threshold 降级） | D7-S2-F01 | `coordinator/routing_test.go`；`coordinator/orchestrator_test.go` | IMPLEMENTED | P1 |

---

## Cross-Domain (D7 契约)

| T ID | 描述 | 归属 | Test 位置 | Status | Priority |
|------|------|------|-----------|--------|----------|
| D7-D1-T01 | D1 调用 D7 而非 D2（d7_enabled） | D7-S2 | `tests/integration/d7/d7_entry_test.go`（WireD7 全栈）；`coordinator/entry_test.go` | IMPLEMENTED | P0 |
| D7-D4-T01 | D2 loop 无 delegate hooks | D7-S2 | `contextengine/query/loop_thin_test.go` | IMPLEMENTED | P0 |
| D7-D6-T01 | D6 校验编排决策（advisory）+ `orchestration.d6.validation.{pass,fail,timeout,error}` metric | D7-S5 | `internal/layers/orchestration/coordinator/d6_metrics_test.go` | IMPLEMENTED | P1 |
| D7-D6-T02 | D6 校验超时 50ms 视为 pass | D7-S5 | `internal/layers/orchestration/coordinator/entry_test.go` | IMPLEMENTED | P2 |
| D7-D6-T03 | 4 counter 注入 + result.Pass 分流 | D7-S5 | `internal/layers/orchestration/coordinator/d6_metrics_test.go` | IMPLEMENTED | P0 |
| D7-D6-T04 | timeout_rate > 5% 触发 AlertHook（5min 滑窗） | D7-S5 | `internal/layers/orchestration/coordinator/d6_metrics_test.go` | IMPLEMENTED | P0 |
| D7-D6-T05 | panic-recovered 计入 error 路径 | D7-S2 | `internal/layers/orchestration/coordinator/d6_metrics_test.go` | IMPLEMENTED | P0 |
| D7-D6-T06 | nil validator 与 nil metrics 都降级 no-op | D7-S2 | `internal/layers/orchestration/coordinator/d6_metrics_test.go` | IMPLEMENTED | P0 |
| D7-MIG-T01 | D7-only ingress × plan.enabled 组合回归 | D7-S2 | `tests/integration/d7/d7_entry_test.go::TestIntegration_D7Entry_PlanModeStillUsesD7Path`；`coordinator_matrix_test.go` | IMPLEMENTED | P0 |
| D7-THIN-T01 | loop.go 无编排字段 | D2 瘦身 | `contextengine/query/loop_thin_test.go` | IMPLEMENTED | P0 |
| D7-THIN-T02 | loop.go Run ≤200 行 | D2 瘦身 | `contextengine/query/loop.go` (170 行) | IMPLEMENTED | P0 |

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
| 96 | 96 | 0 | 0 | 67 |

### 按 Scenario

| Scenario | Total | IMPLEMENTED | PLANNED |
|----------|-------|-------------|---------|
| D7-S1 | 8 | 8 | 0 |
| D7-S2 | 23 | 23 | 0 |
| D7-S3 | 20 | 20 | 0 |
| D7-S4 | 9 | 9 | 0 |
| D7-S5 | 28 | 28 | 0 |
| 契约/迁移 | 6 | 6 | 0 |

> **v3.0 closure (2026-06-15):** v1.2 + v2.0-b/c/f 全部闭环。D7-S1-T08 (state machine), D7-S5-A01-T01 (confidence threshold), D7-S2-A06-T01..T04 (turn leader), D7-S2-A07-T01..T02 (LLM invoker) 全部 IMPLEMENTED。IMPLEMENTED 58→66，PLANNED 9→0。全部 T 点闭环。
>
> **v3.1 closure (2026-06-16):** **devrix-d7-uncertainty-gaps (DM-20260616-001) 归档**：+26 T 点全部 IMPLEMENTED（PlanAgent runtime gate 4 + PlanMode LLM guard 2 + ConflictGuard TOCTOU 4 + FlowEvent sink 2 + PlanModeApproveGate removal 2 + dead code markers 2 + 积分测试 10）。IMPLEMENTED 66→92，P0 44→63。
>
> **v3.2 closure (2026-06-17):** **devrix-d7-turn-history-persist (DM-20260617-003) 归档**：+2 T 点 IMPLEMENTED（D7-S5-A04-T01/T02 turn adapter persist + 3-轮集成）。IMPLEMENTED 94→96，P0 65→67。

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
| 2.7.0 | 2026-06-15 | **D7-S5-A03-T03 LLM Decomposer 闭环**：`coordinator/llm_decomposer_test.go` 7 T sub-cases（happy path / bad JSON / enum coercion / unknown deps / extractJSON / nil LLM / SynthesizeTaskGraph routing）；D7-S5 总数 14→21，IMPLEMENTED 11→18 |
| 2.8.0 | 2026-06-15 | **D2 Thin + CLI Worker + BackgroundRun 闭环**：(1) D7-D4-T01/D7-THIN-T01/D7-THIN-T02 PLANNED→IMPLEMENTED；(2) D7-S3-T11 PARTIAL→IMPLEMENTED（SIGTERM/SIGKILL 测试）；(3) D7-S1-T07 PARTIAL→IMPLEMENTED（LocalWorkModel.SetBackgroundProvider + GlobalBackgroundRegistry 初始化）；IMPLEMENTED 53→58，PARTIAL 1→0，PLANNED 13→9 |
| 3.0.0 | 2026-06-15 | **v1.2 + v2.0-b/c/f 全部闭环**：(1) D7-S1-T08 state machine guard + test（IsLegalTransition 24 transition + 4 journey）；(2) D7-S5-A01-T01 confidence threshold verification + FastPathThreshold gating；(3) D7-S2-A06-T01..T04 turn leader 全部 IMPLEMENTED（含 SubQuery nested turn）；(4) D7-S2-A07-T01/T02 LLM invoker breaker/timeout 测试（llm_test.go 9 tests）；IMPLEMENTED 58→66，PLANNED 9→1 |
| **3.1.0** | **2026-06-16** | **devrix-d7-uncertainty-gaps (DM-20260616-001) 归档**：(1) D7-S3 +9 T 点（ConflictGuard TOCTOU 4 + FlowEvent sink 2 + WaveScheduler 积分 3）；(2) D7-S5 +12 T 点（PlanAgent runtime gate 4 + PlanMode LLM guard 2 + PlanModeApproveGate removal 2 + LLM Decomposer 积分 4）；(3) D7-S2 +5 T 点（dead code markers 2 + multi-turn 积分 3）。IMPLEMENTED 66→92，P0 44→63 |
| **3.2.0** | **2026-06-16** | **devrix-d7-loop-first-routing (DM-20260616-002) 归档**：Loop-First L5 登记 D7-S2-L5-01..06（6 P0/P1） |
| **3.3.0** | **2026-06-17** | **devrix-queryloop-legacy-decommission (DM-20260617-001)**：(1) D7-S2-A06-T09 登记（orchestrator 不触 D2.QueryLoop.Run）；(2) D7-S2-A06-T10 登记（Run() 必增 metric）。IMPLEMENTED 92→94 |
| **3.4.0** | **2026-06-19** | **devrix-d7-v2-structure (DM-20260619-005)**：T ID 不变（66/66 IMPLEMENTED）；测试文件随实现迁至 sessionorchestrator/decisionplanning/wavescheduler/executionflow/ |
