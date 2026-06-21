# D5 Observability Layer Specification

**Capability:** observability
**Domain:** D5
**DSAFT Type:** 公共域 (Common Domain)
**Version:** 3.0.0
**Status:** Canonical — source of truth (v2.1 Terminal)
**Last Updated:** 2026-06-19
**Layering Spec:** `openspec/specs/architecture/layering.md`

**Archived Changes:** devrix-observability (2026-06-07), devrix-observability-fix (2026-06-07), devrix-observability-coverage (2026-06-08), devrix-harness-bootstrap (2026-06-10), devrix-observability-enhancement P0–P3 (2026-06-10), devrix-queryloop-spans-v1.1

> **阅读优先级：MUST** — 本文档是 D5 的 Gherkin 验收 SoT。先读 `d5-domain.md`（领域全景），再读本文档（验收用例）。

---

## Overview

D5 可观测性域提供 Tracing、Metrics、结构化 Logging、Bridge 集成、Operation 代码染色（Coverage）、诊断工具链（Tracker / Doctor / FaultInject / DebugFilter）、Session Incident Export 与运行时路径指标。对齐 OpenTelemetry 语义，通过 `{layer}.{module}.{action}` canonical Operation 名称贯穿 Jaeger/OTLP 与 Prometheus。

**现行主路径（2026-06-19）：** D7 Turn 主路径（`orchestration.turn.*`）。**`query.loop.*` span 族已退役**（DM-20260618-010），仅保留 RETIRED 节追溯。

| 版本里程碑 | 能力 |
|-----------|------|
| V1.0 | Tracer / Metrics / Logger / Bridge 基础 |
| V1.1 | Gauge/Histogram 修复、Shutdown flush、UpDownCounter |
| V1.2 | Jaeger Service/Operation 命名对齐 |
| V1.3 | Operation Registry + Runtime Hit Counter + Coverage 对账 |
| V1.4 | Harness Bootstrap span + info 事件双写 |
| V1.5–V1.7 | Log-Trace-LLM 关联、`gen_ai.*` 双写、tool_latency/compression metrics、debug export |
| V1.8–V1.9 | W3C Baggage、`cache_read`/`reasoning` token 细分 |
| V2.0 | QueryLoop span 族、Orchestration span、Runtime path metric；PEV 文档退役 |
| **V2.1** | **Terminal：S21–S24 号段冻结、bridge 删除、诊断工具链 A/F/T 注册、D7 Turn 主路径** |

---

## DSAFT 结构

| 层级 | ID | 名称 | 说明 |
|------|-----|------|------|
| D | D5 | Observability | 公共域/裁判域，横向 Tracing/Metrics/Logging/诊断能力 |
| S | D5-S21 | Instrument | 遥测生成：Span + Metric + Log + 属性构建 |
| S | D5-S22 | Export | 遥测导出：OTLP/Prometheus/Console |
| S | D5-S23 | Diagnose | 诊断辅助：Coverage + Incident + Doctor + Tracker + FaultInject |
| S | D5-S24 | Configure | 配置与运行时管理：yaml 切换 + runtime path 计数 |
| S | D5-S0 | Facade | 横切：Init/Shutdown/Bridge/SessionGauge |

---

## Scenarios

| ID | Scenario | Responsibility | Status |
|----|----------|----------------|--------|
| D5-S21 | Instrument | Span 生命周期、Metrics 注册/Label 白名单、结构化日志、DebugFilter、W3C/Baggage 传播 | IMPLEMENTED |
| D5-S22 | Export | Console/OTLP/Memory SpanExporter | IMPLEMENTED |
| D5-S23 | Diagnose | Coverage 对账、Incident 导出、Doctor 环境检查、Tracker 代码变更追踪、FaultInject(test) | IMPLEMENTED |
| D5-S24 | Configure | 配置加载/校验、Runtime path 指标 | IMPLEMENTED |
| D5-S0 | Facade | Init/Shutdown/Bridge/SessionGauge；观测失败不阻断业务 | IMPLEMENTED |

---

## Architecture

```
D1 Gateway ──→ root span (gateway.message.receive)
                    │
D7 Orchestration ──→ orchestration.turn.run
                    ├─→ orchestration.turn.iteration
                    ├─→ orchestration.llm.invoke
                    │   └─→ llm.stream (D3)
                    └─→ context.process (D2, caller=d7)
                    │
D2 ContextEngine ──→ context.process
                    ├─→ context.compression.run (+ step spans)
                    ├─→ context.harness.* (legacy path, harness.enabled)
                    └─→ tool.execute.*
                    │
D3 LLMGateway ─────→ llm.stream → llm.adapter.stream
                    │
D4 MultiAgent ─────→ agent.run / agent.tool.call / agent.fork|join
                    │
                    ▼
         ┌──────────────────────────────────────┐
         │  Observability Facade (D5)             │
         │  Instrument │ Export │ Diagnose        │
         │  Configure │ Bridge │ Incident         │
         └──────────────────────────────────────┘
                    │
                    ▼
         Console / OTLP / Prometheus / ~/.devrix/coverage/
```

