# Proposal: Devrix 可观察层增强 — AI 排查就绪

**Change ID:** devrix-observability-enhancement
**Demand ID:** DM-20260610-001
**Layer:** L5 - Observability
**Type:** Enhancement
**Status:** Draft (Revised 2026-06-10 — post code review)
**Based on:** `docs/observability-design.md`, `openspec/l5-registry.md`, `demand.md`

---

## Revision Note（供二次 Review）

> **2026-06-10 Code Review 结论**：原 proposal 按「现状缺失」编写，与代码严重脱节。本版将需求重心从「补埋点 / 提覆盖率」调整为 **「AI 与人类可可靠还原请求因果链」**。已完成项标记 ✅，剩余工作按 P0/P1/P2 重排。

---

## Problem Statement

### 问题 0（新，P0）：文档与代码脱节

| 原 proposal 声称 | 代码实际（2026-06-10） |
|-----------------|----------------------|
| `AddLLMRequestEvent` 从未调用 | ✅ `pev_engine.go:273/353/558` 已调用 |
| 无 `context.pev.iteration` | ✅ 已实现 |
| 缺 compression/recall/synthesis 等 9 个 operation | ✅ Registry 已有 44 个 operation，主链埋点已落地 |
| 覆盖率 ~50% | Registry 静态注册接近完整；**Runtime Hit** 与 **Span 层级** 才是真实缺口 |

**影响**：若按旧 proposal 验收，会出现「文档全 TODO、代码已绿」的假象，掩盖 AI 排查的核心 blocker。

### 问题 1（P0）：Span 因果链扁平化 — AI 无法可靠 RCA

Jaeger 中 operation **存在**，但 **父子层级错误**：

```
# 期望（AI 可推理）
context.pev.run
└── context.pev.iteration
    ├── context.pev.llm_call → llm.stream → llm.adapter.stream
    ├── context.pev.tool_execute → context.pev.permission_check
    └── context.pev.verify

# 实际（代码缺陷）
context.pev.run
├── context.pev.iteration    ← defer 导致时长/生命周期错误
├── context.pev.llm_call     ← 与 iteration 平级（ctx 未传递）
├── llm.stream               ← 挂在 run 下，非 llm_call 子节点
└── ...
```

**根因**：
1. `_, iterSpan := e.startSpan(ctx, ...)` 丢弃返回 ctx，子 span 无法挂到 iteration 下
2. `ChatStream(ctx, req)` 使用 runSpan ctx，非 llmSpan ctx
3. 循环内 `defer iterSpan.End()`，所有 iteration span 在函数返回时一次性结束

**AI 影响**：模型无法从 trace 树推断「第 N 轮迭代 → 哪次 LLM → 哪个 tool → verify 为何失败」，易误判时序与根因。

### 问题 2（P0）：Logs ↔ Trace ↔ LLM 三轨分离

| 信号源 | 现状 | AI 影响 |
|--------|------|---------|
| Trace (Jaeger/OTLP) | span + LLM events | 层级扁平 |
| LLM JSONL (`~/.devrix/logs/llm/`) | 按 session 落盘，内容最全 | **无 trace_id** |
| 业务 slog | gateway/engine/pev 广泛使用 | **无 trace_id 自动注入** |

`StructuredLogger.WithTrace()` 存在但未接入业务路径；`include_trace_id: true` 对 slog 无效。

**AI 影响**：无法从 error log 一键跳转 trace，需跨三存储手工用 session_id 对齐。

### 问题 3（P1）：Metrics 缺口 — AI 无法先做聚合筛选

| Metric | 设计 | 代码 |
|--------|------|------|
| `tool_latency` Histogram | 有 | ❌ 仅有 `engine_tool_calls` Counter |
| `compression_ratio` Histogram | 有 | ❌ 未实现 |
| `session_active_count` | 有 | ✅ 已有 `active_sessions` Gauge |

**AI 影响**：无法回答「哪个 tool P99 最慢」，只能逐条读 trace。

### 问题 4（P1）：缺少决策语义 — AI 只能猜「为什么」

| 决策点 | 当前 span 属性 | 缺失 |
|--------|---------------|------|
| PEV 进入下一轮 | `verify.passed=false` | 失败原因摘要 |
| 触发 compression | tokens 前后 | budget 阈值、触发策略 |
| synthesis / fallback | event metadata | 未结构化进 span |
| Tool 执行 | input/output **preview 500 字符** | 完整 I/O 仅在 JSONL（未关联 trace） |

### 问题 5（P2，降级）：Baggage 未使用

单进程场景 span attributes 已足够；Baggage 降为 P2，等多服务时再启用 OTel 标准 propagation。

---

## Proposed Solution

### 方案 1（P0）：Span 传播规范 + PEV 层级修复 ✅→🔧

**规范（强制）**：凡创建子 span，必须使用返回 ctx 并传给下游：

```go
// ✅ 正确
ctx, iterSpan := e.startSpan(ctx, OpContextPEVIteration, ...)
defer iterSpan.End() // 禁止在 for 循环内 defer

ctx, llmSpan := e.startSpan(ctx, OpContextPEVLLMCall, ...)
defer llmSpan.End()
AddLLMRequestEvent(llmSpan, ...)
chunks, err := e.llm.ChatStream(ctx, req) // ctx 含 llmSpan

// ❌ 禁止
_, iterSpan := e.startSpan(ctx, ...)
defer iterSpan.End() // 在 loop 内
chunks, err := e.llm.ChatStream(ctx, req) // 未更新 ctx
```

