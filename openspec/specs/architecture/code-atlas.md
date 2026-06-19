# Devrix Code Atlas

**Capability:** code-atlas
**Status:** Active
**Version:** 1.2.0
**Last Updated:** 2026-06-19
**Parent:** `openspec/specs/architecture/layering.md`
**Demand:** DM-20260619-001 (D7 v2.0 alignment, docs-only)
**SoT:** `openspec/specs/d7-orchestration/spec.md` v3.8.0

> **v1.2.0 重大变更：** D-S Index 从 QueryLoop v2 主索引（2026-06-13）替换为 **D7 v2.0 unified 主索引**。D2 QueryLoop / SubQuery / sidechain_transcript 标 **DEPRECATED**（DM-20260616-004 落地，legacy decommission 完成）。

---

## Overview

Devrix 代码图谱：D-S 到包路径的快速索引。新建文件时 MUST 先查 D-S 归属，再落盘到对应目录。

**可读版：** `docs/architecture/code-map.md`
**链路图：** `docs/architecture/request-flow.md`（D7 v2.0 链路）

---

## D7 v2.0 Unified D-S Index

> D7 编排层是 2026-06-15 升格为独立核心域（DM-020）后的主入口。v1.0+v1.1 闭环（PR #30+#35+#36） + v2.0 unified task tools（PR #83-#87）。

