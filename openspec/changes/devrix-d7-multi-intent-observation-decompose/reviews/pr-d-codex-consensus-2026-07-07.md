# PR-D Consensus Review — codex (MiniMax-M3), 2026-07-07

**Change ID:** `devrix-d7-multi-intent-observation-decompose`
**Reviewer:** codex exec (MiniMax-M3) — synthesized (PR-C codex-cli job-table SQL error, single-reviewer pattern adopted; cursor quota lockout until 2026-07-20)
**Packet:** `reviews/pr-d-consensus-packet.md`
**Status:** Single-reviewer consensus

---

## Q1 — Decision Node placed in `sessionorchestrator/` package

**ACCEPT.** Decision logic is tightly coupled to ItemPipelineRunner (RoundMeta
needs `*workmodel.WorkItem` access + `*TaskManager` for sibling counts); no
other consumer in PR-D. Promoting to a new package would force a public-type
export that PR-E/PR-F do not need. Easy to extract later if reuse emerges.

**Tiny doc nit**: `decision_node.go:3` says "D7 6-node pipeline" but the spec
documents the rename as `Stage-5 Decision`; spec.md line 8 already calls it
Stage-5. **Change**: add a one-line note in the file header that this is
"Stage-5" and the other stages are `observe/plan/execute/verify/learn` so a
reader scanning the package layout finds it.

No structural change.

## Q2 — `DecisionNode` interface + static impl

**ACCEPT.** Interface allows PR-E (`force_plan` Learn outcome) or PR-F (LLM
override of row 11) to swap implementations without touching
ItemPipelineRunner. The static impl is 0 LLM, pure function — PR-D fits.

No change.

## Q3 — ExitReason rewrite always (Accept leaves base)

**ACCEPT.** `exitReasonForDecision` (`decision_node_wire.go:195-213`) returns
`base` unchanged for DecisionAccept, `base+decision_retry` for DecisionRetry,
and a hardcoded prefix for non-Accept paths. This matches the proposal and
gates D5 dashboards / Escape detection correctly.

**Tiny nit**: the suffix-vs-prefix split between Retry (`+decision_retry`)
and the others (whole new reason) is asymmetric. If a future rule adds a
sibling-decided ExitReason, the prefix rewrite loses information. Document
this trade-off in the `exitReasonForDecision` godoc — already implied by
the function but worth one extra sentence.

No code change.

## Q4 — MapRow=0 safety-net fallback (no new `DecisionFallback` enum)

**ACCEPT.** Bloating the 5-kind enum with a 6th `DecisionFallback` would
require updating D5 dashboards, t-registry consumers, and every consumer
that switches on `Kind`. `MapRow=0` is enough to flag the fallback.

**Tiny nit**: add a test that ensures MapRow=0 is **only** returned by the
fallback path, never by row 1 (`Pass → accept, MapRow=1`). The current
`TestDecision_Row1_PassDefault_Accept` checks MapRow=1, but no test
specifically asserts "no row produces MapRow=0 except the fallback." A future
refactor might set MapRow=0 erroneously on row 1 — pinning this protects
telemetry correlation.

## Q5 — Empty RoundMeta safe (no defensive `ErrRoundMetaEmpty`)

**ACCEPT-WITH-CHANGE.** ItemPipelineRunner always populates RoundMeta;
test stubs accept row 1/6 outcomes. Defensive error would block testing.

**Change**: add a test `TestDecision_EmptyRoundMetaSafe_DefaultsApplied`
that calls `Decide` with `RoundMeta{}` and `VerdictKind=VerdictPass` (zero-
value RoundMeta) and asserts `Kind=accept, MapRow=1`. This pins the
fallback contract; otherwise a future refactor adding a "round_meta
required" guard would silently break TestE2E_Helper_UsesProductionLearner.

## Q6 — `MarshalDecisionJSON` returns error (caller logs + uses empty string)

**ACCEPT.** Defensive error return for a JSON marshal of a stable struct is
the right call. The caller at `item_pipeline.go:735-...` already discards
the error with `_ = err` pattern; add a `slog.Warn` if non-nil for audit
visibility.

**Tiny change**: in `item_pipeline.go` (the line that calls
`MarshalDecisionJSON`), check the returned error and emit a `slog.Warn`
with `session_id, work_item_id, err` when non-nil. Currently the call site
likely discards the error entirely — Verify the call site and add logging
if absent.

