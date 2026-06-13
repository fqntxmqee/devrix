# D4 Multi-Agent Layer Specification

**Capability:** multi-agent
**Version:** 2.0.0
**Status:** Canonical — source of truth
**Last Updated:** 2026-06-13
**Layering Spec:** `openspec/specs/architecture/layering.md`

---

## Overview

D4 多智能体域负责 Agent 生命周期状态机、Fork/Join 并行子 Agent、协作模式提示词增强、异步权限门（AgentPermissionGate）、Hub-Spoke 委托编排（Delegate Service）、Agent Tool 注册与执行、SessionView COW 隔离和 Agent 可观测性指标。

## Scenarios

| ID | Scenario | Responsibility | Status |
|----|----------|----------------|--------|
| D4-S1 | Factory | Agent 工厂创建与会话配额管理 | IMPLEMENTED |
| D4-S2 | Agent | 生命周期状态机、Run/Wait/Terminate | IMPLEMENTED |
| D4-S3 | ForkJoin | Fork/Join 并行子 Agent + 消息隔离 | IMPLEMENTED |
| D4-S4 | Collaboration | 推理模式校验与提示词增强（CoT/IR） | IMPLEMENTED |
| D4-S5 | Observer | Agent 事件桥接到 IObserver / AgentObserverChain | IMPLEMENTED |
| D4-S6 | AgentTool | CLI/Cursor Agent Tool 注册与 session 管理 | IMPLEMENTED |
| D4-S7 | Builtin | SubQuery 内置 Agent (Explore/Plan/Implement) | IMPLEMENTED |
| D4-S8 | Observability | Fork SessionView 策略计数器 + D5Sink | IMPLEMENTED |
| D4-S9 | SessionView | COW Fork 隔离 View（DM-20260611-005） | IMPLEMENTED |
| D4-S10 | Delegate | Hub-Spoke 委托编排（同步/异步/回退） | IMPLEMENTED |

## Architecture

```
Gateway (D1) ──→ AgentFactory (D4-S1)
                      │
                      ├─→ Agent (D4-S2): lifecycle state machine
                      │     ├─ Fork/Join (D4-S3): child agent creation
                      │     ├─ PermissionGate (D4-S2): async permission via channel
                      │     ├─ WorkerEngine (D4-S2): sidechain context overlay
                      │     └─ SessionView (D4-S9): COW fork isolation
                      │
                      ├─→ Delegate Service (D4-S10): Hub-Spoke orchestration
                      │     ├─ FlowBridge → ExecutionFlowHub (D7)
                      │     ├─ Worktree isolation
                      │     └─ Async notification → SessionQueue (D2)
                      │
                      ├─→ Collaboration (D4-S4): prompt enhancement
                      ├─→ Agent Tools (D4-S6): CLI/Cursor subprocess
                      ├─→ Builtin Agents (D4-S7): SubQuery fallback
                      ├─→ Observer (D4-S5): event bridge → IObserver
                      └─→ Observability (D4-S8): metrics counters
```

## Cross-Domain Dependencies

| Domain | 依赖内容 | 使用位置 |
|--------|---------|---------|
| D1 Communication | `PermissionManager` → `PermissionGateAdapter` | agent/perm_gate → gateway |
| D2 Context Engine | `contracts.IEngine`, `query.LoopDeps`, `queue.SessionQueue`, SubQuery | agent, delegate, builtin |
| D5 Observability | `observability.Bridge` (tracer) | factory, agent |
| D7 Orchestration | `contracts.ExecutionFlowHub`, `flow.GlobalHub` | delegate/bridge |
| Shared | `contracts`, `config`, `errors`, `types` | 全子包 |

## Package Map

| 子包 | 场景 | 职责 |
|------|------|------|
| `contracts.go` (root) | D4-S1~S5 | 核心接口与类型: Agent, IAgentFactory, AgentState, AgentConfig, AgentEvent, PermissionGate, AgentObserver |
| `agent/` | D4-S2, D4-S3 | Impl (Agent 实现), 状态机, Fork/Join, WorkerEngine, PermissionGate |
| `factory/` | D4-S1 | AgentFactory (Create/CreateWithView/ReleaseSession), 配额管理, 配置校验 |
| `collaboration/` | D4-S4 | ValidateMode, BuildPromptForMode (CoT/IR 提示词增强) |
| `delegate/` | D4-S10 | Service (Hub-Spoke 委托), FlowBridge, WorkerSpec, DelegateResult |
| `tool/` | D4-S6 | AgentTool 接口, CLI/Cursor 适配器, Registry, stream-json 解析 |
| `builtin/` | D4-S7 | SubQuery 内置 Agent: RunExplore/RunPlan/RunImplement |
| `observer/` | D4-S5 | NoOpAgentObserver, AgentObserverChain (contracts.go) |
| `sessionview/` | D4-S9 | COW Fork 隔离 View (Fork/Create/SetMetadata/SetSnapshot/MergeToParent) |
| `observability/` | D4-S8 | ForkSessionView 策略计数器, D5Sink, Policy 常量 |

## Key Design Patterns

1. **Agent 状态机**: `CREATED → RUNNING → ITERATING → WAITING_PERMISSION → TERMINATED`，严格的状态转换表 (`agent/state.go`)，非法转换返回 `AgentInvalidTransitionError`。
2. **Fork/Join 消息隔离**: 子 Agent 独立消息缓冲区，Join 时合并到父 Agent，通过 `tool_call_id` 去重。SessionView COW 模式确保子 Agent 写入不污染父 Session。
3. **AgentPermissionGate**: Agent 实现 `PermissionGate` 接口，CRITICAL 工具通过 channel 阻塞等待 Gateway 注入用户响应，非 CRITICAL 工具直接返回 true。
4. **WorkerEngine**: `agent.NewWorkerEngine` 包装 `contracts.IEngine`，通过 `ProcessOverlay` 为 worker 注入 sidechain context（AgentID/WorkerRole/SystemPrompt/ModelTier）。
5. **Hub-Spoke Delegate**: Leader Agent 调用 `delegate_explore/plan/implement` 工具 → `Service.DelegateOrFallback` → Fork Worker → FlowBridge 发布生命周期事件到 `ExecutionFlowHub`。
6. **SessionView COW**: `sessionview.Fork(parent)` 创建子 View，不可变字段共享，可变字段（metadata/snapshot）隔离，`MergeToParent` 时写入父 Session。

## Registries

- **A 层**: `a-registry.md` — 16 Activities
- **F 层**: `f-registry.md` — 30 Function Points
- **T 层**: `t-registry.md` — Test Points (IMPLEMENTED)

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-08 | Initial Multi-Agent V1 (DM-20260608-005) |
| 1.1.0 | 2026-06-10 | D4-S10 Hub-Spoke Delegate (DM-20260610-012) |
| 2.0.0 | 2026-06-13 | SessionView COW (DM-20260611-005), Agent Tools, Observability counters, 全文档同步 |
