# 通信层详细设计（D1 Communication）

> **文档类型：** 详细架构设计（遵循 `docs/methodology/detail-design-framework.md`）
> **Change ID：** devrix-foundation → devrix-event-channel
> **Demand ID：** DM-20260607-001, DM-20260611-003
> **版本：** 2.0.0
> **状态：** Active — 主路径 CommunicationGateway；新引入 BackpressureEventBus（DM-20260611-003）
> **DSAFT 域归属：** D1 Communication Domain（Core）
> **关联 OpenSpec：** 规格 SoT：`openspec/specs/d1-communication/spec.md` · Delta：`layer-delta.md` · A/F/T 注册表：`a-registry.md` / `f-registry.md` / `t-registry.md`

---

## 文档索引

| 文档 | 用途 |
|------|------|
| 本文档 | 按六段式框架展开的**可读架构设计**（评审 / onboarding） |
| `openspec/specs/d1-communication/spec.md` | 验收规格（Gherkin Scenario → T 层，canonical） |
| `openspec/specs/d1-communication/layer-delta.md` | 层能力 Delta SoT（V1 → V2 演进记录） |
| `openspec/specs/d1-communication/a-registry.md` | A 层活动注册表（21 Activities） |
| `openspec/specs/d1-communication/f-registry.md` | F 层功能点注册表（43 Function Points） |
| `openspec/specs/d1-communication/t-registry.md` | T 层测试点注册表（44 / 44 IMPLEMENTED） |
| `openspec/specs/d1-communication/span-registry.md` | Span 注册表（15 ops，gateway + adapter） |
| `openspec/specs/architecture/layering.md` | Devrix 分层架构与 D/S/A/F/T 编号规则 |

---

## ① 架构目标

### 1.1 业务目标

| 痛点 | 目标能力 | 用户可感知结果 |
|------|----------|----------------|
| 多入口（飞书 / 钉钉 / CLI）消息格式与生命周期各异 | 统一消息网关 + 适配器矩阵 | 任一入口接入即可获得一致会话、权限、观测体验 |
| 长会话中 IM 端消息流易受背压、断连影响 | BackpressureEventBus（Drain → Compact → Reconnect） | 弱网/重连场景不丢关键事件（complete / error 必达） |
| IM 端流式输出延迟与「打字机」体验缺失 | CardKit 元素级流式 + 限流 + 降级 Patch | 长答案像人类打字机一样逐步呈现 |
| 工具调用风险无差别提示 | PermissionManager + YOLO 自动审批 | 低风险工具无感；高风险工具强制人工确认 |
| 多 Agent 协作缺乏统一入口 | CommunicationGateway 接入 MultiAgent AgentFactory | 通过 IM 触发多 Agent 任务编排 |
| 卡片渲染与平台耦合 | `core.Card` 平台无关模型 + 多 Renderer | 一份卡片定义同时渲染 CLI / 飞书 / 钉钉 |

### 1.2 技术目标（量化）

| 指标 | 目标 | 测量方式 |
|------|------|----------|
| **IM 入口启动延迟** | P99 < 500ms（飞书 WebSocket 握手成功） | span `comm.adapter.feishu.start` |
| **消息端到端延迟**（入站 → 首 `thinking`） | P99 < 800ms | Gateway span `comm.gateway.route` |
| **CardKit 流式节流** | 默认 ≥ 80ms / chunk（可配） | `streamThrottle.MinInterval` |
| **背压 Drain 收敛** | 低水位阈值 `LowWatermark` 内 P99 < 1s | EventBus DrainReport.Duration |
| **Critical 事件送达** | 100%（永不丢、必经独立通道） | 测试 D1-S9-A01-T02 |
| **单实例会话容量** | ≥ 1000 Session / 进程 | 与 `session.max_sessions=1000` 对齐 |
| **权限响应超时** | 默认 60s，可配 | PermissionConfig.DefaultTimeout |
| **飞书重连退避上限** | ≤ 60s 指数退避 | ConnectionManager.RestoreConnection |
| **IM 实例健康判定** | `LastSeen` > 30s 视为 unhealthy | InstanceRegistry timeout=30s |

### 1.3 约束条件

