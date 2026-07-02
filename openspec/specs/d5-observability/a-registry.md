# D5 Observability Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 4.0.0
**Last Updated:** 2026-06-19
**Parent:** `openspec/specs/architecture/layering.md`
**Change:** devrix-d5-v2-terminal（DM-20260619-006 / v2.1 Terminal；Code Location 从 bridge 路径校正为 canonical 路径；+5 A：A14 FilterDebugLog + A03 TrackActiveSessions + A07 Tracker + A09 FaultInject + A10 Doctor）

---

## Overview

D5 可观测性域 A 层活动注册表（Canonical v4.0）。S 层为 4+1 价值流（S21–S24 + S0）。Code Location 列全部使用 v2.1 canonical 物理路径。

> **代码锚点：** 本文件 Code Location 列更新为 canonical 路径，是 Phase A 的 ≥1 个代码锚点之一（AC-A8）。

---

## D5-S21: Instrument（遥测生成）

**承诺 C1：** 为任意操作生成遥测数据（Span + Metric + Log + 属性构建）

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D5-S21-A01 | CreateSpan | A-BE | operation, parent_ctx, opts | span, ctx | span.started | `instrument/tracer/tracer.go` (Tracer.Start) | S1-A01 |
| D5-S21-A02 | EndSpan | A-BE | span | -- | span.ended | `instrument/tracer/span.go` (Span.End) | S1-A02 |
| D5-S21-A03 | PropagateContext | A-BE | ctx, carrier | injected_carrier | -- | `instrument/tracer/propagation.go`, `instrument/tracer/baggage.go` | S1-A03 |
| D5-S21-A04 | ShutdownTracer | A-BE | ctx | error | spans.flushed | `instrument/tracer/tracer.go` (TracerProvider.Shutdown) | S1-A04 |
| D5-S21-A05 | RecordMetric | A-BE | instrument, value, labels | -- | metric.recorded | `instrument/metrics/meter.go` | S2-A01 |
| D5-S21-A06 | ExportPrometheus | A-BE | -- | exposition_text | -- | `instrument/metrics/prometheus.go` (Handler) | S2-A02 |
| D5-S21-A07 | RecordGenAITokens | A-BE | model, usage_breakdown | -- | counter.inc | `instrument/metrics/genai_tokens.go` (RecordGenAITokenUsage) | S2-A03 |
| D5-S21-A08 | LogRecord | A-BE | level, message, fields | -- | log.emitted | `instrument/logger/logger.go` | S3-A01 |
| D5-S21-A09 | InstallSlogBridge | A-BE | -- | -- | slog.trace_injection | `instrument/logger/slog_bridge.go` | S3-A02 |
| D5-S21-A10 | ShutdownLogger | A-BE | -- | error | sampler.reset | `instrument/logger/logger.go` (Close) | S3-A03 |
| D5-S21-A11 | ResolveLayerComponent | A-BE | operation | layer, component | -- | `instrument/telemetry/names.go` (LayerAndComponent) | S6-A01 |
| D5-S21-A12 | BuildSpanAttrs | A-BE | operation, extras | []Attribute | -- | `instrument/telemetry/names.go` (SpanAttrs) | S6-A02 |
| D5-S21-A13 | BuildGenAIAttrs | A-BE | model, tokens, session | []Attribute | -- | `instrument/telemetry/names.go` (GenAIUsageAttrs) | S6-A03 |
| **D5-S21-A14** | **FilterDebugLog** | **A-BE** | **categories, level, msg** | **filtered** | **--** | **`instrument/logger/debugfilter/filter.go`** | **—** |

## D5-S22: Export（遥测导出）

**承诺 C2：** 将遥测数据导出到外部系统（OTLP/Prometheus/Console）

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D5-S22-A01 | ExportSpans | A-BE | span_batch | error | spans.exported | `export/console.go`, `export/otlp.go`, `export/memory.go` | S4-A01 |
| D5-S22-A02 | CreateExporter | A-BE | tracing_config | SpanExporter | -- | `export/factory.go` (NewTracingExporter) | S4-A02 |

## D5-S23: Diagnose（诊断辅助）

**承诺 C3：** 提供诊断辅助（Coverage 报告 + Incident 导出 + Health Check + Doctor + Tracker + FaultInject）

> **S23 子承诺：** C3a Coverage · C3b Incident · C3c Doctor · C3d Tracker · C3e FaultInject。硬边界：S23 只含"事后审计/举证"；子承诺数 ≤ 7；不 import D2/D4/D7。

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D5-S23-A01 | RecordOperationHit | A-BE | operation_name | -- | hit.inc | `diagnose/coverage/coverage.go` (RecordHit), `instrument/tracer/tracer.go` | S5-A01 |
| D5-S23-A02 | AssessCoverage | A-BE | operations[] | coverage_report | -- | `diagnose/coverage/coverage.go` (Counter.Report) | S5-A02 |
| D5-S23-A03 | GenerateDailyReport | A-BE | -- | daily_report | report.persisted | `diagnose/coverage/reporter.go` (GenerateNow) | S5-A03 |
| D5-S23-A04 | ExportSessionBundle | A-BE | session_id, opts | JSON bundle | -- | `diagnose/incident/export.go` (BuildBundle) | S8-A01 |
| D5-S23-A05 | RecordLLMPayload | A-BE | span, session, payload | -- | jsonl.written | `diagnose/incident/llm_log.go` (RecordLLMSpanPayload) | S8-A02 |
| D5-S23-A06 | HealthCheck | A-BE | -- | status_map | -- | `health.go`, `observability.go` (HealthCheck) | S0-A02 |
| **D5-S23-A07** | **RunDiagnosticTracker** | **A-BE** | **watch_paths, linter_config** | **diff_report, lint_issues** | **--** | **`diagnose/tracker/tracker.go`** | **—** |
| **D5-S23-A09** | **InjectFault** | **A-BE** | **fault_config** | **--** | **fault.injected** | **`diagnose/faultinject/injector.go`** (testbuild only) | **—** |
| **D5-S23-A10** | **RunDoctor** | **A-BE** | **check_list** | **health_report** | **--** | **`diagnose/doctor/doctor.go`** | **—** |

