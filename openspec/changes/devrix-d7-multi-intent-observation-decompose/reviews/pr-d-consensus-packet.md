# PR-D Consensus Packet — Stage-5 Decision Node + DAGExecutor routing flag + e2e LP-3/LP-4

**Date:** 2026-07-07
**PR:** PR-D (DM-20260707-001, 7-PR split, step 5/7)
**Author:** Claude (Sonnet 4.6)
**Reviewers:** codex (MiniMax-M3), cursor-agent
**Predecessors:** PR-A1 #451, PR-A2 #452, PR-B #453 (consensus 100% adopted), PR-B fixes #454, PR-C #455 MERGED (consensus 100% adopted)

---

## 1. PROBLEM

PR-A1/A2/B/C shipped the **runtime** for multi-intent decompose:
- **PR-A1**: IntentSegment + PlanDAG grammar + validateDAG.
- **PR-A2**: SegmenterDispatcher producing IntentSegmentSet at Observe time + AC contract + LLM IO.
- **PR-B**: DAGExecutor runtime (`plan.PlanDAG → wavescheduler.TaskGraph`) with `<-chan SegmentEmit` + audit + artifact-driven emit + re-drain race defense.
- **PR-C**: ItemPipelineRunner.Run() DAG fork + per-segment Learn + Feishu streaming + emit-dedup.

PR-B only shipped the **DAGExecutor adapter** (T18-T21). The **Decision Node**
(T46-T49) was deferred to PR-D per the S3-Gate review consensus.

Now **PR-D** ships the **Stage-5 Decision Node + config gating + e2e**:
1. **Decision Node** (T46-T49) — pure rule engine, 0 LLM, 11-row static mapping table.
2. **Decision persistence** (T51) — 4 fields on WorkItemPipelineRound.
3. **Decision unit tests** (T50) — 18+ sub-cases covering all 11 rows + edge cases.
4. **Config flag** (T29) — `devrix.d7.dag_executor.enabled` (default false, 5%→100% gray rollout).
5. **E2E tests** (T52 + T30) — LP-3 multi-intent + LP-4 decision-pending.

PR-D is the **gating PR** — it makes the existing DAG fork safe to ship
behind a flag, adds the missing Stage-5 Decision node, and proves the
end-to-end LP-3/LP-4 contracts work via E2E tests.

---

## 2. CONSTRAINTS

### 2.1 Hard constraints
1. **Decision Node is a PURE RULE ENGINE** — `NewStaticDecisionNode().Decide(ctx)` is a pure function, no LLM, no I/O, no time.Now(). Latency target: < 5ms.
2. **11-row static mapping table is single source of truth** — any routing decision must trace back to one of the 11 rows in `decision_node.go`. No ad-hoc DecisionNode implementations allowed.
3. **Decision persistence is additive** — 4 new optional fields on `WorkItemPipelineRound`. Legacy pre-PR-D rounds without these fields remain readable (zero-value DecisionKind string omitempty).
4. **Config flag default OFF** — `devrix.d7.dag_executor.enabled = false` is the production default. PR-D code paths must produce 0 behavioral change when the flag is off.
5. **DAGExecutor != nil check is independent gate** — flipping `DAGEnabled=true` without wiring `DAGExecutor` must NOT crash. The legacy single-WorkItem path runs.
6. **Backward compat** — `DecisionNode` runs on EVERY round (regardless of DAG fork). Decision node is wired into the Verify → Decision transition, so single-WorkItem path also benefits from Decision persistence.
7. **Test runner helper is reused** — E2E tests use `newItemPipelineTestRunner` from `item_pipeline_test.go`. No new test helpers; PR-D tests follow existing patterns.
8. **No new sentinel errors** — PR-D errors fall through to existing `map_miss_fallback` accept + warn log. New sentinels only if PR-E/PR-F demand them.

