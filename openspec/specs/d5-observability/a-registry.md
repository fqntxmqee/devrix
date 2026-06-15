# D5 Observability Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.0.0
**Last Updated:** 2026-06-15
**Parent:** `openspec/specs/architecture/layering.md`
**Change:** devrix-d5-sa-refine（DM-20260615-001 / v1.0 Canonical 重排；4+1 价值流 S 层）

---

## Overview

D5 可观测性域 A 层活动注册表（Canonical v3.0）。S 层从 9 技术模块重切为 4+1 价值流。

---

## D5-S21: Instrument（遥测生成）

**承诺 C1：** 为任意操作生成遥测数据（Span + Metric + Log + 属性构建）

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D5-S21-A01 | CreateSpan | A-BE | operation, parent_ctx, opts | span, ctx | span.started | `tracer/tracer.go` (Tracer.Start) | S1-A01 |
| D5-S21-A02 | EndSpan | A-BE | span | -- | span.ended | `tracer/span.go` (Span.End) | S1-A02 |
| D5-S21-A03 | PropagateContext | A-BE | ctx, carrier | injected_carrier | -- | `tracer/propagation.go`, `tracer/baggage.go` | S1-A03 |
| D5-S21-A04 | ShutdownTracer | A-BE | ctx | error | spans.flushed | `tracer/tracer.go` (TracerProvider.Shutdown) | S1-A04 |
| D5-S21-A05 | RecordMetric | A-BE | instrument, value, labels | -- | metric.recorded | `metrics/meter.go` | S2-A01 |
| D5-S21-A06 | ExportPrometheus | A-BE | -- | exposition_text | -- | `metrics/prometheus.go` (Handler) | S2-A02 |
| D5-S21-A07 | RecordGenAITokens | A-BE | model, usage_breakdown | -- | counter.inc | `genai_tokens.go` (RecordGenAITokenUsage) | S2-A03 |
| D5-S21-A08 | LogRecord | A-BE | level, message, fields | -- | log.emitted | `logger/logger.go` | S3-A01 |
| D5-S21-A09 | InstallSlogBridge | A-BE | -- | -- | slog.trace_injection | `logger/slog_bridge.go`, `slog_bridge.go` | S3-A02 |
| D5-S21-A10 | ShutdownLogger | A-BE | -- | error | sampler.reset | `logger/logger.go` (Close) | S3-A03 |
| D5-S21-A11 | ResolveLayerComponent | A-BE | operation | layer, component | -- | `telemetry/names.go` (LayerAndComponent) | S6-A01 |
| D5-S21-A12 | BuildSpanAttrs | A-BE | operation, extras | []Attribute | -- | `telemetry/names.go` (SpanAttrs) | S6-A02 |
| D5-S21-A13 | BuildGenAIAttrs | A-BE | model, tokens, session | []Attribute | -- | `telemetry/names.go` (GenAIUsageAttrs) | S6-A03 |

## D5-S22: Export（遥测导出）

**承诺 C2：** 将遥测数据导出到外部系统（OTLP/Prometheus/Console）

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D5-S22-A01 | ExportSpans | A-BE | span_batch | error | spans.exported | `exporter/console.go`, `exporter/otlp.go`, `exporter/memory.go` | S4-A01 |
| D5-S22-A02 | CreateExporter | A-BE | tracing_config | SpanExporter | -- | `exporter/factory.go` (NewTracingExporter) | S4-A02 |

## D5-S23: Diagnose（诊断辅助）

**承诺 C3：** 提供诊断辅助（Coverage 报告 + Incident 导出 + Health Check）

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D5-S23-A01 | RecordOperationHit | A-BE | operation_name | -- | hit.inc | `coverage/coverage.go` (RecordHit), `tracer/tracer.go` | S5-A01 |
| D5-S23-A02 | AssessCoverage | A-BE | operations[] | coverage_report | -- | `coverage/coverage.go` (Counter.Report) | S5-A02 |
| D5-S23-A03 | GenerateDailyReport | A-BE | -- | daily_report | report.persisted | `coverage/reporter.go` (GenerateNow) | S5-A03 |
| D5-S23-A04 | ExportSessionBundle | A-BE | session_id, opts | JSON bundle | -- | `incident/export.go` (BuildBundle) | S8-A01 |
| D5-S23-A05 | RecordLLMPayload | A-BE | span, session, payload | -- | jsonl.written | `llm_log.go` (RecordLLMSpanPayload) | S8-A02 |
| D5-S23-A06 | HealthCheck | A-BE | -- | status_map | -- | `health.go`, `observability.go` (HealthCheck) | S0-A02 |

## D5-S24: Configure（配置与运行时管理）

**承诺 C4：** 加载/校验配置 + 运行时路径计数

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D5-S24-A01 | LoadObsConfig | A-BE | yaml/env | *Config | -- | `config.go`, `load.go`, `settings/config.go` | S7-A01 |
| D5-S24-A02 | ValidateObsConfig | A-BE | *Config | error | -- | `config.go` (Validate) | S7-A02 |
| D5-S24-A03 | RecordRuntimePath | A-BE | path_kind | -- | path_counter.inc | `runtime/path_resolver.go` (Record), `runtime/runtime_metric.go` (IncRuntimeMetric) | S9-A01 |
| D5-S24-A04 | RegisterRuntimeMetric | A-BE | meter | error | metric.registered | `runtime/runtime_metric.go` (RegisterRuntimeMetric) | S9-A02 |

## D5-S0: Facade（横切）

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D5-S0-A01 | InitObservability | A-BE | *Config | *Observability | obs.initialized | `observability.go` (New) | S0-A01 |
| D5-S0-A02 | CreateBridge | A-BE | *Observability | *Bridge | -- | `bridge.go` (NewBridge) | S0-A03 |

---

## Legacy Module Index（D5-S1–S9，冻结追溯）

| Legacy S | Module | Canonical S | Scenario |
|----------|--------|-------------|----------|
| D5-S1 | Tracer | S21 | Instrument |
| D5-S2 | Metrics | S21 | Instrument |
| D5-S3 | Logger | S21 | Instrument |
| D5-S4 | Exporter | S22 | Export |
| D5-S5 | Coverage | S23 | Diagnose |
| D5-S6 | Telemetry | S21 | Instrument |
| D5-S7 | Settings | S24 | Configure |
| D5-S8 | Incident | S23 | Diagnose |
| D5-S9 | Runtime | S24 | Configure |

---

## Statistics

| Scenarios | Activities | IMPLEMENTED |
|-----------|------------|-------------|
| 4 (+2 root facade) | 27 | 27 |

## Revision History

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.0.0 | 2026-06-14 | 初版：9+1 技术模块 S 层 |
| **3.0.0** | 2026-06-15 | **SA Refine v1.0**：Canonical 重排为 4+1 价值流 S21–S24；增 Legacy 列 + Module Index |
