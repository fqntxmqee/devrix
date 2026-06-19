# D5 Observability Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.0.0
**Last Updated:** 2026-06-19
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d5-observability/a-registry.md` v4.0
**Change:** devrix-d5-v2-terminal（DM-20260619-006 / v2.1 Terminal；增 canonical_s 列；Code Location 校正为 canonical 路径；+诊断 F：DebugFilter/Tracker/Doctor/FaultInject/SessionBridge）

---

## Overview

D5 可观测性域 F 层功能点注册表（Canonical v3.0）。每个 F 为 A 层活动编排的最小技术单元。新增 `canonical_s` 列标注所属 Scenario。

---

## D5-S21-A01 CreateSpan

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S21-A01-F01 | StartSpan | F-BE | ctx, name, opts | ctx, Span | S21 | `instrument/tracer/tracer.go` (Start) |
| D5-S21-A01-F02 | ApplySampler | F-BE | trace_id | sampled bool | S21 | `instrument/tracer/sampler.go` (Sampler.ShouldSample) |
| D5-S21-A01-F03 | WarnUnknownOp | F-BE | operation | — | S21 | `instrument/tracer/tracer.go` (IsKnown check) |

## D5-S21-A02 EndSpan

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S21-A02-F01 | FinalizeSpan | F-BE | span | — | S21 | `instrument/tracer/span.go` (End) |
| D5-S21-A02-F02 | ExportSpan | F-BE | span | error | S21 | `instrument/tracer/export.go` |

## D5-S21-A03 PropagateContext

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S21-A03-F01 | InjectTraceparent | F-BE | ctx, carrier | — | S21 | `instrument/tracer/propagation.go` (Inject) |
| D5-S21-A03-F02 | ExtractTraceparent | F-BE | carrier | ctx | S21 | `instrument/tracer/propagation.go` (Extract) |
| D5-S21-A03-F03 | SetBaggage | F-BE | ctx, key, value | ctx | S21 | `instrument/tracer/baggage.go` (BaggageManager.Set) |
| D5-S21-A03-F04 | PropagateToSubprocess | F-BE | ctx | env_vars | S21 | `instrument/tracer/propagation_env.go` |

## D5-S21-A04 ShutdownTracer

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S21-A04-F01 | EndAllActiveSpans | F-BE | — | — | S21 | `instrument/tracer/tracer.go` (Shutdown loop) |
| D5-S21-A04-F02 | FlushExporter | F-BE | ctx | error | S21 | `export/*.go` (Shutdown) |

## D5-S21-A05 RecordMetric

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S21-A05-F01 | IncCounter | F-BE | name, labels, n | — | S21 | `instrument/metrics/counter.go` |
| D5-S21-A05-F02 | ObserveHistogram | F-BE | name, value, labels | — | S21 | `instrument/metrics/histogram.go` |
| D5-S21-A05-F03 | SetGauge | F-BE | name, value, labels | — | S21 | `instrument/metrics/gauge.go` |
| D5-S21-A05-F04 | ValidateLabels | F-BE | labels | error | S21 | `instrument/metrics/registry.go` (validateLabels) |

## D5-S21-A06 ExportPrometheus

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S21-A06-F01 | RenderExposition | F-BE | registry | text | S21 | `instrument/metrics/prometheus.go` (Output) |

## D5-S21-A07 RecordGenAITokens

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S21-A07-F01 | AddTokenByType | F-BE | model, token_type, n | — | S21 | `instrument/metrics/genai_tokens.go` (addGenAITokenUsage) |

## D5-S21-A08 LogRecord

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S21-A08-F01 | EmitJSONLog | F-BE | level, msg, fields | — | S21 | `instrument/logger/handler.go` (JSONHandler) |
| D5-S21-A08-F02 | RedactSecrets | F-BE | fields | redacted | S21 | `instrument/logger/redactor.go` |
| D5-S21-A08-F03 | SamplePerSpan | F-BE | span_id, entry | dropped? | S21 | `instrument/logger/logger.go` (sampler) |

## D5-S21-A09 InstallSlogBridge

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S21-A09-F01 | InjectTraceToSlog | F-BE | ctx | log_entry | S21 | `instrument/logger/slog_bridge.go` (ContextHandler) |

## D5-S21-A14 FilterDebugLog

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S21-A14-F01 | FilterByCategory | F-BE | categories, level, msg | filtered | S21 | `instrument/logger/debugfilter/filter.go` |
| D5-S21-A14-F02 | PassthroughNonDebug | F-BE | level, msg | passthrough | S21 | `instrument/logger/debugfilter/filter.go` |

## D5-S23-A01 RecordOperationHit

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S23-A01-F01 | AtomicIncHit | F-BE | operation | — | S23 | `diagnose/coverage/coverage.go` (RecordHit) |

## D5-S23-A02 AssessCoverage

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S23-A02-F01 | BuildReport | F-BE | ops, include_hits | Report | S23 | `diagnose/coverage/coverage.go` (Report) |
| D5-S23-A02-F02 | ListZeroHits | F-BE | report | []OperationMeta | S23 | `diagnose/coverage/coverage.go` |

## D5-S23-A03 GenerateDailyReport

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S23-A03-F01 | PersistJSON | F-BE | report | error | S23 | `diagnose/coverage/persistence.go` |
| D5-S23-A03-F02 | RunReporterLoop | F-BE | ctx, interval | — | S23 | `diagnose/coverage/reporter.go` (Start) |

## D5-S23-A04 ExportSessionBundle

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S23-A04-F01 | ReadLLMRounds | F-BE | session_id, log_dir | rounds | S23 | `diagnose/incident/export.go` (readLLMRounds) |
| D5-S23-A04-F02 | AttachCoverageHits | F-BE | session_id | hits | S23 | `diagnose/incident/export.go` |
| D5-S23-A04-F03 | SerializeBundle | F-BE | bundle | JSON | S23 | `diagnose/incident/export.go` |

## D5-S23-A05 RecordLLMPayload

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S23-A05-F01 | WriteLLMLog | F-BE | span, session, payload | — | S23 | `diagnose/incident/llm_log.go` (RecordLLMSpanPayload) |

## D5-S23-A07 RunDiagnosticTracker

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S23-A07-F01 | CollectDiff | F-BE | watch_paths | diff_report | S23 | `diagnose/tracker/tracker.go` |
| D5-S23-A07-F02 | DedupLRU | F-BE | files | deduped | S23 | `diagnose/tracker/tracker.go` |
| D5-S23-A07-F03 | RunLinter | F-BE | file, linter | lint_issues | S23 | `diagnose/tracker/tracker.go` |

## D5-S23-A09 InjectFault

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S23-A09-F01 | ParseFaultEnv | F-BE | env_vars | fault_config | S23 | `diagnose/faultinject/injector.go` |
| D5-S23-A09-F02 | ApplyFault | F-BE | fault_config | — | S23 | `diagnose/faultinject/injector.go` (testbuild only) |
| D5-S23-A09-F03 | NoOpStub | F-BE | — | — | S23 | `diagnose/faultinject/injector_prod.go` |

## D5-S23-A10 RunDoctor

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S23-A10-F01 | CheckLSP | F-BE | — | check_result | S23 | `diagnose/doctor/doctor.go` |
| D5-S23-A10-F02 | CheckEnvironment | F-BE | — | check_result | S23 | `diagnose/doctor/doctor.go` |
| D5-S23-A10-F03 | GenerateHealthReport | F-BE | checks[] | health_report | S23 | `diagnose/doctor/doctor.go` |

## D5-S23-A06 HealthCheck

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S23-A06-F01 | CollectComponentStatus | F-BE | — | status_map | S23 | `health.go` |
| D5-S23-A06-F02 | ExposeCoverageField | F-BE | — | coverage_json | S23 | `health.go`, `observability.go` |

## D5-S21-A11 ResolveLayerComponent

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S21-A11-F01 | MapOperationPrefix | F-BE | operation | layer, component | S21 | `instrument/telemetry/names.go` (LayerAndComponent) |

## D5-S21-A12 BuildSpanAttrs

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S21-A12-F01 | MergeLayerAttrs | F-BE | operation, extras | []Attribute | S21 | `instrument/telemetry/names.go` (SpanAttrs) |

## D5-S24-A03 RecordRuntimePath

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S24-A03-F01 | IncPathCounter | F-BE | PathKind | — | S24 | `configure/runtime/path_resolver.go` (Record) |
| D5-S24-A03-F02 | BridgeToD5Metric | F-BE | PathKind | — | S24 | `configure/runtime/runtime_metric.go` (IncRuntimeMetric) |

> **v2.1 Terminal:** `legacy_harness` metric help text 标 DEPRECATED。退役计划：v2.1 DEPRECATED → v2.3 自爆机制。

## D5-S0-A01 InitObservability

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S0-A01-F01 | InitTracerProvider | F-BE | tracing_cfg | Tracer | S0 | `observability.go` (New) |
| D5-S0-A01-F02 | InitMeterProvider | F-BE | metrics_cfg | Meter | S0 | `observability.go` (New) |
| D5-S0-A01-F03 | InitCoverageGlobal | F-BE | AllOperations | — | S0 | `observability.go` (coverage.InitGlobal) |

## D5-S0-A02 CreateBridge

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S0-A02-F01 | NewToolBridge | F-BE | bridge | *ToolBridge | S0 | `bridge.go` (NewToolBridgeFromBridge) |
| D5-S0-A02-F02 | NewSessionBridge | F-BE | obs | *SessionBridge | S0 | `bridge.go` (NewSessionBridge) |
| D5-S0-A02-F03 | InitToolLatency | F-BE | tool, risk, status | *ToolLatencyMetrics | S0 | `bridge.go` (InitLatencyMetrics) |

## D5-S0-A03 TrackActiveSessions

| F ID | Name | Type | Input | Output | canonical_s | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D5-S0-A03-F01 | IncActiveSessions | F-BE | adapter | — | S0 | `bridge.go` (SessionBridge.Add) |
| D5-S0-A03-F02 | DecActiveSessions | F-BE | adapter | — | S0 | `bridge.go` (SessionBridge.Remove) |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 22 | 51 |

## Revision History

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.0.0 | 2026-06-14 | 初版：18 A、39 F |
| **3.0.0** | **2026-06-19** | **v2.1 Terminal**：增 canonical_s 列（全量填充）；Code Location 全部校正为 canonical 路径（bridge→instrument/export/diagnose/configure）；+诊断 F（S21-A14 FilterDebugLog ×2、S23-A07 Tracker ×3、S23-A09 FaultInject ×3、S23-A10 Doctor ×3、S0-A03 SessionBridge ×2、S23-A06 HealthCheck ×2）；legacy_harness DEPRECATED 标注 |
