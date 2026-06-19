# Delta: Domain D5 (OBS)

**Change ID:** devrix-foundation → current
**Affects:** observability, tracing, metrics, logging, coverage, incident export, runtime metrics, diagnostic tools
**Version:** 4.0.0
**Status:** Active (v2.1 Terminal)
**Last Updated:** 2026-06-19

D5 可观测性域 V4：56 条 canonical Operation、**D7 Turn 主路径** span 族、W3C Baggage、GenAI token 细分 metrics、诊断工具链（Tracker / Doctor / FaultInject / DebugFilter）、Session incident export、Runtime path metric（`d7_turn`）。~~QueryLoop~~ span 族已 REMOVED（DM-20260618-010）。**v2.1 Terminal = S21–S24+S0 号段 + 物理路径 + 56 ops 长期冻结。**

---

## v2.1-Terminal（2026-06-19 — DM-20260619-006）

### Terminal 冻结声明

以下资产声明为长期冻结（v2.1 归档后至少 2 个 release cycle 不变更）：

| 冻结对象 | 范围 | 变更需 |
|----------|------|--------|
| S 号段 | S21–S24 + S0 | D5 Owner + 2 个业务域 Owner NACK 权 |
| 物理路径 | `instrument/` / `export/` / `diagnose/` / `configure/` | D5 Owner Review |
| Operation Registry | 56 ops（不增删，除非 RETIRED 清理） | D5 Owner Review + span-registry 更新 |
| T ID | 41 T（不改号，仅 canonical_s/a 列校正） | 跨域 Review（T 层宪法） |

### S23 诊断子承诺（C3a–C3e）

| 子承诺 | 能力 | A ID | 时间属性 |
|--------|------|------|---------|
| C3c Doctor + Health | Environment Auditor | D5-S23-A10, A06 | 事前+事中 |
| C3a Coverage | Compliance Auditor | D5-S23-A01/A02/A03 | 事中 |
| C3d Tracker | Continuous Inspector | D5-S23-A07 | 事中 |
| C3b Incident | Evidence Archivist | D5-S23-A04/A05 | 事后（不可补救） |
| C3e FaultInject | Red Team | D5-S23-A09 | 测试（与生产隔离） |

### S23 硬边界

| 边界 | 规则 |
|------|------|
| 语义边界 | S23 只含"事后审计/举证"；"实时准入控制"归 D7，"即时执行决策"归 D2/D4 |
| 数量边界 | 子承诺数 ≤ 7（超过则拆 S25） |
| 依赖边界 | S23 不 import D2/D4/D7（除 contracts 接口） |

### S25 触发条件

| 触发条件 | 含义 |
|----------|------|
| Tracker 独立产品化（被外部系统消费） | 不再是内部审计 → 新 S |
| C3e FaultInject 被要求生产可用 | 不再是 testbuild-only → 新博弈语义 |
| C3 子承诺数 > 7 | Schelling 点：超过 7 个子承诺意味着 S 层语义不再清晰 |

### Bridge 删除策略

9 bridge 包（`tracer/ metrics/ logger/ telemetry/ exporter/ coverage/ incident/ settings/ runtime/`）分两步删除：

- **Phase B2a:** `bridge.go` import 改 `instrument/*` 直连（不改包结构）
- **Phase B2b:** 删除 9 bridge 目录（不留 shim）

B2a↔B2b 之间不留 release 间隔。shim = 道德风险（B2b 被无限期推迟）。

### 代码锚点

Phase A 必须包含 ≥1 个代码锚点（a-registry v4.0 Code Location 更新 + t-registry v3.2 canonical 列校正），不可推迟到 Phase B。纯文档承诺强度为零（cheap talk）。

### legacy_harness 退役计划

`legacy_harness` metric help text 标 DEPRECATED（v2.1）。退役路线：v2.1 DEPRECATED → v2.3 自爆机制（metric 注册时 panic if legacy_harness path still active）。

---

## ADDED (V2.0 — 2026-06-14)

### Requirement: QueryLoop Span Family — **REMOVED (DM-20260618-010)**

> 主路径 span 见 **D7 Turn Span Family**（`orchestration.turn.*`）。下列 op 不再创建：

| Operation | Component | Status |
|-----------|-----------|--------|
| ~~`query.loop.run`~~ | query_loop | REMOVED |
| ~~`query.loop.turn`~~ | query_loop | REMOVED |
| ~~`query.loop.llm.call`~~ | query_loop | REMOVED |

### Requirement: D7 Turn Span Family

D7 Turn 主路径 MUST 注册 `orchestration.turn.run`, `orchestration.turn.iteration`, `orchestration.llm.invoke` under D7 component.

#### Scenario: Turn span hierarchy

