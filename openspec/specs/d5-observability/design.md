# D5 Observability Layer 详细设计

**文档类型:** 详细架构设计（遵循 `docs/methodology/detail-design-framework.md`）
**Domain:** D5 Observability
**DSAFT Type:** 公共域
**Version:** 3.0.0
**Status:** Active (v2.1 Terminal)
**Last Updated:** 2026-06-19
**架构入口:** `openspec/specs/d5-observability/spec.md`
**Operation 常量:** `internal/layers/observability/instrument/telemetry/names.go`
**Registry SoT:** `internal/layers/observability/diagnose/coverage/registry.go`

> **v2.1 Terminal：** S21–S24+S0 号段冻结、物理路径 canonical 化、bridge 删除策略、诊断工具链 A/F/T 注册、D7 Turn 主路径。`query.loop.*` 已退役（DM-20260618-010）。

---

## 文档索引

| 文档 | 用途 | 阅读优先级 |
|------|------|-----------|
| `d5-domain.md` | 领域 SoT（Tl;DR + North Star + 博弈论玩家表） | **MUST** |
| `spec.md` v3.0 | DSAFT 规范 SoT（Scenarios、Requirements） | **MUST** |
| `d5-boundary.md` | 跨域契约（D5→D6 证据移交等） | **MUST** |
| 本文档 | 六段式可读架构设计（评审 / onboarding） | SHOULD |
| `observability-guide.md` | Span↔T + Trace 树 + P0 Runbook | SHOULD |
| `layer-delta.md` | 层能力 Delta（Gherkin Scenario） | REFERENCE |
| `coverage.md` | 代码染色操作手册 | REFERENCE |
| `a-registry.md` v4.0 / `f-registry.md` v3.0 / `t-registry.md` v3.2 | A/F/T 注册表 | REFERENCE |
| `span-registry.md` | Span 注册表（56 ops，全局 Trace Tree） | REFERENCE |

---

## ① 架构目标

### 业务目标

| 痛点 | 目标能力 | 可观测结果 |
|------|----------|------------|
| 多域 Agent 请求链路不可追踪 | W3C Trace Context + Jaeger Operation 树 | 单次会话完整 span 树 |
| 问题定位不知发生在哪一层 | `devrix.layer` / `devrix.component` 属性 | Jaeger 按层/组件过滤 |
| 埋点遗漏无法发现 | Operation Registry + Runtime Hit 对账 | Health `coverage.zero_hit_count` |
| LLM 调试缺乏上下文 | LLM JSONL + trace 关联 + incident export | `devrix debug export` bundle |
| 新旧执行路径混用难量化 | Runtime path metric | `runtime_path_resolved_total` |
| 环境问题难以快速定位 | Doctor 环境检查（7 项） | `/doctor` CLI + Health endpoint |
| 代码变更缺乏可观测性 | Tracker 代码变更追踪 + linter 集成 | diff report + lint issues |

### 技术目标（量化）

| 指标 | 目标 | 测量方式 |
|------|------|----------|
| Span 创建开销 | P99 < 50µs（无 exporter IO） | `bench_test.go` |
| Coverage RecordHit | 原子计数，无锁竞争 panic | `coverage_test.go` 100 goroutine |
| Shutdown flush | 100% pending span 导出 | `tracer_test.go` |
| Gauge 数值正确性 | 100% 精确读写 | `gauge_test.go` |
| Histogram 桶累积 | 与 Prometheus golden 一致 | `histogram_test.go` |
| Registry 对账 | names.go ≡ registry.go（56 ops） | `registry_test.go` |

### 约束条件

| 类型 | 约束 | 设计响应 |
|------|------|----------|
| 架构 | 可观测故障不阻断业务 | `NewNoOp()` + nil Bridge 守卫 |
| 基数 | 禁止 session_id 等高基数 label | Metrics `blocklist` |
| 兼容 | 未知 operation 仍创建 span | WARN + 向后兼容 |
| 退役 | PEV + QueryLoop span 族不再创建 | Registry 已移除 `context.pev.*`；`query.loop.*` 标 RETIRED |
| 终态 | S21–S24+S0 号段 + 物理路径冻结 | Terminal 冻结声明（`d5-domain.md`） |

