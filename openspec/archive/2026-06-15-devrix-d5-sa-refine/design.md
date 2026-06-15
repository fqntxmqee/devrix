# D5 Observability — S 层重构 Design

**Change ID:** devrix-d5-sa-refine  
**Demand ID:** DM-20260615-001  
**阶段:** S3 Design  
**版本:** v1.0  
**状态:** Draft  
**关联:** `proposal.md`

---

## 1. 概述

### 1.1 设计目标

| 目标 | 描述 |
|------|------|
| S 切法 | 按可观测价值流（Instrument→Export→Diagnose→Configure），非按 Go 子包 |
| Legacy 双轨 | S1–S9 冻结为 Legacy；S21–S24 Canonical |
| 零代码变更 | v1.0 仅重排注册表 + 增 canonical_s 列，不改任何 Go 文件 |
| 跨域 | `code-layout.md §4` 补 D5 scenario-slug 注册表 |

### 1.2 版本范围

| 版本 | 范围 |
|------|------|
| v1.0 | Registry 重排 + Legacy 双轨 + canonical_s 列 |
| v2.0 | 物理路径 scenario-slug 迁移（后续 change） |

---

## 2. Decision 记录

### Decision 1: S 切法

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 4+1 价值流 S21–S24 | 与 D3/D4/D7 同型；消费者可验证 | 需要 Legacy 双轨表 |
| B: 保留 9 技术 S 微调 | 改动小 | 不解决包名=S 名问题 |

**选择:** A  
**理由:** Playbook 原则 1（S 回答可验证承诺）；D5 是最后一个未价值流化的域

### Decision 2: S 编号

| 方案 | 选择 | 理由 |
|------|------|------|
| 重编 S1–S9 | 拒绝 | BREAKING T ID |
| 新号段 S21–S24 | **采用** | 避开 D3 S1–S6、D4 S11–S16、D2 S15–S20；D5 独占 20 号段 |

### Decision 3: S6 Telemetry 归属

| 方案 | 选择 | 理由 |
|------|------|------|
| 保留独立 S | 拒绝 | telemetry/names.go 是 Span/Metric 属性构建辅助，不构成独立价值流 |
| 并入 S21 Instrument | **采用** | 属性构建是遥测生成的一部分 |

### Decision 4: S9 Runtime 归属

| 方案 | 选择 | 理由 |
|------|------|------|
| 保留独立 S | 拒绝 | Runtime 只有 2 个 A（路径计数 + 注册），不够独立价值流 |
| 并入 S24 Configure | **采用** | 运行时路径指标是配置驱动的管理能力 |

### Decision 5: 性能 T 归属

| 方案 | 选择 | 理由 |
|------|------|------|
| 保留 D5-S2 | 拒绝 | T06/T07 是跨域性能测试，不属 D5 Metrics |
| 迁至 CROSS 段 | **采用** | t-registry 增 CROSS 段 |

---

## 3. S 层定义（Canonical）

### D5-S21: Instrument

| 属性 | 值 |
|------|---|
| 承诺 | C1：为任意操作生成遥测数据（Span + Metric + Log + 属性） |
| 消费者 | D2/D4/D7（所有域） |
| 涉及 Legacy | S1 Tracer + S2 Metrics + S3 Logger + S6 Telemetry |

**包含 A：**

| A ID | Name | Legacy |
|------|------|--------|
| D5-S21-A01 | CreateSpan | S1-A01 |
| D5-S21-A02 | EndSpan | S1-A02 |
| D5-S21-A03 | PropagateContext | S1-A03 |
| D5-S21-A04 | ShutdownTracer | S1-A04 |
| D5-S21-A05 | RecordMetric | S2-A01 |
| D5-S21-A06 | ExportPrometheus | S2-A02 |
| D5-S21-A07 | RecordGenAITokens | S2-A03 |
| D5-S21-A08 | LogRecord | S3-A01 |
| D5-S21-A09 | InstallSlogBridge | S3-A02 |
| D5-S21-A10 | ShutdownLogger | S3-A03 |
| D5-S21-A11 | ResolveLayerComponent | S6-A01 |
| D5-S21-A12 | BuildSpanAttrs | S6-A02 |
| D5-S21-A13 | BuildGenAIAttrs | S6-A03 |

### D5-S22: Export

