# Design: D5 + D6 SA Refine v2.0 — 物理路径迁移

**Change ID:** devrix-d5-d6-sa-refine-v2.0
**Demand ID:** DM-20260615-003
**阶段:** S3 Design
**版本:** v1.0
**关联:** `proposal.md`

---

## 1. 概述

v2.0 执行 D5/D6 SA Refine 的物理路径迁移阶段。详见 `/Users/fukai/.claude/plans/indexed-spinning-rabbit.md` 完整实施计划。

## 2. 目标目录结构

```
D5 Observability:
  observability/
    instrument/tracer/        (package tracer)
    instrument/metrics/       (package metrics)
    instrument/logger/        (package logger)
    instrument/telemetry/     (package telemetry)
    export/                   (package export)
    diagnose/coverage/        (package coverage)
    diagnose/incident/        (package incident)
    configure/settings/       (package settings)
    configure/runtime/        (package runtime)

D6 Evolution:
  evolution/evaluate/         (package evaluate)
  evolution/guard/            (package guard)
```

## 3. 迁移映射

### D5

| 旧路径 | 新路径 | Package | Importers |
|--------|--------|---------|-----------|
| `observability/tracer/` | `observability/instrument/tracer/` | tracer | 46 |
| `observability/metrics/` | `observability/instrument/metrics/` | metrics | 19 |
| `observability/logger/` | `observability/instrument/logger/` | logger | 3 |
| `observability/telemetry/` | `observability/instrument/telemetry/` | telemetry | 26 |
| `observability/exporter/` | `observability/export/` | export | 1 |
| `observability/coverage/` | `observability/diagnose/coverage/` | coverage | 15 |
| `observability/incident/` | `observability/diagnose/incident/` | incident | 2 |
| `observability/settings/` | `observability/configure/settings/` | settings | 13 |
| `observability/runtime/` | `observability/configure/runtime/` | runtime | 5 |

### D6

| 旧路径 | 新路径 | Package | Importers |
|--------|--------|---------|-----------|
| `evolution/eval/` | `evolution/evaluate/` | evaluate | 2 |
| `evolution/orchestration/` | `evolution/guard/` | guard | 1 |

## 4. Package 重命名（仅 3 个）

| 旧 Package | 新 Package | 原因 |
|------------|-----------|------|
| `eval` | `evaluate` | 语义对齐 S11 RunEvaluation |
| `orchestration` | `guard` | 消除 D7 orchestration 命名冲突 |
| `exporter` | `export` | 语义对齐 S22 Export |

## 5. 向后兼容

11 个 bridge 文件，每个旧目录保留 1 个 `bridge.go`：

```go
// Package <old> is a backward-compatibility bridge.
// Deprecated: use <new-path> instead.
package <old>

import "<module>/<new-path>"

type A = <new-pkg>.A
var Fn = <new-pkg>.Fn
```

生命周期：v2.0 创建 → v2.1 删除。

## 6. 统计

| 指标 | 旧 | 新 |
|------|-----|-----|
| D5 子目录 | 9 | 4 场景目录（含 9 子包） |
| D6 子目录 | 2 | 2（evaluate + guard） |
| Package 重命名 | — | 3 |
| Bridge 文件 | 0 | 11 |
| Import 路径更新 | — | ~133 |
