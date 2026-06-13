# D5 Observability Domain — T 层测试点注册表

**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## D5-S2: Metrics Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D5-S2-A01-T01 | Tracing Span 创建与传播 | Metrics | - | PLANNED |
| D5-S2-A01-T02 | Metrics Counter 计数 | Metrics | - | PLANNED |
| D5-S2-A01-T03 | Gauge Set/Inc/Dec/Add/Sub 数值正确 | Metrics | `internal/layers/observability/metrics/gauge_test.go` | IMPLEMENTED |
| D5-S2-A01-T04 | Histogram Prometheus 输出与 golden 一致 | Metrics | `internal/layers/observability/metrics/histogram_test.go` | IMPLEMENTED |
| D5-S2-A01-T05 | Int64UpDownCounter 返回 Gauge | Metrics | `internal/layers/observability/metrics/meter_test.go` | IMPLEMENTED |
| D5-S2-A01-T06 | Compression P99 latency < 500ms | Metrics | `tests/performance/compression_test.go` | IMPLEMENTED |
| D5-S2-A01-T07 | Concurrent session memory bounded | Metrics | `tests/performance/memory_test.go` | IMPLEMENTED |

## D5-S3: Logger Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D5-S3-A01-T01 | 日志级别过滤 | Logger | - | PLANNED |
| D5-S3-A01-T02 | Shutdown 覆盖 Tracer + Logger | Logger | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED |
| D5-S3-A01-T03 | Error 日志包含 stacktrace 字段 | Logger | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED |
| D5-S3-A01-T04 | 日志采样 max_entries_per_span 生效 | Logger | `internal/layers/observability/logger/sampling_test.go` | IMPLEMENTED |

## D5-S1: Tracer Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D5-S1-A01-T01 | Shutdown 刷写所有 pending spans | Tracer | `internal/layers/observability/tracer/tracer_test.go` | IMPLEMENTED |
| D5-S1-A01-T02 | ConsoleExporter 可直接作为 SpanExporter | Tracer | `internal/layers/observability/exporter/console_test.go` | IMPLEMENTED |

## D5-S4: Exporter Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D5-S4-A01-T01 | LongTerm recall/store 产生 canonical Operation span | Exporter | `internal/layers/contextengine/engine.go` | IMPLEMENTED |
| D5-S4-A01-T02 | QueryLoop 产生 canonical Operation span | Exporter | `internal/layers/contextengine/query/loop.go` | IMPLEMENTED |
| D5-S4-A01-T03 | Feishu 入站产生 adapter.message.receive span | Exporter | `tests/integration/obs_trace_propagation_test.go` | IMPLEMENTED |

## D5-S5: Coverage Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D5-S5-A01-T01 | Operation Registry 与 names.go 常驻全集一致 | Coverage | `internal/layers/observability/coverage/registry_test.go` | IMPLEMENTED |
| D5-S5-A01-T02 | Coverage 报告正确列出 zero_hit operations | Coverage | `tests/integration/obs_coverage_test.go`, `tests/integration/context_harness_obs_test.go` | IMPLEMENTED |

## D5: Cross-Scenario

| T ID | 描述 | Test 位置 | Status |
|-------|------|-----------|--------|
| D5-S0-A01-T01 | Gateway 会话 | - | PLANNED |

---

## Statistics

| Total | IMPLEMENTED | PLANNED |
|-------|-------------|---------|
| 19 | 15 | 4 |
