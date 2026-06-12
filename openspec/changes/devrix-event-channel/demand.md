---
demand-id: DM-20260611-003
title: 事件通道与背压机制 — Channel Drain/Compact/Reconnect
source: devrix-harness-architecture-audit
priority: P0
status: S1_Proposal
dsaft_domain: context-engine
created: 2026-06-11
---

# 事件通道与背压机制

## 0. 范围界定（2026-06-11 修订）

| 概念 | 本需求 | **不属于**本需求 |
|------|--------|------------------|
| `Process()` 返回的 `chan *EngineEvent` | ✅ | |
| `SessionQueue.Drain`（Delegate 进度队列） | | ✅ 见 QueryLoop D4 |
| Feishu IM 卡片 sequence | | ✅ 见 DM-006 |
| Wave Scheduler Worker 事件 fan-in | | ✅ 见 DM-007（可复用本通道抽象） |

**不可丢弃事件（P0 约束）：** `complete`、`error` 类终结事件永远不得被 Drain/Compact 合并或丢弃（参见 tech-debt TD-QL-05）。

## 1. 背景

当前 ContextEngine 使用 `chan *EngineEvent` 作为 Process() 的返回值通道，buffer 固定为 32。当生产者（LLM 流式输出 + 工具执行）速度快于消费者（上层调度器）时，通道写阻塞会导致全链路背压失控。Claude Code 使用 async generator（`async *submitMessage()`），其 `for await...of` 是拉模式（pull-based）：消费者不拉取，生产者不推进，天然零积压。Devrix 的 `chan *EngineEvent` 是推模式（push-based），消费者来不及处理时生产者仍被强制写入，必然积压。两种模式的背压特性存在本质差异。

## 2. 问题陈述

### 2.1 固定缓冲区无背压感知

| 问题 | 位置 | 影响 |
|------|------|------|
| `chan *EngineEvent` buffer=32 | `contextengine/engine.go` | 生产者无法感知消费者压力 |
| 写阻塞无降级策略 | 所有 `ch <- event` 位置 | 阻塞扩散至 LLM 调用和工具执行 |
| 无事件丢弃/合并策略 | 全通道 | 高负载下只能阻塞无法主动降级 |

### 2.2 无 Drain/Compact/Reconnect 生命周期

| 阶段 | 当前行为 | 应有行为 |
|------|----------|----------|
| 拥塞检测 | 无 | 监控通道积压量超过阈值 |
| Drain | 无 | 积压超过阈值时主动排空非关键事件 |
| Compact | 无 | 对同类事件合并（如多个 ProgressEvent→一个聚合事件） |
| Reconnect | 无 | 排空后重建通道继续处理 |

### 2.3 Fork/Join 场景下的热循环风险

| 场景 | 风险 |
|------|------|
| Fork 多个子 Agent 同时产生事件 | 子 Agent 写父通道，32 buffer 瞬间填满 |
| PEV 多轮迭代工具调用 | 每轮产生多个 tool_use/tool_result 事件，叠加后持续压力 |

### 2.4 Claude Code 的流控设计

Claude Code 的 `queryLoop` 是 async generator，不返回事件通道，每个消息通过 `yield` 直接推送给消费者：

```typescript
// 伪代码：queryLoop 的拉模式（async generator 直接 yield）
async function* queryLoop() {
  while (true) {
    // LLM 流式输出：for await 拉取 API 流
    for await (const message of callModel({...})) {
      yield message  // 每条消息直接 yield，不经过中间通道
    }
    // 工具执行结果：同样通过 for await 拉取
    for await (const result of streamingToolExecutor.getResults()) {
      yield result.message
    }
    // 有 tool_use 则继续循环，否则结束
    if (!hasToolUse(lastMessage)) break
  }
}

// 消费者也通过 for await 拉取
for await (const msg of queryLoop()) {
  render(msg)  // 按自己的节奏消费
}
```

**关键机制**：async generator 的 `yield` 本身就是背压边界——生成器 yield 后自动挂起，直到消费者 `for await...of` 拉取下一条才恢复执行。这等价于 Go 中 `chan` 在 buffer 满时写阻塞，但区别在于：async generator 根本没有"积压"的概念，生产者天然不可能快过消费者。

这种设计的核心优势：
1. **零积压** — yield 挂起生成器直到消费者就绪，不存在通道积压问题
2. **自然背压** — 消费者的消费节拍直接控制生产者的推进速度，不需要额外的 Drain/Compact 机制
3. **生命周期内聚** — 循环即生命周期，不需要 Drain→Compact→Reconnect 的外部管理

Devrix 选择 `chan *EngineEvent` 推模式的原因是 Go 中没有 async generator 的原生等价物（channel 是 Go 的标准并发原语），但可以通过 `chan` + 主动背压检测 + Drain/Compact 生命周期来弥补这一差距。

## 3. 验收标准

### P0 (阻止合并)

- [ ] 实现通道背压感知：EngineEvent 通道可查询当前积压量
- [ ] 实现 Drain 机制：积压 > 阈值时排空非关键事件类型，仅保留 EOM/Error 等终止事件
- [ ] 实现事件 Compact：同类连续事件（如 ProgressEvent）合并为聚合事件

### P1 (必须完成)

- [ ] 实现 Reconnect 生命周期：Drain → Compact → 重建通道 → 恢复处理
- [ ] EngineEvent 增加事件优先级字段（Critical / Normal / Low），Drain 时按优先级淘汰
- [ ] Fork/Join 场景下子 Agent 使用独立事件通道，不直接写入父通道
- [ ] 实现背压评测探针（`BackpressureProbe`）：测量通道积压 P50/P95/P99、Drain 触发频率与恢复时间，注册到 D6 Eval 框架

### P2 (建议完成)

- [ ] 提供事件通道监控指标（积压量、丢弃数、合并率），对接 D5 Metrics + Tracing
- [ ] EngineEvent 增加 TraceContext 字段（trace_id / span_id），支持事件维度分布式追踪
- [ ] 支持动态 buffer 大小调整，根据历史负载自动适配

## 4. 领域映射

| 子域 | 影响范围 | 预期工作量 |
|------|----------|-----------|
| `shared/contracts` | EngineEvent 扩展（优先级、Compact 标记） | 中 |
| `contextengine/pev` | PEV 引擎事件发送适配 | 中 |
| `contextengine/compression` | 事件 Compact 实现 | 高 |
| `multiagent/forkjoin` | 子 Agent 通道隔离 | 中 |
| `observability/metrics` | 通道监控指标 | 低 |
| `d6/eval` | Backpressure 探针 | 低 |

## 5. 回归风险

- Drain 逻辑可能意外丢弃关键事件，需严格按事件类型白名单过滤
- Compact 合并可能损失事件粒度，需保证调试场景下可关闭合并
- Reconnect 期间的状态一致性需 `-race` 测试验证
