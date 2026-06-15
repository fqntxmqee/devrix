# Proposal: D5 Observability S/A 重切 — 技术模块→价值流化

**Change ID:** devrix-d5-sa-refine  
**Demand ID:** DM-20260615-001  
**Status:** S2_Proposal  
**Phase Scope:** D + S（A/F 编排在 design.md；本文件含 S 层切法论证）

---

## 1. Background

D5 Observability 自 V1（2026-06-07）至 P3（2026-06-10）共迭代 7 个 change、38 条 T（35 IMPLEMENTED + 3 PLANNED，P0 13 条），功能完整。但 **D5 是 D1–D7 中最后一个未做价值流切法的公共域**：

| 域 | 价值流 S 数 | 价值流化状态 |
|----|------------|------------|
| D1 Communication | 6 (S13–S18) | ✅ v2.0 |
| D2 Context Engine | 6 (S15–S20) | ✅ v2.0 |
| D3 LLM Gateway | 6 (S1–S6) | ✅ v1.0 Registry |
| D4 Multi-Agent | 6 (S11–S16) | ✅ v1.0 Registry |
| **D5 Observability** | **0 / 9** | ❌ **本 change** |
| D6 Evolution | 0 / 4 | ❌ 并行 change |
| D7 Orchestration | 5 (S1–S5) | ✅ v1.0 |

D5 当前 9 个 S（Tracer / Metrics / Logger / Exporter / Coverage / Telemetry / Settings / Incident / Runtime）**全部为 Go 包名**，是 `code-layout.md §2` 禁止名单的典型案例。本 change 延续 D3/D4/D7 SA Refine 模式，将 D5 从技术模块切法转为价值流切法。

---

## 2. Problem Statement

### 2.1 S 层 = 包名，无价值语义

D5 的 9 个 S 直接对应 `internal/layers/observability/` 下的 9 个子目录。消费者（D2/D4/D7）调用 D5 时的思维模型是「我要创建一个 span」「我要记录一个 metric」「我要打一行日志」——三个独立动作，但价值上它们是同一件事：**为我的操作生成遥测数据**。

当前切法导致：
- **Instrument 承诺被拆碎**：Tracer(S1) + Metrics(S2) + Logger(S3) + Telemetry(S6) 实为同一价值流「生成遥测」
- **诊断能力分散**：Coverage(S5) + Incident(S8) 都是「帮我排查问题」，但被拆到两个 S
- **配置与运行时混杂**：Settings(S7) + Runtime(S9) 一个管启动配置、一个管运行时路径计数，分开无意义

### 2.2 跨域引用不精确

| # | 问题 |
|---|------|
| P1 | D5 无 `d5-boundary.md`（D2/D4/D7 已有对称文档） |
| P2 | `cross-domain-boundaries.md` D5 段仅列 Span/Metric 调用方，无契约详表 |
| P3 | `code-layout.md §4` 缺 D5 scenario-slug 注册表 |

### 2.3 T 层归属粒度不均

D5-S2（Metrics）混入性能测试 T（T06 Compression P99 latency、T07 Concurrent session memory），这些属于跨域集成测试，不应挂在 D5-S2 下。

---

## 3. Proposed Solution

### 3.1 D 层（不变）

**D5 Observability** 保持公共域身份，向所有域提供可观测能力，**不调整 D 层职责边界**。

### 3.2 S 层 — Canonical（4+1 价值流）

```
D5（Observability / 公共域）
├── D5-S21 Instrument          # C1：生成遥测（Span + Metric + Log + 属性构建）
├── D5-S22 Export              # C2：导出遥测到外部系统（OTLP/Prometheus/Console）
├── D5-S23 Diagnose            # C3：诊断辅助（Coverage 报告 + Incident 导出 + Health）
├── D5-S24 Configure           # C4：配置加载/验证 + 运行时路径计数
└── D5-S0  Facade              # 横切：Init / Bridge / Shutdown
```

**S ↔ 承诺 1:1 对应表：**

| S ID | Scenario | 对应承诺 | 消费者可验证 WHAT | 旧 S 归属（冻结追溯） |
|------|----------|---------|-------------------|---------------------|
| D5-S21 | Instrument | C1 遥测生成 | 任意操作可创建 Span、记录 Metric、写入 Log | S1 Tracer + S2 Metrics + S3 Logger + S6 Telemetry |
| D5-S22 | Export | C2 遥测导出 | Span/Metric 正确到达 OTLP/Prometheus/Console | S4 Exporter |
| D5-S23 | Diagnose | C3 诊断辅助 | Coverage 报告准确、Incident bundle 可导出、Health 可用 | S5 Coverage + S8 Incident + S0-A02 HealthCheck |
| D5-S24 | Configure | C4 配置管理 | 改 yaml 切换 exporter/采样率，运行时路径计数准确 | S7 Settings + S9 Runtime |

### 3.3 关键设计决策

**D1: S6 Telemetry 并入 S21 Instrument**

