# Proposal: D7 Observational-Answer Fast-Return (DM-20260706-011)

**Change ID:** `devrix-d7-observational-fastpath`
**Demand ID:** DM-20260706-011
**Priority:** P0
**PR Count:** 0 (direct commits; S6 archive retroactive)
**Status:** S4_Implemented → S5_Accepted → S7_Archived (2026-07-07)

---

## 1. Background

D7 MUPS pipeline currently runs the full Observe → Plan → Execute → Verify → Learn
5-node sequence for **every** user directive. For trivial deterministic Q&A
("1+1=几?", "2×3=几?", "法国首都是哪?"), Observe already emits a high-confidence
CatBusiness ObsFact (e.g. `statement="1+1=2 在标准算术下成立"`, `strength=0.99`),
but the round still walks through Plan (proposing 1-3 children) + Execute
(ReAct loop with tool calls) + Verify (code-based or semantic verifier) before
the user sees the answer. That adds **2 redundant LLM calls + 3-5s latency**
for the most common query class.

DM-20260706-009 (Plan single-mode fast-path) already prevented force-decompose
when the observer was confident. This change is the **second half of the
two-stage fast-return**: when the Observe layer is confident AND the report
carries no open questions, skip Plan+Execute+Verify entirely and emit the
ObsFact.Statement as the user-visible finalText. **Learn still runs** so the
reputation system keeps observing the observer's accuracy.

## 2. Problem Statement

Today the trivial-Q&A round looks like:

```
User → Observe(LLM) → Plan(LLM) → Execute(Worker) → Verify(LLM) → Learn
                       ↑ redundant   ↑ redundant       ↑ redundant
                       for trivial Q&A (knowable at Observe time)
```

Total: 3 LLM calls + 1 Worker invocation + ~3-5s latency.

