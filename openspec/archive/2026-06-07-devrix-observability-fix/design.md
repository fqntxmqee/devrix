# Observability Layer Fix Design

**Change ID:** devrix-observability-fix
**Parent:** devrix-observability
**Status:** S2 Design
**Version:** 1.1.0
**Based on:** Code Review findings dated 2026-06-07

> **文档分工：** 问题发现与严重度 → `proposal.md`；修复方案与代码细节 → 本文档；验收规格 → `specs/observability/spec.md`；任务拆分 → `tasks.md`。

---

## 一、Critical Fix 详细设计

### 1.1 Gauge float64 原子转换修复

**文件：** `internal/layers/observability/metrics/gauge.go`

**问题根因：**

```go
// 当前错误实现 (gauge.go:82-88)
func float64ToUint64(f float64) uint64 {
    return uint64(int64(f + (1 << 63)))  // 精度丢失
}
```

float64 尾数只有 53 位，当 `f=1.5` 时 `1.5 + 9.22e18` 在 float64 中无法精确表示小数部分，截断后往返转换产生错误值。

**修复方案：**

```go
import "math"

func float64ToUint64(f float64) uint64 {
    return math.Float64bits(f)
}

func uint64ToFloat64(u uint64) float64 {
    return math.Float64frombits(u)
}
```

`math.Float64bits` 是 IEEE 754 标准规定的位模式复制，零开销且精确往返。但不支持原子 `Add`——Gauge 的 `Add` / `Sub` 操作需要改用 `sync.Mutex` 保护：

```go
type gauge struct {
    name   string
    labels LabelMap
    mu     sync.Mutex
    value  float64
}

func (g *gauge) Set(value float64) {
    g.mu.Lock()
    g.value = value
    g.mu.Unlock()
}

func (g *gauge) Add(value float64) {
    g.mu.Lock()
    g.value += value
    g.mu.Unlock()
}

func (g *gauge) Value() float64 {
    g.mu.Lock()
    defer g.mu.Unlock()
    return g.value
}
```

**性能影响：** Mutex 开销在 metrics 场景下可忽略（Gauge 通常只在 session 创建/销毁时调用，不是热路径）。Counter 保持 atomic 不变（整型 atomic 天然正确）。

**补充修复：AsyncGauge / AsyncCounter 回调竞态**

`async_gauge.go` 和 `async_counter.go` 的 `observe` / `run` 方法在回调执行时可能被并发调用，当前缺少互斥保护。注册和注销回调时也可能与执行产生竞态。

```go
// async_gauge.go — 添加 Mutex 保护回调执行
type asyncGauge struct {
    name      string
    labels    LabelMap
    mu        sync.Mutex
    callbacks []func(context.Context) (float64, error)
}
```

回调操作应在锁内执行：注册/注销/执行全部互斥，确保并发安全。AsyncCounter 同理。

---

### 1.2 Histogram 双重累积修复

**文件：** `internal/layers/observability/metrics/histogram.go` + `prometheus.go`

**问题根因：**

`Observe` 在一次调用中递增**所有** value ≤ bound 的桶：
```go
// 当前错误：value=0.05, bounds=[0.1,0.5,1.0]
// 结果: buckets[0.1]=1, buckets[0.5]=1, buckets[1.0]=1  (存储值已累积)
for _, bound := range h.bounds {
    if value <= bound {
        h.buckets[bound]++
    }
}
```

然后 `prometheus.go:Output()` 又做了一次 cumulative 求和：
```go
// 对已累积的值再做 cumulative → 双重累积
var cumulative uint64
for _, bound := range sortedBounds {
    cumulative += buckets[bound]  // buckets 本身已是累积值
}
```

导致输出：le=0.1→1, le=0.5→2, le=1.0→3 而非正确的 1,1,1。

**修复方案 A（推荐）：存储非累积值，Output 累积**

```go
// histogram.go — Observe 只递增第一个 value <= bound 的桶
func (h *histogram) Observe(value float64) {
    h.mu.Lock()
    defer h.mu.Unlock()

    h.count++
    h.sum += value

    for _, bound := range h.bounds {
        if value <= bound {
            h.buckets[bound]++
            break  // ← 关键：只递增第一个匹配桶
        }
    }
}
```

Prometheus Output 逻辑不变（累加非累积值产生累积视图，行为正确）。

`+Inf` 桶在 `Observe` 中直接递增以维持 `Buckets()` 返回正确数据（而非仅在 Output 中特殊处理）：

