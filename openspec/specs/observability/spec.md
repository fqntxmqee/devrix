# Observability Layer Specification

**Capability:** observability
**Change ID:** devrix-observability (archived 2026-06-07), devrix-observability-fix (archived 2026-06-07), devrix-observability-coverage (archived 2026-06-08), devrix-harness-bootstrap (archived 2026-06-10), devrix-observability-enhancement (archived 2026-06-10, P0), devrix-observability-enhancement-p1 (archived 2026-06-10, P1), devrix-observability-enhancement-p2 (archived 2026-06-10, P2), devrix-observability-baggage (archived 2026-06-10, P3), devrix-observability-token-breakdown (archived 2026-06-10, P3)
**Layer:** Observability
**Version:** 1.9.0
**Status:** Canonical — source of truth

---

## Overview

可观察层提供 Tracing、Metrics、结构化 Logging 与 Bridge 集成。V1（DM-20260607-001）建立基础能力；V1.1（DM-20260607-005）修复 Gauge/Histogram 数据错误、Shutdown 丢 Span、UpDownCounter 语义、日志采样与 ConsoleExporter 接口一致性；V1.2 对齐 Jaeger Service/Operation 命名与 span 属性规范；V1.3（DM-20260607-007）新增 Operation 级运行时代码染色、Registry 对账与模块 Span 补全；V1.4（DM-20260609-004）新增 Harness Bootstrap Jaeger Operation 与 info 事件双写规范；V1.5（DM-20260610-001，P0）修复 PEV Span 层级传播、Log-Trace-LLM 关联、OTel `gen_ai.*` 双写；V1.6（DM-20260610-002，P1）补齐 `tool_latency` / `compression_ratio` metrics、压缩决策属性与 session incident export；V1.7（DM-20260610-003，P2）SpanKind 契约测试、prompt 版本哈希、`gen_ai.client.token.usage` metrics、`devrix debug export` 子命令；V1.8（DM-20260610-005，P3）W3C Baggage 传播；V1.9（DM-20260610-007，P3）`cache_read` / `reasoning` token 细分 metrics 与 span attrs。

---

## ADDED Requirements (V1.3 Runtime Coverage)

### Requirement: Operation Registry

系统 MUST 维护 canonical Operation 静态注册表，包含全部已定义 `{layer}.{module}.{action}` 名称及元数据（`layer`、`component`、`since_version`、`instrumented`）。实现于 `internal/layers/observability/coverage/registry.go`。

**Priority**: P0
**L4**: L4-OBS-REGISTRY
**T**: D5-OBS-T16

#### Scenario: Registry 包含 v1.2 与 v1.3 全部 Operation

- GIVEN `coverage.AllOperations()` 被调用
- WHEN 与 `telemetry/names.go` 中 `Op*` 常量集合对比
- THEN 两者 MUST 完全一致（无遗漏、无多余）
- AND 每个 entry 的 `layer` / `component` 与 `LayerAndComponent(operation)` 一致

#### Scenario: 未知 Operation 启动 Span

- GIVEN Tracer 收到不在 Registry 中的 operation 名
- WHEN `Start` 被调用
- THEN 系统 MUST 仍创建 span（向后兼容）
- AND 记录 WARN 日志 `unknown operation`

---

### Requirement: Runtime Operation Hit Counter

`Tracer.Start` MUST 在创建 span 时无条件递增对应 Operation 的进程内命中计数，**不受** trace 采样策略影响。

**Priority**: P0
**L4**: L4-OBS-COVERAGE
**T**: D5-OBS-T17

#### Scenario: 采样关闭仍计数

- GIVEN `tracing.sampling.type = always_off`
- WHEN 任意 instrumented operation 调用 `Tracer.Start`
- THEN 对应 operation 命中计数 MUST 递增
- AND 无 span 导出到 exporter

#### Scenario: 计数并发安全

- GIVEN 100 个并发 goroutine 对同一 operation 调用 `Start`
- WHEN 无 panic
- THEN 最终命中计数 MUST 等于 100

---

### Requirement: Coverage Reconciliation Report

系统 MUST 提供 Coverage 报告，对比 Registry 全集与进程生命周期内命中计数，列出 `operations_zero_hit`。

**Priority**: P0
**L4**: L4-OBS-COVERAGE
**T**: D5-OBS-T17

#### Scenario: Health 摘要暴露

- GIVEN Observability 已启用
- WHEN `HealthCheck()` 被调用
- THEN 响应 MUST 包含 `coverage` 对象
- AND 含 `operations_total`、`operations_hit`、`coverage_ratio`、`zero_hit_count`

---

