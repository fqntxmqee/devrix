# D5 Observability Layer 详细设计

**文档类型:** 详细架构设计（遵循 `docs/methodology/detail-design-framework.md`）
**Domain:** D5 Observability
**DSAFT Type:** 公共域
**Version:** 2.0.0
**Status:** Active
**Last Updated:** 2026-06-14
**架构入口:** `openspec/specs/d5-observability/spec.md`
**Operation 常量:** `internal/layers/observability/instrument/telemetry/names.go`
**Registry SoT:** `internal/layers/observability/diagnose/coverage/registry.go`

> **注意:** `context.pev.*` 与 `pev_engine` 组件已退役（2026-06-13）。生产路径以 `context.process` + `query.loop.*` + `llm.stream` 为主；`context.harness.*` 仅在 `harness.enabled=true` 时触发。

---

## 文档索引

| 文档 | 用途 |
|------|------|
| `spec.md` | DSAFT 规范 SoT（Scenarios、Requirements） |
| 本文档 | 六段式可读架构设计（评审 / onboarding） |
| `layer-delta.md` | 层能力 Delta（Gherkin Scenario） |
| `coverage.md` | 代码染色操作手册 |
| `a-registry.md` / `f-registry.md` / `t-registry.md` | A/F/T 注册表 |
| `span-registry.md` | Span 注册表（56 ops，全局 Trace Tree） |

---

## ① 架构目标

### 业务目标

| 痛点 | 目标能力 | 可观测结果 |
|------|----------|------------|
| 多域 Agent 请求链路不可追踪 | W3C Trace Context + Jaeger Operation 树 | 单次会话完整 span 树 |
| 问题定位不知发生在哪一层 | `devrix.layer` / `devrix.component` 属性 | Jaeger 按层/组件过滤 |
| 埋点遗漏无法发现 | Operation Registry + Runtime Hit 对账 | Health `coverage.zero_hit_count` |
| LLM 调试缺乏上下文 | LLM JSONL + trace 关联 + incident export | `devrix debug export` bundle |
| 新旧执行路径混用难量化 | Runtime path metric | `runtime_path_resolved_total` |

### 技术目标（量化）

| 指标 | 目标 | 测量方式 |
|------|------|----------|
| Span 创建开销 | P99 < 50µs（无 exporter IO） | `bench_test.go` |
| Coverage RecordHit | 原子计数，无锁竞争 panic | `coverage_test.go` 100 goroutine |
| Shutdown flush | 100% pending span 导出 | `tracer_test.go` |
| Gauge 数值正确性 | 100% 精确读写 | `gauge_test.go` |
| Histogram 桶累积 | 与 Prometheus golden 一致 | `histogram_test.go` |
| Registry 对账 | names.go ≡ registry.go（56 ops） | `registry_test.go` |

### 约束条件

| 类型 | 约束 | 设计响应 |
|------|------|----------|
| 架构 | 可观测故障不阻断业务 | `NewNoOp()` + nil Bridge 守卫 |
| 基数 | 禁止 session_id 等高基数 label | Metrics `blocklist` |
| 兼容 | 未知 operation 仍创建 span | WARN + 向后兼容 |
| 退役 | PEV span 族不再注册 | Registry 已移除 `context.pev.*` |

---

## ② 架构原则

### 设计原则

1. **OTel Native** — SpanContext、Propagator、GenAI 语义属性对齐 OpenTelemetry
2. **Zero-Config Default** — `DefaultConfig()` 开箱 OTLP + Prometheus
3. **Graceful Degradation** — observability 模块故障 → `degraded`，不 panic
4. **Cardinality Safety** — Label allowlist/blocklist 防指标爆炸
5. **Coverage ≠ Sampling** — Hit 计数独立于 trace 采样决策

### 命名规范