### 2.2 Soft constraints
1. Decision Node file ≤ 500 LOC (excluding tests).
2. Wire file (`decision_node_wire.go`) ≤ 250 LOC.
3. Coverage ≥ 80% on the new Decision Node code path.
4. All DecisionKind strings are wire-format (snake_case: accept / retry / child_worker / parent_rollup / human_review) — D5 dashboards grep on these.
5. `exitReasonForDecision` only rewrites base when path diverges (Accept leaves base unchanged).

---

## 3. SCOPE

### 3.1 NEW: `orchtypes/config.go` — DAGExecutorConfig (T29)

```go
type DAGExecutorConfig struct {
    Enabled            bool  // default false; production flag
    MaxFanOut          int   // default 8
    MaxRetryOnPartialFail int // default 1
}

func DefaultDAGExecutorConfig() DAGExecutorConfig {
    return DAGExecutorConfig{Enabled: false, MaxFanOut: 8, MaxRetryOnPartialFail: 1}
}

func BuildDAGExecutorConfig(file *DAGExecutorFileConfig) DAGExecutorConfig {
    if file == nil { return DefaultDAGExecutorConfig() }
    base := DefaultDAGExecutorConfig()
    if file.Enabled != nil { base.Enabled = *file.Enabled }
    if file.MaxFanOut > 0    { base.MaxFanOut = file.MaxFanOut }
    if file.MaxRetryOnPartialFail > 0 { base.MaxRetryOnPartialFail = file.MaxRetryOnPartialFail }
    return base
}
```

`Config.DAGExecutor` field added; `FileConfig.DAGExecutor *DAGExecutorFileConfig` (yaml: `dag_executor`) added.

**Tests**: 5 unit tests in `orchtypes/dag_executor_config_test.go` covering default-off / nil-file / explicit-flip / partial-override / sub-config-wiring.

### 3.2 NEW: `sessionorchestrator/decision_node.go` (T46-T48)

```go
type DecisionKind uint8
const (
    DecisionAccept       DecisionKind = iota // A accept + emit final + Learn
    DecisionRetry                            // B retry (AttemptNo++)
    DecisionChildWorker                      // C spawn child Worker
    DecisionParentRollup                     // D trigger parent rollup
    DecisionHumanReview                      // E ❓ + emit abort
)

type RoundMeta struct {
    AttemptNo            int
    ChildBudgetRemaining int
    RiskLevel            string  // "high" | "normal" | "low"
    IsChildSegment       bool
    SiblingDecidedCount  int
    SiblingTotalCount    int
    HasDecomposableAC    bool
    ToleranceHint        string
    VerdictErrorClass    string
    PlanErrorClass       string
}

type Decision struct {
    Kind             DecisionKind
    Reason           string
    NextWorkItemSpec *ChildWorkItemSpec  // only when Kind == child_worker
    MapRow           int                 // 1-11, 0 = safety-net fallback
}

type ChildWorkItemSpec struct {
    ParentWorkItemID string
    SubSegmentIDs    []string
    InheritACSubset  []AcceptanceCriterion
    MaxBudget        int  // default 2, hard cap 2
}
// Validate() enforces: ParentWorkItemID != "", len(SubSegmentIDs) >= 1,
//                     len(InheritACSubset) >= 1, MaxBudget <= 2.
```

**11-row static mapping table** (rows 1-11, codex consensus 2026-07-07):

