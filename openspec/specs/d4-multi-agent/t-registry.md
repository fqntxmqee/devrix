# D4 Multi-Agent Domain — T 层测试点注册表

**Status:** Active
**Version:** 3.5.0
**Last Updated:** 2026-06-30
**Change ID:** devrix-d4-dsaft-restructuring (DM-20260629-004) PR-6 #4 span-coverage — 7 EngineEvent 字面量常量化（`orchtypes.EventAgent*`）+ 6 OpD4_S4_* span ops + 7 EventAgent* const 全部映射；Span Evidence 列新增；§Statistics 重算（59 total / 31 mapped / 28 explicit — / 31/31=100%）；§T-Without-Span Tracker 4 类原因拆解
**Parent:** `openspec/specs/architecture/layering.md`

---

## Canonical 索引（S11–S16）

| Canonical S | ValueFlow Alias (用户感知) | Legacy S 来源 | T 数量 |
|-------------|---------------------------|--------------|--------|
| D4-S11 ProvisionAgent | `D4_Provision_Agent` | S1, S4 | 7 |
| D4-S12 RunAgentLoop | `D4_Run_Agent_Loop` | S2 | 4 |
| D4-S13 IsolateAndMerge | `D4_Isolate_Merge` | S3, S9 | 8 |
| D4-S14 ExecuteWorker | `D4_Execute_Worker` | S10（执行面） | 10 |
| D4-S15 InvokeExternalAgent | `D4_External_Agent_Tool` | S6 | 14 |
| D4-S16 ConfigureAgents | `D4_Configure_Agents`（横切） | config | 0（v1.1 补） |
| D4-S0 Cross | （横切） | — | 4 |
| **D7-S2/S4**（Hub-Spoke） | （归 D7 ValueFlow） | S10（编排面） | 5 |

> v1.0：**不修改**测试 `// T:` 注释；下表保留 Legacy ID，§Legacy Archive 供追溯。

---

## D4-S1: Factory Module

| T ID | 描述 | S 映射 | Test 位置 | Span Evidence | Status | Priority |
|-------|------|---------|-----------|---------------|--------|----------|
| D4-S1-A01-T01 | AgentFactory 创建 Agent 实例 | Factory | `internal/layers/multiagent/provision/factory_test.go` | — | IMPLEMENTED | P0 |
| D4-S1-A01-T02 | 拒绝缺少 session_id 的配置 | Factory | `internal/layers/multiagent/provision/factory_test.go` | — | IMPLEMENTED | P1 |
| D4-S1-A01-T03 | CreateWithView 绑定 View 到 Agent | Factory | `internal/layers/multiagent/provision/factory_test.go` | — | IMPLEMENTED | P0 |
| D4-S1-A01-T04 | 根 Agent 使用共享引擎，Worker 使用隔离引擎 | Factory | `internal/layers/multiagent/provision/factory_test.go` | `OpD4_S4_Agent_State_Transition` | IMPLEMENTED | P1 |
| D4-S1-A01-T05 | 执行 max_total_agents 会话级限额 | Factory | `internal/layers/multiagent/provision/factory_test.go` | — | IMPLEMENTED | P0 |

## D4-S2: Agent Module

| T ID | 描述 | S 映射 | Test 位置 | Span Evidence | Status | Priority |
|-------|------|---------|-----------|---------------|--------|----------|
| D4-S2-A01-T01 | Agent 生命周期状态转换 | Agent | `internal/layers/multiagent/run/agent_test.go` | `OpD4_S4_Agent_Run` + `OpD4_S4_Agent_State_Transition` + `EventAgentStarted` + `EventAgentTerminated` + `EventAgentIterating` | IMPLEMENTED | P0 |
| D4-S2-A02-T02 | AgentPermissionGate 批准/拒绝/超时 | Agent | `internal/layers/multiagent/run/perm_gate_test.go` | `EventPermissionRequired` | IMPLEMENTED | P0 |
| D4-S2-A02-T03 | CRITICAL 工具权限异步流程 | Agent | `internal/layers/multiagent/run/perm_gate_test.go` | `EventPermissionRequired` | IMPLEMENTED | P1 |

