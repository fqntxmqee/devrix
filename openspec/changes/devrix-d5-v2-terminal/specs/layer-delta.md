# Delta: D5 v2.1 Terminal (devrix-d5-v2-terminal)

**Change ID:** devrix-d5-v2-terminal  
**Demand ID:** DM-20260619-006  
**Base:** `openspec/specs/d5-observability/layer-delta.md` v3.0.0  
**Status:** S3 Draft

> S7 归档时合并入主 `layer-delta.md` 为 §v2.1-Terminal。

---

## ADDED: Documentation Terminal State

D5 MUST publish complete domain doc stack aligned with D7/D6 `*-domain.md` pattern.

#### Scenario: Domain SoT exists

- GIVEN Phase A merged
- WHEN onboarding to D5
- THEN `d5-domain.md`, `observability-guide.md`, `d5-boundary.md`, `terminal-state-guide.md` exist under `openspec/specs/d5-observability/`

---

## ADDED: S23 Sub-Commitments C3a–C3e

D5-S23 Diagnose MUST document five sub-commitments without new S-layer IDs.

| 子承诺 | Activities |
|--------|------------|
| C3a | A01–A03 |
| C3b | A04–A05 |
| C3c | A06, A10 |
| C3d | A07 |
| C3e | A09 |

---

## ADDED: Activities S21-A14, S0-A03, S23-A07/A09/A10

| A ID | Name | Scenario |
|------|------|----------|
| D5-S21-A14 | FilterDebugLog | S21 Instrument |
| D5-S0-A03 | TrackActiveSessions | S0 Facade |
| D5-S23-A07 | TrackFileDiagnostics | S23 C3d |
| D5-S23-A09 | InjectFault | S23 C3e |
| D5-S23-A10 | RunDoctorChecks | S23 C3c |

---

## MODIFIED: spec.md Primary Narrative

`spec.md` v3.0 MUST use D5-S21–S24 as primary DSAFT table; Legacy S1–S9 frozen.

---

## MODIFIED: Primary Trace Documentation

Documentation primary path MUST be D7 Turn spans; `query.loop.*` RETIRED in docs only (code already removed DM-20260618-010).

---

## MODIFIED: a-registry / f-registry Paths

All Code Location columns MUST use `instrument/`, `export/`, `diagnose/`, `configure/` paths.

---

## REMOVED: Bridge Packages (Phase B2)

Nine `observability/{tracer,metrics,logger,telemetry,exporter,coverage,incident,settings,runtime}/` bridge-only directories MUST be deleted.

#### Scenario: No bridge import in Go

- GIVEN Phase B2 merged
- WHEN grep `observability/tracer` in `*.go`
- THEN zero matches outside docs/archive

---

## REMOVED: Root Orphan Files (Phase B1)

- `observability/genai_tokens.go` → `instrument/metrics/`
- `observability/llm_log.go` → `diagnose/incident/`
- `observability/slog_bridge.go` → removed (use `instrument/logger` + `observability.go`)

---

## RETIRED: D5-S23-A08 FilterDebugLog at S23

Semantic ownership moves to D5-S21-A14. T IDs D5-S23-A08-T* unchanged with canonical_s=S21.

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 4.1.0 | 2026-06-19 | v2.1-Terminal delta draft |