| 场景 | 格式 | 示例 |
|------|------|------|
| Span / Jaeger Operation | `{layer}.{module}.{action}` | `query.loop.llm.call` |
| Prometheus metric | `devrix_{instrument}` | `devrix_tool_latency` |
| Layer attribute | `devrix.layer` | `context` |
| Component attribute | `devrix.component` | `query_loop` |

### 代码风格

- 各域通过 `Bridge` 注入，禁止直接 `new Tracer`
- Span 属性使用 `telemetry.SpanAttrs()` 统一注入 layer/component
- 错误日志经 `StructuredLogger`，error 类型自动附加 stack

---

## ③ 业务流程

### 主路径：QueryLoop Trace Tree

```
gateway.message.receive                          [SERVER]
└── context.process                              [INTERNAL]
    ├── context.snapshot.load
    ├── context.system_prompt.load
    ├── context.longterm.recall                  [if longterm.enabled]
    ├── context.compression.run                  [if shouldCompress]
    │   └── context.compression.step.{step}      [per pipeline step]
    ├── d7.turn.run                                [D7 RunTurn primary path]
    │   └── query.loop.turn                      [per turn]
    │       ├── query.loop.llm.call              [CLIENT]
    │       │   └── llm.stream                   [CLIENT]
    │       │       ├── llm.provider.route
    │       │       ├── llm.circuit_breaker
    │       │       ├── llm.retry
    │       │       └── llm.adapter.stream       [CLIENT]
    │       └── tool.execute.single              [if tool_calls]
    │           └── tool.execute.permission      [if CRITICAL]
    ├── context.memory.snapshot.save
    └── context.longterm.store                   [if auto_store]
```

### 条件路径：Legacy Harness

当 `query_loop.enabled=false` 且 `harness.enabled=true`：

```
context.process
├── context.harness.bootstrap.run
│   └── context.harness.bootstrap.stage  (prefetch|guards|setup|deferred_init|tool_pool)
├── context.harness.preflight
├── context.harness.tool_pool
├── context.harness.route
└── context.system_prompt.build
```

### 跨域 Agent / Orchestration

```
agent.run → agent.tool.call → agent.fork|join|terminate
orchestration.wave.schedule → orchestration.wave.task.execute
orchestration.flow.event.publish
```

### 异常与补偿

| 场景 | 行为 | 可观测性 |
|------|------|----------|
| Tracer shutdown 中 | 返回 no-op span | 不 panic |
| Exporter 失败 | span 本地记录，health `degraded` | WARN 日志 |
| 未知 operation | 仍创建 span + WARN | `unknown_hits` 计数 |
| Observability disabled | `NewNoOp()` | 零开销路径 |

---

## ④ 领域模型

### 核心聚合

```
Observability (Facade)
├── TracerProvider → Tracer → Span
├── MeterProvider → Meter → Counter|Histogram|Gauge
├── StructuredLogger → Handler + Sampler + Redactor
├── CoverageReporter → Counter + Persistence
└── Bridge → ToolBridge | SessionBridge
```

### Operation 注册表（56 条，按 Layer 分组）

| Layer | Component 数 | 代表 Operation |
|-------|---------------|----------------|
| `communication` | gateway(11), adapter(3) | `gateway.message.receive`, `adapter.message.receive` |
| `context` | context_engine(10), harness(5), query_loop(3), tool_runner(2), plan_*(7) | `context.process`, `query.loop.run` |
| `llm` | llm_gateway(4), llm_adapter(1) | `llm.stream`, `llm.adapter.stream` |
| `agent` | agent_tool(6) | `agent.run`, `agent.tool.call` |
| `orchestration` | orchestrator(3) | `orchestration.wave.schedule` |

### 值对象

| 类型 | 字段 | 用途 |
|------|------|------|
| `SpanContext` | TraceID, SpanID, TraceFlags | W3C 传播 |
| `OperationMeta` | Name, Layer, Component, SinceVersion, Instrumented | Registry 元数据 |
| `GenAITokenBreakdown` | Input, Output, CacheRead, Reasoning | Token metrics |
| `Bundle` (incident) | schema_version, llm_rounds, trace, coverage_hits | 调试导出 |

