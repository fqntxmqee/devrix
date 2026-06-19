# D2 Context Engine — v2.2 Spec Snapshot (devrix-d2-structure-closure)

**Change ID:** devrix-d2-structure-closure
**Demand ID:** DM-20260619-007
**Base:** `openspec/specs/d2-context-engine/` v8.2.0
**Date:** 2026-06-19

---

## Overview

This spec snapshot captures the **v2.2 Structure 终态** of the D2 Context Engine domain, as closed by devrix-d2-structure-closure. It points to the canonical spec in `openspec/specs/d2-context-engine/` and records the structural deltas introduced by this change.

The v2.2 final form is documented in:
- **`openspec/specs/d2-context-engine/d2-domain.md` v8.2.0** (canonical 域 spec)
- **`openspec/specs/d2-context-engine/layer-delta.md`** (v8.0.0 → v8.2.0 section)
- **`openspec/specs/d2-context-engine/a-registry.md`** (with updated Code Locations)
- **`openspec/specs/d2-context-engine/t-registry.md`** (D2-STRUCT-T01..T07 IMPLEMENTED)
- **`openspec/specs/d2-context-engine/span-registry.md` v2.3.0** (path sync)
- **`openspec/changes/devrix-d2-structure-closure/specs/d2-context-engine_delta.md`** (this change's delta spec)

## Scenario Coverage

| S | Scenario | Status | Notes |
|---|----------|--------|-------|
| D2-S15 | PrepareExecutionContext | ✅ REGISTRY (v8.2 orchestrator wired) | scenario orchestrator = production SoT |
| D2-S17 | PersistSessionState | ✅ REGISTRY (v8.2 orchestrator wired) | scenario orchestrator = production SoT |
| D2-S18 | EnforceExecutionPolicy | ✅ REGISTRY (v8.2 tools/sandbox 归位) | scenario orchestrator = production SoT |
| D2-S16 | RunQueryLoop | ❌ REMOVED (v8.0.0, DM-20260618-010) | → D7-S2-A06 |
| D2-S19 | NestedExecution | ❌ DISMANTLED (v6.4.0) | → S15+S18 split |
| D2-S20 | LegacyHarnessFallback | ❌ REMOVED (v6.5.0) | harness fully removed |

## v2.2 Final Paths (Canonical)

```
internal/layers/contextengine/
├── contracts.go                         # kernel re-export
├── aliases.go                           # type aliases (legacy.* / tools.*)
├── kernel/                              # contracts.go + spans.go
├── prepare/                             # D2-S15 (orchestrator + memory/recall + compression + token + prompt + conversation + attachments + usercontext + tools_list)
├── persist/                             # D2-S17 (orchestrator + snapshot + transcript + commit + memory/store)
├── enforce/                             # D2-S18 (orchestrator + permission + sandbox + tools + background + subquery + planmode_tools + tool_filter + agent_role_filter)
└── legacy/                              # P5 retired (Process() Deprecated + slog.Warn + T07 guard)
```

Cross-references:
- `internal/shared/types/memory.go` — `MemoryEntry` (P4 shared type)
- `internal/shared/contracts/memory.go` — `LongTermRecaller` / `LongTermStore` ports (P4)
- `internal/lint/layer/d2_layout_test.go` — D2-STRUCT-T01..T07 guards (P2/P3/P4/P5)

## Migration Map (Pre-v2.2 → v2.2)

| Pre-v2.2 | v2.2 final | Phase |
|----------|------------|-------|
| `enforce/toolrunner/` (49 prod + 21 test files) | `enforce/tools/` (package `tools`) | P3-T2 |
| `contextengine/sandbox/` | `contextengine/enforce/sandbox/` | P3-T1 |
| `enforce/orchestrator.go` (92 行 stub) | DELETED (dispatch by `turn_adapter`) | P3-T4 |
| `prepare/memory/longterm.go` (combined) | `prepare/memory/recall.go` + `persist/memory/store.go` | P4 |
| `facade/` | `legacy/` (Deprecated) | P5 |
| `engine_persist.go` duplicate inline | `persist/orchestrator.go` + `commit.go` | P1-e |
| `engine_prepare.go` duplicate inline | `prepare/orchestrator.go` | P1-d |
| `engine.go` 212 行 facade | `legacy/engine.go` (Deprecated) | P5 |

## Layout Guards (D2-STRUCT-T01..T07)

All 7 layout guards are **IMPLEMENTED** in `internal/lint/layer/d2_layout_test.go`. See `t-registry.md` for the canonical T-point definitions.

| T ID | Description | Phase |
|------|-------------|-------|
| D2-STRUCT-T01 | Root has only `contracts.go` + `aliases.go` | P2 |
| D2-STRUCT-T02 | No `engine_persist.go` outside `facade/` (or `legacy/`) | P1 |
| D2-STRUCT-T03 | `enforce/tools/` uses `package tools` | P3 |
| D2-STRUCT-T04 | No cyclic import between prepare/memory and persist/memory | P4 |
| D2-STRUCT-T05 | `enforce/orchestrator.go` is deleted | P3 |
| D2-STRUCT-T06 | Scenario sub-directory depth ≤2 | P3 |
| D2-STRUCT-T07 | No new `legacy.Process()` production callers | P5 |

## Acceptance Summary

- **19/19 AC PASS** (6 P0 + 13 P1) — see `acceptance-report.md`
- **9 commits** merged on `feat/d2-structure-p1e-persist-orchestrator`
- **6 docs synced** (d2-domain v8.2.0, code-layout v1.12.0, layering v4.7.0, layer-delta, span-registry v2.3.0, code-atlas)
- **70+ files** renamed (`toolrunner/` → `tools/`)
- **13 files** migrated (`facade/` → `legacy/`)
- **1 stub deleted** (`enforce/orchestrator.go`)
- **2 new files** (`shared/types/memory.go` + `shared/contracts/memory.go`)
- **7 layout guards** IMPLEMENTED

## Status

**S7_Archived.** All implementation complete; spec double-anchored; layout guards enforced; tests green. Change directory moves to `openspec/archive/2026-06-19-devrix-d2-structure-closure/`.