## Cross-Domain Dependencies

| Domain | 依赖内容 | 使用位置 |
|--------|---------|---------|
| D1 Communication | 入站创建 root span、`SessionBridge.ActiveSessions` | `gateway/`, `adapters/` |
| D2 Context Engine | `observability.Bridge` tracer/meter/logger | `engine.go`, `enforce/`, `prepare/` |
| D3 LLM Gateway | `llm.stream` span、`RecordGenAITokenUsage` | `gateway/gateway.go`, `adapter/` |
| D4 Multi-Agent | `agent.*` spans、Fork policy metrics sink | `multiagent/observability/` |
| D7 Orchestration | `orchestration.*` spans | `orchestration/turn/`, `sessionorchestrator/` |
| Shared | `config`, `types` | 全子包 |

## Package Map

| 子包 / 根文件 | 场景 | 职责 |
|--------------|------|------|
| `observability.go`, `bridge.go` | D5-S0 | Facade、`Bridge`/`ToolBridge`/`SessionBridge` |
| `instrument/tracer/` | D5-S21 | TracerProvider、Span、采样、W3C Propagator、Baggage |
| `instrument/metrics/` | D5-S21 | Meter、Counter/Histogram/Gauge、Prometheus 导出、genai_tokens |
| `instrument/logger/` | D5-S21 | StructuredLogger、slog 桥接、脱敏、采样、DebugFilter |
| `instrument/telemetry/` | D5-S21 | `Op*` 常量、`SpanAttrs`、`LayerAndComponent`、`GenAI*` attrs |
| `export/` | D5-S22 | Console、OTLP、Memory、Null exporter |
| `diagnose/coverage/` | D5-S23 | Registry、Counter、Reporter、Persistence、CLI |
| `diagnose/incident/` | D5-S23 | `BuildBundle` schema v1、LLM JSONL |
| `diagnose/doctor/` | D5-S23 | 环境健康检查 |
| `diagnose/tracker/` | D5-S23 | 代码变更追踪、LRU diff、linter 集成 |
| `diagnose/faultinject/` | D5-S23 | 故障注入（testbuild only） |
| `configure/settings/` | D5-S24 | TracingConfig、MetricsConfig、OTLP 配置 |
| `configure/runtime/` | D5-S24 | PathResolver、`runtime_path_resolved_total` |
| `health.go`, `config.go`, `load.go` | — | HealthCheck、配置加载 |

---

## Key Design Patterns

1. **Canonical Operation Naming**: Span name = Jaeger Operation = `{layer}.{module}.{action}`，常量于 `instrument/telemetry/names.go`，注册于 `diagnose/coverage/registry.go`（当前 **56** 条）。
2. **Coverage 独立于采样**: `Tracer.Start` 无条件 `RecordHit`，`always_off` 采样仍计数。
3. **Bridge 零侵入集成**: 各域注入 `*observability.Bridge`，`IsEnabled()` 守卫避免 nil 开销。
4. **Metrics 前缀**: Meter name `devrix` → Prometheus 名 `devrix_{instrument}`（如 `devrix_tool_latency`）。
5. **Log-Trace-LLM 三联**: slog `traceId`/`spanId` + LLM JSONL `trace_id`/`span_id` + span `gen_ai.*` 双写。
6. **Graceful Degradation**: `NewNoOp()` / nil Bridge 时业务路径不受影响；observability 故障标记 `degraded` 不阻断 Process。

---

## Requirements

### Requirement: Operation Registry (P0)

系统 MUST 维护 canonical Operation 静态注册表（`diagnose/coverage/registry.go`），与 `instrument/telemetry/names.go` `Op*` 常量全集一致。

**T:** D5-S23-A01-T01

#### Scenario: Registry 与 names.go 对账

- GIVEN `coverage.AllOperations()` 被调用
- WHEN 与 `registry_test` expected 列表对比
- THEN 两者 MUST 完全一致（当前 56 条）
- AND 每个 entry 的 `layer`/`component` 与 `LayerAndComponent(operation)` 一致

#### Scenario: 未知 Operation 启动 Span

- GIVEN Tracer 收到不在 Registry 中的 operation 名
- WHEN `Start` 被调用
- THEN 系统 MUST 仍创建 span（向后兼容）
- AND 记录 WARN 日志 `unknown operation`

