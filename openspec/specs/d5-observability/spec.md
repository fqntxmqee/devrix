# D5 Observability Layer Specification

**Capability:** observability
**Domain:** D5
**DSAFT Type:** 公共域 (Common Domain)
**Version:** 3.1.0 (V2.1 Terminal)
**Last Updated:** 2026-06-30 (DM-20260630-008 d5-spec-lite v3.1.0 S7_Archived)
**Domain SoT:** `d5-domain.md` v3.0.0 — North Star + DSAFT 资产 + Boundary SoT
**D5 Boundary:** `d5-boundary.md` — D5↔D2/D3/D4/D7 跨域边界规范

> **精简设计契约（Lite-Mode）**：本文档只放当前符合代码的设计契约（v3.1.0）。**过程需求迭代**（如 d5-sa-refine / d5-v2-terminal 13 条 Requirements 详细 Gherkin）不进入本文件，留在 `archive/<change>/specs/` 各 change 归档目录。详细时间线见 [CHANGELOG.md](CHANGELOG.md)。

---

## Overview

D5 可观测性域提供 Tracing、Metrics、结构化 Logging、Bridge 集成、Operation 代码染色（Coverage）、诊断工具链（Tracker / Doctor / FaultInject / DebugFilter）、Session Incident Export 与运行时路径指标。对齐 OpenTelemetry 语义，通过 `{layer}.{module}.{action}` canonical Operation 名称贯穿 Jaeger/OTLP 与 Prometheus。**v2.1 Terminal**：S21–S24 + S0 号段冻结 + D7 Turn 主路径（`orchestration.turn.*`）+ `query.loop.*` span 族退役。

| 承诺 | Canonical S | ValueFlow Alias | 验证入口 |
|------|-------------|-----------------|----------|
| Span/Metric/Log 遥测生成 + 属性构建 | D5-S21 Instrument | `D5_Instrument` | `D5-S21-A04~A09-T01~T05` |
| OTLP/Prometheus/Console 导出 | D5-S22 Export | `D5_Export` | `D5-S22-A01-T01/T02` |
| Coverage + Incident + Doctor + Tracker + FaultInject | D5-S23 Diagnose | `D5_Diagnose` | `D5-S23-A01~A10-T01~T02` |
| yaml 切换 + runtime path 计数 | D5-S24 Configure | `D5_Configure` | `D5-S24-A03-T01` |
| Init/Shutdown/Bridge/SessionGauge（横切） | D5-S0 Facade | `D5_Facade` | `D5-S0-T01~T03` |

### 核心设计原则

1. **Canonical Operation Naming**：Span name = `{layer}.{module}.{action}`，常量于 `instrument/telemetry/names.go`，注册于 `diagnose/coverage/registry.go`（当前 **56** 条）
2. **Coverage 独立于采样**：`Tracer.Start` 无条件 `RecordHit`，`always_off` 采样仍计数（R1 D5-S23 P0）
3. **Bridge 零侵入集成**：各域注入 `*observability.Bridge`，`IsEnabled()` 守卫避免 nil 开销
4. **Metrics 前缀**：Meter name `devrix` → Prometheus 名 `devrix_{instrument}`（如 `devrix_tool_latency`）
5. **Log-Trace-LLM 三联**：slog `traceId`/`spanId` + LLM JSONL `trace_id`/`span_id` + span `gen_ai.*` 双写
6. **Graceful Degradation**：`NewNoOp()` / nil Bridge 时业务路径不受影响；observability 故障标记 `degraded` 不阻断 Process
7. **Layer/Component 编码 Operation 名前缀**：`D{N}_*` 前缀蕴含 layer 与 component；`devrix.layer` / `devrix.component` 不再强制 span attribute
8. **D7 Turn 主路径（v2.1 Terminal canonical）**：`query.loop.*` 族退役 → `orchestration.turn.*` 主路径 + `context.process` (D2, caller=d7)（详见 archive/2026-06-19-devrix-d5-v2-terminal/）

### S 层职责

| S ID | Scenario | 职责 | Status |
|------|----------|------|--------|
| D5-S21 | Instrument | Span 生命周期、Metrics 注册/Label 白名单、结构化日志、DebugFilter、W3C/Baggage 传播 | **REGISTRY** |
| D5-S22 | Export | Console/OTLP/Memory SpanExporter | **REGISTRY** |
| D5-S23 | Diagnose | Coverage 对账、Incident 导出、Doctor 环境检查、Tracker 代码变更追踪、FaultInject(test) | **REGISTRY** |
| D5-S24 | Configure | 配置加载/校验、Runtime path 指标 | **REGISTRY** |
| D5-S0 | Facade | Init/Shutdown/Bridge/SessionGauge；观测失败不阻断业务（横切） | **REGISTRY** |

---

## DSAFT 结构

| 层级 | ID | 名称 | 物理路径 / SoT |
|------|----|------|----------------|
| D | D5 | Observability | `internal/layers/observability/` |
| S | D5-S21 | Instrument | `instrument/{tracer,metrics,logger,telemetry}/` |
| S | D5-S22 | Export | `export/` (Console + OTLP + Memory + Null) |
| S | D5-S23 | Diagnose | `diagnose/{coverage,incident,doctor,tracker,faultinject}/` |
| S | D5-S24 | Configure | `configure/{settings,runtime}/` |
| S | D5-S0 | Facade | `observability.go` + `bridge.go` + `health.go` |
| A | A1-A30 | 30 Activities (v4.0) | `a-registry.md` |
| F | F1-F45 | 45 Function Points (v3.0) | `f-registry.md` |
| T | T1-T41 | 41 Test Points (v3.2) | `t-registry.md` |

