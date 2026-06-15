# D5 Observability Domain — T 层测试点注册表

**Status:** Active
**Version:** 3.0.0
**Last Updated:** 2026-06-15
**Parent:** `openspec/specs/architecture/layering.md`
**Change:** devrix-d5-sa-refine（DM-20260615-001 / v1.0 Canonical 重排；增 canonical_s 列 + Legacy 双轨）

---

## D5-S21: Instrument（遥测生成）

| T ID | 描述 | canonical_s | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|----------|-----------|--------|-------------|
| D5-S21-A01-T01 | Shutdown 刷写所有 pending spans | S21 | P0 | `internal/layers/observability/tracer/tracer_test.go` | IMPLEMENTED | D5-S1-A01-T01 |
| D5-S21-A01-T02 | ConsoleExporter 可直接作为 SpanExporter | S21 | P2 | `internal/layers/observability/exporter/console_test.go` | IMPLEMENTED | D5-S1-A01-T02 |
| D5-S21-A01-T04 | TraceID/SpanID 生成符合 W3C 格式 | S21 | P1 | `internal/layers/observability/tracer/tracer_test.go` | IMPLEMENTED | D5-S1-A01-T04 |
| D5-S21-A03-T01 | Baggage set/get/list 与 header 往返 | S21 | P2 | `internal/layers/observability/tracer/baggage_test.go` | IMPLEMENTED | D5-S1-A03-T01 |
| D5-S21-A03-T02 | Propagator inject/extract traceparent | S21 | P1 | `internal/layers/observability/tracer/propagation_test.go` | IMPLEMENTED | D5-S1-A03-T02 |
| D5-S21-A03-T03 | CLI 子进程环境含 TRACEPARENT + BAGGAGE | S21 | P2 | `internal/layers/observability/tracer/propagation_env_test.go` | IMPLEMENTED | D5-S1-A03-T03 |
| D5-S21-A05-T01 | Tracing Span 创建与传播 | S21 | P1 | — | PLANNED | D5-S2-A01-T01 |
| D5-S21-A05-T02 | Metrics Counter 计数 | S21 | P1 | — | PLANNED | D5-S2-A01-T02 |
| D5-S21-A05-T03 | Gauge Set/Inc/Dec/Add/Sub 数值正确 | S21 | P0 | `internal/layers/observability/metrics/gauge_test.go` | IMPLEMENTED | D5-S2-A01-T03 |
| D5-S21-A05-T04 | Histogram Prometheus 输出与 golden 一致 | S21 | P0 | `internal/layers/observability/metrics/histogram_test.go` | IMPLEMENTED | D5-S2-A01-T04 |
| D5-S21-A05-T05 | Int64UpDownCounter 返回 Gauge | S21 | P0 | `internal/layers/observability/metrics/meter_test.go` | IMPLEMENTED | D5-S2-A01-T05 |
| D5-S21-A05-T06 | tool_latency histogram 注册与 observe | S21 | P1 | `internal/layers/observability/bridge_tool_latency_test.go` | IMPLEMENTED | D5-S2-A01-T09 |
| D5-S21-A07-T01 | gen_ai.client.token.usage 含 input/output/cache_read/reasoning | S21 | P2 | `internal/layers/observability/genai_tokens_test.go` | IMPLEMENTED | D5-S2-A01-T08 |
| D5-S21-A08-T01 | 日志级别过滤 | S21 | P1 | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED | D5-S3-A01-T01 |
| D5-S21-A08-T02 | Shutdown 覆盖 Tracer + Logger | S21 | P0 | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED | D5-S3-A01-T02 |
| D5-S21-A08-T03 | Error 日志包含 stacktrace 字段 | S21 | P1 | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED | D5-S3-A01-T03 |
| D5-S21-A08-T04 | 日志采样 max_entries_per_span 生效 | S21 | P1 | `internal/layers/observability/logger/sampling_test.go` | IMPLEMENTED | D5-S3-A01-T04 |
| D5-S21-A08-T05 | 敏感字段脱敏 [REDACTED] | S21 | P1 | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED | D5-S3-A01-T05 |
| D5-S21-A09-T01 | slog 从 context 注入 traceId/spanId | S21 | P0 | `internal/layers/observability/logger/slog_bridge_test.go` | IMPLEMENTED | D5-S3-A02-T01 |
| D5-S21-A11-T01 | LayerAndComponent 映射 gateway operation | S21 | P0 | `internal/layers/observability/telemetry/names_test.go` | IMPLEMENTED | D5-S6-A01-T01 |
| D5-S21-A12-T01 | SpanAttrs 含 devrix.layer/component | S21 | P0 | `internal/layers/observability/telemetry/names_test.go` | IMPLEMENTED | D5-S6-A01-T02 |
| D5-S21-A13-T01 | GenAIUsageAttrs 含 OTel 语义属性 | S21 | P1 | `internal/layers/observability/telemetry/names_test.go` | IMPLEMENTED | D5-S6-A03-T01 |
| D5-S21-A13-T02 | GenAIUsageAttrs 含 cache_read/reasoning 细分 | S21 | P2 | `internal/layers/observability/telemetry/names_test.go` | IMPLEMENTED | D5-S6-A03-T02 |

## D5-S22: Export（遥测导出）

| T ID | 描述 | canonical_s | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|----------|-----------|--------|-------------|
| D5-S22-A01-T01 | OTLP span 事件属性序列化 | S22 | P1 | `internal/layers/observability/exporter/otlp_event_test.go` | IMPLEMENTED | D5-S4-A01-T01 |
| D5-S22-A01-T02 | QueryLoop 产生 canonical Operation span | S22 | P0 | `internal/layers/contextengine/query/loop.go` | IMPLEMENTED | D5-S4-A01-T02 |
| D5-S22-A01-T03 | Adapter→Gateway trace_id 继承 | S22 | P0 | `tests/integration/obs_trace_propagation_test.go` | IMPLEMENTED | D5-S4-A01-T03 |