| 属性 | 值 |
|------|---|
| 承诺 | C2：将遥测数据导出到外部系统（OTLP/Prometheus/Console） |
| 消费者 | 运维/SRE |
| 涉及 Legacy | S4 Exporter |

**包含 A：**

| A ID | Name | Legacy |
|------|------|--------|
| D5-S22-A01 | ExportSpans | S4-A01 |
| D5-S22-A02 | CreateExporter | S4-A02 |

### D5-S23: Diagnose

| 属性 | 值 |
|------|---|
| 承诺 | C3：提供诊断辅助（Coverage 报告 + Incident 导出 + Health） |
| 消费者 | 运维/调试 |
| 涉及 Legacy | S5 Coverage + S8 Incident + S0-A02 HealthCheck |

**包含 A：**

| A ID | Name | Legacy |
|------|------|--------|
| D5-S23-A01 | RecordOperationHit | S5-A01 |
| D5-S23-A02 | AssessCoverage | S5-A02 |
| D5-S23-A03 | GenerateDailyReport | S5-A03 |
| D5-S23-A04 | ExportSessionBundle | S8-A01 |
| D5-S23-A05 | RecordLLMPayload | S8-A02 |
| D5-S23-A06 | HealthCheck | S0-A02 |

### D5-S24: Configure

| 属性 | 值 |
|------|---|
| 承诺 | C4：加载/校验配置 + 运行时路径计数 |
| 消费者 | Bootstrap / 启动流程 |
| 涉及 Legacy | S7 Settings + S9 Runtime |

**包含 A：**

| A ID | Name | Legacy |
|------|------|--------|
| D5-S24-A01 | LoadObsConfig | S7-A01 |
| D5-S24-A02 | ValidateObsConfig | S7-A02 |
| D5-S24-A03 | RecordRuntimePath | S9-A01 |
| D5-S24-A04 | RegisterRuntimeMetric | S9-A02 |

### D5-S0: Facade（横切）

| A ID | Name | Legacy |
|------|------|--------|
| D5-S0-A01 | InitObservability | S0-A01 |
| D5-S0-A02 | CreateBridge | S0-A03 |

> 注：原 S0-A02 HealthCheck 上移至 S23 Diagnose。

---

## 4. T 层 Legacy → Canonical 映射

