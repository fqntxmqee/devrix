# Devrix 可观察层增强设计

**Change ID:** devrix-observability-enhancement
**Demand ID:** DM-20260610-001
**Layer:** L5 - Observability
**Status:** Draft
**Version:** 2.0.0-draft（2026-06-10 修订：AI 排查就绪 + 代码基线对齐）

---

## 〇、代码基线对照（Review 结论）

> 本节供二次 Review 快速对齐「文档 vs 代码」。

| 能力 | proposal v1 | 代码现状 (2026-06-10) | 本 change 动作 |
|------|-------------|----------------------|----------------|
| LLM span events | TODO | ✅ `pev_engine.go` 已调用 | 维持 + 补 trace_id |
| PEV iteration span | TODO | ✅ 已有 | **修复层级/defer** |
| compression/recall/store span | TODO | ✅ `engine.go` 已有 | 补决策属性 + metrics |
| synthesis/plan/milestone span | TODO | ✅ `pev_engine.go` 已有 | 维持 |
| llm.adapter.stream | TODO | ✅ `gateway.go:194` 已有 | 维持 |
| Registry 44 operations | 目标 33 | ✅ 已完成 | 维持 |
| tool_latency metric | TODO | ❌ 未实现 | **新增** |
| compression_ratio metric | TODO | ❌ 未实现 | **新增** |
| Baggage 业务接入 | TODO | ❌ 未使用 | **降为 P2** |
| Span 层级契约 | 未提及 | ❌ 扁平化 | **新增 P0** |
| Log-Trace 关联 | 未提及 | ❌ slog 无 trace_id | **新增 P0** |
| Incident export | 未提及 | ❌ 不存在 | **新增 P1** |

---

## 一、设计目标（修订）

| 目标 | 旧 V1 | 当前代码 | V2 目标 | 用户/AI 可感知结果 |
|------|-------|----------|---------|-------------------|
| LLM 输入输出可见 | 无 events | ✅ span events + JSONL | 加 trace_id 关联 | AI 可从 trace 跳转 LLM 全量 |
| PEV 迭代可见 | 无 span | ✅ 有 span | **正确层级+生命周期** | Jaeger 可按轮次折叠 |
| 因果链可推理 | 未定义 | ❌ 扁平 | **Canonical Trace Tree** | AI 可输出有序 RCA |
| 工具延迟可分析 | 无 metrics | Counter only | tool_latency Histogram | AI 可先聚合再下钻 |
| Log-Trace 关联 | 未定义 | ❌ | slog 自动 trace_id | 从 log 一键找 trace |
| AI incident 导出 | 未定义 | ❌ | JSON bundle | AI Agent 单次输入足够 |

---

## 二、Canonical Trace Tree（验收契约）

以下树结构为 **L5-OBS-TRACE-04** 验收标准。集成测试 MUST 验证 parent-child 关系，而非仅 span 名称存在。

### 2.1 主链（D1 Gateway → D2 Context → D3 LLM）

```
gateway.message.receive                          [SpanKind: Server]
└── context.process                              [SpanKind: Internal]
    ├── context.system_prompt.load
    ├── context.snapshot.load
    ├── context.longterm.recall                  [条件: longterm.enabled]
    ├── context.compression.run                  [条件: shouldCompress]
    └── context.pev.run
        ├── context.plan.generate                [条件: ShouldPlan]
        ├── context.milestone.run                [条件: milestone 路径]
        └── context.pev.iteration                [每轮迭代，生命周期=单轮]
            ├── context.pev.llm_call
            │   └── llm.stream                   [ctx 必须从 llm_call 传递]
            │       ├── llm.provider.route
            │       ├── llm.circuit_breaker
            │       ├── llm.retry
            │       └── llm.adapter.stream
            ├── context.pev.tool_execute         [每 tool call]
            │   └── context.pev.permission_check
            └── context.pev.verify
        └── context.pev.synthesis                [条件: len(toolResults)>0]
            └── llm.stream                       [synthesis LLM 子链同上]
    ├── context.memory.snapshot.save
    └── context.longterm.store                   [条件: auto_store]
└── gateway.engine_event.handle                  [每 outbound event，兄弟节点]
```

