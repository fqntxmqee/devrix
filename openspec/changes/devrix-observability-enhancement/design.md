# Devrix 可观察层增强设计

**Change ID:** devrix-observability-enhancement
**Demand ID:** DM-20260610-001
**Layer:** L5 - Observability (D2-S12)
**Status:** Draft
**Version:** 1.0.0-draft

---

## 一、设计目标

| 目标 | V1 | V2 | 用户可感知结果 |
|------|----|----|----------------|
| LLM 输入输出可见 | 无 span events | AddLLMRequestEvent 调用 | Jaeger 可看完整 LLM 请求/响应 |
| PEV 迭代可见 | 无独立 span | context.pev.iteration | 可分析迭代次数分布 |
| 工具延迟可分析 | 无 metrics | tool_latency Histogram | 可分析工具执行性能 |
| 埋点覆盖率 | ~50% | ≥80% | 更全面代码染色 |

---

## 二、架构变更

### 2.1 整体结构

```
observability/
├── tracer/           # 无变更
│   └── tracer.go       # RecordHit() 已接入
├── metrics/           # 新增 metrics 注册
│   └── meter.go        # 新增 tool_latency, compression_ratio
├── coverage/          # 新增 operation 注册
│   └── registry.go     # 新增 9 个 operation
├── bridge.go          # 新增 LLMBridge 扩展
├── llm_log.go         # 无变更
└── observability.go    # 新增 CoverageReporter 启动

contextengine/
├── pev_engine.go      # 新增 LLM 日志调用 + 迭代 span
├── engine.go          # 新增压缩/记忆 span
└── llm_logger.go      # 无变更（已在）

llmgateway/
└── gateway/gateway.go  # 新增 adapter span
```

### 2.2 新增 Span Operation 注册

```go
// coverage/registry.go 新增

// context.context_engine
{Name: "context.compression.run", Layer: "context", Component: "context_engine", SinceVersion: "2.1.0", Instrumented: true}
{Name: "context.longterm.recall", Layer: "context", Component: "context_engine", SinceVersion: "2.1.0", Instrumented: true}
{Name: "context.longterm.store", Layer: "context", Component: "context_engine", SinceVersion: "2.1.0", Instrumented: true}
{Name: "context.plan.generate", Layer: "context", Component: "context_engine", SinceVersion: "2.1.0", Instrumented: true}

// context.pev_engine
{Name: "context.pev.iteration", Layer: "context", Component: "pev_engine", SinceVersion: "2.1.0", Instrumented: true}
{Name: "context.pev.synthesis", Layer: "context", Component: "pev_engine", SinceVersion: "2.1.0", Instrumented: true}
{Name: "context.milestone.run", Layer: "context", Component: "pev_engine", SinceVersion: "2.1.0", Instrumented: true}

// llm
{Name: "llm.adapter.stream", Layer: "llm", Component: "llm_adapter", SinceVersion: "2.1.0", Instrumented: true}

// communication.adapter
{Name: "adapter.feishu.outbound", Layer: "communication", Component: "adapter", SinceVersion: "2.1.0", Instrumented: true}
```

---

## 三、详细设计

### 3.1 LLM 日志完整接入

#### 问题

`AddLLMRequestEvent` 和 `AddLLMResponseEvent` 在 `contextengine/llm_logger.go` 定义，但从未被调用。

#### 解决方案

在 PEV Engine 的 LLM 调用处调用：

```go
// pev_engine.go

func (e *PEVEngine) runExecuteVerifyLoop(...) (*PEVRunResult, error) {
    for iter := 0; iter < maxIter; iter++ {
        // 1. 新增: 迭代开始 span
        ctx, iterSpan := e.startSpan(ctx, telemetry.OpContextPEVIteration, tracer.SpanKindInternal,
            tracer.Attribute{Key: "pev.iteration", Value: iter},
        )
        
        // 2. 现有: LLM 调用
        ctx, llmSpan := e.startSpan(ctx, telemetry.OpContextPEVLLMCall, tracer.SpanKindClient, ...)
        
        // 3. 新增: 记录 LLM 请求
        AddLLMRequestEvent(llmSpan, sc.SessionID, iter, sc.Model, req)
        
        chunks, err := e.llm.ChatStream(ctx, req)
        // ...
        
        // 4. 新增: 记录 LLM 响应
        AddLLMResponseEvent(llmSpan, sc.SessionID, iter, assistantText, usage, pendingTools, toolResults)
        
        llmSpan.End()
        iterSpan.End()
    }
}
```

#### Span 属性

```go
// llm.request 事件属性
{
    "llm.iteration": "0",
    "llm.model": "gpt-4o",
    "llm.messages_count": "12",
    "llm.tools_count": "5",
    "llm.request_json": "{...}"  // 完整请求 JSON
}

// llm.response 事件属性
{
    "llm.iteration": "0",
    "llm.response_length": "256",
    "llm.tool_calls_count": "2",
    "llm.response_json": "{...}"
}
```

### 3.2 PEV 迭代独立 Span

#### 问题

PEV 循环的每次迭代没有独立 span，无法分析迭代次数分布。

#### 解决方案

```go
// pev_engine.go
for iter := 0; iter < maxIter; iter++ {
    // 迭代开始 span
    _, iterSpan := e.startSpan(ctx, telemetry.OpContextPEVIteration, tracer.SpanKindInternal,
        tracer.Attribute{Key: "pev.iteration", Value: iter},
        tracer.Attribute{Key: "pev.max_iterations", Value: maxIter},
    )
    
    // PEV 执行...
    
    if iterSpan != nil {
        iterSpan.End()
    }
}
```

### 3.3 工具延迟 Metrics

