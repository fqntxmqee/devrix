# Acceptance Report: D7 S 层归一化

**Demand ID:** DM-20260701-002  
**Change ID:** `devrix-d7-s-layer-normalization`  
**Date:** 2026-07-01  
**Verdict:** ACCEPTED

---

## Summary

D7 current canonical S 层已收敛为 S1-S6。历史 MUPS S7-S14 与 TaskContract S20/S21 不再作为 current S 展示，而是归入 historical / contract mapping。WorkTree+MUPS 下行传播与向上反馈关系已在 spec / registry 中明确，同时补齐了 StrategicPlanReject 回灌和 child-stats uncertainty reconcile。

## L5 / T Verification

| T ID | Result | Evidence |
|------|--------|----------|
| D7-SN-T01 | PASS | OpenSpec change 包完整 |
| D7-SN-T02 | PASS | `TestD7MainPath_CanonicalSLayerNormalized` |
| D7-SN-T03 | PASS | `TestD7MainPath_CanonicalSLayerNormalized` |
| D7-SN-T04 | PASS | `TestD7MainPath_RetiredIngressFilesAbsent` |
| D7-SN-T05 | PASS | `TestRunItemPipeline_StrategicPlanRejectFeedsNextPrompt` |
| D7-SN-T06 | PASS | `TestReconcileUncertaintyFromChildStats_AllPassDropsStoredParent` |

## Quality Gate

- [x] `go test ./internal/layers/orchestration/sessionorchestrator ./internal/layers/orchestration/workmodel -count=1`
- [x] ReadLints: no linter errors on edited Go files

## Domain Docs Sync

- [x] `openspec/specs/d7-orchestration/spec.md`
- [x] `openspec/specs/d7-orchestration/a-registry.md`
- [x] `openspec/specs/d7-orchestration/f-registry.md`
- [x] `openspec/specs/d7-orchestration/t-registry.md`
- [x] `openspec/specs/d7-orchestration/CHANGELOG.md`
- [x] `openspec/specs/architecture/layering.md`
- [x] `openspec/specs/architecture/code-layout.md`

## Residual Risk

历史段仍保留旧 ID 用于追溯，未做全量 T ID 重编号。后续如执行物理目录迁移，应另建 demand。
