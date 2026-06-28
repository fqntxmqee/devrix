# Tasks: D7 Layer SubContext Phase 3

**Change ID:** `devrix-d7-layer-subcontext-phase3`  
**Demand ID:** DM-20260628-002  
**Parent:** DM-20260627-003 (Phase 1+2 archived)  
**Status:** S4_Development

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

## Phase 3-T35 — LLM ObservationProposer (P1, deferred)

| ID | Description | Status |
|----|-------------|--------|
| T35 | Observe LLM proposer + rule validation (D7-S8 PR-A4) | [ ] |
