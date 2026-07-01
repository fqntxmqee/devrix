# Delta Spec: D7 Historical S Cleanup

**Change:** devrix-d7-historical-s-cleanup (DM-20260701-003)
**Base:** spec.md v4.21.0

## ADDED Requirements

### REQ-D7-HC-01: Historical mapping document

The domain SHALL maintain `openspec/specs/d7-orchestration/historical-s-mapping.md` as the sole location for former D7-S7–S21 detailed A/F registration and v6.0.0 remap tables.

#### Scenario: Reader finds former S8 detail

- GIVEN a developer searches for D7-S8-A15 ObserveQuantize detail
- WHEN they open current `a-registry.md`
- THEN they SHALL see a pointer to `historical-s-mapping.md`
- AND the detailed table SHALL NOT appear as a current registry heading

### REQ-D7-HC-02: S3 explicit wave boundary

D7-S3 Wave Scheduler SHALL be documented as an explicit wave/background/delegate invocation path, not the default user-message ingress chain.

#### Scenario: Main path vs wave path

- GIVEN a normal user IM message arrives at ProcessMessage
- WHEN intent is not Command or Skip
- THEN execution SHALL route through RunSessionTurnLoop → ItemPipelineRunner
- AND SHALL NOT require WaveScheduler.Start on that path

## MODIFIED Requirements

### REQ-D7-SN-01 (extends DM-20260701-002): Current registry scope

Current `a-registry.md` and `f-registry.md` SHALL list only canonical S1–S6 activities/functions plus a Historical/Contract pointer section.
