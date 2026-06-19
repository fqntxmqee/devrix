---
demand-id: DM-20260618-010
title: D2 QueryLoop 拆解 — D2 纯上下文组装服务
source: D2 领域定位收敛 + TD-QL-LOC Z1/Z2 演进
priority: P1
status: S2_Proposal
dsaft_domain: context-engine
created: 2026-06-18
---

# D2 QueryLoop 拆解

## 1. 背景

DM-020 已将 **Turn 主循环与 LLM 调用权** 上移至 D7（`RunTurnLoop` + `InvokeLLM → D3`）。DM-20260617-001（Z0）为 legacy `D2.QueryLoop.Run` 加了 Deprecated 标记与 metric，但**未删除代码**。

当前 D2 领域 North Star 要求：

> D2 = 被 D7 调度的纯执行原语：**Prepare / ToolRound / Persist**。不拥有循环、不调用 LLM、不调度 Agent。

然而 `internal/layers/contextengine/query/loop.go` 仍持有 `while(tool_use)` 循环，且被以下路径**活跃使用**（非仅回滚）：

| 调用方 | 路径 |
|--------|------|
| Wave SubAgent | `wire_wave.go` → `RunBackground` → `SubQuery` → `Loop.Run` |
| delegate_subquery | `delegate.go` → `BuildSubQueryRunner` → `Loop.Run` |
| Background 任务 | `enforce/background.go` → `SubQuery` → `Loop.Run` |
| Legacy ingress | `engine.go` → `Process` → `Loop.Run`（`rule_orchestrate`） |

这使 D2 在语义上仍是「迷你 D7」，与领域定位冲突。

## 2. 问题陈述

| # | 问题 | 影响 |
|---|------|------|
| P1 | D2-S16 QueryLoop 仍拥有循环调度权 | 违反 D7=Leader / D2=Follower |
| P2 | D2 Engine 构造硬依赖 `QueryLLMCaller` 注入 | D2 语义上仍「知道 LLM 轮次」 |
| P3 | Wave/SubQuery/Background 绕过 D7 RunTurn | 子路径与主路径架构不一致 |
| P4 | TD-QL-01~03 错误恢复绑在 QueryLoop | 新能力需双路径维护 |
| P5 | `QueryLLMCaller` 拆面 adapter 存在 | D2↔D3 绕道未彻底消除 |

## 3. 目标行为

```text
所有 LLM↔Tool 循环均由 D7 RunTurnLoop 持有：

D1 → D7.ProcessMessage
       ├── 主 Turn（FastPath）     → RunTurn(scope=main)
       ├── SubQuery / Background   → RunTurn(scope=sub|background)
       └── Wave SubAgent Worker    → RunTurn(scope=wave_worker)

D2 仅提供无状态原语：
       PrepareExecutionContext(session, message) → messages, tools, compress_hint
       ExecuteToolRound(session, tool_calls)     → tool_results
       PersistSessionState(session, turn)        → durable snapshot
```

D2 **不得** import D7/D3/D4；D7 通过 bootstrap adapter 调用 D2 三个原语。

## 4. 澄清记录

### Q1: 是否等待 legacy metric 12 周为 0 再删？

**A**: 本 change **不依赖** metric 归零。Z0 metric 针对 `rule_orchestrate` 回滚路径；Wave/SubQuery/Background 是活跃路径，需先迁移调用方再删 Loop。迁移完成后 metric 自然归零。

### Q2: SubQuery 的 sidechain / FlowReporter 如何处理？

**A**: 保留在 D7 SubTurn 层。D7 `RunTurn(scope=sub)` 注入 `SubQueryFlowReporter` 与 sidechain hook；D2 不参与 Flow 语义。

### Q3: TD-QL-01~03（413 恢复、fallback model）归属？

**A**: 迁移至 D7 `DefaultOrchestrator`（或 `turn/recovery.go`）。本 change Phase 4 一并收编，避免双路径。

## 5. 验收标准（AC）

| AC | 描述 | 优先级 |
|----|------|--------|
| AC1 | `contextengine/query/loop.go` 删除；`D2-S16` 标 REMOVED | P0 |
| AC2 | Wave SubAgent / SubQuery / Background 零 `Loop.Run` 调用 | P0 |
| AC3 | `engine.Process` 不再调用 QueryLoop（或 Process 标 Deprecated 仅保留 facade） | P0 |
| AC4 | D2 Engine 构造不再 require `QueryLLMCaller` | P0 |
| AC5 | `turn.QueryLLMCaller` 拆面 adapter 删除；D7 主路径仅用 `GatewayInvoker` | P0 |
| AC6 | `routing_mode=rule_orchestrate` 删除或 thin-wrap 到 D7 RunTurn | P1 |
| AC7 | `query_loop.enabled` 配置项删除 | P1 |
| AC8 | TD-QL-01~03 能力在 D7 Turn 路径有等价实现或显式 defer 登记 | P1 |
| AC9 | `go test ./...` + layer-lint + `d2_d3_ban_test` 全绿 | P0 |
| AC10 | `d2-domain.md` / `d7-boundary.md` / spec delta 合并 | P0 |

## 6. 非目标

- D4 Agent 执行体改造（Worker 仍由 D4 跑，D7 调度不变）
- QueryLoop 时代的历史测试全量重写（LEGACY 测试可删或迁 D7）
- D6 自演化 / WorkTree v2.1 defer 项（见 TD-WT-DEF）

## 7. 依赖

| 依赖 | 状态 |
|------|------|
| DM-020 D7 Turn 编排上移 | ✅ |
| DM-20260617-001 Z0 deprecation + metric | ✅ |
| D7 turn_adapter（Prepare/ToolRound/Persist） | ✅ |
| bootstrap wire_coordinator loopFirst 默认 | ✅ |
