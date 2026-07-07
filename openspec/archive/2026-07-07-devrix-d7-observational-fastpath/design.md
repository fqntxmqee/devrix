# Design: D7 Observational-Answer Fast-Return

## 1. Architecture Position

The fast-path is a **fork gate** at the top of `ItemPipelineRunner.Run()`:

```
Run()
 ├─ Observe  (always)
 ├─ maybeObservationalAnswer()   ← NEW
 │    ├─ Gates (all required)
 │    ├─ Build Verdict + Artifact
 │    ├─ r.Learner.Learn(...)    ← reputation scoring
 │    ├─ ApplyPipelineRound(...) ← persistence
 │    └─ opts.Emit(...)          ← user-visible card
 ├─ Plan     (only on gate miss)
 ├─ Execute  (only on gate miss)
 ├─ Verify   (only on gate miss)
 └─ Learn    (only on gate miss; redundant with maybeObservationalAnswer's Learn)
```

Architecturally: same `LearnRequest` shape, same `ApplyPipelineRound` persistence,
same `opts.Emit` surface. The only behavioural change is **which fields are
populated**:

- `Plan = nil` — no Plan produced
- `Artifact.WorkerType = WorkerWorkItem` (not a real Worker run)
- `Artifact.Metadata.source = "observational_answer_fastpath"` — provenance marker
- `Verdict.SourceID = "obs_fact:<id>"` — observer provenance (not Verify)
- `ExitReason = "observational_answer"` — distinguishable from `decision_*` paths

## 2. Gate Logic (file: `item_pipeline.go:285-298`)

```go
if !isRollup && !isDeliverableSynth && !isParentRollup && r.Learner != nil &&
    !hasObsUncertainty(report) {
    if obsID, factStmt, ok := pickHighStrengthBusinessFact(report, 0.85); ok {
        round, fpErr := r.maybeObservationalAnswer(...)
        if fpErr == nil && round != nil {
            return round, nil
        }
    }
}
```

`fpErr != nil` (e.g. tree mutation error) → log + fall through to Plan.
`round == nil` → no fast-path fired → fall through. Never an error to the user.

## 3. Uncertainty source filter (file: `deliverable_execute.go:198-209`)

`hasObsUncertainty` only blocks on ObsUncertainty rows whose `Source` is NOT
`"item_pipeline"` or `"verify_signal"`. The two filtered sources represent
**mechanical** signals:

- `item_pipeline` — emitted by `observationsFromItem` when item.Uncertainty
  crosses `DefaultUncertaintyDecomposeThreshold`. This is the planner's
  threshold, not a user-facing question.
- `verify_signal` — emitted by the verify node on signal-detection failures
  (not relevant pre-verify; defensive).

What blocks fast-path:
- LLM observation_proposer rows (`Source = "observe_proposer_*"`)
- Scope-contract rows (`Source = "scope_contract_*"`)
- Any user-explicit question

This is the **only** mechanism by which "complex multi-intent" directives
still walk Plan+Execute. The observer's unscoped ObsUncertainty is the gate
that makes this safe.

## 4. pickHighStrengthBusinessFact (file: `deliverable_execute.go:219-247`)

Returns `(obsID, statement, ok)`:

```go
for _, o := range report.Observations {
    if o.Kind == ObsFact && o.Category == CatBusiness &&
       o.Strength >= threshold {
        if fp, ok := o.Payload.(FactPayload); ok && strings.TrimSpace(fp.Statement) != "" {
            return o.ID, fp.Statement, true
        }
    }
}
return "", "", false
```

Order: report iteration order (which is proposal order). Tie-break: first match
wins. CatSystem ObsFact is excluded by category check; the validator at
`format_hints_mups.go` ensures proposers can't sneak through.

## 5. maybeObservationalAnswer (file: `item_pipeline.go:1231-1345`)

Sequence:

1. `hardening.EmitMUPSFastPath(ctx, ...)` — observability span
2. Build `Verdict{Kind: Pass, Confidence: obs.Strength, SourceID: "obs_fact:<id>"}`
3. Build `Artifact{TaskID: item.ID, WorkerType: WorkerWorkItem, Summary: statement,
   Metadata: { source: "observational_answer_fastpath", obs_fact_id, learn_hint,
   stop_reason: "observational_answer", iterations: 0, tool_calls: 0,
   verdict_kind, verdict_source }}`
