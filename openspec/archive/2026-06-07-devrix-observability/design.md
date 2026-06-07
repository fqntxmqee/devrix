# Observability Layer Design (Layer 5)

**Change ID:** devrix-observability
**Layer:** 5 - Observability
**Status:** Draft
**Version:** 1.0
**Based on:** `docs/detail design framework.md`, `openspec/specs/observability_layer_delta.md`

> **文档分工：** 评审与 onboarding 读 `openspec/specs/observability_layer_delta.md`（六段式 + Technical Notes）；开发与任务拆解读本文档 + `tasks.md`；验收读 `specs/observability/spec.md` + L5 注册表。

---

## 一、架构目标

### 1.1 业务目标

| 业务目标 | 量化指标 | V1 |
|---------|---------|-----|
| **调用链追踪** | 问题定位从 30min 缩短到 5min | ✅ Trace/Span |
| **指标量化** | 可配置 Alert，提前发现 API 问题 | ✅ Prometheus Metrics |
| **结构化日志** | 单请求日志一键聚合 | ✅ JSON Log + Trace ID |
| **生产可复现** | Jaeger 中重放请求路径 | ✅ V3: OTLP + Jaeger |

### 1.2 技术目标（量化）

| 指标 | V1 目标 | 测量方式 |
|------|---------|----------|
| **Span 创建开销** | P99 < 1ms | Benchmark Span creation |
| **Trace 导出延迟** | P99 < 100ms (console) | Span End 到 stdout |
| **Metrics 内存占用** | < 10MB (1000 sessions) | 压测 profile |
| **Log 写入吞吐** | > 10,000 logs/sec | Benchmark logging |
| **Prometheus 响应时间** | P99 < 50ms | HTTP /metrics endpoint |
| **启动初始化时间** | < 500ms | Startup benchmark |

### 1.3 层间边界

```
Communication (L1)          Observability (L5)           Downstream
─────────────────          ───────────────────          ──────────
Gateway.RouteInbound  ──▶  Tracer.StartSpan()
                           Meter.Record()
                           Logger.Info()
                                    │
                                    └──▶ go.opentelemetry.io/otel
                                    └──▶ github.com/prometheus/client_golang
                                    └──▶ log/slog
◀── 注入 traceId ────  (context propagation)
```

**禁止：**
- Observability 不得包含业务逻辑
- 各业务层通过接口依赖 Tracer/Meter，不直接依赖具体实现

### 1.4 与现有代码对齐

当前基础设施（已有）：

```go
// internal/layers/communication/metrics/collector.go
// 已实现基础 Counter, Gauge, Histogram
// 需要扩展为 OTel 兼容接口
```

---

## 二、领域模型

### 2.1 核心实体

```go
// internal/layers/observability/tracer/span.go

// SpanContext 包含 trace 标识
type SpanContext struct {
    TraceID    TraceID    // 32-char hex
    SpanID     SpanID     // 16-char hex
    TraceFlags uint8      // 0x01 = sampled
    TraceState TraceState // W3C tracestate
    Remote     bool       // 是否来自外部
}

// Span 代表一个工作单元
type Span interface {
    End(opts ...SpanEndOption)
    SetStatus(code SpanStatusCode, description string)
    RecordError(err error, opts ...RecordErrorOption)
    SetAttributes(kv ...attribute.KeyValue)
    AddEvent(name string, opts ...EventOption)
    SpanContext() SpanContext
}
```

### 2.2 值对象

| 值对象 | 说明 | 不可变 |
|--------|------|--------|
| `TraceID` | 32-char hex W3C 格式 | ✅ |
| `SpanID` | 16-char hex | ✅ |
| `SpanStatusCode` | Unset/Ok/Error | ✅ |
| `MetricLabels` | Label map，cardinality 受控 | ❌ |

### 2.3 与 Go Context 的关系

```
context.Context
  └── context.WithValue(ctx, traceContextKey, SpanContext)
        └── context.WithValue(ctx, spanKey, Span)
```

---

## 三、组件结构

