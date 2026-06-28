# Spec Delta — D7-Orchestration — Layer SubContext Phase 3

**Change ID:** `devrix-d7-layer-subcontext-phase3`  
**Target Spec:** `openspec/specs/d7-orchestration/spec.md`  
**Target Version:** v4.14.0 → v4.15.0  
**Demand ID:** DM-20260628-002  
**Parent:** DM-20260627-003 (Phase 1+2)  
**Created:** 2026-06-28  
**Status:** S7_Archived

---

## ADDED Requirements

### D7-S16-A65: SubTurn → MaterializePolicy

Delegate SubTurn modes shall map to D2 Materialize policies:

| SubTurn mode | Materialize mode |
|--------------|------------------|
| brief | fresh |
| fork | fork |
| full | resume |

When `SubTurnRunner.Materializer` is wired, context assembly uses `PartitionAgent`; nil Materializer falls back to legacy `applyMode`.

#### Scenario: brief sub-agent gets fresh partition

- **Given** SubTurn invoked with mode `brief`
- **When** Materializer is wired on SubTurnRunner
- **Then** Materialize policy shall be `fresh`
- **And** parent ReAct history shall not appear in sub-agent messages

---

### D7-S16-A66: Wave ContextResolver → Materializer

Wave worker context policies shall delegate to D2 Materialize via `PartitionWave`:

| Wave policy | Materialize mode |
|-------------|------------------|
| fresh | fresh |
| resume | resume |
| upstream | upstream |

Bootstrap shall prefer `NewMaterializingContextResolver` when Materializer is available.

#### Scenario: wave resume loads agent sidechain

- **Given** Wave worker with policy `resume` and existing agent sidechain
- **When** MaterializingContextResolver resolves context
- **Then** Materialize shall use `PartitionWave` with resume mode
- **And** agent sidechain messages may appear in worker payload

---

### D7-S16-A74: LLM ObservationProposer @ Observe

Optional `ObservationProposer` may propose Obs* entries from structured signals (directive, ScopeContract, inbound signal lines, prior observations) **without** WorkItem private ReAct transcript.

`ValidateObservationProposals` shall rule-gate proposals:
- ObsFact strength ≤ 0.85
- Mandatory evidence on each proposal

LLM failures shall be fail-safe: rules-only Observe continues without blocking the pipeline.

#### Scenario: over-strength ObsFact rejected

- **Given** LLM proposes ObsFact with strength 0.95
- **When** ValidateObservationProposals runs
- **Then** the proposal shall be rejected
- **And** rules-only Observe output may still proceed
