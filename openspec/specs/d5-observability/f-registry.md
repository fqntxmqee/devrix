# D5 Observability Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d5-observability/a-registry.md`

---

## Overview

D5 可观测性域 F 层功能点注册表。每个 F 为 A 层活动编排的最小技术单元。

---

## D5-S1-A01 CreateSpan

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S1-A01-F01 | StartSpan | F-BE | ctx, name, opts | ctx, Span | `tracer/tracer.go` (Start) |
| D5-S1-A01-F02 | ApplySampler | F-BE | trace_id | sampled bool | `tracer/sampler.go` (Sampler.ShouldSample) |
| D5-S1-A01-F03 | WarnUnknownOp | F-BE | operation | — | `tracer/tracer.go` (IsKnown check) |

## D5-S1-A02 EndSpan

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S1-A02-F01 | FinalizeSpan | F-BE | span | — | `tracer/span.go` (End) |
| D5-S1-A02-F02 | ExportSpan | F-BE | span | error | `tracer/export.go` |

## D5-S1-A03 PropagateContext

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S1-A03-F01 | InjectTraceparent | F-BE | ctx, carrier | — | `tracer/propagation.go` (Inject) |
| D5-S1-A03-F02 | ExtractTraceparent | F-BE | carrier | ctx | `tracer/propagation.go` (Extract) |
| D5-S1-A03-F03 | SetBaggage | F-BE | ctx, key, value | ctx | `tracer/baggage.go` (BaggageManager.Set) |
| D5-S1-A03-F04 | PropagateToSubprocess | F-BE | ctx | env_vars | `tracer/propagation_env.go` |

## D5-S1-A04 ShutdownTracer

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S1-A04-F01 | EndAllActiveSpans | F-BE | — | — | `tracer/tracer.go` (Shutdown loop) |
| D5-S1-A04-F02 | FlushExporter | F-BE | ctx | error | `exporter/*.go` (Shutdown) |

## D5-S2-A01 RecordMetric

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S2-A01-F01 | IncCounter | F-BE | name, labels, n | — | `metrics/counter.go` |
| D5-S2-A01-F02 | ObserveHistogram | F-BE | name, value, labels | — | `metrics/histogram.go` |
| D5-S2-A01-F03 | SetGauge | F-BE | name, value, labels | — | `metrics/gauge.go` |
| D5-S2-A01-F04 | ValidateLabels | F-BE | labels | error | `metrics/registry.go` (validateLabels) |

## D5-S2-A02 ExportPrometheus

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S2-A02-F01 | RenderExposition | F-BE | registry | text | `metrics/prometheus.go` (Output) |

## D5-S2-A03 RecordGenAITokens

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S2-A03-F01 | AddTokenByType | F-BE | model, token_type, n | — | `genai_tokens.go` (addGenAITokenUsage) |

## D5-S3-A01 LogRecord

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S3-A01-F01 | EmitJSONLog | F-BE | level, msg, fields | — | `logger/handler.go` (JSONHandler) |
| D5-S3-A01-F02 | RedactSecrets | F-BE | fields | redacted | `logger/redactor.go` |
| D5-S3-A01-F03 | SamplePerSpan | F-BE | span_id, entry | dropped? | `logger/logger.go` (sampler) |

## D5-S3-A02 InstallSlogBridge

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S3-A02-F01 | InjectTraceToSlog | F-BE | ctx | log_entry | `logger/slog_bridge.go` (ContextHandler) |

## D5-S5-A01 RecordOperationHit

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S5-A01-F01 | AtomicIncHit | F-BE | operation | — | `coverage/coverage.go` (RecordHit) |

## D5-S5-A02 AssessCoverage

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S5-A02-F01 | BuildReport | F-BE | ops, include_hits | Report | `coverage/coverage.go` (Report) |
| D5-S5-A02-F02 | ListZeroHits | F-BE | report | []OperationMeta | `coverage/coverage.go` |

## D5-S5-A03 GenerateDailyReport

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S5-A03-F01 | PersistJSON | F-BE | report | error | `coverage/persistence.go` |
| D5-S5-A03-F02 | RunReporterLoop | F-BE | ctx, interval | — | `coverage/reporter.go` (Start) |

## D5-S6-A01 ResolveLayerComponent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S6-A01-F01 | MapOperationPrefix | F-BE | operation | layer, component | `telemetry/names.go` (LayerAndComponent) |

## D5-S6-A02 BuildSpanAttrs

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S6-A02-F01 | MergeLayerAttrs | F-BE | operation, extras | []Attribute | `telemetry/names.go` (SpanAttrs) |

## D5-S8-A01 ExportSessionBundle

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S8-A01-F01 | ReadLLMRounds | F-BE | session_id, log_dir | rounds | `incident/export.go` (readLLMRounds) |
| D5-S8-A01-F02 | AttachCoverageHits | F-BE | session_id | hits | `incident/export.go` |
| D5-S8-A01-F03 | SerializeBundle | F-BE | bundle | JSON | `incident/export.go` |

## D5-S9-A01 RecordRuntimePath

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S9-A01-F01 | IncPathCounter | F-BE | PathKind | — | `runtime/path_resolver.go` (Record) |
| D5-S9-A01-F02 | BridgeToD5Metric | F-BE | PathKind | — | `runtime/d5_metric.go` (IncD5) |

## D5-S0-A01 InitObservability

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S0-A01-F01 | InitTracerProvider | F-BE | tracing_cfg | Tracer | `observability.go` (New) |
| D5-S0-A01-F02 | InitMeterProvider | F-BE | metrics_cfg | Meter | `observability.go` (New) |
| D5-S0-A01-F03 | InitCoverageGlobal | F-BE | AllOperations | — | `observability.go` (coverage.InitGlobal) |

## D5-S0-A03 CreateBridge

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S0-A03-F01 | NewToolBridge | F-BE | bridge | *ToolBridge | `bridge.go` (NewToolBridgeFromBridge) |
| D5-S0-A03-F02 | NewSessionBridge | F-BE | obs | *SessionBridge | `bridge.go` (NewSessionBridge) |
| D5-S0-A03-F03 | InitToolLatency | F-BE | tool, risk, status | *ToolLatencyMetrics | `bridge.go` (InitLatencyMetrics) |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 18 | 39 |
