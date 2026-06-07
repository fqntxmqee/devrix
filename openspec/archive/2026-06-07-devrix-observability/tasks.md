# Tasks: devrix-observability

**Change ID:** devrix-observability
**Status:** Draft (Design Phase — 无代码任务)
**Based on:** design.md, `openspec/specs/observability_layer_delta.md`

---

## Milestone 1: 基础类型与配置

### Definition of Done
- [ ] 类型与配置编译通过
- [ ] L5-OBS-01 测试点已登记

### Tasks

- [x] **T1**: 新增 `layers/observability/config.go`（Config 结构体）
  - L4: L4-OBS-CONFIG
  - L5: L5-OBS-01
  - Estimate: 2h
  - Dependencies: None

- [x] **T2**: 新增 `layers/observability/tracer/types.go`（TraceID, SpanID, SpanContext）
  - L4: L4-OBS-TRACE
  - L5: L5-OBS-01, L5-OBS-02
  - Estimate: 3h
  - Dependencies: T1

- [x] **T3**: 实现 `config.go` 默认值 + devrix.yaml 配置块
  - L4: L4-OBS-CONFIG
  - L5: —
  - Estimate: 2h
  - Dependencies: T1

---

## Milestone 2: Tracer 实现

### Definition of Done
- [ ] Span 可创建、结束、记录属性
- [ ] Trace ID 在入口生成

### Tasks

- [x] **T4**: 实现 `tracer/span.go`（Span 接口 + 实现）
  - L4: L4-OBS-TRACE
  - L5: L5-OBS-01
  - Estimate: 6h
  - Dependencies: T2

- [x] **T5**: 实现 `tracer/tracer.go`（TracerProvider, Tracer）
  - L4: L4-OBS-TRACE
  - L5: L5-OBS-01
  - Estimate: 4h
  - Dependencies: T4

- [x] **T6**: 实现 `tracer/context.go`（SpanContext in Go context）
  - L4: L4-OBS-TRACE
  - L5: L5-OBS-02
  - Estimate: 3h
  - Dependencies: T4

- [x] **T7**: 实现 `tracer/sampler.go`（AlwaysOn, AlwaysOff, TraceIdRatio）
  - L4: L4-OBS-SAMPLING
  - L5: L5-OBS-10
  - Estimate: 4h
  - Dependencies: T4

- [x] **T8**: 实现 `tracer/propagation.go`（W3C TraceContext inject/extract）
  - L4: L4-OBS-PROPAGATION
  - L5: L5-OBS-12
  - Estimate: 4h
  - Dependencies: T4

---

## Milestone 3: Metrics 实现

### Definition of Done
- [ ] Counter, Histogram, Gauge 可创建和记录
- [ ] Prometheus /metrics 端点可访问

### Tasks

- [x] **T9**: 实现 `metrics/counter.go`（Int64Counter）
  - L4: L4-OBS-METRICS
  - L5: L5-OBS-03
  - Estimate: 3h
  - Dependencies: T1

- [x] **T10**: 实现 `metrics/histogram.go`（Float64Histogram）
  - L4: L4-OBS-METRICS
  - L5: L5-OBS-03
  - Estimate: 3h
  - Dependencies: T1

- [x] **T11**: 实现 `metrics/gauge.go`（Int64UpDownCounter）
  - L4: L4-OBS-METRICS
  - L5: L5-OBS-03
  - Estimate: 2h
  - Dependencies: T1

- [x] **T12**: 实现 `metrics/registry.go`（MetricRegistry + cardinality 控制）
  - L4: L4-OBS-METRICS
  - L5: L5-OBS-09
  - Estimate: 4h
  - Dependencies: T9, T10, T11

- [x] **T13**: 实现 `metrics/meter.go`（MeterProvider, Meter）
  - L4: L4-OBS-METRICS
  - L5: L5-OBS-03
  - Estimate: 3h
  - Dependencies: T12

- [x] **T14**: 实现 `metrics/prometheus.go`（Prometheus exporter + HTTP handler）
  - L4: L4-OBS-EXPORTER
  - L5: L5-OBS-06
  - Estimate: 4h
  - Dependencies: T13

---

## Milestone 4: Logger 实现

### Definition of Done
- [ ] JSON 格式日志可输出
- [ ] 日志包含 traceId

### Tasks

- [x] **T15**: 实现 `logger/handler.go`（JSON/Text handlers）
  - L4: L4-OBS-LOG
  - L5: L5-OBS-04
  - Estimate: 4h
  - Dependencies: T6

- [x] **T16**: 实现 `logger/redactor.go`（Secret redaction）
  - L4: L4-OBS-LOG
  - L5: L5-OBS-11
  - Estimate: 3h
  - Dependencies: T15

- [x] **T17**: 实现 `logger/logger.go`（StructuredLogger wrapper）
  - L4: L4-OBS-LOG
  - L5: L5-OBS-04
  - Estimate: 3h
  - Dependencies: T15

---

## Milestone 5: Exporter 实现

### Definition of Done
- [ ] Console exporter 可用
- [ ] OTLP exporter 可用（V2）

### Tasks

- [x] **T18**: 实现 `exporter/console.go`（Console span exporter）
  - L4: L4-OBS-EXPORTER
  - L5: —
  - Estimate: 2h
  - Dependencies: T4

- [x] **T19**: 实现 `exporter/otlp.go`（OTLP gRPC/HTTP exporter）
  - L4: L4-OBS-EXPORTER
  - L5: L5-OBS-08
  - Estimate: 6h
  - Dependencies: T4

- [x] **T20**: 实现 `exporter/null.go`（No-op exporter，disabled 模式）
  - L4: L4-OBS-EXPORTER
  - L5: —
  - Estimate: 1h
  - Dependencies: T4

