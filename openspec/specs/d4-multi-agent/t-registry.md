# D4 Multi-Agent Domain — T 层测试点注册表

**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`

---

## D4-S1: Factory Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D4-S1-A01-T01 | AgentFactory 创建 Agent 实例 | Factory | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED | P0 |
| D4-S1-A01-T02 | 拒绝缺少 session_id 的配置 | Factory | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED | P1 |
| D4-S1-A01-T03 | CreateWithView 绑定 View 到 Agent | Factory | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED | P0 |
| D4-S1-A01-T04 | 根 Agent 使用共享引擎，Worker 使用隔离引擎 | Factory | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED | P1 |
| D4-S1-A01-T05 | 执行 max_total_agents 会话级限额 | Factory | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED | P0 |

## D4-S2: Agent Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D4-S2-A01-T01 | Agent 生命周期状态转换 | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED | P0 |
| D4-S2-A02-T02 | AgentPermissionGate 批准/拒绝/超时 | Agent | `internal/layers/multiagent/agent/perm_gate_test.go` | IMPLEMENTED | P0 |
| D4-S2-A02-T03 | CRITICAL 工具权限异步流程 | Agent | `internal/layers/multiagent/agent/perm_gate_test.go` | IMPLEMENTED | P1 |

## D4-S3: ForkJoin Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D4-S3-A01-T01 | Fork/Join 消息隔离模型 | ForkJoin | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED | P0 |
| D4-S3-A01-T02 | Fork 双层限额 MaxChildren+MaxTotalAgents | ForkJoin | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED | P0 |
| D4-S3-A01-T03 | Agent 超时自动终止 | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED | P1 |
| D4-S3-A01-T04 | Context 取消传播到子 Agent | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED | P1 |
| D4-S3-A01-T05 | Fork metadata 写入不污染父 Session (COW) | SessionView | `internal/layers/multiagent/agent/forkjoin_isolation_test.go` | IMPLEMENTED | P0 |
| D4-S3-A01-T06 | 并发 Fork 三子 Agent Join 一致性 | ForkJoin | `internal/layers/multiagent/agent/forkjoin_isolation_test.go` | IMPLEMENTED | P0 |
| D4-S3-A02-T07 | Join 排序 + tool_call ID 去重 + SessionView COW | SessionView | `internal/layers/multiagent/sessionview/sessionview_test.go` | IMPLEMENTED | P0 |
| D4-S3-A02-T08 | Join dedupToolCallMessages 正确去重 | ForkJoin | `internal/layers/multiagent/agent/forkjoin_isolation_test.go` | IMPLEMENTED | P1 |

## D4-S4: Collaboration Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D4-S4-A01-T01 | CoT prompt 增强 | Collaboration | `internal/layers/multiagent/collaboration/mode_test.go` | IMPLEMENTED | P1 |
| D4-S4-A01-T02 | Iterative-Refinement prompt 增强 | Collaboration | `internal/layers/multiagent/collaboration/mode_test.go` | IMPLEMENTED | P1 |

## D4-S5: Observer Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D4-S5-A01-T01 | NoOpAgentObserver 安全丢弃事件不 panic | Observer | `internal/layers/multiagent/observer/noop.go` | IMPLEMENTED | P1 |

## D4-S6: Agent Tool Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D4-S6-A01-T01 | Agent Tool Registry 注册/查找/按能力查询 | AgentTool | `internal/layers/multiagent/tool/registry_test.go` | IMPLEMENTED | P0 |
| D4-S6-A02-T02 | CLI 适配器正常启动子进程并解析 stream-json | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P0 |
| D4-S6-A02-T03 | CLI 适配器超时正确终止子进程 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P1 |
| D4-S6-A02-T04 | Session 首次创建子进程，后续调用复用同一进程 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P0 |
| D4-S6-A02-T05 | Session 空闲超时自动回收子进程 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P1 |
| D4-S6-A02-T06 | D1 Session 销毁清理关联的 Agent Tool 子进程 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P1 |
| D4-S6-A02-T07 | 不同 D1 Session 的 Agent Tool 隔离运行互不干扰 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P0 |
| D4-S6-A02-T08 | Cursor 适配器基本文本执行 | AgentTool | `internal/layers/multiagent/tool/cursor_adapter_test.go` | IMPLEMENTED | P1 |
| D4-S6-A02-T09 | Cursor 适配器 Session 复用与并发隔离 | AgentTool | `internal/layers/multiagent/tool/cursor_adapter_test.go` | IMPLEMENTED | P1 |
| D4-S6-A02-T10 | Cursor 适配器超时/错误处理 | AgentTool | `internal/layers/multiagent/tool/cursor_adapter_test.go` | IMPLEMENTED | P1 |
| D4-S6-A02-T11 | Cursor 适配器 ToolCall + Thinking 事件解析 | AgentTool | `internal/layers/multiagent/tool/cursor_adapter_test.go` | IMPLEMENTED | P1 |
| D4-S6-A03-T12 | ParseStreamJSONLine devrix 格式解析 | AgentTool | `internal/layers/multiagent/tool/stream_json_test.go` | IMPLEMENTED | P1 |
| D4-S6-A03-T13 | ParseStreamJSONLine Claude assistant/result 解析 | AgentTool | `internal/layers/multiagent/tool/stream_json_test.go` | IMPLEMENTED | P1 |
| D4-S6-A03-T14 | CLI 适配器 Claude stream-json 端到端 | AgentTool | `internal/layers/multiagent/tool/stream_json_test.go` | IMPLEMENTED | P1 |

## D4-S8: Observability Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D4-S8-A01-T01 | IncForkSessionView 按 policy 计数 | Observability | `internal/layers/multiagent/observability/metrics_test.go` | IMPLEMENTED | P1 |
| D4-S8-A01-T02 | IncForkSessionView 并发原子性 | Observability | `internal/layers/multiagent/observability/metrics_test.go` | IMPLEMENTED | P1 |

## D4-S10: Delegate Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D4-S10-A01-T01 | Leader delegate_explore 创建 Worker / MaxWorkers | Delegate | `internal/layers/multiagent/delegate/service_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T02 | Worker Run 设置 AgentID，sidechain 隔离 | Delegate | `internal/layers/contextengine/worker_tools_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T03 | Worker 不能 delegate_* 或 Fork | Delegate | `internal/layers/multiagent/agent/worker_engine_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T04 | Worker Engine Process 注入 overlay sidechain | Delegate | `internal/layers/multiagent/agent/worker_engine_test.go` | IMPLEMENTED | P1 |
| D4-S10-A01-T05 | DelegateSync 在 worktree 沙箱运行 Worker | Delegate | `internal/layers/multiagent/delegate/service_worktree_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T06 | DelegateAsync 异步通知 enqueue 主线程 | Delegate | `internal/layers/multiagent/delegate/service_async_test.go` | IMPLEMENTED | P1 |
| D4-S10-A01-T07 | D4 未启用 delegate 降级 SubQuery | Delegate | `internal/layers/contextengine/delegate_fallback_flow_test.go` | IMPLEMENTED | P0 |
| D4-S10-A02-T08 | delegate-progress 仅 Leader Drain | Delegate | `internal/layers/contextengine/queue/delegate_progress_test.go` | IMPLEMENTED | P0 |
| D4-S10-A02-T09 | worker_progress 到达 Gateway/IM | Delegate | `internal/layers/orchestration/imsink/gateway_test.go` | IMPLEMENTED | P0 |
| D4-S10-A02-T10 | SubQuery 与 D4 Worker 共用 FlowEvent schema | Delegate | `internal/layers/orchestration/flow/hub_test.go` | IMPLEMENTED | P0 |
| D4-S10-A02-T11 | FlowStarted 自动 task owner + in_progress | Delegate | `internal/layers/orchestration/flow/hub_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T12 | 用户单会话：无第二对话入口 | Delegate | `internal/bootstrap/cli_events_test.go` | IMPLEMENTED | P0 |

## D4: Cross-Scenario Tests

| T ID | 描述 | Test 位置 | Status | Priority |
|-------|------|-----------|--------|----------|
| D4-S0-A01-T01 | Agent 并发安全 (-race) | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED | P0 |
| D4-S0-A01-T02 | Fork 消息隔离并发安全 | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED | P0 |
| D4-S0-A01-T03 | Gateway → ResolvePermission 集成全流程 | `tests/integration/agent_integration_test.go` | IMPLEMENTED | P0 |
| D4-S0-A01-T04 | E2E Fork 端到端 | `tests/e2e/agent_fork_e2e_test.go` | IMPLEMENTED | P0 |

---

## Statistics

| Total | IMPLEMENTED | P0 |
|-------|-------------|-----|
| 38 | 38 | 19 |

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | Initial T registry (24 test points, 9 P0) |
| 2.0.0 | 2026-06-14 | Fixed D4-S5-A01-T01 path (observer/adapter.go→observer/noop.go); added forkjoin_isolation tests (T05/T06/T08); added cursor_adapter tests (T08-T11); added stream_json tests (T12-T14); added observability metrics tests (S8); added delegate async/worktree tests (T04-T06); added factory validation tests (T02-T04); 38 total, 19 P0 |
