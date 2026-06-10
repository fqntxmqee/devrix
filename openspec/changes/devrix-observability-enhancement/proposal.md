# Proposal: Devrix 可观察层增强

**Change ID:** devrix-observability-enhancement
**Demand ID:** DM-20260610-001
**Layer:** L5 - Observability (D2-S12)
**Type:** Enhancement
**Status:** Draft
**Based on:** `docs/observability-design.md`, `openspec/l5-registry.md`

---

## Problem Statement

Devrix 可观察层已具备 **Tracer / Metrics / Logger / Coverage** 四柱基础能力，但通过深度分析发现以下缺口：

### 问题 1: LLM 请求/响应未在 span events 中记录

| 痛点 | 现状 | 期望 |
|------|------|------|
| LLM 输入输出不可见 | `AddLLMRequestEvent()` 定义但从未调用 | Jaeger span events 可见完整 LLM 请求/响应 |
| PEV 迭代日志缺失 | 无独立 iteration span | 每轮迭代有独立 span + metrics |
| 工具执行无独立 trace | 工具执行混入 PEV span | 工具执行有独立 span + latency metrics |

**根因**: `contextengine/llm_logger.go` 定义了 `AddLLMRequestEvent/ResponseEvent`，但：
1. LLM Gateway 层未调用
2. PEV Engine 未传递完整请求信息

### 问题 2: 部分关键路径缺失埋点

| 操作 | Layer | 影响 |
|------|-------|------|
| `context.compression.run` | context_engine | 无法分析压缩触发率 |
| `context.longterm.recall` | context_engine | 无法分析记忆召回命中率 |
| `context.pev.iteration` | pev_engine | 无法分析迭代次数分布 |
| `context.pev.synthesis` | pev_engine | 无法分析工具结果合成 |
| `tool_latency` (metrics) | - | 无法分析工具执行延迟 |

**覆盖率评估**: ~22/44 ≈ 50%

### 问题 3: Baggage 未充分利用

已实现 `BaggageManager` 但仅少量使用，无法传递关键业务上下文（如 session_id、user_intent）。

### 问题 4: Metrics 未完整注册

部分定义的 metrics 未注册到 registry：
- `tool_latency` - 工具执行延迟
- `compression_ratio` - 压缩率
- `session_duration` - 会话时长

---

## Proposed Solution

### 方案 1: LLM 日志完整接入

在 PEV Engine 的 LLM 调用处调用 `AddLLMRequestEvent/ResponseEvent`：

```go
// pev_engine.go
ctx, span := e.startSpan(ctx, telemetry.OpContextPEVLLMCall, ...)
AddLLMRequestEvent(span, sc.SessionID, iter, sc.Model, req)
```

### 方案 2: 缺失埋点补充

补充 9 个缺失的 span operation + 3 个 metrics：

| 新增 Operation | Layer | Component | 说明 |
|----------------|-------|------------|------|
| `context.compression.run` | context | context_engine | 压缩执行 |
| `context.longterm.recall` | context | context_engine | 记忆召回 |
| `context.longterm.store` | context | context_engine | 记忆存储 |
| `context.pev.iteration` | context | pev_engine | 迭代开始 |
| `context.pev.synthesis` | context | pev_engine | 工具合成 |
| `context.milestone.run` | context | pev_engine | 里程碑执行 |
| `context.plan.generate` | context | context_engine | 计划生成 |
| `llm.adapter.stream` | llm | llm_adapter | 适配器调用 |
| `adapter.feishu.outbound` | communication | adapter | 飞书出站 |

| 新增 Metrics | Type | Labels | 说明 |
|-------------|------|--------|------|
| `tool_latency` | Histogram | tool, risk_level | 工具执行延迟 |
| `compression_ratio` | Histogram | - | 压缩率分布 |
| `session_active_count` | Gauge | adapter | 活跃会话数 |

### 方案 3: Baggage 增强使用

在 ContextEngine 中传递关键上下文：

```go
ctx = baggageManager.Set(ctx, "session_id", sc.SessionID)
ctx = baggageManager.Set(ctx, "model", sc.Model)
```

---

## Capabilities

| Capability | L5 ID | 说明 |
|------------|--------|------|
| llm-trace-complete | L5-OBS-TRACE-01 | 完整 LLM 输入输出 trace |
| pev-iteration-trace | L5-OBS-TRACE-02 | PEV 迭代独立 span |
| tool-latency-metrics | L5-OBS-METRICS-01 | 工具延迟指标 |
| baggage-context-propagation | L5-OBS-TRACE-03 | Baggage 传递业务上下文 |
| coverage-completion | L5-OBS-COVERAGE-01 | 埋点覆盖率 ≥80% |

---

## Scope

### In Scope
- `contextengine/pev_engine.go` - LLM 日志调用
- `contextengine/engine.go` - 缺失埋点补充
- `coverage/registry.go` - 新增 operation 注册
- `observability/bridge.go` - 新增 metrics 注册
- `llmgateway/gateway/gateway.go` - LLM adapter span

### Out of Scope
- 新增 exporter（已有 OTLP/Prometheus）
- 长期数据存储方案
- 前端可视化

---

## Impact Analysis

| 组件 | 变更 | 风险 |
|------|------|------|
| `contextengine/pev_engine.go` | 新增 2 个 span events 调用 | 低 |
| `contextengine/engine.go` | 新增 5 个 span 创建 | 低 |
| `coverage/registry.go` | 新增 9 个 operation | 低 |
| `observability/bridge.go` | 新增 3 个 metrics | 低 |
| Jaeger | 无变更 | - |

---

## Success Criteria (S3 准出)

- [ ] `docs/observability-design.md` 中所有 P0 建议落地为设计文档
- [ ] 埋点覆盖率从 50% 提升至 ≥80%
- [ ] 每个新 operation 有对应单元测试
- [ ] LLM 日志可通过 Jaeger 查看完整请求/响应
