# Delta: Domain D5 (OBS)

**Change ID:** devrix-foundation → current
**Affects:** observability, tracing, metrics, logging, coverage, incident export, runtime metrics
**Version:** 3.0.0
**Status:** Active
**Last Updated:** 2026-06-14

---

## Current State Summary

D5 可观测性域已从 V1 基础能力演进为完整的 V2 实现：56 条 canonical Operation、QueryLoop 主路径 span 族、W3C Baggage、GenAI token 细分 metrics、Session incident export、Runtime path metric。PEV 引擎退役后，Registry 与文档已移除 `context.pev.*` 族。

---

## ADDED (V2.0 — 2026-06-14)

### Requirement: QueryLoop Span Family

QueryLoop 主路径 MUST 注册并创建以下 canonical Operation span：

| Operation | Component | SpanKind |
|-----------|-----------|----------|
| `query.loop.run` | query_loop | INTERNAL |
| `query.loop.turn` | query_loop | INTERNAL |
| `query.loop.llm.call` | query_loop | CLIENT |

#### Scenario: QueryLoop span hierarchy

- GIVEN `query_loop.enabled=true` and tracing enabled
- WHEN `ContextEngine.Process` completes one turn with LLM call
- THEN span `query.loop.run` exists under `context.process`
- AND `query.loop.turn` parent is `query.loop.run`
- AND `query.loop.llm.call` parent is `query.loop.turn`
- AND `llm.stream` parent is `query.loop.llm.call`

---

### Requirement: Tool Execution Span Family

| Operation | Component | 触发点 |
|-----------|-----------|--------|
| `tool.execute.single` | tool_runner | QueryLoop 工具执行 |
| `tool.execute.permission` | tool_runner | CRITICAL 工具权限检查 |

---

### Requirement: Task/Plan Span Family

| Operation | Component |
|-----------|-----------|
| `task.plan.generate` | plan_agent |
| `task.plan_mode.enter/execute/approve/reject` | plan_mode |
| `task.manager.create/update` | task_manager |

---

### Requirement: Orchestration Span Family

| Operation | Component |
|-----------|-----------|
| `orchestration.wave.schedule` | orchestrator |
| `orchestration.wave.task.execute` | orchestrator |
| `orchestration.flow.event.publish` | orchestrator |

---

### Requirement: Runtime Path Metric

系统 MUST 注册 `devrix_runtime_path_resolved_total` Counter，labels `path` ∈ {`query_loop`, `legacy_harness`}，与 in-process `PathResolver` 同步。

#### Scenario: Path counter increments on Process

- GIVEN observability metrics enabled and RegisterRuntimeMetric called
- WHEN ContextEngine routes to QueryLoop path
- THEN `devrix_runtime_path_resolved_total{path="query_loop"}` increments by 1

---

### Requirement: Compression Step Spans

`context.compression.step` 族（`OpContextCompressionStep + "." + step`）MUST 在七步压缩管道每步触发时创建子 span。

---

## ADDED (V1.3–V1.9 — 已归档，仍有效)

### Requirement: Operation Registry & Runtime Hit Counter

- Registry 56 条与 `names.go` 对账（`registry_test.go`）
- `Tracer.Start` 无条件 `RecordHit`，不受采样影响
- `HealthCheck` 暴露 coverage 摘要

### Requirement: Jaeger Alignment

- `service.name` / `service.version` Resource 属性
- `devrix.layer` / `devrix.component` span 属性
- OTLP ScopeSpans.scope.name 取自 `devrix.component`

### Requirement: Log-Trace-LLM Correlation

- slog `traceId`/`spanId` 注入（`ContextHandler`）
- LLM JSONL `trace_id`/`span_id` 字段

### Requirement: GenAI Semantic Attributes & Metrics

- Span 双写 `gen_ai.*` + `llm.*`
- `devrix_gen_ai.client.token.usage` 含 `cache_read`/`reasoning` 细分

### Requirement: Tool Latency & Compression Metrics

- `devrix_tool_latency` Histogram
- `devrix_compression_ratio` Histogram
- `compression.trigger_reason` / `compression.ratio` span attrs

### Requirement: Session Incident Export