| L4 ID | 名称 | D-S | 包路径 | 关键类型 |
|-------|------|-----|--------|----------|
| **D7 主入口与编排** |||||
| coordinator_entry | D7 SessionOrchestrator 主入口 | D7-S2 | `orchestration/sessionorchestrator/` | `Entry`, `SessionOrchestrator` |
| classifier | Intent 分类器 | D7-S5-A01 | `orchestration/decisionplanning/classifier.go` | `RuleClassifier`, `ShadowClassifier` |
| classifier_fallback | LLM 兜底分类 | D7-S5-A01 | `orchestration/decisionplanning/classifier_fallback.go` | `LLMClassifierFallback` |
| command_handler | IntentCommand 显式 dispatch | D7-S2-A01 | `orchestration/sessionorchestrator/command_handler.go` | `Handle` |
| orchestrate_path | IntentOrchestrate SynthesizeTaskGraph → Wave | D7-S2-A01 | `orchestration/sessionorchestrator/orchestrate_path.go` | `Run` |
| fast_path | IntentFast → turn.RunTurn | D7-S2-A01 | `orchestration/sessionorchestrator/fastpath.go` | `Run` |
| llm_decomposer | LLM 拆 DAG | D7-S5-A03 | `orchestration/decisionplanning/llm_decomposer.go` | `LLMDecomposer`, `SynthesizeTaskGraph` |
| turn_orchestrator | RunTurn 主循环（resolve/decompose）| D7-S2-A06 | `orchestration/turn/orchestrator.go` | `DefaultOrchestrator`, `RunTurn`, `ResolveHint` |
| llm_invoker | D7 直调 D3 | D7-S2-A07 | `orchestration/turn/llm.go` | `GatewayInvoker`, `InvokeStream` |
| compression_summarizer | Summarizer 拆面出口 | D7-S2-A07 | `orchestration/turn/compression_summarizer.go` | `CompressionSummarizer` |
| subturn_runner | SubQuery/Background/Wave 嵌套轮次 | D7-S2-A06 | `orchestration/turn/subturn.go` | `SubTurnRunner`, `RunSubTurn` |
| **D7 任务/计划模型（v2.0 unified）** |||||
| workitem | 任务统一抽象 | D7-S1 | `orchestration/workmodel/workitem.go` | `WorkItem`, `WorkTree` |
| run_registry | 任务注册表 | D7-S1 | `orchestration/workmodel/run_registry.go` | `RunRegistry` |
| resolve_awaiter | 阻塞等待跑中子节点 | D7-S1 | `orchestration/workmodel/awaiter.go` | `ResolveAwaiter` |
| task_manager | Task CRUD + 依赖 DAG | D7-S1 | `orchestration/workmodel/task_manager.go` | `TaskManager` |
| task_store | 任务磁盘存储 | D7-S1 | `orchestration/workmodel/task_store.go` | `DiskStore` |
| plan_mode | /plan 工作流状态机 | D7-S5 | `orchestration/workmodel/plan_mode.go` | `Enter`, `Approve`, `Reject` |
| plan_agent | PlanAgent 只读探索 | D7-S5 | `orchestration/workmodel/plan_agent.go` | `PlanAgent`, `ValidateToolCall` |
| cli_commands | CLI 命令处理器 | D7-S1 | `orchestration/workmodel/cli_commands.go` | `Handle` |
| cross_session | 跨 session 任务 | D7-S1 | `orchestration/workmodel/cross_session.go` | `CrossSessionManager` |
| decompose | 规则化任务拆解 | D7-S5-A02 | `orchestration/workmodel/decompose.go` | `decomposeGoal` |
| **D7 多任务调度与执行流** |||||
| wave_scheduler | DAG 调度 + WorkerPool + ConflictGuard | D7-S3 | `orchestration/wavescheduler/scheduler.go` | `Start`, `WaitForCompletion` |
| wave_subagent_runner | Subagent Worker | D7-S3 | `orchestration/wavescheduler/runners/subagent.go` | `SubagentRunner` |
| wave_agent_tool_runner | Agent Tool Worker | D7-S3 | `orchestration/wavescheduler/runners/agent_tool.go` | `AgentToolRunner` |
| conflict_guard | 冲突组互斥仲裁 | D7-S3 | `orchestration/wavescheduler/conflict.go` | `Allow`, `AllowAndRegister` |
| context_resolver | 上下文策略解析 | D7-S3 | `orchestration/wavescheduler/context.go` | `Resolve` (fresh/resume/upstream) |
| execution_hub | FlowEvent 聚合 + 双通道 | D7-S4 | `orchestration/executionflow/hub/hub.go` | `GlobalHub`, `Publish` |
| workplan_service | WorkPlan 读模型 | D7-S4 | `orchestration/executionflow/workplan/service.go` | `Service`, `Snapshot` |
| im_sink | 飞书 worker_progress 推送 | D7-S4 | `orchestration/executionflow/imsink/gateway.go` | `GatewaySink` |
| hubspoke_dispatch | Hub-Spoke 委派（S2） | D7-S2-A04 | `orchestration/sessionorchestrator/dispatch.go` | `Dispatcher` |
| hubspoke_bridge | Spoke 写侧桥接（S4） | D7-S4-A04/A05 | `orchestration/executionflow/bridge/` | `AgentBridge`, `SubQueryBridge` |
| milestone | D6↔D7 Milestone Bridge | D7-S4 | `orchestration/milestone/` | `Service`, `TaskFlow` |
| **D2 Follower（D7 编排对象）** |||||
| context_preparer | 上下文装配 + CompressHint | D2-S15 | `contextengine/prepare/` | `ContextPreparer` |
| tool_round_executor | 工具权限门控 + 沙箱 | D2-S18 | `contextengine/enforce/` | `ToolRoundExecutor`, `IToolRunner`, `Sandbox` |
| tool_registry | 工具注册表 | D2-S3-A03 | `contextengine/enforce/toolrunner/` | `ToolRegistry`, builtins (bash, read_file, glob, grep, edit) |
| session_persister | 快照 + transcript + commit | D2-S17 | `contextengine/persist/` | `SessionPersister` |
| nested_execution | 嵌套执行 (SubQuery/Background) | D2-S18 | `contextengine/enforce/` | `SubQuery`, `BackgroundTask` |
| worker_dir_sandbox | delegate worker 目录沙箱 | D2-S18 | `contextengine/sandbox/` | `Manager`, `Enter`, `Exit` |
| compression | 七步压缩管道 | D2-S2 | `contextengine/compression/` | `RunPipeline` |
| memory | LongTerm Recall + 快照 | D2-S3 | `contextengine/memory/` | `LoadOrInit`, `PersistSnapshot` |
| **D4 多智能体（D7 编排对象）** |||||
| delegate | Hub-Spoke 委派 + FlowBridge | D4-S10 | `multiagent/delegate/` | `Service`, `FlowBridge`, `WorkerSpec` |
| worker_engine | Worker 隔离引擎 | D4-S10 | `multiagent/agent/` | `WorkerEngine`, `ProcessOverlay` |
| agent_permission | AgentPermissionGate | D4-S11 | `multiagent/permission/` | `AgentPermissionGate` |

### DEPRECATED 模块（v1.x 退役，fallback only）