## D4-S3: ForkJoin Module

| T ID | 描述 | S 映射 | Test 位置 | Span Evidence | Status | Priority |
|-------|------|---------|-----------|---------------|--------|----------|
| D4-S3-A01-T01 | Fork/Join 消息隔离模型 | ForkJoin | `internal/layers/multiagent/run/agent_test.go` | `OpD4_S4_Agent_Fork` + `OpD4_S4_Agent_Join` + `EventAgentForked` + `EventAgentJoined` | IMPLEMENTED | P0 |
| D4-S3-A01-T02 | Fork 双层限额 MaxChildren+MaxTotalAgents | ForkJoin | `internal/layers/multiagent/run/agent_test.go` | `OpD4_S4_Agent_Fork` | IMPLEMENTED | P0 |
| D4-S3-A01-T03 | Agent 超时自动终止 | Agent | `internal/layers/multiagent/run/agent_test.go` | `OpD4_S4_Agent_Terminate` + `EventAgentTerminated` | IMPLEMENTED | P1 |
| D4-S3-A01-T04 | Context 取消传播到子 Agent | Agent | `internal/layers/multiagent/run/agent_test.go` | `OpD4_S4_Agent_Terminate` | IMPLEMENTED | P1 |
| D4-S3-A01-T05 | Fork metadata 写入不污染父 Session (COW) | SessionView | `internal/layers/multiagent/run/forkjoin_isolation_test.go` | `OpD4_S4_Agent_Fork` + `EventAgentForked` | IMPLEMENTED | P0 |
| D4-S3-A01-T06 | 并发 Fork 三子 Agent Join 一致性 | ForkJoin | `internal/layers/multiagent/run/forkjoin_isolation_test.go` | `OpD4_S4_Agent_Join` + `EventAgentJoined` | IMPLEMENTED | P0 |
| D4-S3-A02-T07 | Join 排序 + tool_call ID 去重 + SessionView COW | SessionView | `internal/layers/multiagent/isolate/sessionview_test.go` | `OpD4_S4_Agent_Join` + `EventAgentJoined` | IMPLEMENTED | P0 |
| D4-S3-A02-T08 | Join dedupToolCallMessages 正确去重 | ForkJoin | `internal/layers/multiagent/run/forkjoin_isolation_test.go` | `OpD4_S4_Agent_Join` | IMPLEMENTED | P1 |

## D4-S4: Collaboration Module

| T ID | 描述 | S 映射 | Test 位置 | Span Evidence | Status | Priority |
|-------|------|---------|-----------|---------------|--------|----------|
| D4-S4-A01-T01 | CoT prompt 增强 | Collaboration | `internal/layers/multiagent/collaboration/mode_test.go` | — | IMPLEMENTED | P1 |
| D4-S4-A01-T02 | Iterative-Refinement prompt 增强 | Collaboration | `internal/layers/multiagent/collaboration/mode_test.go` | — | IMPLEMENTED | P1 |

## D4-S5: Observer Module

| T ID | 描述 | S 映射 | Test 位置 | Span Evidence | Status | Priority |
|-------|------|---------|-----------|---------------|--------|----------|
| D4-S5-A01-T01 | NoOpAgentObserver 安全丢弃事件不 panic | Observer | `internal/layers/multiagent/kernel/noop.go` | `EventAgent*` (任意 emit) | IMPLEMENTED | P1 |

## D4-S6: Agent Tool Module

