---
demand-id: DM-20260611-001
title: D2 上下文引擎 Agentic Loop 深化 — 无限循环 + 流式工具执行 + 多层错误恢复
source: devrix-harness-architecture-audit
priority: P0
status: Superseded
superseded-by: DM-20260610-012
superseded-date: 2026-06-11
l1-domain: context-engine
created: 2026-06-11
---

# D2 上下文引擎 Agentic Loop 深化

> **状态：Superseded（2026-06-11）**
>
> 本需求主体能力已由 **DM-20260610-012（QueryLoop）** 交付：
> - while-true 循环（`query/loop.go`）
> - tool_use 驱动 continuation
> - `StreamingToolExecutor` 并行安全工具
> - `FilterIncompleteToolCalls` 孤儿 tool_use 补偿
> - `max_turns` 硬限制
>
> **未竟项**已迁移至 tech-debt，见 `openspec/tech-debt/queryloop-error-recovery.md`。
> 请勿在本 change 上继续开发。

## 1. 背景（审计原文，已过时）

当前 PEV 引擎使用固定次数循环（`MaxIterations=3`），没有真正的 Agentic Loop 能力。Claude Code Harness 的 `queryLoop()` 使用 while-true 循环……

> ⚠️ 当 `query_loop.enabled=true`（生产默认）时，上述前提不再成立。PEV 经 `runViaQueryLoop` 走 QueryLoop 主路径。

## 2. 已由 DM-012 覆盖的验收项

| 原 P0/P1 | DM-012 对应 |
|----------|-------------|
| while-true + tool_use 驱动 | `query/loop.go` |
| StreamingToolExecutor | `query/streaming_executor.go` |
| 强类型 tool ID | QueryLoop tool batch |
| 孤儿 tool_use 补偿 | `FilterIncompleteToolCalls` |
| max_turns 兜底 | QueryLoop 配置 |

## 3. 迁移至 tech-debt 的未竟项

| 项 | 目标承载 |
|----|----------|
| 413 → Collapse/Reactive Compact 恢复链 | `tech-debt/queryloop-error-recovery.md` |
| max_output_tokens recovery | 同上 |
| Loop 级 fallback model 接线 | 同上 |
| D6 LoopDepthProbe / ToolConcurrencyProbe | 同上 |

## 4. 关闭原因

审计批次（2026-06-11）编写时 QueryLoop 尚未合入；继续独立推进会导致与 DM-012 重复建设。
