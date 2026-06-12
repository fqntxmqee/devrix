# Devrix 可观察层深度分析

## 一、架构总览

```
┌─────────────────────────────────────────────────────────────────┐
│                    可观察层 (Observability)                      │
├─────────────────────────────────────────────────────────────────┤
│  Tracer          │  Metrics        │  Logger      │  Coverage   │
│  ─────           │  ──────         │  ──────       │  ───────    │
│  分布式追踪      │  指标聚合       │  结构化日志   │  代码染色    │
│  (Jaeger/OTLP) │  (Prometheus)   │  (slog)      │  (覆盖率)   │
└─────────────────┴─────────────────┴───────────────┴─────────────┘
                              │
                              ▼
              ┌───────────────────────────────┐
              │      Bridge (统一入口)        │
              │  tracer / meter / logger     │
              │  LLMBridge / ToolBridge    │
              │  SessionBridge              │
              └───────────────────────────────┘
```

## 二、核心数据结构

### 2.1 Span (分布式追踪)

```go
// tracer/span.go
type Span struct {
    name      string
    sc        SpanContext  // TraceID + SpanID + Flags
    parent    *SpanContext
    kind      SpanKind     // internal/server/client/producer/consumer
    startTime time.Time
    endTime   time.Time
    attrs     map[string]interface{}  // 关键属性
    events    []Event             // 事件列表
    status    Status
}

// Span 属性结构
Attributes = {
    "devrix.layer":        "context|llm|communication|agent",
    "devrix.component":     "pev_engine|context_engine|llm_gateway|...",
    "session.id":          "sess_xxx",
    "llm.provider":        "openai|anthropic",
    "llm.model":           "gpt-4|claude-3",
    "llm.tokens.prompt":    "128",
    "llm.tokens.completion": "256",
    "llm.latency_ms":      "150",
    "pev.iteration":       "0|1|2",
    "tool.name":            "bash|write_file|...",
    "tool.risk_level":      "low|medium|high|critical",
}
```

### 2.2 Operation 注册表 (Coverage)

```go
// coverage/registry.go
type OperationMeta struct {
    Name          string  // "context.pev.run"
    Layer         string  // "context"
    Component     string  // "pev_engine"
    SinceVersion  string  // "1.2.0"
    Instrumented  bool
}

// 层级结构
Layer.Component:
├── context.context_engine
│   ├── context.process
│   ├── context.snapshot.load
│   ├── context.compression.run
│   ├── context.plan.generate
│   └── context.longterm.*
├── context.pev_engine
│   ├── context.pev.run
│   ├── context.pev.llm_call
│   ├── context.pev.tool_execute
│   ├── context.pev.verify
│   └── context.pev.permission_check
├── llm.llm_gateway
│   ├── llm.stream
│   ├── llm.provider.route
│   ├── llm.circuit_breaker
│   └── llm.retry
├── communication.gateway
│   ├── gateway.message.receive
│   ├── gateway.session.*
│   └── gateway.store.*
└── agent.agent_tool
    ├── agent.run
    ├── agent.tool.call
    └── agent.fork|join|terminate
```

### 2.3 Metrics 指标

```go
// metrics/meter.go
类型:
├── Counter      // 计数器 (累加值)
├── Histogram    // 直方图 (延迟分布)
├── Gauge       // 仪表 (当前值)
└── UpDownCounter // 增减计数器

标签:
"provider": "openai"
"model": "gpt-4o"
"token_type": "input|output|cache_read|reasoning"
"tool": "bash"
"risk_level": "high"
"adapter": "cli"
"status": "ok|error|denied"
```

### 2.4 LLM 日志

```go
// llm_log.go
LLMCallInfo = {
    iteration: 0,
    model: "gpt-4",
    message_count: 12,
    tool_count: 5,
    messages: [
        {role: "user", content: "..."},
        {role: "assistant", content: "..."},
    ],
    tool_calls: [
        {name: "bash", input: "ls -la"},
    ],
    prompt_tokens: 512,
    completion_tokens: 128,
}
```