**Goal shape**: when Observe emits `CatBusiness ObsFact` with `strength ≥ 0.85`
(the LLM proposer's cap) and no LLM/scoped `ObsUncertainty`, emit
`ObsFact.Statement` directly as the finalText. Total: **1 LLM call + ~1s
latency**.

## 3. Goals

| Goal | Metric | Target |
|------|--------|--------|
| Trivial Q&A latency reduced | `User message` → `user-visible finalText` | ≤ 1.5s |
| LLM calls per trivial round | Counted via MuPS span attributes | 1 (down from 3) |
| Reputation scoring preserved | Learn still runs, α/β updated | 100% compatible |
| Quality preserved | ObsFact statement identical to post-Plan/Execute finalText | 100% for high-strength CatBusiness |
| CI green | 27/27 orchestration packages `go test -race` | PASS |

## 4. Non-Goals

- ❌ **Multi-intent decomposition** (DM-20260707-001 handles that for
  "1+1=几 + 巴黎时区" mixed directives). This change only handles single-intent
  deterministic Q&A.
- ❌ **LLM-bypass verification**. The ObsFact IS the answer — no code-based or
  semantic verifier runs. Reputation BayesianUpdate(VerdictPass) accumulates the
  observer's accuracy signal so misfires degrade next-round confidence.
- ❌ **Cross-round consistency**. Each fast-return round is independent; if the
  user repeats the directive, Observe re-runs and re-evaluates ObsFact strength.
- ❌ **Pre-emptive answer caching**. We don't memoise answers; we trust the
  observer's strength signal.
- ❌ **Configurable threshold knob**. The 0.85 threshold matches the LLM
  proposer's strength cap (see `format_hints_mups.go` suffix). Per-tenant tuning
  is future work.

## 5. Solution

### Gate Logic (all required, fail-safe to Plan+Execute)

| # | Gate | Source | Why |
|---|------|--------|-----|
| 1 | `!isRollup && !isDeliverableSynth && !isParentRollup` | ItemPipelineRunner | Rollup items must walk the rollup path; bypassing would break parent aggregation |
| 2 | `r.Learner != nil` | ItemPipelineRunner | Learn is the only reputation writer; no Learn → no observer signal → no fast-return |
| 3 | `!hasObsUncertainty(report)` | deliverable_execute.go | Any LLM/scoped question blocks fast-return. Mechanical "item_pipeline"/"verify_signal" rows (decomposition hints) are filtered out |
| 4 | `pickHighStrengthBusinessFact(report, 0.85)` returns non-empty Statement | deliverable_execute.go | CatBusiness ObsFact with strength ≥ 0.85 (LLM proposer cap) is the deterministic-Q&A signature |

### emit observ fast-path

On gate hit, `maybeObservationalAnswer(...)` constructs:

- `verdict = { Kind: VerdictPass, Confidence: obs.Strength, SourceID: "obs_fact:<id>" }`
- `artifact = { TaskID: item.ID, WorkerType: WorkerWorkItem, Summary: statement,
   Metadata: { source: "observational_answer_fastpath", obs_fact_id, ... } }`
- `round = { RoundNo, Trigger, ArtifactID: item.ID, ArtifactSummary: statement,
   VerdictKind: VerdictPass, ExitReason: "observational_answer", UncertaintyMean: 0 }`

Then:
1. `r.Learner.Learn(ctx, LearnRequest{Verdict, Plan: nil, Artifact, Observations})`
   — Plan intentionally nil (the Plan node was skipped).
2. `r.Tasks.Tree().ApplyPipelineRound(sessionID, item.ID, round, RoundPhaseIdle)`
   — phase = Idle so downstream SpawnPolicy doesn't re-trigger.
3. `opts.Emit(...)` — `complete` EngineEvent so Feishu card renders the answer.

Any gate miss → falls through to the legacy Plan+Execute path. No errors.

### Source-filtered uncertainty

`hasObsUncertainty(report)` ignores `Source == "item_pipeline"` and
`Source == "verify_signal"` rows because those are **mechanical** signals
(decomposition threshold hints + verify noise), not user-facing questions.
This is what makes "complex multi-intent" directives still hit the gate — the
unscoped ObsUncertainty from the observer is what triggers fall-through.

### i18n prompt update

`observationTaskAppendixZHSuffix` / `observationTaskAppendixENSuffix` in
`format_hints_mups.go` gained a one-line instruction telling the Observe LLM
that for deterministic Q&A the `statement` field is emitted directly, so the
LLM must write a complete answer (not a partial thought).

## 6. Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| ObsFact.statement is LLM-generated; could be wrong | Medium | 0.85 strength is high bar; BayesianUpdate records each fire; reputation β/(α+β) > 0.7 → "force_plan" Learn signal gates next round's fast-return |
| Skips Verify, errors reach user | Medium | Same β-driven Learn gate; User Accept/Cancel hooks (DM-20260625-003 PR-V5.6) still operate |
| Cross-round consistency | Low | Each round runs fresh Observe; stale ObsFact can't leak |
| i18n prompt change affects other scenarios | Low | Suffix-only change; semantic role preserved; ObsUncertainty/ObsDeviation/ObsSignal emission unchanged |

## 7. Acceptance Criteria

| AC | Description | Status |
|----|-------------|--------|
| AC1 | High-strength CatBusiness ObsFact + no ObsUncertainty → fast-path fires | PASS |
| AC2 | Any ObsUncertainty (LLM/scoped) blocks fast-path → falls to Plan | PASS |
| AC3 | CatSystem ObsFact blocks fast-path regardless of strength | PASS |
| AC4 | strength < 0.85 blocks fast-path | PASS |
| AC5 | Rollup / DeliverableSynth / ParentRollup blocks fast-path | PASS |
| AC6 | r.Learner == nil blocks fast-path | PASS |
| AC7 | `r.Learner.Learn` receives `VerdictPass` + `SourceID="obs_fact:<id>"` | PASS |
| AC8 | Round is persisted via `ApplyPipelineRound(..., RoundPhaseIdle)` | PASS |
| AC9 | `opts.Emit` fires `complete` EngineEvent with finalText = statement | PASS |
| AC10 | 27/27 orchestration packages `go test -race` PASS | PASS |
| AC11 | `go vet ./...` 0 warning | PASS |