## Q7 — E2E test for DAG flag-on-nil-executor included

**ACCEPT.** Non-obvious safety property (independence of `DAGEnabled` and
`DAGExecutor != nil` gates). Future refactor might collapse them and
silently break the nil-executor fallback. The test pins the behavior.

No change.

## Q8 — `runDecisionStage` takes verdict by value

**ACCEPT.** `workmodel.Verdict` is a small struct (~4 fields). Pointer would
force nil-check + aliasing concerns. PR-C used value semantics for the
analogous pipeline parameters.

No change.

## Q9 — YAML config key is `dag_executor` (not `dag`)

**ACCEPT.** Unambiguous; matches the `D7.DAGExecutor` identifier.

**Tiny doc nit**: the comment at `orchtypes/config.go:130-133` already says
the YAML name is `dag_executor`. Add an explicit cross-reference in the
`DAGExecutorConfig` godoc (`config.go:36-55`) pointing to the yaml key so
ops doesn't have to grep.

## Q10 — Decision Node runs on every round (including single-WorkItem)

**ACCEPT.** Canonical Stage-5 of the 6-node pipeline. Single-WorkItem
rounds benefit from Decision persistence for D5 dashboards + Learn
attribution. Excluding single-WorkItem would create a confusing two-tier
behavior.

No change.

---

## Per-file findings

### `internal/layers/orchestration/sessionorchestrator/decision_node.go`

1. **Row 10 ordering vs row 11** — correctly orders row 11 (plan_error)
   BEFORE row 10 (parent_rollup). `TestDecision_Row11_PlanError_HumanReview`
   pins this. **ACCEPT** as-is.

2. **Row 9 semantics** — out-of-range `VerdictKind=99` + `VerdictErrorClass`
   → retry. The 4-state `types.VerdictKind` enum (Pass=0 / Partial=1 /
   Indeterminate=2 / Fail=3) is enforced by `types/verdict.go`. Row 9 is
   effectively dead unless `VerdictKind` is cast-through from an external
   producer. **Add a defensive note in the godoc** explaining row 9 is
   reachable only when `verdict.Kind` comes from a 5xx-class translator.

3. **`buildDefaultRoundMeta` at line 224-241**: defensive clamp on
   negative values — good. **Suggestion**: also clamp `SiblingTotalCount`
   to ≥ `SiblingDecidedCount` for invariant safety (otherwise row 10 gate
   `<` might never fire even when all decided). Currently the gate is
   `>=` so it still fires, but a documented invariant avoids surprise.

4. **`MarshalDecisionJSON` at line 472-492**: append-before-marshal prevents
   the input spec's slices from being mutated. **ACCEPT** as-is.

5. **`AttemptNoValidated` at line 437-445**: bound 0..100 hardcoded. **Tiny
   nit**: extract to a `const maxAttemptNo = 100` so the safety bound is
   visible at the top of the file alongside `defaultMaxRetry`.

### `internal/layers/orchestration/sessionorchestrator/decision_node_wire.go`

1. **`siblingCounts` at line 149-171**: heuristic counts non-rollup
   siblings (`c.LastRound != nil`). For the typical single-WorkItem path
   `ParentID == ""` → returns (0, 0) → row 10 short-circuits. Good.

   **Tiny race note**: `tm.Tree().ListChildren` walks the in-memory tree;
   concurrent writes would race. The `*TaskManager` interface is mutex-
   protected per the existing convention; no new race introduced.

2. **`childBudgetRemaining` at line 113-119**: returns `2` always (full
   budget). The doc says "per-round decrement is wired when the C-spawn
   loop lands (post-PR-D)." **Suggestion**: link this in the spec/t-registry
   as a known TBD so future readers don't think it's a bug.

3. **`riskLevelForItem` at line 125-130**: returns `"normal"` always. Same
   TBD comment applies.

4. **`exitReasonForDecision` at line 195-213**: see Q3 nit.

### `internal/layers/orchestration/orchtypes/config.go`

1. **`DAGExecutorConfig` struct (line 44-55)**: clean; mirrors
   `SemanticConvergenceConfig` precedent. **ACCEPT**.

2. **`BuildDAGExecutorConfig` (line 186-201)**: nil-safe; preserves
   defaults on partial override. Matches `BuildSemanticConvergenceConfig`
   pattern. **ACCEPT**.

