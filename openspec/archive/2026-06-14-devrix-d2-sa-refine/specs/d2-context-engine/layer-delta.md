# Delta: D2 SA Refine — Canonical S15–S20 + D7 Boundary

**Change ID:** devrix-d2-sa-refine  
**Demand ID:** DM-20260614-009  
**Canonical spec:** `openspec/specs/d2-context-engine/spec.md`  
**Status:** Merged (v1.0 registry)  
**Affects:** D2 S layer, D7↔D2 boundary, registries, code-layout

---

## ADDED

### Requirement: D2 Canonical Value Stream S15–S20

D2 Context Engine MUST register six canonical scenarios (S15–S20) by execution lifecycle. Legacy S1–S14 MUST remain frozen for traceability only.

#### Scenario: Canonical SoT documented

- GIVEN DM-20260614-009 merged
- WHEN reader opens `d2-domain.md`
- THEN S15–S20 table is present with North Star mapping
- AND S1–S14 marked Legacy Module Index

### Requirement: D2 Execution Follower Boundary

D2 MUST act as Execution Follower to D7 Leader. Task write model, PlanMode decisions, delegate routing, and FlowEvent hub aggregation MUST NOT be D2 SoT.

#### Scenario: Out of Scope table complete

- GIVEN `d7-boundary.md`
- WHEN cross-domain matrix is read
- THEN tasks/, delegate_tools, queue delegate-progress list D7 targets
- AND v2.0 migration phase is declared

### Requirement: D7 QueryLoopExecutor Integration

Production path MUST wire D7 `SessionOrchestrator` to D2 via `bootstrap.d2Executor` implementing `QueryLoopExecutor`, calling `contracts.IEngine.Process`.

#### Scenario: WireD7 active path

- GIVEN `d7.enabled=true`
- WHEN process starts
- THEN gateway orchestration entry is non-nil
- AND D2 Process is reachable only through D7 adapter or tests

---

## MODIFIED

### Requirement: D2 Domain Documentation

`d2-domain.md` MUST reference `d7-boundary.md` as cross-domain SoT. `layering.md` D2 section MUST show dual-track Canonical + Legacy tables.

### Requirement: Registry Canonical Columns

`a-registry.md`, `f-registry.md`, `t-registry.md` MUST include Canonical S15–S20 entries and Legacy→Canonical mapping columns without changing existing test `// T:` annotations.

---

## REMOVED

(None — v1.0 registry only)
