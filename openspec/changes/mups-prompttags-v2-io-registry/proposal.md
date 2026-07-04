# Proposal: MUPS prompttags v2 IO registry

**Change ID:** `mups-prompttags-v2-io-registry`  
**Demand ID:** DM-20260704-005  
**Created:** 2026-07-04  
**Status:** S4_Implementation  
**Demand:** [`demand.md`](demand.md)

---

## 1. Problem Statement

After DM-20260704-004, MUPS I/O shapes are implemented but not centrally cataloged. The Observe appendix says "maximum 3 proposals" but Go accepts unlimited valid proposals. Plan prompts omit `uncertainty_mean` despite `PlanUserFrame` field order reserving it. Multi-round Observe can duplicate prior observations.

## 2. Proposed Solution

| Component | Responsibility |
|-----------|----------------|
| `MUPSIOCatalog` | Single index of envelope tags, line frames, whole-body shapes |
| `LineFrameRegistry` | Register `ObserveUserFrame` / `PlanUserFrame` with `EncodingLineFrame` profile |
| `maxObservationProposals = 3` | Cap in `ValidateObservationProposals` (first 3 valid) |
| Plan prompt fix | Inject `uncertainty_mean` when `StrategicPlanInput.UncertaintyMean > 0` |
| Observe incremental | `prior_observation_ids` + `incremental_only: true` when `LastRound` has obs IDs |

## 3. Capabilities

| Capability | Layer | Package |
|------------|-------|---------|
| IO registry catalog | L4 shared | `internal/shared/prompttags` |
| Observe proposal cap | L4 D7 | `sessionorchestrator/observation_proposer.go` |
| Plan uncertainty inject | L4 D7 | `sessionorchestrator/strategic_plan_proposer.go` |
| Observe incremental frame | L4 D7 | `observation_proposer.go` + `llm_observation_proposer.go` |

## 4. Non-goals

- Reject feedback loops (P2)
- Changes to Execute envelope tag set
- i18n tactical prose migration