```go
// histogram.go — Observe 同时递增 count 桶
func (h *histogram) Observe(value float64) {
    h.mu.Lock()
    defer h.mu.Unlock()

    h.count++
    h.sum += value
    h.buckets[math.Inf(1)]++  // +Inf 桶 = count

    for _, bound := range h.bounds {
        if value <= bound {
            h.buckets[bound]++
            break
        }
    }
}
```

这样 `Buckets()` 方法始终返回非累积的正确值，Prometheus Output 仅做累加转换：

```go
// prometheus.go — 统一遍历
for _, bound := range sortedBounds {
    cumulative += buckets[bound]
    sb.WriteString(fmt.Sprintf("%d\n", cumulative))
}
```

**验证方法：** 构造已知输入序列，对比 Prometheus 输出与 golden 文件。

---

### 1.3 Shutdown 刷写 Pending Spans

**文件：** `internal/layers/observability/tracer/tracer.go`

**问题根因：**

```go
// 当前：Shutdown 只设置标志，不处理 activeSpans
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
    tp.shutdownMu.Lock()
    defer tp.shutdownMu.Unlock()
    if tp.shutdown { return nil }
    tp.shutdown = true
    return nil
}
```

Tracer 的 `activeSpans` map 中包含所有未 End 的 span，Shutdown 时这些数据直接丢失。

**修复方案：**

`TracerProvider` 需要持有对所有 Tracer 的引用，以便 Shutdown 时遍历：

```go
type TracerProvider struct {
    config     *settings.TracingConfig
    sampler    Sampler
    exporter   SpanExporter
    tracers    []*Tracer          // 新增：追踪所有 Tracer
    tracersMu  sync.Mutex
    shutdownMu sync.RWMutex
    shutdown   bool
}

func (tp *TracerProvider) Tracer(name string) *Tracer {
    t := &Tracer{
        name:     name,
        provider: tp,
        activeSpansMu: sync.RWMutex{},
        activeSpans:   make(map[SpanID]*span),
    }
    tp.tracersMu.Lock()
    tp.tracers = append(tp.tracers, t)
    tp.tracersMu.Unlock()
    return t
}

func (tp *TracerProvider) Shutdown(ctx context.Context) error {
    tp.shutdownMu.Lock()
    defer tp.shutdownMu.Unlock()
    if tp.shutdown { return nil }
    tp.shutdown = true

    // 结束所有 Tracer 中所有 active span
    tp.tracersMu.Lock()
    for _, t := range tp.tracers {
        t.activeSpansMu.Lock()
        for _, s := range t.activeSpans {
            s.End()
        }
        t.activeSpansMu.Unlock()
    }
    tp.tracersMu.Unlock()

    // 关闭 exporter
    if tp.exporter != nil {
        return tp.exporter.Shutdown(ctx)
    }
    return nil
}
```

**约束：** Span.End 必须是幂等的（当前代码已满足：`s.mu.Lock(); if !s.recording { s.mu.Unlock(); return }`）。

**补充修复：Start() 中 double-check 关闭状态**

`Tracer.Start()` 当前在写入 `activeSpans` 前检查 `isShutdown()`，但在写入 map 后（释放写锁前），Shutdown 可能刚好进入并读取到该 span。需在写入后再次检查：

```go
func (t *Tracer) Start(ctx context.Context, name string) *span {
    if t.provider.isShutdown() {
        return &span{recording: false}  // 已关闭，返回 noop span
    }

    s := newSpan(name, t)
    t.activeSpansMu.Lock()
    t.activeSpans[s.id] = s
    t.activeSpansMu.Unlock()

    // double-check: 写入后再次确认，覆盖竞态窗口
    if t.provider.isShutdown() {
        t.activeSpansMu.Lock()
        delete(t.activeSpans, s.id)
        t.activeSpansMu.Unlock()
        s.End()  // 幂等结束，触发 export
    }
    return s
}
```

此模式确保：Start 写入 activeSpans 后 / Shutdown 读取 activeSpans 之间的竞态窗口被 double-check 覆盖，不会丢失 span。

---

## 二、High Fix 详细设计

### 2.1 Int64UpDownCounter 返回 Gauge

**文件：** `internal/layers/observability/metrics/meter.go`

**当前错误：**
```go
func (m *Meter) Int64UpDownCounter(name string, opts ...CounterOption) (Counter, error) {
    counter := NewCounter(fullMetricName(m.name, name), cfg.Labels)  // Counter!
}
```

**修复：** 改为创建 Gauge 并注册到 Registry。