| 类型 | 约束 | 设计响应 |
|------|------|----------|
| **架构** | D1 不得反向依赖 D2 之外的领域逻辑 | 仅通过 `contracts.IEngine` 调用 D2 |
| **架构** | D1 Adapter 仅通过 `GatewayAPI` 暴露反向调用 | 见 `gateway/api.go: GatewayAPI` |
| **可观测** | 必须接入 D5 `observability.Bridge`（D1 旧 metrics 包已弃用） | `internal/layers/communication/metrics/` 标注 DEPRECATED |
| **存储** | V2 默认文件存储（`FileSessionStore`），原子写 | 见 §4 领域模型 |
| **跨域** | Worker 卡片数据来自 D7 `wave.WorkerEvent` | 见 §5 核心链路 |
| **配置** | `devrix.yaml` → `connection.*` / `rate_limit.*` / `feishu.*` | 通过 `config.CommunicationConfig` 注入 |
| **测试** | T 层 44 个测试点全部 IMPLEMENTED | `openspec/specs/d1-communication/t-registry.md` |

---

## ② 架构原则

### 2.1 设计原则

1. **Gateway-Adapter 解耦**：所有适配器（Feishu / DingTalk / CLI）实现 `EventHandler`，并通过 `GatewayAPI` 调用 Gateway。Gateway 永不直接调用适配器发送 API（除非借助 `EventDispatcher` 异步回写）。
2. **EventBus 状态机唯一权威**：`Running → Draining → Compacting → Reconnecting → Closed` 是生命周期唯一路径。Critical 事件走独立无缓冲通道（`PublishCritical`），保证 P0 送达。
3. **Permission YOLO 模式**：用户配置 `yolo_mode=true` 时，按风险等级自动审批；CRITICAL 风险永不自动审批（DM-20260607-001）。
4. **Card 模型平台无关**：`core.Card` 仅表达意图（Markdown / Divider / Actions / ListItem / Select / Note / Header），由 Renderer 落地到飞书 JSON / 钉钉 JSON / CLI ANSI。
5. **会话原子写**：`FileSessionStore.atomicWrite` 先写临时文件再 `os.Rename`，避免半写状态。
6. **CardKit 降级**：当 CardKit API 失败或不可用时，自动降级到 `Im.Message.Patch`（DM-20260611-003）。
7. **背压可观测**：每次 Drain / Compact / Reconnect 返回结构化 Report，注入 metrics。

### 2.2 命名规范

| 类型 | 规范 | 示例 |
|------|------|------|
| 包 | 一级目录 = 场景 ID 名 | `gateway` / `adapters` / `eventbus` |
| 接口 | `I` 前缀 + 能力描述 | `IInstanceRegistry` / `IMilestoneService` |
| 适配器 | `<Platform>Adapter` | `FeishuAdapter` / `DingTalkAdapter` / `CLIAdapter` |
| 渲染器 | `<Format>Renderer` | `CLIRenderer` / `DingTalkCardRenderer` |
| Activity 编号 | `D1-S{N}-A{NN}` | `D1-S1-A02 RouteMessage` |
| Function 编号 | `D1-S{N}-A{NN}-F{NN}` | `D1-S2-A03-F01 EmitWorkerEvent` |

### 2.3 代码风格

- 工具类不过业务逻辑（Renderer / SessionStore / ConnectionManager 各自只做自己职责）。
- 异常不过模块边界——`GatewayAPI` 返回 `error` 由调用方处理，不在 Gateway 内部 swallow。
- 统一日志 `slog`，关键事件：`message.routed` / `permission.resolved` / `eventbus.drained` / `connection.lost`。
- 所有时间字段为 `time.Time`（UTC 序列化），文件 Session 通过 `MarshalIndent` 持久化。

## ③ 业务流程

### 3.1 核心用例：用户通过飞书发起 Agent 任务