### 2.2 层级约束（MUST）

| 规则 ID | 约束 |
|---------|------|
| R1 | `context.pev.llm_call` 的 parent MUST 为 `context.pev.iteration` |
| R2 | `llm.stream` 的 parent MUST 为 `context.pev.llm_call` 或 `context.pev.synthesis` |
| R3 | `context.pev.permission_check` 的 parent MUST 为 `context.pev.tool_execute` |
| R4 | `context.pev.iteration` 的 duration MUST 近似单轮耗时（禁止 loop defer） |
| R5 | 同一 trace 内所有 span MUST 共享 trace_id；`session.id` 属性一致 |

### 2.3 当前代码违规点（待修复）

```go
// pev_engine.go — 违规示例
for iter := 0; iter < maxIter; iter++ {
    _, iterSpan := e.startSpan(ctx, ...)   // R1 违规：未更新 ctx
    defer iterSpan.End()                    // R4 违规：loop defer
    _, llmSpan := e.startSpan(ctx, ...)   // R1 违规：parent=run 非 iteration
    e.llm.ChatStream(ctx, req)              // R2 违规：ctx 无 llmSpan
}
```

---

## 三、Span 传播规范（P0）

### 3.1 强制模式

```go
func (e *PEVEngine) runExecuteVerifyLoop(...) (*PEVRunResult, error) {
    ctx, runSpan := e.startSpan(ctx, telemetry.OpContextPEVRun, ...)
    if runSpan != nil {
        defer runSpan.End()
    }

    for iter := 0; iter < maxIter; iter++ {
        func(iter int) { // 或 inline End，禁止 loop defer
            ctx, iterSpan := e.startSpan(ctx, telemetry.OpContextPEVIteration,
                tracer.Attribute{Key: "pev.iteration", Value: fmt.Sprintf("%d", iter)},
            )
            if iterSpan != nil {
                defer iterSpan.End()
            }

            ctx, llmSpan := e.startSpan(ctx, telemetry.OpContextPEVLLMCall, ...)
            if llmSpan != nil {
                defer llmSpan.End()
            }
            AddLLMRequestEvent(llmSpan, sc.SessionID, iter, sc.Model, req)
            chunks, err := e.llm.ChatStream(ctx, req) // ctx 含 llmSpan
            // ...
        }(iter)
    }
}
```

### 3.2 参考实现（已正确）

`permission_check` 作为 `tool_execute` 子 span — 应作为全链路范本：

```go
toolCtx, toolSpan := e.startSpan(ctx, telemetry.OpContextPEVToolExecute, ...)
_, permSpan := e.startSpan(toolCtx, telemetry.OpContextPEVPermissionCheck, ...)
```

### 3.3 SpanKind 规范（P1）

SpanKind 影响 Jaeger UI 呈现方式和后端采样决策。MUST 标注：

| Operation | SpanKind | 依据 |
|-----------|----------|------|
| `gateway.message.receive` | `SERVER` | 接收外部请求 |
| `context.pev.llm_call`, `llm.adapter.stream` | `CLIENT` | 跨进程调用 LLM API |
| `context.pev.tool_execute` | `INTERNAL` | 本进程内工具执行 |
| `context.pev.permission_check` | `INTERNAL` | 本进程内权限检查 |
| `context.pev.iteration`, `context.pev.run` | `INTERNAL` | 本进程内编排 |
| `context.compression.run` | `INTERNAL` | 本进程内压缩 |
| `context.system_prompt.build` | `INTERNAL` | 本进程内组装 |
| 所有 harness bootstrap 操作 | `INTERNAL` | 本进程内启动编排 |

