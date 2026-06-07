# Observability Layer Specification

**Change ID:** devrix-observability
**Spec Version:** 1.0.0
**Status:** Draft
**Based on:** `openspec/specs/observability_layer_delta.md`

---

## 概述

本文档定义可观察层的详细验收规格，与 `observability_layer_delta.md` 中的 Requirements 对应。

---

## 1. Trace/Span Data Model

### 1.1 Span 创建

#### Gherkin: Create root span on message arrival

```gherkin
Feature: Root Span Creation

  Scenario: Create root span on message arrival
    Given a new message arrives at Communication Layer
    When RouteInbound is called
    Then a root span is created with:
      | Attribute | Value |
      | traceId | 32-char hex string (W3C standard) |
      | spanId | 16-char hex string |
      | parentSpanId | empty (root span) |
      | service.name | "devrix" |
      | service.version | from devrix.yaml |
      | session.id | from message |
    And span start timestamp is recorded
```

#### Gherkin: Create child span for LLM call

```gherkin
Feature: Child Span for LLM

  Scenario: Create child span for LLM call
    Given a root span exists with traceId
    When LLMGateway.chat is called
    Then a child span is created with:
      | Attribute | Value |
      | parentSpanId | from parent span |
      | span.name | "llm.chat" |
      | llm.provider | provider name |
      | llm.model | model name |
      | llm.tokens.input | input token count |
      | llm.tokens.output | output token count |
      | llm.latency_ms | call duration |
```

### 1.2 Span 状态

#### Gherkin: End span with status

```gherkin
Feature: Span Lifecycle

  Scenario: End span with status
    Given a span is active
    When operation completes
    Then span is ended with:
      | Field | Value |
      | end timestamp | current time |
      | span.status.code | Unset or Ok or Error |
      | span.status.description | error message if Error |
```

#### Gherkin: Record exception in span

```gherkin
  Scenario: Record exception in span
    Given a span is active and an error occurs
    When RecordError is called
    Then span is updated with:
      | Field | Value |
      | span.events[-1].name | "exception" |
      | span.events[-1].attributes.exception.type | error type |
      | span.events[-1].attributes.exception.message | error message |
      | span.events[-1].attributes.exception.stacktrace | stack trace |
      | span.status.code | Error |
```

---

## 2. Span Lifecycle

### 2.1 Parent-Child Relationship

#### Gherkin: Span starts with correct parent

```gherkin
Feature: Span Parent-Child

  Scenario: Span starts with correct parent
    Given a context with parent SpanContext
    When tracer.StartSpan is called
    Then new span's parent is the given SpanContext
    And traceId is inherited from parent
```

#### Gherkin: Span starts with no parent

```gherkin
  Scenario: Span starts with no parent (root)
    Given a context with no SpanContext
    When tracer.StartSpan is called
    Then new span is a root span
    And new traceId is generated
```

### 2.2 Context Propagation

#### Gherkin: Span context propagation via Context

```gherkin
  Scenario: Span context propagation via Context
    Given a span is active
    When context is passed to child operation
    Then child inherits span via context.WithSpan
    And span can be retrieved via SpanFromContext
```

---

## 3. Trace ID Propagation

### 3.1 W3C Trace Context

#### Gherkin: Generate trace ID on entry

```gherkin
Feature: Trace ID Generation

  Scenario: Generate trace ID on entry
    Given a request enters via CLI adapter
    When RouteInbound is called
    Then traceId is generated as:
      """
      Format: 32-char lowercase hex (W3C Traceparent compatible)
      Example: 4bf92f3577b34da6a3ce929d0e0e4736
      """
    Or traceId is extracted from incoming traceparent header if present
```

#### Gherkin: Extract trace ID from incoming traceparent header

```gherkin
  Scenario: Extract trace ID from incoming traceparent header
    Given an incoming request has traceparent header
    When headers are parsed
    Then traceId is extracted from header format:
      """
      00-{traceId}-{spanId}-{flags}
      """
    And span is created with that traceId
    And incoming spanId becomes parent
```

### 3.2 Propagation Through Layers

#### Gherkin: Propagate trace ID through layers

```gherkin
Feature: Trace Propagation

  Scenario: Propagate trace ID through layers
    Given traceId exists at Communication Layer entry
    When message flows through layers (Context Engine, LLM Gateway, Tool Registry)
    Then traceId is passed via Go context.Context
    And traceId is included in all log entries
```

#### Gherkin: Inject trace ID into outgoing request headers

```gherkin
  Scenario: Inject trace ID into outgoing request headers
    Given trace context exists
    When outgoing HTTP/gRPC request is made
    Then traceparent header is injected
    And tracestate header is injected if baggage exists
```

---

