# Delta: Observability Layer (Layer 5)

**Change ID:** devrix-foundation
**Affects:** observability, tracing, metrics, logging
**Version:** 2.0.0
**Status:** Draft
**Last Updated:** 2026-06-07

---

## Overview

可观察层为 Devrix 提供分布式追踪、指标采集和结构化日志能力，对齐 OpenTelemetry (OTel) 标准。分为 **Trace**（调用链）、**Metrics**（指标）、**Log**（日志）三大支柱。

### Design Principles

1. **OTel Native** - 数据模型、SpanContext、Propagator 对齐 W3C Trace Context
2. **Zero-Config Default** - 开箱即用，无需配置即可工作
3. **Graceful Degradation** - 可观察模块故障不影响核心业务
4. **Cardinality Safety** - Label 白名单防止高基数指标爆炸

### Version Scope

| Version | Milestone | Features |
|---------|-----------|----------|
| V1 | MVP | Console exporter, basic spans, Prometheus metrics |
| V2 | Enhanced | OTLP exporter, sampling, baggage propagation |
| V3 | Full | Jaeger/Zipkin, tail-based sampling, APM integration |

---

## ADDED

### Requirement: Trace/Span Data Model

基于 OpenTelemetry Span 模型实现调用链追踪。

#### Scenario: Create root span on message arrival
- GIVEN a new message arrives at Communication Layer
- WHEN `RouteInbound` is called
- THEN a root span is created with:
  - `traceId`: 32-char hex string (W3C standard)
  - `spanId`: 16-char hex string
  - `parentSpanId`: empty (root span)
  - `service.name`: "devrix"
  - `service.version`: from `devrix.yaml`
  - `session.id`: from message
  - start timestamp

#### Scenario: Create child span for LLM call
- GIVEN a root span exists with traceId
- WHEN `LLMGateway.chat` is called
- THEN a child span is created with:
  - `parentSpanId`: from parent span
  - `span.name`: "llm.chat"
  - `span.attributes["llm.provider"]`: provider name
  - `span.attributes["llm.model"]`: model name
  - `span.attributes["llm.tokens.input"]`: input token count
  - `span.attributes["llm.tokens.output"]`: output token count
  - `span.attributes["llm.latency_ms"]`: call duration

#### Scenario: Create child span for tool execution
- GIVEN a span exists with traceId
- WHEN `ToolRegistry.execute` is called
- THEN a child span is created with:
  - `span.name`: "tool.execute"
  - `span.attributes["tool.name"]`: tool name
  - `span.attributes["tool.risk_level"]`: risk level
  - `span.attributes["tool.args"]`: sanitized args (secrets redacted)
  - `span.status`: Ok or Error

#### Scenario: End span with status
- GIVEN a span is active
- WHEN operation completes
- THEN span is ended with:
  - end timestamp
  - `span.status.code`: Unset | Ok | Error
  - `span.status.description`: error message if Error

#### Scenario: Record exception in span
- GIVEN a span is active and an error occurs
- WHEN `RecordError` is called
- THEN span is updated with:
  - `span.events` adds exception event
  - `span.events[-1].name`: "exception"
  - `span.events[-1].attributes["exception.type"]`: error type
  - `span.events[-1].attributes["exception.message"]`: error message
  - `span.events[-1].attributes["exception.stacktrace"]`: stack trace
  - `span.status.code`: Error

---

### Requirement: Span Lifecycle

Span 的完整生命周期管理。

#### Scenario: Span starts with correct parent
- GIVEN a context with parent SpanContext
- WHEN tracer.StartSpan is called
- THEN new span's parent is the given SpanContext
- AND traceId is inherited from parent

#### Scenario: Span starts with no parent (root)
- GIVEN a context with no SpanContext
- WHEN tracer.StartSpan is called
- THEN new span is a root span
- AND new traceId is generated

#### Scenario: Span context propagation via Context
- GIVEN a span is active
- WHEN context is passed to child operation
- THEN child inherits span via `context.WithSpan`
- AND span can be retrieved via `SpanFromContext`

#### Scenario: Span attributes are mutable
- GIVEN an active span
- WHEN SetAttribute is called multiple times
- THEN attributes are merged (later wins for same key)
- AND attribute count does not exceed 128