```
┌────┐  ┌─────────────┐  ┌──────────┐  ┌────────┐  ┌────────────┐  ┌────────┐  ┌──────────┐
│User│  │FeishuAdapter│  │ Gateway  │  │PermMgr │  │AgentFactory│  │D2 Engine│  │ EventBus │
└─┬──┘  └──────┬──────┘  └────┬─────┘  └───┬────┘  └─────┬──────┘  └───┬────┘  └─────┬────┘
  │  IM msg    │              │            │             │            │             │
  │───────────▶│ ParseInbound │            │             │            │             │
  │            │ (D1-S2-A01)  │            │             │            │             │
  │            │ RouteInbound │            │             │            │             │
  │            │─────────────▶│ ResolvePerm│             │            │             │
  │            │              │───────────▶│             │            │             │
  │            │              │ if YOLO+LOW auto-approve│            │             │
  │            │              │───────────▶│             │            │             │
  │            │              │ routeInboundViaAgent     │            │             │
  │            │              │──────────────────────────▶ Agent.Run  │             │
  │            │              │            │             │───────────▶│             │
  │            │              │            │             │ EngineEvent{thinking}    │
  │            │              │            │             │─────────────────────────▶│
  │            │ OnStatus     │            │             │            │ Publish      │
  │◀───────────│ (CardKit 流) │            │             │            │             │
  │            │              │            │             │ EngineEvent{tool_call}  │
  │            │              │            │             │─────────────────────────▶│
  │            │              │            │             │            │ (Critical    │
  │            │              │            │             │            │  bypasses)   │
  │            │              │            │             │ EngineEvent{complete}   │
  │            │              │            │             │─────────────────────────▶│
  │            │ OnMessage    │            │             │            │ Fanout→Crit  │
  │◀───────────│ (Patch 终态) │            │             │            │             │
```

**关键步骤 RT 标注**：

| 步骤 | P99 上限 |
|------|---------|
| ParseInbound → RouteInbound | < 5ms |
| ResolvePermission（YOLO 自动） | < 1ms |
| routeInboundViaAgent | < 5ms |
| Engine 首个 `thinking` 事件 | < 800ms（与 LLM 网络时延叠加） |
| EventBus.Publish 扇出 | < 1ms |

### 3.2 异常补偿

| 异常场景 | 补偿机制 | 配置项 |
|----------|---------|--------|
| 飞书 WebSocket 断开 | ConnectionManager 指数退避重连（上限 60s） | `connection.heartbeat_interval` / `connection.heartbeat_timeout` |
| Webhook 接收 401 / 403 | Adapter 校验失败直接拒绝 | `feishu.encrypt_key` 校验 |
| LLM 调用失败 | EventBus 不直接感知；D2 Engine 负责 | 由 D2/D3 链路处理 |
| Permission 请求超时 | PermissionManager 触发 `recordTimeout` + Denied | `permission.default_timeout` |
| 飞书实例心跳超时 | InstanceRegistry 标记 unhealthy | `instance.health_timeout` |
| EventBus 背压 | Drain → Compact → Reconnect | `eventbus.high_watermark` / `low_watermark` |
| CardKit API 不可用 | 降级为 `Im.Message.Patch` | 内部 fallback |
| 工具 YOLO 自动拒绝（CRITICAL） | PermissionManager 直接返回 denied | `user.yolo_mode` |

### 3.3 分支处理

- **`/new` 命令**：CLI/飞书 → `adapters.ParseCommand` → `gateway.RouteMessage` → 新 Session。
- **`/stop` 命令**：→ `gateway.StopProcess(sessionID)` → `cancel()` 当前 Process。
- **`/help` 命令**：→ 静态文案 + 当前可用命令列表。
- **`/task` / `/plan`**：→ 进入 D7 TaskFlow / Plan 编排（DM-20260607-001）。

---

## ④ 领域模型

### 4.1 聚合根

| 聚合根 | 子对象 | 生命周期 | 持久化 |
|--------|--------|---------|--------|
| `Session` | `Message[]`、`SessionState` | `Created → Active → Idle → Closed` | `FileSessionStore`（JSON） |
| `Milestone` | `dependencies[]`、`Progress` | `Created → Progress → Completed/Failed` | DAG 内存模型 + EventEmitter |
| `TaskFlow` | `milestones[]`、`OverallProgress` | `Created → Started → Progress → Completed/Failed` | 内存模型 |
| `Instance` | `LastSeen`、`Status` | `Registered → Healthy → Unhealthy → Unregistered` | `InstanceRegistry` 内存 |
| `Connection` | `Heartbeat Timer`、`OnLost/OnRestored` | `Connected → Lost → Restored` | `ConnectionManager` 内存 |
| `Card` | `CardHeader`、`CardElement[]` | 临时对象（构造 → 渲染 → 释放） | 不持久化 |
| `EngineEvent` | `Type`、`Content`、`Metadata` | 由 D2 产出，经 EventBus 扇出 | 不持久化（短期保留于 Snapshot） |

