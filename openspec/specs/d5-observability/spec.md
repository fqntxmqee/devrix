# D5 Observability Layer Specification

**Capability:** observability
**Domain:** D5
**DSAFT Type:** 公共域 (Common Domain)
**Version:** 2.0.0
**Status:** Canonical — source of truth
**Last Updated:** 2026-06-14
**Layering Spec:** `openspec/specs/architecture/layering.md`

**Archived Changes:** devrix-observability (2026-06-07), devrix-observability-fix (2026-06-07), devrix-observability-coverage (2026-06-08), devrix-harness-bootstrap (2026-06-10), devrix-observability-enhancement P0–P3 (2026-06-10), devrix-queryloop-spans-v1.1

---

## Overview

D5 可观测性域提供 Tracing、Metrics、结构化 Logging、Bridge 集成、Operation 代码染色（Coverage）、Session Incident Export 与运行时路径指标。对齐 OpenTelemetry 语义，通过 `{layer}.{module}.{action}` canonical Operation 名称贯穿 Jaeger/OTLP 与 Prometheus。

**现行主路径（2026-06-14）：** `context.process` → `query.loop.*` → `llm.stream`。**PEV 引擎与 `context.pev.*` span 族已退役**（见 D2-S1 RETIRED）。

| 版本里程碑 | 能力 |
|-----------|------|
| V1.0 | Tracer / Metrics / Logger / Bridge 基础 |
| V1.1 | Gauge/Histogram 修复、Shutdown flush、UpDownCounter |
| V1.2 | Jaeger Service/Operation 命名对齐 |
| V1.3 | Operation Registry + Runtime Hit Counter + Coverage 对账 |
| V1.4 | Harness Bootstrap span + info 事件双写 |
| V1.5–V1.7 | Log-Trace-LLM 关联、`gen_ai.*` 双写、tool_latency/compression metrics、debug export |
| V1.8–V1.9 | W3C Baggage、`cache_read`/`reasoning` token 细分 |
| V2.0 | QueryLoop span 族、Orchestration span、Runtime path metric；PEV 文档退役 |

---

## DSAFT 结构

| 层级 | ID | 名称 | 说明 |
|------|-----|------|------|
| D | D5 | Observability | 公共域，横向 Tracing/Metrics/Logging 能力 |
| S | D5-S1 | Tracer | 分布式追踪、W3C 传播、Baggage |
| S | D5-S2 | Metrics | Counter/Histogram/Gauge、Prometheus 导出 |
| S | D5-S3 | Logger | 结构化日志、slog 桥接、采样与脱敏 |
| S | D5-S4 | Exporter | Console / OTLP / Memory span 导出 |
| S | D5-S5 | Coverage | Operation 注册表、Runtime Hit、日报 |
| S | D5-S6 | Telemetry | Operation 常量、`SpanAttrs`、Layer/Component 映射 |
| S | D5-S7 | Settings | Tracing/Metrics 配置 schema |
| S | D5-S8 | Incident | Session incident bundle 导出 |
| S | D5-S9 | Runtime | QueryLoop vs LegacyHarness 路径计数 |

---

## Scenarios

| ID | Scenario | Responsibility | Status |
|----|----------|----------------|--------|
| D5-S1 | Tracer | Span 生命周期、采样、Shutdown flush、W3C/Baggage 传播 | IMPLEMENTED |
| D5-S2 | Metrics | 指标注册、Label 白名单、Prometheus `/metrics` | IMPLEMENTED |
| D5-S3 | Logger | JSON/Text 日志、trace 注入、error stack、per-span 采样 | IMPLEMENTED |
| D5-S4 | Exporter | Console/OTLP/Memory SpanExporter | IMPLEMENTED |
| D5-S5 | Coverage | 56 Operation 注册表、Runtime Hit、Health 对账 | IMPLEMENTED |
| D5-S6 | Telemetry | `telemetry/names.go` canonical Operation + `LayerAndComponent` | IMPLEMENTED |
| D5-S7 | Settings | `settings/` Tracing/Metrics 配置 | IMPLEMENTED |
| D5-S8 | Incident | `debug export` schema v1 bundle | IMPLEMENTED |
| D5-S9 | Runtime | `runtime_path_resolved_total{path}` 路径分流指标 | IMPLEMENTED |

---

## Architecture

```
D1 Gateway ──→ root span (gateway.message.receive)
                    │
D2 ContextEngine ──→ context.process
                    ├─→ query.loop.run → query.loop.turn → query.loop.llm.call
                    ├─→ context.compression.run (+ step spans)
                    ├─→ context.harness.* (legacy path, harness.enabled)
                    └─→ tool.execute.*
                    │
D3 LLMGateway ─────→ llm.stream → llm.adapter.stream
                    │
D4 MultiAgent ─────→ agent.run / agent.tool.call / agent.fork|join
                    │
D7 Orchestration ──→ orchestration.wave.* / orchestration.flow.*
                    │
                    ▼
         ┌──────────────────────────────────────┐
         │  Observability Facade (D5)             │
         │  Tracer │ Metrics │ Logger │ Coverage │
         │  Bridge │ LLMLog │ Incident │ Runtime │
         └──────────────────────────────────────┘
                    │
                    ▼
         Console / OTLP / Prometheus / ~/.devrix/coverage/
```

