# Delta: Observability Layer (Layer 5)

**Change ID:** devrix-foundation
**Affects:** observability, tracing, metrics, logging

---

## ADDED

### Requirement: Trace Events

Structured event emission for debugging.

#### Scenario: Emit im_message_received
- GIVEN message arrives at Communication Layer
- WHEN processMessage is called
- THEN trace event is emitted:
  - traceId: 'adapterId:sessionId:messageId'
  - event: 'im_message_received'
  - timestamp: ISO8601

#### Scenario: Emit llm_call
- GIVEN LLM is about to be called
- WHEN LLMGateway.chat is called
- THEN trace event is emitted:
  - event: 'llm_call'
  - provider, model, tokenCount

#### Scenario: Emit tool_execution
- GIVEN tool is about to execute
- WHEN ToolRegistry.execute is called
- THEN trace event is emitted:
  - event: 'tool_execution'
  - toolName, args, riskLevel

#### Scenario: Emit permission_request
- GIVEN permission is needed for tool
- WHEN PermissionPipeline.request is called
- THEN trace event is emitted:
  - event: 'permission_request'
  - toolName, riskLevel, timeout

#### Scenario: Emit permission_response
- GIVEN user responds to permission
- WHEN PermissionPipeline.resolve is called
- THEN trace event is emitted:
  - event: 'permission_response'
  - granted: boolean, responseTime

---

### Requirement: Metrics Collection

Quantitative measurements for monitoring.

#### Scenario: Track llm_tokens_total
- GIVEN LLM call completes
- WHEN response is received
- THEN Counter metric 'llm_tokens_total' is incremented
- AND labels: provider, model, direction (input/output)

#### Scenario: Track llm_latency_seconds
- GIVEN LLM call completes
- WHEN response is received
- THEN Histogram metric 'llm_latency_seconds' is recorded
- AND labels: provider, model

#### Scenario: Track tool_calls_total
- GIVEN tool execution completes
- WHEN ToolRegistry.execute returns
- THEN Counter metric 'tool_calls_total' is incremented
- AND labels: toolName, riskLevel

#### Scenario: Track session_active gauge
- GIVEN session is created
- WHEN session is created
- THEN Gauge 'session_active' is incremented
- AND when session expires, it is decremented

#### Scenario: Track permission_timeouts
- GIVEN permission request times out
- WHEN PermissionPipeline.handleTimeout is called
- THEN Counter metric 'permission_timeouts' is incremented

---

### Requirement: Structured Logging

JSON logging with trace context.

#### Scenario: Log with trace context
- GIVEN log statement is executed
- WHEN logger.info/error/debug is called
- THEN log entry includes:
  ```json
  {
    "timestamp": "2026-06-06T10:00:00.000Z",
    "level": "INFO",
    "traceId": "cli:session123:msg456",
    "component": "Gateway",
    "message": "Message received"
  }
  ```

#### Scenario: Log component identification
- GIVEN log comes from different layer
- WHEN logger is instantiated in component
- THEN component name is automatically added
- AND distinguishes Gateway, Engine, Agent, etc.

#### Scenario: Log level filtering
- GIVEN log level is set to INFO
- WHEN DEBUG level log is emitted
- THEN log is not output (filtered)

---

### Requirement: Trace ID Propagation

End-to-end request tracing.

#### Scenario: Generate trace ID
- GIVEN new message arrives
- WHEN CommunicationLayer processes it
- THEN traceId is generated: '{adapterId}:{sessionId}:{messageId}'

#### Scenario: Propagate trace ID
- GIVEN traceId exists at entry
- WHEN message flows through layers
- THEN traceId is passed to each layer
- AND included in all logs and events

#### Scenario: No trace ID (initial)
- GIVEN request has no trace ID
- WHEN it enters the system
- THEN new traceId is generated
- AND trace is treated as root

---

## MODIFIED

(None - initial layer specification)

---

## REMOVED

| Item | Reason |
|------|--------|
| Agent Fork/Merge trace events | V3 feature |
| Verify Pass/Fail trace events | V3 feature |
| Distributed tracing (Jaeger/Zipkin) | Future feature |