| Legacy T ID | Canonical T ID | Canonical S | 备注 |
|-------------|----------------|-------------|------|
| D5-S1-A01-T01 | D5-S21-A01-T01 | S21 | Shutdown 刷写 |
| D5-S1-A01-T02 | D5-S21-A01-T02 | S21 | ConsoleExporter |
| D5-S1-A03-T01 | D5-S21-A03-T01 | S21 | Baggage 往返 |
| D5-S1-A03-T02 | D5-S21-A03-T02 | S21 | Propagator inject/extract |
| D5-S1-A03-T03 | D5-S21-A03-T03 | S21 | CLI TRACEPARENT |
| D5-S1-A01-T04 | D5-S21-A01-T04 | S21 | W3C TraceID/SpanID |
| D5-S2-A01-T01 | D5-S21-A05-T01 | S21 | Tracing Span（PLANNED） |
| D5-S2-A01-T02 | D5-S21-A05-T02 | S21 | Metrics Counter（PLANNED） |
| D5-S2-A01-T03 | D5-S21-A05-T03 | S21 | Gauge 数值 |
| D5-S2-A01-T04 | D5-S21-A05-T04 | S21 | Histogram golden |
| D5-S2-A01-T05 | D5-S21-A05-T05 | S21 | Int64UpDownCounter |
| D5-S2-A01-T06 | **CROSS-T01** | CROSS | Compression P99 latency（迁出） |
| D5-S2-A01-T07 | **CROSS-T02** | CROSS | Concurrent session memory（迁出） |
| D5-S2-A01-T08 | D5-S21-A07-T01 | S21 | gen_ai token usage |
| D5-S2-A01-T09 | D5-S21-A05-T06 | S21 | tool_latency histogram |
| D5-S3-A01-T01 | D5-S21-A08-T01 | S21 | 日志级别过滤 |
| D5-S3-A01-T02 | D5-S21-A08-T02 | S21 | Shutdown 覆盖 |
| D5-S3-A01-T03 | D5-S21-A08-T03 | S21 | Error stacktrace |
| D5-S3-A01-T04 | D5-S21-A08-T04 | S21 | 日志采样 |
| D5-S3-A02-T01 | D5-S21-A09-T01 | S21 | slog bridge traceId |
| D5-S3-A01-T05 | D5-S21-A08-T05 | S21 | 敏感字段脱敏 |
| D5-S4-A01-T01 | D5-S22-A01-T01 | S22 | OTLP 序列化 |
| D5-S4-A01-T02 | D5-S22-A01-T02 | S22 | QueryLoop canonical span |
| D5-S4-A01-T03 | D5-S22-A01-T03 | S22 | Adapter→Gateway trace_id |
| D5-S5-A01-T01 | D5-S23-A01-T01 | S23 | Operation Registry 56 条 |
| D5-S5-A01-T02 | D5-S23-A01-T02 | S23 | zero_hit 报告 |
| D5-S5-A01-T03 | D5-S23-A01-T03 | S23 | 并发 RecordHit |
| D5-S5-A01-T04 | D5-S23-A01-T04 | S23 | 采样关闭仍 RecordHit |
| D5-S5-A01-T05 | D5-S23-A01-T05 | S23 | Harness span 树 |
| D5-S5-A02-T01 | D5-S23-A02-T01 | S23 | 端到端染色集成 |
| D5-S6-A01-T01 | D5-S21-A11-T01 | S21 | LayerAndComponent |
| D5-S6-A01-T02 | D5-S21-A12-T01 | S21 | SpanAttrs |
| D5-S6-A03-T01 | D5-S21-A13-T01 | S21 | GenAIUsageAttrs OTel |
| D5-S6-A03-T02 | D5-S21-A13-T02 | S21 | cache_read/reasoning |
| D5-S8-A01-T01 | D5-S23-A04-T01 | S23 | Export bundle schema |
| D5-S8-A01-T02 | D5-S23-A04-T02 | S23 | CLI debug export |
| D5-S9-A01-T01 | D5-S24-A03-T01 | S24 | 幂等注册 path counter |
| D5-S9-A01-T02 | D5-S24-A03-T02 | S24 | IncRuntimeMetric 桥接 |
| D5-S9-A01-T03 | D5-S24-A03-T03 | S24 | PathResolver 并发 |
| D5-S0-A02-T01 | D5-S23-A06-T01 | S23 | SessionBridge gauge |
| D5-S0-A02-T02 | D5-S23-A06-T02 | S23 | HealthCheck coverage（PLANNED） |

---

## 5. Legacy Module Index（D5-S1–S9）

| S ID | Module | Status | Canonical |
|------|--------|--------|-----------|
| D5-S1 | Tracer | Legacy | → S21 |
| D5-S2 | Metrics | Legacy | → S21（7 T）+ CROSS（2 T） |
| D5-S3 | Logger | Legacy | → S21 |
| D5-S4 | Exporter | Legacy | → S22 |
| D5-S5 | Coverage | Legacy | → S23 |
| D5-S6 | Telemetry | Legacy | → S21 |
| D5-S7 | Settings | Legacy | → S24 |
| D5-S8 | Incident | Legacy | → S23 |
| D5-S9 | Runtime | Legacy | → S24 |

---

## 6. 物理路径

| Canonical S | scenario-slug | v1.0 当前 | v2.0 目标 |
|-------------|---------------|----------|-----------|
| S21 | `instrument` | `tracer/` + `metrics/` + `logger/` + `telemetry/` | `observability/instrument/` |
| S22 | `export` | `exporter/` | `observability/export/` |
| S23 | `diagnose` | `coverage/` + `incident/` | `observability/diagnose/` |
| S24 | `configure` | `settings/` + `runtime/` | `observability/configure/` |
| S0 | facade | `observability/` 根 | 保持 |

---

## 7. 统计

| 指标 | 旧 | 新 |
|------|-----|-----|
| S 数 | 9+1 | 4+1 |
| A 数 | 27 | 27（0 变更） |
| T 数 | 41 | 39 + 2 CROSS |
| P0 | 14 | 14（保持） |
| IMPLEMENTED | 38 | 38（保持） |
| PLANNED | 3 | 3（保持） |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-15 | 初稿：S21–S24 + Legacy 双轨 + T 映射 |
