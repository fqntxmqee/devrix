# Tasks: D5 + D6 SA Refine v2.0 — 物理路径迁移

**Demand ID:** DM-20260615-003

## Phase 1 — D6 迁移（~0.5h）

| ID | 任务 | 估行 |
|----|------|------|
| T1 | `evolution/orchestration/` → `evolution/guard/`（git mv + package + import） | ~20 |
| T2 | `evolution/eval/` → `evolution/evaluate/`（git mv + package + import） | ~30 |
| T3 | D6 bridge 文件（eval/bridge.go + orchestration/bridge.go） | ~50 |

## Phase 2 — D5 S22 Export（~0.3h）

| ID | 任务 | 估行 |
|----|------|------|
| T4 | `observability/exporter/` → `observability/export/`（git mv + package + import） | ~20 |
| T5 | Bridge 文件: `observability/exporter/bridge.go` | ~30 |

## Phase 3 — D5 S24 Configure（~0.5h）

| ID | 任务 | 估行 |
|----|------|------|
| T6 | `observability/settings/` → `observability/configure/settings/` | ~15 |
| T7 | `observability/runtime/` → `observability/configure/runtime/` | ~20 |
| T8 | Bridge 文件: settings/bridge.go + runtime/bridge.go | ~20 |

## Phase 4 — D5 S23 Diagnose（~0.5h）

| ID | 任务 | 估行 |
|----|------|------|
| T9 | `observability/coverage/` → `observability/diagnose/coverage/` | ~25 |
| T10 | `observability/incident/` → `observability/diagnose/incident/` | ~15 |
| T11 | Bridge 文件: coverage/bridge.go + incident/bridge.go | ~30 |

## Phase 5 — D5 S21 Instrument（~1.5h）

| ID | 任务 | 估行 |
|----|------|------|
| T12 | `observability/tracer/` → `observability/instrument/tracer/`（46 importers） | ~40 |
| T13 | `observability/metrics/` → `observability/instrument/metrics/`（19 importers） | ~25 |
| T14 | `observability/logger/` → `observability/instrument/logger/`（3 importers） | ~15 |
| T15 | `observability/telemetry/` → `observability/instrument/telemetry/`（26 importers） | ~30 |
| T16 | Bridge 文件: tracer/metrics/logger/telemetry（4 文件） | ~40 |
| T17 | 更新 `observability/observability.go` 根 facade import | ~10 |

## Phase 6 — 文档同步（~0.3h）

| ID | 任务 | 估行 |
|----|------|------|
| T18 | `code-layout.md` §4.6/§4.7 更新路径 | ~10 |
| T19 | `layering.md` D5/D6 布局更新 | ~10 |
| T20 | `cross-domain-boundaries.md` revision entry | ~5 |

## 依赖

```
T1 → T2（D6 独立）
T4 → T6/T7 → T9/T10（D5 按依赖顺序）
T12/T13/T14/T15 可并行 → T17
T18 → T19/T20（文档依赖代码完成）
```

## 分支

`feat/DM-20260615-003-d5-d6-physical-migration`