---

### Requirement: Trace ID Propagation

跨组件和跨进程的消息传播。

#### Scenario: Generate trace ID on entry
- GIVEN a request enters via CLI adapter
- WHEN `RouteInbound` is called
- THEN traceId is generated as:
  - Format: 32-char lowercase hex (W3C Traceparent compatible)
  - Example: `4bf92f3577b34da6a3ce929d0e0e4736`
  - OR extracted from incoming `traceparent` header if present

#### Scenario: Extract trace ID from incoming traceparent header
- GIVEN an incoming request has `traceparent` header
- WHEN headers are parsed
- THEN traceId is extracted from header format: `00-{traceId}-{spanId}-{flags}`
- AND span is created with that traceId
- AND incoming spanId becomes parent

#### Scenario: Propagate trace ID through layers
- GIVEN traceId exists at Communication Layer entry
- WHEN message flows through layers (Context Engine → LLM Gateway → Tool Registry)
- THEN traceId is passed via Go context.Context
- AND included in all log entries

#### Scenario: Inject trace ID into outgoing request headers
- GIVEN trace context exists
- WHEN outgoing HTTP/gRPC request is made
- THEN `traceparent` header is injected
- AND `tracestate` header is injected if baggage exists

#### Scenario: No trace ID (initial request)
- GIVEN request has no trace context
- WHEN it enters the system
- THEN new traceId is generated (root trace)
- AND `tracestate` is empty

---

### Requirement: Metrics Collection

Prometheus 格式指标采集，对齐 OpenMetrics 标准。

#### Scenario: Track llm_tokens_total counter
- GIVEN LLM call completes
- WHEN response is received
- THEN Counter `devrix_llm_tokens_total` is incremented
- AND labels:
  - `provider`: "anthropic" | "deepseek" | "openai"
  - `model`: model name
  - `direction`: "input" | "output"

#### Scenario: Track llm_latency_seconds histogram
- GIVEN LLM call completes
- WHEN response is received
- THEN Histogram `devrix_llm_latency_seconds` is recorded
- AND labels: `provider`, `model`
- AND bucket bounds: [0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, +Inf]

#### Scenario: Track llm_errors_total counter
- GIVEN LLM call fails
- WHEN error is returned
- THEN Counter `devrix_llm_errors_total` is incremented
- AND labels: `provider`, `model`, `error_type`

#### Scenario: Track tool_calls_total counter
- GIVEN tool execution completes
- WHEN `ToolRegistry.execute` returns
- THEN Counter `devrix_tool_calls_total` is incremented
- AND labels: `tool`, `risk_level`, `status` (success | error)

#### Scenario: Track session_active gauge
- GIVEN session is created
- WHEN `CreateSession` succeeds
- THEN Gauge `devrix_session_active` is incremented

#### Scenario: Track session_active gauge decrements
- GIVEN an active session
- WHEN `ExpireSession` is called
- THEN Gauge `devrix_session_active` is decremented

#### Scenario: Track permission_timeouts counter
- GIVEN permission request times out
- WHEN timeout handler is called
- THEN Counter `devrix_permission_timeouts_total` is incremented

#### Scenario: Track permission_decisions counter
- GIVEN user responds to permission
- WHEN response is processed
- THEN Counter `devrix_permission_decisions_total` is incremented
- AND labels: `decision`: "approved" | "denied"

#### Scenario: Label cardinality is controlled
- GIVEN metric labels are configured
- WHEN label value has high cardinality (e.g., session ID)
- THEN label is NOT added to metric
- AND warning is logged if debug enabled

#### Scenario: Track context_tokens_gauge
- GIVEN context engine processes a message
- WHEN token count is calculated
- THEN Gauge `devrix_context_tokens_current` is set
- AND labels: `session_id` (truncated to 8 chars)

---

### Requirement: Metrics Exporter

指标导出到 Prometheus 端点。

#### Scenario: Prometheus endpoint returns metrics
- GIVEN metrics are collected
- WHEN `GET /metrics` is called
- THEN response is in Prometheus exposition format
- AND includes TYPE and HELP comments
- AND includes all registered metrics