```go
func (m *Meter) Int64UpDownCounter(name string, opts ...CounterOption) (Gauge, error) {
    cfg := &CounterConfig{Labels: make(LabelMap)}
    for _, opt := range opts {
        if opt != nil { opt(cfg) }
    }

    gauge := NewGauge(fullMetricName(m.name, name), cfg.Labels)

    if err := m.provider.registry.RegisterGauge(
        fullMetricName(m.name, name), cfg.Labels, gauge,
    ); err != nil {
        return nil, err
    }
    return gauge, nil
}
```

**影响面：** 调用方 `SessionBridge.ActiveSessions` 当前返回 `Counter`，需要改为返回 `Gauge`：

```go
// bridge.go
func (b *SessionBridge) ActiveSessions(adapter string) (metrics.Gauge, error) {
    if b.bridge == nil || b.bridge.meter == nil {
        return nil, nil
    }
    return b.bridge.meter.Int64UpDownCounter("active_sessions",
        metrics.WithLabels(metrics.LabelMap{"adapter": adapter}))
}
```

---

### 2.2 Observability.Shutdown 覆盖全组件

**文件：** `internal/layers/observability/observability.go`

```go
func (o *Observability) Shutdown(ctx context.Context) error {
    o.mu.Lock()
    defer o.mu.Unlock()

    var errs []error

    // 1. Shutdown tracer provider (刷写 pending spans)
    if o.tracerProvider != nil {
        if err := o.tracerProvider.Shutdown(ctx); err != nil {
            errs = append(errs, fmt.Errorf("tracer shutdown: %w", err))
        }
    }

    // 2. Shutdown logger (flush buffer)
    if o.log != nil {
        if err := o.log.Close(); err != nil {
            errs = append(errs, fmt.Errorf("logger shutdown: %w", err))
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("shutdown errors: %v", errs)
    }
    return nil
}
```

Logger 需要增加 `Close()` 方法用于 flush handler 的 writer buffer。

---

### 2.3 删除 Counter 中的死 Mutex

**文件：** `internal/layers/observability/metrics/counter.go`

删除 `counter` 结构体中的 `mu sync.Mutex` 字段——所有操作已使用 `sync/atomic`，该 Mutex 从未被引用。

---

### 2.4 日志采样实现

**文件：** `internal/layers/observability/logger/logger.go`

`StructuredLogger` 需要追踪每个 Span 的日志条目数。通过 `LogEntry` 中的 `SpanID` 字段来分组：

```go
const maxTrackedSpans = 10_000

type spanLogTracker struct {
    mu      sync.Mutex
    counts  map[string]int     // spanId → count (最多 10,000 个 key)
    dropped map[string]int     // spanId → dropped count
    max     int
}

func newSpanLogTracker(max int) *spanLogTracker {
    return &spanLogTracker{
        counts:  make(map[string]int),
        dropped: make(map[string]int),
        max:     max,
    }
}

// shouldLog returns true if the entry should be logged for this span.
func (t *spanLogTracker) shouldLog(spanID string) (bool, int) {
    if spanID == "" || t.max <= 0 {
        return true, 0
    }
    t.mu.Lock()
    defer t.mu.Unlock()
    t.counts[spanID]++
    if t.counts[spanID] <= t.max {
        return true, 0
    }
    t.dropped[spanID]++
    dropped := t.dropped[spanID]
    // 容量保护：超过 maxTrackedSpans 个 span 时淘汰最旧条目
    if len(t.counts) > maxTrackedSpans {
        for k := range t.counts {
            if t.counts[k] <= t.max {
                delete(t.counts, k)
                delete(t.dropped, k)
                break
            }
        }
    }
    return false, dropped
}
```

`buildEntry` 集成：
```go
func (l *StructuredLogger) buildEntry(level LogLevel, msg string, args ...any) *LogEntry {
    // ... existing logic ...

    // 采样检查
    if l.sampler != nil {
        if spanID, ok := fields["spanId"].(string); ok {
            ok, dropped := l.sampler.shouldLog(spanID)
            if !ok {
                if dropped == 1 {
                    // 发出采样警告
                    entry.Message = fmt.Sprintf(
                        "log sampling threshold reached for span %s (first 100 entries kept, further entries dropped)", spanID[:8],
                    )
                    entry.Level = "WARN"
                } else {
                    return nil  // 丢弃条目
                }
            }
        }
    }
    return entry
}
```

---

### 2.5 Registry.Reset() 覆盖全类型

**文件：** `internal/layers/observability/metrics/registry.go`

当前 `Registry.Reset()` 仅重置 Counter：

```go
// 当前：只重置 counters，遗漏 gauges 和 histograms
func (r *Registry) Reset() {
    for _, c := range r.counters {
        c.Reset()
    }
}
```

