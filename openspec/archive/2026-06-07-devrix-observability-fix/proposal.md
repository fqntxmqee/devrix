# Proposal: Devrix 可观察层缺陷修复

**Change ID:** devrix-observability-fix
**Demand ID:** DM-20260607-005
**Parent Change:** devrix-observability
**Status:** S7 Archived
**Version:** 1.1.0
**Author:** Architecture
**Date:** 2026-06-07

---

## Archive Information

**Archived:** 2026-06-07
**Outcome:** Successfully implemented
**Canonical Spec:** `openspec/specs/observability/spec.md` v1.1.0

---

## 1. Background

`devrix-observability` (V1) 已完成 S4 实施，30 个 Go 文件、40 个任务全部标记完成。在深度代码 Review 中，发现 **13 个缺陷**，其中 **3 个 Critical**（数据错误 + 数据丢失），**4 个 High**（行为偏离规范），**6 个 Medium**（代码质量）。这些缺陷影响 Metrics 数据正确性、Graceful Shutdown 可靠性、以及规范契约一致性。

---

## 2. Problem Statement

### 2.1 缺陷清单

| # | Severity | 缺陷 | 文件 | 影响 |
|---|----------|------|------|------|
| 1 | **Critical** | Gauge float64 → uint64 转换精度丢失 | `metrics/gauge.go:82-88` | Gauge 所有操作产生错误数值 |
| 2 | **Critical** | Histogram 桶计数双重累积 | `metrics/histogram.go:56-60` + `prometheus.go:80-94` | Prometheus 输出桶计数严重偏高 |
| 3 | **Critical** | TracerProvider.Shutdown 未刷写 pending spans | `tracer/tracer.go:47-57` | 关闭时未完成的 Span 丢失 |
| 4 | **High** | Int64UpDownCounter 返回 Counter 而非 Gauge | `metrics/meter.go:88-108` | Session Gauge 无法递减 |
| 5 | **High** | Observability.Shutdown 遗漏 Metrics/Logger | `observability.go:105-123` | 组件未正确关闭 |
| 6 | **High** | Counter 中未使用的 sync.Mutex | `metrics/counter.go:19-24` | 死代码 |
| 7 | **High** | 日志采样配置存在但未实现 | `config.go` + `logger/logger.go` | 高频 Span 日志可能撑爆存储 |
| 8 | **Medium** | 错误日志无堆栈跟踪 | `logger/logger.go:106-182` | 问题定位困难 |
| 9 | **Medium** | Registry.sortedKeys 使用冒泡排序 | `metrics/registry.go:110-124` | 性能损耗 |
| 10 | **Medium** | ConsoleExporter 未直接实现 SpanExporter | `exporter/console.go` | 接口不一致 |
| 11 | **Medium** | NullExporter.ExportBatch 不在接口中 | `exporter/null.go:23-25` | 死代码 |
| 12 | **Medium** | 部分单元测试无实际断言 | `logger/logger_test.go`, `exporter/exporter_test.go` | 假绿测试 |
| 13 | **Medium** | LogEntry 中 SpanContext 引用不正确 | `logger/logger.go` → `tracer.SpanContext` | 直接依赖 tracer 包而非值类型 |

### 2.2 根本原因

- 自研 Metrics 实现中 float64 ↔ uint64 原子转换采用了错误的数学方法
- Histogram 的 Observe 逻辑和 Prometheus Output 逻辑各自实现了一次累积，产生双重累积
- TracerProvider/Tracer 的 activeSpans map 仅在 Start 时写入、EndSpan 时删除，Shutdown 未遍历
- Int64UpDownCounter 简化实现为 Counter，忽略了 Gauge 的双向语义

---

## 3. Proposed Solution

### 3.1 修复策略

| 缺陷 | 修复方案 | 影响文件 |
|------|---------|---------|
| Gauge float64 转换 | 使用 `math.Float64bits` / `math.Float64frombits` + Mutex | gauge.go |
| AsyncGauge/Counter 竞态 | 添加 `sync.Mutex` 保护回调执行 | async_gauge.go, async_counter.go |
| Histogram 双重累积 | Observe break + +Inf bucket 在 Observe 中递增 | histogram.go, prometheus.go |
| Shutdown 无刷写 | Shutdown 遍历 activeSpans → End → exporter.Shutdown; Start double-check | tracer.go |
| UpDownCounter → Gauge | 改为创建 Gauge 实例 | meter.go |
| Shutdown 遗漏 | 增加 logger 的关闭逻辑 | observability.go |
| Registry.Reset 不完整 | Reset 增加 Gauge/Histogram 重置 | registry.go |
| Handler 缺少 Close | Handler 接口新增 `Close() error` | logger.go |
| 死 Mutex | 删除 `mu sync.Mutex` 字段 | counter.go |
| 日志采样 | 实现 spanLogTracker（容量保护 + 淘汰） | logger.go |
| 堆栈跟踪 | StackTracer 接口 + runtime.Callers 回退 | logger.go |
| 冒泡排序 | 替换为 `sort.Strings` | registry.go |
| ConsoleExporter | 添加 `Export(ctx, span)` 方法，移除 adapter | console.go, factory.go |
| Logger 依赖 tracer | 改为接收 traceID/spanID 字符串 | logger.go |