## D5-S24: Configure（配置与运行时管理）

**承诺 C4：** 加载/校验配置 + 运行时路径计数

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D5-S24-A01 | LoadObsConfig | A-BE | yaml/env | *Config | -- | `config.go`, `load.go`, `configure/settings/config.go` | S7-A01 |
| D5-S24-A02 | ValidateObsConfig | A-BE | *Config | error | -- | `config.go` (Validate) | S7-A02 |
| D5-S24-A03 | RecordRuntimePath | A-BE | path_kind | -- | path_counter.inc | `configure/runtime/path_resolver.go` (Record), `configure/runtime/runtime_metric.go` (IncRuntimeMetric) | S9-A01 |
| D5-S24-A04 | RegisterRuntimeMetric | A-BE | meter | error | metric.registered | `configure/runtime/runtime_metric.go` (RegisterRuntimeMetric) | S9-A02 |

## D5-S0: Facade（横切）

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| D5-S0-A01 | InitObservability | A-BE | *Config | *Observability | obs.initialized | `observability.go` (New) | S0-A01 |
| D5-S0-A02 | CreateBridge | A-BE | *Observability | *Bridge | -- | `bridge.go` (NewBridge) | S0-A03 |
| **D5-S0-A03** | **TrackActiveSessions** | **A-BE** | **adapter, delta** | **--** | **gauge.updated** | **`bridge.go` (SessionBridge.ActiveSessions)** | **—** |

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


## D5-S25: Termination Invariants (LTL-Lite L4–L6 + L7 umbrella)

**承诺 C5：** 为 Execute 节点 4 ToolChannel 提供 LTL-Lite L4–L6 termination 不变量 + L7 子类（Fact/Action/Experiment deadline），支持 Bounded(n) hard reject + Quotient(metric) + Synthesize(min_chars) 三层收敛。

| A ID | Name | Type | Input | Output | State Change | Code Location | Legacy |
|------|------|------|-------|--------|--------------|---------------|--------|
| **D5-S25-A01** | **BoundedInvariant** | **A-BE** | **state, MaxN** | **(ok, reason)** | **—** | **`observability/instrument/ltl/invariants/termination/bounded.go`** (BoundedInvariant.Check) | **—** |
| **D5-S25-A02** | **QuotientInvariant** | **A-BE** | **state, Threshold, Metric** | **(ok, reason)** | **—** | **`observability/instrument/ltl/invariants/termination/bounded.go`** (QuotientInvariant.Check) | **—** |
| **D5-S25-A03** | **SynthesizeInvariant** | **A-BE** | **state, MinChars** | **(ok, reason)** | **—** | **`observability/instrument/ltl/invariants/termination/bounded.go`** (SynthesizeInvariant.Check) | **—** |

**L7 umbrella**（FactSameQuery / ActionPostSnapshot / ExperimentDeadline）在 `bounded.go` 同文件实现，作为 termination 4-元 umbrella 类不另开 A ID（避免 L4/L5/L6 占用后无 slot 编号）。

| L 编号 | 大类 | 具体 invariant | 适用 Channel |
|--------|------|----------------|--------------|
| L4 | Bounded | `L4-BOUNDED-ITERATIONS` | ProbeToolChannel (Bounded(n)) |
| L5 | Quotient | `L5-QUOTIENT-THRESHOLD` | ExperimentToolChannel (Quotient 0.8) |
| L6 | Synthesize | `L6-SYNTHESIZE-MIN-CHARS` | ProbeToolChannel (synthesize 阶段) |
| L7 | Umbrella | `L7-FACT-SAME-Q-5x` / `L7-ACTION-POSTSNAPSHOT` / `L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE` | Fact / Action / Experiment |

> **Code 锚点**: 本 change `devrix-mups-tool-classification-and-channel-autonomy` (DM-20260701-007) Phase B 落地。

## Statistics

| Scenarios | Activities | IMPLEMENTED |
|-----------|------------|-------------|
| 6 (S21–S25 + S0) | 35 | 35 |

## Revision History

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.0.0 | 2026-06-14 | 初版：9+1 技术模块 S 层 |
| 3.0.0 | 2026-06-15 | SA Refine v1.0：Canonical 重排为 4+1 价值流 S21–S24；增 Legacy 列 + Module Index |
| **4.0.0** | **2026-06-19** | **v2.1 Terminal（代码锚点）**：Code Location 全部校正为 canonical 路径（bridge→instrument/export/diagnose/configure）；+5 A（S21-A14 FilterDebugLog、S23-A07 Tracker、S23-A09 FaultInject、S23-A10 Doctor、S0-A03 TrackActiveSessions）；S23 硬边界 + 子承诺标注 |
