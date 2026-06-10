# Delta Spec: Devrix 可观察层增强 — AI 排查就绪

**Change ID:** devrix-observability-enhancement
**Based on:** `openspec/l5-registry.md` v1.0.0, `demand.md` v2026-06-10
**Status:** Draft (Revised 2026-06-10)

---

## 〇、Revision Summary

原 delta 假设「新增 9 operation + LLM events」。Code Review 后修订：

- **删除**：重复 operation 注册、LLM events 接入（已 DONE）
- **新增**：Span 层级契约、Log-Trace 关联、决策属性、Incident export
- **降级**：Baggage → P2 DEFERRED

---

## 一、变更范围

### 1.1 新增文件

```
observability/logger/slog_bridge.go          # slog trace_id 注入
cmd/debug/main.go                            # incident export CLI（或扩展现有 cmd）
openspec/changes/devrix-observability-enhancement/demand.md  # 新增
```

### 1.2 修改文件

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `contextengine/pev_engine.go` | **修复** ctx 传播 + defer；决策属性；tool_latency | P0/P1 |
| `contextengine/engine.go` | 决策属性 + compression_ratio | P1 |
| `observability/llm_log.go` | JSONL 增加 trace_id/span_id | P0 |
| `observability/bridge.go` | ToolBridge.InitLatencyMetrics | P1 |
| `tests/integration/full_chain_trace_test.go` | Span hierarchy 断言 R1-R5 | P0 |
| `docs/observability-design.md` | Canonical Trace Tree | P2 |

### 1.3 不再修改（已 DONE，勿重复）

| 文件 | 原因 |
|------|------|
| `coverage/registry.go` | 44 operations 已完整 |
| `llmgateway/gateway/gateway.go` | llm.adapter.stream 已有 |
| `contextengine/llm_logger.go` | AddLLM* 已调用 |

---

## 二、Registry 增量

**无新增 operation**。本 change 聚焦层级质量与 AI 消费，不扩 Registry。

可选后续（Out of Scope）：
- `debug.incident.export` — 若 export 本身需要 span

---

## 三、Span 变更增量

### 3.1 PEV Engine — 修复（非新增）

```diff
 // contextengine/pev_engine.go
 for iter := 0; iter < maxIter; iter++ {
-    _, iterSpan := e.startSpan(ctx, telemetry.OpContextPEVIteration, ...)
-    defer iterSpan.End()
-    _, llmSpan := e.startSpan(ctx, telemetry.OpContextPEVLLMCall, ...)
+    ctx, iterSpan := e.startSpan(ctx, telemetry.OpContextPEVIteration, ...)
+    // End at iteration boundary (NOT loop defer)
+    ctx, llmSpan := e.startSpan(ctx, telemetry.OpContextPEVLLMCall, ...)
     AddLLMRequestEvent(llmSpan, ...)
-    chunks, err := e.llm.ChatStream(ctx, req)
+    chunks, err := e.llm.ChatStream(ctx, req)  // ctx includes llmSpan
+    // verify span: use iter ctx
+    if verifySpan != nil {
+        verifySpan.SetAttributes(
+            tracer.Attribute{Key: "verify.failure_reason", Value: vr.Reason},
+        )
+    }
 }
```

### 3.2 新增 Span 属性

| Operation | 新增 Attribute |
|-----------|---------------|
| `context.pev.verify` | `verify.failure_reason` |
| `context.compression.run` | `compression.trigger_reason` |
| `context.pev.synthesis` | `pev.synthesis_source` |
| `*` (error paths) | `error.code` |

---

## 四、Metrics 增量

```diff
 // observability/bridge.go
+ func (b *ToolBridge) InitLatencyMetrics(toolName, riskLevel string) (*ToolLatencyMetrics, error)
+
+ type ToolLatencyMetrics struct {
+     Latency metrics.Histogram  // name: tool_latency
+ }

+ func (b *LLMBridge) InitCompressionMetrics() (*CompressionMetrics, error)
+ // name: compression_ratio
```

---

## 五、Log / Export 增量

### 5.1 LLM JSONL Schema v2

```diff
 {
   "timestamp": "...",
   "session_id": "...",
+  "trace_id": "...",
+  "span_id": "...",
   "phase": "request|response",
   ...
 }
```

### 5.2 Incident Export API

```
GET /debug/export?session_id=xxx   # 可选 HTTP
CLI: devrix debug export --session xxx --format json
```

---

## 六、L5 Registry 增量（待登记）

| ID | 说明 | 优先级 |
|----|------|--------|
| L5-OBS-TRACE-04 | Span 父子层级契约 | P0 |
| L5-OBS-TRACE-05 | Log-Trace-LLM 关联 | P0 |
| L5-OBS-EXPORT-01 | Session incident export | P1 |
| L5-OBS-DECISION-01 | 决策语义 span 属性 | P1 |

---

## 七、兼容性

### 7.1 向后兼容

- Trace JSON 形态变化（层级修复）— **breaking for dashboards** 依赖扁平结构的需更新
- LLM JSONL 新增字段 — 向后兼容（additive）
- 新 metrics — additive

### 7.2 迁移

- Jaeger 查询：按 iteration 折叠 previously 不可用，修复后可用
- 无数据库 migration

---

## 八、部署注意事项

- 开发环境保持 `tracing.sampling.type: always_on` 直至 hierarchy 测试稳定
- 生产启用 `log_content: true` 前评估 redaction（见 acceptance I3）
- Incident export 需访问 `~/.devrix/logs/llm/` 读权限