| T ID | 描述 | S 映射 | Test 位置 | Span Evidence | Status | Priority |
|-------|------|---------|-----------|---------------|--------|----------|
| D4-S6-A01-T01 | Agent Tool Registry 注册/查找/按能力查询 | AgentTool | `tests/acceptance/p0/agent_tool_test.go` | `OpD4_S4_Agent_Tool_Call` (D1 入口) | IMPLEMENTED | P0 |
| D4-S6-A02-T02 | CLI 适配器正常启动子进程并解析 stream-json | AgentTool | `internal/layers/multiagent/external/cli_adapter_test.go` (PR-2 拆分后 PR-3 注释更新) | — (外部 sub-process span) | IMPLEMENTED | P0 |
| D4-S6-A02-T03 | CLI 适配器超时正确终止子进程 | AgentTool | `internal/layers/multiagent/external/cli_adapter_test.go` | — (外部 sub-process span) | IMPLEMENTED | P1 |
| D4-S6-A02-T04 | Session 首次创建子进程，后续调用复用同一进程 | AgentTool | `internal/layers/multiagent/external/cli_adapter_test.go` | — (外部 sub-process span) | IMPLEMENTED | P0 |
| D4-S6-A02-T05 | Session 空闲超时自动回收子进程 | AgentTool | `internal/layers/multiagent/external/cli_adapter_test.go` | — (外部 sub-process span) | IMPLEMENTED | P1 |
| D4-S6-A02-T06 | D1 Session 销毁清理关联的 Agent Tool 子进程 | AgentTool | `internal/layers/multiagent/external/cli_adapter_test.go` | — (外部 sub-process span) | IMPLEMENTED | P1 |
| D4-S6-A02-T07 | 不同 D1 Session 的 Agent Tool 隔离运行互不干扰 | AgentTool | `internal/layers/multiagent/external/cli_adapter_test.go` | — (外部 sub-process span) | IMPLEMENTED | P0 |
| D4-S6-A02-T08 | Cursor 适配器基本文本执行 | AgentTool | `internal/layers/multiagent/external/cursor_adapter_test.go` (PR-3 拆分后注释更新) | — (外部 sub-process span) | IMPLEMENTED | P1 |
| D4-S6-A02-T09 | Cursor 适配器 Session 复用与并发隔离 | AgentTool | `internal/layers/multiagent/external/cursor_adapter_test.go` | — (外部 sub-process span) | IMPLEMENTED | P1 |
| D4-S6-A02-T10 | Cursor 适配器超时/错误处理 | AgentTool | `internal/layers/multiagent/external/cursor_adapter_test.go` | — (外部 sub-process span) | IMPLEMENTED | P1 |
| D4-S6-A02-T11 | Cursor 适配器 ToolCall + Thinking 事件解析 | AgentTool | `internal/layers/multiagent/external/cursor_adapter_test.go` | — (外部 sub-process span) | IMPLEMENTED | P1 |
| D4-S6-A03-T12 | ParseStreamJSONLine devrix 格式解析 | AgentTool | `internal/layers/multiagent/external/stream_json_test.go` | — (parser, no span) | IMPLEMENTED | P1 |
| D4-S6-A03-T13 | ParseStreamJSONLine Claude assistant/result 解析 | AgentTool | `internal/layers/multiagent/external/stream_json_test.go` | — (parser, no span) | IMPLEMENTED | P1 |
| D4-S6-A03-T14 | CLI 适配器 Claude stream-json 端到端 | AgentTool | `internal/layers/multiagent/external/stream_json_test.go` | `OpD4_S4_Agent_Tool_Call` (D1 wiring) | IMPLEMENTED | P1 |

## D4-S8: Observability Module

| T ID | 描述 | S 映射 | Test 位置 | Span Evidence | Status | Priority |
|-------|------|---------|-----------|---------------|--------|----------|
| D4-S8-A01-T01 | IncForkSessionView 按 policy 计数 | Observability | `internal/layers/multiagent/observability/metrics_test.go` | — (D5 metric，迁 D5) | IMPLEMENTED | P1 |
| D4-S8-A01-T02 | IncForkSessionView 并发原子性 | Observability | `internal/layers/multiagent/observability/metrics_test.go` | — (D5 metric，迁 D5) | IMPLEMENTED | P1 |