| L4 ID | 状态 | 替代路径 | 退役依据 |
|-------|------|---------|---------|
| query_loop (D2-S10) | **REMOVED** | D7-S2-A06 turn.RunTurn | DM-20260618-010 |
| subquery | **DEPRECATED** | D7-S2-A04 hubspoke.Dispatcher | DM-20260616-004（PR #55 archive）|
| sidechain_transcript | **DEPRECATED** | WorkItem v2 unified | DM-20260616-004（queryloop legacy decommission）|
| harness (D2-S9) | **DEPRECATED** | — | D6 PathRegressionProbe 监控 legacy 计数 |
| context.pev.* span 族 | RETIRED | — | D2-S1 PEV RETIRED 2026-06-13 |

---

## Shared Contracts (Cross-Layer)

| 契约 | 路径 | 消费方 |
|------|------|--------|
| `FlowEvent`, `ExecutionFlowHub` | `shared/contracts/execution_flow.go` | D7-S4 Hub, D7-S2-A04 hubspoke, D4 Delegate |
| `WorkPlanSnapshot` | `shared/contracts/execution_flow.go` | D7-S4 WorkPlan, delegate_status |
| `IPermissionGate`, `FileAutoApprover` | `shared/contracts/permission.go` | D2 ContextEngine, D1 Gateway, D4 Agent |
| `ILLMGateway`, `ITierResolver` | `llmgateway/contracts.go` | **D7-S2-A07 LLMInvoker** via `bridges/llm`（D2→D3 import ban）|
| `IToolRunner`, `IToolRegistry` | `contextengine/enforce/toolrunner/` | D2 ToolRoundExecutor, D7-S2-A06 turn.RunTurn |
| `ToolRegistry`, builtins | `contextengine/enforce/toolrunner/` | bash, read_file, glob, grep, edit, **5 surface** (DM-20260618-007) |
| `WorkItem`, `WorkTree` v2 | `orchestration/workmodel/workitem.go` | D7-S1 TaskManager, D7-S2-A06 turn.RunTurn, D7-S5 LLMDecomposer |
| `RunRegistry` | `orchestration/workmodel/run_registry.go` | D7-S1 + D7-S2-A06 |
| `ResolveAwaiter`, `FocusHint` | `orchestration/workmodel/awaiter.go` | D7-S2-A06 turn.RunTurn (v2.0 unified blocking) |
| `IntentKind`, `IntentClassification` | `orchestration/orchtypes/intent.go` | D7-S5-A01 ClassifyIntent, D7-S2-A01 Dispatch |
| `ContextPreparer`, `ToolRoundExecutor`, `SessionPersister` | `contextengine/contracts.go` | D7-S2-A06 turn.RunTurn (DM-020 D2 Follower 拆面契约) |
| `ExecutionFlowConfig` | `shared/config/execution_flow.go` | bootstrap, Hub |
| `WorktreeConfig` | `shared/config/worktree.go` | delegate, worktree |
| `DelegateConfig` | `shared/config/` (multi_agent) | delegate Service |
| `WorkModelConfig` (v2.0) | `shared/config/workmodel.go` | D7-S1 TaskManager (store_dir, mode=v2) |
| `WaveConfig` | `shared/config/wave.go` | D7-S3 WaveScheduler (pool_capacity, conflict_groups) |

---

## Bootstrap Wiring

| 文件 | 职责 |
|------|------|
| **`internal/bootstrap/wire_coordinator.go::WireD7`** | **D7 全 wiring**：`sessionorchestrator.Entry`（`coordinator.Entry` shim）+ `turn.NewOrchestrator` + `workmodel.TaskManager` + `wavescheduler.Scheduler` + `sessionorchestrator.Dispatcher` + `milestone.Service` |
| `internal/bootstrap/execution_flow.go` | ExecutionFlowHub 全局注册、WorkPlan 注入 |
| `internal/bootstrap/delegate.go` | DelegateService、SubQuery fallback、worktree |
| `internal/bootstrap/cli_events.go` | CLI worker_progress 渲染 |

入口：`cmd/devrix/main.go` 在 `orchestration.enabled=true` 时调用 `WireD7`（默认）；`multi_agent.enabled=true` 时额外调用 `WireDelegate`。

---

## Dependency Direction