#### Scenario: Metrics endpoint with authentication
- GIVEN auth is configured for metrics endpoint
- WHEN `GET /metrics` is called without token
- THEN 401 Unauthorized is returned
- AND no metrics are exposed

#### Scenario: OTLP metrics export
- GIVEN OTLP exporter is configured
- WHEN metric is recorded
- THEN metric is exported via OTLP protocol
- AND batched (every 5 seconds or 100 metrics)

#### Scenario: Metrics export on graceful shutdown
- GIVEN shutdown is initiated
- WHEN `Shutdown` is called on metrics exporter
- THEN all pending metrics are flushed
- AND exporter connection is closed

---

### Requirement: Structured Logging

JSON 格式结构化日志，带 trace context。

#### Scenario: Log entry includes trace context
- GIVEN logger is called
- WHEN `logger.Info` / `logger.Error` is invoked
- THEN log entry includes:
  - `timestamp`: ISO8601
  - `level`: INFO/WARN/ERROR
  - `message`: the log message
  - `traceId`: 32-char hex
  - `spanId`: 16-char hex
  - `component`: layer name
  - `service`: "devrix"
  - `version`: from config

#### Scenario: Log entry from different component
- GIVEN log comes from communication layer
- WHEN logger is instantiated in component
- THEN `component` field is set to "communication"
- AND distinguishes: "communication", "context_engine", "llm_gateway", "tool_registry", "multi_agent"

#### Scenario: Log level filtering
- GIVEN log level is set to INFO
- WHEN `logger.Debug` is called
- THEN log is not output (filtered)
- AND `logger.Info` / `logger.Warn` / `logger.Error` are output

#### Scenario: Error log includes stack trace
- GIVEN an error occurs
- WHEN `logger.Error` is called with error
- THEN log entry includes:
  - `error`: error message
  - `stacktrace`: formatted stack trace
  - `error.type`: error type name

#### Scenario: Log sampling for high-volume spans
- GIVEN span has > 100 log entries
- WHEN sampling is enabled
- THEN only first 10 and last 10 logs are kept
- AND warning indicates sampling occurred

#### Scenario: Secret redaction in logs
- GIVEN log contains sensitive fields
- WHEN `logger` formats message
- THEN keys matching `[password, token, secret, api_key]` are redacted
- AND value is replaced with `"[REDACTED]"`

---

### Requirement: Configuration Schema

可观察模块的配置管理。

#### Scenario: Observability disabled
- GIVEN `observability.enabled: false`
- WHEN Devrix starts
- THEN no tracing, metrics, or structured logging is initialized
- AND application functions normally without observability overhead

#### Scenario: Tracing enabled with console exporter
- GIVEN `observability.tracing.enabled: true`
- AND `observability.tracing.exporter: "console"`
- WHEN Devrix starts
- THEN spans are printed to stdout in JSON format

#### Scenario: Tracing enabled with OTLP exporter
- GIVEN `observability.tracing.enabled: true`
- AND `observability.tracing.exporter: "otlp"`
- AND `observability.tracing.otlp.endpoint: "localhost:4317"`
- WHEN Devrix starts
- THEN spans are exported via OTLP/gRPC

#### Scenario: Sampling rate configuration
- GIVEN `observability.tracing.sampling.rate: 0.1`
- WHEN span is created
- THEN 10% of spans are sampled
- AND traceId ending determines sampling decision

#### Scenario: Metrics enabled with Prometheus exporter
- GIVEN `observability.metrics.enabled: true`
- AND `observability.metrics.exporter: "prometheus"`
- AND `observability.metrics.endpoint: "/metrics"`
- WHEN Devrix starts
- THEN Prometheus endpoint is registered
- AND metrics are served at configured path

#### Scenario: Log level configuration
- GIVEN `observability.logging.level: "debug"`
- WHEN logger is initialized
- THEN DEBUG level logs are output
- AND higher severity logs (INFO, WARN, ERROR) are also output

#### Scenario: JSON log format
- GIVEN `observability.logging.format: "json"`
- WHEN log entry is written
- THEN output is valid JSON to stdout