### 4.2 限界上下文

| 上下文 | 关注点 | 跨上下文协议 |
|--------|--------|--------------|
| **Communication Context** | IM 适配器 / Gateway / SessionStore / Permission | `EventHandler` / `GatewayAPI` |
| **Card Context** | `core.Card` 模型 + Renderer | 平台 JSON 输出 |
| **EventBus Context** | Drain / Compact / Reconnect | `Event` / `Report` |
| **Lifecycle Context** | ConnectionManager / InstanceRegistry | `Heartbeat` / `HealthCheck` |
| **Progress Context** | Milestone / TaskFlow DAG | `EventEmitter` 抽象 |

### 4.3 领域事件

| 事件 | 触发方 | 订阅方 | 携带字段 |
|------|--------|--------|---------|
| `session.created/closed/expired` | `gateway/store.go` | FileSessionStore / observers | `session_id` |
| `permission.requested/resolved` | `gateway/permission.go` | Adapter / Agent | `request_id`、`decision` |
| `milestone.{created,progress,completed,failed}` | `milestone/service.go` | Renderers / EventBus | `milestone_id`、`progress` |
| `taskflow.{started,progress,completed,failed}` | `milestone/taskflow.go` | Renderers / EventBus | `taskflow_id`、`progress` |
| `eventbus.{draining,compacting,reconnecting}` | `eventbus/{drain,compact,reconnect}.go` | Metrics / Logger | `session_id`、`report` |
| `connection.{registered,unregistered,restored,lost}` | `connection/manager.go` | Observability | `connection_id` |
| `instance.{registered,unregistered,healthy,unhealthy}` | `instance/registry.go` | Observability | `instance_id` |
| `engine.{thinking,text,tool_call,tool_result,complete,error}` | D2 Engine | Adapter / CardKit Streamer | 见 D2 contracts |

### 4.4 关键类型（D1 内部）

```go
// gateway/api.go
type GatewayAPI interface {
    GetSession(sessionID string) (*types.Session, error)
    ResolveSessionByChatID(chatID string) (*types.Session, error)
    CreateSession(chatID, workDir string) (*types.Session, error)
    RouteInbound(ctx context.Context, msg *types.InboundMessage) error
    RouteOutbound(msg *types.OutboundMessage) error
    StopProcess(sessionID string) error
}

// gateway/gateway.go
type EventHandler interface {
    OnMessage(msg *types.OutboundMessage)
    OnPermissionRequest(req *types.PermissionRequest) bool
    OnError(err error, sessionID string)
    OnStatus(sessionID string, state types.SessionState)
}

// eventbus/types.go
type Priority int      // Critical(0) | Normal(1) | Low(2)
type State int         // Running | Draining | Compacting | Reconnecting | Closed
type Event struct {
    *contracts.EngineEvent
    Priority    Priority
    Sequence    uint64
    PublishedAt time.Time
}

// connection/manager.go
type Connection struct {
    ID, AdapterID, Type, Status string
    LastSeen  time.Time
    Heartbeat *time.Timer
    OnLost(*Connection); OnRestored(*Connection)
}

// instance/registry.go
type InstanceInfo struct {
    ID, Name, Address, Status string
    Port int; StartedAt, LastSeen time.Time
}

// core/card.go (sealed interface)
type CardElement interface{ cardElement() }
// CardMarkdown | CardDivider | CardActions | CardListItem | CardSelect | CardNote
```

## ⑤ 核心链路图

### 5.1 端到端路径（IM → Engine → IM）

