---
demand-id: DM-20260607-007
title: 可观察层运行时代码染色与 Operation 对账
source: 架构 Review / 无效代码治理
priority: P1
status: S7_ARCHIVED
l1-domain: devrix
created: 2026-06-07
---

# 可观察层运行时代码染色与 Operation 对账

## 1. 原始描述

> 可观察层 V1.2 已在主链路（Gateway → Context → PEV → LLM）完成 Jaeger Operation 对齐，但大量模块（LongTerm、Plan/Milestone、Feishu Adapter）无 Span 埋点，无法通过线上流量判断功能切片是否被使用。需要按 OpenSpec 规范设计「运行时代码染色」能力，辅助识别疑似无效代码/闲置功能路径。

## 2. 澄清记录

### Q1: 「代码染色」粒度是什么？

**A**: V1.3 限定为 **Operation 级**（`{layer}.{module}.{action}`），不做函数/文件级 runtime coverage。函数级死代码仍依赖静态分析（CodeGraph / `deadcode`），线上染色用于 **功能切片是否被触发** 的粗粒度证据。 — 2026-06-07

### Q2: 对账数据来源？

**A**: **进程内** Operation 命中计数器（Span Start 时递增），与静态 Operation Registry 对账；不依赖 Jaeger API 查询（留 V1.4）。报告通过 Health 子端点或 CLI `obs-coverage-report` 输出。 — 2026-06-07

### Q3: 与 V1.2 canonical spec 关系？

**A**: 扩展 `telemetry/names.go` 与 canonical spec v1.3.0；已有 11 个 Operation 保持不变，新增 Operation 走 ADDED Requirements。 — 2026-06-07

### Q4: Metrics 统一范围？

**A**: Gateway 会话指标从 `communication/metrics/collector.go` 迁到 `SessionBridge`；权限相关 counter 本变更仅登记 spec，实现放 P1 任务末尾。 — 2026-06-07

### Q5: 采样对染色的影响？

**A**: 命中计数在 **Span Start** 时记录（不论 `recording` 是否采样丢弃），确保染色不受 `trace_id_ratio` 影响；仅导出到 Jaeger 的 span 仍受采样控制。 — 2026-06-07

## 3. 澄清范围

### 3.1 L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | devrix | Devrix 平台 | 已有 |
| L2 | L2-OBS-COVERAGE | 线上可观测性与闲置路径治理 | **新增** |
| L3-BE | L3-BE-OBS-01 | 导出运行期 Operation 覆盖报告 | **新增** |
| L3-BE | L3-BE-OBS-02 | 补全关键模块 Span 埋点 | **新增** |
| L4 | L4-OBS-REGISTRY | Operation 注册表 | **新增** |
| L4 | L4-OBS-COVERAGE | Operation 命中对账 | **新增** |
| L4 | L4-OBS-INSTRUMENT | 扩展 Span 埋点 | **新增** |
| L4 | L4-OBS-METRICS | SessionBridge 指标统一 | 已有（扩展） |
| L5 | L5-OBS-13 ~ L5-OBS-18 | 见 proposal / l5-registry | 草拟 |

### 3.2 范围

**In Scope**:

- Operation Registry 静态清单 + 元数据（layer、component、since_version）
- Tracer Start 时递增 Operation 命中计数（采样无关）
- Coverage 报告：registry 全集 vs 进程生命周期命中集
- P0 Span 补全：LongTerm、Plan/Milestone、Feishu Adapter 入站
- Gateway 会话 Gauge 迁至 SessionBridge
- OpenSpec delta spec v1.3.0 + tasks + L5 登记

**Out of Scope**:

- Go `runtime/coverage` 生产覆盖
- `net/http/pprof` 端点
- Jaeger / Prometheus 远程查询自动对账
- OTel SDK 迁移或自动插桩
- 函数级死代码检测（属 dev-brain / CI 静态分析）