---

### Requirement: Runtime Operation Hit Counter (P0)

`Tracer.Start` MUST 无条件递增对应 Operation 的进程内命中计数，不受 trace 采样影响。

**T:** D5-S23-A01-T02, D5-S23-A01-T03

#### Scenario: 采样关闭仍计数

- GIVEN `tracing.sampling.type = always_off`
- WHEN instrumented operation 调用 `Tracer.Start`
- THEN 命中计数 MUST 递增
- AND 无 span 导出到 exporter

#### Scenario: 计数并发安全

- GIVEN 100 个并发 goroutine 对同一 operation 调用 `Start`
- THEN 最终命中计数 MUST 等于 100

---

### Requirement: Coverage Health Summary (P0)

`HealthCheck()` MUST 暴露 `coverage` 对象：`operations_total`、`operations_hit`、`coverage_ratio`、`zero_hit_count`。

**T:** D5-S23-A01-T02

---

### Requirement: Jaeger Service Identity (P0)

OTLP Resource MUST 包含 `service.name`（默认 `devrix`）与 `service.version`。

---

### Requirement: Layer/Component Encoded in Operation Name

Operation 命名规范（`D{N}_{...}` 前缀）已蕴含 layer 与 component 信息。

- `D1_*` → communication/gateway（`D1_Adapter_*` → communication/adapter）
- `D2_Context_Harness_*` → context/harness（其他 `D2_*` 见 LayerAndComponent 映射表）
- `D3_LLM_Adapter_*` → llm/llm_adapter（其他 `D3_LLM_*` → llm/llm_gateway）
- `D4_*` → agent/agent_tool
- `D6_*` → evolution/validation
- `D7_*` → orchestration/orchestrator

`devrix.layer` / `devrix.component` 不再作为强制 span attribute 注入；需要时调用
`telemetry.LayerAndComponent(operation)` 显式查询（如 OTLP ScopeSpans.scope.name、调试 dump）。

| Layer | Components |
|-------|------------|
| `communication` | `gateway`, `adapter` |
| `context` | `context_engine`, `harness`, `tool_runner`, `plan_agent`, `plan_mode`, `task_manager` |
| `llm` | `llm_gateway`, `llm_adapter` |
| `agent` | `agent_tool` |
| `orchestration` | `orchestrator` |

---

### Requirement: D7 Turn Span Hierarchy (P0)

> **Canonical 主路径（v2.1 Terminal）。** 主路径 LLM↔Tool span 在 D7：

```
gateway.message.receive [SERVER]
└── orchestration.turn.run [INTERNAL]
    └── orchestration.turn.iteration [INTERNAL]
        ├── orchestration.llm.invoke [CLIENT]
        │   └── llm.stream [CLIENT]
        └── context.process [INTERNAL]   ← D2 Prepare (caller=d7)
            └── tool.execute.single [INTERNAL]
```

**T:** D5-S22-A01-T02, D7-S2-A06 span tests

### Requirement: QueryLoop Span Hierarchy — **REMOVED (DM-20260618-010)**

~~QueryLoop 主路径~~ 已删除。历史 `query.loop.*` span 仅作 RETIRED 追溯登记。

---

### Requirement: Diagnostic Tools (P0)

D5-S23 Diagnose 包含以下诊断子能力（S23 子承诺 C3a–C3e）：

| 子承诺 | 能力 | A ID | 时间属性 |
|--------|------|------|---------|
| C3a Coverage | Operation 注册表对账、Runtime Hit、Health zero_hit | D5-S23-A01/A02/A03 | 事中 |
| C3b Incident | Session incident bundle 导出 | D5-S23-A04/A05 | 事后（不可补救） |
| C3c Doctor | 环境健康检查（7 项） | D5-S23-A10 | 事前+事中 |
| C3d Tracker | 代码变更追踪、LRU diff、linter 集成 | D5-S23-A07 | 事中 |
| C3e FaultInject | 故障注入（testbuild only） | D5-S23-A09 | 测试（与生产隔离） |

**T:** D5-S23-A03-T01/T02 (Doctor), D5-S23-A07-T01/T02 (Tracker), D5-S23-A08-T01/T02 (DebugFilter), D5-S23-A09-T01/T02 (FaultInject)

---

### Requirement: Harness Bootstrap Spans (P0, 条件触发)

当 `harness.enabled=true` 时，`context.harness.*` 与 `context.system_prompt.build` span MUST 作为 `context.process` 子 span 创建。

**T:** D5-S23-A01-T05 (integration `context_harness_obs_test.go`)

---