### Requirement: Extended Module Instrumentation

以下模块 MUST 在关键路径创建 canonical Operation span，并传播 trace context。

**Priority**: P0
**L4**: L4-OBS-INSTRUMENT

| Operation | 触发点 | T 层 |
|-----------|--------|-----|
| `adapter.message.receive` | Feishu 入站消息 | D5-OBS-T15 |
| `context.longterm.recall` | LongTerm recall 注入 | D5-OBS-T13 |
| `context.longterm.store` | LongTerm auto_store | D5-OBS-T13 |
| `context.plan.generate` | PlanEngine 生成 DAG | D5-OBS-T14 |
| `context.milestone.run` | Milestone 执行 | D5-OBS-T14 |
| `gateway.session.lifecycle` | 会话创建/过期 | D5-OBS-T18 |

---

### Requirement: Session Metrics via SessionBridge

Communication Gateway MUST 通过 `SessionBridge.ActiveSessions` 管理会话活跃 Gauge；`communication/metrics/collector.go` 已 Deprecated。

**Priority**: P1
**L4**: L4-OBS-METRICS
**T**: D5-OBS-T18

---

## ADDED Requirements (V1.2 Jaeger Alignment)

### Requirement: Jaeger Service Identity

OTLP Resource MUST 包含 `service.name`（默认 `devrix`，部署形态可覆盖为 `devrix-feishu` / `devrix-cli`）与 `service.version`（来自 `observability.tracing.service_version`）。

**Priority**: P0

#### Scenario: Jaeger service filter

- GIVEN OTLP exporter enabled
- WHEN span is exported
- THEN Jaeger Service 列表显示配置的 `service.name`
- AND Resource 包含 `service.version`

---

### Requirement: Canonical Operation Names

Span name（Jaeger Operation）MUST 使用 `{layer}.{module}.{action}` 格式，常量定义于 `internal/layers/observability/telemetry/names.go`。

**Priority**: P0

| Operation | Layer | span.kind | 必填 Attributes |
|-----------|-------|-----------|-----------------|
| `gateway.message.receive` | communication | SERVER | `session.id`, `message.len`, `devrix.layer`, `devrix.component` |
| `context.process` | context | INTERNAL | `session.id`, `message.len` |
| `context.snapshot.load` | context | INTERNAL | — |
| `context.compression.run` | context | INTERNAL | `context.tokens_before`, `context.tokens_after` |
| `llm.stream` | llm | CLIENT | `llm.provider`, `llm.model`, `llm.tokens.*`, `llm.latency_ms`, `llm.status` |
| `adapter.message.receive` | communication | SERVER | `adapter`, `message.len` |
| `context.plan.generate` | context | INTERNAL | `plan.task_id`, `plan.milestone_count` |
| `context.milestone.run` | context | INTERNAL | `plan.task_id`, `milestone.id` |
| `context.longterm.recall` | context | INTERNAL | `longterm.topic`, `longterm.entries` |
| `context.longterm.store` | context | INTERNAL | `longterm.topic` |
| `gateway.session.lifecycle` | communication | INTERNAL | `session.action`, `session.id`, `adapter` |

---

### Requirement: Devrix Layer Attributes

每个 span MUST 包含 `devrix.layer`（`communication` \| `context` \| `llm`）与 `devrix.component`（`gateway` \| `context_engine` \| `harness` \| `llm_gateway`），由 `telemetry.SpanAttrs` 注入。

> **RETIRED（2026-06-13）**：`context.pev.*` operation 族随 PEV 引擎下线，不再注册于 coverage registry。

**Priority**: P0

---

### Requirement: OTLP Instrumentation Scope

OTLP `ScopeSpans.scope.name` MUST 取自 span 的 `devrix.component` 属性（缺省 `devrix`），便于 Jaeger 按组件过滤。

**Priority**: P1

---

## ADDED Requirements (V1.1 Fix)

### Requirement: Gauge Numeric Correctness

Gauge MUST 使用 mutex 保护 float64 读写，`Set`/`Add`/`Sub`/`Inc`/`Dec` 结果精确。

**Priority**: P0
**T**: D5-OBS-FIX-T01

---

### Requirement: Histogram Bucket Correctness

Histogram `Observe` MUST 仅递增第一个匹配桶；Prometheus 输出 MUST 正确累积各 `le` 桶与 `+Inf` 计数。

**Priority**: P0
**T**: D5-OBS-FIX-T02

---

### Requirement: Tracer Shutdown Flush

`TracerProvider.Shutdown` MUST 遍历 active spans、调用 `End` 并刷写至 exporter，避免 pending span 丢失。