| # | Verdict | Other | Decision | Reason prefix |
|---|---------|-------|----------|---------------|
| 1 | Pass | (default) | accept | verdict_pass |
| 2 | Partial | Tolerance=high OR ChildBudget=0 | accept | verdict_partial+accept_high_tolerance |
| 3 | Partial | DecomposableAC + ChildBudget>0 | child_worker | verdict_partial+ac_decomposable |
| 4 | Partial | (other) | accept | verdict_partial+accept_default |
| 5 | Fail | AttemptNo < MaxRetry | retry | verdict_fail+attempt_N<max_M |
| 6 | Fail | AttemptNo >= MaxRetry | human_review | verdict_fail+attempt_N>=max_M |
| 7 | Indeterminate | RiskLevel=high | human_review | verdict_indeterminate+risk_high |
| 8 | Indeterminate | RiskLevel=normal/low | retry | verdict_indeterminate+risk_normal |
| 9 | VerdictKind=99 (out of enum) | VerdictErrorClass=network_timeout | retry | row_9_error_retry |
| 10 | (any) | IsChildSegment + SiblingDecided==SiblingTotal | parent_rollup | all_siblings_decided |
| 11 | (no Verdict) | PlanErrorClass set | human_review | plan_error:<class> |

**Safety-net fallback**: out-of-range VerdictKind + no VerdictErrorClass → `Decision{Kind: Accept, Reason: "decision_map_miss_fallback", MapRow: 0}` + warn log.

**Runaway guards**: AttemptNo > 100 or < 0 → error (caller falls back to A accept + warn).

### 3.3 NEW: `sessionorchestrator/decision_node_wire.go` (T49)

```go
func (r *ItemPipelineRunner) runDecisionStage(
    sessionID string,
    item *workmodel.WorkItem,
    verdict workmodel.Verdict,
    roundNo int,
) Decision {
    // Builds RoundMeta from item + r.Tasks (sibling counts, attempt no, etc.)
    // Calls NewStaticDecisionNode().Decide()
    // Returns Decision (callers persist to round fields)
}
```

**Helpers**:
- `attemptNoFromLastRound(item, roundNo) int` — converts roundNo to AttemptNo
- `childBudgetRemaining(item) int` — returns 2 (full budget, per-round decrement post-PR-D)
- `riskLevelForItem(item) string` — returns "normal" (per-WorkItem metadata post-PR-D)
- `isChildSegment(item) bool` — checks item.ParentID != ""
- `siblingCounts(sessionID, item, tm) (int, int)` — counts non-rollup children with LastRound
- `verdictErrorClassFor(verdict) string` — maps Verdict.SourceID to row 9 class
- `exitReasonForDecision(base, d) string` — rewrites base ExitReason per DecisionKind

### 3.4 MODIFIED: `workmodel/pipeline_round.go` — Decision persistence (T51)

Added 4 fields to `WorkItemPipelineRound`:
```go
DecisionKind   string `json:"decision_kind,omitempty"`
DecisionReason string `json:"decision_reason,omitempty"`
DecisionMapRow int    `json:"decision_map_row,omitempty"`
DecisionJSON   string `json:"decision_json,omitempty"`
```

`MarshalDecisionJSON(d Decision) (string, error)` produces canonical JSON:
```json
{
  "kind": "accept",
  "reason": "verdict_pass",
  "map_row": 1,
  "next_spec": null,  // omit when nil (DecisionAccept / Retry / ParentRollup / HumanReview)
  "decided_at": "2026-07-07T..."
}
```

### 3.5 MODIFIED: `sessionorchestrator/item_pipeline.go` — Wire into Run() (T49 wiring)

After Verify phase, before `endVerifyPhase`:
```go
decision := r.runDecisionStage(sessionID, item, verdict, roundNo)
round.DecisionKind = decision.Kind.String()
round.DecisionReason = decision.Reason
round.DecisionMapRow = decision.MapRow
round.DecisionJSON, _ = MarshalDecisionJSON(decision)
round.ExitReason = exitReasonForDecision(round.ExitReason, decision)
```

The DAG fork gate (added in PR-C) gets an additional `r.DAGEnabled` check:
```go
if r.DAGEnabled && pl.DAG != nil && pl.IntentSegmentSet != nil && r.DAGExecutor != nil {
    return r.executePlanDAG(...)
}
// else: legacy single-WorkItem path
```