## Cross-Domain Dependencies

| Domain | 依赖内容 | 使用位置 |
|--------|---------|---------|
| D1 Communication | 入站创建 root span、`SessionBridge.ActiveSessions` | `gateway/`, `adapters/` |
| D2 Context Engine | `observability.Bridge` tracer/meter/logger | `engine.go`, `query/loop.go`, `harness/` |
| D3 LLM Gateway | `llm.stream` span、`RecordGenAITokenUsage` | `gateway/gateway.go`, `adapter/` |
| D4 Multi-Agent | `agent.*` spans、Fork policy metrics sink | `multiagent/observability/` |
| D7 Orchestration | `orchestration.*` spans | `orchestration/flow/`, `wave/` |
| Shared | `config`, `types` | 全子包 |

## Package Map

| 子包 / 根文件 | 场景 | 职责 |
|--------------|------|------|
| `observability.go`, `bridge.go` | D5-S1~S3 | Facade、`Bridge`/`ToolBridge`/`SessionBridge` |
| `tracer/` | D5-S1 | TracerProvider、Span、采样、W3C Propagator、Baggage |
| `metrics/` | D5-S2 | Meter、Counter/Histogram/Gauge、Prometheus 导出 |
| `logger/` | D5-S3 | StructuredLogger、slog 桥接、脱敏、采样 |
| `exporter/` | D5-S4 | Console、OTLP、Memory、Null exporter |
| `coverage/` | D5-S5 | Registry、Counter、Reporter、Persistence、CLI |
| `telemetry/` | D5-S6 | `Op*` 常量、`SpanAttrs`、`LayerAndComponent`、`GenAI*` attrs |
| `settings/` | D5-S7 | TracingConfig、MetricsConfig、OTLP 配置 |
| `incident/` | D5-S8 | `BuildBundle` schema v1 |
| `runtime/` | D5-S9 | PathResolver、`runtime_path_resolved_total` D5 bridge |
| `llm_log.go`, `genai_tokens.go` | D5-S2/S3 | LLM JSONL、gen_ai token metrics |
| `health.go`, `config.go`, `load.go` | — | HealthCheck、配置加载 |

---

## Key Design Patterns

1. **Canonical Operation Naming**: Span name = Jaeger Operation = `{layer}.{module}.{action}`，常量于 `telemetry/names.go`，注册于 `coverage/registry.go`（当前 **56** 条）。
2. **Coverage 独立于采样**: `Tracer.Start` 无条件 `RecordHit`，`always_off` 采样仍计数。
3. **Bridge 零侵入集成**: 各域注入 `*observability.Bridge`，`IsEnabled()` 守卫避免 nil 开销。
4. **Metrics 前缀**: Meter name `devrix` → Prometheus 名 `devrix_{instrument}`（如 `devrix_tool_latency`）。
5. **Log-Trace-LLM 三联**: slog `traceId`/`spanId` + LLM JSONL `trace_id`/`span_id` + span `gen_ai.*` 双写。
6. **Graceful Degradation**: `NewNoOp()` / nil Bridge 时业务路径不受影响；observability 故障标记 `degraded` 不阻断 Process。

---

## Requirements

### Requirement: Operation Registry (P0)

系统 MUST 维护 canonical Operation 静态注册表（`coverage/registry.go`），与 `telemetry/names.go` `Op*` 常量全集一致。

**T:** D5-S5-A01-T01

#### Scenario: Registry 与 names.go 对账

- GIVEN `coverage.AllOperations()` 被调用
- WHEN 与 `registry_test` expected 列表对比
- THEN 两者 MUST 完全一致（当前 56 条）
- AND 每个 entry 的 `layer`/`component` 与 `LayerAndComponent(operation)` 一致

#### Scenario: 未知 Operation 启动 Span

- GIVEN Tracer 收到不在 Registry 中的 operation 名
- WHEN `Start` 被调用
- THEN 系统 MUST 仍创建 span（向后兼容）
- AND 记录 WARN 日志 `unknown operation`

---

### Requirement: Runtime Operation Hit Counter (P0)

`Tracer.Start` MUST 无条件递增对应 Operation 的进程内命中计数，不受 trace 采样影响。

**T:** D5-S5-A01-T02, D5-S5-A01-T03

#### Scenario: 采样关闭仍计数

- GIVEN `tracing.sampling.type = always_off`
- WHEN instrumented operation 调用 `Tracer.Start`
- THEN 命中计数 MUST 递增
- AND 无 span 导出到 exporter

#### Scenario: 计数并发安全

- GIVEN 100 个并发 goroutine 对同一 operation 调用 `Start`
- THEN 最终命中计数 MUST 等于 100

---

### Requirement: Coverage Health Summary (P0)

`HealthCheck()` MUST 暴露 `coverage` 对象：`operations_total`、`operations_hit`、`coverage_ratio`、`zero_hit_count`。

**T:** D5-S5-A01-T02

---

### Requirement: Jaeger Service Identity (P0)

