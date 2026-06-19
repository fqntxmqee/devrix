# D5 Observability Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Draft — devrix-d5-v2-terminal（S3）
**Version:** 4.0.0
**Last Updated:** 2026-06-19
**Parent:** `openspec/specs/architecture/layering.md`
**Change:** devrix-d5-v2-terminal（DM-20260619-006 / v2.1 路径同步 + S23 扩展 + S21-A14 + S0-A03）

> S7 归档后替换 `openspec/specs/d5-observability/a-registry.md`。

---

## Overview

D5 A 层终态登记（Canonical v4.0）。Code Location 对齐 v2.0 物理路径；补登诊断工具 A；DebugFilter→S21；SessionBridge→S0。

---

## D5-S21: Instrument（遥测生成）

**承诺 C1** · **博弈角色:** Signal Producer

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D5-S21-A01 | CreateSpan | A-BE | operation, parent_ctx, opts | span, ctx | span.started | `instrument/tracer/tracer.go` (Start) | S1-A01 |
| D5-S21-A02 | EndSpan | A-BE | span | — | span.ended | `instrument/tracer/span.go` (End) | S1-A02 |
| D5-S21-A03 | PropagateContext | A-BE | ctx, carrier | injected_carrier | — | `instrument/tracer/propagation.go`, `baggage.go` | S1-A03 |
| D5-S21-A04 | ShutdownTracer | A-BE | ctx | error | spans.flushed | `instrument/tracer/tracer.go` (Shutdown) | S1-A04 |
| D5-S21-A05 | RecordMetric | A-BE | instrument, value, labels | — | metric.recorded | `instrument/metrics/meter.go` | S2-A01 |
| D5-S21-A06 | ExportPrometheus | A-BE | — | exposition_text | — | `instrument/metrics/prometheus.go` | S2-A02 |
| D5-S21-A07 | RecordGenAITokens | A-BE | model, usage_breakdown | — | counter.inc | `instrument/metrics/genai_tokens.go` | S2-A03 |
| D5-S21-A08 | LogRecord | A-BE | level, message, fields | — | log.emitted | `instrument/logger/logger.go` | S3-A01 |
| D5-S21-A09 | InstallSlogBridge | A-BE | — | — | slog.trace_injection | `instrument/logger/slog_bridge.go` | S3-A02 |
| D5-S21-A10 | ShutdownLogger | A-BE | — | error | sampler.reset | `instrument/logger/logger.go` (Close) | S3-A03 |
| D5-S21-A11 | ResolveLayerComponent | A-BE | operation | layer, component | — | `instrument/telemetry/names.go` | S6-A01 |
| D5-S21-A12 | BuildSpanAttrs | A-BE | operation, extras | []Attribute | — | `instrument/telemetry/names.go` (SpanAttrs) | S6-A02 |
| D5-S21-A13 | BuildGenAIAttrs | A-BE | model, tokens, session | []Attribute | — | `instrument/telemetry/names.go` (GenAIUsageAttrs) | S6-A03 |
| **D5-S21-A14** | **FilterDebugLog** | A-BE | level, category, fields | pass/drop | log.filtered | `instrument/logger/debugfilter/filter.go` | — (T: S23-A08-T*) |

---

## D5-S22: Export（遥测导出）

**承诺 C2** · **博弈角色:** Signal Shipper

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D5-S22-A01 | ExportSpans | A-BE | span_batch | error | spans.exported | `export/console.go`, `otlp.go`, `memory.go` | S4-A01 |
| D5-S22-A02 | CreateExporter | A-BE | tracing_config | SpanExporter | — | `export/factory.go` | S4-A02 |

---

## D5-S23: Diagnose（诊断辅助）

**承诺 C3** · **博弈角色:** Auditor + Evidence Clerk

**子承诺:** C3a Coverage · C3b Incident · C3c Health/Doctor · C3d Tracker · C3e FaultInject

