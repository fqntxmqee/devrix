# Observability Layer Specification

**Capability:** observability
**Change ID:** devrix-observability (archived 2026-06-07), devrix-observability-fix (archived 2026-06-07)
**Layer:** Observability
**Version:** 1.1.0
**Status:** Canonical — source of truth

---

## Overview

可观察层提供 Tracing、Metrics、结构化 Logging 与 Bridge 集成。V1（DM-20260607-001）建立基础能力；V1.1（DM-20260607-005）修复 Gauge/Histogram 数据错误、Shutdown 丢 Span、UpDownCounter 语义、日志采样与 ConsoleExporter 接口一致性。

---

## ADDED Requirements (V1.1 Fix)

### Requirement: Gauge Numeric Correctness

Gauge MUST 使用 mutex 保护 float64 读写，`Set`/`Add`/`Sub`/`Inc`/`Dec` 结果精确。

**Priority**: P0
**L5**: L5-OBS-FIX-01

---

### Requirement: Histogram Bucket Correctness

Histogram `Observe` MUST 仅递增第一个匹配桶；Prometheus 输出 MUST 正确累积各 `le` 桶与 `+Inf` 计数。

**Priority**: P0
**L5**: L5-OBS-FIX-02

---

### Requirement: Tracer Shutdown Flush

`TracerProvider.Shutdown` MUST 遍历 active spans、调用 `End` 并刷写至 exporter，避免 pending span 丢失。

**Priority**: P0
**L5**: L5-OBS-FIX-03

---

### Requirement: Observability Graceful Shutdown

`Observability.Shutdown` MUST 关闭 TracerProvider 与 Logger（`Close()`），错误聚合返回。

**Priority**: P0
**L5**: L5-OBS-FIX-04

---

### Requirement: Int64UpDownCounter Semantics

`Meter.Int64UpDownCounter` MUST 返回 Gauge（可增减），用于 Session 活跃数等场景。

**Priority**: P0
**L5**: L5-OBS-FIX-05

---

### Requirement: Error Log Stack Trace

结构化日志在 `error` 字段为 error 类型时 MUST 附加 `stack` 字段（`debug.Stack()`）。

**Priority**: P1
**L5**: L5-OBS-FIX-06

---

### Requirement: Per-Span Log Sampling

Logger MUST 遵守 `max_entries_per_span` 配置，超限时丢弃并发出 WARN。

**Priority**: P1
**L5**: L5-OBS-FIX-07

---

### Requirement: ConsoleExporter SpanExporter

`ConsoleExporter` MUST 直接实现 `SpanExporter` 接口（`Export(ctx, span)`），无需 adapter。

**Priority**: P2
**L5**: L5-OBS-FIX-08

---

## Inherited Requirements (V1)

V1 基线能力（Tracing Span 生命周期、Counter/Histogram 注册、JSON/Text 日志、Bridge 集成）见归档包 `openspec/archive/2026-06-07-devrix-observability/`。
