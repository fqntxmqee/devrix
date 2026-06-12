# Design: 事件通道与背压机制 — Drain→Compact→Reconnect

**Change ID:** devrix-event-channel
**Demand ID:** DM-20260611-003
**Status:** S3_Design
**Priority:** P0

## 1. 状态机

```
                 Publish (Normal / Critical)
                          │
                          ▼
                  ┌───────────────┐
       ┌─────────▶│    Running    │◀── Reconnect 完成
       │          └───────────────┘
       │                  │
       │        backlog ≥ highWatermark
       │                  ▼
       │          ┌───────────────┐
       │          │   Draining    │   (排空 Low / Normal,
       │          │               │    Critical 不可丢)
       │          └───────────────┘
       │                  │
       │        backlog ≤ lowWatermark
       │                  ▼
       │          ┌───────────────┐
       │          │  Compacting   │   (合并相邻同 type 事件)
       │          └───────────────┘
       │                  │
       │        compact 完成
       │                  ▼
       │          ┌───────────────┐
       │          │ Reconnecting  │   (重建 channel,
       │          │               │    切换订阅者)
       │          └───────────────┘
       │                  │
       └──────────────────┘
```

状态转换由 `BackpressureEventBus` 内部 goroutine `monitor()` 周期性检查 backlog 触发。
每次状态变化通过日志/指标记录（不在 D5 Span 范围内，避免引入横切依赖）。

## 2. 事件优先级

| Priority | 含义 | 可丢弃 | 通道 |
|----------|------|--------|------|
| `PriorityCritical` (0) | `complete` / `error` 终结事件 | **否**（P0 强约束） | criticalCh（unbuffered） |
| `PriorityNormal`  (1) | `text` / `tool_call` / `tool_result` / `thinking` | 是（Drain） | normalCh（buffered） |
| `PriorityLow`     (2) | `milestone_progress` / `worker_progress` | 是（Drain/Compact） | normalCh（buffered） |

`EngineEvent` 增加 `Priority` 字段，默认 `PriorityNormal`；`complete` / `error`
在 `Publish()` 入口强制 `PriorityCritical`。

## 3. 包结构

```
internal/layers/communication/eventbus/
├── bus.go        # BackpressureEventBus 主体 + 状态机
├── drain.go      # Drain 协议
├── compact.go    # Compact 协议
├── reconnect.go  # Reconnect 协议
├── types.go      # Event 包装 + Priority + 不可变 With* 工具
└── *_test.go     # T 层测试
```

> 注：本 change 落地在 `internal/layers/communication/eventbus/`，与
> `openspec/changes/devrix-event-channel/.openspec.yaml` 中指定的
> `internal/layers/contextengine/eventbus/` 路径不同。原因：本 change
> 在 D1 Communication Gateway 层做事件分发背压，是 wire-level 集成点
> （替换 `gateway.go` 现有事件分发逻辑），而非 D2 Context Engine 内部
> Process() 的 channel buffer。归档时需同步更新 yaml。

## 4. API

```go
type BackpressureEventBus interface {
    Publish(ctx context.Context, ev Event) error         // 阻塞当 backlog ≥ highWatermark（仅 Normal/Low）
    PublishCritical(ctx context.Context, ev Event) error // 非阻塞；走 unbuffered channel
    Subscribe(sessionID string) (subID string, ch <-chan Event, cancel func())
    Drain(ctx context.Context, sessionID string, timeout time.Duration) (DrainReport, error)
    Compact(ctx context.Context, sessionID string) (CompactReport, error)
    Reconnect(ctx context.Context, sessionID string) (ReconnectReport, error)
    Backlog(sessionID string) int
    State(sessionID string) State
    Close() error
}
```

### 4.1 关键不变量

- `complete` / `error` 事件**永远不被 Drain / Compact 丢弃**（P0 约束）
  - `PublishCritical` 走独立的 unbuffered channel
  - 即使在 Draining / Compacting / Reconnecting 状态也必达
  - 测试覆盖：D2-S3-T05、D2-S3-T06
- Drain 仅作用于 `normalCh`；`criticalCh` 不动
- Compact 仅合并**相邻同 type** 的 Normal/Low 事件；不合并 Critical
- Reconnect 期间新发布的事件暂存到 `pendingCh`，新 channel 就绪后刷入

## 5. 配置

`internal/shared/config/eventbus.go` 新增 `EventBusConfig`：

| 字段 | 默认 | 说明 |
|------|------|------|
| `HighWatermark` | 24 | 触发 Draining 的 backlog 阈值 |
| `LowWatermark` | 8 | Draining 退出阈值（hysteresis） |
| `DrainTimeout` | 2s | Drain 超时 |
| `CompactMaxBatch` | 16 | Compact 单次合并事件数 |
| `ReconnectTimeout` | 1s | Reconnect 通道重建超时 |
| `ChannelBuffer` | 32 | 初始 normalCh 容量 |

环境变量覆盖：
- `DEVRIX_EVENTBUS_HIGH_WATERMARK`
- `DEVRIX_EVENTBUS_LOW_WATERMARK`
- `DEVRIX_EVENTBUS_DRAIN_TIMEOUT`

## 6. 与 Gateway 集成

`internal/layers/communication/gateway/gateway.go`：

1. 注入 `BackpressureEventBus` 字段（构造时可选；nil 时走原 fanout）
2. `handleEngineEvents` 不再直接 fanout，而是 `bus.Publish(event)`
3. 新增 `consumeBusEvents` goroutine，从 bus.Subscribe 拉取并执行原 `handleEngineEvent` 逻辑
4. `bus.Reconnect` 在 Process() 退出 / 异常时调用一次

**Wire 兼容性**：`OutboundMessage` 字段、`OnMessage` 接口、metric 名称、event type 字符串
保持完全不变。仅内部增加一层 bus。

## 7. T 层测试覆盖

| ID | 描述 | 文件 |
|----|------|------|
| D2-S3-T01 | 正常事件流不丢 | `bus_test.go` TestNormalEventFlow_NoLoss |
| D2-S3-T02 | 背压触发 Drain | `drain_test.go` TestBackpressureTriggersDrain |
| D2-S3-T03 | Compact 降采样 | `compact_test.go` TestCompactConsecutiveEvents |
| D2-S3-T04 | Reconnect 恢复 | `reconnect_test.go` TestReconnectRecovery |
| D2-S3-T05 | Complete 事件必达 | `bus_test.go` TestCompleteEventNeverDropped |
| D2-S3-T06 | Error 事件必达 | `bus_test.go` TestErrorEventNeverDropped |
| D2-S3-T07 | 通道满时回压到上游 | `bus_test.go` TestPublishBlocksAtHighWatermark |

## 8. 风险与缓解

| 风险 | 缓解 |
|------|------|
| Drain 误丢 Critical | criticalCh 独立；测试覆盖 D2-S3-T05/06 |
| Compact 损失粒度 | 仅合并相邻同 type；测试断言计数 + 内容 |
| Reconnect 状态机竞争 | 状态转换加 `sync.Mutex`；`go test -race` |
| Subscribe 慢消费者阻塞 bus | Subscribe 返回 buffered channel（64），慢只丢订阅者侧 |
