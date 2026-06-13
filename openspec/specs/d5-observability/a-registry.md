# D5 Observability Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D5 可观测性域 A 层活动注册表。每个 Activity 为调用方可发起的原子可观测动作。

---

## D5-S1: Tracer

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S1-A01 | CreateSpan | A-BE | operation, parent_ctx, opts | span, ctx | span.started | `tracer/tracer.go` (Tracer.Start) |
| D5-S1-A02 | EndSpan | A-BE | span | — | span.ended | `tracer/span.go` (Span.End) |
| D5-S1-A03 | PropagateContext | A-BE | ctx, carrier | injected_carrier | — | `tracer/propagation.go`, `tracer/baggage.go` |
| D5-S1-A04 | ShutdownTracer | A-BE | ctx | error | spans.flushed | `tracer/tracer.go` (TracerProvider.Shutdown) |

## D5-S2: Metrics

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S2-A01 | RecordMetric | A-BE | instrument, value, labels | — | metric.recorded | `metrics/meter.go` |
| D5-S2-A02 | ExportPrometheus | A-BE | — | exposition_text | — | `metrics/prometheus.go` (Handler) |
| D5-S2-A03 | RecordGenAITokens | A-BE | model, usage_breakdown | — | counter.inc | `genai_tokens.go` (RecordGenAITokenUsage) |

## D5-S3: Logger

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S3-A01 | LogRecord | A-BE | level, message, fields | — | log.emitted | `logger/logger.go` |
| D5-S3-A02 | InstallSlogBridge | A-BE | — | — | slog.trace_injection | `logger/slog_bridge.go`, `slog_bridge.go` |
| D5-S3-A03 | ShutdownLogger | A-BE | — | error | sampler.reset | `logger/logger.go` (Close) |

## D5-S4: Exporter

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S4-A01 | ExportSpans | A-BE | span_batch | error | spans.exported | `exporter/console.go`, `exporter/otlp.go`, `exporter/memory.go` |
| D5-S4-A02 | CreateExporter | A-BE | tracing_config | SpanExporter | — | `exporter/factory.go` (NewTracingExporter) |

## D5-S5: Coverage

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S5-A01 | RecordOperationHit | A-BE | operation_name | — | hit.inc | `coverage/coverage.go` (RecordHit), `tracer/tracer.go` |
| D5-S5-A02 | AssessCoverage | A-BE | operations[] | coverage_report | — | `coverage/coverage.go` (Counter.Report) |
| D5-S5-A03 | GenerateDailyReport | A-BE | — | daily_report | report.persisted | `coverage/reporter.go` (GenerateNow) |

## D5-S6: Telemetry

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S6-A01 | ResolveLayerComponent | A-BE | operation | layer, component | — | `telemetry/names.go` (LayerAndComponent) |
| D5-S6-A02 | BuildSpanAttrs | A-BE | operation, extras | []Attribute | — | `telemetry/names.go` (SpanAttrs) |
| D5-S6-A03 | BuildGenAIAttrs | A-BE | model, tokens, session | []Attribute | — | `telemetry/names.go` (GenAIUsageAttrs) |

## D5-S7: Settings

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S7-A01 | LoadObsConfig | A-BE | yaml/env | *Config | — | `config.go`, `load.go`, `settings/config.go` |
| D5-S7-A02 | ValidateObsConfig | A-BE | *Config | error | — | `config.go` (Validate) |

## D5-S8: Incident

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S8-A01 | ExportSessionBundle | A-BE | session_id, opts | JSON bundle | — | `incident/export.go` (BuildBundle) |
| D5-S8-A02 | RecordLLMPayload | A-BE | span, session, payload | — | jsonl.written | `llm_log.go` (RecordLLMSpanPayload) |

## D5-S9: Runtime

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S9-A01 | RecordRuntimePath | A-BE | path_kind | — | path_counter.inc | `runtime/path_resolver.go` (Record), `runtime/d5_metric.go` (IncD5) |
| D5-S9-A02 | RegisterRuntimeMetric | A-BE | meter | error | metric.registered | `runtime/d5_metric.go` (RegisterD5) |

## Root Facade (跨场景)

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S0-A01 | InitObservability | A-BE | *Config | *Observability | obs.initialized | `observability.go` (New) |
| D5-S0-A02 | HealthCheck | A-BE | — | status_map | — | `health.go`, `observability.go` (HealthCheck) |
| D5-S0-A03 | CreateBridge | A-BE | *Observability | *Bridge | — | `bridge.go` (NewBridge) |

---

## Statistics

| Scenarios | Activities |
|-----------|------------|
| 9 (+3 root) | 24 |