---

## Milestone 6: Facade 与集成

### Definition of Done
- [ ] Observability 可初始化
- [ ] Health endpoint 可访问

### Tasks

- [x] **T21**: 实现 `observability.go`（Facade，初始化入口）
  - L4: L4-OBS-CONFIG
  - L5: L5-OBS-01
  - Estimate: 4h
  - Dependencies: T5, T13, T17

- [x] **T22**: 实现 `health.go`（Health checks）
  - L4: L4-OBS-HEALTH
  - L5: L5-OBS-07
  - Estimate: 3h
  - Dependencies: T21

- [x] **T23**: 实现 `shutdown.go`（Graceful shutdown）
  - L4: L4-OBS-SHUTDOWN
  - L5: L5-OBS-05
  - Estimate: 3h
  - Dependencies: T21

- [x] **T24**: `cmd/devrix/main.go` 注入 Observability
  - L4: L4-OBS-CONFIG
  - L5: —
  - Estimate: 2h
  - Dependencies: T21

---

## Milestone 7: 各层集成

### Definition of Done
- [ ] Communication Layer 调用 Tracer
- [ ] LLM Gateway 记录 metrics

### Tasks

- [x] **T25**: Communication Layer 集成 Tracer（RouteInbound 创建 root span）
  - L4: L4-COMM-TRACING
  - L5: L5-OBS-01, L5-OBS-02
  - Estimate: 4h
  - Dependencies: T21, T24

- [x] **T26**: LLM Gateway 集成 metrics（记录 token/latency/error）
  - L4: L4-LLM-METRICS
  - L5: L5-OBS-03
  - Estimate: 4h
  - Dependencies: T21

- [x] **T27**: Tool Registry 集成 metrics（记录 tool_calls_total）
  - L4: L4-AGENT-METRICS
  - L5: L5-OBS-03
  - Estimate: 3h
  - Dependencies: T21

---

## Milestone 8: 测试与验收

### Definition of Done
- [ ] 单元测试覆盖核心路径
- [ ] L5 测试点全绿

### Tasks

- [x] **T28**: Tracer 单元测试
  - L4: L4-OBS-TRACE
  - L5: L5-OBS-01, L5-OBS-02
  - Estimate: 4h
  - Dependencies: T5, T6

- [x] **T29**: Metrics 单元测试
  - L4: L4-OBS-METRICS
  - L5: L5-OBS-03, L5-OBS-09
  - Estimate: 4h
  - Dependencies: T13, T14

- [x] **T30**: Logger 单元测试
  - L4: L4-OBS-LOG
  - L5: L5-OBS-04, L5-OBS-11
  - Estimate: 3h
  - Dependencies: T17

- [x] **T31**: Shutdown 单元测试
  - L4: L4-OBS-SHUTDOWN
  - L5: L5-OBS-05
  - Estimate: 2h
  - Dependencies: T23

- [x] **T32**: 验收测试 `tests/acceptance/p0/obs_trace_propagation_test.go`
  - L4: L4-OBS-TRACE
  - L5: L5-OBS-02
  - Estimate: 4h
  - Dependencies: T25

- [x] **T33**: 验收测试 `tests/acceptance/p1/obs_prometheus_test.go`
  - L4: L4-OBS-EXPORTER
  - L5: L5-OBS-06
  - Estimate: 3h
  - Dependencies: T14

- [x] **T34**: 更新 `openspec/l5-registry.md` IMPLEMENTED 状态
  - L5: L5-OBS-01 ~ L5-OBS-12
  - Estimate: 1h
  - Dependencies: T28-T33

- [x] **T35**: S5 `./scripts/gen-acceptance-report.sh --change devrix-observability`
  - L5: 全部 P0
  - Estimate: 1h
  - Dependencies: T34

---


---

## Milestone 9: 增强功能

### Definition of Done
- [ ] Resource 自动注入
- [ ] Baggage 传播可用

### Tasks

- [x] **T36**: 实现 `layers/observability/resource.go`（Resource 定义与注入）
  - L4: L4-OBS-CONFIG
  - L5: —
  - Estimate: 2h
  - Dependencies: T21

- [x] **T37**: 实现 `layers/observability/baggage.go`（Baggage 传播）
  - L4: L4-OBS-PROPAGATION
  - L5: L5-OBS-12
  - Estimate: 3h
  - Dependencies: T6

- [x] **T38**: Context Engine 集成测试
  - L4: L4-OBS-TRACE
  - L5: L5-OBS-02
  - Estimate: 4h
  - Dependencies: T26

- [x] **T39**: 性能基准测试
  - L4: L4-OBS-CONFIG
  - L5: —
  - Estimate: 3h
  - Dependencies: T21

- [x] **T40**: 架构设计审查
  - L4: —
  - L5: —
  - Estimate: 2h
  - Dependencies: T28-T33

---

## 任务统计

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

---

## V2 backlog（本变更不实施）

- [ ] OTLP exporter 增强
- [ ] Tail-based sampling
- [ ] Jaeger 集成

---

## 依赖关系图

```
T1 ─┬─ T4 ─ T5 ─ T6 ─ T8 ─ T25 ─ T28 ─ T32
     │         │                    │
     └─ T9 ───┴─ T12 ─ T13 ─ T14 ─ T29 ─ T33
     │
     └─ T15 ─ T16 ─ T17 ─ T30
     
T4 ─ T18 (Console)
T4 ─ T19 (OTLP)
T4 ─ T20 (Null)

T5 + T13 + T17 ─ T21 ─ T22 ─ T23
                          │
                          └─ T24 ─ T25
```

