# Design: MUPS parse reject feedback

**Change ID:** `mups-parse-reject-feedback`  
**Demand:** DM-20260705-002

## 1. ParseRejectRecord

**Path:** `internal/shared/prompttags/parse_reject.go`

```json
{"phase":"plan","code":"budget_cap","field":"children","message":"...","requested":5,"max_allowed":2}
```

Codes: `parse_fail`, `budget_cap`, `uncertainty_gate`, `scope_gate`, `validate_empty`.

## 2. Persistence

**Path:** `workmodel.WorkItemPipelineRound`

| Field | Consumer |
|-------|----------|
| `ObserveParseReject` | Next Observe `prior_parse_reject` |
| `PlanParseReject` | Next Plan `prior_parse_reject` |

`SpawnRationale` unchanged (human/Decide prose).

## 3. Capture points (D7)

| Event | Setter |
|-------|--------|
| Observe proposer/validate fail | `mergeProposedObservations` → `observeParseReject` |
| Plan JSON parse fail | `parseRejectFromPlanError` |
| StrategicPlanReject | `parseRejectFromStrategicPlan` |
| Scope gate reject | `RejectScopeGate` record |

## 4. Injection (D2)

- `TagPriorParseReject` after `directive` in Observe/Plan lineframes
- `RenderFrameFieldGuideForFields` + semantics i18n when-use (zh/en)
- Plane: **control**

## 5. Invariants

- No same-round LLM retry
- No D7 tactical prose in prompts
- Execute retry continues `machineSpawnFeedback` only (L5-MUPS-REJ-04)
