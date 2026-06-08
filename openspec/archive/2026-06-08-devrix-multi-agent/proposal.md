# Proposal: Multi-Agent Layer V1

**Change ID:** devrix-multi-agent
**Demand ID:** DM-20260608-005
**Status:** Archived
**Version:** 1.0.0

---

## 1. Background

Devrix 六层架构中 L4（多智能体层）目前缺失代码实现。L1/L2/L3/L5 均已可用，L4 是连接 L1 CommunicationGateway 与 L2 ContextEngine 之间的编排层，负责 Agent 生命周期管理和并行任务编排。

## 2. Problem Statement

| # | 问题 | 影响 |
|---|------|------|
| P1 | 无并行任务能力 | 复杂任务（多文件重构）总耗时 = 串行步骤之和 |
| P2 | 无推理策略选择 | 所有任务使用同一 system prompt，无法按场景优化 |
| P3 | 无 Agent 状态追踪 | 用户无法感知 Agent 当前状态 |
| P4 | 工具权限与 Agent 状态割裂 | CRITICAL 工具缺少 WAITING_PERMISSION 状态标识 |

## 3. Alternatives Considered

### A: Agent 直接调用 LLM（绕过 PEVEngine）
- Pros: 简单直接
- Cons: 破坏分层，PEVEngine 循环需在 Agent 层重建
- **拒绝** — 重复建设

### B: 在 L2 内部实现 Agent
- Pros: 减少层数
- Cons: L2 职责膨胀
- **拒绝** — 违反单一职责

### C: Agent 委托 PEVEngine（采用）
- Pros: 复用 PEVEngine + PermissionManager + Registry，职责清晰
- Cons: V1 不实现 COW，需 RWMutex 保护
- **采纳** — 最优平衡点

## 4. Proposed Solution

Agent 层作为协调者，委托 L2 ContextEngine 执行 LLM 推理：

```
L1 CommunicationGateway → IAgentFactory.Create() → Agent.Run()
  → L2 IContextEngine.Process() → PEVEngine 循环
    → L3 ILLMGateway.ChatStream()
```

**V1 核心：**
- Agent 状态机（5 状态）
- Fork/Join 并行子 Agent（共享 `*SessionContext`）
- CollaborationMode prompt 增强
- 权限委托 `gateway.PermissionManager`
- Observer 桥接 `contextengine.IObserver`

## 5. Capabilities

| ID | Name | Description |
|----|------|-------------|
| AGT-FACTORY | Agent Factory | 创建 Agent，注入依赖，校验配置 |
| AGT-LIFECYCLE | Lifecycle | 状态机 + Run/Terminate/Wait |
| AGT-FORKJOIN | Fork/Join | 子 Agent 创建 + 并行 + 合并（max 3） |
| AGT-COLLAB | Collaboration | CoT / Iterative-Refinement / Default prompt |
| AGT-PERMISSION | Permission | 委托 PermissionManager，CRITICAL 需确认 |
| AGT-OBSERVER | Observer | AgentEvent → IObserver 适配器 |
| AGT-BOOTSTRAP | Bootstrap | WireMultiAgent 引导集成 |

## 6. Out of Scope (V1)

- Supervisor-Worker 自动检测并行任务（V2）
- Peer-Review / Vote-Consensus 模式（V3）
- Agent 持久化（V2）
- 完整 COW SessionContext（V2）
- 自建 Tool Registry / Permission Pipeline

## 7. Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| 共享 SessionContext data race | High | `sync.RWMutex` + `-race` CI |
| Fork 数量失控 | Medium | MaxChildren=3 硬限制 |
| 崩溃丢 Agent 状态 | Low | SessionContext 消息历史可恢复 |
| 权限等待阻塞 | Medium | 60s 超时自动拒绝 |

## 8. Dependencies

| Component | Layer | Status |
|-----------|-------|--------|
| `IContextEngine.Process()` | L2 | Available |
| `PermissionManager.Request()` | L1 | Available |
| `contextengine.IObserver` | L2 | Available |
| `observability.Bridge` | L5 | Available |

## 9. Success Metrics

| Metric | Target |
|--------|--------|
| Agent 创建延迟 | P99 < 10ms |
| Fork 创建延迟 | P99 < 15ms |
| Join 合并延迟 | P99 < 5ms |
| 状态转换覆盖 | 100% 合法 + 100% 非法拒绝 |
| 并发安全 | `-race` 0 failures |
| 测试覆盖 | ≥ 80% |

## 10. Related

- `docs/multi-agent-design.md` — 可读架构设计文档
- `openspec/specs/multi-agent/spec.md` — Canonical 规格
- `openspec/l5-registry.md` — L5 测试点注册

---

## Archive Information

**Archived:** 2026-06-08
**Duration:** 1 day
**Outcome:** Successfully implemented
**PR:** [#7](https://github.com/fqntxmqee/devrix/pull/7)
**Verdict:** ACCEPTED

### Specs Updated
- `openspec/specs/multi-agent/spec.md`