**Priority**: P0
**T**: D5-OBS-FIX-T03

---

### Requirement: Observability Graceful Shutdown

`Observability.Shutdown` MUST 关闭 TracerProvider 与 Logger（`Close()`），错误聚合返回。

**Priority**: P0
**T**: D5-OBS-FIX-T04

---

### Requirement: Int64UpDownCounter Semantics

`Meter.Int64UpDownCounter` MUST 返回 Gauge（可增减），用于 Session 活跃数等场景。

**Priority**: P0
**T**: D5-OBS-FIX-T05

---

### Requirement: Error Log Stack Trace

结构化日志在 `error` 字段为 error 类型时 MUST 附加 `stack` 字段（`debug.Stack()`）。

**Priority**: P1
**T**: D5-OBS-FIX-T06

---

### Requirement: Per-Span Log Sampling

Logger MUST 遵守 `max_entries_per_span` 配置，超限时丢弃并发出 WARN。

**Priority**: P1
**T**: D5-OBS-FIX-T07

---

### Requirement: ConsoleExporter SpanExporter

`ConsoleExporter` MUST 直接实现 `SpanExporter` 接口（`Export(ctx, span)`），无需 adapter。

**Priority**: P2
**T**: D5-OBS-FIX-T08

---

## ADDED Requirements (V1.4 Harness Bootstrap)

### Requirement: Harness Bootstrap Jaeger Operations

Harness Bootstrap 相关 Span MUST 使用 `{layer}.{module}.{action}` canonical 名称，常量定义于 `telemetry/names.go`，并登记于 `coverage/registry.go`（`Instrumented: true`，`SinceVersion: "2.1.0"`）。

**Priority**: P0
**Rationale**: Harness 多阶段编排需可追踪；Jaeger 过滤依赖 canonical Operation
**L4 映射**: L4-OBS-REGISTRY, L4-OBS-COVERAGE
**T 映射**: D2-S9-T11, D5-S5-T02

#### Scenario: Registry includes harness operations

- GIVEN `coverage.AllOperations()` is loaded
- WHEN comparing to `telemetry/names.go` harness constants
- THEN all harness operation names exist with `Layer=context`, `Component=context_engine`
- AND `registry_test` expected list includes each harness operation

#### Scenario: Span hierarchy under context.process

- GIVEN harness enabled and OTLP/console tracing enabled
- WHEN `ContextEngine.Process` completes one turn
- THEN span `context.process` exists
- AND child span `context.harness.bootstrap.run` parent is `context.process` (first Process only)
- AND child span `context.system_prompt.build` parent is `context.process` and precedes `context.pev.run`
- AND child spans have `devrix.layer=context` and `devrix.component=context_engine`

#### Scenario: Bootstrap stage spans with ctx propagation

- GIVEN harness bootstrap runs with prefetch and tool_pool stages
- WHEN bootstrap completes
- THEN span `context.harness.bootstrap.stage` is emitted per stage
- AND each stage span parent MUST be `context.harness.bootstrap.run`
- AND each stage span has attribute `harness.stage` ∈ {prefetch, guards, setup, deferred_init, tool_pool}
- AND `context.harness.tool_pool` span includes `harness.tools.before` and `harness.tools.after`

#### Scenario: System prompt build span attributes

- GIVEN SystemPromptAssembler.Build completes
- WHEN span `context.system_prompt.build` ends
- THEN attributes include `system_prompt.total_tokens`, `system_prompt.memory_truncated`
- AND attributes include `system_prompt.layer3_tokens` and comma-separated `system_prompt.blocks`

#### Scenario: Preflight span without score cardinality explosion

- GIVEN preflight enabled with warnings
- WHEN `context.harness.preflight` span ends
- THEN attributes include `preflight.mode`, `preflight.warning_count`
- AND attributes MUST NOT include unbounded label values (no raw user message)

#### Scenario: harness.disabled skips harness spans

- GIVEN `context_engine.harness.enabled=false`
- WHEN Process runs
- THEN spans matching `context.harness.*` are NOT created
- AND span `context.system_prompt.build` is NOT created
- AND legacy `context.system_prompt.load` behavior unchanged

---

### Requirement: Harness Bootstrap Info Events

Bootstrap 各阶段 MUST 产生 info 事件（与 span 双写），供 Adapter 四流展示。

**Priority**: P1
**Rationale**: 用户需在 CLI/飞书看到 bootstrap 进度，不仅 Jaeger
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-HARNESS
**T 映射**: D2-S9-T08

#### Scenario: Bootstrap stages observable via info events