修复：遍历所有已注册的 Gauge 和 Histogram 并调用 Reset：

```go
func (r *Registry) Reset() {
    r.mu.RLock()
    defer r.mu.RUnlock()
    for _, c := range r.counters {
        c.Reset()
    }
    for _, g := range r.gauges {
        g.Set(0)  // Gauge 的 "零值"
    }
    for _, h := range r.histograms {
        h.Reset()
    }
}
```

Gauge 和 Histogram 接口需要增加 `Set(0)` / `Reset()` 方法，Counter 已有。

---

### 2.6 Handler 接口扩展 Close()

**文件：** `internal/layers/observability/logger/logger.go`

Logger 的 `Close()` 需要 flush 底层 Handler 的 writer buffer。当前 Handler 接口没有 `Close()` 方法：

```go
// 当前
type Handler interface {
    Handle(entry *LogEntry) error
}
```

修复：扩展 Handler 接口，JSONHandler / TextHandler 实现空操作：

```go
type Handler interface {
    Handle(entry *LogEntry) error
    Close() error  // 新增：flush + 关闭底层 writer
}
```

`JSONHandler.Close()` 和 `TextHandler.Close()` 初始实现为 `return nil`。如将来底层 writer 需要 flush（如 bufio.Writer），在对应 Handler 的 Close 中处理。

StructuredLogger.Close() 委托给 Handler：

```go
func (l *StructuredLogger) Close() error {
    return l.handler.Close()
}
```

### 3.1 错误日志堆栈跟踪

**文件：** `internal/layers/observability/logger/logger.go`

在 `buildEntry` 中处理 error 值时追加堆栈。采用两级优先级策略：

**Priority 1 — StackTracer 接口（自动堆栈注入）：**

```go
// StackTracer 由支持堆栈跟踪的错误类型实现
type StackTracer interface {
    StackTrace() []uintptr
}
```

如果 error 实现了 `StackTracer`，直接提取其调用栈 pcs，通过 `runtime.CallersFrames` 格式化输出。

**Priority 2 — runtime.Callers 回退：**

如果 error 未实现 `StackTracer`，使用 `runtime.Callers(3, ...)` 捕获当前 goroutine 调用栈（skip=3 跳过 `buildEntry` / `log` / `runtime.Callers` 自身）。

```go
import "runtime"

const maxStackFrames = 32

func captureStack(err error) string {
    // Priority 1: StackTracer 接口
    if st, ok := err.(interface{ StackTrace() []uintptr }); ok {
        pcs := st.StackTrace()
        return formatFrames(pcs)
    }
    // Priority 2: runtime.Callers 回退
    var pcs [maxStackFrames]uintptr
    n := runtime.Callers(3, pcs[:])
    return formatFrames(pcs[:n])
}

func formatFrames(pcs []uintptr) string {
    frames := runtime.CallersFrames(pcs)
    var sb strings.Builder
    for {
        frame, more := frames.Next()
        sb.WriteString(fmt.Sprintf("%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line))
        if !more { break }
    }
    return sb.String()
}
```

在 `buildEntry` 中集成：

```go
if err, ok := value.(error); ok {
    fields[key] = err.Error()
    if key == "error" {
        fields["error_type"] = fmt.Sprintf("%T", err)
        fields["stacktrace"] = captureStack(err)
    }
}
```

**不采用 `debug.Stack()`：** 它始终分配完整栈帧（含 runtime 内部帧），不可控且开销大。`runtime.Callers` 可指定 skip 数量，输出更精简。第三方错误库（如 `pkg/errors`）通常已实现 `StackTrace()` 或类似接口，Priority 1 优先使用之，避免重复捕获。

---

### 3.2 Registry.sortedKeys 替换

**文件：** `internal/layers/observability/metrics/registry.go`

```go
// 替换前（registry.go:110-124）
func sortedKeys(m map[string]string) []string {
    keys := make([]string, 0, len(m))
    for k := range m { keys = append(keys, k) }
    // 冒泡排序
    for i := 0; i < len(keys)-1; i++ {
        for j := i + 1; j < len(keys); j++ {
            if keys[i] > keys[j] {
                keys[i], keys[j] = keys[j], keys[i]
            }
        }
    }
    return keys
}

// 替换后
import "sort"

func sortedKeys(m map[string]string) []string {
    keys := make([]string, 0, len(m))
    for k := range m { keys = append(keys, k) }
    sort.Strings(keys)
    return keys
}
```

---

### 3.3 ConsoleExporter 接口一致性

**文件：** `internal/layers/observability/exporter/console.go`

