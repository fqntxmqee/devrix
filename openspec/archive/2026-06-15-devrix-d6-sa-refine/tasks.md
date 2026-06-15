# Tasks: D6 Evolution S/A 重切

**Demand ID:** DM-20260615-002

## Phase 1 — v1.0 Registry（~0.5d）

| ID | 任务 | L4 | 估行 |
|----|------|-----|------|
| T1 | `a-registry.md` Canonical 重排（4 S11–S14）+ Legacy 双轨 | L4-ARCH-LAYER-D6-SA | ~60 |
| T2 | `t-registry.md` 增 canonical_s 列 + Legacy T ID 列 | L4-ARCH-LAYER-D6-SA | ~80 |
| T3 | `layering.md` §D6 Canonical 表 + Legacy 表 | — | ~20 |
| T4 | `code-layout.md §4.7` D6 scenario-slug 注册表 | — | ~20 |
| T5 | `cross-domain-boundaries.md` 变更记录 | — | ~5 |
| T6 | `design.md` A/F 编排 + Decision 表 | — | ~50 |

## Phase 2 — v2.0 物理迁移（后续 change）

| ID | 任务 |
|----|------|
| T7 | `evolution/orchestration/` → `evolution/guard/`（消除 D7 命名冲突） |
| T8 | `evolution/eval/` → `evolution/evaluate/`（slug 语义化） |

## 依赖

```
T1 → T2 → T3/T4/T5（并行）→ T6
```

## 分支

`feat/DM-20260615-002-d6-sa-refine`
