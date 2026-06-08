# Observability Layer Specification

**Capability:** observability
**Change ID:** devrix-observability (archived 2026-06-07), devrix-observability-fix (archived 2026-06-07), devrix-observability-coverage (archived 2026-06-08)
**Layer:** Observability
**Version:** 1.3.0
**Status:** Canonical — source of truth

---

## Overview

可观察层提供 Tracing、Metrics、结构化 Logging 与 Bridge 集成。V1（DM-20260607-001）建立基础能力；V1.1（DM-20260607-005）修复 Gauge/Histogram 数据错误、Shutdown 丢 Span、UpDownCounter 语义、日志采样与 ConsoleExporter 接口一致性；V1.2 对齐 Jaeger Service/Operation 命名与 span 属性规范；V1.3（DM-20260607-007）新增 Operation 级运行时代码染色、Registry 对账与模块 Span 补全。

---

## ADDED Requirements (V1.3 Runtime Coverage)

### Requirement: Operation Registry

系统 MUST 维护 canonical Operation 静态注册表，包含全部已定义 `{layer}.{module}.{action}` 名称及元数据（`layer`、`component`、`since_version`、`instrumented`）。实现于 `internal/layers/observability/coverage/registry.go`。

**Priority**: P0
**L4**: L4-OBS-REGISTRY
**L5**: L5-OBS-16

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
**L5**: L5-OBS-17

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
**L5**: L5-OBS-17

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

| Operation | 触发点 | L5 |
|-----------|--------|-----|
| `adapter.message.receive` | Feishu 入站消息 | L5-OBS-15 |
| `context.longterm.recall` | LongTerm recall 注入 | L5-OBS-13 |
| `context.longterm.store` | LongTerm auto_store | L5-OBS-13 |
| `context.plan.generate` | PlanEngine 生成 DAG | L5-OBS-14 |
| `context.milestone.run` | Milestone 执行 | L5-OBS-14 |
| `gateway.session.lifecycle` | 会话创建/过期 | L5-OBS-18 |

---

### Requirement: Session Metrics via SessionBridge

Communication Gateway MUST 通过 `SessionBridge.ActiveSessions` 管理会话活跃 Gauge；`communication/metrics/collector.go` 已 Deprecated。

**Priority**: P1
**L4**: L4-OBS-METRICS
**L5**: L5-OBS-18

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
| `context.pev.run` | context | INTERNAL | `pev.max_iterations` |
| `context.pev.llm_call` | context | CLIENT | `pev.iteration`, `llm.model`, `llm.tokens.*`, `llm.latency_ms` |
| `context.pev.tool_execute` | context | INTERNAL | `tool.name`, `tool.input`, `tool.output`, `tool.duration_ms` |
| `context.pev.permission_check` | context | INTERNAL | `tool.name`, `permission.result` |
| `context.pev.verify` | context | INTERNAL | `verify.mode`, `verify.passed`, `verify.deviation` |
| `llm.stream` | llm | CLIENT | `llm.provider`, `llm.model`, `llm.tokens.*`, `llm.latency_ms`, `llm.status` |
| `adapter.message.receive` | communication | SERVER | `adapter`, `message.len` |
| `context.plan.generate` | context | INTERNAL | `plan.task_id`, `plan.milestone_count` |
| `context.milestone.run` | context | INTERNAL | `plan.task_id`, `milestone.id` |
| `context.longterm.recall` | context | INTERNAL | `longterm.topic`, `longterm.entries` |
| `context.longterm.store` | context | INTERNAL | `longterm.topic` |
| `gateway.session.lifecycle` | communication | INTERNAL | `session.action`, `session.id`, `adapter` |

---

### Requirement: Devrix Layer Attributes

每个 span MUST 包含 `devrix.layer`（`communication` \| `context` \| `llm`）与 `devrix.component`（`gateway` \| `context_engine` \| `pev_engine` \| `llm_gateway`），由 `telemetry.SpanAttrs` 注入。

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
**L5**: L5-OBS-FIX-01

---

### Requirement: Histogram Bucket Correctness

Histogram `Observe` MUST 仅递增第一个匹配桶；Prometheus 输出 MUST 正确累积各 `le` 桶与 `+Inf` 计数。

**Priority**: P0
**L5**: L5-OBS-FIX-02

---

### Requirement: Tracer Shutdown Flush

`TracerProvider.Shutdown` MUST 遍历 active spans、调用 `End` 并刷写至 exporter，避免 pending span 丢失。

**Priority**: P0
**L5**: L5-OBS-FIX-03

---

### Requirement: Observability Graceful Shutdown

`Observability.Shutdown` MUST 关闭 TracerProvider 与 Logger（`Close()`），错误聚合返回。

**Priority**: P0
**L5**: L5-OBS-FIX-04

---

### Requirement: Int64UpDownCounter Semantics

`Meter.Int64UpDownCounter` MUST 返回 Gauge（可增减），用于 Session 活跃数等场景。

**Priority**: P0
**L5**: L5-OBS-FIX-05

---

### Requirement: Error Log Stack Trace

结构化日志在 `error` 字段为 error 类型时 MUST 附加 `stack` 字段（`debug.Stack()`）。

**Priority**: P1
**L5**: L5-OBS-FIX-06

---

### Requirement: Per-Span Log Sampling

Logger MUST 遵守 `max_entries_per_span` 配置，超限时丢弃并发出 WARN。

**Priority**: P1
**L5**: L5-OBS-FIX-07

---

### Requirement: ConsoleExporter SpanExporter

`ConsoleExporter` MUST 直接实现 `SpanExporter` 接口（`Export(ctx, span)`），无需 adapter。

**Priority**: P2
**L5**: L5-OBS-FIX-08

---

## Inherited Requirements (V1)

V1 基线能力（Tracing Span 生命周期、Counter/Histogram 注册、JSON/Text 日志、Bridge 集成）见归档包 `openspec/archive/2026-06-07-devrix-observability/`。
