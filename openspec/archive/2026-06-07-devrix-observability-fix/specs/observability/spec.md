# Observability Layer Fix Specification

**Change ID:** devrix-observability-fix
**Parent Spec:** `openspec/archive/2026-06-07-devrix-observability/specs/observability/spec.md`
**Status:** Draft
**Version:** 1.1.0

> 本文档仅记录**变更和新增**的验收场景。不变的部分由 parent spec 覆盖。

---

## 1. Gauge 数值正确性（新增）

### 1.1 Float64 操作精度

```gherkin
Feature: Gauge Float64 Precision

  Scenario: Gauge Set then Value returns exact value
    Given a Gauge is created with name "test"
    When Set(1.5) is called
    Then Value() returns 1.5 exactly

  Scenario: Gauge Add accumulates correctly
    Given a Gauge with initial value 0.0
    When Add(1.25) and Add(0.25) are called
    Then Value() returns 1.5 exactly

  Scenario: Gauge Sub subtracts correctly
    Given a Gauge with initial value 5.0
    When Sub(1.75) is called
    Then Value() returns 3.25 exactly

  Scenario: Gauge Inc/Dec on integer values
    Given a Gauge with initial value 0.0
    When Inc() and Inc() and Dec() are called
    Then Value() returns 1.0 exactly

  Scenario: Gauge Set large value preserves precision
    Given a Gauge is created
    When Set(9007199254740992.0) is called
    Then Value() returns 9007199254740992.0 exactly
```

---

### 1.2 AsyncGauge 并发安全

```gherkin
Feature: AsyncGauge Concurrency Safety

  Scenario: Register callback is safe during observe
    Given an AsyncGauge is created
    And a goroutine is running Observe() in a loop
    When Unregister() is called to remove the callback
    Then no race condition occurs

  Scenario: Observe serializes callbacks
    Given an AsyncGauge with 2 registered callbacks
    When Observe() is called concurrently from 3 goroutines
    Then each callback execution is serialized via mutex
    And the final observed value matches the last callback's return
```

---

## 2. Histogram 桶计数正确性（新增）

### 2.1 非累积存储 + 累积输出

```gherkin
Feature: Histogram Bucket Correctness

  Scenario: Single observation falls into correct bucket
    Given a Histogram with bounds [0.1, 0.5, 1.0]
    When Observe(0.05) is called
    Then Prometheus output has:
      | le=0.1 | 1 |
      | le=0.5 | 1 |
      | le=1.0 | 1 |
      | le=+Inf | 1 |

  Scenario: Multiple observations produce correct cumulative view
    Given a Histogram with bounds [0.1, 0.5, 1.0]
    When Observe(0.05), Observe(0.3), Observe(0.8) are called
    Then Prometheus output has:
      | le=0.1 | 1 |
      | le=0.5 | 2 |
      | le=1.0 | 3 |
      | le=+Inf | 3 |
    And sum equals 1.15
    And count equals 3

  Scenario: Observation above all bounds lands only in +Inf
    Given a Histogram with bounds [0.1, 0.5, 1.0]
    When Observe(5.0) is called
    Then Prometheus output has:
      | le=0.1 | 0 |
      | le=0.5 | 0 |
      | le=1.0 | 0 |
      | le=+Inf | 1 |

  Scenario: Empty histogram outputs zero buckets
    Given a Histogram with bounds [0.1, 0.5, 1.0]
    When no observations are made
    Then Prometheus output has all bucket values as 0
    And count equals 0
    And sum equals 0

  Scenario: Buckets() returns correct non-cumulative +Inf value
    Given a Histogram with bounds [0.1, 0.5, 1.0]
    When Observe(0.05) and Observe(5.0) are called
    Then Buckets() returns:
      | +Inf | 2 |
      | 0.1  | 1 |
      | 0.5  | 0 |
      | 1.0  | 0 |
    And Buckets()[+Inf] equals Count() (invariant)
```

---

## 3. Graceful Shutdown（修改）

### 3.1 刷写 Pending Spans

