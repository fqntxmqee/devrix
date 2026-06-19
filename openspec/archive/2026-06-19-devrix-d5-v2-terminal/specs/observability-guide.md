# D5 Observability — 可观测性与验收指南

**Capability:** observability
**Status:** S3 Design Draft
**Version:** 1.0.0
**Last Updated:** 2026-06-19
**Parent:** `d5-domain.md` · `span-registry.md` · `t-registry.md`
**Complements:** `../d7-orchestration/observability-guide.md` · `terminal-state-guide.md`

> S7 归档后迁入 `openspec/specs/d5-observability/observability-guide.md`。

---

## 0. 文档定位

| 本文档提供 | 权威 SoT 在其他文件 |
|-----------|-------------------|
| D5 Span↔T P0 绑定（按 S21–S24） | 全量 T → `t-registry.md` |
| Canonical Trace 树（D7 Turn） | 56 ops → `span-registry.md` |
| P0 Runbook | Gherkin → `spec.md` |
| 跨域 Trace 分工 | `d5-boundary.md` |

---

## 1. Canonical Trace 树（生产主路径）

```text
gateway.message.receive                    [SERVER, D1]
└── orchestration.turn.run               [INTERNAL, D7]
    └── orchestration.turn.iteration     [per iteration]
        ├── orchestration.llm.invoke     [CLIENT, D7]
        │   └── llm.stream               [CLIENT, D3]
        │       └── llm.adapter.stream   [CLIENT, D3]
        └── tool.execute.single          [INTERNAL, D7→D2]
            └── context.process          [INTERNAL, D2, caller=d7]
```

**条件路径：** `context.compression.*` · `orchestration.wave.*` · `agent.*` · `context.harness.*`（Legacy）

**REMOVED:** `query.loop.*` · `context.pev.*`

---

## 2. Span ↔ T 绑定（P0 摘要）

代码常量 SoT：`internal/layers/observability/instrument/telemetry/names.go`。

### D5-S21 Instrument

| 能力 | 绑定 T（P0 加粗） |
|------|------------------|
| Tracer Shutdown flush | **D5-S21-A01-T01** |
| W3C Propagation | **D5-S22-A01-T03**（跨域） |
| Gauge/Histogram 正确性 | **D5-S21-A05-T03** … **T05** |
| slog trace 注入 | **D5-S21-A09-T01** |
| SpanAttrs layer/component | **D5-S21-A11-T01**, **A12-T01** |
| DebugFilter | **D5-S23-A08-T01/T02**（canonical_s=S21, A14） |

### D5-S22 Export

| 能力 | 绑定 T |
|------|--------|
| D7 Turn span 存在 | **D5-S22-A01-T02** |
| Adapter→Gateway trace_id | **D5-S22-A01-T03** |

### D5-S23 Diagnose

| 能力 | 绑定 T |
|------|--------|
| Registry 56 对账 | **D5-S23-A01-T01** |
| zero_hit 报告 | **D5-S23-A01-T02** |
| 100 并发 RecordHit | **D5-S23-A01-T03** |
| always_off 仍 Hit | **D5-S23-A01-T04** |
| Doctor healthy / fail | **D5-S23-A03-T01/T02**（canonical_a=A10） |
| Incident export | **D5-S23-A04-T01** |
| Tracker LRU/Diff/Linter | **D5-S23-A07-T01/T02**, W6/W7 cross-ref |
| FaultInject | **D5-S23-A09-T01/T02** |

### D5-S24 Configure

| 能力 | 绑定 T |
|------|--------|
| Runtime path counter | **D5-S24-A03-T01** … |

### D5-S0 Facade

| 能力 | 绑定 T |
|------|--------|
| SessionBridge gauge | **D5-S23-A06-T01**（canonical_s=S0, A03） |

---

## 3. 按 S 验收摘要

| S | P0 验收一句话 | 关键测试 |
|---|--------------|----------|
| S21 | Span/Metric/Log 正确 + 属性齐全 | `instrument/**_test.go` |
| S22 | Turn trace 链完整 + trace 传播 | `orchestrator_test`, `obs_trace_propagation_test` |
| S23 | Coverage 对账 + Doctor + Tracker 非阻塞 | `coverage_test`, `doctor_test`, `tracker_test` |
| S24 | path metric 与 PathResolver 同步 | `runtime_metric_test` |
| S0 | Init/Bridge/NoOp 降级 | `bootstrap/observability_test` |

---

## 4. P0 Runbook

### 4.1 Health 检查

```bash
curl -s http://localhost:8080/health | jq '.coverage'
```

期望字段：`operations_total`, `operations_hit`, `coverage_ratio`, `zero_hit_count`。

**zero_hit_count > 0：** 对照 `coverage/registry.go` 与 `names.go`；确认对应 path 是否 `Tracer.Start` 被调用。

### 4.2 Jaeger 主路径抽查

1. 打开 Jaeger UI（默认 `http://localhost:16686`）
2. Service `devrix`，找最近 `gateway.message.receive`
3. 展开至 `orchestration.turn.run` → `orchestration.llm.invoke` → `llm.stream`
4. 若见 `query.loop.*` → **回归**：QueryLoop span 未清干净

### 4.3 Incident Export

```bash
devrix debug export --session <session_id>
```

验证 schema v1：`llm_rounds`、可选 `trace`、`coverage_hits`。

### 4.4 Doctor CLI

```bash
devrix doctor
```

7 项全 pass → healthy；missing LSP → StatusFail（见 `doctor_test.go`）。

### 4.5 Observability 降级

`observability.enabled=false` 启动后：业务可用；Health 显示 tracer/metrics/logging `disabled`。

---

## 5. 与 D7 observability-guide 关系

| 主题 | 读哪份 |
|------|--------|
| D7 Turn span 属性、`orchestration.route` | D7 `observability-guide.md` |
| D5 Registry、Coverage、Export、Bridge | 本文 + `coverage.md` |
| 跨域 Trace 责任矩阵 | `d5-boundary.md` §4 |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-19 | S3 设计稿 |
