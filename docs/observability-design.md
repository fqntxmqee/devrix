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
"tool": "bash"
"risk_level": "high"
"adapter": "cli"
"status": "success|error"
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

跨服务传递上下文（如 trace ID 不足以携带业务语义）：
- session_id 在 span 之间传递
- 用户意图摘要
- 中间计算结果

## 四、当前埋点覆盖

### 4.1 已埋点 (按 Layer)

| Layer | Component | Operation | 状态 |
|-------|-----------|-----------|------|
| context | pev_engine | context.pev.run | ✅ |
| context | pev_engine | context.pev.llm_call | ✅ |
| context | pev_engine | context.pev.verify | ✅ |
| context | pev_engine | context.pev.tool_execute | ✅ |
| context | pev_engine | context.pev.permission_check | ✅ |
| context | context_engine | context.process | ✅ |
| llm | llm_gateway | llm.stream | ✅ |
| llm | llm_gateway | llm.retry | ✅ |
| llm | llm_gateway | llm.circuit_breaker | ✅ |
| llm | llm_gateway | llm.provider.route | ✅ |
| communication | gateway | gateway.session.* | ✅ |
| communication | gateway | gateway.store.* | ✅ |
| communication | adapter | adapter.message.receive | ✅ |
| communication | adapter | adapter.cli.send | ✅ |
| agent | agent_tool | agent.* | ⚠️ 部分 |

### 4.2 缺失的埋点

#### 高优先级

```
context.context_engine:
  ❌ context.system_prompt.load      # System prompt 加载
  ❌ context.memory.snapshot.save    # 快照保存
  ❌ context.compression.run        # 压缩执行（只在触发时）
  ❌ context.verify.command          # 命令验证

context.pev_engine:
  ❌ context.pev.iteration         # 每次迭代开始
  ❌ context.pev.synthesis          # 工具结果合成
  ❌ context.milestone.run           # 里程碑执行
  ❌ context.plan.generate         # 计划生成

llm.llm_gateway:
  ❌ llm.adapter.stream            # 适配器层调用

communication.gateway:
  ❌ gateway.permission.check       # 权限检查
  ❌ gateway.engine_event.handle    # 事件处理

agent.agent_tool:
  ❌ agent.state.transition        # 状态转换
  ❌ agent.terminate               # 终止
```

#### 中优先级

```
context.context_engine:
  ❌ context.longterm.recall       # 长期记忆召回
  ❌ context.longterm.store        # 长期记忆存储

llm.llm_gateway:
  ❌ llm.fallback                 # 模型降级
  ❌ llm.timeout                  # 超时处理
```

### 4.3 埋点缺失的根因分析

1. **迭代过程中遗漏**: PEV 引擎中部分子操作没有独立 span
2. **工具层未覆盖**: Tool 执行层没有独立的 operation
3. **压缩/合成等条件分支**: 非常规路径可能被忽略
4. **Agent 层早期实现**: D4 层还在完善

## 五、遗漏发现

### 5.1 LLM 请求/响应未在 span events 中记录

**问题**: `AddLLMRequestEvent` 和 `AddLLMResponseEvent` 在 contextengine 中定义，但：

1. **未在 LLM Gateway 层调用** - LLM gateway 直接写 span attributes，未调用这些函数
2. **未在 PEV Engine 中传递** - PEV 调用 LLM 时没有记录完整请求/响应

**影响**: 
- 无法在 Jaeger 中查看 LLM 完整输入输出
- 调试 LLM 相关问题需要额外日志

**建议**:
```go
// 在 pev_engine.go 中调用
_, span := e.startSpan(ctx, telemetry.OpContextPEVLLMCall, ...)
AddLLMRequestEvent(span, sc.SessionID, iter, sc.Model, req)
```

### 5.2 Metrics 未完整使用

**现状**: Metrics 定义完善，但部分未注册：
- `tool_latency` - 工具执行延迟（未注册）
- `compression_ratio` - 压缩率（未注册）
- `session_duration` - 会话时长（未注册）

### 5.3 Baggage 未充分利用

**现状**: Baggage 已实现但未广泛使用
**建议**: 在 context engine 中传递关键上下文：
```go
// 传递 session 相关 baggage
ctx = baggageManager.Set(ctx, "session_id", sc.SessionID)
ctx = baggageManager.Set(ctx, "user_intent", summary)
```

### 5.4 缺少的关联能力

1. **Trace 与 Metrics 关联**: 通过 trace_id 关联
2. **Trace 与 Logs 关联**: 通过 session_id 关联
3. **Trace 与 Coverage 关联**: 已实现 (operation 名称)

## 六、改进建议

### 6.1 高优先级修复

```go
// 1. 在 PEV Engine 中添加完整的 LLM 请求记录
func (e *PEVEngine) startSpan(...) {
    _, span := e.obsBridge.Tracer().Start(ctx, operation, opts...)
    
    // 添加 LLM 日志
    if operation == telemetry.OpContextPEVLLMCall {
        AddLLMRequestEvent(span, sc.SessionID, iter, sc.Model, req)
    }
}

// 2. 添加缺失的 context engine 埋点
ctx, span := e.startSpan(ctx, telemetry.OpContextCompressionRun, ...)
ctx, span := e.startSpan(ctx, telemetry.OpContextLongTermRecall, ...)

// 3. 添加 PEV 迭代计数
ctx, span := e.startSpan(ctx, telemetry.OpContextPEVIteration, ...)

// 4. 添加工具执行独立埋点
ctx, span := e.startSpan(ctx, telemetry.OpContextPEVToolExecute, tracer.Attribute{Key: "tool.name", Value: tc.Name})
```

### 6.2 新增 Metrics

```go
// 在 pev_engine.go
e.toolLatency, _ = m.Float64Histogram("tool_latency",
    metrics.WithHistogramLabels(labels),
    metrics.WithBounds(metrics.DefaultHistogramBounds()))

// 在工具执行时
e.toolLatency.Observe(time.Since(start).Seconds())
```

### 6.3 Baggage 增强

```go
// 添加到 context engine
type ContextBaggage struct {
    SessionID    string
    UserIntent   string
    WorkDir      string
    Model        string
}

// 在 PEV 循环中传递
ctx = baggageManager.Set(ctx, "user_intent", extractIntent(message))
```

### 6.4 架构增强: 多维分析

```go
// 添加 Span 属性支持多维查询
type SpanAttributes struct {
    // 基础维度
    Layer      string
    Component  string
    Operation  string
    
    // 业务维度
    SessionID  string
    UserID    string
    WorkDir   string
    
    // 技术维度
    Model      string
    Provider   string
    Duration   int64
    
    // 质量维度
    Status     string
    ErrorCode  string
}
```

## 七、总结

### 设计优点

1. **分层清晰**: L1-L6 分层与架构对应
2. **标准化**: 遵循 OpenTelemetry 规范
3. **可扩展**: Bridge 模式便于新增功能
4. **代码染色**: 与 operation 注册表联动

### 改进空间

1. **埋点覆盖率**: 约 60%，部分关键路径缺失
2. **LLM 日志**: 未充分利用 span events
3. **Metrics**: 部分指标未注册
4. **Baggage**: 未充分使用

### 行动计划

| 优先级 | 任务 | 工作量 |
|--------|------|--------|
| P0 | PEV Engine 完整 LLM 日志 | 2h |
| P0 | 添加缺失的 context 埋点 | 3h |
| P1 | 添加 tool latency metrics | 2h |
| P1 | Agent 层埋点完善 | 4h |
| P2 | Baggage 增强使用 | 2h |
| P2 | 多维分析支持 | 4h |