```
User
  │  WebSocket 消息
  ▼
[FeishuAdapter.OnMessage]
  │  ParseInbound (D1-S2-A01)
  ▼
[FeishuAdapter.handleMessage]
  │  resolveOrCreateSession (D1-S2-A05)
  ▼
[CommunicationGateway.RouteInbound]
  │  D1-S1-A02 RouteMessage
  ▼
[CommunicationGateway.routeInboundViaAgent | routeInboundDirect]
  │  ├─ ResolvePermission (D1-S1-A03)
  │  └─ IAgentFactory.Create (D1-S1-A04)
  ▼
[D2 contracts.IEngine.Process(ctx, sessionID, msg)]
  │  EngineEvent{thinking|text|tool_call|tool_result|complete|error}
  ▼
[BackpressureEventBus]  ──DM-20260611-003
  │  Critical: 独立无缓冲通道（PublishCritical）
  │  Normal/Low: buffered channel，monitor 检测 backlog
  ▼
[EventDispatcher] → [Adapter.OnStatus / OnMessage]
  │  FeishuAdapter → CardKit StreamElementContent (D1-S2-A04-F02)
  │  或降级 Im.Message.Patch
  ▼
User IM 端
```

**SLA 承诺标注**：

| 节点 | SLA | 单点风险 |
|------|-----|----------|
| FeishuAdapter.OnMessage | P99 < 50ms | 否（多适配器并行） |
| CommunicationGateway.RouteInbound | P99 < 100ms | 是（单进程 Gateway） |
| EventBus.Publish / PublishCritical | P99 < 1ms | 否（in-process channel） |
| Feishu CardKit StreamElementContent | 受飞书 API 限流 | 否（飞书服务端） |
| FileSessionStore atomicWrite | 受磁盘 IO 影响 | 否（本地 FS） |

### 5.2 时序图：EventBus Drain → Compact → Reconnect

```
                  ┌──────────────────────────────────────────────────┐
                  │                EventBus.Bus                       │
                  │                                                  │
   Publish(N) ──▶│  normalCh(buf=N)  ──▶  monitor goroutine  ──▶  subscribers
                  │      │ backlog++                              ▲
                  │      │ if backlog > HighWatermark              │ fanout
                  │      ▼                                         │
                  │  setState(StateDraining)                       │
                  │      │                                         │
   Drain(ctx) ───▶│  monitor 暂停消费 normalCh                     │
                  │  Pull events until backlog ≤ LowWatermark      │
                  │      │ discard Normal/Low, keep Critical       │
                  │      ▼                                         │
                  │  setState(StateCompacting)                     │
                  │      │ group by Type → emit aggregate          │
                  │      ▼                                         │
                  │  setState(StateReconnecting)                   │
                  │      │ rebuild normalCh + pendingCh            │
                  │      ▼                                         │
                  │  setState(StateRunning)                        │
                  └──────────────────────────────────────────────────┘
```

- **不变契约（P0）**：Critical 事件永不进入 normalCh，永不被 Drain / Compact 影响；`PublishCritical` 是同步扇出，调用返回即所有活跃订阅者已收。
- **背压触发**：`TriggerDrain()` 在每次 Publish 时由 monitor 检测 backlog 超过 HighWatermark 自动触发。
- **重连超时**：超过 `cfg.ReconnectTimeout` 仍处于 StateReconnecting → 返回 `ErrReconnectTimeout`。

### 5.3 单点风险与切换方案

| 风险点 | 现行方案 | 演进方向 |
|--------|----------|----------|
| CommunicationGateway 单进程 | 状态由 `sync.Map` + 文件 Session 持久化 | 多实例时引入外部 SessionStore（Redis / DB） |
| EventBus in-process | 进程内 channel | 多实例时升级为外部 broker（Redis Streams / NATS） |
| FileSessionStore | 本地 JSON 文件 | V3 可换 SQLite（已在 D2 长程记忆验证） |
| ConnectionManager 单进程 | in-memory `connections` map | 多实例时拆分为独立服务 |

---

## ⑥ 接口 / API 设计

### 6.1 风格

- **入站（IM → Devrix）**：WebSocket 长连接（飞书/钉钉）+ Webhook（HTTP POST）；统一抽象为 `EventHandler.OnMessage`。
- **出站（Devrix → IM）**：HTTP API（飞书 OpenAPI、钉钉 OpenAPI）；CLI 走 stdout / stdin。
- **跨进程**：D1 不直接暴露 RPC；只与 D2/D4 在进程内通过接口协作。

### 6.2 关键契约

