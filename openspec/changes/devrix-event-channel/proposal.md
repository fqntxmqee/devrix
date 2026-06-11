# Proposal: 事件通道与背压机制 — Channel Drain/Compact/Reconnect

**Change ID:** devrix-event-channel
**Demand ID:** DM-20260611-003
**Status:** S2_Proposal
**Priority:** P0

## 1. Background

ContextEngine.Process() 返回固定 buffer 32 的 `chan *EngineEvent`。Go 的 chan 是 push-based：消费者慢、生产者快时必然积压或阻塞。Claude Code 用 async generator 天然零积压（yield 挂起直到消费者拉取）。Devrix 因 Go 无原生等价物，需通过主动背压检测 + Drain/Compact 生命周期弥补。

## 2. Problem Statement

| 问题 | 位置 | 严重度 |
|------|------|--------|
| 固定 buffer=32 无背压感知 | `contextengine/engine.go` | P0 |
| 写阻塞无降级 | 全部 `ch <- event` 位置 | P0 |
| 无事件丢弃/合并策略 | 全通道 | P0 |
| Fork 多个子 Agent 同时写父通道瞬时填满 | `multiagent/forkjoin` | P0 |
| 无拥塞检测 | 全栈 | P1 |
| 无 Drain/Compact/Reconnect 生命周期 | 全栈 | P0 |

## 3. Proposed Solution

### 3.1 事件优先级

- `EngineEvent` 加 `Priority int` 字段（Critical=0, Normal=1, Low=2）
- 终结事件（`complete`/`error`）强制 Critical（P0 约束：不可丢弃）
- 进度事件（`progress`/`thinking`）可设 Low

### 3.2 BackpressureEventBus

新建 `internal/layers/contextengine/eventbus/`：

| 组件 | 职责 |
|------|------|
| `backpressure.go` | 监控通道积压量，超阈值触发 Drain 回调 |
| `drain.go` | Drain 策略：按优先级淘汰，保留 Critical 队列 |
| `compact.go` | 同类连续事件合并（ProgressEvent 聚合） |
| `reconnect.go` | Drain→Compact→重建 channel 生命周期 |

### 3.3 Fork 通道隔离

- Fork 子 Agent 拿到 `*BackpressureEventBus` 独立实例（默认 buffer=16 + 父 bus reference）
- Join 时聚合子 bus 事件回父 bus（按子完成序）
- SubAgent 数量 > 8 时强制 fresh context policy（与 DM-007 协调）

### 3.4 Reconnect 生命周期

```
Normal → BackpressureRising → DrainTriggered(排空 Low)
                                  ↓
                              Compact(合并 Low)
                                  ↓
                              ChannelRebuilt(新 channel)
                                  ↓
                              Normal
```

状态变化通过 D5 Span 记录：`eventbus.state` 标签。

### 3.5 监控指标

| 指标 | 类型 | 标签 |
|------|------|------|
| `eventbus.backlog` | Gauge | session_id, channel |
| `eventbus.drained_total` | Counter | priority |
| `eventbus.compacted_total` | Counter | event_type |
| `eventbus.reconnect_total` | Counter | — |
| `eventbus.dropped_total` | Counter | priority, event_type |

## 4. Success Metrics

| 指标 | 基线 | 目标 |
|------|------|------|
| 通道积压 P99 峰值 | 32 (buffer 上限) | < 8 |
| 阻塞 LLM/工具调用次数/会话 | 1+ | 0 |
| Drain 触发后 1s 内积压清零率 | N/A | ≥ 95% |
| Critical 事件丢弃数 | N/A | 0（强约束） |
| Fork 5 子 Agent 同时段无丢失 | 不保证 | 保证 |

## 5. Implementation Plan

| Phase | 内容 | 估时 |
|-------|------|------|
| P1 | EngineEvent 加 Priority + BackpressureEventBus 骨架 | 1d |
| P2 | Drain + Compact + Reconnect 完整生命周期 | 1.5d |
| P3 | Fork 通道隔离 + Join 聚合 | 1d |
| P4 | 监控指标 + D5 span | 0.5d |
| P5 | BackpressureProbe 注册 D6 | 0.5d |
| **Total** | | **4.5d** |

**合并策略**：2 个 PR（Bus 核心 + 隔离/指标），独立可回滚。

## 6. Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| Drain 误丢关键事件 | 强约束 Critical 不可丢；事件类型白名单；调试模式下关闭 Drain |
| Compact 损失粒度 | 调试 flag `eventbus.compact.enabled=false` 可关闭 |
| Reconnect 状态不一致 | `-race` + 并发回归测试；状态机转换用 if/else 严格校验 |
| Fork 通道资源耗尽 | 限流：单 session max 8 Fork；超过走 shared bus + 强优先级 |

## 7. Out of Scope

- 拉模式（async generator）— Go 无原生，需 runtime 级协程调度改造成本过高
- 跨 session 通道复用（cross-session eventbus）— 见 DM-007 Wave Scheduler
- 完整事件溯源（event sourcing）— 当前仅快照 + 流式，无完整 replay
- 不可丢弃事件约束例外（永远不丢 `complete`/`error`）— 无
