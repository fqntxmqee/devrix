# D4 Multi-Agent Domain — T 层测试点注册表

**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## D4-S1: Factory Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D4-S1-A01-T01 | AgentFactory 创建 Agent 实例 | Factory | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED |

## D4-S2: Agent Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D4-S2-A01-T01 | Agent 生命周期状态转换 | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| D4-S2-A02-T02 | AgentPermissionGate 批准/拒绝/超时 | Agent | `internal/layers/multiagent/agent/perm_gate_test.go` | IMPLEMENTED |
| D4-S2-A02-T03 | CRITICAL 工具权限异步流程 | Agent | `internal/layers/multiagent/agent/perm_gate_test.go` | IMPLEMENTED |

## D4-S3: ForkJoin Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D4-S3-A01-T01 | Fork/Join 消息隔离模型 | ForkJoin | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| D4-S3-A01-T02 | Fork 双层限额 MaxChildren+MaxTotalAgents | ForkJoin | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED |
| D4-S3-A01-T03 | Agent 超时自动终止 | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| D4-S3-A01-T04 | Context 取消传播到子 Agent | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| D4-S3-A02-T05 | Join 排序 + tool_call ID 去重 + SessionView COW | SessionView | `internal/layers/multiagent/sessionview/sessionview_test.go` | IMPLEMENTED |

## D4-S4: Collaboration Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D4-S4-A01-T01 | CoT prompt 增强 | Collaboration | `internal/layers/multiagent/collaboration/mode_test.go` | IMPLEMENTED |
| D4-S4-A01-T02 | Iterative-Refinement prompt 增强 | Collaboration | `internal/layers/multiagent/collaboration/mode_test.go` | IMPLEMENTED |

## D4-S5: Observer Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D4-S5-A01-T01 | ObserverAdapter 桥接 AgentEvent → IObserver | Observer | `internal/layers/multiagent/observer/adapter.go` | IMPLEMENTED |

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

## D4-S10: Delegate Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D4-S10-A01-T01 | Leader delegate_explore 创建 Worker / MaxWorkers | Delegate | `internal/layers/multiagent/delegate/service_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T02 | Worker Run 设置 AgentID，sidechain 隔离 | Delegate | `internal/layers/contextengine/worker_tools_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T03 | Worker 不能 delegate_* 或 Fork | Delegate | `internal/layers/multiagent/agent/worker_engine_test.go` | IMPLEMENTED | P0 |
| D4-S10-A02-T04 | delegate-progress 仅 Leader Drain | Delegate | `internal/layers/contextengine/queue/delegate_progress_test.go` | IMPLEMENTED | P0 |
| D4-S10-A02-T05 | worker_progress 到达 Gateway/IM | Delegate | `internal/layers/orchestration/imsink/gateway_test.go` | IMPLEMENTED | P0 |
| D4-S10-A02-T06 | SubQuery 与 D4 Worker 共用 FlowEvent schema | Delegate | `internal/layers/orchestration/flow/hub_test.go` | IMPLEMENTED | P0 |
| D4-S10-A02-T07 | FlowStarted 自动 task owner + in_progress | Delegate | `internal/layers/orchestration/flow/hub_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T08 | D4 未启用 delegate 降级 SubQuery | Delegate | `internal/layers/contextengine/delegate_fallback_flow_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T09 | 用户单会话：无第二对话入口 | Delegate | `internal/bootstrap/cli_events_test.go` | IMPLEMENTED | P0 |

## D4: Cross-Scenario Tests

| T ID | 描述 | Test 位置 | Status |
|-------|------|-----------|--------|
| D4-S0-A01-T01 | Agent 并发安全 (-race) | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| D4-S0-A01-T02 | Fork 消息隔离并发安全 | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| D4-S0-A01-T03 | Gateway → ResolvePermission 集成全流程 | `tests/integration/agent_integration_test.go` | IMPLEMENTED |
| D4-S0-A01-T04 | E2E Fork 端到端 | `tests/e2e/agent_fork_e2e_test.go` | IMPLEMENTED |

---

## Statistics

| Total | IMPLEMENTED | P0 |
|-------|-------------|-----|
| 24 | 24 | 9 |