| A ID | Name | 子承诺 | Type | Input | Output | Code Location | Legacy / T 锚点 |
|------|------|--------|------|-------|--------|---------------|-----------------|
| D5-S23-A01 | RecordOperationHit | C3a | A-BE | operation_name | — | `diagnose/coverage/coverage.go`, `instrument/tracer/tracer.go` | S5-A01 |
| D5-S23-A02 | AssessCoverage | C3a | A-BE | operations[] | coverage_report | `diagnose/coverage/coverage.go` | S5-A02 |
| D5-S23-A03 | GenerateDailyReport | C3a | A-BE | — | daily_report | `diagnose/coverage/reporter.go` | S5-A03 |
| D5-S23-A04 | ExportSessionBundle | C3b | A-BE | session_id, opts | JSON bundle | `diagnose/incident/export.go` | S8-A01 |
| D5-S23-A05 | RecordLLMPayload | C3b | A-BE | span, session, payload | — | `diagnose/incident/llm_log.go` | S8-A02 |
| D5-S23-A06 | HealthCheck | C3c | A-BE | — | status_map | `health.go`, `observability.go` | S0-A02 |
| **D5-S23-A07** | **TrackFileDiagnostics** | C3d | A-BE | file_path, edit_event | diagnostics | `diagnose/tracker/tracker.go` | A07-T* |
| D5-S23-A08 | ~~FilterDebugLog~~ | — | — | — | — | **RETIRED → S21-A14** | A08-T* |
| **D5-S23-A09** | **InjectFault** | C3e | A-BE | hook_name, env | error/none | `diagnose/faultinject/injector.go` | A09-T* |
| **D5-S23-A10** | **RunDoctorChecks** | C3c | A-BE | — | doctor_report | `diagnose/doctor/doctor.go` | **canonical for A03-T01/T02** |

---

## D5-S24: Configure（配置与运行时管理）

**承诺 C4** · **博弈角色:** Rule Setter

| A ID | Name | Type | Input | Output | Code Location | Legacy |
|------|------|------|-------|--------|---------------|--------|
| D5-S24-A01 | LoadObsConfig | A-BE | yaml/env | *Config | `config.go`, `load.go`, `configure/settings/config.go` | S7-A01 |
| D5-S24-A02 | ValidateObsConfig | A-BE | *Config | error | `config.go` (Validate) | S7-A02 |
| D5-S24-A03 | RecordRuntimePath | A-BE | path_kind | — | `configure/runtime/path_resolver.go` | S9-A01 |
| D5-S24-A04 | RegisterRuntimeMetric | A-BE | meter | error | `configure/runtime/runtime_metric.go` | S9-A02 |

---

## D5-S0: Facade（横切）

**博弈角色:** Integration Shell（非独立子博弈场）

| A ID | Name | Type | Input | Output | Code Location | Legacy |
|------|------|------|-------|--------|---------------|--------|
| D5-S0-A01 | InitObservability | A-BE | *Config | *Observability | `observability.go` (New) | S0-A01 |
| D5-S0-A02 | CreateBridge | A-BE | *Observability | *Bridge | `bridge.go` (NewBridge) | S0-A03 |
| **D5-S0-A03** | **TrackActiveSessions** | A-BE | adapter, session_event | — | `bridge.go` (SessionBridge) | **canonical for A06-T01** |

---

## Legacy Module Index（D5-S1–S9，冻结追溯）

| Legacy S | Module | Canonical S |
|----------|--------|-------------|
| D5-S1–S3, S6 | Tracer/Metrics/Logger/Telemetry | S21 |
| D5-S4 | Exporter | S22 |
| D5-S5, S8 | Coverage/Incident | S23 |
| D5-S7, S9 | Settings/Runtime | S24 |

---

## Statistics

| Scenarios | Activities | 变更 |
|-----------|------------|------|
| S0 + S21–S24 | **30** | +A14, +A03, +A07, +A09, +A10；A08 RETIRED |

## Revision History

| 版本 | 日期 | 变更 |
|------|------|------|
| 3.0.0 | 2026-06-15 | SA Refine v1.0 S21–S24 |
| **4.0.0** | **2026-06-19** | **v2.1：路径同步 + 诊断 A + 归属校正** |
