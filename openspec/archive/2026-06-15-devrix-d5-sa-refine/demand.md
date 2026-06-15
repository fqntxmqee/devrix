---
demand-id: DM-20260615-001
title: D5 Observability — 技术模块→价值流 S/A 重切
source: 架构审计（D5 S 层 9 个 S 全部为 Go 包名，违反 code-layout.md §2）
priority: P1
status: S3_Design
dsaft_domain: observability
created: 2026-06-15
parent: dsaft-refactoring-playbook
related:
  - DM-20260614-016  # D3 SA Refine（首案）
  - DM-20260614-018  # D4 SA Refine
  - DM-20260615-002  # D6 SA Refine（并行）
---

# D5 Observability — 技术模块→价值流 S/A 重切

## 1. 背景

### 1.1 D5 根本目标

**作为公共域，向 D1–D7 所有域提供可观测能力：生成遥测数据（Span/Metric/Log）、导出到外部系统、诊断辅助、配置管理。**

消费者可验证承诺：

| # | 承诺 | 验收主体 |
|---|------|---------|
| C1 | **Instrument**：任意操作可创建 Span、记录 Metric、写入 Log，属性正确 | 所有域 |
| C2 | **Export**：Span/Metric 正确到达 OTLP/Prometheus/Console | 运维/SRE |
| C3 | **Diagnose**：Coverage 报告准确、Incident bundle 可导出、Health 可用 | 运维/调试 |
| C4 | **Configure**：改 yaml 切换 exporter/采样率；运行时路径计数准确 | Bootstrap |

### 1.2 现状问题

| 问题 | 根因 |
|------|------|
| D5-S1–S9 全部为 Go 包名（Tracer/Metrics/Logger/Exporter/...） | S 被目录结构绑架 |
| Instrument 承诺被拆为 4 个 S（S1 Tracer + S2 Metrics + S3 Logger + S6 Telemetry） | 技术模块切法 |
| Diagnose 能力分散在 S5 Coverage + S8 Incident | 无统一诊断入口 |
| `code-layout.md §4` 缺 D5 scenario-slug 注册表 | 历史遗漏 |

---

## 2. 方案概要

**S 层从 9 技术模块重切为 4+1 价值流（S21–S24 + S0 Facade）：**

| Canonical S | Scenario | 旧 S 归入 |
|-------------|----------|-----------|
| D5-S21 | Instrument | S1 Tracer + S2 Metrics + S3 Logger + S6 Telemetry |
| D5-S22 | Export | S4 Exporter |
| D5-S23 | Diagnose | S5 Coverage + S8 Incident + S0-A02 HealthCheck |
| D5-S24 | Configure | S7 Settings + S9 Runtime |

**v1.0 范围：** 纯文档 — 重排 a/t-registry、更新 layering/code-layout/cross-domain-boundaries。零 Go 代码变更。

---

## 3. 验收标准

- [x] `a-registry.md` Canonical v3.0（4+1 S + Legacy 列）
- [x] `t-registry.md` Canonical v3.0（canonical_s 列 + Legacy T ID 列）
- [x] `layering.md` §D5 Canonical + Legacy 双轨
- [x] `code-layout.md §4.6` D5 scenario-slug 注册表
- [ ] 38 T 追溯表 100% 覆盖验证
- [ ] S3-Gate review 通过
