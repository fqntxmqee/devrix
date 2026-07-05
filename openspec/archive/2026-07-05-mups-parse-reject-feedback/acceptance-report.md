# Acceptance Report: DM-20260705-002

**Change ID:** `mups-parse-reject-feedback`  
**Verdict:** ACCEPTED

## L5 Results

| ID | Given-When-Then | Result |
|----|-----------------|--------|
| L5-MUPS-REJ-01 | Observe parse fail → next Observe frame has `prior_parse_reject` | PASS |
| L5-MUPS-REJ-02 | Plan parse fail → next Plan frame has reject | PASS |
| L5-MUPS-REJ-03 | StrategicPlanReject → next Plan frame | PASS |
| L5-MUPS-REJ-04 | Execute retry unchanged | PASS |

## Tests

```bash
go test ./internal/shared/prompttags/... ./internal/layers/orchestration/sessionorchestrator/... ./internal/layers/contextengine/i18n/... -count=1
# PASS
```

## Domain doc sync

- [x] `openspec/specs/shared/prompttags.md` — v4 parse reject + invariant #5
- [x] `openspec/t-registry.md` + D2/D7 domain registries — A98 T points