**当前计数（v3.1.0）**：D=1, S=5 (canonical: S21-S24 + S0), A=30, F=45, T=41, Operation=56。

---

## Scenarios

| ID | Scenario | Responsibility | Status | 验证入口 |
|----|----------|----------------|--------|----------|
| D5-S21 | Instrument | Span + Metric + Log + 属性 + Baggage 传播 | **REGISTRY** | `D5-S21-A04~A09-T01~T05` |
| D5-S22 | Export | Console/OTLP/Memory SpanExporter | **REGISTRY** | `D5-S22-A01-T01/T02` |
| D5-S23 | Diagnose | Coverage 对账 + Incident + Doctor + Tracker + FaultInject | **REGISTRY** | `D5-S23-A01~A10-T01~T02` |
| D5-S24 | Configure | 配置加载/校验 + Runtime path 指标 | **REGISTRY** | `D5-S24-A03-T01` |
| D5-S0 | Facade | Init/Shutdown/Bridge/SessionGauge | **REGISTRY** | `D5-S0-T01~T03` |

---

## Architecture

```
D1 Gateway → gateway.message.receive [SERVER]
              │
D7 Orchestration → orchestration.turn.run [INTERNAL]
              ├─→ orchestration.turn.iteration
              ├─→ orchestration.llm.invoke [CLIENT]
              │   └─→ llm.stream [CLIENT] (D3)
              └─→ context.process [INTERNAL] (D2, caller=d7)
                  └─→ tool.execute.single
              │
D2 ContextEngine → context.process + context.compression.run
D3 LLMGateway → llm.stream + llm.adapter.stream + RecordGenAITokenUsage
D4 MultiAgent → agent.run + agent.tool.call + agent.fork|join
              │
              ▼
   ┌──────────────────────────────────────┐
   │  Observability Facade (D5)            │
   │  Instrument │ Export │ Diagnose       │
   │  Configure │ Bridge │ Incident        │
   └──────────────────────────────────────┘
              │
              ▼
   Console / OTLP / Prometheus / ~/.devrix/coverage/
```

### 域边界

| D5 拥有 | D5 调用（不拥有） | D5 不拥有 |
|---------|------------------|----------|
| Span/Metric/Log 生成 + 属性构建 | D1 Gateway 接收 root span | Span name 业务含义定义（各域） |
| Operation Registry + Coverage 对账 | D2 Engine 调用 Bridge tracer/meter/logger | 业务 Span 触发点（各域） |
| Bridge 零侵入集成（IsEnabled 守卫） | D3 LLMGateway RecordGenAITokenUsage | OTLP Collector 部署（运维） |
| Diagnostic 工具链（Coverage/Incident/Doctor/Tracker/FaultInject） | D4 MultiAgent Fork policy metrics sink | Prometheus scrape config |
| 诊断工具 CLI（`devrix debug export`） | D7 Orchestration orchestration.* spans | 业务告警阈值（运维） |
| Session Incident Export（schema v1） | — | Log aggregation pipeline |

---

## 关键 Scenario 范式

### 范式：D5-S23 Coverage HealthCheck 运行时命中计数（Coverage 独立于采样）

#### Scenario: Coverage 独立于采样 + Operation Registry 对账

- **GIVEN** `tracing.sampling.type = always_off` + Operation Registry 56 条全注册
- **WHEN** instrumented operation 调用 `Tracer.Start` + 100 个并发 goroutine 对同一 operation 调用 `Start`
- **THEN** 命中计数 MUST 等于 100（无丢失，无重复）
- **AND** `HealthCheck()` 暴露 `coverage.{operations_total, operations_hit, coverage_ratio, zero_hit_count}`
- **AND** 无 span 导出到 exporter（采样关闭）

---

## 关键链路口

1. **D7 Turn 主路径**：`orchestration.turn.run → iteration → llm.invoke → llm.stream` (D7→D3) + `context.process` (D2, caller=d7)
2. **GenAI Token 链**：`llm.stream` 双写 `gen_ai.*` attrs + `devrix_gen_ai.client.token.usage` Counter (input/output/cache_read/reasoning)
3. **Coverage 链**：`Tracer.Start` 无条件 `RecordHit` + `~/.devrix/coverage/` 持久化 + `devrix debug export --session <id>` 导出
4. **Diagnostic 工具链**：Coverage (事中) + Incident (事后) + Doctor (事前+事中) + Tracker (事中) + FaultInject (testbuild only)
5. **W3C Baggage 链**：Gateway 入站 `session.id` + `user.id` → CLI 子进程 `TRACEPARENT` + `BAGGAGE` 环境变量
6. **Runtime Path 链**：`devrix_runtime_path_resolved_total{path="d7_turn|legacy_harness"}` + `legacy_harness` DEPRECATED（v2.3 自爆机制）

---

## 附录：总览

- **当前活跃 Requirement 数**：5 canonical（每段 1 句 + 1 canonical Gherkin，详见 archive 详细文本）
- **历史 Requirement 详细文本（13 条）**：在 `archive/<change>/specs/` 各 change 目录（详见 CHANGELOG.md）
- **RETIRED 登记**：`query.loop.*` / `context.pev.*` / `legacy_harness`（详见 d5-domain.md §RETIRED）
- **当前 spec 版本**：v3.1.0
- **下一次架构级变更触发**：D5 域升级 v4.0+ 或 Operation Registry 跨域变化时重新审计 Boundary Debt Decisions