```gherkin
Feature: Shutdown Flush

  Scenario: Shutdown ends all active spans
    Given 3 spans are started but not ended
    When TracerProvider.Shutdown is called
    Then all 3 spans are ended
    And all 3 spans are exported

  Scenario: Shutdown shuts down exporter
    Given an exporter is registered
    When TracerProvider.Shutdown is called
    Then exporter.Shutdown is called exactly once

  Scenario: Shutdown after span end is no-op for that span
    Given a span is started and ended
    When TracerProvider.Shutdown is called
    Then no duplicate export occurs

  Scenario: Span started during shutdown is ended and exported
    Given TracerProvider.Shutdown is called concurrently
    And a goroutine calls Start() repeatedly
    When shutdown completes
    Then all started spans are either never active or exported
    And no span is left in activeSpans after shutdown

  Scenario: Double-check closes race between Start and Shutdown
    Given a span is started (written to activeSpans)
    And TracerProvider.shutdown is set to true immediately after
    When Start() performs double-check
    Then the span is removed from activeSpans
    And the span is ended and exported
```

### 3.2 全组件 Shutdown

```gherkin
Feature: Full Component Shutdown

  Scenario: Shutdown covers Tracer, Logger
    Given Observability with Tracer and Logger enabled
    When Observability.Shutdown is called
    Then TracerProvider.Shutdown is called
    And Logger is flushed

  Scenario: Shutdown with only Tracer enabled
    Given Observability with Tracer enabled, Logger disabled
    When Observability.Shutdown is called
    Then TracerProvider.Shutdown is called
    And no error is returned about Logger

  Scenario: Shutdown aggregates errors
    Given TracerProvider.Shutdown returns an error
    And Logger.Close returns an error
    When Observability.Shutdown is called
    Then the returned error contains both error messages
```

---

## 4. Int64UpDownCounter → Gauge（修改）

### 4.1 类型修正

```gherkin
Feature: UpDown Counter Returns Gauge

  Scenario: Int64UpDownCounter returns a Gauge
    Given a Meter is created
    When Int64UpDownCounter("active_sessions") is called
    Then the returned value implements Gauge interface
    And the returned value does NOT implement Counter interface

  Scenario: UpDown Gauge can go below zero
    Given an UpDown Gauge with initial value 5
    When Sub(10) is called
    Then Value() returns -5.0

  Scenario: SessionBridge.ActiveSessions returns Gauge
    Given a SessionBridge is created
    When ActiveSessions("cli") is called
    Then the returned value is a Gauge
```

---

## 5. Structured Logging（修改）

### 5.1 错误堆栈跟踪

```gherkin
Feature: Error Log Stack Trace

  Scenario: Error implementing StackTracer provides stack via interface
    Given an error implements StackTracer interface (e.g. pkg/errors)
    When logger.Error("operation failed", "error", err) is called
    Then the stacktrace field contains frames from err.StackTrace()
    And no runtime.Callers fallback is triggered

  Scenario: Plain error uses runtime.Callers fallback
    Given a plain fmt.Errorf("something broke") error
    When logger.Error("operation failed", "error", err) is called
    Then the log entry fields include:
      | error | "something broke" |
      | error_type | "*errors.errorString" |
      | stacktrace | formatted goroutine call stack from runtime.Callers |

  Scenario: Non-error key does not include stacktrace
    Given a log entry with a non-error key named "duration"
    When logger.Info("completed", "duration", 100*time.Millisecond) is called
    Then the log entry does NOT have a stacktrace field

  Scenario: Stack trace capture is bounded
    Given maxStackFrames is 32
    When an error with deep call stack is logged
    Then the stacktrace contains at most 32 frames
```

### 5.2 日志采样

```gherkin
Feature: Log Sampling

  Scenario: First N entries within a span are logged
    Given log sampling is enabled with max_entries_per_span=10
    And a span with id "abc123" is active
    When 5 log entries are emitted
    Then all 5 entries are logged

  Scenario: Entries beyond max are dropped
    Given log sampling is enabled with max_entries_per_span=10
    And a span with id "abc123" is active
    When 15 log entries are emitted
    Then first 10 entries are logged
    And entries 11-15 are dropped

  Scenario: First dropped entry triggers a warning
    Given log sampling is enabled with max_entries_per_span=10
    And a span with id "abc123" is active
    When the 11th log entry is emitted
    Then a WARN log entry is emitted indicating sampling active
    And that warning is NOT sampled itself

  Scenario: Different spans have independent counters
    Given log sampling with max_entries_per_span=10
    And spans with id "span-a" and "span-b" are active
    When 10 entries are emitted for span-a
    Then entries for span-b are still logged (not affected)

  Scenario: Tracker evicts oldest span when exceeding maxTrackedSpans
    Given maxTrackedSpans is set to 2
    And spans "span-a", "span-b" have tracked entries below max
    When span "span-c" first exceeds max_entries_per_span
    Then one of the under-limit spans ("span-a" or "span-b") is evicted from tracker
    And the tracker's span count never exceeds maxTrackedSpans

  Scenario: Tracker unbounded growth is prevented
    Given log sampling is enabled
    When 20,000 unique spans each emit 1 entry
    Then the tracker's internal map never exceeds 10,000 entries
```