`telemetry/names.go`（ResolveLayerComponent / BuildSpanAttrs / BuildGenAIAttrs）是 Span/Metric 创建的**辅助函数**，不构成独立价值流。归入 Instrument。

**D2: S9 Runtime 并入 S24 Configure**

`runtime/path_resolver.go`（RecordRuntimePath）+ `runtime/runtime_metric.go`（RegisterRuntimeMetric）是配置驱动的运行时指标，与 Settings 同属「配置与管理」价值流。

**D3: S0-A02 HealthCheck 上移至 S23 Diagnose**

HealthCheck 返回 coverage 摘要 + tracer/logger 状态，语义上属于诊断面。

**D4: 性能 T 下沉到 CROSS 段**

D5-S2-A01-T06（Compression P99 latency）和 T07（Concurrent session memory）从 D5-S2 迁至 t-registry CROSS 段，不作为 D5 域内 T。

### 3.4 scenario-slug 注册表（草案）

| S ID | scenario-slug | v2.0 目标目录 | 当前路径 |
|------|---------------|-------------|---------|
| D5-S21 | `instrument` | `observability/instrument/` | `tracer/` + `metrics/` + `logger/` + `telemetry/` |
| D5-S22 | `export` | `observability/export/` | `exporter/` |
| D5-S23 | `diagnose` | `observability/diagnose/` | `coverage/` + `incident/` |
| D5-S24 | `configure` | `observability/configure/` | `settings/` + `runtime/` |
| — | `facade` | `observability/` | `observability.go` + `bridge.go` + `health.go` + `config.go` |

### 3.5 T 层迁移策略

38 条 T（35 IMPLEMENTED + 3 PLANNED）**不改测试代码**。`t-registry.md` 增 `canonical_s` 列：

| 旧 S | T 数 | 新 Canonical S | 备注 |
|------|------|---------------|------|
| D5-S1 Tracer | 6 | D5-S21 Instrument | 全部迁移 |
| D5-S2 Metrics | 9 | D5-S21 Instrument（7）+ CROSS（2） | T06/T07 → CROSS |
| D5-S3 Logger | 6 | D5-S21 Instrument | 全部迁移 |
| D5-S4 Exporter | 3 | D5-S22 Export | 全部迁移 |
| D5-S5 Coverage | 6 | D5-S23 Diagnose | 全部迁移 |
| D5-S6 Telemetry | 4 | D5-S21 Instrument | 全部迁移 |
| D5-S8 Incident | 2 | D5-S23 Diagnose | 全部迁移 |
| D5-S9 Runtime | 3 | D5-S24 Configure | 全部迁移 |
| D5-S0 Cross | 2 | D5-S23 Diagnose（1）+ CROSS（1） | T02 PLANNED → CROSS |

**特殊处理：D5-S7 Settings**

Settings 的 2 个 A（LoadObsConfig / ValidateObsConfig）无独立 T 点，其 A 归入 D5-S24。

---

## 4. Success Metrics

| 指标 | 基线 | v1.0 目标 |
|------|------|----------|
| D5 价值流 S 数 | 0/9 | 4+1 |
| S 名语义化 | 0/9（全为包名） | 4/4（动词名词） |
| P0 T 全绿 | 13 | 13（保持） |
| T 总数 | 38 | 38 + canonical 列 |
| scenario-slug 语义化 | 0/9 | 4/4 |

---

## 5. Implementation Plan

### Phase A — S1→S2 澄清

- `demand.md`
- `proposal.md`（本文件）

### Phase B — v1.0 Registry（纯文档，零代码变更）

- D5 `a-registry.md` Canonical 重排（4+1 S 层）
- D5 `t-registry.md` 增 `canonical_s` 列 + Legacy 双轨
- D5 `f-registry.md`（如存在）同步
- `layering.md` §D5 双轨
- `code-layout.md §4` 补 D5 scenario-slug 注册表
- `cross-domain-boundaries.md` §D5 扩展

### Phase C — S3 design + S3-Gate

- `design.md`：A/F 编排 + Decision 表
- S3-Gate review

### Phase D — v1.0 验收

- 38 T 追溯表 100% 覆盖
- `acceptance-report.md`
- **零 Go 变更**

### Phase E — v2.0（后续 change）

- 物理路径 scenario-slug 迁移
- `observability/instrument/` `observability/export/` `observability/diagnose/` `observability/configure/`

---

## 6. Out of Scope（本 change v1.0）

- Go 代码移动
- 修改已有 `// T:` 测试注释
- 物理路径迁移（v2.0）
- D6 probe 对接

---

## 7. 相关文档

| 文档 | 用途 |
|------|------|
| `docs/methodology/dsaft-refactoring-playbook.md` | 方法论 SoT |
| `openspec/archive/2026-06-14-devrix-d3-sa-refine/proposal.md` | 首案样板 |
| `openspec/archive/2026-06-15-devrix-d4-sa-refine/proposal.md` | 同型参考 |
| `openspec/specs/d5-observability/a-registry.md` | 现行 A 注册表 |
| `openspec/specs/d5-observability/t-registry.md` | 现行 T 注册表 |
