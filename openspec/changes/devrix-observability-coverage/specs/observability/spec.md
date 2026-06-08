# Observability Layer Specification (Delta v1.3.0)

**Capability:** observability
**Change ID:** devrix-observability-coverage
**Demand ID:** DM-20260607-007
**Parent Version:** 1.2.0
**Target Version:** 1.3.0
**Status:** Delta — pending S7 merge

---

## Overview

V1.3 在 V1.2 Jaeger 对齐基础上，新增 **Operation 级运行时代码染色** 与 **Registry 对账**，并补全 LongTerm / Plan / Milestone / Adapter 等模块 Span 埋点，支撑线上闲置功能路径识别。

---

## ADDED Requirements (V1.3 Runtime Coverage)

### Requirement: Operation Registry

系统 MUST 维护 canonical Operation 静态注册表，包含全部已定义 `{layer}.{module}.{action}` 名称及元数据（`layer`、`component`、`since_version`、`instrumented`）。

**Priority**: P0
**L4**: L4-OBS-REGISTRY
**L5**: L5-OBS-16

#### Scenario: Registry 包含 v1.2 与 v1.3 全部 Operation

- GIVEN `telemetry.AllOperations()` 被调用
- WHEN 与 `telemetry/names.go` 中 `Op*` 常量集合对比
- THEN 两者 MUST 完全一致（无遗漏、无多余）
- AND 每个 entry 的 `layer` / `component` 与 `LayerAndComponent(operation)` 一致

#### Scenario: 未知 Operation 启动 Span

- GIVEN Tracer 收到不在 Registry 中的 operation 名
- WHEN `Start` 被调用
- THEN 系统 MUST 仍创建 span（向后兼容）
- AND 记录 WARN 日志 `unknown operation`
- AND 命中计数记入 `__unknown__` 桶

---

### Requirement: Runtime Operation Hit Counter

`Tracer.Start` MUST 在创建 span 时无条件递增对应 Operation 的进程内命中计数，**不受** trace 采样策略影响。

**Priority**: P0
**L4**: L4-OBS-COVERAGE
**L5**: L5-OBS-17

#### Scenario: 采样关闭仍计数

- GIVEN `tracing.sampling.type = always_off`
- WHEN Gateway 处理一条入站消息
- THEN `gateway.message.receive` 命中计数 MUST 递增
- AND 无 span 导出到 exporter

#### Scenario: 计数幂等与并发安全

- GIVEN 100 个并发 goroutine 对同一 operation 调用 `Start`
- WHEN 无 panic
- THEN 最终命中计数 MUST 等于 100

---

### Requirement: Coverage Reconciliation Report

系统 MUST 提供 Coverage 报告，对比 Registry 全集与进程生命周期内命中计数，列出 `operations_zero_hit`。

**Priority**: P0
**L4**: L4-OBS-COVERAGE
**L5**: L5-OBS-17

#### Scenario: 生成 zero_hit 列表

- GIVEN Registry 含 N 个 `instrumented=true` 的 Operation
- AND 仅 M 个 Operation 曾被命中（M < N）
- WHEN `Coverage.Report()` 被调用
- THEN `operations_total` MUST 等于 N
- AND `operations_hit` MUST 等于 M
- AND `operations_zero_hit` MUST 包含未命中的 N-M 个 Operation 元数据
- AND `coverage_ratio` MUST 等于 M/N（保留 3 位小数）

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
| `adapter.message.receive` | Feishu 入站消息解析完成 | L5-OBS-15 |
| `context.longterm.recall` | LongTerm recall 注入 system prompt | L5-OBS-13 |
| `context.longterm.store` | LongTerm auto_store 写入 | L5-OBS-13 |
| `context.plan.generate` | PlanEngine 生成 milestone DAG | L5-OBS-14 |
| `context.milestone.run` | MilestoneRunner 执行单个 milestone | L5-OBS-14 |
| `gateway.session.lifecycle` | 会话创建或过期 | L5-OBS-18（P1 属性） |

#### Scenario: LongTerm recall span

- GIVEN `longterm.enabled=true` 且存在可 recall 条目
- WHEN `EnrichWithLongTermRecall` 成功注入
- THEN MUST 产生 `context.longterm.recall` span
- AND span 含 `devrix.layer=context`、`devrix.component=context_engine`
- AND 含 `longterm.entries` 属性

#### Scenario: Plan generate span

- GIVEN `plan.enabled=true`
- WHEN PlanEngine 成功解析 LLM 输出并持久化 DAG
- THEN MUST 产生 `context.plan.generate` span
- AND 含 `plan.task_id`、`plan.milestone_count`

#### Scenario: Feishu adapter span

- GIVEN Feishu WebSocket 收到用户文本消息
- WHEN adapter 解析为 `InboundMessage`
- THEN MUST 产生 `adapter.message.receive` span
- AND 含 `adapter=feishu`、`message.len`
- AND 后续 Gateway 处理继承同一 traceId

#### Scenario: Milestone run span

- GIVEN MilestoneRunner 开始执行 milestone M
- WHEN `Run` 进入 execute→verify 循环
- THEN MUST 产生 `context.milestone.run` span
- AND 含 `milestone.id`、`plan.task_id`

---

### Requirement: Session Metrics via SessionBridge

Communication Gateway MUST 通过 `SessionBridge.ActiveSessions` 管理会话活跃 Gauge，不再使用 `communication/metrics/collector.go` 作为主路径。

**Priority**: P1
**L4**: L4-OBS-METRICS
**L5**: L5-OBS-18

#### Scenario: 会话创建递增 Gauge

- GIVEN Observability metrics 已启用
- WHEN 新会话被创建
- THEN `active_sessions{adapter}` Gauge MUST 递增 1
- AND 指标由 observability Meter 注册（非 legacy collector）

#### Scenario: 会话过期递减 Gauge

- GIVEN 活跃会话存在
- WHEN 会话因 idle timeout 过期
- THEN `active_sessions{adapter}` Gauge MUST 递减 1

---

## MODIFIED Requirements (V1.3)

### Requirement: Canonical Operation Names

Span name（Jaeger Operation）MUST 使用 `{layer}.{module}.{action}` 格式。V1.3 在 V1.2 表基础上 **追加**：

| Operation | Layer | span.kind | 必填 Attributes |
|-----------|-------|-----------|-----------------|
| `adapter.message.receive` | communication | SERVER | `adapter`, `message.len`, `devrix.layer`, `devrix.component` |
| `context.plan.generate` | context | INTERNAL | `plan.task_id`, `plan.milestone_count` |
| `context.milestone.run` | context | INTERNAL | `plan.task_id`, `milestone.id` |
| `context.longterm.recall` | context | INTERNAL | `longterm.topic`, `longterm.entries` |
| `context.longterm.store` | context | INTERNAL | `longterm.topic` |
| `gateway.session.lifecycle` | communication | INTERNAL | `session.action`, `session.id`, `adapter` |

V1.2 已有 11 个 Operation **不变**。

---

## Inherited Requirements

V1.1 Fix（L5-OBS-FIX-01~08）与 V1.2 Jaeger Alignment 要求原样继承，见 `openspec/specs/observability/spec.md` v1.2.0。

---

## Out of Scope (V1.3)

- Go `runtime/coverage` 函数级覆盖
- `net/http/pprof`
- Jaeger Query API 自动对账
- 自动插桩 / OTel SDK 替换