### 3.6 MODIFIED: `bootstrap/wire_item_pipeline.go` — DAGEnabled wiring (T29 wiring)

Added `DAGEnabled bool` field to `ItemPipelineWireDeps`. Forwarded to `NewItemPipelineRunner(ItemPipelineDeps{...DAGEnabled: deps.DAGEnabled})`.

### 3.7 NEW: 18 unit tests (`sessionorchestrator/decision_node_test.go`) (T50)

| Test | Covers |
|------|--------|
| TestDecision_KindStringAllFive | Wire format locked (accept/retry/child_worker/parent_rollup/human_review) |
| TestDecision_InvalidKindRejects | Validate catches unknown enums |
| TestDecision_Row1..Row11 | All 11 rows of mapping table |
| TestDecision_Row10_OrderingNotReady_FallsThrough | Row 10 ordering guard |
| TestDecision_MapMiss_FallbackAccept | Out-of-range verdict → safety-net accept |
| TestDecision_RunawayAttemptNo_ErrorsOut | AttemptNo>100 → error |
| TestDecision_NegativeAttemptNo_ErrorsOut | AttemptNo<0 → error |
| TestChildWorkItemSpec_Validate | 6 sub-cases (nil/missing parent/missing subs/missing AC/over-budget/valid) |
| TestMarshalDecisionJSON_RoundTrip | Wire format round-trip |
| TestMarshalDecisionJSON_NoSpec | omit-empty for D/A/B/E paths |

### 3.8 NEW: 7 E2E tests (`sessionorchestrator/decision_node_e2e_test.go`) (T52, T30)

| Test | Covers |
|------|--------|
| TestE2E_LP4_DecisionPending_ParentRollup | Row 10 sibling-gating parent_rollup |
| TestE2E_LP4_HappyPath_Accept | Row 1 accept + ExitReason preserved |
| TestE2E_LP3_DAGForkSuppressedWhenFlagOff | PR-D gate: flag off → legacy path |
| TestE2E_LP3_DAGFlagOn_NilExecutor_LegacyPath | PR-D gate: flag on + nil executor → legacy path |
| TestE2E_LP4_FailAfterMaxRetry_HumanReview | Row 6 fail+max → human_review |
| TestE2E_LP4_AcceptDecision_PreservesBaseExitReason | DecisionAccept → base ExitReason unchanged |
| TestE2E_Helper_UsesProductionLearner | Sanity guard for test runner |

### 3.9 NOT in scope (deferred to PR-E, PR-F)

- **PR-E**: LearnRequest contract精简 + ReputationRow DB migration + AsyncLearner + 22 场景覆盖 (T53-T65)
- **PR-F**: Plan 节点 26 场景 (T66-T72)

---

## 4. OPEN QUESTIONS

### Q1. Decision Node placement — separate package or sessionorchestrator?
**(A)** `sessionorchestrator/decision_node.go` (current proposal)
**(B)** New `decision/` package — separate from sessionorchestrator
**(C)** `executionflow/decision/` — sibling of `executionflow/verify/`

→ **Recommendation: (A)** — the Decision Node is a per-round helper tightly
coupled to ItemPipelineRunner. No other consumer exists yet (PR-F's
plan_error path uses the same node). Promoting to a separate package is
premature. Easy to refactor later if reuse emerges.

### Q2. DecisionNode interface — interface or concrete type?
**(A)** Interface `DecisionNode { Decide(ctx) (Decision, error) }` + static impl (current proposal)
**(B)** Single static function `Decide(ctx) (Decision, error)` (no interface)

→ **Recommendation: (A)** — the interface allows future swap to LLM-based
or hybrid decision nodes (PR-E/PR-F may want this for force_plan).
Concrete function would block that evolution.

### Q3. ExitReason rewrite — always or only on divergence?
**(A)** Always rewrite (current `exitReasonForDecision` returns base for Accept, "decision_retry" for Retry, etc.)
**(B)** Always preserve base, attach DecisionKind as suffix only