4. Build `round` with `ExitReason = "observational_answer"` and
   `UncertaintyMean = 0` (no work performed → no uncertainty)
5. `r.Learner.Learn(ctx, LearnRequest{Verdict, Plan: nil, Artifact, Observations})`
   — non-fatal on error (slog.Warn, fall through to return round anyway)
6. `r.Tasks.Tree().ApplyPipelineRound(sessionID, item.ID, round, RoundPhaseIdle)`
   — phase = Idle gates downstream SpawnPolicy
7. Caller emits `complete` EngineEvent with `Content = statement` so Feishu card
   renders the answer

## 6. i18n prompt update (file: `format_hints_mups.go`)

Added 1 line to both ZH and EN suffix:

```
- 对于确定性问答(数学、常识、定义),把完整答案放在 statement 字段,strength ≥ 0.9;
  不要再返回 obs_uncertainty 追问(plan 节点会跳过 execute 直接 emit statement)。
```

Why this matters: without the instruction, the Observe LLM might emit a
**partial** statement ("1+1") and assume Plan/Execute will fill in the rest.
With the instruction, the LLM writes a **complete** answer ("1+1=2 (二)").

The 0.9 in the instruction text is the proposer's cap; the gate uses 0.85 to
give a small safety margin (so the LLM doesn't have to hit exactly 0.9 to
qualify). Golden hash regenerated in commit `a61c1e58`.

## 7. Test surface (file: `item_pipeline_fastpath_test.go`, 394 lines)

| Test | What it pins |
|------|--------------|
| `TestObservationalAnswerFastPath_TriggersOnHighStrengthFact` | Gate fires end-to-end; round + artifact persisted |
| `TestObservationalAnswerFastPath_SkippedWhenUncertaintyExists` | LLM ObsUncertainty blocks |
| `TestObservationalAnswerFastPath_SkippedForLowStrengthFact` | strength=0.5 blocks |
| `TestObservationalAnswerFastPath_SkippedForSystemCategory` | CatSystem blocks |
| `TestObservationalAnswerFastPath_LearnerReceivesVerdict` | r.Learner receives correct Verdict |
| `TestObservationalAnswerFastPath_PersistsArtifactMetadata` | artifact ID = item.ID, Metadata source marker present |
| `TestObservationalAnswerFastPath_SkippedForRollupItems` | isRollup/isDeliverableSynth/isParentRollup block |
| `TestObservationalAnswerFastPath_RoundIsCallerReady` | no events emitted by runner; caller must Emit |
| `TestObservationalAnswerFastPath_LearnerSourceIDIncludesObsID` | Verdict.SourceID = "obs_fact:<id>" |

Test fixtures (`fastPathObsFact`, `fastPathObsUncertainty`, `fastPathObsSystemFact`)
provide typed constructors so tests don't hand-craft Observation structs.

## 8. Failure modes

| Failure | Behaviour |
|---------|-----------|
| `r.Learner == nil` | Gate 2 fails → fall through to Plan |
| `r.Tasks.Tree().ApplyPipelineRound` returns error | `fpErr != nil` → slog.Warn + fall through to Plan |
| `r.Learner.Learn` returns error | Non-fatal: slog.Warn + continue (round still persisted, user still gets answer) |
| ObsFact.Statement empty (whitespace only) | pickHighStrengthBusinessFact returns ok=false → fall through |

In all cases: user experience degrades to "slightly slower (Plan path)" rather
than "error" or "silent failure".

## 9. Relationship to PR-E (force_plan)

The `force_plan` mechanism (PR-E, T63) reads `LearnResponse.ForcePlanFlag` set
by `BayesianUpdate` when `β/(α+β) > 0.7`. This change provides the reputation
**writes** that PR-E's `force_plan` reads. Without observational fast-path
running, the observer's accuracy signal wouldn't accumulate; with it, the
reputation system has enough volume to make `force_plan` decisions meaningful
in production (today most trivial Q&A rounds don't even reach Learn).

## 10. Verification

- 9 new unit tests PASS
- 27/27 orchestration packages `go test -race` PASS
- `go vet ./...` 0 warning
- Manual: trivial Q&A round-trip latency 3-5s → ~1s (50-70% reduction)
- Manual: complex multi-intent directive falls through to Plan as expected
- Jaeger: fast-path trace has Observe + Learn spans only (no Plan / Execute / Verify spans)
- Reputation: β drift visible in dashboard after 100+ fast-path rounds