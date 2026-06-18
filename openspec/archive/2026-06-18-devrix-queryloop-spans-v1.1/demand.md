---
demand-id: DM-20260612-014
title: QueryLoop Span 对齐 v1.1 — runViaQueryLoop 补 iteration / llm_call span
source: devrix-harness-unification v1.0 S4 副作用（QueryLoop 取代 legacy PEV 后丢失中间 span）
priority: P1
status: S1_Proposal
dsaft_domain: context-engine
parent_change: devrix-harness-unification
created: 2026-06-12
---

# QueryLoop Span 对齐 v1.1

## 1. 背景

DM-20260611-004（Harness Unification S4）将 `query_loop.enabled` 默认设为 `true`，
`PEVEngine.runExecuteVerifyLoop` 实际路径改走 `runViaQueryLoop`。但
`runViaQueryLoop` **只发射** `OpContextPEVRun` 一根 span，**缺失**：

- `OpContextPEVIteration`（每轮 LLM↔Tool 循环）
- `OpContextPEVLLMCall`（每轮 LLM 子 span，client kind + `gen_ai.*` 属性）

而 D5 既有集成测试仍按 legacy PEV 的 span 树写断言：

| 测试 | Build Tag | 失败断言 |
|------|----------|---------|
| `TestIntegration_PEVSpanHierarchy_should_match_canonical_tree` | `integration && cross` | `no spans named "context.pev.iteration"` |
| `TestIntegration_FullChainD1toD3` | `integration && cross` | `expected >= 3 PEV iterations, got 0` |
| `TestIntegration_D2MultiRoundPEV` | `integration && cross` | iteration span 缺失 |

3 个测试在 `master` 与 `fix/remaining-critical` 上均失败 — 属于 DM-20260611-004 合入后
未同步的可观测性回归，不阻塞 5 个 S4 change 合并（验收已 100% 覆盖各自范围），但是
**D5-TRACE-T04 / D5-TRACE-T06 在 CI 上长期红灯**，需 v1.1 闭环。

## 2. 改动范围

### 2.1 `runViaQueryLoop` 内嵌 iteration / llm_call span

`internal/layers/contextengine/query_loop_run.go::runViaQueryLoop`：

```go
// 当前：单根 OpContextPEVRun → query.Loop.Run（无 span）
// 期望：
//   OpContextPEVRun (existing)
//     ├── OpContextPEVIteration (per-turn, internal kind)
//     │     ├── OpContextPEVLLMCall (client kind, gen_ai.* attrs)
//     │     │     └── OpLLMStream (already emitted by llm bridge)
//     │     └── OpContextPEVToolExecute (per-tool, internal kind)
//     │           └── OpContextPEVPermissionCheck
//     └── OpContextPEVVerify (per-turn, after AfterToolRound)
```

最小实现路径（不动 `query/loop.go` 内部代码）：

1. `query.LoopHooks` 增加 `BeforeTurn` / `AfterTurn` / `BeforeLLMCall` / `AfterLLMCall` 4 个 hook 点
2. `runViaQueryLoop` 注入 hook：在 `BeforeTurn` 起 iteration span，在 `AfterTurn` 关闭；
   在 `BeforeLLMCall` 起 llm_call span 并附 `gen_ai.request.model` / `gen_ai.conversation.id`，
   `AfterLLMCall` 写入 usage 后关闭
3. `WrapToolContext` 自然把 toolExecute span 串到 iteration ctx 下（如需 toolExecute 显式 span 由
   `toolExecutor` 内部发射）

### 2.2 LLM bridge 上的 `gen_ai.*` 属性双写

确认 `OpContextPEVLLMCall` span 写入：

- `gen_ai.system = devrix`
- `gen_ai.request.model = sc.Model`
- `gen_ai.conversation.id = sc.SessionID`
- `gen_ai.usage.input_tokens` / `gen_ai.usage.output_tokens`（done 后写）

### 2.3 测试

- 现有 3 个测试**不修改**，期望直接转绿
- 新增 `TestIntegration_QueryLoopSpans_should_emit_iteration_per_turn`（cross tag）
  覆盖 2 轮 tool round 场景，断言：
  - 1 个 `context.pev.run` + 2 个 `context.pev.iteration`
  - 每个 iteration 下至少 1 个 `context.pev.llm_call`

## 3. T 层覆盖

| 测试 ID | 描述 |
|---------|------|
| D5-TRACE-T04（既有） | Canonical span hierarchy parent-child |
| D5-TRACE-T06（既有） | SpanKind contract |
| D5-TRACE-T07（新增） | QueryLoop 路径下 iteration span 数量等于 turn 数 |

## 4. 验收门禁

- `go test -tags 'integration cross' ./tests/integration/` 0 failure
- `go test ./internal/layers/contextengine/...` 0 failure
- D5-TRACE-T04 / -06 / -07 全部 PASS
- D5 canonical trace tree 文档同步更新（`openspec/specs/project/canonical-trace-tree.md` 若存在）

## 5. 时间窗口

- S2-S4：1-2 天（hook 注入 + 测试）
- S5 验收：当日
- 阻塞性：**非阻塞** 5 个 S4 change 合并；可在 master 合并后单独 PR
