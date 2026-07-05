# Proposal: MUPS parse reject feedback

**Change ID:** `mups-parse-reject-feedback`  
**Demand:** DM-20260705-002

## Why

Tag semantics (DM-20260705-001) tell the LLM **how** to format output; parse/budget rejects still do not round-trip into Observe/Plan **user frames**, so models repeat JSON/budget mistakes across rounds.

## What

| Capability | Description |
|------------|-------------|
| **shared-A98** | `ParseRejectRecord` compact JSON + `prior_parse_reject` lineframe field |
| **D7-S5-A98** | Capture Observe/Plan rejects on `WorkItemPipelineRound` |
| **D2-S15-A98** | Inject reject into next-round user frames + i18n semantics |

## Scope

- Observe wholebody parse/validate fail → `ObserveParseReject` → next Observe frame
- Plan JSON parse / StrategicPlanReject / scope gate → `PlanParseReject` → next Plan frame
- Execute unchanged (PriorVerifyReason path)

## Out of scope

- Same-round LLM format-hint retry
- SpawnPolicy changes
