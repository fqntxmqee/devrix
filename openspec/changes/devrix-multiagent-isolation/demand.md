---
demand-id: DM-20260611-005
title: 多 Agent 会话隔离 — Join 合并与 Session 元数据隔离
source: devrix-harness-architecture-audit（2026-06-11 修订）
priority: P1
status: S2_Revised
revised: 2026-06-11
l1-domain: multi-agent
created: 2026-06-11
---

# 多 Agent 会话隔离

## 1. 背景（修订）

2026-06-11 审计指出 Fork/Join 共享 `session` 指针导致竞态。  
**DM-012 QueryLoop v2 已部分缓解：**

| 机制 | 位置 | 作用 |
|------|------|------|
| 每 Agent 独立 `messageBuffer` | `agent/agent.go` | Agent 层消息累积隔离 |
| Fork Worker → `WorkerEngine` | `factory/factory.go:91-93` | D2 `ProcessOverlay` 上下文隔离 |
| `BuildForkedMessages` | `query/fork.go` | SubQuery cache-friendly fork 前缀 |

**仍存在的真实风险**（修订后 P0 焦点）：

- Fork 子 Agent 仍持有父 `*types.Session` 指针（session ID / metadata 共享）
- Join 合并语义未保证排序/去重
- 并发 Fork 时 Session 元数据 / ContextSnapshot 写入未 `-race` 验证
- 与 **DM-007 Wave Scheduler** SubAgent 槽的 `ContextPolicy=fresh` 需对齐

## 2. 问题陈述（修订）

### 2.1 ~~消息列表竞态~~ → 降级为已缓解

原审计「子 Agent A/B 同时写 Session.Messages 导致交错」—— 当前 Agent 层使用独立 `messageBuffer`，D2 侧 Worker 使用 `ProcessOverlay`。**不应再作为 P0 主论据。**

### 2.2 Session 元数据与 Join 合并（新 P0）

| 问题 | 位置 | 影响 |
|------|------|------|
| 共享 `*types.Session` 指针 | `agent/agent.go` Fork → `creator.Create(..., a.session)` | metadata / snapshot 字段并发写 |
| Join 合并无排序去重 | `agent/agent.go` Join | 父上下文乱序或重复 |
| 压缩触发跨 Agent | D2 compression | 多 Worker 同时 compact 同一 Session |

### 2.3 与 Wave Scheduler 的衔接

DM-007 要求 SubAgent 槽 **fresh context**（directive only）。本需求 P1 应提供：

- Session 快照 API：`SnapshotMetadata()` / `ForkSessionView()` 供 Scheduler 注入
- Join 时 artifact 合并而非全量 Messages 灌入 Leader

## 3. 验收标准（修订）

### P0

- [ ] Join() 合并路径：消息按 agent 完成序排序 + tool_call ID 去重，`-race` 测试通过
- [ ] Fork 子 Agent 对 Session **metadata 写操作**隔离（COW 或显式禁止回写）
- [ ] 集成测试：并发 3 Fork + Join，父 Agent 上下文一致

### P1

- [ ] Session 快照机制：`ForkSessionView(parent)` 共享 ID + 隔离 overlay 字段
- [ ] 与 DM-007 `ContextPolicy` resolver 对接
- [ ] `SessionIsolationProbe` 注册 D6 Eval

### P2

- [ ] Task 抽象（local_bash / local_agent）作为 Fork 通用替代
- [ ] Fork 策略注入：CopyOnWrite / Snapshot / Shared（Shared 仅测试/兼容）

## 4. 非目标

- **不**要求每个 Fork 完整复制 Session（内存开销大；Wave fresh policy 只需 directive + 元数据子集）
- **不**重复 DM-012 SubQuery / Delegate 已实现的 overlay 隔离

## 5. 领域映射

| 子域 | 变更 |
|------|------|
| `multiagent/agent` | Join 合并 + metadata COW |
| `multiagent/factory` | ForkSessionView 注入 |
| `contextengine` | ProcessOverlay 与 snapshot 对齐 |
| `orchestration/`（DM-007） | ContextPolicy 消费快照 API |

## 6. 回归风险

- Join 排序变更可能影响依赖「追加顺序」的现有测试
- metadata COW 需压测大 Session 拷贝开销
