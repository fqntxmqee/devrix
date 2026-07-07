# Spec Delta: D7 Observational-Answer Fast-Return

**Domain**: D7 (Orchestration)
**Feature**: observational-fastpath
**Status**: S5_Accepted (2026-07-07)
**Versions**: d7-orchestration v4.27.0 → v4.28.0
**Change ID**: devrix-d7-observational-fastpath
**Demand ID**: DM-20260706-011

## Feature Statement

When the D7 Observe node emits a high-strength CatBusiness ObsFact
(`strength ≥ 0.85`) AND the report carries no LLM/scoped ObsUncertainty, the
ItemPipelineRunner **skips** Plan + Execute + Verify and emits the ObsFact's
Statement directly as the user-visible finalText. The Learn node still runs so
reputation scoring keeps observing the observer's accuracy.

## Scenario: trivial deterministic Q&A (happy path)

```
Given: User sends "1+1=几?"
When: Observe emits {Kind: ObsFact, Category: CatBusiness, Strength: 0.95,
       Payload: {Statement: "1+1=2 (二)"}}
  And: Report contains no ObsUncertainty rows with Source != "item_pipeline" && Source != "verify_signal"
Then: ItemPipelineRunner.Run() returns round without invoking Plan/Execute/Verify
  And: opts.Emit fires complete EngineEvent with Content = "1+1=2 (二)"
  And: r.Learner.Learn runs with Verdict{Kind: Pass, SourceID: "obs_fact:<id>"}
  And: Round is persisted with Phase = Idle, ExitReason = "observational_answer"
```

## Scenario: complex multi-intent (fall-through)

```
Given: User sends "1+1=几 + 查 devrix 项目结构"
When: Observe emits 2 CatBusiness ObsFacts
  And: Observe emits 1 ObsUncertainty {Source: "observe_proposer_llm",
       Strength: 0.7, Question: "项目结构查询范围?"}
Then: hasObsUncertainty returns true
  And: Fast-path gate fails
  And: Plan + Execute + Verify + Learn run as normal (multi-intent handled by
       DM-20260707-001 separately)
```

## Scenario: low-strength ObsFact (gate miss)

```
Given: User sends obscure question
When: Observe emits CatBusiness ObsFact with Strength = 0.5
Then: pickHighStrengthBusinessFact(0.85) returns ok=false
  And: Fast-path gate fails
  And: Plan + Execute + Verify + Learn run as normal
```

## Scenario: CatSystem ObsFact (gate miss)

```
Given: User sends "清空回收站" (system-side action)
When: Observe emits CatSystem ObsFact with Strength = 0.99
Then: pickHighStrengthBusinessFact excludes CatSystem
  And: Fast-path gate fails (system actions need Plan)
```

## Scenario: rollup item (gate miss)

```
Given: Parent has multiple completed child WorkItems
When: Rollup WorkItem fires
  And: isRollup = true
Then: Fast-path gate fails (Gate 1)
  And: Rollup synthesis path runs
```

## Gate Conditions (4 gates, all required)

| # | Condition | Rationale |
|---|-----------|-----------|
| 1 | `!isRollup && !isDeliverableSynth && !isParentRollup` | Rollup items must walk rollup path |
| 2 | `r.Learner != nil` | Learn is the only reputation writer; no Learn → no observer signal |
| 3 | `!hasObsUncertainty(report)` | Any LLM/scoped question blocks fast-return |
| 4 | `pickHighStrengthBusinessFact(report, 0.85)` non-empty | CatBusiness + ≥ 0.85 + non-empty Statement |

Any gate miss → fall through to Plan+Execute+Verify path with no error.

## Source-filter semantics (hasObsUncertainty)

Excluded sources (treated as non-blocking):
- `item_pipeline` — mechanical decomposition hint from `observationsFromItem`
- `verify_signal` — verify-node noise (defensive; not relevant pre-verify)

Blocking sources:
- `observe_proposer_*` — LLM-raised user-facing questions
- `scope_contract_*` — scope-contract discrepancies
- Any user-explicit ObsUncertainty

## Verdict Provenance

On fast-path:
- `Verdict.SourceID = "obs_fact:<obs_id>"` — distinguishes from Verify-emitted verdicts
- `Artifact.Metadata.source = "observational_answer_fastpath"`
- `Round.ExitReason = "observational_answer"`
- `Round.Trigger = <inherited from Run() context>` (initial | inline | rollup)

These markers let dashboards distinguish fast-path rounds from regular rounds
without parsing the round's full event log.

## Failure modes (non-fatal)

| Failure | Behaviour |
|---------|-----------|
| `r.Learner == nil` | Gate 2 fails → fall through to Plan |
| `ApplyPipelineRound` error | `fpErr != nil` → slog.Warn + fall through |
| `r.Learner.Learn` error | Non-fatal: slog.Warn + continue |
| ObsFact.Statement empty | pickHighStrengthBusinessFact returns false → fall through |

In all cases: user experience degrades to "slightly slower (Plan path)".

## Relationship to Other Changes

- **Upstream**: DM-20260706-009 (Plan single-mode fast-path on high-strength fact) — Plan no longer force-decomposes when observer is confident
- **Downstream**: DM-20260707-001 PR-E T63 (force_plan) — Learn BayesianUpdate β/(α+β) > 0.7 → ForcePlanFlag; fast-path provides the reputation writes that make this meaningful
- **Adjacent**: DM-20260707-001 PR-D (Decision Node) — runs only when gate miss; no interaction on fast-path
- **Predecessor**: DM-20260706-010 (strip thinking from result.Content) — prerequisite so the emitted statement is clean

## Test Surface

- `internal/layers/orchestration/sessionorchestrator/item_pipeline_fastpath_test.go` — 9 tests, 394 lines
- All 9 tests use typed `fastPathObsFact` / `fastPathObsUncertainty` / `fastPathObsSystemFact` fixtures
- `gateCountingExecutor` records Execute calls so tests can assert fast-path bypassed Execute

## Domain Doc Sync

| File | Status |
|------|--------|
| d7 spec.md | v4.27.0 → v4.28.0 |
| d7 CHANGELOG.md | 顶部条目 |
| d7 t-registry.md | D7-S5-A118 + D7-S9-A119 段 9 T-points |
| 根 t-registry.md | D7 段引用 |