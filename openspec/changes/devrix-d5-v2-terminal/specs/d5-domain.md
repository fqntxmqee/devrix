# D5 Observability Domain

**Domain ID:** D5
**Slug:** `observability`
**Type:** Common Domain（公共域 / 裁判域）
**Status:** Active — Canonical S21–S24 + S0（v2.1 terminal 设计目标）
**Version:** 1.0.0
**Last Updated:** 2026-06-19
**Depends On:** Shared `config` / `types`；无业务域硬依赖
**Depended By:** D1, D2, D3, D4, D6, D7（全员 Bridge + Registry）
**Cross-Domain SoT:** `d5-boundary.md` · `../architecture/cross-domain-boundaries.md` §D5

> S3 设计稿 · Change `devrix-d5-v2-terminal`（DM-20260619-006）。S7 归档后迁入 `openspec/specs/d5-observability/d5-domain.md`。

---

## North Star

**作为全链路可观测性基础设施（裁判域），为任意域操作提供可追踪、可度量、可诊断的遥测数据，并保证 Jaeger / Prometheus / Health / Incident Export 可交叉验证。**

| 可验证承诺 | Canonical S | 消费者可验证 WHAT |
|-----------|-------------|-------------------|
| C1 遥测生成 | D5-S21 Instrument | 任意 operation → Span + Metric + Log + layer/component |
| C2 遥测导出 | D5-S22 Export | Span→OTLP/Console；Metric→Prometheus |
| C3 诊断辅助 | D5-S23 Diagnose | Coverage 对账、Incident、Doctor、Tracker、FaultInject(test) |
| C4 配置管理 | D5-S24 Configure | yaml 切换 exporter/采样；runtime path 计数 |
| C0 横切 Facade | D5-S0 | Init/Shutdown/Bridge/SessionGauge；观测失败不阻断业务 |

---

## Out of Scope

| 能力 | 归属 |
|------|------|
| Turn 主循环 / LLM 调用权 | D7 |
| Prepare / ToolRound / 沙箱 | D2 |
| IM ingress / 展示 | D1 |
| LLM 路由 / 熔断 | D3 |
| Agent 生命周期 | D4 |
| 结论质量 / Guard 决策 | D6 |
| Task/Plan 写模型 | D7 workmodel |

---

## DSAFT 资产

### Canonical 价值流 — S21–S24 + S0

| S ID | Scenario | 博弈角色 | Status |
|------|----------|----------|--------|
| D5-S21 | Instrument | Signal Producer | ACTIVE |
| D5-S22 | Export | Signal Shipper | ACTIVE |
| D5-S23 | Diagnose | Auditor + Evidence Clerk | ACTIVE |
| D5-S24 | Configure | Rule Setter | ACTIVE |
| D5-S0 | Facade | Integration Shell | ACTIVE |

### 登记规模（终态目标 — v2.1 后复核）

| 层 | 数量 | SoT 文件 |
|----|------|----------|
| S | 5（S0+S21–S24） | `d5-domain.md` + `spec.md` |
| A | 30 | `a-registry.md` v4.0 |
| F | ~45 | `f-registry.md` v3.0 |
| T | 41（14 P0） | `t-registry.md` v3.2 |
| Span ops | 56 | `span-registry.md` |

### Legacy Module Index（冻结）— D5-S1–S9

| Module | Canonical |
|--------|-----------|
| S1–S3, S6 | S21 |
| S4 | S22 |
| S5, S8 | S23 |
| S7, S9 | S24 |

### 物理路径映射

| Canonical S | 物理路径 |
|-------------|----------|
| S21 | `instrument/{tracer,metrics,logger,telemetry}/` |
| S22 | `export/` |
| S23 | `diagnose/{coverage,incident,doctor,tracker,faultinject}/` + `health.go` |
| S24 | `configure/{settings,runtime}/` + `config.go` |
| S0 | `observability.go`, `bridge.go` |

---

## 规格文档索引

| 文档 | 用途 |
|------|------|
| `d5-domain.md`（本文） | 领域 SoT |
| `spec.md` v3.0 | Gherkin 验收 |
| `observability-guide.md` | Span↔T、Trace、Runbook |
| `terminal-state-guide.md` | 终态叠合 |
| `d5-boundary.md` | 跨域契约 |
| `gaming-analysis.md` | 博弈论推导（change 根目录） |
| `d5-requirements-clarifications.md` | Review 澄清 + Grill 问题 |
| `design.md` | 六段式 + Decision |
| `a-registry` / `f-registry` / `t-registry` | A/F/T |
| `span-registry.md` / `coverage.md` | Ops + 染色手册 |
| `layer-delta.md` | §v2.1-Terminal |
| `dsaft-architecture.md` | 五层计数 Stub |
| `../architecture/code-layout.md` §4.6 | scenario-slug |

---

## 跨域契约（摘要）

见 `d5-boundary.md`。要点：D7 创建 Turn span；D5 提供 Op 常量；D2 Tracker 只读；RecordHit 独立于采样。

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-19 | S3 设计稿 v2.1 terminal |