→ **Recommendation: (A)** — current behavior is correct. Accept leaves
base unchanged (no suffix), Retry adds "+decision_retry" suffix. Dashboards
need the explicit DecisionKind signal in ExitReason.

### Q4. MapRow 0 vs safety-net accept — same or different?
**(A)** Both MapRow=0 (safety net) and MapRow=1 (Pass→accept) produce `DecisionAccept` but with different Reason strings. MapRow disambiguates.
**(B)** Safety net produces different DecisionKind (e.g., new `DecisionFallback`)

→ **Recommendation: (A)** — adding a new DecisionKind bloats the enum
(5 → 6) without functional benefit. MapRow=0 is enough to flag "we hit
the safety net" for telemetry. Dashboards can filter on MapRow.

### Q5. RoundMeta zero-value behavior — is the empty RoundMeta safe?
**(A)** Yes — `Decide(ctx)` handles zero-value RoundMeta by falling through to row 1 (Pass→accept) or row 6 (Fail→human_review) depending on VerdictKind.
**(B)** Add defensive `ErrRoundMetaEmpty` to force callers to populate.

→ **Recommendation: (A)** — current behavior is correct. ItemPipelineRunner
always populates RoundMeta; only test stubs may pass empty meta, and
they accept row 1/6 outcomes. Defensive error would block testing.

### Q6. MarshalDecisionJSON error handling
**(A)** Returns error (caller logs + uses empty string) — current proposal
**(B)** Always returns valid JSON (panic on internal error, never happens in practice)

→ **Recommendation: (A)** — JSON marshal of a stable struct shouldn't fail,
but defensive error returns the sentinel `ErrDecisionMarshalFailed` for
auditing. Caller in item_pipeline.go logs + uses empty string.

### Q7. E2E test for DAG flag flip-on with nil executor — necessary?
**(A)** Yes — current proposal. Tests the "DAGEnabled && DAGExecutor != nil" gate independence.
**(B)** No — flag-flip tests are enough; nil-executor is a production invariant.

→ **Recommendation: (A)** — the two gates being independent is a
non-obvious safety property. A future refactor might collapse them into
one and silently break the nil-executor fallback. The test pins the
behavior.

### Q8. RunDecisionStage signature — accept verdict by value or pointer?
**(A)** By value (current proposal: `verdict workmodel.Verdict`)
**(B)** By pointer (`*workmodel.Verdict`)

→ **Recommendation: (A)** — Verdict is a small struct (4 fields), value
semantics are cleaner. Pointer would force nil-check + aliasing concerns.

### Q9. DAGExecutor config field naming — `dag_executor` or `dag`?
**(A)** `dag_executor` (current proposal: full name, matches D7 d7_dag_executor naming)
**(B)** `dag` (shorter, but ambiguous)

→ **Recommendation: (A)** — `dag_executor` is unambiguous; matches the
D7 DAGExecutor identifier. Future fields (DAG validation, etc.) can
nest under `dag_executor` without collision.

### Q10. Should Decision Node run on legacy single-WorkItem path?
**(A)** Yes — current proposal. Every round's Verify → Decision transition fires.
**(B)** No — Decision Node only runs on multi-intent DAG path.

→ **Recommendation: (A)** — the Decision Node is the canonical Stage-5
of the 6-node pipeline. Single-WorkItem rounds benefit from the same
Decision persistence (decision_kind on round) for D5 dashboards + Learn
attribution. Excluding single-WorkItem path would create a confusing
two-tier behavior.

---

## 5. DELIVERABLES