---

## ② 架构原则

### 设计原则

1. **OTel Native** — SpanContext、Propagator、GenAI 语义属性对齐 OpenTelemetry
2. **Zero-Config Default** — `DefaultConfig()` 开箱 OTLP + Prometheus
3. **Graceful Degradation** — observability 模块故障 → `degraded`，不 panic
4. **Cardinality Safety** — Label allowlist/blocklist 防指标爆炸
5. **Coverage ≠ Sampling** — Hit 计数独立于 trace 采样决策

### 命名规范

| 场景 | 格式 | 示例 |
|------|------|------|
| Span / Jaeger Operation | `{layer}.{module}.{action}` | `orchestration.turn.run` |
| Prometheus metric | `devrix_{instrument}` | `devrix_tool_latency` |
| Layer attribute | `devrix.layer` | `orchestration` |
| Component attribute | `devrix.component` | `orchestrator` |

### 代码风格

- 各域通过 `Bridge` 注入，禁止直接 `new Tracer`
- Span 属性使用 `telemetry.SpanAttrs()` 统一注入 layer/component
- 错误日志经 `StructuredLogger`，error 类型自动附加 stack

---

## ③ 业务流程

### 主路径：D7 Turn Trace Tree（v2.1 Terminal）

```
gateway.message.receive                          [SERVER]
└── orchestration.turn.run                       [INTERNAL]
    └── orchestration.turn.iteration             [per turn]
        ├── orchestration.llm.invoke             [CLIENT]
        │   └── llm.stream                       [CLIENT]
        │       ├── llm.provider.route
        │       ├── llm.circuit_breaker
        │       ├── llm.retry
        │       └── llm.adapter.stream           [CLIENT]
        └── context.process                      [INTERNAL, caller=d7]
            ├── context.snapshot.load
            ├── context.compression.run          [if entry compress]
            └── tool.execute.single              [if tool_calls]
                └── tool.execute.permission      [if CRITICAL]
```

### 条件路径：Legacy Harness — **DEPRECATED (v2.1)**

当 `query_loop.enabled=false` 且 `harness.enabled=true`：

```
context.process
├── context.harness.bootstrap.run
│   └── context.harness.bootstrap.stage  (prefetch|guards|setup|deferred_init|tool_pool)
├── context.harness.preflight
├── context.harness.tool_pool
├── context.harness.route
└── context.system_prompt.build
```

> **v2.1 Terminal:** `legacy_harness` metric help text 标 DEPRECATED。退役路线：v2.1 DEPRECATED → v2.3 自爆机制。

### 跨域 Agent / Orchestration

```
agent.run → agent.tool.call → agent.fork|join|terminate
orchestration.wave.schedule → orchestration.wave.task.execute
orchestration.flow.event.publish
```

### 诊断工具链

| 工具 | 场景 | 触发方式 |
|------|------|----------|
| Coverage | 运行时自动 RecordHit，定期持久化 | `Tracer.Start` 自动触发 |
| Doctor | 环境健康检查（7 项：LSP、config、dirs 等） | `/doctor` CLI 或 Health endpoint |
| Tracker | 代码变更追踪（500-file LRU + diff + linter） | 非阻塞 tick 或 WatchFile |
| FaultInject | 故障注入（env 解析 + Hook） | testbuild only，生产 no-op |
| DebugFilter | 按 categories 过滤 debug 级别日志 | logger handler chain |

### 异常与补偿

| 场景 | 行为 | 可观测性 |
|------|------|----------|
| Tracer shutdown 中 | 返回 no-op span | 不 panic |
| Exporter 失败 | span 本地记录，health `degraded` | WARN 日志 |
| 未知 operation | 仍创建 span + WARN | `unknown_hits` 计数 |
| Observability disabled | `NewNoOp()` | 零开销路径 |

---

## ④ 领域模型

### 核心聚合