修改点：`startSpan` 签名扩展 `opts ...tracer.SpanOption`，各调用处加 `tracer.WithSpanKind(...)`。

### 3.4 OTel 标准错误记录（P0 — 当前缺口）

当前大量错误路径仅 `span.SetStatus(codes.Error, ...)` 或完全未设置，缺少 `RecordError` 导致 AI 无法区分「正常结束标记 error」 vs 「异常崩溃」。MUST 执行：

```go
// ✅ 强制模式
if err != nil {
    span.SetStatus(codes.Error, err.Error())           // 1. 设置状态
    span.RecordError(err, tracer.WithStackTrace(true)) // 2. 记录异常事件 + 堆栈
    span.SetAttributes(attr.String("error.code", sentinelCode)) // 3. SentineError code
}

// ❌ 禁止
span.SetAttributes(attr.String("error.code", "..."))   // 仅有 code，缺 Status 和 RecordError
// 或完全沉默错误
```

验证：集成测试断言错误 span 同时满足：
- `span.Status.Code == Error`
- `span.Events` 包含 `exception` 类型事件
- `Attributes` 含 `error.code`

---

## 四、Log-Trace-LLM 关联（P0）

### 4.1 slog Trace 注入

```
observability/
└── logger/
    └── slog_bridge.go          # 新增：slog.Handler 包装器
        - 从 ctx 读取 SpanContext → 注入 traceId, spanId
        - 从 ctx 或 attrs 读取 sessionId
```

业务层继续用 `slog.Info/Warn`，无需改调用点。

配置项（已有）：`observability.logging.include_trace_id: true`

### 4.2 LLM JSONL 扩展

```json
{
  "timestamp": "2026-06-10T12:00:00Z",
  "session_id": "sess_xxx",
  "trace_id": "abc123...",
  "span_id": "def456...",
  "phase": "request",
  "iteration": 0,
  "model": "gpt-4o",
  "data": { }
}
```

修改点：`observability/llm_log.go` → `appendLLMLogRaw`；调用方传入当前 span context。

---

## 五、决策语义 + GenAI 标准化属性（P0/P1）

### 5.1 决策语义属性（P1，现有提案）

| 属性 Key | 写入位置 | 示例值 |
|----------|----------|--------|
| `verify.failure_reason` | `pev_engine.go` verify 后 | `"command output empty"` |
| `verify.passed` | 已有 | `true/false` |
| `compression.trigger_reason` | `engine.go` shouldCompress | `"token_budget_exceeded"` |
| `compression.ratio` | 已有 tokens 属性旁 | `0.42` |
| `pev.synthesis_source` | synthesis/fallback | `synthesis` / `tool_fallback` |
| `error.code` | 所有 RecordError span | SentinelError code |

### 5.2 `gen_ai.*` 语义属性双写（P0 — OTel 标准兼容）

所有 LLM 相关 span 在现有自定义属性基础上，**额外写入** OTel GenAI 语义公约属性，确保与 Jaeger v2 / Grafana Tempo GenAI 面板兼容：

```go
// LLM call span attributes — 双写（自定义 + gen_ai.*）
span.SetAttributes(
    // 自定义（保持向前兼容）
    attr.String("llm.model", model),
    attr.Int("llm.input_tokens", inputTokens),
    // gen_ai.* 标准（OTel 生态）
    attr.String("gen_ai.request.model", model),
    attr.Int("gen_ai.usage.input_tokens", inputTokens),
    attr.Int("gen_ai.usage.output_tokens", outputTokens),
    attr.StringSlice("gen_ai.response.finish_reasons", finishReasons),
    attr.String("gen_ai.agent.name", "Devrix"),
    attr.String("gen_ai.conversation.id", sessionID),
)
```

**适用 span**：`context.pev.llm_call`, `llm.adapter.stream`, `context.pev.synthesis` 下 llm 子链。

### 5.3 Prompt 版本标记（P1 — AI 排查入口）