#### 6.2.1 GatewayAPI（D1 ↔ Adapter / D4）

| 方法 | 入参 | 出参 | 错误语义 |
|------|------|------|----------|
| `CreateSession(chatID, workDir)` | chat key, 工作目录 | `*Session`, error | chatID 冲突 → 返回已存在 Session |
| `ResolveSessionByChatID(chatID)` | chat key | `*Session`（最近非 Idle） | 不存在 → `ErrSessionNotFound` |
| `RouteInbound(ctx, msg)` | 入站消息 | error | ctx 取消 → ctx.Err |
| `RouteOutbound(msg)` | 出站消息 | error | 内部错误由 EventHandler.OnError 处理 |
| `StopProcess(sessionID)` | 会话 ID | error | 不存在 → nil |

幂等：`CreateSession` 同 chatID 重复调用返回同一 Session；`StopProcess` 重复调用幂等。

#### 6.2.2 EventBus 公开 API

```go
type Bus struct{ /* ... */ }

func (b *Bus) Publish(ctx context.Context, e Event) error         // Normal/Low
func (b *Bus) PublishCritical(ctx context.Context, e Event) error // 同步扇出
func (b *Bus) Subscribe(topic string) (<-chan Event, func())    // 返回取消函数
func (b *Bus) Drain(ctx context.Context, sessionID string, timeout time.Duration) (DrainReport, error)
func (b *Bus) Compact(ctx context.Context, sessionID string) (CompactReport, error)
func (b *Bus) Reconnect(ctx context.Context, sessionID string) (ReconnectReport, error)
func (b *Bus) TriggerDrain()                                     // 立即进入 Draining
func (b *Bus) Backlog() int                                      // 当前 backlog
func (b *Bus) State() State                                      // 当前状态
func (b *Bus) Close() error
```

错误返回（`eventbus/bus.go`）：

| 错误 | 触发 | 行为 |
|------|------|------|
| `ErrBusClosed` | Close 后 Publish / Drain | 调用者应停止生产 |
| `ErrDrainTimeout` | Drain 超过 timeout 仍未到低水位 | 调用者重试或转 Reconnect |
| `ErrReconnectTimeout` | Reconnect 超过 timeout 仍未回到 Running | 上层告警 / 重建实例 |
| `ErrContextCancelled` | Publish 时 ctx 提前取消 | 调用者重试 |

#### 6.2.3 Permission 协议

| 字段 | 必填 | 说明 |
|------|------|------|
| `RequestID` | 是 | UUIDv4，由 Adapter 注入 |
| `ToolName` | 是 | 工具名 |
| `RiskLevel` | 是 | `LOW`/`MEDIUM`/`HIGH`/`CRITICAL` |
| `Timeout` | 否 | 默认 60s（`permission.default_timeout`） |
| `YOLOAuto` | 是（决策时） | `true` 表示 YOLO 自动审批 |

YOLO 规则（DM-20260607-001）：

| 风险等级 | YOLO=true | YOLO=false |
|----------|-----------|------------|
| `LOW` | 自动批准 | 弹卡片等待用户 |
| `MEDIUM` | 自动批准 | 弹卡片等待用户 |
| `HIGH` | 自动批准 | 弹卡片等待用户 |
| `CRITICAL` | **永不自动** | 弹卡片等待用户 |

### 6.3 错误码

| 错误码 | 含义 | 来源 |
|--------|------|------|
| `COMM_SESSION_NOT_FOUND_2001` | 会话不存在 | gateway |
| `COMM_SESSION_EXISTS_2002` | CreateSession 冲突 | gateway |
| `COMM_PERMISSION_DENIED_2003` | 权限被拒绝（人工 / CRITICAL YOLO） | gateway/permission.go |
| `COMM_PERMISSION_TIMEOUT_2004` | 权限请求超时 | gateway/permission.go |
| `COMM_RATE_LIMITED_2005` | 触发限流 | ratelimit |
| `COMM_EVENTBUS_CLOSED_2006` | EventBus 已关闭 | eventbus |
| `COMM_EVENTBUS_DRAIN_TIMEOUT_2007` | Drain 超时 | eventbus |
| `COMM_CONNECTION_LOST_2008` | IM 连接丢失 | connection |
| `COMM_INSTANCE_UNHEALTHY_2009` | IM 实例健康失败 | instance |
| `COMM_ADAPTER_NOT_REGISTERED_2010` | 未注册适配器被调用 | adapters |