## 三、第一性原理分析

### 3.1 为什么这样设计？

**问题 1: 为什么需要分层 (L1-L6)？**

Devrix 是一个复杂的多智能体系统，需要：
- 理解请求在不同层之间的流转
- 定位问题发生在哪一层
- 关联跨层的调用链路

**设计选择**: Jaeger 的 Layer/Component 结构天然适配

```
L1: Communication  ←→  L2: Context  ←→  L3: LLM
     ↓                     ↓                  ↓
  用户输入              上下文管理          模型调用
  飞书/CLI              PEV Loop           OpenAI/Claude
```

**问题 2: 为什么用 Operation Name 作为代码染色的粒度？**

- 比函数级别更粗（避免过多噪音）
- 比模块级别更细（能定位到具体操作）
- 与 Jaeger trace 直接对应（可关联查看）

**问题 3: 为什么 Metrics 和 Tracer 分开？**

| 维度 | Metrics | Tracer |
|------|---------|--------|
| 用途 | 聚合统计 | 单次请求追踪 |
| 保留 | 长期 | 短期 |
| 聚合 | sum/avg/p99 | - |
| 场景 | 容量规划、SLO | 调试、根因 |

**互补设计**: Metrics 回答"系统怎么样"，Tracer 回答"这次请求发生了什么"

**问题 4: 为什么需要 Baggage？**

跨服务传递上下文（单进程 monolith 下 span attributes 通常足够，见 §十）。

## 四、当前埋点覆盖（2026-06-10，P0–P2 后）

### 4.1 Registry 状态

- **Operation 总数**: 44+（见 `internal/layers/observability/coverage/registry.go`）
- **静态注册**: 主链 operation 均已 `Instrumented: true`
- **Runtime Hit**: 取决于流量路径；条件分支（compression、plan、milestone、longterm）在未触发时为零命中，**不代表死代码**

### 4.2 主链埋点（已实现）

| Layer | 代表 Operation | 状态 |
|-------|----------------|------|
| communication | `gateway.message.receive` | ✅ SERVER |
| context | `context.process`, `context.compression.run`, `context.system_prompt.build` | ✅ |
| context | `context.pev.run` → `iteration` → `llm_call` / `tool_execute` / `verify` | ✅ 层级契约 |
| llm | `llm.stream` → `llm.adapter.stream` | ✅ CLIENT |
| agent | `agent.tool.call`, `agent.run` | ✅ |

### 4.3 条件触发 Operation（zero-hit 常见）

| Operation | 触发条件 |
|-----------|----------|
| `context.compression.run` | token 超 `CompressionTarget` |
| `context.plan.generate` | PEV plan 模式 |
| `context.milestone.run` | milestone DAG |
| `context.longterm.*` | longterm.enabled |
| `context.harness.*` | harness.enabled |
| `context.pev.synthesis` | 工具轮次后合成 |
| `context.pev.tool_execute` | LLM 返回 tool_calls |

### 4.4 仍待扩展（非阻塞）

| 项 | 说明 |
|----|------|
| Baggage 业务接入 | 单进程 monolith 下 span attributes 已够用；多服务拆分时再启用 |
| `cache_read` / `reasoning` token metrics | Provider usage details → metrics + span attrs |
| OTLP tail-sampling | 见 §九，仅规划 |

---

## 五、Canonical Trace Tree（D5-TRACE-T04/06）

集成测试 `tests/integration/obs_pev_span_hierarchy_test.go` 验证 R1–R2 与 SpanKind。