#### 问题

工具执行延迟没有独立 metrics，无法分析工具性能。

#### 解决方案

```go
// bridge.go 新增 ToolBridge 方法

func (b *ToolBridge) InitLatencyMetrics(toolName, riskLevel string) (*ToolLatencyMetrics, error) {
    labels := metrics.LabelMap{
        "tool": toolName,
        "risk_level": riskLevel,
    }
    
    latency, err := b.meter.Float64Histogram("tool_latency",
        metrics.WithHistogramLabels(labels),
        metrics.WithHistogramBounds(metrics.DefaultHistogramBounds()),
    )
    if err != nil {
        return nil, err
    }
    
    return &ToolLatencyMetrics{Latency: latency}, nil
}

type ToolLatencyMetrics struct {
    Latency metrics.Histogram
}
```

#### 使用

```go
// pev_engine.go 工具执行处
start := time.Now()
result, err := e.tools.Execute(ctx, tc)
if m := e.toolMetrics[tc.Name]; m != nil {
    m.Latency.Observe(time.Since(start).Seconds())
}
```

### 3.4 压缩 Span

```go
// engine.go

if e.shouldCompress(msgs, sc.TokenBudget) {
    ctx, compSpan := e.startSpan(ctx, telemetry.OpContextCompressionRun, tracer.SpanKindInternal,
        tracer.Attribute{Key: "context.messages_before", Value: len(msgs)},
        tracer.Attribute{Key: "context.tokens_before", Value: countTokens(msgs)},
    )
    
    compressed, report, err := e.compressionPipeline.Run(...)
    
    if compSpan != nil {
        compSpan.SetAttributes(
            tracer.Attribute{Key: "context.tokens_after", Value: report.CompressedTokens},
            tracer.Attribute{Key: "context.compression_ratio", Value: fmt.Sprintf("%.2f", report.Ratio)},
        )
        compSpan.End()
    }
}
```

### 3.5 记忆召回 Span

```go
// engine.go

recallCtx, recallSpan := e.startSpan(ctx, telemetry.OpContextLongTermRecall, tracer.SpanKindInternal,
    tracer.Attribute{Key: "context.query_length", Value: len(message)},
)

err := e.memory.EnrichWithLongTermRecall(recallCtx, sc, message)

if recallSpan != nil {
    recallSpan.SetAttributes(
        tracer.Attribute{Key: "context.recall_entries", Value: recallResult.Count},
    )
    recallSpan.End()
}
```

---

## 四、新增 Metrics

### 4.1 Tool Latency Histogram

```go
// observability/bridge.go

type ToolBridge struct {
    bridge *Bridge
    latencies sync.Map  // map[string]*ToolLatencyMetrics
}

type ToolLatencyMetrics struct {
    Latency metrics.Histogram
}

// 标签: tool, risk_level
// Buckets: [0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30]
```

### 4.2 Compression Ratio Histogram

```go
// observability/bridge.go

func (b *LLMBridge) InitCompressionMetrics() (*CompressionMetrics, error) {
    ratio, err := b.meter.Float64Histogram("compression_ratio",
        metrics.WithHistogramBounds([]float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9}),
    )
    return &CompressionMetrics{Ratio: ratio}, nil
}

type CompressionMetrics struct {
    Ratio metrics.Histogram
}
```

### 4.3 Active Sessions Gauge

```go
// observability/bridge.go

func (b *SessionBridge) ActiveSessions(adapter string) (metrics.Gauge, error) {
    return b.meter.Int64UpDownCounter("session_active_count",
        metrics.WithLabels(metrics.LabelMap{"adapter": adapter}),
    )
}
```

---

## 五、Baggage 增强

### 5.1 Context Baggage

```go
// engine.go

// 导入 baggage manager
import "github.com/devrix/devrix/internal/layers/observability"

// 在 Process 开始时设置
baggage := observability.NewBaggageManager(32)
ctx = baggage.Set(ctx, "session_id", sc.SessionID)
ctx = baggage.Set(ctx, "model", sc.Model)
ctx = baggage.Set(ctx, "work_dir", sc.WorkDir)

// 在 LLM 调用 span 中记录
if span != nil {
    span.SetAttributes(
        tracer.Attribute{Key: "session.id", Value: sc.SessionID},
        tracer.Attribute{Key: "llm.model", Value: sc.Model},
    )
}
```

---

## 六、覆盖率目标

| Layer | Component | Operations | 当前 | 目标 |
|-------|-----------|------------|------|------|
| context | context_engine | 10 | 2 | 6 |
| context | pev_engine | 8 | 4 | 7 |
| llm | llm_gateway | 4 | 1 | 3 |
| llm | llm_adapter | 1 | 0 | 1 |
| communication | gateway | 12 | 8 | 10 |
| communication | adapter | 3 | 1 | 2 |
| agent | agent_tool | 6 | 0 | 4 |
| **Total** | | **44** | **16 (36%)** | **33 (75%)** |

---

## 七、合规 L5 Registry ID

| 新增 ID | Capability | 说明 |
|---------|------------|------|
| L5-OBS-TRACE-01 | llm-trace-complete | LLM 输入输出完整 trace |
| L5-OBS-TRACE-02 | pev-iteration-trace | PEV 迭代独立 span |
| L5-OBS-TRACE-03 | baggage-context-propagation | Baggage 传递业务上下文 |
| L5-OBS-METRICS-01 | tool-latency-metrics | 工具延迟指标 |
| L5-OBS-METRICS-02 | compression-metrics | 压缩率指标 |
| L5-OBS-COVERAGE-01 | coverage-80pct | 埋点覆盖 ≥80% |