```
Observability (Facade)
├── TracerProvider → Tracer → Span
├── MeterProvider → Meter → Counter|Histogram|Gauge
├── StructuredLogger → Handler + Sampler + Redactor + DebugFilter
├── CoverageReporter → Counter + Persistence
├── Doctor → Environment Checker (7 checks)
├── Tracker → DiffCollector + LRUDedup + LinterRunner
├── FaultInject → EnvParser + Hook (testbuild only)
└── Bridge → ToolBridge | SessionBridge
```

### Operation 注册表（56 条，按 Layer 分组）

| Layer | Component 数 | 代表 Operation |
|-------|---------------|----------------|
| `communication` | gateway(12), adapter(3) | `gateway.message.receive`, `adapter.message.receive` |
| `context` | context_engine(9), harness(6), query_loop(3 RETIRED), tool_runner(2), plan_*(7) | `context.process` |
| `llm` | llm_gateway(4), llm_adapter(1) | `llm.stream`, `llm.adapter.stream` |
| `agent` | agent_tool(6) | `agent.run`, `agent.tool.call` |
| `orchestration` | orchestrator(3) | `orchestration.turn.run` |

### 值对象

| 类型 | 字段 | 用途 |
|------|------|------|
| `SpanContext` | TraceID, SpanID, TraceFlags | W3C 传播 |
| `OperationMeta` | Name, Layer, Component, SinceVersion, Instrumented | Registry 元数据 |
| `GenAITokenBreakdown` | Input, Output, CacheRead, Reasoning | Token metrics |
| `Bundle` (incident) | schema_version, llm_rounds, trace, coverage_hits | 调试导出 |
| `HealthReport` | checks[], overall_status | Doctor 环境检查 |

---

## ⑤ S23 硬边界与 S25 触发条件

### S23 硬边界（语义/数量/依赖）

| 边界 | 规则 |
|------|------|
| 语义边界 | S23 只含"事后审计/举证"；"实时准入控制"归 D7，"即时执行决策"归 D2/D4 |
| 数量边界 | 子承诺数 ≤ 7（超过则拆 S25） |
| 依赖边界 | S23 不 import D2/D4/D7（除 contracts 接口） |

### S25 触发条件（预先承诺 — 防止策略漂移）

| 触发条件 | 含义 |
|----------|------|
| Tracker 独立产品化（被外部系统消费） | 不再是内部审计 → 新 S |
| C3e FaultInject 被要求生产可用 | 不再是 testbuild-only → 新博弈语义 |
| C3 子承诺数 > 7 | Schelling 点：超过 7 个子承诺意味着 S 层语义不再清晰 |

### S23 子承诺新增的举证责任

任何提议新增 C3f 的提案，必须证明：
1. 该能力无法归入现有 C3a–C3e
2. 该能力的消费者与现有子承诺的消费者不同
3. 新增不导致 S23 超过 7 个子承诺上限

---

## ⑥ 核心链路图

### 端到端可观测路径

```
用户消息 (D1)
  → gateway.message.receive span [trace_id 生成/继承]
  → orchestration.turn.run span (D7)
  → orchestration.llm.invoke span
  → llm.stream span (D3) [gen_ai.* attrs + devrix_gen_ai.client.token.usage]
  → context.process span (D2, caller=d7)
  → tool.execute.single span [devrix_tool_latency observe]
  → slog.InfoContext [traceId 注入]
  → LLM JSONL [trace_id 写入]
  → OTLP exporter → Jaeger
  → coverage.RecordHit [无条件，不受采样影响]
  → HealthCheck [coverage 摘要 + Doctor 环境状态]
```

### 单点风险

| 节点 | 风险 | 缓解 |
|------|------|------|
| OTLP Collector 不可用 | span 丢失 | Console/Memory fallback；health `degraded` |
| `~/.devrix/coverage/` 不可写 | 日报失败 | 进程内 Report 仍可用 |
| 高基数 label 误用 | Prometheus 爆炸 | Registry blocklist + validateLabels |

---

## ⑦ 接口 / API 设计

### 编程接口