- GIVEN harness enabled and observability bridge configured
- WHEN bootstrap runs
- THEN info events are emitted per stage with metadata `tools.before`, `tools.after`, `trusted`
- AND event metadata aligns with corresponding span attributes

---

## ADDED Requirements (V1.5 AI Debug Readiness — P0)

### Requirement: Canonical PEV Span Hierarchy (RETIRED)

> **2026-06-13**：PEV 引擎与 `context.pev.*` span 族已下线。现行 QueryLoop 路径以 `context.process` + `llm.stream` 为主。

PEV 执行链曾要求 Canonical Trace Tree：`context.pev.iteration` → `context.pev.llm_call` → `llm.stream`。**本 requirement 不再适用于生产路径。**

**Priority**: P0
**L4 映射**: L4-OBS-SPAN-TREE
**T 映射**: D5-OBS-TRACE-T04

#### Scenario: LLM call nested under iteration

- GIVEN PEV runs one iteration with LLM call
- WHEN spans are collected after Process
- THEN `context.pev.llm_call` parent MUST be `context.pev.iteration`
- AND `llm.stream` parent MUST be `context.pev.llm_call`
- AND iteration span ends before next iteration starts (no overlapping defer)

#### Scenario: LLM ChatStream receives span context

- GIVEN observability tracing enabled
- WHEN PEV calls `ChatStream`
- THEN ctx MUST include `context.pev.llm_call` span context
- AND downstream LLM gateway spans share the same trace_id

---

### Requirement: Log-Trace-LLM Correlation

slog 与 LLM JSONL MUST 携带 trace_id/span_id，与 Jaeger span 可交叉引用。

**Priority**: P0
**L4 映射**: L4-OBS-LOG-CORR
**T 映射**: D5-OBS-TRACE-T05

#### Scenario: slog injects traceId from context

- GIVEN a recording span in context
- WHEN slog.InfoContext is called
- THEN log output includes traceId and spanId matching the span

#### Scenario: LLM JSONL includes trace_id

- GIVEN `observability.llm.log_content=true`
- WHEN LLM request/response is logged to JSONL
- THEN each record includes trace_id and span_id fields

---

### Requirement: GenAI Semantic Attributes

LLM call spans MUST 双写 OTel `gen_ai.*` 属性（与现有 `llm.*` 并存）。

**Priority**: P0
**T 映射**: D5-OBS-GENAI-TATTR

#### Scenario: gen_ai attributes on LLM span

- GIVEN PEV completes an LLM call
- WHEN `context.pev.llm_call` span ends
- THEN attributes include `gen_ai.request.model`, `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`
- AND include `gen_ai.agent.name` and `gen_ai.conversation.id`

---

### Requirement: Verify Failure Semantics (partial P1)

Verify 失败时 span MUST 携带可读 `verify.failure_reason`。

**Priority**: P1
**T 映射**: D5-OBS-DECISION-T01

#### Scenario: Verify failure reason on span

- GIVEN verify does not pass
- WHEN `context.pev.verify` span ends
- THEN attribute `verify.failure_reason` is set
- AND `verify.passed=false`

---

## ADDED Requirements (V1.6 P1 Metrics & Export)

### Requirement: Tool Latency Histogram

系统 MUST 注册 `devrix_tool_latency` Histogram，labels：`tool`、`risk_level`、`status`；PEV 工具执行完成后 MUST observe 秒级延迟。

**Priority**: P1
**T 映射**: D5-OBS-METRICS-T01

#### Scenario: Tool latency recorded after execution

- GIVEN observability metrics enabled
- WHEN PEV executes a tool successfully
- THEN `devrix_tool_latency{tool, risk_level, status="ok"}` receives an observation

---

### Requirement: Compression Ratio Histogram

系统 MUST 注册 `devrix_compression_ratio` Histogram；上下文压缩成功后 MUST observe `CompressedTokens/OriginalTokens`。

**Priority**: P1
**T 映射**: D5-OBS-METRICS-T02

#### Scenario: Compression ratio observed

- GIVEN compression pipeline reduces token count
- WHEN `context.compression.run` completes successfully
- THEN `devrix_compression_ratio` receives an observation in (0,1]

---

### Requirement: Compression Decision Attributes

`context.compression.run` span MUST 携带 `compression.trigger_reason` 与 `compression.ratio`。

**Priority**: P1
**T 映射**: D5-OBS-DECISION-T02

#### Scenario: Compression span decision attrs

- GIVEN compression triggered by token budget
- WHEN compression span ends
- THEN `compression.trigger_reason=token_budget_exceeded`
- AND `compression.ratio` reflects token reduction ratio

---

