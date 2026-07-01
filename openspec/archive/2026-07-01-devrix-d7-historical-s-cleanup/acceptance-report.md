# Acceptance Report: D7 Historical S Cleanup

**Demand ID:** DM-20260701-003  
**Change ID:** devrix-d7-historical-s-cleanup  
**Verdict:** ACCEPTED  
**Date:** 2026-07-01

## Summary

Moved former D7-S7–S21 detailed A/F registration out of current registries into `historical-s-mapping.md`, clarified D7-S3 WaveScheduler as an explicit wave/background path (not user-message main ingress), and extended architecture guard tests.

## L5 / T Verification

| T ID | Priority | Result | Evidence |
|------|----------|--------|----------|
| D7-HC-T01 | P0 | PASS | change package complete |
| D7-HC-T02 | P0 | PASS | `historical-s-mapping.md` + spec pointer |
| D7-HC-T03 | P0 | PASS | a-registry trimmed; guard test |
| D7-HC-T04 | P1 | PASS | f-registry trimmed; no fastpath |
| D7-HC-T05 | P1 | PASS | spec S3 explicit wave wording |
| D7-HC-T06 | P1 | PASS | Architecture diagram updated |

## Quality Gate

```
go test ./internal/layers/orchestration/sessionorchestrator/ -run TestD7MainPath -count=1
→ PASS
```

## Domain Docs Sync

| Document | Action |
|----------|--------|
| `openspec/specs/d7-orchestration/spec.md` | MODIFIED — v4.22.0, S3 boundary, Architecture |
| `openspec/specs/d7-orchestration/a-registry.md` | MODIFIED — v5.3.0, trimmed |
| `openspec/specs/d7-orchestration/f-registry.md` | MODIFIED — v5.3.0, trimmed |
| `openspec/specs/d7-orchestration/historical-s-mapping.md` | ADDED |
| `openspec/specs/d7-orchestration/t-registry.md` | MODIFIED — D7-HC T section |
| `internal/.../main_path_arch_test.go` | MODIFIED — extended guards |

## Residual Risks

- T-registry historical sections still reference D7-S8–S14 IDs inline (by design; T trace keys immutable).
- `historical-s-mapping.md` code paths may lag runtime; current path correction table in f-registry remains authoritative for runtime.