### Requirement: Log-Trace-LLM Correlation (P0)

slog 与 LLM JSONL MUST 携带 `trace_id`/`span_id`，与 Jaeger span 可交叉引用。

**T:** D5-S21-A09-T01 (slog bridge), D5-S23-A04-T01 (LLM JSONL in bundle)

---

### Requirement: GenAI Semantic Attributes (P0)

LLM call spans MUST 双写 OTel `gen_ai.*` 属性（与 `llm.*` 并存）。

**T:** D5-S21-A07-T01

---

### Requirement: Tool Latency & Compression Metrics (P1)

- `devrix_tool_latency` Histogram: labels `tool`, `risk_level`, `status`
- `devrix_compression_ratio` Histogram: 压缩成功后 observe ratio
- `context.compression.run` span: `compression.trigger_reason`, `compression.ratio`

**T:** D5-S21-A05-T06, D5-S21-A05-T03

---

### Requirement: GenAI Token Usage Metrics (P2)

`devrix_gen_ai.client.token.usage` Counter: labels `token_type`（`input`/`output`/`cache_read`/`reasoning`）, `model`。

**T:** D5-S21-A07-T01

---

### Requirement: Session Incident Export (P1)

主二进制 MUST 支持 `devrix debug export --session <id>`，输出 schema v1 JSON（`llm_rounds`, 可选 `trace`, `coverage_hits`）。

**T:** D5-S23-A04-T01, D5-S23-A04-T02

---

### Requirement: W3C Baggage Propagation (P2)

Gateway 入站 MUST 写入 baggage `session.id`；`user.id` 可用时写入。CLI 子进程通过 `TRACEPARENT` + `BAGGAGE` 环境变量继承。

**T:** D5-S21-A03-T03

---

### Requirement: Tracer Shutdown Flush (P0)

`TracerProvider.Shutdown` MUST 结束所有 active spans 并刷写 exporter。

**T:** D5-S21-A01-T01

---

### Requirement: Metrics Correctness (P0)

- Gauge `Set`/`Add`/`Sub`/`Inc`/`Dec` 精确（mutex 保护）
- Histogram 仅递增第一个匹配桶
- `Int64UpDownCounter` 返回 Gauge 语义

**T:** D5-S21-A05-T03, D5-S21-A05-T04, D5-S21-A05-T05

---

### Requirement: Logger Semantics (P1)

- Error 日志 MUST 附加 `stack` 字段
- `max_entries_per_span` 采样超限 MUST 发 WARN

**T:** D5-S21-A08-T02, D5-S21-A08-T03, D5-S21-A08-T04

---

### Requirement: Runtime Path Metric (P1)

`devrix_runtime_path_resolved_total{path="d7_turn|legacy_harness"}` MUST 与 in-process `PathResolver` 同步。

> **v2.1 Terminal:** `legacy_harness` metric help text 标 DEPRECATED。退役计划：v2.1 DEPRECATED → v2.3 自爆机制（详见 `design.md` §12）。

**T:** D5-S24-A03-T01

---

## RETIRED

| Item | 退役日期 | 替代 |
|------|----------|------|
| `context.pev.*` operation 族 | 2026-06-13 | `query.loop.*` → `orchestration.turn.*` |
| PEV Span Hierarchy requirements | 2026-06-13 | QueryLoop Span Hierarchy → D7 Turn Span Hierarchy |
| `context.plan.generate` / `context.milestone.run` (旧 PEV plan) | 2026-06-13 | `task.plan.*` / `task.manager.*` |
| `query.loop.run` / `query.loop.turn` / `query.loop.llm.call` | 2026-06-18 | `orchestration.turn.*` (D7 Turn 主路径) |
| QueryLoop Span Hierarchy requirements | 2026-06-18 | D7 Turn Span Hierarchy |
| `legacy_harness` runtime path | 2026-06-19 | `d7_turn`（DEPRECATED，v2.3 移除） |

---

## Registries

- **A 层**: `a-registry.md` — 30 Activities (v4.0)
- **F 层**: `f-registry.md` — 45 Function Points (v3.0)
- **T 层**: `t-registry.md` — 41 Test Points (v3.2)

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0–1.9.0 | 2026-06-07–10 | V1 基线 → Baggage + token breakdown |
| 2.0.0 | 2026-06-14 | QueryLoop/Orchestration span 族、PEV 退役、全文档 DSAFT 对齐 |
| **3.0.0** | **2026-06-19** | **v2.1 Terminal**：Canonical S21–S24+S0 主表；D7 Turn 主路径；query.loop 全部下沉 RETIRED；诊断工具链 A/F/T 注册；legacy_harness DEPRECATED；阅读优先级标注 |
