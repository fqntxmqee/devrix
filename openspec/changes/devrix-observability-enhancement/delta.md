# Delta Spec: Devrix 可观察层增强

**Change ID:** devrix-observability-enhancement
**Based on:** `openspec/l5-registry.md` v1.0.0
**Status:** Draft

---

## 一、变更范围

### 1.1 新增文件

```
openspec/changes/devrix-observability-enhancement/
├── proposal.md      # 问题陈述和解决方案概述
├── design.md        # 详细设计
├── tasks.md         # 任务分解
├── acceptance-report.md  # 验收标准
└── delta.md        # 本文件

internal/layers/observability/
└── (无新增文件，仅修改现有文件)

internal/layers/contextengine/
└── (无新增文件，仅修改现有文件)
```

### 1.2 修改文件

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `coverage/registry.go` | 新增 operation 注册 | +9 个 operation |
| `observability/bridge.go` | 新增 metrics | +3 个 metrics |
| `contextengine/pev_engine.go` | 新增 span 调用 | LLM 日志 + 迭代 span |
| `contextengine/engine.go` | 新增 span 调用 | 压缩/记忆 span |
| `docs/observability-design.md` | 新增 | 深度分析文档 |
| `docs/coverage.md` | 更新 | 使用手册 |

### 1.3 删除文件

无

---

## 二、Registry 增量

### 2.1 新增 Operation

```diff
 // coverage/registry.go

+ // === 新增 context.context_engine ===
+ {Name: "context.compression.run", Layer: "context", Component: "context_engine", SinceVersion: "2.1.0", Instrumented: true}
+ {Name: "context.longterm.recall", Layer: "context", Component: "context_engine", SinceVersion: "2.1.0", Instrumented: true}
+ {Name: "context.longterm.store", Layer: "context", Component: "context_engine", SinceVersion: "2.1.0", Instrumented: true}
+ {Name: "context.plan.generate", Layer: "context", Component: "context_engine", SinceVersion: "2.1.0", Instrumented: true}

 // === 新增 context.pev_engine ===
+ {Name: "context.pev.iteration", Layer: "context", Component: "pev_engine", SinceVersion: "2.1.0", Instrumented: true}
+ {Name: "context.pev.synthesis", Layer: "context", Component: "pev_engine", SinceVersion: "2.1.0", Instrumented: true}
+ {Name: "context.milestone.run", Layer: "context", Component: "pev_engine", SinceVersion: "2.1.0", Instrumented: true}

 // === 新增 llm ===
+ {Name: "llm.adapter.stream", Layer: "llm", Component: "llm_adapter", SinceVersion: "2.1.0", Instrumented: true}

 // === 新增 communication ===
+ {Name: "adapter.feishu.outbound", Layer: "communication", Component: "adapter", SinceVersion: "2.1.0", Instrumented: true}
```

### 2.2 新增 Metrics

```diff
 // observability/bridge.go

+ type ToolBridge struct {
+   // 新增
+   latencies sync.Map  // map[string]*ToolLatencyMetrics
+ }
+
+ type ToolLatencyMetrics struct {
+   Latency metrics.Histogram
+ }
+
+ func (b *ToolBridge) InitLatencyMetrics(...) (*ToolLatencyMetrics, error)

+ type LLMBridge struct {
+   // 新增
+   compression Metrics *CompressionMetrics
+ }
+
+ type CompressionMetrics struct {
+   Ratio metrics.Histogram
+ }
+
+ func (b *LLMBridge) InitCompressionMetrics() (*CompressionMetrics, error)
```

---

## 三、Span 调用增量

### 3.1 PEV Engine

```diff
 // contextengine/pev_engine.go

  for iter := 0; iter < maxIter; iter++ {
+   // 新增: 迭代开始 span
+   _, iterSpan := e.startSpan(ctx, telemetry.OpContextPEVIteration, tracer.SpanKindInternal, ...)
+   defer func() { iterSpan.End() }()

    // LLM 调用
    _, llmSpan := e.startSpan(ctx, telemetry.OpContextPEVLLMCall, ...)

+   // 新增: LLM 请求日志
+   AddLLMRequestEvent(llmSpan, sc.SessionID, iter, sc.Model, req)

    chunks, err := e.llm.ChatStream(ctx, req)

+   // 新增: LLM 响应日志
+   AddLLMResponseEvent(llmSpan, sc.SessionID, iter, assistantText, usage, pendingTools, toolResults)
  }

  // 工具合成
+ // 新增: 工具合成 span
+ ctx, synthSpan := e.startSpan(ctx, telemetry.OpContextPEVSynthesis, ...)
```

### 3.2 Context Engine

```diff
 // contextengine/engine.go

  // 压缩
  if e.shouldCompress(msgs, sc.TokenBudget) {
+   // 新增: 压缩执行 span
+   ctx, compSpan := e.startSpan(ctx, telemetry.OpContextCompressionRun, ...)
+   defer func() { compSpan.End() }()

    compressed, report, err := e.compressionPipeline.Run(...)
  }

  // 记忆召回
+ // 新增: 记忆召回 span
+ recallCtx, recallSpan := e.startSpan(ctx, telemetry.OpContextLongTermRecall, ...)
  err := e.memory.EnrichWithLongTermRecall(recallCtx, sc, message)
```

---

## 四、配置变更

### 4.1 新增配置项

```diff
 # internal/shared/config/contextengine.go 或 devrix.yaml

+ observability:
+   coverage:
+     enabled: true
+     dir: "~/.devrix/coverage"
+     interval: 1h
```

---

## 五、兼容性

### 5.1 向后兼容

- 所有新增 operation 默认 `Instrumented: true`
- `coverage.enabled=false` 时行为不变
- Metrics 可选注册

### 5.2 数据库迁移

无

---

## 六、部署注意事项

- 无特殊部署要求
- Metrics 和 Tracing 默认开启
- Coverage 报表存储在 `~/.devrix/coverage/`