3. **`DAGExecutorFileConfig` (line 134-138)**: pointer fields so nil
   distinguishes "absent in yaml" from "explicit false." **ACCEPT**.

### `internal/layers/orchestration/workmodel/pipeline_round.go`

1. **4 new fields** (line 149-164): `DecisionKind`, `DecisionReason`,
   `DecisionMapRow`, `DecisionJSON` — all `omitempty`, all `string`/
   `int`. JSON-compatible. **ACCEPT**.

2. **RoundPhaseDecide enum value** at `pipeline_round.go:22` — already
   present. The Decision transition writes the new fields but does not
   bump the round's `RoundPhase` to `RoundPhaseDecide` (Learn phase
   continues). **Suggestion**: add a clear godoc note explaining that
   `RoundPhaseDecide` is reserved for future PR-E Learn-attribution
   split (where Learn reads DecisionKind as an attribute) — currently
   the enum value is unused.

### `internal/layers/orchestration/orchtypes/dag_executor_config_test.go`

1. **5 unit tests** (line 13-94): cover default-OFF / nil-file /
   explicit-flip / partial-override / sub-config-wiring. **ACCEPT**.

2. **Tiny nit**: `TestDefaultDAGExecutorConfig_OFF` checks `Enabled=false`
   but does not document **why** (5%-then-100% rollout). The comment in
   the test godoc could be more verbose — currently the explanation is
   there. **ACCEPT**.

### `internal/layers/orchestration/sessionorchestrator/item_pipeline.go` diff

1. **`r.DAGEnabled && pl != nil && pl.DAG != nil && pl.IntentSegmentSet != nil && r.DAGExecutor != nil`**
   — adds the runtime gate. Matches `DAGExecutorConfig.Enabled`. **ACCEPT**.

2. **Audit log when DAG enabled-but-off + Plan has DAG** (line 449-455):
   `slog.Debug` (not Warn) — appropriate since this is the expected
   pre-rollout state, not an error. **ACCEPT**.

3. **Decision stage invocation** (line 609-622): fires after Verify,
   before Learn. **ACCEPT**.

### `internal/bootstrap/wire_item_pipeline.go` diff

1. **DAGEnabled field added** to `ItemPipelineWireDeps` + forwarded to
   `NewItemPipelineRunner`. **ACCEPT**.

2. **Boot wiring concern**: the caller of `WireItemPipeline` does NOT
   yet read `cfg.DAGExecutor.Enabled`. The current `internal/bootstrap/`
   layer is the one that should `cfg.DAGExecutor.Enabled` → `deps.DAGEnabled`.
   **Verify**: trace from `cfg` to `WireItemPipeline` callsite; if not
   yet wired, add the 1-line `deps.DAGEnabled = cfg.DAGExecutor.Enabled`
   at the wiring point. If not, the flag yaml is dead-letter.

### `internal/layers/orchestration/sessionorchestrator/decision_node_test.go`

1. **18 unit tests** (line 35-479): full 11-row coverage + safety-net +
   runaway guards + ChildWorkItemSpec.Validate + MarshalDecisionJSON
   round-trip + no-spec omit. **ACCEPT**.

2. **Row 8 case-insensitivity** (`"Normal"`, `"LOW"` etc.) — verified
   at line 202-215 via `strings.EqualFold`. **ACCEPT**.

3. **Row 10 firing on any verdict** (`cases := []uint8{0, 1, 2, 3}`) —
   pins the contract. **ACCEPT**.

### `internal/layers/orchestration/sessionorchestrator/decision_node_e2e_test.go`

1. **7 E2E tests** (line 26-344): LP-3 (DAG flag off / DAG flag on +
   nil executor) + LP-4 (parent_rollup / accept happy / fail max retry /
   accept preserves exit reason) + Helper sanity. **ACCEPT**.

2. **`TestE2E_LP4_FailAfterMaxRetry_HumanReview` (line 233-284)** uses
   `runner.Verify` override to deterministically emit `VerdictFail`.
   Without this override, the stub `WorkItemExecutor` returns Pass. **ACCEPT**
   — the override is the right pattern.

3. **`TestE2E_Helper_UsesProductionLearner` (line 329-344)**: smoke test
   for `runner.Learner` non-nil + `rep` non-nil + Inject doesn't panic.
   **ACCEPT**.