## 4. Metrics Collection

### 4.1 LLM Metrics

#### Gherkin: Track llm_tokens_total counter

```gherkin
Feature: LLM Metrics

  Scenario: Track llm_tokens_total counter
    Given LLM call completes
    When response is received
    Then Counter devrix_llm_tokens_total is incremented
    And labels are:
      | Label | Values |
      | provider | anthropic, deepseek, openai |
      | model | model name |
      | direction | input, output |
```

#### Gherkin: Track llm_latency_seconds histogram

```gherkin
  Scenario: Track llm_latency_seconds histogram
    Given LLM call completes
    When response is received
    Then Histogram devrix_llm_latency_seconds is recorded
    And labels: provider, model
    And bucket bounds: [0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, +Inf]
```

#### Gherkin: Track llm_errors_total counter

```gherkin
  Scenario: Track llm_errors_total counter
    Given LLM call fails
    When error is returned
    Then Counter devrix_llm_errors_total is incremented
    And labels: provider, model, error_type
```

### 4.2 Tool Metrics

#### Gherkin: Track tool_calls_total counter

```gherkin
Feature: Tool Metrics

  Scenario: Track tool_calls_total counter
    Given tool execution completes
    When ToolRegistry.execute returns
    Then Counter devrix_tool_calls_total is incremented
    And labels: tool, risk_level, status (success | error)
```

### 4.3 Session Metrics

#### Gherkin: Track session_active gauge

```gherkin
Feature: Session Metrics

  Scenario: Track session_active gauge
    Given session is created
    When CreateSession succeeds
    Then Gauge devrix_session_active is incremented

  Scenario: Track session_active gauge decrements
    Given an active session
    When ExpireSession is called
    Then Gauge devrix_session_active is decremented
```

### 4.4 Permission Metrics

#### Gherkin: Track permission_timeouts counter

```gherkin
Feature: Permission Metrics

  Scenario: Track permission_timeouts counter
    Given permission request times out
    When timeout handler is called
    Then Counter devrix_permission_timeouts_total is incremented

  Scenario: Track permission_decisions counter
    Given user responds to permission
    When response is processed
    Then Counter devrix_permission_decisions_total is incremented
    And labels: decision (approved | denied)
```

### 4.5 Cardinality Control

#### Gherkin: Label cardinality is controlled

```gherkin
Feature: Metric Cardinality

  Scenario: Label cardinality is controlled
    Given metric labels are configured
    When label value has high cardinality (e.g., session ID)
    Then label is NOT added to metric
    And warning is logged if debug enabled
```

---

## 5. Metrics Exporter

### 5.1 Prometheus Endpoint

#### Gherkin: Prometheus endpoint returns metrics

```gherkin
Feature: Metrics Exporter

  Scenario: Prometheus endpoint returns metrics
    Given metrics are collected
    When GET /metrics is called
    Then response is in Prometheus exposition format
    And includes TYPE and HELP comments
    And includes all registered metrics
```

#### Gherkin: Metrics endpoint with authentication

```gherkin
  Scenario: Metrics endpoint with authentication
    Given auth is configured for metrics endpoint
    When GET /metrics is called without token
    Then 401 Unauthorized is returned
    And no metrics are exposed
```

---

## 6. Structured Logging

### 6.1 Trace Context in Logs

#### Gherkin: Log entry includes trace context

```gherkin
Feature: Structured Logging

  Scenario: Log entry includes trace context
    Given logger is called
    When logger.Info or logger.Error is invoked
    Then log entry includes:
      | Field | Description |
      | timestamp | ISO8601 |
      | level | INFO, WARN, ERROR |
      | message | the log message |
      | traceId | 32-char hex |
      | spanId | 16-char hex |
      | component | layer name |
      | service | "devrix" |
```

### 6.2 Log Level Filtering

#### Gherkin: Log level filtering

```gherkin
  Scenario: Log level filtering
    Given log level is set to INFO
    When logger.Debug is called
    Then log is not output (filtered)
    And logger.Info, logger.Warn, logger.Error are output
```

### 6.3 Error Logging

#### Gherkin: Error log includes stack trace

```gherkin
  Scenario: Error log includes stack trace
    Given an error occurs
    When logger.Error is called with error
    Then log entry includes:
      | Field | Description |
      | error | error message |
      | stacktrace | formatted stack trace |
      | error.type | error type name |
```

### 6.4 Security

#### Gherkin: Secret redaction in logs

```gherkin
  Scenario: Secret redaction in logs
    Given log contains sensitive fields
    When logger formats message
    Then keys matching [password, token, secret, api_key] are redacted
    And value is replaced with [REDACTED]
```

---

## 7. Configuration

### 7.1 Observability Toggle

