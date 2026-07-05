# Acceptance Report: DM-20260705-002

**Change ID:** `mups-parse-reject-feedback`  
**Verdict:** ACCEPTED (pending merge)

## L5 Results

| ID | Given-When-Then | Result |
|----|-----------------|--------|
| L5-MUPS-REJ-01 | Observe parse fail → next Observe frame has `prior_parse_reject` | PASS |
| L5-MUPS-REJ-02 | Plan parse fail → next Plan frame has reject | PASS (via parseRejectFromPlanError unit path) |
| L5-MUPS-REJ-03 | StrategicPlanReject → next Plan frame | PASS |
| L5-MUPS-REJ-04 | Execute retry unchanged | PASS (no Plan frame inject on Execute path) |

## Tests

```bash
go test ./internal/shared/prompttags/... ./internal/layers/orchestration/sessionorchestrator/... ./internal/layers/contextengine/i18n/... -count=1
# PASS
```

## Domain doc sync (pre-merge)

- [ ] `openspec/specs/shared/prompttags.md` — invariant #7 + lineframe fields
- [ ] `openspec/t-registry.md` — D7-S5-A98, D2-S15-A98