#### Scenario: Text log format
- GIVEN `observability.logging.format: "text"`
- WHEN log entry is written
- THEN output is human-readable text format

---

### Requirement: Graceful Shutdown

可观察模块的优雅关闭。

#### Scenario: Flush traces on shutdown
- GIVEN Devrix receives SIGTERM
- WHEN shutdown is initiated
- THEN TracerProvider.Shutdown is called
- AND all in-progress spans are ended
- AND pending spans are exported

#### Scenario: Flush metrics on shutdown
- GIVEN Devrix receives SIGTERM
- WHEN shutdown is initiated
- THEN MetricsExporter.Shutdown is called
- AND all pending metric batches are sent

#### Scenario: Shutdown timeout
- GIVEN shutdown is initiated
- WHEN TracerProvider.Shutdown hangs
- THEN shutdown fails after 5 second timeout
- AND warning is logged

---

### Requirement: Health Endpoint

可观察模块自身健康状态。

#### Scenario: Health check includes observability status
- GIVEN health check is requested
- WHEN `GET /health` is called
- THEN response includes:
  - `status`: "healthy" | "degraded" | "unhealthy"
  - `components.tracer.status`: current tracer state
  - `components.tracer.exported_spans`: count
  - `components.metrics.status`: current metrics state
  - `components.metrics.collected_metrics`: count

#### Scenario: Unhealthy tracer does not affect app
- GIVEN tracer fails to export
- WHEN span is created
- THEN span is still recorded locally
- AND app continues to function
- AND health check shows tracer as degraded

---

## MODIFIED

| Item | Change | Reason |
|------|--------|--------|
| Trace Events → Trace/Span Data Model | 重构为 OTel Span 模型 | 对齐 OpenTelemetry 标准 |
| Metrics Collection | 增加 llm_errors_total, context_tokens_gauge | 完善可观察性 |
| Structured Logging | 增加 secret redaction, sampling | 生产环境安全 |
| Trace ID Propagation | 使用 W3C Trace Context 标准 | 跨系统互通性 |

---

## REMOVED

| Item | Reason |
|------|--------|
| Agent Fork/Merge trace events | V3 feature |
| Verify Pass/Fail trace events | V3 feature |
| Jaeger/Zipkin native exporters | V2+ use OTLP instead |
| Custom traceId format | Replaced with W3C 32-char hex |

---

## Technical Notes

### File Structure

```
internal/layers/observability/
├── tracer/
│   ├── tracer.go           # TracerProvider, Span creation
│   ├── span.go             # Span implementation
│   ├── propagation.go      # W3C TraceContext inject/extract
│   ├── sampler.go          # Sampling strategies
│   └── context.go          # SpanContext in Go context
├── metrics/
│   ├── meter.go            # MeterProvider, metric instruments
│   ├── counter.go          # Counter implementation
│   ├── histogram.go        # Histogram implementation
│   ├── gauge.go            # Gauge implementation
│   ├── registry.go         # Metric registry, label validation
│   └── prometheus.go       # Prometheus exporter
├── logger/
│   ├── logger.go           # StructuredLogger
│   ├── handler.go          # Log handlers (JSON, text)
│   └── redactor.go         # Secret redaction
├── exporter/
│   ├── console.go          # Console span/log exporter
│   ├── otlp.go             # OTLP gRPC/HTTP exporter
│   └── null.go             # No-op exporter (disabled)
├── config.go               # Config structs
├── observability.go        # Facade, initialization
├── health.go               # Health checks
└── shutdown.go             # Graceful shutdown
```

### Config Schema

```yaml
observability:
  enabled: true

  tracing:
    enabled: true
    service_name: "devrix"
    service_version: "1.0.0"
    exporter: "console"  # console | otlp | null
    sampling:
      type: "always_on"  # always_on | always_off | trace_id_ratio
      rate: 1.0           # 0.0-1.0, used when type is trace_id_ratio
    otlp:
      endpoint: "localhost:4317"
      insecure: true

  metrics:
    enabled: true
    exporter: "prometheus"  # prometheus | otlp | null
    endpoint: "/metrics"
    labels:
      allowlist:
        - provider
        - model
        - adapter
        - tool
        - risk_level
        - status
        - direction
      blocklist:
        - session_id
        - user_id

  logging:
    enabled: true
    level: "info"          # debug | info | warn | error
    format: "json"         # json | text
    include_trace_id: true
    sampling:
      enabled: true
      max_entries_per_span: 100
```

