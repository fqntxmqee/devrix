---
demand-id: DM-20260607-005
title: 可观察层缺陷修复（Gauge/Histogram/Shutdown）
source: 架构/Code Review
priority: P0
status: S7_ARCHIVED
l1-domain: devrix
created: 2026-06-07
---

# 可观察层缺陷修复

## 1. 原始描述

> `devrix-observability` V1 归档后深度 Review 发现 13 个缺陷（3 Critical、4 High、6 Medium），影响 Metrics 正确性、Graceful Shutdown 与规范一致性。本变更修复 V1.1 范围内问题，不引入新外部依赖。

## 2. 澄清记录

### Q1: 修复范围？

**A**: V1.1 仅修复 proposal 中 13 项；Tail-based Sampling、APM 归 V3。 — 2026-06-07

### Q2: 是否破坏公开 API？

**A**: Bridge 层 API 不变；Metrics/Tracer 内部实现修复，配置 YAML 兼容。 — 2026-06-07

## 3. 范围

**In Scope**: Gauge 精度、Histogram 累积、Tracer Shutdown 刷写、M1 Critical（T1–T3）

**Out of Scope**: 日志采样、ConsoleExporter 重构（M2+）