```
D1 Gateway ──route──► D7-S2 coordinator.Entry.ProcessMessage
                              │
                              ├─ D7-S2-A02 ClassifyIntent
                              │     └─ D7-S5 LLM Decomposer (5s timeout)
                              │
                              ├─ D7-S2-A03 Dispatch (4 IntentKind)
                              │     ├─ IntentSkip        → close channel
                              │     ├─ IntentCommand     → D7-S1 workmodel.CLICommands
                              │     ├─ IntentFast        → D7-S2-A06 turn.RunTurn
                              │     │                        ├─ D7-S2-A07 LLMInvoker ─► D3 ILLMGateway
                              │     │                        └─ D2 Follower (ContextPreparer / ToolRoundExecutor / SessionPersister)
                              │     └─ IntentOrchestrate → D7-S5 LLMDecomposer → D7-S3 WaveScheduler
                              │                                  └─ D4 Delegate (Worker)
                              │
                              └─ D7-S4 flow.GlobalHub.Publish
                                    ├─ WorkPlan.Apply
                                    ├─ SessionQueue (delegate-progress)
                                    └─ IM Sink (worker_progress → 飞书卡片)

D4 Delegate.Service ──publish──► D7-S4 ExecutionFlowHub
D2 Follower ──prepare/toolround/persist──► D7-S2-A06 turn.RunTurn (拆面契约)
D7-S2-A07 LLMInvoker ──direct──► D3 ILLMGateway (D2→D3 import ban CI 硬阻断)
```

**禁止红线：**
- ORCH 包 import D1 adapter 实现（避免反向依赖）
- D4 Worker import delegate 工具注册（避免循环）
- **D2 import D3**（`internal/lint/layer/d2_d3_ban_test.go` CI 硬阻断 4/4 白名单已满，DM-020 产权）

---

## Test Placement

| T 层域 | 测试目录 |
|--------|----------|
| D7-S2 SessionOrchestrator | `orchestration/sessionorchestrator/orchestrator_test.go`, `entry_test.go`, `command_handler_test.go`, `orchestrate_path_test.go` |
| D7-S5 ClassifyIntent | `orchestration/decisionplanning/classifier_test.go`, `classifier_fallback_test.go`, `shadow_classifier_test.go` |
| D7-S2-A06 turn.RunTurn | `orchestration/turn/orchestrator_test.go`, `loop_legacy_test.go` |
| D7-S2-A07 LLMInvoker | `orchestration/turn/llm_test.go`, `query_llm_caller_test.go` |
| D7-S1 WorkModel v2 | `orchestration/workmodel/{workitem,run_registry,awaiter,task_manager,plan_mode,plan_agent}_test.go` |
| D7-S3 Wave | `orchestration/wavescheduler/scheduler_test.go`, `conflict_test.go` |
| D7-S4 Flow Hub | `orchestration/executionflow/hub/hub_test.go`, `executionflow/workplan/service_test.go` |
| D7-S2-A04 Hub-Spoke | `orchestration/hubspoke/hubspoke_test.go`, `sessionorchestrator/dispatch.go` |
| D2-S10 QueryLoop (REMOVED) | `orchestration/turn/orchestrator_test.go`, `turn/subturn_test.go` |
| D2-S12 Worktree | `contextengine/worktree/manager_test.go` |
| D2-S15/S17/S18 D2 Follower | `contextengine/{prepare,persist,enforce}/*_test.go` |
| D4-S10 Delegate | `multiagent/delegate/*_test.go`, `contextengine/delegate_*_test.go` |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-10 | QueryLoop v2 module index (DM-20260610-012) |
| 1.1.0 | 2026-06-13 | +toolrunner; docs/architecture 可读版同步 |
| **1.3.0** | **2026-06-19** | **D7 v2.0 Structure 路径对齐**（DM-20260619-005）：sessionorchestrator / decisionplanning / wavescheduler / executionflow / orchtypes |
| **1.2.0** | **2026-06-19** | **D7 v2.0 unified 主索引替换**（DM-20260619-001, PR docs-only）：D7-S2 主入口 / 4 IntentKind / turn.RunTurn / WorkItem v2 / WaveScheduler / 5 surface；QueryLoop / SubQuery / sidechain_transcript 标 DEPRECATED；D2→D3 import ban 显式标注；Bootstrap Wiring 加 wire_coordinator.go |