- `devrix debug export --session <id>` schema v1 bundle

### Requirement: W3C Baggage Propagation

- Gateway 入站 `session.id` / `user.id`
- CLI 子进程 `TRACEPARENT` + `BAGGAGE` 环境变量

### Requirement: Harness Bootstrap Spans (条件)

- `context.harness.*` + `context.system_prompt.build` 作为 `context.process` 子 span

---

## MODIFIED

| Item | Change | Reason |
|------|--------|--------|
| Canonical Trace Tree | PEV 层级 → QueryLoop 层级 | D2-S1 PEV 退役，D2-S10 QueryLoop 主路径 |
| Operation 总数 | 44+ → 56 | 新增 query/tool/task/orchestration 族 |
| `context.plan.generate` | 从 PEV plan 迁移到 `task.plan.generate` | Plan 能力重构到 task 子系统 |
| Coverage 文档 | 移除 pev_engine 组件 | 对齐现行 Registry |
| A/F/T 注册表 | 骨架 → 完整代码映射 | DSAFT 文档规范对齐 |

---

## REMOVED / RETIRED

| Item | 退役日期 | 原因 |
|------|----------|------|
| `context.pev.run` | 2026-06-13 | PEV 引擎下线 |
| `context.pev.iteration` | 2026-06-13 | 由 `query.loop.turn` 替代 |
| `context.pev.llm_call` | 2026-06-13 | 由 `query.loop.llm.call` 替代 |
| `context.pev.tool_execute` | 2026-06-13 | 由 `tool.execute.single` 替代 |
| `context.pev.verify` | 2026-06-13 | Verify 逻辑重构 |
| `context.pev.synthesis` | 2026-06-13 | QueryLoop 内置合成 |
| `context.pev.permission_check` | 2026-06-13 | 由 `tool.execute.permission` 替代 |
| `context.milestone.run` | 2026-06-13 | Milestone 重构到 task 子系统 |
| `pev_engine` component | 2026-06-13 | Registry/文档移除 |
| PEV Span Hierarchy requirements | 2026-06-13 | spec.md V2.0 标记 RETIRED |
| Custom traceId `{adapter}:{session}:{msg}` | V1.2 | W3C 32-char hex 替代 |
| `communication/metrics/collector.go` Session gauge | V1.3 | `SessionBridge.ActiveSessions` 替代 |

---

## Technical Notes

### File Structure (current)

```
internal/layers/observability/
├── observability.go          # Facade: New, Shutdown, HealthCheck
├── bridge.go                 # Bridge, ToolBridge, SessionBridge
├── config.go, load.go        # 配置加载
├── genai_tokens.go           # GenAI token metrics
├── llm_log.go                # LLM JSONL capture
├── health.go                 # Health endpoint
├── tracer/                   # D5-S1
├── metrics/                  # D5-S2
├── logger/                   # D5-S3
├── exporter/                 # D5-S4
├── coverage/                 # D5-S5
├── telemetry/                # D5-S6
├── settings/                 # D5-S7
├── incident/                 # D5-S8
└── runtime/                  # D5-S9
```

### Metric Definitions (current)

| Metric | Type | Labels |
|--------|------|--------|
| `devrix_tool_latency` | Histogram | tool, risk_level, status |
| `devrix_compression_ratio` | Histogram | — |
| `devrix_gen_ai.client.token.usage` | Counter | token_type, model |
| `devrix_active_sessions` | Gauge | adapter |
| `devrix_runtime_path_resolved_total` | Counter | path |
| `devrix_llm_*` | Counter/Histogram | provider, model |

### Sampling (current)

生产默认 `always_on`（rate=1.0）。OTLP tail-sampling 规划于 Collector 侧，应用层未实现。

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-07 | V1 MVP: Console exporter, basic spans |
| 2.0.0 | 2026-06-07 | OTel Span 模型、Prometheus metrics |
| 2.1.0–2.9.0 | 2026-06-07–10 | Fix + Jaeger + Coverage + Harness + GenAI + Baggage |
| 3.0.0 | 2026-06-14 | QueryLoop/Orchestration span 族、PEV 退役、DSAFT 全文档同步 |
