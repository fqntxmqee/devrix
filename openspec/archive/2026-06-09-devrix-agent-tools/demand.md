---
demand-id: DM-20260608-012
title: Agent Tool 系统 — 注册外部 CLI 工具并由 LLM 按需分发
priority: P1
status: S1_Proposal
l1-domain: multi-agent
created: 2026-06-08
---

# Agent Tool 系统

## 1. 背景

当前 devrix 的 Multi-Agent 层（D4）只有同质的 `agent.Impl`，所有子 Agent 共享同一套 prompt 和能力配置，无法根据任务类型选择不同的外部工具来执行。

实际使用中，不同的外部 CLI Agent 有各自的擅长领域：Claude Code 擅长编码和代码审查，Gemini CLI 擅长研究和多模态分析，Cursor 擅长项目级重构。devrix 需要一个机制来注册多个外部 Agent 工具，由 LLM 根据用户意图自动分发给最合适的工具执行。

cc-connect 已经实现了类似的模式——通过 CLI 子进程启动外部 Agent（`claude --print --output-format stream-json`），devrix 可以复用这个思路，但需要通过 LLM tool calling 做路由决策，而不是 cc-connect 的平台消息路由。

## 2. 问题陈述

1. **单一种类 Agent**：当前 `agent.Impl` 是所有子 Agent 的唯一实现，无法为不同场景选择不同工具
2. **无意图路由**：Context Engine（D2）的 LLM 不知道可以调用外部 Agent，所有任务只能由主 LLM 自己完成
3. **无法扩展**：新增一个外部 Agent 需要改代码，没有配置驱动的注册机制
4. **无标准化接口**：外部 Agent 的调用方式、输出格式、超时控制没有统一抽象

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | 可通过 `devrix.yaml` 配置外部 Agent 工具（命令、参数、超时、能力标签） | P0 |
| AC2 | LLM 可通过 `call_agent` 工具调用外部 Agent 并获取结果 | P0 |
| AC3 | Agent 工具的注册、查找、按能力查询有标准 Registry | P0 |
| AC4 | CLI Agent 子进程有统一的超时控制和流式输出解析 | P0 |
| AC5 | 未启用 Agent 工具时对系统无影响（不注册 `call_agent` 工具） | P1 |
| AC6 | Registry 线程安全，支持并发调用 | P1 |
| AC7 | Agent Tool Session 按 `(D1 Session, Tool)` 隔离，不同 D1 Session 的同名工具独立运行互不干扰 | P0 |
| AC8 | Agent Tool Session 在首次调用时创建子进程，同 D1 Session 内再次调用同工具时复用已有进程 | P0 |
| AC9 | D1 Session 销毁时自动清理该 Session 下所有 Agent Tool 子进程 | P1 |
| AC10 | Agent Tool Session 空闲超时后自动回收子进程，释放资源 | P1 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | cc-connect 的 CLI 子进程通信模式参考（stream-json） |
| 约束 | 不可引入新的外部依赖库 |
| 约束 | CLI 子进程必须受 context timeout 控制，防止永久挂起 |
| 约束 | Agent 工具的注册不改变现有 Multi-Agent (D4) 的同质 Agent 协作逻辑 |

## 5. 变更范围

### 新增
- 统一的 Agent Tool 接口和注册表（D4 layer）
- CLI 子进程适配器（D4 layer）
- `call_agent` 内置工具注册到 Context Engine 的 ToolRegistry（D2 layer）
- Agent Tool 配置类型和加载函数（Shared Config）

### 修改
- `bootstrap.NewContextEngine()` 接受 Agent Tool Registry 参数
- `bootstrap.ContextEngineBuilder` 接受 Agent Tool Registry 参数
- `cmd/devrix/main.go` 从配置加载 Agent Tools 并构建 Registry
- `devrix.yaml` 添加 `agent_tools` 配置段

### 不变更
- 现有 Multi-Agent (D4) 的同质 Agent 协作逻辑（Fork/Join、Collaboration Mode）
- Context Engine (D2) 的核心 PEV 流程
- LLM Gateway (D3) 的 Provider/Model 路由
- Communication Layer (D1) 的消息处理

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| CLI 子进程长时间挂起 | 阻塞 LLM 响应 | context timeout 兜底 + 默认 5min 超时 |
| stream-json 格式不兼容 | 解析失败，结果丢失 | 非 JSON 行降级为 text 事件 |
| 多个外部 Agent 同时执行 | 资源竞争 | Registry 线程安全，每个 Execute 独立子进程 |