```go
// Facade
obs, _ := observability.New(cfg)
bridge := observability.NewBridge(obs)

// Span 创建（各域标准模式）
ctx, span := bridge.Tracer().Start(ctx, telemetry.OpOrchestrationTurnRun,
    tracer.WithSpanKind(tracer.SpanKindInternal),
    tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpOrchestrationTurnRun)...),
)
defer span.End()

// GenAI token metrics
observability.RecordGenAITokenUsage(bridge.Meter(), model, usage)

// Health + Coverage
obs.HealthCheck()        // → coverage 摘要 + Doctor 状态
obs.CoverageReport(true) // → 完整 Report
```

### HTTP 端点

| 端点 | 方法 | 响应 |
|------|------|------|
| `/health` | GET | `{status, components, coverage}` |
| `/metrics` | GET | Prometheus exposition format |

### CLI

```bash
# Session incident export
devrix debug export --session sess_xxx --output /tmp/incident.json

# Doctor 环境检查
devrix doctor

# Coverage 报表
go run ./cmd/coverage --date 2026-06-14 --summary
```

### 配置 Schema（摘要）

```yaml
observability:
  enabled: true
  tracing:
    service_name: devrix
    exporter: otlp  # console | otlp | memory | null
    sampling: { type: always_on, rate: 1.0 }
  metrics:
    exporter: prometheus
    endpoint: /metrics
    labels:
      allowlist: [provider, model, tool, risk_level, status, token_type, adapter, path]
      blocklist: [session_id, user_id]
  logging:
    level: info
    format: json
    sampling: { max_entries_per_span: 100 }
  llm:
    log_content: true
    log_dir: ~/.devrix/logs/llm
  coverage:
    enabled: true
    dir: ~/.devrix/coverage
    interval: 1h
```

### Metrics 目录

| Metric | Type | Labels | 写入位置 |
|--------|------|--------|----------|
| `devrix_tool_latency` | Histogram | tool, risk_level, status | D7 Turn → D2 ToolRound |
| `devrix_compression_ratio` | Histogram | — | context compression |
| `devrix_gen_ai.client.token.usage` | Counter | token_type, model | LLM gateway |
| `devrix_active_sessions` | Gauge | adapter | SessionBridge (S0-A03) |
| `devrix_runtime_path_resolved_total` | Counter | path | configure/runtime (S24) |

---

## ⑧ Phase 总览与 Bridge 删除策略

### Phase 总览（v2.1 Terminal 6 PR）

| Phase | PR | 内容 | 风险 |
|-------|-----|------|------|
| **A** | `docs/d5-v2-terminal-spec` | 规格终态（12 docs + ≥1 代码锚点） | 低（无代码风险） |
| **B2a** | `chore/d5-bridge-import-fix` | `bridge.go` import 改 canonical（不删包） | 低 |
| **B2b** | `chore/d5-bridge-removal` | 删除 9 bridge 包（不留 shim） | 中 |
| **B1** | `chore/d5-root-file-relocate` | 根目录 git mv（可并行 B2a/B2b） | 低 |
| **B3** | `chore/d5-t-registry-correction` | t-registry 校正 + PLANNED T 闭合 | 低 |
| **C** | 验收归档 | acceptance-report + S7 归档 | — |

### Bridge 删除拆步策略（B2a → B2b）

**B2a（改 import，不删包）：**
1. `bridge.go` import 改 `instrument/tracer|metrics|logger`
2. 测试文件改 canonical import
3. `go build ./...` + `go test ./...` 通过（bridge 包仍存在）

**B2b（删包，不留 shim）：**
1. 删除 9 个 bridge 目录
2. 全仓 `grep` 旧 bridge 路径 → 0 命中（除 archive/docs）
3. layer-lint 若有 bridge 白名单 → 移除
4. CI 增加 bridge 防回归规则（grep 9 个旧路径 = 0 命中）
5. `go test ./... -race` + obs integration + layer-lint 全绿

> **不留 shim。** B2a↔B2b 之间不留 release 间隔。shim = 道德风险（B2b 被无限期推迟）。

### Phase A 代码锚点

Phase A 必须包含 ≥1 个代码变更（a-registry v4.0 Code Location 更新 或 t-registry v3.2 canonical 列校正），不可推迟到 Phase B。纯文档承诺强度为零（cheap talk — v1.0 教训）。

### Phase B 启动对账条件

