# Spec Delta — D2-Context-Engine — Layer SubContext Phase 3

**Change ID:** `devrix-d7-layer-subcontext-phase3`  
**Target Spec:** `openspec/specs/d2-context-engine/spec.md`  
**Target Version:** v8.1.0 → v8.2.0  
**Demand ID:** DM-20260628-002  
**Created:** 2026-06-28  
**Status:** S7_Archived

---

## MODIFIED Requirements

### D2-S16-A20 ContextMaterializer.Materialize

Extended partition support: `PartitionAgent` (SubTurn) and `PartitionWave` (Wave scheduler) in addition to `PartitionWorkItem`.

---

## ADDED Requirements

### D2-S16-A22: SubTurn and Wave Materialize Paths

`PolicyFromSubTurnMode` maps SubTurn `brief`/`fork`/`full` to Materialize `fresh`/`fork`/`resume`.

`PolicyFromWaveContext` maps Wave `fresh`/`resume`/`upstream` to corresponding Materialize modes.

`ComposeSubTurnMessages` preserves brief/fork/full semantics aligned with context-budget Phase B.

Sub-agent sidechains at `subagents/<agent_id>.jsonl` support SubTurn resume and Wave resume policies.

#### Scenario: wave upstream injects artifact summary only

- **Given** Wave worker with policy `upstream` and upstream artifact summary
- **When** Materialize runs for `PartitionWave`
- **Then** system prompt may include truncated artifact summary
- **And** upstream private jsonl full text shall not appear

**D7 boundary:** SubTurn and Wave workers use Materialize partitions without Leader transcript inheritance.
