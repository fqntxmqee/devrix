---
demand-id: DM-20260611-005
title: 多 Agent 会话隔离 — Fork/Join 独立 Session 上下文
source: devrix-harness-architecture-audit
priority: P0
status: S1_Proposal
l1-domain: multi-agent
created: 2026-06-11
---

# 多 Agent 会话隔离

## 1. 背景

当前 Fork/Join 实现中，Fork() 创建子 Agent 时直接复用父 Agent 的 `session` 指针。Join() 将所有子 Agent 的消息追加到同一共享 Session。当子 Agent 并发运行时会存在竞态写入和状态污染风险。对照 Claude Code Harness 的 Task 隔离模型（4 种 Task 类型各有独立状态上下文），差距明显。

## 2. 问题陈述

### 2.1 共享 Session 指针

| 问题 | 位置 | 影响 |
|------|------|------|
| Fork() 子 Agent 使用 `a.session` | `agent/agent.go` Fork() | 子 Agent 与父 Agent 共享同一 Session |
| Join() 追加至同一 Session | `agent/agent.go` Join() | 所有子 Agent 输出合并到同一上下文 |
| 并发写入无保护 | `agent/agent.go` | 竞态条件导致消息交错或丢失 |

**证据**：`internal/layers/multiagent/agent/agent.go` 中 Fork() 创建的 child Agent 持有 `a.session` 的同一引用。

### 2.2 引入的缺陷场景

| 场景 | 后果 |
|------|------|
| 子 Agent A 写 Session 消息列表时被 B 中断 | 消息交错，上下文乱序 |
| 子 Agent 修改 Session Metadata | 父 Agent 感知到意外状态变更 |
| 多个子 Agent 触发压缩 | 重复压缩或压缩冲突 |

### 2.3 Claude Code 的对照

Claude Code 使用 Task 类型（local_bash / local_agent / remote_agent / dream），每个 Task 拥有独立的：
- 工作目录（cwd）
- 状态上下文（state）
- 输出通道（output channel）
- 生命周期（独立的创建 → 执行 → 完成 → 清理）

Fork/Join 应借鉴此模式，为每个子 Agent 分配独立的 Session 快照而非共享指针。

## 3. 验收标准

### P0 (阻止合并)

- [ ] Fork() 创建子 Agent 时分配独立的 Session 副本（Copy-on-Write 或完整快照）
- [ ] Join() 合并时进行消息排序和去重，保证父 Agent 上下文一致性
- [ ] 消除所有因共享 Session 指针导致的竞态风险，通过 `-race` 测试

### P1 (必须完成)

- [ ] 实现 Session 快照机制：允许按需选择共享字段元数据和隔离消息列表
- [ ] 子 Agent 生命周期内对 Session 的修改不回写父 Agent（除非显式 Export）
- [ ] 提供兼容模式（通过 Opt-in 保留共享行为）以便逐步迁移现有调用方
- [ ] 实现会话隔离评测探针（`SessionIsolationProbe`）：`-race` 测试持续通过率 + 快照拷贝性能基线，注册到 D6 Eval 框架

### P2 (建议完成)

- [ ] 实现 Task 抽象的独立上下文（参考 Claude Code 的 4 种 Task 类型），作为 Fork/Join 的通用替代
- [ ] Fork 支持策略注入（CopyOnWrite / Snapshot / Shared）按场景选择
- [ ] Fork/Join 运行时指标：Fork 次数、Session 快照大小、快照拷贝延迟、Join 合并消息数，通过 D5 暴露

## 4. 领域映射

| 子域 | 影响范围 | 预期工作量 |
|------|----------|-----------|
| `multiagent/agent` | Fork/Join 会话隔离改造 | 高 |
| `multiagent/contracts` | AgentState 扩展（Session 策略） | 中 |
| `shared/contracts` | Task 抽象定义 | 高 |
| `contextengine` | 适配独立 Session 的压缩和路由 | 中 |
| `communication` | 适配独立 Session 的响应分发 | 低 |
| `observability/metrics` | Fork/Join 操作指标 | 低 |
| `d6/eval` | SessionIsolation 探针 | 低 |

## 5. 回归风险

- 从共享 Session 到独立 Session 的迁移可能破坏依赖共享状态的外部逻辑
- Copy-on-Write 实现可能引入显著的额外内存开销（需压测验证）
- Task 抽象引入后需同步更新 Fork/Join 控制流
