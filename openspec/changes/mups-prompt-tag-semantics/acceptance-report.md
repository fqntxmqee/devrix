# Acceptance Report: MUPS 标签语义层

**Change ID:** `mups-prompt-tag-semantics`  
**Demand ID:** DM-20260705-001  
**Date:** 2026-07-05  
**Verdict:** ACCEPTED

---

## 1. Summary

Implemented `TagSemanticsRegistry` and injected `RenderSemanticAppendix` before `DocBlock*` in Observe/Plan/Execute dynamic prompts. Observe/Plan user frames now include `[control]`/`[data]` prefixes and a compact field guide. No new D7 tactical prose added.

## 2. L5 Verification

| L5 ID | Given-When-Then | Result |
|-------|-----------------|--------|
| **L5-MUPS-TAG-01** | Observe appendix lists `obs_uncertainty` when-use | PASS — `TestObservationTaskAppendix_IncludesObserveKindSemantics` |
| **L5-MUPS-TAG-02** | Plan appendix includes execution_mode decision + U≥0.45 gate | PASS — `TestStrategicPlanAppendix_IncludesExecutionModeSemantics` |
| **L5-MUPS-TAG-03** | Execute hints include Required/Optional matrix | PASS — `TestWorkItemExecuteOutputHints_IncludesRequiredOptionalMatrix` |
| **L5-MUPS-TAG-04** | zh/en appendix SHA-256 golden stable | PASS — `TestMUPSSemanticAppendix_GoldenHash` |
| **L5-MUPS-TAG-05** | User frame fields annotated control/data | PASS — `TestRenderFrameFieldGuide_ObservePlan`, `TestBuildStrategicPlanUserPrompt_IncludesFrameGuide` |

## 3. T-Layer Evidence

| T ID | Status |
|------|--------|
| D2-S15-A97-T01..T04 | IMPLEMENTED |
| D7-S5-A97-T01..T02 | IMPLEMENTED |

## 4. Test Commands

```bash
go test ./internal/shared/prompttags/... -count=1          # PASS
go test ./internal/layers/contextengine/i18n/... -count=1  # PASS
go test ./internal/layers/contextengine/materialize/... -count=1  # PASS
go test ./internal/layers/orchestration/sessionorchestrator/... -count=1  # PASS
```

## 5. Token Budget Note

Semantic appendix adds fixed bullet blocks per phase (Observe ~350 tokens zh target, Plan ~450, Execute ~500 per design). No Materialize `TokenEst` gate regression observed in unit tests; staging A/B comparison deferred to post-merge monitoring.

## 6. 领域文档同步

| 文档 | 状态 |
|------|------|
| `openspec/specs/shared/prompttags.md` — v3 semantics + invariant #6 | SYNCED |
| `openspec/specs/d2-context-engine/t-registry.md` — D2-S15-A97 | SYNCED |
| `openspec/specs/d7-orchestration/t-registry.md` — D7-S5-A97 | SYNCED |
| `openspec/t-registry.md` — index counts | SYNCED |

## 7. Non-goals / Deferrals

- P2 parse reject → next-round user frame (unchanged defer)
- Verify/Learn/Decide implementation (out of scope)

## 8. Sign-off

All P0 tasks complete. Ready for S6 merge.