```
internal/layers/observability/
├── tracer/
│   ├── tracer.go           # TracerProvider, Tracer
│   ├── span.go             # Span, SpanContext
│   ├── propagation.go      # W3C TraceContext inject/extract
│   ├── sampler.go          # AlwaysOn/AlwaysOff/TraceIdRatio
│   └── context.go          # SpanContext in Go context
├── metrics/
│   ├── meter.go            # MeterProvider, Meter
│   ├── counter.go          # Int64Counter
│   ├── histogram.go        # Float64Histogram  
│   ├── gauge.go            # Int64UpDownCounter (gauge)
│   ├── registry.go         # MetricRegistry, label validation
│   └── prometheus.go       # Prometheus exporter + HTTP handler
├── logger/
│   ├── logger.go           # StructuredLogger (wraps slog)
│   ├── handler.go          # JSON/Text handlers
│   └── redactor.go         # Secret redaction
├── exporter/
│   ├── console.go          # Console span exporter
│   ├── otlp.go             # OTLP gRPC/HTTP exporter
│   └── null.go             # No-op exporter (disabled)
├── config.go               # Config structs
├── observability.go         # Facade, initialization
├── health.go               # Health checks
└── shutdown.go             # Graceful shutdown
```

---

## 四、Span 生命周期

### 4.1 入口点（Communication Layer）

```go
// internal/layers/communication/gateway/gateway.go 修改

func (g *CommunicationGateway) RouteInbound(ctx context.Context, msg *types.InboundMessage) error {
    // 创建 root span
    ctx, span := g.tracer.Start(ctx, "message.receive",
        trace.WithAttributes(
            attribute.String("session.id", msg.SessionID),
            attribute.String("adapter", "cli"),
        ),
    )
    defer span.End()
    
    // 注入 span context 到日志
    sc := span.SpanContext()
    ctx = slog.WithTraceContext(ctx, sc.TraceID, sc.SpanID)
    
    // 原有逻辑...
}
```

### 4.2 LLM Gateway

```go
// internal/layers/llmgateway/ 调用示例

func (g *LLMGateway) Chat(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
    ctx, span := g.tracer.Start(ctx, "llm.chat",
        trace.WithAttributes(
            attribute.String("llm.provider", req.Provider),
            attribute.String("llm.model", req.Model),
        ),
    )
    defer span.End()
    
    start := time.Now()
    resp, err := g.callAPI(ctx, req)
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        g.meter.RecordLLMError(req.Provider, req.Model, errType(err))
        return nil, err
    }
    
    span.SetAttributes(
        attribute.Int("llm.tokens.input", resp.Usage.InputTokens),
        attribute.Int("llm.tokens.output", resp.Usage.OutputTokens),
        attribute.Float64("llm.latency_ms", float64(time.Since(start).Milliseconds())),
    )
    g.meter.RecordTokens(req.Provider, req.Model, "input", resp.Usage.InputTokens)
    g.meter.RecordTokens(req.Provider, req.Model, "output", resp.Usage.OutputTokens)
    g.meter.RecordLatency(req.Provider, req.Model, time.Since(start))
    
    return resp, nil
}
```

### 4.3 Tool Registry

```go
// internal/layers/multiagent/tool.go 调用示例

func (r *ToolRegistry) Execute(ctx context.Context, call ToolCall) (*ToolResult, error) {
    ctx, span := r.tracer.Start(ctx, "tool.execute",
        trace.WithAttributes(
            attribute.String("tool.name", call.Name),
            attribute.String("tool.risk_level", string(call.RiskLevel)),
        ),
    )
    defer span.End()
    
    result, err := r.executeImpl(ctx, call)
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        r.meter.RecordToolError(call.Name, string(call.RiskLevel))
    } else {
        r.meter.RecordToolCall(call.Name, string(call.RiskLevel), "success")
    }
    
    return result, err
}
```

---

