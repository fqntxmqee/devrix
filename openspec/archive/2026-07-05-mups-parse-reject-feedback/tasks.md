# Tasks: MUPS parse reject feedback

**Change ID:** `mups-parse-reject-feedback`  
**Demand:** DM-20260705-002

## P0

| Task | L4/L5 | Status |
|------|-------|--------|
| T1 `ParseRejectRecord` + tests | shared-A98 | [x] |
| T2 `TagPriorParseReject` + lineframes | shared-A98 | [x] |
| T3 Round fields `ObserveParseReject` / `PlanParseReject` | D7-S5-A98 | [x] |
| T4 Capture Observe reject in pipeline | D7-S5-A98 | [x] |
| T5 Capture Plan reject (parse/budget/scope) | D7-S5-A98 | [x] |
| T6 Inject into Observe/Plan user prompts | D2-S15-A98 | [x] |
| T7 i18n semantics zh/en | D2-S15-A98 | [x] |
| T8 Tests L5-MUPS-REJ-01..03 | — | [x] |

## P1

| Task | L4/L5 | Status |
|------|-------|--------|
| T9 Golden hash update (if appendix changes) | L5-MUPS-REJ-04 | [x] N/A (user frame only) |
| T10 t-registry D7-S5-A98 / D2-S15-A98 | — | [x] |

## Verification

```bash
go test ./internal/shared/prompttags/... -count=1
go test ./internal/layers/orchestration/sessionorchestrator/... -count=1
go test ./internal/layers/contextengine/i18n/... -count=1
```