1. `orchtypes/config.go` (MODIFIED) — DAGExecutorConfig + BuildDAGExecutorConfig + FileConfig.DAGExecutor field (~50 LOC)
2. `orchtypes/dag_executor_config_test.go` (NEW) — 5 unit tests (~100 LOC)
3. `sessionorchestrator/decision_node.go` (NEW) — DecisionKind + RoundMeta + Decision + ChildWorkItemSpec + 11-row mapping + MarshalDecisionJSON (~430 LOC)
4. `sessionorchestrator/decision_node_wire.go` (NEW) — runDecisionStage + helpers (~190 LOC)
5. `sessionorchestrator/decision_node_test.go` (NEW) — 18+ unit tests (~480 LOC)
6. `sessionorchestrator/decision_node_e2e_test.go` (NEW) — 7 E2E tests (~335 LOC)
7. `workmodel/pipeline_round.go` (MODIFIED) — 4 Decision fields (~20 LOC)
8. `sessionorchestrator/item_pipeline.go` (MODIFIED) — runDecisionStage call + DAGEnabled flag wiring (~30 LOC)
9. `bootstrap/wire_item_pipeline.go` (MODIFIED) — DAGEnabled wiring (~5 LOC)
10. Tests: 26/26 orchestration packages `go test -race` PASS
11. `go vet ./...` clean
12. Spec/t-registry/changelog delta sync (T31)
13. PR-D ready for review

---

## 6. TEST MATRIX

| Scenario | Expected | Test |
|----------|----------|------|
| Row 1 Pass→accept | accept, MapRow=1 | TestDecision_Row1_PassDefault_Accept |
| Row 2 Partial+high tolerance→accept | accept, MapRow=2 | TestDecision_Row2a_PartialToleranceHigh_Accept |
| Row 2 Partial+ChildBudget=0→accept | accept, MapRow=2 | TestDecision_Row2b_PartialChildBudgetZero_Accept |
| Row 3 Partial+decomposable→child_worker | child_worker, MapRow=3, next_spec populated | TestDecision_Row3_PartialDecomposable_ChildWorker |
| Row 4 Partial+other→accept | accept, MapRow=4 | TestDecision_Row4_PartialFallback_Accept |
| Row 5 Fail+AttemptNo<MaxRetry→retry | retry, MapRow=5 | TestDecision_Row5_FailBelowMaxRetry_Retry |
| Row 6 Fail+AttemptNo>=MaxRetry→human_review | human_review, MapRow=6 | TestDecision_Row6_FailAtOrAboveMaxRetry_HumanReview |
| Row 7 Indeterminate+high risk→human_review | human_review, MapRow=7 | TestDecision_Row7_IndeterminateRiskHigh_HumanReview |
| Row 8 Indeterminate+normal risk→retry | retry, MapRow=8 | TestDecision_Row8_IndeterminateRiskNormal_Retry |
| Row 9 out-of-range VerdictKind+Network→retry | retry, MapRow=9 | TestDecision_Row9_OutOfRangeVerdictError_Retry |
| Row 10 child+all siblings decided→parent_rollup | parent_rollup, MapRow=10 | TestDecision_Row10_AllSiblingsDecided_ParentRollup |
| Row 10 ordering guard | child+not-all-siblings→falls through to row 1 | TestDecision_Row10_OrderingNotReady_FallsThrough |
| Row 11 plan_error→human_review | human_review, MapRow=11 | TestDecision_Row11_PlanError_HumanReview |
| Safety-net fallback | out-of-range verdict+no class→accept, MapRow=0 | TestDecision_MapMiss_FallbackAccept |
| Runaway AttemptNo | AttemptNo=101 → error | TestDecision_RunawayAttemptNo_ErrorsOut |
| Negative AttemptNo | AttemptNo=-1 → error | TestDecision_NegativeAttemptNo_ErrorsOut |
| ChildWorkItemSpec.Validate | 6 sub-cases (nil/missing fields/over-budget/valid) | TestChildWorkItemSpec_Validate |
| MarshalDecisionJSON round-trip | kind/reason/map_row/next_spec all survive | TestMarshalDecisionJSON_RoundTrip |
| MarshalDecisionJSON no-spec | D/A/B/E paths omit next_spec field | TestMarshalDecisionJSON_NoSpec |
| E2E LP-4 row 10 | parent_rollup via sibling gating | TestE2E_LP4_DecisionPending_ParentRollup |
| E2E LP-4 happy path | accept + ExitReason preserved | TestE2E_LP4_HappyPath_Accept |
| E2E LP-3 DAG flag off | DAG fork suppressed, legacy path | TestE2E_LP3_DAGForkSuppressedWhenFlagOff |
| E2E LP-3 flag on + nil executor | legacy path runs, no crash | TestE2E_LP3_DAGFlagOn_NilExecutor_LegacyPath |
| E2E LP-4 fail + max retry | row 6 human_review | TestE2E_LP4_FailAfterMaxRetry_HumanReview |
| E2E LP-4 accept preserves ExitReason | no decision_retry suffix on Accept | TestE2E_LP4_AcceptDecision_PreservesBaseExitReason |
| Test runner uses production learner | smoke test for LP-1/LP-4 contract | TestE2E_Helper_UsesProductionLearner |