```go
// system_prompt.build span
span.SetAttributes(
    attr.String("gen_ai.prompt.version", devrixVersion),     // e.g. "2.1.0"
    attr.String("gen_ai.prompt.template_hash", hash),         // 四层模板内容 hash
)
```

**AI 价值**：快速判断「当前 session 的 prompt 是否与 release 版本一致」。

### 5.4 Token 类型细化（P1 — 成本分析）

```go
// LLM call span — 在已有总 token 基础上拆分
span.SetAttributes(
    attr.Int("gen_ai.usage.input_tokens", inputTokens),
    attr.Int("gen_ai.usage.output_tokens", outputTokens),
    attr.Int("gen_ai.usage.cache_read.input_tokens", cacheHitTokens),    // prompt cache 节省
    attr.Int("gen_ai.usage.reasoning.output_tokens", reasoningTokens),   // reasoning 模型思考
)
```

---

## 六、Metrics（P1 — 真正剩余缺口）

### 6.1 Tool Latency Histogram

```go
// bridge.go — ToolBridge 扩展
func (b *ToolBridge) InitLatencyMetrics(toolName, riskLevel string) (*ToolLatencyMetrics, error) {
    return b.meter.Float64Histogram("tool_latency",
        metrics.WithHistogramLabels(metrics.LabelMap{
            "tool": toolName, "risk_level": riskLevel,
        }),
        metrics.WithBounds(metrics.DefaultHistogramBounds()),
    )
}
```

使用：`pev_engine.go` 工具执行处 `Observe(time.Since(start).Seconds())`

### 6.2 Compression Ratio Histogram

压缩成功后在 `engine.go` observe `report.Ratio`。

### 6.3 与 Trace 互补

- Trace：`tool.duration_ms`（单次）
- Metrics：`tool_latency` P50/P99（聚合）
- AI 工作流：Metrics 筛 anomaly → Trace 下钻根因

### 6.4 Token Usage Breakdown Metrics（P1）

```go
// meter 注册
tokenUsage, _ := b.meter.Int64Counter("gen_ai.client.token.usage",
    metrics.WithLabels(metrics.LabelMap{
        "token_type": "input|output|cache_read|reasoning",
        "model":      model,
    }),
)
```

使用：`pev_engine.go` LLM 调用返回后按 token_type 分别 Add。

---

## 七、Session Incident Export（P1）

### 7.1 CLI

```bash
devrix debug export --session sess_xxx --format json [--output /tmp/incident.json]
```

### 7.2 Bundle Schema（v1）

```json
{
  "schema_version": "1.0",
  "session_id": "sess_xxx",
  "exported_at": "2026-06-10T12:00:00Z",
  "trace": {
    "trace_id": "...",
    "spans": [
      {
        "name": "context.pev.iteration",
        "span_id": "...",
        "parent_span_id": "...",
        "start_time": "...",
        "duration_ms": 1234,
        "attributes": {},
        "events": []
      }
    ]
  },
  "llm_rounds": [],
  "errors": [],
  "coverage_hits": [
    {
      "operation": "context.pev.iteration",
      "layer": "d2",
      "hit_count": 5,
      "last_hit": "2026-06-10T12:00:00Z"
    }
  ],
  "eval_scores": {
    "verify.failure_reason": "command output empty",
    "preflight.warnings": ["token budget near limit"]
  },
  "prompt_versions": {
    "devrix": "2.1.0",
    "agents_md_hash": "a3f2b9",
    "template_hash": "f7e8d1"
  }
}
```

**实现路径**：复用 `MemoryExporter` 逻辑 + 读 LLM JSONL + 可选 coverage snapshot。

---

## 八、Baggage（P2 — 降级）

单进程 monolith 下 `session.id` / `llm.model` 已在 span attributes 中。

**触发条件**：adapter 或 agent_tool 拆为独立进程时再启用 OTel Baggage propagation。

