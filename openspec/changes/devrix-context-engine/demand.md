---
demand-id: DM-20260607-002
title: 上下文引擎（Layer 2）详细方案设计
source: 产品/架构团队
priority: P0
status: IN_PROGRESS
l1-domain: devrix
created: 2026-06-07
---

# 上下文引擎（Layer 2）详细方案设计

## 1. 原始描述

> 按照 OpenSpec 规范，为 Devrix 上下文引擎（Context Engine Layer）做详细方案设计。
> 参考项目架构文档（`openspec/specs/context_engine_layer_delta.md`、`openspec/archive/devrix-foundation/`、通信层 design 中的引擎集成章节），
> 替换当前 `StubContextEngine`，实现 PEV 循环、七步压缩与分层记忆。

## 2. 澄清记录

### Q1: docs/ 目录与框架？
**A**: 已建立 `docs/detail design framework.md`（六段式框架）与 `docs/context-engine-design.md`（按框架展开的 Layer 2 详细设计）。OpenSpec 变更目录保留实施级 design/spec/tasks。 — 2026-06-07

### Q2: V1 范围？
**A**: V1 实现真实 ContextEngine（替换 stub）：简化 PEV（Execute→Verify）、压缩步骤 1-5+7、Working+ShortTerm 记忆；Plan 阶段与 Milestone DAG 对接留 V3。 — 2026-06-07

### Q3: 与通信层边界？
**A**: 通信层只依赖 `IContextEngine.Process()` 事件流；上下文引擎不反向依赖 Adapter。Session.ContextSnapshot 由引擎写入。 — 2026-06-07

### Q4: LLM / Multi-Agent 依赖？
**A**: V1 通过接口依赖 `ILLMGateway`、`IToolRunner`、`IToolRegistry`（由 multi-agent 层提供 stub/最小实现）；不阻塞上下文引擎独立开发与测试。 — 2026-06-07

### Q5: 权限握手与 Gateway 关系？
**A**: V1 采用 `IPermissionGate` 注入 L2；PEV 在 `IToolRunner` 前同步审批；Gateway 对 `tool_call` 仅展示，不阻塞事件流。 — 2026-06-07

### Q6: verify_mode 默认值？
**A**: V1 默认 `basic`（tool result 无 error 即通过）。 — 2026-06-07

## 3. 澄清范围

### 3.1 L1-L5 映射（草案）

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | devrix | 开发大脑 | 已有 |
| L2 | L2-DEVRIX-02 | 对话式开发助手 | 已有 |
| L3-BE | L3-BE-CTX-01 | 处理用户消息并维护上下文 | 新增 |
| L3-BE | L3-BE-CTX-02 | 超长对话压缩 | 新增 |
| L4-BE | L4-CTX-PEV | PEV 执行循环 | 新增 |
| L4-BE | L4-CTX-COMPRESS | 七步压缩管道 | 新增 |
| L4-BE | L4-CTX-MEMORY | 分层记忆 | 新增 |
| L4-BE | L4-CTX-STATE | 上下文状态管理 | 新增 |
| L5 | L5-CTX-* | 见 `l5-registry.md` | 草拟 |

### 3.2 范围

**In Scope（本变更）**:
- OpenSpec 四件套（proposal / design / specs / tasks）
- L5 测试点登记
- 架构与接口设计（Go 实现导向）

**Out of Scope（本变更）**:
- 代码实现（S4）
- Autocompact（V2）
- 长期记忆 SQLite（V3）
- LLM Gateway 完整实现
