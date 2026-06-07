# Proposal: Devrix 可观察层

**Change ID:** devrix-observability
**Demand ID:** DM-20260607-001
**Status:** Proposed
**Version:** 1.0.0
**Author:** Architecture
**Date:** 2026-06-07

---

## 1. Background

Devrix 当前缺乏可观察性基础设施，各层（Communication、LLM Gateway、Multi-Agent）散乱记录日志和指标，无法实现：
- 端到端请求追踪
- 跨层性能分析
- 统一监控告警

随着项目复杂度增长，可观察性成为 SRE 和问题定位的必须能力。

---

## 2. Problem Statement

### 2.1 现状问题

| 问题 | 影响 |
|------|------|
| 无法追踪消息从入口到 LLM 的完整路径 | 问题定位耗时 30min+ |
| LLM 调用异常无量化指标 | 无法配置告警，提前发现问题 |
| 日志散乱，难以关联同一请求 | 日志聚合困难 |
| 各层独立记录 metrics | 指标口径不一致 |

### 2.2 根本原因

- 可观察性未作为独立模块设计
- 各层自行实现，缺乏统一标准
- 缺少 OTel 对齐

---

## 3. Alternatives Considered

### 3.1 不做（Status Quo）

**Pros**:
- 无开发成本

**Cons**:
- 无法支持生产环境运维
- 问题定位依赖人工猜测
- 未来重构成本更高

**结论**: ❌ 不可接受

---

### 3.2 简单日志增强

在现有代码中增加 `slog` + traceId 字段。

**Pros**:
- 改动最小
- 快速见效

**Cons**:
- 无法统一 metrics
- 无法支持分布式追踪
- 无法导出 Prometheus

**结论**: ⚠️ 部分满足，不够系统

---

### 3.3 OpenTelemetry 标准实现（推荐）

实现独立的 `layers/observability/` 模块，对齐 OTel 标准。

**Pros**:
- 行业标准，互操作性强
- 支持 OTLP/Jaeger/Prometheus 生态
- 接口统一，易于扩展
- 未来无需重构

**Cons**:
- 初期学习成本
- 依赖 `go.opentelemetry.io/otel`

**结论**: ✅ 推荐

---

## 4. Proposed Solution

### 4.1 目标

在 `internal/layers/observability/` 实现独立可观察层，对齐 OpenTelemetry 标准，支持：
- **Trace**: W3C Trace Context，root/child span
- **Metrics**: Prometheus 格式，Counter/Histogram/Gauge
- **Log**: 结构化 JSON，含 trace context

### 4.2 技术方案

#### 架构

```
Observability (L5)
├── TracerProvider → Tracer → Span → Exporter (Console/OTLP)
├── MeterProvider → Meter → Instruments → Exporter (Prometheus/OTLP)
└── LoggerProvider → StructuredLogger → Handler (JSON/Text)
```

#### 核心接口

```go
// Tracer
type Tracer interface {
    Start(ctx context.Context, name string, opts ...SpanStartOption) (context.Context, Span)
}

// Meter
type Meter interface {
    Int64Counter(name string, opts ...CounterOption) Int64Counter
    Float64Histogram(name string, opts ...HistogramOption) Float64Histogram
    Int64UpDownCounter(name string, opts ...CounterOption) Int64UpDownCounter
}

// StructuredLogger
type StructuredLogger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    With(args ...any) StructuredLogger
}
```

#### 集成点

| 层 | 集成内容 |
|----|----------|
| L1 Communication | RouteInbound 创建 root span |
| L3 LLM Gateway | 记录 token/latency/error metrics |
| L4 Multi-Agent | 记录 tool_calls_total metrics |
| L2 Context Engine | V2 集成 |

### 4.3 配置

```yaml
observability:
  enabled: true
  tracing:
    enabled: true
    exporter: "console"  # V1
  metrics:
    enabled: true
    exporter: "prometheus"
    endpoint: "/metrics"
  logging:
    enabled: true
    level: "info"
    format: "json"
```

---

## 5. Capabilities

| Capability | Description | L4 映射 |
|-----------|-------------|----------|
| **OBS-TRACE** | Trace/Span 数据模型 | L4-OBS-TRACE |
| **OBS-METRICS** | Prometheus 指标采集 | L4-OBS-METRICS |
| **OBS-LOG** | 结构化日志 | L4-OBS-LOG |
| **OBS-EXPORTER** | 多后端导出器 | L4-OBS-EXPORTER |
| **OBS-CONFIG** | 配置管理 | L4-OBS-CONFIG |
| **OBS-HEALTH** | 健康检查 | L4-OBS-HEALTH |
| **OBS-SAMPLING** | 采样策略 | L4-OBS-SAMPLING |
| **OBS-PROPAGATION** | Trace Context 传播 | L4-OBS-PROPAGATION |
| **OBS-SHUTDOWN** | 优雅关闭 | L4-OBS-SHUTDOWN |

---

## 6. Version Scope

### V1 (本提案目标)

| Feature | Description |
|---------|-------------|
| Trace/Span | OTel 兼容模型，Console Exporter |
| Metrics | Prometheus 端点，8 个核心指标 |
| Logging | JSON 格式，trace context |
| Health | /health 端点 |

### V2