---

## ⑤ 核心链路图

### 端到端可观测路径

```
用户消息 (D1)
  → gateway.message.receive span [trace_id 生成/继承]
  → context.process span [ctx 传播]
  → query.loop.run span
  → query.loop.llm.call span
  → llm.stream span (D3) [gen_ai.* attrs + devrix_gen_ai.client.token.usage]
  → tool.execute.single span [devrix_tool_latency observe]
  → slog.InfoContext [traceId 注入]
  → LLM JSONL [trace_id 写入]
  → OTLP exporter → Jaeger
  → coverage.RecordHit [无条件，不受采样影响]
  → HealthCheck [coverage 摘要]
```

### 单点风险

| 节点 | 风险 | 缓解 |
|------|------|------|
| OTLP Collector 不可用 | span 丢失 | Console/Memory fallback；health `degraded` |
| `~/.devrix/coverage/` 不可写 | 日报失败 | 进程内 Report 仍可用 |
| 高基数 label 误用 | Prometheus 爆炸 | Registry blocklist + validateLabels |

---

## ⑥ 接口 / API 设计

### 编程接口

```go
// Facade
obs, _ := observability.New(cfg)
bridge := observability.NewBridge(obs)

// Span 创建（各域标准模式）
ctx, span := bridge.Tracer().Start(ctx, telemetry.OpQueryLoopLLMCall,
    tracer.WithSpanKind(tracer.SpanKindClient),
    tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpQueryLoopLLMCall)...),
)
defer span.End()

// GenAI token metrics
observability.RecordGenAITokenUsage(bridge.Meter(), model, usage)

// Health + Coverage
obs.HealthCheck()        // → coverage 摘要
obs.CoverageReport(true) // → 完整 Report
```

### HTTP 端点

| 端点 | 方法 | 响应 |
|------|------|------|
| `/health` | GET | `{status, components, coverage}` |
| `/metrics` | GET | Prometheus exposition format |

### CLI

```bash
# Session incident export
devrix debug export --session sess_xxx --output /tmp/incident.json

# Coverage 报表
go run ./cmd/coverage --date 2026-06-14 --summary
```

### 配置 Schema（摘要）

```yaml
observability:
  enabled: true
  tracing:
    service_name: devrix
    exporter: otlp  # console | otlp | memory | null
    sampling: { type: always_on, rate: 1.0 }
  metrics:
    exporter: prometheus
    endpoint: /metrics
    labels:
      allowlist: [provider, model, tool, risk_level, status, token_type, adapter, path]
      blocklist: [session_id, user_id]
  logging:
    level: info
    format: json
    sampling: { max_entries_per_span: 100 }
  llm:
    log_content: true
    log_dir: ~/.devrix/logs/llm
  coverage:
    enabled: true
    dir: ~/.devrix/coverage
    interval: 1h
```

### Metrics 目录

| Metric | Type | Labels | 写入位置 |
|--------|------|--------|----------|
| `devrix_tool_latency` | Histogram | tool, risk_level, status | QueryLoop tool execute |
| `devrix_compression_ratio` | Histogram | — | context compression |
| `devrix_gen_ai.client.token.usage` | Counter | token_type, model | LLM gateway |
| `devrix_active_sessions` | Gauge | adapter | SessionBridge |
| `devrix_runtime_path_resolved_total` | Counter | path | runtime.D5 bridge |

---

## 改进行动（剩余）

| 优先级 | 任务 | 状态 |
|--------|------|------|
| — | V1.0–V1.9 主链 | **DONE** |
| — | QueryLoop span 族 + Registry 对账 | **DONE** (V2.0) |
| P3 | OTLP tail-sampling | 规划（需 Collector 侧配置） |
| P3 | SpanKind 契约集成测试（QueryLoop 路径） | PLANNED |