### Requirement: Session Incident Export

系统 MUST 提供 CLI `debug-export --session <id>`，输出 schema v1 JSON bundle（含 `llm_rounds`、可选 `trace` 与 `coverage_hits`）。

**Priority**: P1
**T 映射**: D5-OBS-EXPORT-T01

#### Scenario: Export valid incident bundle

- GIVEN LLM JSONL exists for session
- WHEN `debug-export --session {id}` runs
- THEN stdout/file contains valid JSON with `schema_version=1.0` and `llm_rounds` array

---

## ADDED Requirements (V1.7 P2 SpanKind / Prompt / Token Metrics / Debug CLI)

### Requirement: SpanKind Contract

关键 span MUST 使用正确 SpanKind：`gateway.message.receive` = SERVER；`context.pev.llm_call` / `llm.stream` / `llm.adapter.stream` = CLIENT；其余引擎内操作 = INTERNAL。

**Priority**: P2
**T 映射**: D5-OBS-TRACE-T06

#### Scenario: Integration asserts SpanKind

- GIVEN full PEV request via gateway
- WHEN spans are exported to memory
- THEN `gateway.message.receive` has SpanKind SERVER
- AND `context.pev.llm_call` has SpanKind CLIENT

---

### Requirement: Prompt Version Metadata

`context.system_prompt.build` span MUST 携带 `gen_ai.prompt.version`、`gen_ai.prompt.template_hash`；可选 `gen_ai.prompt.agents_md_hash`。

**Priority**: P2
**T 映射**: D5-OBS-DECISION-T03

#### Scenario: Stable template hash

- GIVEN identical SystemPromptAssembler inputs
- WHEN Build runs twice
- THEN `template_hash` is identical

---

### Requirement: GenAI Token Usage Metrics

系统 MUST 注册 `devrix_gen_ai.client.token.usage` Counter（labels: `token_type`, `model`），LLM 调用成功后按 input/output 分别 Add。

**Priority**: P2
**T 映射**: D5-OBS-METRICS-T03

---

### Requirement: Debug Export Subcommand

主二进制 MUST 支持 `devrix debug export --session <id>`，行为与 `cmd/debug-export` 一致。

**Priority**: P2
**T 映射**: D5-OBS-EXPORT-T02

---

## ADDED Requirements (V1.8 Baggage)

### Requirement: W3C Baggage Propagation

系统 MUST 通过 W3C `baggage` 头在 context 与 HTTP/子进程边界传播业务键值；Gateway 入站 MUST 写入 `session.id`，并在 `user.id` 可用时写入 baggage。

**Priority**: P2
**L4**: L4-OBS-BAGGAGE
**T**: D5-OBS-TRACE-T03

#### Scenario: Propagator 往返 baggage

- GIVEN context 含有效 span 与 baggage `session.id=sess_1`
- WHEN `Propagator.Inject` 后 `ExtractContext`
- THEN `baggage` 头 MUST 非空
- AND 提取后 context MUST 含 `session.id=sess_1`

#### Scenario: CLI 子进程继承传播环境

- GIVEN 父 context 含 trace 与 baggage
- WHEN `CLIAgentTool` 创建新子进程
- THEN 子进程环境 MUST 含 `TRACEPARENT` 与 `BAGGAGE`

---

## ADDED Requirements (V1.9 Token Breakdown)

### Requirement: GenAI Token Type Breakdown

系统 MUST 在 provider 返回 usage details 时，将 `cache_read` 与 `reasoning` 分别写入 `devrix_gen_ai.client.token.usage`（label `token_type`）及 LLM span 属性。

**Priority**: P2
**L4**: L4-OBS-METRICS
**T**: D5-OBS-METRICS-T04

#### Scenario: Provider 返回 cached/reasoning tokens

- GIVEN SSE usage 含 `prompt_tokens_details.cached_tokens` 与 `completion_tokens_details.reasoning_tokens`
- WHEN LLM 调用完成
- THEN metrics MUST 含 `token_type=cache_read` 与 `token_type=reasoning`
- AND span MUST 含 `gen_ai.usage.cache_read.input_tokens` 与 `gen_ai.usage.reasoning.output_tokens`

#### Scenario: Provider 无 details 字段

- GIVEN usage 仅含 prompt/completion tokens
- WHEN LLM 调用完成
- THEN 行为 MUST 与 V1.7 一致（仅 input/output）

---

## Inherited Requirements (V1)

V1 基线能力（Tracing Span 生命周期、Counter/Histogram 注册、JSON/Text 日志、Bridge 集成）见归档包 `openspec/archive/2026-06-07-devrix-observability/`。
