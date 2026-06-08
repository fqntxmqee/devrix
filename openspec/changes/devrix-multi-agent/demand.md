---
demand-id: DM-20260608-005
title: "Multi-Agent Layer V1 — 生命周期 + Fork/Join + 协作模式"
source: docs/multi-agent-design.md
priority: P0
status: S5_ACCEPTANCE
l1-domain: multi-agent
created: 2026-06-08
---

# Multi-Agent Layer V1

## 1. 原始描述

实现 Layer 4 Multi-Agent 层：Agent 生命周期状态机、Factory 创建、Fork/Join（消息隔离 + Join 合并）、协作模式 prompt 增强、AgentPermissionGate 异步权限、Observer 桥接，并接入 CommunicationGateway bootstrap。

## 2. 澄清记录

### Q1: Agent 直接调用 LLM 还是委托 PEVEngine？

**A**: 委托 PEVEngine（`IContextEngine.Process()`）。Agent 是协调者，不持有 LLM/ToolRunner。 — 2026-06-08

### Q2: Fork 消息如何共享？

**A**: Grill 决策：子 Agent 使用独立消息缓冲区（`GetMessages()`），Join 时合并到父 Agent；Session 元信息共享 `*types.Session`，避免并发写 SessionContext。 — 2026-06-08

### Q3: 权限模型？

**A**: Agent 实现 `contextengine.IPermissionGate`（AgentPermissionGate），CRITICAL 工具通过 channel 阻塞，Gateway 调用 `ResolvePermission()` 注入用户响应；非 CRITICAL 自动批准。 — 2026-06-08

### Q4: V1 范围？

**A**: 不含 Supervisor-Worker / Peer-Review / Vote-Consensus。Fork 同步 Wait→Join 模型。子 Agent 上限硬编码为 3。Agent V1 不持久化。 — 2026-06-08

### Q5: CollaborationMode 是接口还是配置？

**A**: 配置类型 + prompt 生成器。本质是 system prompt 差异，不需要独立接口。 — 2026-06-08

### Q6: Observer 自建还是桥接？

**A**: 通过 ObserverAdapter 桥接到 `contextengine.IObserver`，复用现有事件管线。 — 2026-06-08

## 3. 范围定义

### In Scope

- Agent 状态机（5 状态、8 合法转换）
- AgentFactory + Agent 接口 + 实现
- Fork/Join（V1 简化版，共享指针 + RWMutex）
- CollaborationMode（3 种模式 prompt 增强）
- 权限委托 PermissionManager
- ObserverAdapter 桥接
- 错误码（9 个 AGT_*）+ 配置类型
- 单元测试 + 集成测试 + -race 检测

### Out of Scope

- Supervisor-Worker 模式（V2）
- 完整 COW SessionContext（V2）
- Peer-Review / Vote-Consensus（V3）
- Full Fork/Merge + Milestone DAG（V3）
- Agent 持久化（V2）
- 自建 Tool Registry / Permission Pipeline / Event Bus

## 4. L1-L5 映射

| 层级 | 资产 |
|------|------|
| L1 | AgentFactory 注入 CommunicationGateway；PermissionManager 委托 |
| L2 | IContextEngine.Process() 委托；IObserver 桥接 |
| L3 | 间接通过 PEVEngine，Agent 不直接调用 |
| L4 | Agent / Factory / ForkJoin / Collaboration / Observer（本需求全部） |
| L5 | L5-4-1-01 ~ L5-4-0-04（15 个测试点） |

## 5. 验收标准

- P0：Factory 创建 + 生命周期状态机 + Fork/Join 单元测试全绿 + -race 通过
- P1：Permission 流程集成 + Observer 桥接 + Collaboration prompt 生成
- P2：Gateway bootstrap 接入 + E2E Fork 场景

## 6. 依赖关系

| 依赖 | 状态 |
|------|------|
| L2 `IContextEngine.Process()` | Available |
| L1 `PermissionManager.Request()` | Available |
| L2 `IObserver` | Available |
| L5 `observability.Bridge` | Available |
| `types.SessionContext` | Available |
| `types.RiskLevel` | Available |