## 五、Metrics 命名规范

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `devrix_llm_tokens_total` | Counter | provider, model, direction | Total LLM tokens |
| `devrix_llm_latency_seconds` | Histogram | provider, model | LLM call latency |
| `devrix_llm_errors_total` | Counter | provider, model, error_type | LLM call errors |
| `devrix_tool_calls_total` | Counter | tool, risk_level, status | Tool execution count |
| `devrix_session_active` | Gauge | adapter | Active sessions |
| `devrix_permission_timeouts_total` | Counter | - | Permission timeouts |
| `devrix_permission_decisions_total` | Counter | decision | Permission decisions |
| `devrix_context_tokens_current` | Gauge | session_id (truncated) | Current context tokens |

### 5.1 Prometheus 端点

```go
// internal/layers/observability/metrics/prometheus.go

func (p *PrometheusExporter) Handler() http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        p.mu.Lock()
        defer p.mu.Unlock()
        
        // 收集所有指标
        output := p.registry.Output()
        
        w.Header().Set("Content-Type", "text/plain; version=0.0.4")
        w.Write([]byte(output))
    })
}
```

---

## 六、Structured Logging

### 6.1 日志格式（JSON）

```json
{
  "timestamp": "2026-06-07T10:00:00.000Z",
  "level": "INFO",
  "message": "LLM call completed",
  "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
  "spanId": "00abcd1234efgh56",
  "component": "llm_gateway",
  "service": "devrix",
  "version": "1.0.0"
}
```

### 6.2 集成 slog

```go
// internal/layers/observability/logger/logger.go

type StructuredLogger struct {
    handler *JSONHandler
    attrs   []any
}

func NewStructuredLogger(cfg *LoggingConfig) *StructuredLogger {
    handler := NewJSONHandler(cfg)
    return &StructuredLogger{handler: handler}
}

func (l *StructuredLogger) WithTrace(sc tracer.SpanContext) *StructuredLogger {
    return &StructuredLogger{
        handler: l.handler,
        attrs: append(l.attrs, 
            "traceId", sc.TraceID.String(),
            "spanId", sc.SpanID.String(),
        ),
    }
}
```

### 6.3 Secret Redaction

```go
// internal/layers/observability/logger/redactor.go

var SensitiveKeys = []string{
    "password", "token", "secret", "api_key", 
    "authorization", "private_key", "access_token",
}

func (r *Redactor) Redact(data map[string]any) map[string]any {
    result := make(map[string]any)
    for k, v := range data {
        if r.isSensitive(k) {
            result[k] = "[REDACTED]"
        } else {
            result[k] = v
        }
    }
    return result
}
```

---

## 七、配置契约（devrix.yaml）

```yaml
observability:
  enabled: true

  tracing:
    enabled: true
    service_name: "devrix"
    service_version: "1.0.0"
    exporter: "console"              # console | otlp | null
    sampling:
      type: "always_on"              # always_on | always_off | trace_id_ratio
      rate: 1.0                      # 0.0-1.0
    otlp:
      endpoint: "localhost:4317"
      insecure: true
      timeout: 5000ms

  metrics:
    enabled: true
    exporter: "prometheus"           # prometheus | otlp | null
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
        - api_key

  logging:
    enabled: true
    level: "info"                   # debug | info | warn | error
    format: "json"                  # json | text
    include_trace_id: true
    sampling:
      enabled: true
      max_entries_per_span: 100
    redactor:
      enabled: true
      patterns:
        - password
        - token
        - secret
        - api_key

  health:
    enabled: true
    endpoint: "/health"
```

---

## 八、依赖集成

### 8.1 与通信层（Layer 1）

入口点注入 Tracer：

```go
// cmd/devrix/main.go

func main() {
    cfg := config.Load()
    
    // 初始化可观察层
    obs, err := observability.New(&cfg.Observability)
    if err != nil {
        slog.Error("failed to init observability", "err", err)
    }
    defer obs.Shutdown(context.Background())
    
    // 注入到 Gateway
    gateway := gateway.NewCommunicationGateway(
        gateway.WithTracer(obs.Tracer()),
        gateway.WithMeter(obs.Meter()),
        gateway.WithLogger(obs.Logger()),
    )
}
```

