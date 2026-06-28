# Proposal: D7 Layer SubContext Phase 3

**Change ID:** `devrix-d7-layer-subcontext-phase3`  
**Demand ID:** DM-20260628-002  
**Status:** S4_Development  
**Parent:** `openspec/archive/2026-06-28-devrix-d7-layer-subcontext/` (DM-20260627-003)

---

## Goal

Close Phase 3 gaps from Layer SubContext archive: unify delegate SubTurn context assembly with D2 Materialize, merge Wave ContextResolver, and (separately) add LLM ObservationProposer.

## Phase 3-T33 (this PR)

Map context-budget SubTurn modes to Materialize policy:

| SubTurn mode | Materialize mode | Behavior |
|--------------|------------------|----------|
| brief | fresh | No parent history |
| fork | fork | BuildForkedMessages prefix |
| full | resume | Parent history + optional agent sidechain |

`SubTurnRunner` uses `Materializer` when wired (bootstrap); legacy `applyMode` remains fallback.

## Phase 3-T34 (this PR)

Wave scheduler context policies delegate to D2 Materializer via `PartitionWave`:

| Wave policy | Materialize mode |
|-------------|------------------|
| fresh | fresh |
| resume | resume (+ agent sidechain) |
| upstream | upstream (+ artifact summary in system prompt) |

`NewMaterializingContextResolver` replaces raw `ContextResolver` in bootstrap when Materializer is wired. Legacy resolver remains fallback when Materializer is nil.

## Deferred

- **T35:** LLM ObservationProposer @ Observe (independent change per R1)
