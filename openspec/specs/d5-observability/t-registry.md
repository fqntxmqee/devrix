# D5 Observability Domain — T 层测试点注册表

**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`

---

## D5-S1: Tracer Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D5-S1-A01-T01 | Shutdown 刷写所有 pending spans | Tracer | `internal/layers/observability/tracer/tracer_test.go` | IMPLEMENTED | P0 |
| D5-S1-A01-T02 | ConsoleExporter 可直接作为 SpanExporter | Tracer | `internal/layers/observability/exporter/console_test.go` | IMPLEMENTED | P2 |
| D5-S1-A03-T01 | Baggage set/get/list 与 header 往返 | Tracer | `internal/layers/observability/tracer/baggage_test.go` | IMPLEMENTED | P2 |
| D5-S1-A03-T02 | Propagator inject/extract traceparent | Tracer | `internal/layers/observability/tracer/propagation_test.go` | IMPLEMENTED | P1 |
| D5-S1-A03-T03 | CLI 子进程环境含 TRACEPARENT + BAGGAGE | Tracer | `internal/layers/observability/tracer/propagation_env_test.go` | IMPLEMENTED | P2 |
| D5-S1-A01-T04 | TraceID/SpanID 生成符合 W3C 格式 | Tracer | `internal/layers/observability/tracer/tracer_test.go` | IMPLEMENTED | P1 |

## D5-S2: Metrics Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D5-S2-A01-T01 | Tracing Span 创建与传播 | Metrics | — | PLANNED | P1 |
| D5-S2-A01-T02 | Metrics Counter 计数 | Metrics | — | PLANNED | P1 |
| D5-S2-A01-T03 | Gauge Set/Inc/Dec/Add/Sub 数值正确 | Metrics | `internal/layers/observability/metrics/gauge_test.go` | IMPLEMENTED | P0 |
| D5-S2-A01-T04 | Histogram Prometheus 输出与 golden 一致 | Metrics | `internal/layers/observability/metrics/histogram_test.go` | IMPLEMENTED | P0 |
| D5-S2-A01-T05 | Int64UpDownCounter 返回 Gauge | Metrics | `internal/layers/observability/metrics/meter_test.go` | IMPLEMENTED | P0 |
| D5-S2-A01-T06 | Compression P99 latency < 500ms | Metrics | `tests/performance/compression_test.go` | IMPLEMENTED | P1 |
| D5-S2-A01-T07 | Concurrent session memory bounded | Metrics | `tests/performance/memory_test.go` | IMPLEMENTED | P1 |
| D5-S2-A01-T08 | gen_ai.client.token.usage 含 input/output/cache_read/reasoning | Metrics | `internal/layers/observability/genai_tokens_test.go` | IMPLEMENTED | P2 |
| D5-S2-A01-T09 | tool_latency histogram 注册与 observe | Metrics | `internal/layers/observability/bridge_tool_latency_test.go` | IMPLEMENTED | P1 |

## D5-S3: Logger Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D5-S3-A01-T01 | 日志级别过滤 | Logger | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED | P1 |
| D5-S3-A01-T02 | Shutdown 覆盖 Tracer + Logger | Logger | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED | P0 |
| D5-S3-A01-T03 | Error 日志包含 stacktrace 字段 | Logger | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED | P1 |
| D5-S3-A01-T04 | 日志采样 max_entries_per_span 生效 | Logger | `internal/layers/observability/logger/sampling_test.go` | IMPLEMENTED | P1 |
| D5-S3-A02-T01 | slog 从 context 注入 traceId/spanId | Logger | `internal/layers/observability/logger/slog_bridge_test.go` | IMPLEMENTED | P0 |
| D5-S3-A01-T05 | 敏感字段脱敏 [REDACTED] | Logger | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED | P1 |

## D5-S4: Exporter Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D5-S4-A01-T01 | OTLP span 事件属性序列化 | Exporter | `internal/layers/observability/exporter/otlp_event_test.go` | IMPLEMENTED | P1 |
| D5-S4-A01-T02 | QueryLoop 产生 canonical Operation span | Exporter | `internal/layers/contextengine/query/loop.go` | IMPLEMENTED | P0 |
| D5-S4-A01-T03 | Adapter→Gateway trace_id 继承 | Exporter | `tests/integration/obs_trace_propagation_test.go` | IMPLEMENTED | P0 |

## D5-S5: Coverage Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D5-S5-A01-T01 | Operation Registry 与 names.go 全集一致（56 条） | Coverage | `internal/layers/observability/coverage/registry_test.go` | IMPLEMENTED | P0 |
| D5-S5-A01-T02 | Coverage 报告正确列出 zero_hit operations | Coverage | `internal/layers/observability/coverage/coverage_test.go` | IMPLEMENTED | P0 |
| D5-S5-A01-T03 | 100 并发 RecordHit 计数正确 | Coverage | `internal/layers/observability/coverage/coverage_test.go` | IMPLEMENTED | P0 |
| D5-S5-A01-T04 | 采样关闭仍 RecordHit | Coverage | `internal/layers/observability/coverage/coverage_test.go` | IMPLEMENTED | P0 |
| D5-S5-A01-T05 | Harness enabled 产生 harness span 树 | Coverage | `tests/integration/context_harness_obs_test.go` | IMPLEMENTED | P1 |
| D5-S5-A02-T01 | 端到端染色集成 | Coverage | `internal/layers/observability/coverage_integration_test.go` | IMPLEMENTED | P1 |

## D5-S6: Telemetry Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D5-S6-A01-T01 | LayerAndComponent 映射 gateway operation | Telemetry | `internal/layers/observability/telemetry/names_test.go` | IMPLEMENTED | P0 |
| D5-S6-A01-T02 | SpanAttrs 含 devrix.layer/component | Telemetry | `internal/layers/observability/telemetry/names_test.go` | IMPLEMENTED | P0 |
| D5-S6-A03-T01 | GenAIUsageAttrs 含 OTel 语义属性 | Telemetry | `internal/layers/observability/telemetry/names_test.go` | IMPLEMENTED | P1 |
| D5-S6-A03-T02 | GenAIUsageAttrs 含 cache_read/reasoning 细分 | Telemetry | `internal/layers/observability/telemetry/names_test.go` | IMPLEMENTED | P2 |

## D5-S8: Incident Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D5-S8-A01-T01 | Export bundle schema v1 有效 JSON | Incident | `internal/layers/observability/incident/export_test.go` | IMPLEMENTED | P1 |
| D5-S8-A01-T02 | CLI `devrix debug export` 行为一致 | Incident | `internal/cli/debug/export_test.go` | IMPLEMENTED | P2 |

## D5-S9: Runtime Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D5-S9-A01-T01 | RegisterD5 幂等注册 path counter | Runtime | `internal/layers/observability/runtime/d5_metric_test.go` | IMPLEMENTED | P1 |
| D5-S9-A01-T02 | IncD5 桥接 query_loop/legacy_harness 计数 | Runtime | `internal/layers/observability/runtime/d5_metric_test.go` | IMPLEMENTED | P1 |
| D5-S9-A01-T03 | PathResolver 并发 Record 安全 | Runtime | `internal/layers/observability/runtime/path_resolver_test.go` | IMPLEMENTED | P1 |

## D5-S0: Cross-Scenario

| T ID | 描述 | Test 位置 | Status | Priority |
|-------|------|-----------|--------|----------|
| D5-S0-A02-T01 | SessionBridge ActiveSessions gauge 增减 | `tests/integration/obs_session_bridge_test.go` | IMPLEMENTED | P1 |
| D5-S0-A02-T02 | HealthCheck 含 coverage 摘要 | — | PLANNED | P1 |

---

## Statistics

| Total | IMPLEMENTED | PLANNED |
|-------|-------------|---------|
| 38 | 35 | 3 |

## P0 测试点清单

D5-S1-A01-T01, D5-S2-A01-T03, D5-S2-A01-T04, D5-S2-A01-T05, D5-S3-A02-T01, D5-S4-A01-T02, D5-S4-A01-T03, D5-S5-A01-T01~T04, D5-S6-A01-T01~T02