### 8.2 与 LLM Gateway（Layer 3）

通过接口依赖：

```go
// internal/layers/llmgateway/llm.go

type LLMGateway struct {
    tracer trace.Tracer     // 注入
    meter  metric.Meter    // 注入
    // ...
}
```

### 8.3 与 Multi-Agent（Layer 4）

工具执行记录 Trace/Metrics：

```go
// internal/layers/multiagent/tool.go

type ToolRegistry struct {
    tracer trace.Tracer
    meter  metric.Meter
}
```

---

## 九、版本分期

### V1（本变更实现目标）

| 能力 | 说明 |
|------|------|
| Trace/Span 模型 | OTel 兼容，root/child span |
| Console Exporter | JSON 格式输出到 stdout |
| Prometheus Metrics | /metrics 端点 |
| Structured Logger | JSON + trace context |
| Graceful Shutdown | 刷写 pending spans |

### V2

| 能力 | 说明 |
|------|------|
| OTLP Exporter | gRPC/HTTP 导出 |
| Sampling | trace_id_ratio 采样 |
| Health Endpoint | /health 状态 |

### V3

| 能力 | 说明 |
|------|------|
| Jaeger 集成 | 通过 OTLP 桥接 |
| Tail Sampling | 错误 span 全量保留 |
| APM 集成 | 性能分析 |

---

## 十、测试策略与 L5 映射

| L5 ID | 描述 | 优先级 | 测试层级 |
|-------|------|--------|----------|
| L5-OBS-01 | Trace ID 在消息入口生成 | P0 | unit |
| L5-OBS-02 | Trace ID 传播至 LLM 调用 | P0 | acceptance |
| L5-OBS-03 | LLM 调用记录 latency/token metrics | P0 | unit |
| L5-OBS-04 | 结构化日志包含 traceId | P0 | unit |
| L5-OBS-05 | Graceful shutdown 刷写 traces | P0 | unit |
| L5-OBS-06 | Prometheus /metrics 端点可访问 | P1 | acceptance |
| L5-OBS-07 | Health endpoint 返回 observability 状态 | P1 | acceptance |
| L5-OBS-08 | OTLP exporter 导出到收集器 | P1 | unit |
| L5-OBS-09 | Label cardinality 被正确控制 | P1 | unit |
| L5-OBS-10 | Sampling 策略按配置生效 | P2 | unit |
| L5-OBS-11 | Secret redaction 在日志中生效 | P2 | unit |
| L5-OBS-12 | W3C traceparent 头部注入/提取 | P2 | unit |

---

## 十一、开放问题

| # | 问题 | 建议 | 决策人 |
|---|------|------|--------|
| 1 | Span 内存上限 | 1000 active spans per session | 架构 |
| 2 | Metric reset 策略 | On Prometheus scrape vs. periodic | 架构 |
| 3 | 日志输出目标 | stdout vs. file vs. syslog | 产品 |
| 4 | Trace 保留策略 | V1 无持久化，V2 可选 file | 架构 |

---


## 十二、Metrics 定义

### 12.1 指标清单

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `devrix_llm_tokens_total` | Counter | provider, model, direction | Total LLM tokens |
| `devrix_llm_latency_seconds` | Histogram | provider, model | LLM call latency |
| `devrix_llm_errors_total` | Counter | provider, model, error_type | LLM call errors |
| `devrix_tool_calls_total` | Counter | tool, risk_level, status | Tool execution count |
| `devrix_session_active` | Gauge | adapter | Active sessions |
| `devrix_permission_timeouts_total` | Counter | - | Permission timeouts |
| `devrix_permission_decisions_total` | Counter | decision | Permission decisions |
| `devrix_context_tokens_current` | Gauge | session_id (truncated) | Current context tokens |

### 12.2 Label 规范

**允许的 Labels（Allowlist）**:
- `provider`, `model`, `adapter`, `tool`, `risk_level`, `status`, `direction`, `decision`, `error_type`

