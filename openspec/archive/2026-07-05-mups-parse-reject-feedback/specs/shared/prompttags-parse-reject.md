# Delta: shared/prompttags parse reject feedback

**Change ID:** `mups-parse-reject-feedback`  
**Demand:** DM-20260705-002

## ADDED

**Path:** `internal/shared/prompttags/parse_reject.go`

- `ParseRejectRecord` — compact JSON for cross-round LLM feedback
- `TagPriorParseReject` — lineframe field on Observe/Plan user frames (control plane)

## MODIFIED lineframes

| Frame | Field order change |
|-------|-------------------|
| `ObserveUserFrame` | `prior_parse_reject` after `directive` |
| `PlanUserFrame` | `prior_parse_reject` after `directive` |

## MODIFIED round persistence

`WorkItemPipelineRound.ObserveParseReject` / `PlanParseReject` feed next-round user frames; `SpawnRationale` unchanged.

## Invariant 7 (new)

**Parse-reject round-trip** — Observe/Plan reject records persisted on round N appear in matching phase user frame on round N+1.