### Key Interfaces

```go
// Span represents an OpenTelemetry-compatible span
type Span interface {
    SpanContext() SpanContext
    SetAttribute(key string, value interface{})
    SetStatus(code StatusCode, description string)
    RecordError(err error)
    AddEvent(name string, attributes map[string]interface{})
    End()
}

// SpanContext contains trace identification
type SpanContext struct {
    TraceID    TraceID
    SpanID     SpanID
    TraceFlags uint8
    TraceState TraceState
    Remote     bool
}

// Tracer creates spans
type Tracer interface {
    StartSpan(name string, opts ...SpanOption) (Span, context.Context)
    TracerProvider() TracerProvider
}

// Meter creates metric instruments
type Meter interface {
    NewCounter(name string, opts ...MetricOption) Counter
    NewHistogram(name string, opts ...MetricOption) Histogram
    NewGauge(name string, opts ...MetricOption) Gauge
}

// StructuredLogger provides trace-aware logging
type StructuredLogger interface {
    Debug(msg string, args ...interface{})
    Info(msg string, args ...interface{})
    Warn(msg string, args ...interface{})
    Error(msg string, args ...interface{})
    With(args ...interface{}) StructuredLogger
}

// MetricRegistry validates label cardinality
type MetricRegistry interface {
    Register(metric Metric) error
    Get(name string) (Metric, bool)
    List() []Metric
    ValidateLabels(labels map[string]string) error
}

### Metric Definitions

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| devrix_llm_tokens_total | Counter | provider, model, direction | Total LLM tokens |
| devrix_llm_latency_seconds | Histogram | provider, model | LLM call latency |
| devrix_llm_errors_total | Counter | provider, model, error_type | LLM call errors |
| devrix_tool_calls_total | Counter | tool, risk_level, status | Tool execution count |
| devrix_session_active | Gauge | adapter | Active sessions |
| devrix_permission_timeouts_total | Counter | - | Permission timeouts |
| devrix_permission_decisions_total | Counter | decision | Permission decisions |
| devrix_context_tokens_current | Gauge | session_id (truncated) | Current context tokens |

### Span Naming Convention

| Operation | Span Name | Attributes |
|-----------|-----------|------------|
| Message received | `message.receive` | adapter, session_id |
| LLM chat | `llm.chat` | provider, model, tokens |
| Tool execution | `tool.execute` | tool_name, risk_level |
| Permission request | `permission.request` | tool_name, timeout |
| Context compression | `context.compress` | before_tokens, after_tokens |
| Session create | `session.create` | adapter |
| Session expire | `session.expire` | session_id, reason |

### W3C TraceContext Format

Traceparent header format:
```
traceparent: 00-{TraceID}-{SpanID}-{TraceFlags}
```

Example:
```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00abd067c2d9ab65-01
```

TraceFlags:
- `01`: Sampled flag (span should be recorded)
- `00`: Not sampled

### Sampling Strategies

| Type | Description | Use Case |
|------|-------------|----------|
| always_on | All spans are recorded | Development, debugging |
| always_off | No spans are recorded | Performance-critical paths |
| trace_id_ratio | Sample based on traceId hash | Production with high volume |

### Dependencies (Go)

```go
// Core OTel
go.opentelemetry.io/otel

// OTLP Exporters
go.opentelemetry.io/otel/exporters/otlp/otlptrace
go.opentelemetry.io/otel/exporters/otlp/otlpmetric

// Prometheus
github.com/prometheus/client_golang

// Logging (using slog - builtin)
log/slog
```

### Backward Compatibility Notes

V1 implementations using custom traceId format `{adapterId}:{sessionId}:{messageId}` should be migrated to W3C format. A migration script will be provided in V2.

For environments requiring legacy traceId format, a configuration option `tracing.legacy_trace_id_format: true` will be available in V2 as a temporary bridge.

---

