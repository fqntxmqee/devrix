# Proposal: D7 Historical S Cleanup

## Problem

DM-20260701-002 labeled S7+ as historical but left ~450 lines of MUPS node detail inside current registries, causing readers to treat former S IDs as active scenarios. S3 WaveScheduler role was also ambiguous relative to the ItemPipeline main path.

## Solution

1. Create `openspec/specs/d7-orchestration/historical-s-mapping.md` as the single historical SoT for former S7–S21 A/F detail and v6.0.0 remap tables.
2. Trim `a-registry.md` and `f-registry.md` to canonical S1–S6 only, with a short pointer section.
3. Clarify in `spec.md` that D7-S3 serves explicit wave/background/delegate scheduling, not ordinary user-message ingress.
4. Extend architecture guard tests to block S7+ headings and retired fastpath paths in current registries.

## Success Criteria

- Current registries shrink materially while historical traceability remains in one file.
- S3 boundary is explicit in spec Architecture + Scenarios tables.
- All D7-HC T points IMPLEMENTED.

## Risks

- External docs linking to old registry anchors — mitigated by keeping IDs in historical doc.
- T registry still references D7-S8-A15 etc. — acceptable; T IDs are immutable trace keys.