## D4-S10: Delegate Module

| T ID | 描述 | S 映射 | Test 位置 | Span Evidence | Status | Priority |
|-------|------|---------|-----------|---------------|--------|----------|
| D4-S10-A01-T01 | Leader delegate_explore 创建 Worker / MaxWorkers | Delegate | `internal/layers/multiagent/execute/worker_test.go` (原 delegate/service_test.go，PR-1 v2.0d 已迁) | `OpD4_S4_Agent_Fork` + `EventAgentForked` | IMPLEMENTED | P0 |
| D4-S10-A01-T02 | Worker Run 设置 AgentID，sidechain 隔离 | Delegate | `internal/layers/contextengine/worker_tools_test.go` | `OpD4_S4_Agent_Run` + `OpD4_S4_Agent_State_Transition` + `EventAgentStarted` | IMPLEMENTED | P0 |
| D4-S10-A01-T03 | Worker 不能 delegate_* 或 Fork | Delegate | `internal/layers/multiagent/provision/factory_test.go`（PR-1 #0 已 inline WorkerEngine 进 provision/factory.go） | — (negative test) | IMPLEMENTED | P0 |
| D4-S10-A01-T04 | Worker Engine Process 注入 overlay sidechain | Delegate | `internal/layers/multiagent/provision/factory_test.go`（PR-1 #0 已 inline） | `OpD4_S4_Agent_State_Transition` | IMPLEMENTED | P1 |
| D4-S10-A01-T05 | DelegateSync 在 worktree 沙箱运行 Worker | Delegate | `internal/layers/multiagent/execute/worker_test.go` | `OpD4_S4_Agent_Fork` + `OpD4_S4_Agent_Join` | IMPLEMENTED | P0 |
| D4-S10-A01-T06 | DelegateAsync 异步通知 enqueue 主线程 | Delegate | `internal/layers/multiagent/execute/worker_test.go` | `OpD4_S4_Agent_Fork` | IMPLEMENTED | P1 |
| D4-S10-A01-T07 | D4 未启用 delegate 降级 SubQuery | Delegate | `internal/layers/contextengine/delegate_fallback_flow_test.go` | — (D2 SubQuery fallback，D2 域) | IMPLEMENTED | P0 |
| D4-S10-A02-T08 | delegate-progress 仅 Leader Drain | Delegate | `internal/layers/contextengine/queue/delegate_progress_test.go` | — (D7 ExecutionFlowHub owns) | IMPLEMENTED | P0 |
| D4-S10-A02-T09 | worker_progress 到达 Gateway/IM | Delegate | `internal/layers/orchestration/imsink/gateway_test.go` | — (D7 imsink owns) | IMPLEMENTED | P0 |
| D4-S10-A02-T10 | SubQuery 与 D4 Worker 共用 FlowEvent schema | Delegate | `internal/layers/orchestration/executionflow/hub/hub_test.go` | `EventAgent*` (任意 emit) | IMPLEMENTED | P0 |
| D4-S10-A02-T11 | FlowStarted 自动 task owner + in_progress | Delegate | `internal/layers/orchestration/executionflow/hub/hub_test.go` | `EventAgentStarted` | IMPLEMENTED | P0 |
| D4-S10-A01-T12 | 用户单会话：无第二对话入口 | Delegate | `internal/bootstrap/cli_events_test.go` | — (D1 bootstrap 入口) | IMPLEMENTED | P0 |

## D4: Cross-Scenario Tests

