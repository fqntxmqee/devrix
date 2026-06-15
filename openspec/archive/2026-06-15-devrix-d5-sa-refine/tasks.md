# Tasks: D5 Observability S/A 重切

**Demand ID:** DM-20260615-001

## Phase 1 — v1.0 Registry（~0.5d）

| ID | 任务 | L4 | 估行 |
|----|------|-----|------|
| T1 | `a-registry.md` Canonical 重排（4+1 S21–S24）+ Legacy 双轨 | L4-ARCH-LAYER-D5-SA | ~80 |
| T2 | `t-registry.md` 增 canonical_s 列 + Legacy T ID 列 + CROSS 段 | L4-ARCH-LAYER-D5-SA | ~100 |
| T3 | `layering.md` §D5 Canonical 表 + Legacy 表 | — | ~20 |
| T4 | `code-layout.md §4.6` D5 scenario-slug 注册表 | — | ~20 |
| T5 | `cross-domain-boundaries.md` 变更记录 | — | ~5 |
| T6 | `design.md` A/F 编排 + Decision 表 | — | ~60 |

## Phase 2 — v2.0 物理迁移（后续 change）

| ID | 任务 |
|----|------|
| T7 | `tracer/` + `metrics/` + `logger/` + `telemetry/` → `observability/instrument/` |
| T8 | `exporter/` → `observability/export/` |
| T9 | `coverage/` + `incident/` → `observability/diagnose/` |
| T10 | `settings/` + `runtime/` → `observability/configure/` |

## 依赖

```
T1 → T2 → T3/T4/T5（并行）→ T6
```

## 分支

`feat/DM-20260615-001-d5-sa-refine`