当前 ConsoleExporter 的 `Export` 方法签名为 `Export(s tracer.ReadableSpan) error`，SpanExporter 接口要求 `Export(ctx context.Context, span ReadableSpan) error`。

**方案：直接修改 ConsoleExporter.Export 签名以匹配接口：**

```go
func (e *ConsoleExporter) Export(_ context.Context, s tracer.ReadableSpan) error {
    // ... 原有逻辑 ...
}

func (e *ConsoleExporter) Shutdown(_ context.Context) error {
    return nil
}
```

然后删除 `factory.go` 中的 `consoleExporterAdapter`，`NewConsoleExporterSpanExporter` 直接返回 `NewConsoleExporter()`。

`NullExporter.ExportBatch` 方法删除（不在接口中，死代码）。

---

### 3.4 Logger 与 Tracer 包的依赖解耦

**文件：** `internal/layers/observability/logger/logger.go`

当前 `StructuredLogger` 通过 `WithTrace(sc tracer.SpanContext)` 直接依赖 `tracer.SpanContext` 类型。这导致 logger 包对 tracer 包有编译时依赖。

**方案：** 将 `WithTrace` 改为接收 traceId 和 spanId 字符串：

```go
// 替换前
func (l *StructuredLogger) WithTrace(sc tracer.SpanContext) *StructuredLogger {
    return l.With("traceId", sc.TraceID.String(), "spanId", sc.SpanID.String())
}

// 替换后 — 不依赖 tracer 包
func (l *StructuredLogger) WithTrace(traceID, spanID string) *StructuredLogger {
    return l.With("traceId", traceID, "spanId", spanID)
}
```

这样 logger 包不再 import tracer 包，消除循环依赖风险。

**调用方影响确认：** 通过代码搜索验证，`WithTrace` 在 `internal/layers/observability/` 外无任何调用方。仅有 2 处内部调用（均在 `observability.go`），同步修改即可。编译时类型检查确保签名变更不会遗漏。

---

## 四、受影响的文件清单

```
internal/layers/observability/
├── metrics/
│   ├── gauge.go          # MODIFIED: float64 原子转换 + Mutex
│   ├── async_gauge.go    # MODIFIED: callback 互斥保护
│   ├── async_counter.go  # MODIFIED: callback 互斥保护
│   ├── histogram.go      # MODIFIED: Observe break + +Inf bucket
│   ├── counter.go        # MODIFIED: 删除死 Mutex
│   ├── meter.go          # MODIFIED: Int64UpDownCounter 返回 Gauge
│   ├── registry.go       # MODIFIED: sort.Strings + RegisterGauge + Reset
│   └── prometheus.go     # MODIFIED: 简化累加逻辑
├── tracer/
│   └── tracer.go         # MODIFIED: Shutdown 刷写 + double-check
├── logger/
│   ├── logger.go         # MODIFIED: +采样, +堆栈(StackTracer), 解耦 tracer, +Close
│   └── logger_test.go    # MODIFIED: 增强断言
├── exporter/
│   ├── console.go        # MODIFIED: Export 签名匹配接口
│   ├── null.go           # MODIFIED: 删除 ExportBatch
│   └── factory.go        # MODIFIED: 删除 consoleExporterAdapter
├── bridge.go             # MODIFIED: ActiveSessions 返回 Gauge
├── observability.go      # MODIFIED: Shutdown 全组件
└── bench_test.go         # MODIFIED: 增加 Gauge benchmark
```

---

## 五、回归风险评估

| 变更 | 回归风险 | 缓解措施 |
|------|---------|---------|
| Gauge 改为 Mutex | 低 — 调用方仅观察 Value() | 现有集成测试覆盖 |
| AsyncGauge/Counter 加锁 | 低 — 回调注册在初始化阶段 | `-race` 测试 |
| Histogram Observe break + +Inf | 低 — 修正错误行为 | 新增 golden 测试 |
| Shutdown 刷写 + double-check | 中 — 涉及并发 | `-race` 测试 |
| UpDownCounter → Gauge | 低 — SessionBridge 调用方同步修改 | 编译时类型检查 |
| 日志采样容量保护 | 低 — 新增行为，不影响现有路径 | 单元测试验证 |
| Registry.Reset 全类型 | 低 — 补充缺失逻辑 | 现有测试覆盖 |
| Logger 解耦 tracer | 低 — 仅签名变化 | 编译时检查 |
| Logger.Close + Handler.Close | 低 — 新增空实现 | 接口编译验证 |
| ConsoleExporter 签名 | 低 — 新增 ctx 参数未使用 | 接口编译验证 |