### 6.4 幂等性

| 操作 | 幂等保证 | 实现 |
|------|----------|------|
| `Session.Create` | 同 chatID 重复 → 同一 Session | `ResolveSessionByChatID` 优先 |
| `Session.Update` | 覆盖写 | 原子 rename |
| `Milestone.Complete` | 同 ID 重复 → no-op | 状态机校验 |
| `Permission.Resolve` | 同 RequestID 重复 → no-op | signals map |
| `EventBus.Publish` | Sequence 单调递增 | atomic.Uint64 |
| `Connection.Register` | 同 ID 重复 → 覆盖 | `connections[id] = conn` |

### 6.5 版本

- D1 内部包路径无版本号（Go 模块内稳定）。
- 对外契约变更通过 OpenSpec Change ID 跟踪：`devrix-foundation` (V1) → `devrix-event-channel` (V2)。
- 每次 S6 归档：`openspec/archive/<YYYY-MM-DD>-<change-id>/`。

### 6.6 限流

- **IM 入口限流**：`RateLimiter`（令牌桶）按 `adapter_id` 维度限流，默认 100 req/min，burst 10。
- **HTTP Webhook 限流**：`ratelimit.HTTPMiddleware` 返回 `429 + Retry-After`。
- **EventBus 背压**：见 §5.2。

## 附录 A：A/F/T 注册表

完整的活动、功能点、测试点清单见独立注册表文件，本文档不重复列出：

| 注册表 | 路径 | 条目数 |
|--------|------|--------|
| A 层活动 | `openspec/specs/d1-communication/a-registry.md` | 21 Activities（12 Scenarios） |
| F 层功能点 | `openspec/specs/d1-communication/f-registry.md` | 43 F Points |
| T 层测试点 | `openspec/specs/d1-communication/t-registry.md` | 44 / 44 IMPLEMENTED |

关键 Bridge 功能点：
- `D1-S5-A01-F06 AdaptToPlanner`：Milestone → Planner 接口（`bridges/milestone/wire.go`）
- `D1-S1-A03-F03 AdaptToPermissionGate`：Permission → IPG（`gateway/permission_adapter.go`）

---

## 附录 B：相关文档

| 文档 | 关系 |
|------|------|
| `openspec/specs/d2-context-engine/design.md` | D2 QueryLoop 引擎设计（D1 调用方） |
| `openspec/specs/d3-llm-gateway/design.md` | D3 LLM 网关设计（D1 不直接依赖） |
| `openspec/specs/d4-multi-agent/design.md` | D4 MultiAgent 设计（D1 通过 AgentFactory 协作） |
| `openspec/specs/d5-observability/spec.md` | D5 Observability 桥接规范（D1 强制接入） |
| `openspec/specs/d7-orchestration/spec.md` | D7 任务/计划编排（Worker 卡片数据来源） |
| `openspec/specs/project/master.md` | Devrix 项目研发规范（路由入口） |
| `openspec/specs/project/architecture-design.md` | 架构设计规范（六段式来源） |
| `docs/methodology/dsaft-methodology.md` | DSAFT 方法论（ID 格式权威） |
| `docs/methodology/detail-design-framework.md` | 六段式详细设计框架 |
| `docs/config.md` | `devrix.yaml` 配置契约 |

---

## 附录 C：版本历史

| 版本 | 日期 | 主要变更 | Change ID |
|------|------|----------|-----------|
| 1.0.0 | 2026-06-07 | 初版：CLI + 飞书基础适配器 + Gateway + Session | devrix-foundation |
| 1.1.0 | 2026-06-09 | 钉钉适配器、Card 模型、Renderer 矩阵 | devrix-card-system |
| 1.2.0 | 2026-06-10 | Milestone / TaskFlow、PermissionManager + YOLO | devrix-milestone-permission |
| 2.0.0 | 2026-06-11 | BackpressureEventBus（Drain→Compact→Reconnect） | devrix-event-channel |

> 当前文档为 **V2.0.0**，与代码现状对齐（截至 2026-06-13）。