```
gateway.message.receive                          [Server]
└── context.process                              [Internal]
    ├── context.system_prompt.load
    ├── context.snapshot.load
    ├── context.longterm.recall                  [if longterm.enabled]
    ├── context.compression.run                  [if shouldCompress]
    │   attrs: compression.trigger_reason, compression.ratio
    ├── context.system_prompt.build              [if harness.enabled]
    │   attrs: gen_ai.prompt.version, gen_ai.prompt.template_hash
    └── context.pev.run
        ├── context.plan.generate                [if ShouldPlan]
        ├── context.milestone.run
        └── context.pev.iteration                [per iter; ctx propagated]
            ├── context.pev.llm_call             [Client]
            │   └── llm.stream                   [Client; ctx from llm_call]
            │       ├── llm.provider.route
            │       ├── llm.circuit_breaker
            │       ├── llm.retry
            │       └── llm.adapter.stream       [Client]
            ├── context.pev.tool_execute         [Internal]
            │   └── context.pev.permission_check
            └── context.pev.verify
        └── context.pev.synthesis                [if tools]
            └── llm.stream → … (同上)
    ├── context.memory.snapshot.save
    └── context.longterm.store
```

### 层级约束（MUST）

| 规则 | 约束 | 测试 |
|------|------|------|
| R1 | `context.pev.llm_call` parent = `context.pev.iteration` | ✅ |
| R2 | `llm.stream` parent = `context.pev.llm_call` 或 synthesis 链 | ✅ |
| R3 | `context.pev.permission_check` parent = `context.pev.tool_execute` | 结构保证 |
| R4 | `context.pev.iteration` 生命周期 = 单轮（无 loop defer） | ✅ |
| R5 | 同 trace 共享 trace_id；`session.id` 一致 | ✅ |

---

## 六、Metrics 目录（已实现）

| Metric | Labels | 写入位置 |
|--------|--------|----------|
| `devrix_tool_latency` | tool, risk_level, status | PEV tool execute |
| `devrix_compression_ratio` | — | context compression |
| `devrix_gen_ai.client.token.usage` | token_type, model | LLM gateway + PEV |
| `devrix_engine_tool_calls` | tool, risk_level | PEV |
| `devrix_llm_*` | provider, model | LLM gateway |
| `devrix_active_sessions` | adapter | Session bridge |

Prometheus 端点：`observability.metrics` 配置（默认 `/metrics`）。

---

## 七、Log ↔ Trace ↔ LLM 关联（P0）

| 信号 | 关联字段 | 实现 |
|------|----------|------|
| slog | `trace_id`, `span_id` | `InstallSlogBridge()` |
| LLM JSONL | `trace_id`, `span_id` | `llm_log.go` |
| Span | `gen_ai.*` 双写 | PEV / LLM spans |

---

## 八、Session Incident Export

```bash
# 主二进制
devrix debug export --session sess_xxx --output /tmp/incident.json

# 独立命令（等价）
go run ./cmd/debug-export --session sess_xxx
```

Bundle schema v1：`internal/layers/observability/incident/export.go`（`llm_rounds`, `trace`, `coverage_hits`）。

---

## 九、采样策略（规划，未实现）

| 流量 | 建议采样率 |
|------|-----------|
| ERROR span | 100% |
| 高延迟 P99+ | 100% |
| 正常流量 | 5–20% |

当前：**全量采集**（sampling=1.0）。待 OTLP 成本数据积累后启用 Collector tail-sampling。

---

## 十、Baggage

W3C `baggage` 头由 `tracer.Propagator` inject/extract；Gateway 入站写入 `session.id` / `user.id`；CLI agent 子进程通过 `TRACEPARENT` + `BAGGAGE` 环境变量继承。

| Key | 设置点 |
|-----|--------|
| `session.id` | Gateway `RouteInbound` |
| `user.id` | Gateway（UserID 非空时） |

---

## 十一、改进行动（剩余）

| 优先级 | 任务 | 状态 |
|--------|------|------|
| — | Observability P0–P2 主链 | **DONE** |
| P3 | Baggage propagation | **DONE** (DM-20260610-005) |
| P3 | cache_read/reasoning token metrics | **DONE** (DM-20260610-007) |
| P3 | OTLP tail-sampling | 规划 |