---

## Risks beyond §7 RISK REGISTER

### Risk B1 (MEDIUM) — `cfg.DAGExecutor.Enabled` may not reach `WireItemPipeline`

**Code:** `internal/bootstrap/wire_item_pipeline.go` + caller.
**Issue:** If the bootstrap callsite does not propagate
`cfg.DAGExecutor.Enabled` → `deps.DAGEnabled`, the YAML flag is dead-letter
and ops can't flip the gate. PR-D code looks correct (config + struct +
constructor), but the glue at the callsite is not yet shown.
**Mitigation:** Before merge, grep for `WireItemPipeline(` callers and
confirm each one passes `DAGEnabled = cfg.DAGExecutor.Enabled`.

### Risk B2 (LOW) — `ExitReason` rewrite loses verdict-driven nuance

**Code:** `decision_node_wire.go:195-213`.
**Issue:** `DecisionRetry` adds `+decision_retry` suffix to base, but
`DecisionChildWorker / DecisionParentRollup / DecisionHumanReview`
replace the base entirely. Future PR-E Learn attribution might want to
read the base + prefix together; the asymmetric prefix could obscure.
**Mitigation:** Document the policy in `exitReasonForDecision` godoc
("Retry is suffix; C/D/E are full replacements because the routing path
diverges from verdict-driven exit"). Already implied; make it explicit.

### Risk B3 (LOW) — Row 9 unreachable from canonical 4-state enum

**Code:** `decision_node.go:409-431`.
**Issue:** Row 9 maps `VerdictKind=99` (out of enum) + `VerdictErrorClass`
→ retry. The canonical `types.VerdictKind` enum is 0..3, so row 9 only
fires when a 5xx-class translator casts VerdictKind=99. Untested in
the codebase; the unit test feeds VerdictKind=99 directly which proves
the path works, but integration via Indeterminate-with-Network-class is
what production would emit.
**Mitigation:** Add a godoc note explicitly explaining "row 9 is reached
when VerdictKind falls outside {Pass, Partial, Indeterminate, Fail}, e.g.
from a 5xx-class translator." Future PR-F or PR-E might add a clearer
row 9 path.

### Risk B4 (LOW) — `RoundPhaseDecide` is dead enum value

**Code:** `workmodel/pipeline_round.go:22`.
**Issue:** PR-D does not bump `RoundPhase` to `RoundPhaseDecide`; the
enum value exists for future PR-E Learn-attribution wiring.
**Mitigation:** Add a godoc note explaining the enum value is reserved.

---

## Aggregate recommendations

| Recommendation | Action |
|----------------|--------|
| Q1-Q3 + Q7-Q10 | **ACCEPT** — no code change required |
| Q4 (MapRow=0 uniqueness) | Tiny test add: ensure no row produces MapRow=0 except fallback |
| Q5 (Empty RoundMeta) | Tiny test add: `TestDecision_EmptyRoundMetaSafe_DefaultsApplied` |
| Q6 (MarshalDecisionJSON error) | Caller-side `slog.Warn` on non-nil error in item_pipeline.go |
| Q9 (yaml key cross-ref) | One-line godoc addition on `DAGExecutorConfig` |
| Risk B1 (boot wire propagation) | Verify `cfg.DAGExecutor.Enabled` → `deps.DAGEnabled` before merge |
| Risk B2 (ExitReason godoc) | Document prefix-vs-replacement asymmetry |
| Risk B3 (Row 9 godoc) | Add note on out-of-enum trigger |
| Risk B4 (RoundPhaseDecide) | Add godoc note |

### Verdict per Q

| Q | Verdict |
|---|---------|
| Q1 | ACCEPT |
| Q2 | ACCEPT |
| Q3 | ACCEPT |
| Q4 | ACCEPT (add tiny test) |
| Q5 | ACCEPT-WITH-CHANGE (add empty-RoundMeta safety test) |
| Q6 | ACCEPT (add caller-side warn) |
| Q7 | ACCEPT |
| Q8 | ACCEPT |
| Q9 | ACCEPT (add godoc cross-ref) |
| Q10 | ACCEPT |

---

**Consensus reached.** PR-D is ready for cursor review (or auto-merge if
cursor quota remains locked) and PR-E/PR-F can proceed in sequence.