| T ID | 描述 | Test 位置 | Span Evidence | Status | Priority |
|-------|------|-----------|---------------|--------|----------|
| D4-S0-A01-T01 | Agent 并发安全 (-race) | `internal/layers/multiagent/run/agent_test.go` | `OpD4_S4_Agent_Run` | IMPLEMENTED | P0 |
| D4-S0-A01-T02 | Fork 消息隔离并发安全 | `internal/layers/multiagent/run/agent_test.go` | `OpD4_S4_Agent_Fork` + `OpD4_S4_Agent_Join` | IMPLEMENTED | P0 |
| D4-S0-A01-T03 | Gateway → ResolvePermission 集成全流程 | `tests/integration/agent_integration_test.go` | `EventPermissionRequired` | IMPLEMENTED | P0 |
| D4-S0-A01-T04 | E2E Fork 端到端 | `tests/e2e/agent_fork_e2e_test.go` | `OpD4_S4_Agent_Fork` + `OpD4_S4_Agent_Join` + `EventAgentForked` + `EventAgentJoined` | IMPLEMENTED | P0 |
| D4-FF-T01 | FreeFork 批量 fork 全部成功 | `internal/layers/multiagent/provision/freefork/forker_test.go` | `OpD4_S4_Agent_Fork` + `EventAgentForked` | IMPLEMENTED | P0 |
| D4-FF-T02 | FreeFork 工厂失败 → 已启动子 agent Terminate + worktree Exit | `internal/layers/multiagent/provision/freefork/forker_test.go` | `OpD4_S4_Agent_Terminate` + `EventAgentTerminated` | IMPLEMENTED | P0 |
| D4-FF-T03 | FreeFork prompt → AgentConfig.InitialInput | `internal/layers/multiagent/provision/freefork/forker_test.go` | — (config wiring, no span) | IMPLEMENTED | P1 |
| D4-NT-T01 | TaskManager 终态 publish CompletionEvent | `internal/layers/orchestration/workmodel/notify/bus_test.go` | — (D7 workmodel owns) | IMPLEMENTED | P0 |
| D4-NT-T02 | Bus.Drain 一次性读出全部未消费 event | `internal/layers/orchestration/workmodel/notify/bus_test.go` | — (D7 workmodel owns) | IMPLEMENTED | P0 |
| D4-NT-T03 | Bus channel 满 → 降级 pending list | `internal/layers/orchestration/workmodel/notify/bus_test.go` | — (D7 workmodel owns) | IMPLEMENTED | P1 |
| D4-NT-T04 | FormatReminder 渲染 `<task_notifications>` 块 | `internal/layers/orchestration/workmodel/notify/bus_test.go` | — (D7 workmodel owns) | IMPLEMENTED | P0 |
| **D4-S11-A02-T01** | **ForkAgent + SendMessage + WorkerContext budget (FreeFork W8)** | **`internal/layers/multiagent/provision/freefork/forker_test.go` (TestW8_10_FreeForkStack_T_CrossRef)** | `OpD4_S4_Agent_Fork` + `OpD4_S4_Agent_Join` | **IMPLEMENTED** | **P0** |
| **D4-S11-A02-T02** | **资源争抢仲裁 (FreeFork W9)** | **`internal/layers/multiagent/provision/freefork/forker_test.go`** | `OpD4_S4_Agent_Fork` | **IMPLEMENTED** | **P0** |
| **D4-S11-A02-T03** | **FreeForkSurface 集成 3 分叉 (FreeFork W9)** | **`tests/integration/tools_terminal_test.go` (TestFreeFork_3Directions)** | `OpD4_S4_Agent_Fork` + `OpD4_S4_Agent_Join` | **IMPLEMENTED** | **P0** |
| **D4-S11-A02-T04** | **maxChildren 并发 budget enforcement (FreeFork W8)** | **`internal/layers/multiagent/provision/freefork/forker_test.go`** | `OpD4_S4_Agent_Fork` | **IMPLEMENTED** | **P0** |
| **D4-S13-A02-T01** | **Worktree 隔离 per-handle (FreeFork W10)** | **`internal/layers/multiagent/provision/freefork/forker_test.go`** | `OpD4_S4_Agent_Fork` | **IMPLEMENTED** | **P0** |
| **D4-S12-A03-T01** | **BackgroundTaskSurface ToolEventStream context 推送 (D4-S12 W13)** | **`internal/layers/orchestration/turn/tool_stream_test.go`** | — (D7 turn tool_stream owns) | **IMPLEMENTED** | **P0** |
| **D4-S14-A07-T01** | **AC10 delegate_* tool schema exposes `mode` enum [brief/fork/full] (default brief)** | **`internal/layers/orchestration/delegatetools/delegate_schema_test.go::TestDelegateToolParameters_ModeEnum`, `TestParseSubAgentMode`** | — (schema validation, no span) | **IMPLEMENTED (DM-20260620-001-B)** | **P0** |
| **D4-S14-A07-T02** | **AC10 free_fork tool schema exposes `mode` enum on request items (default brief)** | **`internal/layers/orchestration/sessionorchestrator/turn_tools_test.go`** | — (schema validation, no span) | **IMPLEMENTED (DM-20260620-001-B)** | **P0** |