### 5.3 Logger-Tracer 依赖解耦

```gherkin
Feature: Logger Tracer Decoupling

  Scenario: WithTrace accepts string parameters
    Given a StructuredLogger
    When WithTrace("trace123", "span456") is called
    Then the returned logger includes fields "traceId"="trace123" and "spanId"="span456"
    And the logger package does not import the tracer package

  Scenario: WithTrace is backward compatible for callers
    Given the observability package is the sole caller of WithTrace
    When the signature is changed from (sc SpanContext) to (traceID, spanID string)
    Then the observability.go call site is updated to pass sc.TraceID.String(), sc.SpanID.String()
```

---

## 6. 接口一致性（新增）

### 6.1 ConsoleExporter 实现 SpanExporter

```gherkin
Feature: ConsoleExporter Compliance

  Scenario: ConsoleExporter implements SpanExporter
    Given a ConsoleExporter is created
    Then it can be assigned to a SpanExporter variable without compilation error

  Scenario: ConsoleExporter Export accepts context
    Given a ConsoleExporter and a context.Background()
    When Export(ctx, readableSpan) is called
    Then the method signature matches SpanExporter.Export
```

---

## 7. 代码质量（新增）

### 7.1 死代码清理

```gherkin
Feature: Dead Code Removal

  Scenario: Counter has no unused mutex
    Given the counter struct is inspected
    Then it does not contain an unused sync.Mutex field

  Scenario: NullExporter has no orphan methods
    Given NullExporter is inspected
    Then it only has methods defined in SpanExporter interface

  Scenario: Registry sortedKeys uses sort.Strings
    Given the sortedKeys function is inspected
    Then it calls sort.Strings from the standard library
```

### 7.2 Registry.Reset 全类型

```gherkin
Feature: Registry Full Reset

  Scenario: Reset clears all counter values
    Given a Counter registered with value 10
    When Reset() is called
    Then the Counter value is 0

  Scenario: Reset clears all gauge values
    Given a Gauge registered with value 5.5
    When Reset() is called
    Then the Gauge value is 0.0

  Scenario: Reset clears all histogram data
    Given a Histogram with 3 observations
    When Reset() is called
    Then Histogram.Count() returns 0
    And Histogram.Sum() returns 0
    And all bucket values are 0
```

### 7.3 Handler.Close 接口

```gherkin
Feature: Handler Close Interface

  Scenario: Handler interface includes Close method
    Given the Handler interface is defined
    Then it has a Close() error method

  Scenario: JSONHandler Close returns nil
    Given a JSONHandler with a bytes.Buffer
    When Close() is called
    Then no error is returned

  Scenario: StructuredLogger.Close delegates to Handler
    Given a StructuredLogger with a mock Handler
    When Close() is called
    Then handler.Close() is called exactly once

  Scenario: Logger Close is safe when handler is nil
    Given a StructuredLogger with no handler
    When Close() is called
    Then no panic occurs
```

---

## 附录：修复前后对比

### A.1 Gauge 转换

修复前：
```
Set(1.5) → uint64(f + 9.22e18) → 位模式被截断 → Value() 返回错误值
```

修复后：
```
Set(1.5) → math.Float64bits(1.5) → 精确位模式 → math.Float64frombits → 1.5
```

### A.2 Histogram 输出

修复前：
```
# 输入: [0.05, 0.3, 0.8]
le=0.1 → 3    (错误，应为 1)
le=0.5 → 5    (错误，应为 2)
le=1.0 → 6    (错误，应为 3)
```

修复后：
```
# 输入: [0.05, 0.3, 0.8]
le=0.1 → 1    ✅
le=0.5 → 2    ✅
le=1.0 → 3    ✅
```

### A.3 Shutdown 流程

修复前：
```
Shutdown() → shutdown=true → 返回 nil (pending spans 丢失)
```

修复后：
```
Shutdown() → 遍历 activeSpans → 每个 span.End() → exporter.Shutdown() → 返回
```
