# Tasks: D7 Layer SubContext Phase 3

**Change ID:** `devrix-d7-layer-subcontext-phase3`  
**Demand ID:** DM-20260628-002  
**Parent:** DM-20260627-003 (Phase 1+2 archived)  
**Status:** S7_Archived

---

## Phase 3-T33 — SubTurn → MaterializePolicy (P0)

| ID | Description | Status |
|----|-------------|--------|
| T33a | `PolicyFromSubTurnMode` brief/fork/full → fresh/fork/resume | [x] |
| T33b | `ComposeSubTurnMessages` + agent partition Materialize path | [x] |
| T33c | `SubTurnRunner.Materializer` optional wiring + bootstrap | [x] |
| T33d | Unit + integration tests | [x] |

## Phase 3-T34 — Wave ContextResolver merge (P1)

| ID | Description | Status |
|----|-------------|--------|
| T34a | Wave fresh/upstream/resume → Materialize `PartitionWave` | [x] |
| T34b | `NewMaterializingContextResolver` + bootstrap wire | [x] |
| T34c | Tests | [x] |

## Phase 3-T35 — LLM ObservationProposer (P1)

| ID | Description | Status |
|----|-------------|--------|
| T35a | `ObservationProposer` interface + `ValidateObservationProposals` rule gate | [x] |
| T35b | `LLMObservationProposer` (structured signals only, no wi ReAct) | [x] |
| T35c | ItemPipeline + bootstrap wiring | [x] |
| T35d | Tests | [x] |

---

## Archive Footer

**Archived:** 2026-06-28  
**Outcome:** All Phase 3 tasks complete. Layer SubContext Phase 1–3 closed.