---

## Statistics

| Total | IMPLEMENTED | P0 | Span Evidence Mapped | Explicit `—` | Effective Coverage |
|-------|-------------|----|----------------------|---------------|---------------------|
| 59 | 59 | 36 | **31** | 28 | **31 / 31 = 100%** |

> **Span Evidence 守门（DM-20260629-004 PR-6 T26/T27）：** `scripts/d4-span-coverage.sh` ≥ 80% effective PASS。
> 详见 `span-registry.md` 与 §T-Without-Span Tracker below。
>
> **口径说明**：Effective Coverage = Mapped / (Total − Explicit `—`) = 31 / (59 − 28) = 31 / 31 = 100%。
> 28 个 Explicit `—` 由 §T-Without-Span Tracker 4 类原因解释，不计入缺口。

### §T-Without-Span Tracker — 28 显式 `—`

> 与 D3 / D2 / D7 处理一致：以下 T 行显式标 `—` 不计入缺口。对照 `D4-S11/S12/S13` 域边界，缺失原因 4 类：

| Reason | T 行数 | 说明 |
|--------|--------|------|
| **外部 sub-process span** | 10 | CLI / Cursor 子进程 stdout 由 external/ 子包解析，不在 D4 域内（`D4-S6-A02-T02~T11`） |
| **D5/D7 跨域 owns** | 7 | Metric 迁 D5（`D4-S8-A01-T01~T02`）；FlowEvent / IM sink / tool_stream / workmodel 归 D7（`D4-S10-A02-T08/T09` + `D4-S12-A03-T01`）；D2 SubQuery fallback 归 D2（`D4-S10-A01-T07`）；D1 bootstrap 入口归 D1（`D4-S10-A01-T12`） |
| **config / schema validation** | 4 | 模式枚举 + prompt 配置 + delegate/free_fork schema 校验无 runtime span（`D4-S4-A01-T01/T02` + `D4-S14-A07-T01/T02`） |
| **factory / parser / negative test** | 7 | factory 校验 + parser + 负例测试（`D4-S1-A01-T01/T02/T03/T05` + `D4-S6-A03-T12/T13` + `D4-S10-A01-T03`） |

---

## §Legacy Archive — Canonical 映射