#### Gherkin: Observability disabled

```gherkin
Feature: Configuration

  Scenario: Observability disabled
    Given observability.enabled is false
    When Devrix starts
    Then no tracing, metrics, or structured logging is initialized
    And application functions normally without observability overhead
```

### 7.2 Tracing Config

#### Gherkin: Tracing enabled with console exporter

```gherkin
  Scenario: Tracing enabled with console exporter
    Given observability.tracing.enabled is true
    And observability.tracing.exporter is "console"
    When Devrix starts
    Then spans are printed to stdout in JSON format
```

#### Gherkin: Sampling rate configuration

```gherkin
  Scenario: Sampling rate configuration
    Given observability.tracing.sampling.rate is 0.1
    When span is created
    Then 10% of spans are sampled
    And traceId ending determines sampling decision
```

### 7.3 Metrics Config

#### Gherkin: Metrics enabled with Prometheus exporter

```gherkin
  Scenario: Metrics enabled with Prometheus exporter
    Given observability.metrics.enabled is true
    And observability.metrics.exporter is "prometheus"
    And observability.metrics.endpoint is "/metrics"
    When Devrix starts
    Then Prometheus endpoint is registered
    And metrics are served at configured path
```

---

## 8. Graceful Shutdown

### 8.1 Trace Flush

#### Gherkin: Flush traces on shutdown

```gherkin
Feature: Graceful Shutdown

  Scenario: Flush traces on shutdown
    Given Devrix receives SIGTERM
    When shutdown is initiated
    Then TracerProvider.Shutdown is called
    And all in-progress spans are ended
    And pending spans are exported
```

### 8.2 Shutdown Timeout

#### Gherkin: Shutdown timeout

```gherkin
  Scenario: Shutdown timeout
    Given shutdown is initiated
    When TracerProvider.Shutdown hangs
    Then shutdown fails after 5 second timeout
    And warning is logged
```

---

## 9. Health Endpoint

### 9.1 Health Check

#### Gherkin: Health check includes observability status

```gherkin
Feature: Health Endpoint

  Scenario: Health check includes observability status
    Given health check is requested
    When GET /health is called
    Then response includes:
      | Field | Description |
      | status | healthy, degraded, or unhealthy |
      | components.tracer.status | current tracer state |
      | components.tracer.exported_spans | count |
      | components.metrics.status | current metrics state |
      | components.metrics.collected_metrics | count |
```

#### Gherkin: Unhealthy tracer does not affect app

```gherkin
  Scenario: Unhealthy tracer does not affect app
    Given tracer fails to export
    When span is created
    Then span is still recorded locally
    And app continues to function
    And health check shows tracer as degraded
```

---

## 附录：Golden Test Cases

### A.1 Happy Path Trace

```json
{
  "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
  "spans": [
    {
      "spanId": "a1b2c3d4e5f60718",
      "parentSpanId": "",
      "name": "message.receive",
      "status": "Ok"
    },
    {
      "spanId": "b2c3d4e5f60718a1",
      "parentSpanId": "a1b2c3d4e5f60718",
      "name": "llm.chat",
      "attributes": {
        "llm.provider": "anthropic",
        "llm.model": "claude-3-5-sonnet"
      },
      "status": "Ok"
    }
  ]
}
```

### A.2 Prometheus Output

```
# HELP devrix_llm_tokens_total Total LLM tokens
# TYPE devrix_llm_tokens_total counter
devrix_llm_tokens_total{direction="input",model="claude-3-5-sonnet",provider="anthropic"} 1234
devrix_llm_tokens_total{direction="output",model="claude-3-5-sonnet",provider="anthropic"} 567

# HELP devrix_llm_latency_seconds LLM call latency
# TYPE devrix_llm_latency_seconds histogram
devrix_llm_latency_seconds_bucket{le="1.0",model="claude-3-5-sonnet",provider="anthropic"} 5
devrix_llm_latency_seconds_bucket{le="+Inf",model="claude-3-5-sonnet",provider="anthropic"} 10
devrix_llm_latency_seconds_sum{model="claude-3-5-sonnet",provider="anthropic"} 8.5
devrix_llm_latency_seconds_count{model="claude-3-5-sonnet",provider="anthropic"} 10
```

### A.3 JSON Log Output

```json
{
  "timestamp": "2026-06-07T10:00:00.000Z",
  "level": "INFO",
  "message": "LLM call completed",
  "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
  "spanId": "b2c3d4e5f60718a1",
  "component": "llm_gateway",
  "service": "devrix",
  "version": "1.0.0",
  "llm": {
    "provider": "anthropic",
    "model": "claude-3-5-sonnet",
    "tokens": 1234
  }
}
```