**禁止的 Labels（Blocklist）**:
- `session_id`, `user_id`, `api_key`

### 12.3 Histogram Buckets

```go
// LLM Latency buckets
[]float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}
```

---

## 十三、Span 命名规范

### 14.1 命名约定

| 操作 | Span Name | Attributes |
|------|-----------|------------|
| 消息接收 | `message.receive` | adapter, session_id |
| LLM 聊天 | `llm.chat` | provider, model, tokens |
| 工具执行 | `tool.execute` | tool.name, risk_level |
| 权限请求 | `permission.request` | tool_name, timeout |
| 上下文压缩 | `context.compress` | before_tokens, after_tokens |
| 会话创建 | `session.create` | adapter |
| 会话过期 | `session.expire` | session_id, reason |

### 14.2 OTel 属性规范

**LLM 属性**（对齐 OTel LLM 语义约定）:
```go
attribute.String("llm.system", provider)
attribute.String("llm.request.model", model)
attribute.Int("llm.usage.input_tokens", 123)
attribute.Int("llm.usage.output_tokens", 456)
```

**Tool 属性**:
```go
attribute.String("tool.name", "Read")
attribute.String("tool.risk.level", "LOW")
```

**Error 属性**:
```go
attribute.String("error.type", "rate_limit_error")
attribute.String("error.message", "rate limit exceeded")
```

### 14.3 Span 状态码

| Code | 使用场景 |
|------|----------|
| `Unset` | 默认，未设置 |
| `Ok` | 操作成功完成 |
| `Error` | 操作失败 |

---

## 十四、Resource 与属性注入

### 15.1 Resource 定义

```go
// internal/layers/observability/resource.go

type Resource struct {
    ServiceName    string
    ServiceVersion string
    Environment    string
    GitCommit      string
    BuildTime      string
}
```

### 15.2 注入方式

所有 Span 和 Metric 自动带上 Resource 属性：

```go
attrs := []attribute.KeyValue{
    attribute.String("service.name", "devrix"),
    attribute.String("service.version", version.Version),
    attribute.String("deployment.environment", env),
}

tracerProvider := otel.NewTracerProvider(otel.WithAttributes(attrs...))
meterProvider := otel.NewMeterProvider(otel.WithAttributes(attrs...))
```

---

## 十五、Baggage Propagation

### 16.1 接口设计

```go
type BaggageManager interface {
    Set(ctx context.Context, key, value string) context.Context
    Get(ctx context.Context, key string) (string, bool)
    List(ctx context.Context) []BaggageItem
}
```

### 16.2 使用场景

```go
// 入口设置
ctx = baggage.Set(ctx, "tenant_id", "tenant-123")

// 传播到日志
logger := logger.With("tenant_id", tenantID)
```

### 16.3 W3C Baggage 头部

```
tracestate: tenant_id=tenant-123, user_id=user-456
```

---

## 十六、性能基准

### 17.1 Benchmark 任务

| Benchmark | 目标 | 测试方法 |
|-----------|------|----------|
| `BenchmarkSpanCreate` | P99 < 0.01ms | 10000 次 Create + End |
| `BenchmarkCounterInc` | P99 < 0.001ms | 100000 次 Inc |
| `BenchmarkJSONLog` | > 50000 logs/sec | 1000 并发写入 |
| `BenchmarkPrometheusOutput` | P99 < 50ms | 1000 metrics |

### 17.2 测试代码结构

```
internal/layers/observability/
└── *_bench_test.go
```

---

## 十七、修订后的任务统计

| Milestone | 任务数 | 预估 |
|-----------|--------|------|
| M1 基础 | 3 | 7h |
| M2 Tracer | 5 | 24h |
| M3 Metrics | 6 | 19h |
| M4 Logger | 3 | 10h |
| M5 Exporter | 3 | 9h |
| M6 Facade | 4 | 12h |
| M7 集成 | 3 | 11h |
| M8 测试 | 8 | 22h |
| M9 增强 | 5 | 14h |
| **合计** | **40** | **~128h** |