- GIVEN tracing enabled and D7 FastPath active
- WHEN `TurnExecutor.RunTurn` completes one iteration with LLM call
- THEN span `orchestration.turn.run` exists
- AND `orchestration.llm.invoke` parent is turn iteration
- AND `llm.stream` parent is `orchestration.llm.invoke`

---

### Requirement: Tool Execution Span Family

| Operation | Component | 触发点 |
|-----------|-----------|--------|
| `tool.execute.single` | tool_runner | D7→D2 ToolRound |
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

系统 MUST 注册 `devrix_runtime_path_resolved_total` Counter，labels `path` ∈ {`d7_turn`, `legacy_harness`}，与 in-process `PathResolver` 同步。

> **v2.1 Terminal:** `legacy_harness` metric help text 标 DEPRECATED。

#### Scenario: Path counter increments on Process

- GIVEN observability metrics enabled and RegisterRuntimeMetric called
- WHEN ContextEngine routes via PreparedTurnRunner (D7 Turn path)
- THEN `devrix_runtime_path_resolved_total{path="d7_turn"}` increments by 1

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
| Canonical Trace Tree | PEV 层级 → QueryLoop 层级 → **D7 Turn 层级** | D2-S1 PEV 退役 → D7 Turn 主路径 |
| Operation 总数 | 44+ → 56 | 新增 query/tool/task/orchestration 族 |
| `context.plan.generate` | 从 PEV plan 迁移到 `task.plan.generate` | Plan 能力重构到 task 子系统 |
| Coverage 文档 | 移除 pev_engine + query_loop 组件 | 对齐现行 Registry |
| A/F/T 注册表 | 骨架 → 完整代码映射 → **v2.1 代码锚点** | DSAFT 文档规范对齐 + cheap talk 防御 |
| Bridge 路径 | bridge 9 包 → canonical instrument/export/diagnose/configure | v2.1 Terminal 物理路径归位 |
| S 层 | 9 技术模块 → 4+1 价值流 S21–S24+S0 | SA Refine v1.0 → v2.1 冻结 |

---

## REMOVED / RETIRED

| Item | 退役日期 | 原因 |
|------|----------|------|
| `context.pev.*` (7 ops) | 2026-06-13 | PEV 引擎下线 |
| `context.milestone.run` | 2026-06-13 | Milestone 重构到 task 子系统 |
| `pev_engine` component | 2026-06-13 | Registry/文档移除 |
| PEV Span Hierarchy requirements | 2026-06-13 | spec.md V2.0 标记 RETIRED |
| Custom traceId `{adapter}:{session}:{msg}` | V1.2 | W3C 32-char hex 替代 |
| `communication/metrics/collector.go` Session gauge | V1.3 | `SessionBridge.ActiveSessions` 替代 |
| `query.loop.run` / `query.loop.turn` / `query.loop.llm.call` | 2026-06-18 | D7 Turn 主路径替代（DM-20260618-010） |
| QueryLoop Span Hierarchy requirements | 2026-06-18 | D7 Turn Span Hierarchy |
| `legacy_harness` runtime path | 2026-06-19 | `d7_turn`（DEPRECATED，v2.3 移除） |

---

## Technical Notes

### File Structure (v2.1 Terminal canonical)

```
internal/layers/observability/
├── observability.go          # Facade: New, Shutdown, HealthCheck (S0)
├── bridge.go                 # Bridge, ToolBridge, SessionBridge (S0)
├── config.go, load.go        # 配置加载 (S24)
├── health.go                 # Health endpoint (S23)
├── instrument/
│   ├── tracer/               # D5-S21: TracerProvider, Span, W3C, Baggage
│   ├── metrics/              # D5-S21: Meter, Counter/Histogram/Gauge, genai_tokens
│   ├── logger/               # D5-S21: StructuredLogger, slog, DebugFilter
│   └── telemetry/            # D5-S21: Op*, SpanAttrs, LayerAndComponent
├── export/                   # D5-S22: Console, OTLP, Memory, Null exporter
├── diagnose/                 # D5-S23: Coverage, Incident, Doctor, Tracker, FaultInject
│   ├── coverage/
│   ├── incident/
│   ├── doctor/
│   ├── tracker/
│   └── faultinject/
└── configure/                # D5-S24: Settings, Runtime path resolver
    ├── settings/
    └── runtime/
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
| **4.0.0** | **2026-06-19** | **v2.1 Terminal**：Terminal 冻结声明、S23 诊断子承诺 C3a–C3e + 硬边界 + S25 触发条件、Bridge 删除策略（B2a+B2b 不留 shim）、代码锚点要求、legacy_harness DEPRECATED + 退役路线；物理路径 canonical 化；query.loop.* → RETIRED；版本 3.0→4.0 跳号（对齐 v2.1 终态语义） |