Phase B 启动前必须满足：
1. `grep query.loop` 仅 RETIRED/Legacy 节（AC-A7）
2. spec 主表无 D5-S1–S9（AC-A2）
3. a-registry v4.0 Code Location 全部 canonical（AC-A6）
4. t-registry canonical_s/a 校正完成（AC-A6）

### 跨 Change 依赖

| 关系 | 域 | 说明 |
|------|-----|------|
| 序贯级联 | D6→D7→D5 | D6 先行验证"bridge 可安全删除"→ D7 扩展模式 → D5 第三个先例 |
| 平行级联 | D5↔D2 | 同期但独立，D2 v2.2 closure 同期执行 |

---

## ⑨ Bridge 防回归 CI 规则

`grep` 9 个旧 bridge 路径 = 0 命中，否则 CI 拒绝：

```bash
# CI 防回归检查（Phase B2b 合并后激活）
grep -r "internal/layers/observability/tracer/" --include="*.go" . && exit 1
grep -r "internal/layers/observability/metrics/" --include="*.go" . && exit 1
grep -r "internal/layers/observability/logger/" --include="*.go" . && exit 1
grep -r "internal/layers/observability/telemetry/" --include="*.go" . && exit 1
grep -r "internal/layers/observability/exporter/" --include="*.go" . && exit 1
grep -r "internal/layers/observability/coverage/" --include="*.go" . && exit 1
grep -r "internal/layers/observability/incident/" --include="*.go" . && exit 1
grep -r "internal/layers/observability/settings/" --include="*.go" . && exit 1
grep -r "internal/layers/observability/runtime/" --include="*.go" . && exit 1
exit 0
```

> 排除 archive/docs 目录。`.github/workflows/` 落盘（Phase B2b.4）。

---

## ⑩ legacy_harness 退役计划

`devrix_runtime_path_resolved_total{path="legacy_harness"}` 退役路线：

| 阶段 | 版本 | 行为 |
|------|------|------|
| **DEPRECATED** | v2.1 | metric help text 标 DEPRECATED；`PathResolver` 仍可计数 |
| **WARN** | v2.2 | legacy_harness 被计数时 emit WARN 日志（stdout 可见） |
| **自爆** | v2.3 | legacy_harness metric 注册时 panic（`init()` 阶段 fail-fast） |

> **设计原则：** 渐进式退役（DEPRECATED → WARN → 自爆），给 Harness 剩余的消费方迁移窗口。v2.1 仅标 DEPRECATED，不影响运行时行为。

---

## ⑪ 改进行动（剩余）

| 优先级 | 任务 | 状态 |
|--------|------|------|
| — | V1.0–V1.9 主链 | **DONE** |
| — | QueryLoop span 族 + Registry 对账 | **DONE** (V2.0) |
| — | D7 Turn 主路径 span 落地 | **DONE** (V2.1) |
| — | 诊断工具链（Tracker/Doctor/FaultInject/DebugFilter） | **DONE** (V2.1) |
| — | Bridge 删除（Phase B2a + B2b） | **PLANNED** (Phase B) |
| — | 根目录文件归位（Phase B1） | **PLANNED** (Phase B) |
| — | legacy_harness 退役（v2.1 DEPRECATED → v2.3 自爆） | **PLANNED**（跨 release） |
| P3 | OTLP tail-sampling | 规划（需 Collector 侧配置） |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 2.0.0 | 2026-06-14 | QueryLoop/Orchestration span 族、PEV 退役、六段式设计 |
| **3.0.0** | **2026-06-19** | **v2.1 Terminal**：D7 Turn 主路径替代 QueryLoop；§5 新增 S23 硬边界 + S25 触发条件 + 子承诺举证责任；§8 新增 Phase 总览 + Bridge 删除拆步策略（B2a+B2b 不留 shim）+ Phase A 代码锚点 + Phase B 启动对账条件 + 跨 Change 级联标注；§9 Bridge 防回归 CI 规则；§10 legacy_harness 退役计划（DEPRECATED→WARN→自爆）；S23 诊断工具链（Doctor/Tracker/FaultInject/DebugFilter）；阅读优先级标注 |
