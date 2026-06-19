# D5 Observability — Spec Delta (devrix-d5-v2-terminal)

**Change ID:** devrix-d5-v2-terminal  
**Demand ID:** DM-20260619-006  
**Base:** `openspec/specs/d5-observability/spec.md` v2.0.0  
**Target:** v3.0.0（Canonical S21–S24 主叙事）

---

## MODIFIED: DSAFT Structure Primary Narrative

`spec.md` DSAFT 结构表与 Scenarios 表 MUST 以 **D5-S21–S24 + S0** 为 Canonical 主表；D5-S1–S9 下沉至「Legacy Module Index（冻结追溯）」节。

#### Scenario: Canonical S 表为 spec 主叙事

- GIVEN `devrix-d5-v2-terminal` Phase A is merged
- WHEN a reader opens `openspec/specs/d5-observability/spec.md`
- THEN the DSAFT 结构表 lists D5-S21 Instrument, S22 Export, S23 Diagnose, S24 Configure, S0 Facade
- AND D5-S1 Tracer through D5-S9 Runtime appear only under Legacy 冻结追溯

<!-- T: — (文档 AC) -->

---

## MODIFIED: Primary Trace Path

生产主路径 Trace MUST 以 D7 Turn span 族为 Canonical；`query.loop.*` MUST NOT appear as active primary path in Overview / Architecture / design 主流程图。

#### Scenario: D7 Turn is canonical trace root under gateway

- GIVEN tracing enabled and D7 Turn path active
- WHEN a user message completes one LLM iteration
- THEN Jaeger shows `gateway.message.receive` → `orchestration.turn.run` → `orchestration.turn.iteration` → `orchestration.llm.invoke` → `llm.stream`
- AND no `query.loop.run` span is created

<!-- T: D5-S22-A01-T02, D7-S2-A06-T01 -->

#### Scenario: query.loop documented as RETIRED only

- GIVEN Phase A documentation sync
- WHEN `grep query.loop` runs on `openspec/specs/d5-observability/`
- THEN matches appear only in RETIRED / Legacy / Revision History sections
- AND NOT in Overview primary path or Scenarios IMPLEMENTED tables as active ops

<!-- T: — (文档 AC-A7) -->

---

## ADDED: Domain SoT Document

D5 MUST have `d5-domain.md` as domain-level SoT，结构对齐 `d6-domain.md` / `d7-domain.md`。

#### Scenario: d5-domain North Star discoverable

- GIVEN Phase A is merged
- WHEN an architect onboards to D5
- THEN `openspec/specs/d5-observability/d5-domain.md` exists
- AND it contains North Star, 4 可验证承诺, Out of Scope, 物理路径表, Legacy 双轨

<!-- T: — (AC-A1) -->

---

## ADDED: Observability Guide

D5 MUST publish `observability-guide.md` with Span↔T P0 matrix, D7 Turn Trace tree, and P0 Runbook.

#### Scenario: P0 Runbook lists Health and export

- GIVEN Phase A is merged
- WHEN SRE runs P0 observability checks
- THEN `observability-guide.md` §Runbook documents `/health` coverage fields, zero_hit investigation, and `devrix debug export`
- AND Span↔T matrix references existing T IDs without renumbering

<!-- T: D5-S23-A01-T02, D5-S23-A04-T01, D5-S8-A01-T02 -->

---

## ADDED: Cross-Domain Boundary Document

D5 MUST have `d5-boundary.md` documenting Bridge contracts and D2→D5 Tracker read boundary.

#### Scenario: D2 TrackerSurface read-only contract documented

- GIVEN Phase A is merged
- WHEN D2 engineer implements TrackerSurface
- THEN `d5-boundary.md` states D2 MUST only read via `diagnose/tracker` APIs
- AND writes remain D5-owned

<!-- T: D5-S23-A07-T01 -->

---

## MODIFIED: S23 Diagnose Sub-Commitments

D5-S23 Diagnose MUST document sub-commitments C3a–C3e without adding new S-layer IDs.

| 子承诺 | A 层（终态） |
|--------|-------------|
| C3a Coverage | A01–A03 |
| C3b Incident | A04–A05 |
| C3c Health + Doctor | A06, A10 |
| C3d File Tracker | A07 |
| C3e FaultInject | A09 |