---

## 7. RISK REGISTER

| Risk | Severity | Mitigation |
|------|----------|-----------|
| Decision Node fields break legacy round JSON deserialization | Low | 4 new fields are omitempty; legacy readers ignore them |
| DAGEnabled flag accidentally flipped to true in production | Med | Default OFF; BuildDAGExecutorConfig preserves default if file section absent |
| RunDecisionStage called when Tasks=nil (test stub) | Low | Wire has nil-check + falls back to row 1/6 only |
| 11-row mapping table diverges from codex consensus | Low | codex consensus 2026-07-07 reviewed all 11 rows; docstring cites decision-tree.md §8.6.1 |
| AttemptNo runaway causes infinite retry | Med | Err when AttemptNo>100; runDecisionStage catches + logs + returns safety-net accept |
| Test runner helper signature drift | Low | E2E tests use newItemPipelineTestRunner(t) → (runner, tm, rep); pinned to item_pipeline_test.go:161 |
| Decision persistence drops VerdictKind=0 (Pass) due to JSON unmarshal zero-value | Low | DecisionKind is `string` field, not enum; omitempty keeps empty rounds clean |
| ExitReason rewrite loses original verdict-driven reason | Low | exitReasonForDecision concatenates base + decision suffix; base preserved |
| E2E LP-4 fail test flakes due to stub WorkItemExecutor producing Pass | Med | Test overrides `runner.Verify` to return VerdictFail deterministically |

---

## 8. CONSENSUS QUESTIONS (for codex/cursor)

Please respond with **ACCEPT / ADOPT-WITH-CHANGE / REJECT** for each:

1. **Q1**: Decision Node placed in `sessionorchestrator/` package (not separate `decision/` or `executionflow/decision/`)?
2. **Q2**: DecisionNode interface (with static impl), allowing future swap?
3. **Q3**: ExitReason rewrite always (Accept leaves base, Retry adds "+decision_retry")?
4. **Q4**: Safety-net fallback uses MapRow=0 (no new DecisionKind enum value)?
5. **Q5**: Empty RoundMeta safe (no defensive ErrRoundMetaEmpty)?
6. **Q6**: MarshalDecisionJSON returns error (caller logs + uses empty string)?
7. **Q7**: E2E test for DAG flag-on-nil-executor included?
8. **Q8**: runDecisionStage takes verdict by value (not pointer)?
9. **Q9**: YAML config key is `dag_executor` (not `dag`)?
10. **Q10**: Decision Node runs on every round (including single-WorkItem legacy path)?

---

## 9. REVIEW DELIVERABLES

After review, expected output:
- `reviews/pr-d-codex-consensus-2026-07-07.md`
- `reviews/pr-d-cursor-consensus-2026-07-07.md`
- Implementation follows adopted consensus
