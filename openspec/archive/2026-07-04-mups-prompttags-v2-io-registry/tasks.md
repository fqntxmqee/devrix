# Tasks: mups-prompttags-v2-io-registry

**Demand ID:** DM-20260704-005  
**Change ID:** `mups-prompttags-v2-io-registry`

---

## P0

| Task | L4/L5 | Status |
|------|-------|--------|
| T1 OpenSpec change package (demand/proposal/design/specs/tasks) | — | [x] |
| T2 Extend `registry.go` — `EncodingLineFrame`, `LineFrameRegistry`, `MUPSIOCatalog`, `WholeBodyRegistry` | D2-S15-A96 | [x] |
| T3 `ValidateObservationProposals` max-3 cap + test | D7-S16-A96-T01 | [x] |
| T4 Delta spec `specs/shared/prompttags-io-registry.md` | — | [x] |
| T5 Register T points D2-S15-A96 + D7-S16-A96 | L5 | [x] |
| T6 `go test ./internal/shared/prompttags/... ./internal/layers/orchestration/sessionorchestrator/...` | L5 | [x] |

## P1

| Task | L4/L5 | Status |
|------|-------|--------|
| T7 `buildStrategicPlanUserPrompt` inject `uncertainty_mean` + test | D7-S16-A96-T02 | [x] |
| T8 Observe incremental frame (`prior_observation_ids`, `incremental_only`) + wire LastRound | D7-S16-A96-T03 | [x] |

## P2 (defer)

| Task | Status |
|------|--------|
| Reject feedback loops | [ ] |