#### Scenario: Doctor canonical A is A10

- GIVEN `a-registry.md` v4.0 is merged
- WHEN Doctor checks are traced to A layer
- THEN canonical Activity is D5-S23-A10 RunDoctorChecks
- AND T IDs D5-S23-A03-T01/T02 remain frozen with `canonical_a=A10`

<!-- T: D5-S23-A03-T01, D5-S23-A03-T02 -->

---

## MODIFIED: DebugFilter belongs to S21

DebugFilter MUST be registered as D5-S21-A14 FilterDebugLog; physical path `instrument/logger/debugfilter/`.

#### Scenario: DebugFilter T canonical_s is S21

- GIVEN `t-registry.md` v3.2
- WHEN D5-S23-A08-T01 is listed
- THEN `canonical_s` column reads S21
- AND `canonical_a` reads A14

<!-- T: D5-S23-A08-T01, D5-S23-A08-T02 -->

---

## MODIFIED: SessionBridge belongs to S0

SessionBridge ActiveSessions gauge MUST be registered as D5-S0-A03 TrackActiveSessions.

#### Scenario: SessionBridge T canonical_s is S0

- GIVEN `t-registry.md` v3.2
- WHEN D5-S23-A06-T01 is listed
- THEN `canonical_s` column reads S0
- AND `canonical_a` reads A03

<!-- T: D5-S23-A06-T01 -->

---

## MODIFIED: Bridge Package Removal (Phase B)

After Phase B2, deprecated bridge packages MUST NOT exist under `internal/layers/observability/`.

#### Scenario: No legacy bridge import in code

- GIVEN Phase B2 is merged
- WHEN `grep` runs for `internal/layers/observability/tracer` in `*.go` (excluding docs/archive)
- THEN zero matches
- AND Facade imports `instrument/tracer`, `instrument/metrics`, `instrument/logger` directly

<!-- T: 全量 41 T (AC-B1) -->

#### Scenario: Root orphan files relocated

- GIVEN Phase B1 is merged
- WHEN listing `internal/layers/observability/`
- THEN `genai_tokens.go` lives under `instrument/metrics/`
- AND `llm_log.go` lives under `diagnose/incident/`
- AND root `slog_bridge.go` is removed (logic via `instrument/logger` + `observability.go`)

<!-- T: D5-S21-A07-T01, D5-S23-A05 (LLM JSONL) -->

---

## MODIFIED: Registry and F-Layer Sync

`a-registry.md` v4.0 and `f-registry.md` v3.0 MUST sync Code Location to v2.0 physical paths and include diagnostic A/F entries.

#### Scenario: a-registry Code Location uses instrument path

- GIVEN Phase A is merged
- WHEN reading D5-S21-A01 CreateSpan Code Location
- THEN path is `instrument/tracer/tracer.go` not `tracer/tracer.go`

<!-- T: D5-S21-A01-T01 -->

---

## REMOVED: Deprecated Bridge Packages

The following directories MUST be deleted in Phase B2:

- `observability/tracer/`, `metrics/`, `logger/`, `telemetry/`, `exporter/`
- `observability/coverage/`, `incident/`, `settings/`, `runtime/`

Each previously contained only `bridge.go` re-exporting canonical packages.

---

## MODIFIED: Documentation Status (Phase A)

| Document | Required state after Phase A |
|----------|---------------------------|
| `spec.md` | v3.0.0, Canonical S 主表 |
| `d5-domain.md` | 新建 Active |
| `observability-guide.md` | 新建 Active |
| `terminal-state-guide.md` | 新建 Active |
| `dsaft-architecture.md` | Stub v1.0.0 |
| `d5-boundary.md` | 新建 Active |
| `design.md` | v3.0.0, D7 Turn 主路径 |
| `layer-delta.md` | §v2.1-Terminal |
| `a-registry.md` | v4.0.0 |
| `f-registry.md` | v3.0.0 |
| `span-registry.md` / `coverage.md` | query.loop RETIRED only |
| `architecture/code-layout.md` §4.6 | diagnose 全子目录 |
| `architecture/cross-domain-boundaries.md` | D5 契约表升级 |