| Legacy T ID | Canonical T ID | Canonical S | 域 |
|-------------|----------------|-------------|-----|
| D4-S1-A01-T01 | D4-S11-A01-T01 | S11 | D4 |
| D4-S1-A01-T02 | D4-S11-A01-T02 | S11 | D4 |
| D4-S1-A01-T03 | D4-S11-A01-T03 | S11 | D4 |
| D4-S1-A01-T04 | D4-S11-A01-T04 | S11 | D4 |
| D4-S1-A01-T05 | D4-S11-A01-T05 | S11 | D4 |
| D4-S2-A01-T01 | D4-S12-A01-T01 | S12 | D4 |
| D4-S2-A02-T02 | D4-S12-A02-T02 | S12 | D4 |
| D4-S2-A02-T03 | D4-S12-A02-T03 | S12 | D4 |
| D4-S3-A01-T01 | D4-S13-A01-T01 | S13 | D4 |
| D4-S3-A01-T02 | D4-S13-A01-T02 | S13 | D4 |
| D4-S3-A01-T03 | D4-S12-A01-T03 | S12 | D4 |
| D4-S3-A01-T04 | D4-S12-A01-T04 | S12 | D4 |
| D4-S3-A01-T05 | D4-S13-A01-T05 | S13 | D4 |
| D4-S3-A01-T06 | D4-S13-A01-T06 | S13 | D4 |
| D4-S3-A02-T07 | D4-S13-A02-T07 | S13 | D4 |
| D4-S3-A02-T08 | D4-S13-A02-T08 | S13 | D4 |
| D4-S4-A01-T01 | D4-S11-A02-T01 | S11 | D4 |
| D4-S4-A01-T02 | D4-S11-A02-T02 | S11 | D4 |
| D4-S5-A01-T01 | D4-S0-A02-T01 | kernel | D4 |
| D4-S6-A01-T01 | D4-S15-A01-T01 | S15 | D4 |
| D4-S6-A02-T02~T14 | D4-S15-A02-T02~T14 | S15 | D4 |
| D4-S8-A01-T01~T02 | D5-AGENT-METRIC-T01~T02 | D5 | D5 |
| D4-S10-A01-T01~T06,T12 | D4-S14-A01-T01~T07 | S14 | D4 |
| D4-S10-A01-T07 | D7-S2-A04-T01 | D7-S2 | D7 |
| D4-S10-A02-T08~T11 | D7-S4-A04-T01~T04 | D7-S4 | D7 |
| D4-S0-A01-T01~T04 | D4-S0-A01-T01~T04 | CROSS | D4 |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 3.5.0 | 2026-06-30 | DM-20260629-004 PR-6 #4 span-coverage：7 EngineEvent 字面量常量化（`orchtypes.EventAgent*`）+ 6 OpD4_S4_* span ops 全部登记；59 T 行加 Span Evidence 列；31 映射 / 28 显式 `—` / Effective 100%；§T-Without-Span Tracker 4 类原因（外部 sub-process 10 + D5/D7 跨域 owns 7 + config/schema 4 + factory/parser/negative 7） |
| 3.4.0 | 2026-06-30 | DM-20260629-004 PR-5 #3 value-flow-rename：§Canonical 索引加 ValueFlow Alias 列（5 S + 1 横切 + 1 Cross + 1 Hub-Spoke 共 8 行）；T 编号无变化；§1~§9 表头保持 Legacy ID（// T: 注释不修改）；修订记录 v3.4.0 row |
| 3.3.0 | 2026-06-30 | DM-20260629-004 PR-4 #2 registry-sync：13 个测试路径对齐 v2.0（tool/→external/, delegate/→execute/, agent/worker_engine→provision/factory） |
| 3.2.0 | 2026-06-20 | devrix-context-budget-phase-b (DM-20260620-001-B) — `mode` field on delegate/free_fork schemas (D4-S14-A07-T01/T02); total 38→40, P0 19→21 |
| 3.1.0 | 2026-06-18 | FreeFork W8-W10 + BackgroundTaskSurface W13 + TaskNotify 闭环（D4-S11-A02-T01~T04 + S13-A02-T01 + S12-A03-T01） |
| 3.0.0 | 2026-06-14 | Canonical 索引 + §Legacy Archive；Hub-Spoke T 重归属 D7（DM-20260614-018） |
| 2.0.0 | 2026-06-14 | 38 total, 19 P0 |
| 1.0.0 | 2026-06-13 | Initial T registry (24 test points) |