### 方案 2（P0）：Log-Trace-LLM 关联

1. slog Handler 从 ctx 自动注入 `traceId` / `sessionId`
2. LLM JSONL 记录增加 `trace_id` / `span_id` 字段
3. 错误 span 统一写入 `error.code`（SentinelError）

### 方案 3（P1）：Metrics 补齐

- `tool_latency` Histogram（labels: tool, risk_level, status）
- `compression_ratio` Histogram（压缩触发时 observe）
- `pev_iteration_total` Counter（可选，便于聚合迭代次数）

### 方案 4（P1）：决策语义 Span 属性

- `verify.failure_reason` — verify 未通过时的可读摘要
- `compression.trigger_reason` — 超 budget / message count 等
- `pev.synthesis_source` — synthesis | tool_fallback

### 方案 5（P1）：Session Incident Export

CLI/API：`devrix debug export --session <id> --format json`

输出 bundle：
```json
{
  "session_id": "...",
  "trace_tree": { /* 有序 span 树 */ },
  "llm_rounds": [ /* 来自 JSONL，含 trace_id */ ],
  "errors": [ /* 关联 error spans + logs */ ],
  "metrics_snapshot": { /* 相关 histogram 摘要 */ }
}
```

### 方案 6（P2，可选）：Baggage

等多服务拆分再启用；本 change 不阻塞。

---

## Capabilities

| Capability | L5 ID | 优先级 | 状态 |
|------------|--------|--------|------|
| llm-trace-complete | L5-OBS-TRACE-01 | P0 | ✅ 代码已有 |
| pev-iteration-trace | L5-OBS-TRACE-02 | P0 | ✅ 代码已有，🔧 层级待修 |
| span-hierarchy-contract | L5-OBS-TRACE-04 | P0 | ❌ 新增 |
| log-trace-correlation | L5-OBS-TRACE-05 | P0 | ❌ 新增 |
| tool-latency-metrics | L5-OBS-METRICS-01 | P1 | ❌ 新增 |
| compression-metrics | L5-OBS-METRICS-02 | P1 | ❌ 新增 |
| session-incident-export | L5-OBS-EXPORT-01 | P1 | ❌ 新增 |
| verify-decision-attrs | L5-OBS-DECISION-01 | P1 | ❌ 新增 |
| baggage-context-propagation | L5-OBS-TRACE-03 | P2 | 降级 |
| coverage-completion | L5-OBS-COVERAGE-01 | P2 | 辅助指标 |

---

## Scope

### In Scope
- `contextengine/pev_engine.go` — ctx 传播 + defer 修复 + 决策属性
- `contextengine/engine.go` — compression 决策属性
- `observability/` — slog trace 注入、LLM JSONL trace_id、metrics、incident export
- `tests/integration/full_chain_trace_test.go` — 层级契约测试
- `docs/observability-design.md` — Canonical Trace Tree

### Out of Scope
- Agent 路径 trace 统一（独立 change）
- Error-biased sampling（独立 change）
- 前端可视化
- 新增 exporter

---

## Goals (SLO) — AI 排查就绪

| 档位 | 场景 | 目标 | 验收 |
|------|------|------|------|
| L1 辅助 | 人工/AI 读 Jaeger + JSONL | 中等复杂度 RCA | 现有 ~70%，层级修复后 ≥85% |
| L2 基本 | 给定 session_id 自动还原因果链 | AI 可输出有序事件链 | incident export + 层级测试全绿 |
| L3 自主 | AI Agent 闭环排查 | 暂不要求 | Out of Scope |

---

## Success Criteria (S3 准出)

- [ ] Span 层级集成测试通过（见 design.md §Canonical Trace Tree）
- [ ] PEV 循环无 loop defer；`llm.stream.parent == context.pev.llm_call`
- [ ] slog 日志含 traceId；LLM JSONL 含 trace_id
- [ ] `tool_latency` / `compression_ratio` metrics 可 scrape
- [ ] `devrix debug export --session` 输出合法 JSON bundle
- [ ] verify 失败时 span 含 `verify.failure_reason`
- [ ] proposal/design/tasks 与代码状态对齐（已完成项标记 DONE）

---

## Alternatives Considered

| 方案 | 结论 |
|------|------|
| 仅补埋点提覆盖率 | ❌ 代码已基本完成，不解决 AI RCA 核心问题 |
| 只做 Jaeger UI | ❌ AI 需要机器可读 export，非 UI |
| 全面 OTel Logs Bridge | 延后；先用 slog Handler 注入 trace_id 成本低 |
| Baggage 全量接入 | 降级 P2；单进程收益有限 |

---

## Impact Analysis

| 组件 | 变更 | 风险 |
|------|------|------|
| `pev_engine.go` | ctx 传播 + defer 修复 | 中（影响 trace 形态，需集成测试） |
| `observability/logger` | slog bridge | 低 |
| `llm_log.go` | 增加 trace_id 字段 | 低 |
| `bridge.go` | 新 metrics | 低 |
| `cmd/` 或 `coverage/` | incident export CLI | 低 |