| Feature | Description |
|---------|-------------|
| OTLP | gRPC/HTTP Exporter |
| Sampling | trace_id_ratio 采样 |
| Health Enhanced | 详细组件状态 |

### V3

| Feature | Description |
|---------|-------------|
| Jaeger | OTLP 桥接 |
| Tail Sampling | 错误 span 全量保留 |
| APM | 性能分析集成 |

---

## 7. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| OTel 学习成本 | 中 | 参考 `context-engine-design.md` 模式 |
| 性能开销 | 低 | 异步导出，开销 < 1% CPU |
| 依赖外部库 | 低 | 使用稳定版，引脚版本 |
| 侵入性修改 | 中 | 接口注入，最小侵入 |

---

## 8. Dependencies

| 依赖 | 类型 | 说明 |
|------|------|------|
| go.opentelemetry.io/otel | Go 库 | OTel 核心 |
| github.com/prometheus/client_golang | Go 库 | Prometheus 导出 |
| Go 1.21+ | 运行时 | slog 内置 |

---

## 9. Open Questions

| # | 问题 | 建议 | 状态 |
|---|------|------|------|
| 1 | Span 内存上限 | 1000 active spans/session | OPEN |
| 2 | Metric reset 策略 | On scrape vs. periodic | OPEN |
| 3 | 日志输出目标 | stdout vs. file | OPEN |
| 4 | Trace 持久化 | V1 无，V2 可选 | OPEN |

---

## 10. Success Metrics

| Metric | Target |
|--------|--------|
| 问题定位时间 | 从 30min → 5min |
| Trace 覆盖 | 100% 跨层调用 |
| Metrics 完整性 | 8/8 核心指标 |
| 代码覆盖率 | ≥ 80% |

---

## 11. L1-L5 映射

### 11.1 L1 领域

| L1 ID | 领域名称 | 类型 | 说明 |
|--------|----------|------|------|
| L1-DEVRIX | Devrix 核心域 | 核心 | 可观察性属于基础设施域 |

### 11.2 L2 场景

| L2 ID | 场景名称 | L1 归属 | 说明 |
|--------|----------|---------|------|
| L2-OBS-TRACING | 调用链追踪 | L1-DEVRIX | Trace/Span 全链路 |
| L2-OBS-METRICS | 指标量化 | L1-DEVRIX | Prometheus metrics |
| L2-OBS-LOGGING | 结构化日志 | L1-DEVRIX | JSON log + trace |

### 11.3 L4 功能点

| L4 ID | 功能点名称 | L2 归属 | 说明 |
|--------|------------|---------|------|
| L4-OBS-TRACE | Trace/Span 数据模型 | L2-OBS-TRACING | OTel 兼容 |
| L4-OBS-SAMPLING | 采样策略 | L2-OBS-TRACING | always_on/ratio |
| L4-OBS-PROPAGATION | Trace Context 传播 | L2-OBS-TRACING | W3C 标准 |
| L4-OBS-METRICS | 指标采集 | L2-OBS-METRICS | Counter/Histogram/Gauge |
| L4-OBS-LOG | 结构化日志 | L2-OBS-LOGGING | JSON + traceId |
| L4-OBS-EXPORTER | 多后端导出器 | L2-OBS-METRICS | Console/Prometheus/OTLP |
| L4-OBS-CONFIG | 配置管理 | L2-OBS-TRACING | YAML 配置 |
| L4-OBS-HEALTH | 健康检查 | L2-OBS-METRICS | /health 端点 |
| L4-OBS-SHUTDOWN | 优雅关闭 | L2-OBS-TRACING | 刷写 spans |
| L4-OBS-RESOURCE | Resource 注入 | L2-OBS-TRACING | service.name/version |
| L4-OBS-BAGGAGE | Baggage 传播 | L2-OBS-TRACING | 业务上下文传递 |

### 11.4 L5 测试点

| L5 ID | 测试点名称 | L4 归属 | 优先级 |
|--------|------------|---------|--------|
| L5-OBS-01 | Trace ID 在消息入口生成 | L4-OBS-TRACE | P0 |
| L5-OBS-02 | Trace ID 传播至 LLM 调用 | L4-OBS-TRACE | P0 |
| L5-OBS-03 | LLM 调用记录 latency/token metrics | L4-OBS-METRICS | P0 |
| L5-OBS-04 | 结构化日志包含 traceId | L4-OBS-LOG | P0 |
| L5-OBS-05 | Graceful shutdown 刷写 traces | L4-OBS-SHUTDOWN | P0 |
| L5-OBS-06 | Prometheus /metrics 端点可访问 | L4-OBS-EXPORTER | P1 |
| L5-OBS-07 | Health endpoint 返回 observability 状态 | L4-OBS-HEALTH | P1 |
| L5-OBS-08 | OTLP exporter 导出到收集器 | L4-OBS-EXPORTER | P1 |
| L5-OBS-09 | Label cardinality 被正确控制 | L4-OBS-METRICS | P1 |
| L5-OBS-10 | Sampling 策略按配置生效 | L4-OBS-SAMPLING | P2 |
| L5-OBS-11 | Secret redaction 在日志中生效 | L4-OBS-LOG | P2 |
| L5-OBS-12 | W3C traceparent 头部注入/提取 | L4-OBS-PROPAGATION | P2 |