## D5-S23: Diagnose（诊断辅助）

| T ID | 描述 | canonical_s | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|----------|-----------|--------|-------------|
| D5-S23-A01-T01 | Operation Registry 与 names.go 全集一致（56 条） | S23 | P0 | `internal/layers/observability/coverage/registry_test.go` | IMPLEMENTED | D5-S5-A01-T01 |
| D5-S23-A01-T02 | Coverage 报告正确列出 zero_hit operations | S23 | P0 | `internal/layers/observability/coverage/coverage_test.go` | IMPLEMENTED | D5-S5-A01-T02 |
| D5-S23-A01-T03 | 100 并发 RecordHit 计数正确 | S23 | P0 | `internal/layers/observability/coverage/coverage_test.go` | IMPLEMENTED | D5-S5-A01-T03 |
| D5-S23-A01-T04 | 采样关闭仍 RecordHit | S23 | P0 | `internal/layers/observability/coverage/coverage_test.go` | IMPLEMENTED | D5-S5-A01-T04 |
| D5-S23-A01-T05 | Harness enabled 产生 harness span 树 | S23 | P1 | `tests/integration/context_harness_obs_test.go` | IMPLEMENTED | D5-S5-A01-T05 |
| D5-S23-A02-T01 | 端到端染色集成 | S23 | P1 | `internal/layers/observability/coverage_integration_test.go` | IMPLEMENTED | D5-S5-A02-T01 |
| D5-S23-A04-T01 | Export bundle schema v1 有效 JSON | S23 | P1 | `internal/layers/observability/incident/export_test.go` | IMPLEMENTED | D5-S8-A01-T01 |
| D5-S23-A04-T02 | CLI `devrix debug export` 行为一致 | S23 | P2 | `internal/cli/debug/export_test.go` | IMPLEMENTED | D5-S8-A01-T02 |
| D5-S23-A06-T01 | SessionBridge ActiveSessions gauge 增减 | S23 | P1 | `tests/integration/obs_session_bridge_test.go` | IMPLEMENTED | D5-S0-A02-T01 |
| D5-S23-A06-T02 | HealthCheck 含 coverage 摘要 | S23 | P1 | — | PLANNED | D5-S0-A02-T02 |

## D5-S24: Configure（配置与运行时管理）

| T ID | 描述 | canonical_s | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|----------|-----------|--------|-------------|
| D5-S24-A03-T01 | RegisterRuntimeMetric 幂等注册 path counter | S24 | P1 | `internal/layers/observability/runtime/runtime_metric_test.go` | IMPLEMENTED | D5-S9-A01-T01 |
| D5-S24-A03-T02 | IncRuntimeMetric 桥接 query_loop/legacy_harness 计数 | S24 | P1 | `internal/layers/observability/runtime/runtime_metric_test.go` | IMPLEMENTED | D5-S9-A01-T02 |
| D5-S24-A03-T03 | PathResolver 并发 Record 安全 | S24 | P1 | `internal/layers/observability/runtime/path_resolver_test.go` | IMPLEMENTED | D5-S9-A01-T03 |

## CROSS: 跨域性能测试（从 D5-S2 迁出）

| T ID | 描述 | Priority | Test 位置 | Status | Legacy T ID |
|------|------|----------|-----------|--------|-------------|
| CROSS-D5-T01 | Compression P99 latency < 500ms | P1 | `tests/performance/compression_test.go` | IMPLEMENTED | D5-S2-A01-T06 |
| CROSS-D5-T02 | Concurrent session memory bounded | P1 | `tests/performance/memory_test.go` | IMPLEMENTED | D5-S2-A01-T07 |

---

## Legacy Module Index（旧 T 编号→新 Canonical）

| Legacy S | T 数 | Canonical S | Scenario |
|----------|------|-------------|----------|
| D5-S1 Tracer | 6 | S21 | Instrument |
| D5-S2 Metrics | 7 + 2 CROSS | S21 + CROSS | Instrument |
| D5-S3 Logger | 6 | S21 | Instrument |
| D5-S4 Exporter | 3 | S22 | Export |
| D5-S5 Coverage | 6 | S23 | Diagnose |
| D5-S6 Telemetry | 4 | S21 | Instrument |
| D5-S8 Incident | 2 | S23 | Diagnose |
| D5-S9 Runtime | 3 | S24 | Configure |
| D5-S0 Cross | 2 | S23 | Diagnose |

---

## Statistics

| Total | IMPLEMENTED | PLANNED |
|-------|-------------|---------|
| 41 | 38 | 3 |

> 注：含 2 条 CROSS 段性能测试（CROSS-D5-T01/T02）。D5-S21-A01-T03 为历史缺口（原始 S1-A01 无 T03），非本 change 引入。

## P0 测试点清单

D5-S21-A01-T01, D5-S21-A05-T03, D5-S21-A05-T04, D5-S21-A05-T05, D5-S21-A08-T02, D5-S21-A09-T01, D5-S21-A11-T01, D5-S21-A12-T01, D5-S22-A01-T02, D5-S22-A01-T03, D5-S23-A01-T01, D5-S23-A01-T02, D5-S23-A01-T03, D5-S23-A01-T04

## Revision History

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.0.0 | 2026-06-14 | 初版：9 技术模块 S 层，38 T |
| **3.0.0** | 2026-06-15 | **SA Refine v1.0**：Canonical S21–S24 重排；增 canonical_s + Legacy T ID 列；2 性能 T 迁 CROSS 段 |