本 change **不实现** Baggage 业务接入。

---

## 九、采样策略（P2 — 现在规划）

### 9.1 当前策略

V2 阶段：**全量采集**（sampling=1.0）。埋点增量全部 export，适用于开发和初期生产。

### 9.2 未来 tail-sampling 触发条件

当 OTLP export 成本或后端存储成为瓶颈时，启用 OTel Collector tail-sampling processor：

| 流量类型 | 建议采样率 | 理由 |
|---------|-----------|------|
| ERROR span（status=Error） | **100%** | 漏 error = 漏 RCA |
| 高延迟（P99 以上） | **100%** | 长尾也是排查重点 |
| eval 评分低于阈值 | **100%** | 质量事故信号 |
| 高 token 消耗（> 预算 2×） | **100%** | 成本异常 |
| 正常流量 | **5-20%** | 统计分布，无需全量 |

### 9.3 本 change 动作

本章节仅为 **规划文档**，不实现采样逻辑。在 `docs/observability-design.md` 中记录策略，待 OTLP 成本数据积累后再启用。

---

## 十、覆盖率目标（修订为辅助指标）

| 指标 | 说明 | 目标 |
|------|------|------|
| Static coverage | Registry Instrumented + 代码有 startSpan | ≥95%（已基本达成） |
| Runtime coverage | coverage.Report() Hit/Total | 分 Layer 统计，不设全局 80% 硬门槛 |
| Hierarchy coverage | 集成测试 R1-R5 规则 | **100%**（P0 硬门槛） |

低频 operation（`agent.*`、`context.tools.register`）不计入 Runtime 80% 分母。

---

## 十一、L5 Registry（修订）

| ID | Capability | 优先级 | 说明 |
|----|------------|--------|------|
| L5-OBS-TRACE-01 | llm-trace-complete | P0 | ✅ 已有 |
| L5-OBS-TRACE-02 | pev-iteration-trace | P0 | ✅ 已有，🔧 层级 |
| L5-OBS-TRACE-04 | span-hierarchy-contract | P0 | **新增** |
| L5-OBS-TRACE-05 | log-trace-correlation | P0 | **新增** |
| L5-OBS-TRACE-06 | spankind-compliance | P1 | **新增** |
| L5-OBS-TRACE-07 | error-recording-standard | P0 | **新增** |
| L5-OBS-METRICS-01 | tool-latency-metrics | P1 | **新增** |
| L5-OBS-METRICS-02 | compression-metrics | P1 | **新增** |
| L5-OBS-METRICS-03 | token-usage-breakdown | P1 | **新增** |
| L5-OBS-EXPORT-01 | session-incident-export | P1 | **新增** |
| L5-OBS-DECISION-01 | verify-decision-attrs | P1 | **新增** |
| L5-OBS-DECISION-02 | prompt-version-traceable | P1 | **新增** |
| L5-OBS-TRACE-03 | baggage-context-propagation | P2 | 降级 |
| L5-OBS-COVERAGE-01 | coverage-80pct | P2 | 辅助 |

---

## 十二、AI 排查就绪评分卡（Review 快照）

供其他模型 Review 时对照：

```
能力维度                  现状    V2目标   本change覆盖
─────────────────────────────────────────────────────
完整因果链 (trace tree)   ■■■■□□  ■■■■■■   §三 P0
LLM 决策可还原            ■■■■■□  ■■■■■■   §四 trace_id
Tool I/O 可还原           ■■■□□□  ■■■■□□   JSONL+export
Log-Trace 关联            ■■□□□□  ■■■■■□   §四 P0
机器可读 export           ■■□□□□  ■■■■■□   §七 P1
聚合异常 (metrics)        ■■■□□□  ■■■■□□   §六 P1
多路径 (Agent)            ■■■□□□  —        Out of Scope
```

**Verdict**：V2 完成后达到 **L2 基本满足**（见 proposal Goals）；L3 自主闭环需后续 change。