### 3.2 不改动的部分

- 不修改 Bridge 层的公开 API（LLMBridge、ToolBridge 不变；SessionBridge 返回类型改为 Gauge 但编译时检查）
- 不修改配置结构体（YAML 兼容）
- 不引入新的外部依赖
- 不改变 SpanExporter / Span / Counter 等公开接口签名
- Logger.Handler 接口新增 Close() 为新增方法，两个内置实现为空操作

---

## 4. Version Scope

### V1.1（本变更）

| Category | Items |
|----------|-------|
| Critical Fix | Gauge 转换、AsyncGauge/Counter 竞态、Histogram 累积、Shutdown 刷写 + double-check |
| High Fix | UpDownCounter→Gauge、全组件 Shutdown、Registry.Reset、Handler.Close、死 Mutex 删除、日志采样 |
| Medium Fix | 堆栈跟踪（StackTracer）、Logger 解耦、sort.Strings、接口一致性、死代码清理 |
| Test Enhancement | 增加 11 个新 L5 测试点，强化现有测试断言 |

---

## 5. Success Metrics

| Metric | Target |
|--------|--------|
| Gauge 操作数值正确性 | 100%（float64 往返精度 ≤ 1e-15） |
| AsyncGauge/Counter 并发安全 | 100%（`-race` 零警告） |
| Histogram Prometheus 输出正确性 | 100%（与 golden 样本逐一比对） |
| Shutdown 刷写覆盖率 | 100%（Tracer + Logger） |
| Registry.Reset 覆盖类型 | 100%（Counter + Gauge + Histogram） |
| 新增 L5 测试通过率 | 11/11 P0 |
| 回归测试通过率 | 100%（已有集成/验收测试） |

---

## 6. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Gauge 修复影响已记录指标 | 低 — V1 尚未上生产 | 增加 L5-OBS-FIX-01 验证 |
| Histogram 修复改变输出格式 | 低 — 当前输出本身错误 | 增加 golden 样本测试 |
| Shutdown 改动引入竞态 | 中 | `-race` 测试 + sync.Mutex 保护 |

---

## 7. L5 测试点（新增）

| L5 ID | 描述 | 优先级 | 对应缺陷 |
|-------|------|--------|----------|
| L5-OBS-FIX-01 | Gauge Set/Inc/Dec/Add/Sub 数值正确 | P0 | #1 |
| L5-OBS-CONCUR-01 | AsyncGauge/Counter 回调并发安全 | P0 | 新增 |
| L5-OBS-FIX-02 | Histogram Prometheus 输出与 golden 一致 + Buckets() +Inf | P0 | #2 |
| L5-OBS-FIX-03 | Shutdown 刷写所有 pending spans + double-check | P0 | #3 |
| L5-OBS-FIX-04 | Shutdown 覆盖 Tracer + Logger | P0 | #5 |
| L5-OBS-FIX-05 | Int64UpDownCounter 返回 Gauge，Dec 可降为负 | P0 | #4 |
| L5-OBS-FIX-06 | Error 日志包含 stacktrace 字段（StackTracer + Callers） | P1 | #8 |
| L5-OBS-FIX-07 | 日志采样 max_entries_per_span + 容量保护 | P1 | #7 |
| L5-OBS-FIX-08 | ConsoleExporter 可直接作为 SpanExporter | P2 | #10 |
| L5-OBS-FIX-09 | Registry.Reset 覆盖 Counter + Gauge + Histogram | P1 | 新增 |
| L5-OBS-FIX-10 | Handler.Close 接口 + 委托验证 | P1 | 新增 |

---

## 8. 任务估算

| Milestone | 任务数 | 预估 |
|-----------|--------|------|
| M1 Critical Fix | 4 | 9.5h |
| M2 High Fix | 7 | 10.5h |
| M3 Medium Fix | 4 | 3.5h |
| M4 Test Enhancement | 3 | 8.5h |
| **合计** | **18** | **~32h** |
