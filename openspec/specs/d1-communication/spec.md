# D1 Communication Domain Specification

**Capability:** communication
**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-13

---

## Overview

通信域负责消息网关、多协议适配器（飞书/钉钉/CLI）、会话管理、权限、限流、背压事件总线、消息渲染、CardKit 流式卡片、Worker 卡片和 Agent 路由。作为 Devrix 的入口层，所有用户交互经由此域进入系统。

## Scenarios

| ID | Scenario | Responsibility | Status |
|----|----------|----------------|--------|
| D1-S1 | Gateway | 消息网关、路由、会话管理、Agent 路由、事件分发 | IMPLEMENTED |
| D1-S2 | Adapters | 飞书（WebSocket/Webhook/CardKit 流式/Worker 卡片）、钉钉、CLI 适配器 | IMPLEMENTED |
| D1-S3 | Commands | CLI 命令解析 (/new, /stop, /help, /task, /plan) | IMPLEMENTED |
| D1-S4 | Auth | 认证与授权 | PLANNED |
| D1-S5 | Milestone | 里程碑 DAG 跟踪、TaskFlow 编排 | IMPLEMENTED |
| D1-S6 | RateLimit | 令牌桶限流控制 + HTTP 中间件 | IMPLEMENTED |
| D1-S7 | Metrics | 通信层指标（已弃用，迁移至 D5 observability.Bridge） | IMPLEMENTED |
| D1-S8 | Renderers | CLI/钉钉 消息渲染器 + 通用 Component 组件 | IMPLEMENTED |
| D1-S9 | EventBus | 背压感知事件总线（Drain → Compact → Reconnect 生命周期） | IMPLEMENTED |
| D1-S10 | Connection | WebSocket/Webhook 连接管理与指数退避重连 | IMPLEMENTED |
| D1-S11 | Core | Card 模型与链式 Builder（平台无关卡片抽象） | IMPLEMENTED |
| D1-S12 | Instance | IM 实例注册中心（Register/Unregister/HealthCheck） | IMPLEMENTED |

## Architecture

```
User/IM ──→ Adapters (D1-S2) ──→ Gateway (D1-S1) ──→ Context Engine (D2)
  │            │  │                    │                    │
  │  Feishu    │  ├─ CardKit 流式      ├─ AgentRoute ──→ D4 MultiAgent
  │  DingTalk  │  ├─ Worker 卡片       ├─ EventDispatcher
  │  CLI       │  └─ Session Resolve   ├─ PermissionManager
  │            │                       ├─ SessionStore (File)
  └─ Renderers (D1-S8) ←── EventBus (D1-S9) ←─┘
       │                         │
       ├─ CLIRenderer            ├─ Drain/Compact/Reconnect
       ├─ DingTalkCardRenderer   └─ Priority (Critical/Normal/Low)
       └─ Components
```

## Cross-Domain Dependencies

| Domain | 依赖内容 | 使用位置 |
|--------|---------|---------|
| D2 Context Engine | `contracts.IEngine`, tasks (CLICommands, PlanMode) | gateway, adapters/cli |
| D4 MultiAgent | `IAgentFactory`, `Agent`, `AgentObserver` | gateway/agent_route |
| D5 Observability | `Observability`, `Bridge`, tracer, telemetry | gateway, adapters |
| D7 Orchestration | `wave.WorkerType`, `wave.WorkerEvent` | adapters/feishu_worker_card |
| Shared | config, contracts, errors, types | 全子包 |

## Package Map

| 子包 | 场景 | 职责 |
|------|------|------|
| `gateway/` | D1-S1 | CommunicationGateway, SessionStore, PermissionManager, EventDispatcher, GatewayAPI |
| `adapters/` | D1-S2, D1-S3 | FeishuAdapter, DingTalkAdapter, CLIAdapter, CardkitClient, WorkerCardRenderer |
| `core/` | D1-S11 | Card, CardBuilder, CardElement 接口及所有元素类型 |
| `eventbus/` | D1-S9 | BackpressureEventBus (Bus), Event, Priority, State 状态机 |
| `milestone/` | D1-S5 | MilestoneService, TaskFlowService, DAG 操作与事件发射 |
| `ratelimit/` | D1-S6 | RateLimiter (令牌桶), HTTP Middleware |
| `metrics/` | D1-S7 | [DEPRECATED] Counter/Gauge/Histogram — 使用 D5 observability.Bridge |
| `renderers/` | D1-S8 | CLIRenderer, DingTalkCardRenderer, Component 接口及组件 |
| `connection/` | D1-S10 | ConnectionManager, Connection, 心跳与指数退避重连 |
| `instance/` | D1-S12 | InstanceRegistry, IInstanceRegistry, InstanceInfo |

## Key Design Patterns

1. **Gateway-Adapter**: `CommunicationGateway` 在适配器与 Context Engine 之间路由消息。适配器实现 `EventHandler` 接口并使用 `GatewayAPI`。
2. **EventBus 状态机**: `eventbus.Bus` 实现 Drain → Compact → Reconnect 生命周期，5 种状态 (Running/Draining/Compacting/Reconnecting/Closed)。Critical 事件通过 `PublishCritical` 绕过常规通道直接扇出（P0 送达保证）。
3. **PermissionManager + YOLO 模式**: 权限请求根据风险等级自动审批，CRITICAL 风险永不自动审批。
4. **Card 系统**: 平台无关的 `core.Card` 模型 + `CardBuilder` 链式 API，渲染为平台特定的 JSON/Markdown。
5. **Session 持久化**: `FileSessionStore` 使用原子写入（临时文件 + rename），支持 `ResolveSessionByChatID` 实现重启后恢复。
6. **CardKit 流式**: 飞书适配器支持元素级流式输出（打字机效果），带节流配置，CardKit 不可用时降级为 `Im.Message.Patch`。

## Registries

- **A 层**: `a-registry.md` — 21 Activities
- **F 层**: `f-registry.md` — 43 Function Points
- **T 层**: `t-registry.md` — 44 Test Points (44 IMPLEMENTED)