OTLP Resource MUST 包含 `service.name`（默认 `devrix`）与 `service.version`。

---

### Requirement: Devrix Layer Attributes (P0)

每个 span MUST 包含 `devrix.layer` 与 `devrix.component`，由 `telemetry.SpanAttrs` 注入。

| Layer | Components |
|-------|------------|
| `communication` | `gateway`, `adapter` |
| `context` | `context_engine`, `harness`, `query_loop`, `tool_runner`, `plan_agent`, `plan_mode`, `task_manager` |
| `llm` | `llm_gateway`, `llm_adapter` |
| `agent` | `agent_tool` |
| `orchestration` | `orchestrator` |

---

### Requirement: QueryLoop Span Hierarchy (P0)

QueryLoop 主路径 MUST 形成以下层级（PEV 已退役）：

```
gateway.message.receive [SERVER]
└── context.process [INTERNAL]
    └── query.loop.run [INTERNAL]
        └── query.loop.turn [INTERNAL]
            ├── query.loop.llm.call [CLIENT]
            │   └── llm.stream [CLIENT]
            └── tool.execute.single [INTERNAL]
```

**T:** D5-S4-A01-T02

---

### Requirement: Harness Bootstrap Spans (P0, 条件触发)

当 `harness.enabled=true` 时，`context.harness.*` 与 `context.system_prompt.build` span MUST 作为 `context.process` 子 span 创建。

**T:** D5-S5-A01-T02 (integration `context_harness_obs_test.go`)

---

### Requirement: Log-Trace-LLM Correlation (P0)

slog 与 LLM JSONL MUST 携带 `trace_id`/`span_id`，与 Jaeger span 可交叉引用。

**T:** D5-S3-A01-T04 (slog bridge), D5-S8-A01-T01 (LLM JSONL in bundle)

---

### Requirement: GenAI Semantic Attributes (P0)

LLM call spans MUST 双写 OTel `gen_ai.*` 属性（与 `llm.*` 并存）。

**T:** D5-S2-A01-T08

---

### Requirement: Tool Latency & Compression Metrics (P1)

- `devrix_tool_latency` Histogram: labels `tool`, `risk_level`, `status`
- `devrix_compression_ratio` Histogram: 压缩成功后 observe ratio
- `context.compression.run` span: `compression.trigger_reason`, `compression.ratio`

**T:** D5-S2-A01-T03, D5-S2-A01-T09

---

### Requirement: GenAI Token Usage Metrics (P2)

`devrix_gen_ai.client.token.usage` Counter: labels `token_type`（`input`/`output`/`cache_read`/`reasoning`）, `model`。

**T:** D5-S2-A01-T08

---

### Requirement: Session Incident Export (P1)

主二进制 MUST 支持 `devrix debug export --session <id>`，输出 schema v1 JSON（`llm_rounds`, 可选 `trace`, `coverage_hits`）。

**T:** D5-S8-A01-T01, D5-S8-A01-T02

---

### Requirement: W3C Baggage Propagation (P2)

Gateway 入站 MUST 写入 baggage `session.id`；`user.id` 可用时写入。CLI 子进程通过 `TRACEPARENT` + `BAGGAGE` 环境变量继承。

**T:** D5-S1-A01-T03

---

### Requirement: Tracer Shutdown Flush (P0)

`TracerProvider.Shutdown` MUST 结束所有 active spans 并刷写 exporter。

**T:** D5-S1-A01-T01

---

### Requirement: Metrics Correctness (P0)

- Gauge `Set`/`Add`/`Sub`/`Inc`/`Dec` 精确（mutex 保护）
- Histogram 仅递增第一个匹配桶
- `Int64UpDownCounter` 返回 Gauge 语义

**T:** D5-S2-A01-T03, D5-S2-A01-T04, D5-S2-A01-T05

---

### Requirement: Logger Semantics (P1)

- Error 日志 MUST 附加 `stack` 字段
- `max_entries_per_span` 采样超限 MUST 发 WARN

**T:** D5-S3-A01-T02, D5-S3-A01-T03, D5-S3-A01-T04

---

### Requirement: Runtime Path Metric (P1)

`devrix_runtime_path_resolved_total{path="query_loop|legacy_harness"}` MUST 与 in-process `PathResolver` 同步。

**T:** D5-S9-A01-T01

---

## RETIRED

| Item | 退役日期 | 替代 |
|------|----------|------|
| `context.pev.*` operation 族 | 2026-06-13 | `query.loop.*` |
| PEV Span Hierarchy requirements | 2026-06-13 | QueryLoop Span Hierarchy |
| `context.plan.generate` / `context.milestone.run` (旧 PEV plan) | 2026-06-13 | `task.plan.*` / `task.manager.*` |

---

## Registries

- **A 层**: `a-registry.md` — 18 Activities
- **F 层**: `f-registry.md` — 39 Function Points
- **T 层**: `t-registry.md` — Test Points

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0–1.9.0 | 2026-06-07–10 | V1 基线 → Baggage + token breakdown |
| 2.0.0 | 2026-06-14 | QueryLoop/Orchestration span 族、PEV 退役、全文档 DSAFT 对齐 |
