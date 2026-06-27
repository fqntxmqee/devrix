# Spec Delta — D7-Orchestration — WorkItem Rollup 闭环

**Change ID:** `devrix-d7-workitem-rollup-pipeline`  
**Target Spec:** `openspec/specs/d7-orchestration/spec.md`  
**Target Version:** v4.12.0 → v4.13.0  
**Demand ID:** DM-20260627-001  
**Created:** 2026-06-27  
**Status:** S3_Design — 待 Review

---

## ADDED Requirements

本 delta 在 **Scenario D7-S15（WorkItem Rollup 闭环）** 下新增 7 个 A 能力（Phase 1）与 2 个 A 能力（Phase 2 登记）。

### D7-S15-A50: Parent Rollup Gate — 子 terminal 后父不 auto-complete

When all direct non-checklist children of a decomposed parent WorkItem reach a terminal status, the WorkTree resolver shall **not** mark the parent as `completed`. Instead it shall set `NeedsRollup=true`, reset the parent to `pending`, and allow `GetPipelineFocus` to select the parent for a subsequent MUPS round.

#### Scenario: decompose parent triggers rollup gate when all children terminal

- **Given** parent WorkItem `P` with `LastRound.SpawnPolicy=SpawnDecompose` and direct children `C1`, `C2` (kind `implement`, not checklist)
- **And** `P` has `RollupGatePolicy=best_effort` (default)
- **When** `C1` and `C2` both reach terminal status (`completed` or `failed`)
- **Then** `ShouldRollupAfterChildren` shall return true
- **And** `ReevaluateParentAfterChild` shall set `P.NeedsRollup=true` and `P.Status=pending`
- **And** `P.Status` shall **not** be set to `completed` until rollup MUPS round finishes

#### Scenario: all_pass policy blocks rollup when any child fails

- **Given** parent `P` with `RollupGatePolicy=all_pass` and children `C1` (pass), `C2` (fail, terminal)
- **When** `ReevaluateParentAfterChild` runs
- **Then** `ShouldRollupAfterChildren` shall return false
- **And** `P.NeedsRollup` shall remain false until `C2` is retried to pass or policy changes

#### Scenario: min_coverage policy triggers rollup before all children terminal

- **Given** parent `P` with `RollupGatePolicy=min_coverage`, `MinChildCoverageRatio=0.8`, and 5 direct implement children
- **When** 4 of 5 children reach terminal status
- **Then** `ShouldRollupAfterChildren` shall return true
- **And** `ReevaluateParentAfterChild` shall set `P.NeedsRollup=true`

#### Scenario: parent without decompose children completes normally

- **Given** parent WorkItem `P` (non-root) with zero direct non-checklist children
- **When** `P` finishes its own MUPS round with `SpawnPolicy=SpawnNone`
- **Then** `P` shall transition to `completed` or `failed` without setting `NeedsRollup`
- **Unless** Path B Root Session Rollup Fallback applies (see D7-S15-A54)

#### Scenario: root goal with spawn none and checklist triggers Path B not immediate terminal

- **Given** session root goal `R` with `SpawnPolicy=SpawnNone` after Round 1
- **And** ≥1 direct ephemeral checklist child exists under `R`
- **When** `HasOpenWork` would become false (no non-ephemeral pending work)
- **Then** `maybeRootRollupFallback` shall set `R.NeedsRollup=true` before session closes
- **And** `R` shall **not** remain terminal failed without a rollup round

#### Scenario: GetPipelineFocus prioritizes rollup pending parent

- **Given** parent `P` with `NeedsRollup=true` and `Status=pending`
- **And** open explore children still exist in the session
- **When** `GetPipelineFocus` is called
- **Then** `P` shall be selected before new explore children (rollup priority boost)

---

### D7-S15-A51: Summary Bubble Materialize — 子 ArtifactSummary 进入父 Observe（P0 — T05 闭合）

The parent WorkItem Observe phase shall materialize **Summary** context bubbles from each direct child's `LastRound.ArtifactSummary`, truncated per ContextGraph CB3 budget, **in addition to** mandatory Structured bubbles. Rollup Round 2+ Observe **must** ingest **both** bubble kinds when child summaries exist — structured-only Observe is non-compliant.

#### Scenario: parent Observe contains summary_child_bubble for each direct child

- **Given** parent `P` entering rollup Round 2 Observe
- **And** child `C1` with `LastRound.ArtifactSummary="Review findings for prepare/ ..."` (len > 0)
- **When** `buildObserveContext` runs for `P`
- **Then** observations shall include a statement matching `summary_child_bubble: child=C1; len=...; preview="..."`
- **And** observations shall **also** include `structured_child_bubble: child=C1; ...` for the same child
- **And** preview length shall be ≤ CB3 truncate budget (default 2048 runes)

#### Scenario: rollup Observe requires dual bubble injection (T05 closure)

- **Given** decompose parent `P` with two terminal implement children, each with non-empty `ArtifactSummary`
- **When** `observationsFromChildSummaryBubbles` and structured bubble collectors run for rollup Observe
- **Then** Observe output shall contain **both** `structured_child_bubble` and `summary_child_bubble` per child
- **And** rollup Execute directive shall list truncated summaries sourced from Summary bubbles (not structured metadata alone)

#### Scenario: empty child summary skips summary bubble but keeps structured bubble

- **Given** child `C2` with empty `ArtifactSummary` but non-empty verdict metadata
- **When** parent Observe runs
- **Then** `structured_child_bubble: child=C2; ...` shall be present
- **And** no `summary_child_bubble` shall be emitted for `C2`

---

### D7-S15-A60: Parent Rollup Round 2+ MUPS Synthesize

When `NeedsRollup=true`, the ItemPipelineRunner shall execute an additional full MUPS round on the parent with a rollup-specific Plan and Directive that synthesizes child outputs into a single deliverable artifact.

#### Scenario: rollup round uses CommitmentPlan with rollup FailureCriteria

- **Given** parent `P` with `NeedsRollup=true` and two terminal children
- **When** `RunItemPipeline(P)` executes
- **Then** `Plan.PlanKind` shall be `CommitmentPlan`
- **And** `Plan.FailureCriteria` shall include rollup template checks (minimum summary length, child coverage)

#### Scenario: rollup Execute directive lists child verdicts and summaries

- **Given** children `C1` (pass) and `C2` (fail) with artifact summaries
- **When** parent rollup Execute runs
- **Then** the WorkItemExecutor directive shall enumerate each child's `wi_id`, verdict, and truncated summary
- **And** the directive shall instruct the model to output a final deliverable (not planning meta-text)

#### Scenario: rollup Verify Pass clears NeedsRollup and completes parent

- **Given** parent rollup round produces `ArtifactSummary` with len ≥ template threshold
- **When** Verify returns `VerdictPass`
- **Then** `P.NeedsRollup=false` and `P.Status=completed`
- **And** `P.LastRound.ArtifactSummary` shall contain the synthesized deliverable

#### Scenario: Jaeger shows two MUPS pipelines on same parent wi_id

- **Given** integration fixture with decompose + rollup
- **When** trace is collected for parent `wi_*`
- **Then** ≥2 spans with operation `D7_MUPS_Pipeline` shall share the same `workitem.id` attribute

---

### D7-S15-A61: Session complete Deliverable — root summary 非空

`RunSessionTurnLoop` shall populate the terminal `complete` EngineEvent `Content` with the session deliverable summary derived from the root WorkItem post-rollup artifact, reversing the intentional empty-Content hotfix for user-facing sessions.

#### Scenario: complete Content equals root post-rollup ArtifactSummary

- **Given** session with root WorkItem `R` that completed rollup with `LastRound.ArtifactSummary` len ≥ 500
- **When** `RunSessionTurnLoop` exits with no open work
- **Then** the final `complete` event shall have `Content=R.LastRound.ArtifactSummary`
- **And** `Content` shall not be empty string

#### Scenario: complete fallback when root summary empty

- **Given** root rollup failed but children have partial summaries
- **When** session closes
- **Then** `complete.Content` shall be best-effort concatenation of direct child summaries
- **Or** if still empty, an `error` event shall explain missing deliverable (no silent empty complete)

---

### D7-S15-A53: Ephemeral Checklist Gate — 不参与 MUPS focus

Ephemeral checklist WorkItems created via `todo_write` shall **not** be selected by `GetReadyItems` / `GetFocus` for ItemPipeline MUPS execution, and shall not be the sole reason a session remains open after root rollup completes.

#### Scenario: GetFocus skips ephemeral checklist items

- **Given** session with root `R` terminal after Round 1
- **And** 11 direct ephemeral checklist children with `Status=pending`
- **When** `GetPipelineFocus` is called
- **Then** focus shall **not** be any checklist child
- **And** no `D7_MUPS_Pipeline` span shall be emitted for checklist `wi_*` IDs

#### Scenario: ephemeral-only open work does not block session close after parent rollup

- **Given** parent `P` with `NeedsRollup=false` after successful rollup
- **And** only checklist/ephemeral children remain open
- **When** `HasOpenWork(session)` is evaluated at TurnLoop exit
- **Then** session shall be eligible to close and emit `complete` with deliverable

#### Scenario: decomposed parent with terminal implement children still requires rollup

- **Given** parent `P` spawned via `SpawnDecompose` with ≥1 implement child terminal
- **When** all implement children are terminal
- **Then** `NeedsRollup` shall be true regardless of open checklist items

---

### D7-S15-A54: Root Session Rollup Fallback + Virtual Checklist Bubbles

When a session is about to close with a root goal that used `SpawnNone` (or failed Round 1) but has ephemeral checklist children from `todo_write`, the TurnLoop shall force a root rollup round using **virtual checklist bubbles** (checklist directives as Observe input) without running MUPS on each checklist item.

#### Scenario: maybeRootRollupFallback sets NeedsRollup on root with checklist

- **Given** root goal `R` with `LastRound.SpawnPolicy=SpawnNone` and `VerdictKind=fail`
- **And** direct ephemeral checklist children `CL1..CLN` under `R`
- **And** no pending non-checklist non-ephemeral children
- **When** TurnLoop evaluates session closure
- **Then** `maybeRootRollupFallback` shall set `R.NeedsRollup=true` and `R.Status=pending`
- **And** the next `RunItemPipeline` target shall be `R`

#### Scenario: rollup Observe includes checklist_child_bubble statements

- **Given** root `R` entering Path B rollup Observe
- **And** checklist child `CL1` with `Directive="Review prepare/ pipeline"`
- **When** `buildObserveContext` runs for `R`
- **Then** observations shall include `checklist_child_bubble: child=CL1; status=...; directive="..."`
- **And** directive preview shall be truncated per CB3 budget

#### Scenario: trace replay fixture produces two MUPS pipelines on root wi_id

- **Given** integration fixture mimicking trace `58e6c55dd4d42284e4c2bed3ebeda28b` (review + todo_write, spawn=none)
- **When** `RunSessionTurnLoop` completes
- **Then** Jaeger (or span recorder) shall show ≥2 `D7_MUPS_Pipeline` spans with the same root `workitem.id`
- **And** zero `D7_MUPS_Pipeline` spans for ephemeral checklist IDs
- **And** `complete.Content` shall contain review deliverable structure (P0/P1 sections or equivalent)

#### Scenario: free_fork artifacts remain out of rollup scope in Phase 1

- **Given** root rollup round with checklist bubbles only (no free_fork WorkTree nodes)
- **When** rollup Execute synthesizes deliverable
- **Then** output shall be based on checklist directives and root R1 context
- **And** absence of free_fork artifacts shall be documented as known Phase 1 limitation

---

### D7-S15-A55: RollupGatePolicy — ShouldRollupAfterChildren 门控

When a decomposed parent awaits direct children, `ReevaluateParentAfterChild` shall evaluate `ShouldRollupAfterChildren(parent, RollupGatePolicy)` before setting `NeedsRollup`. Phase 1 default policy is `best_effort` (all direct non-checklist children terminal). Path B Root Fallback uses hard-coded `best_effort` semantics without persisting policy on the WorkItem.

#### Scenario: best_effort triggers rollup when all children terminal including failures

- **Given** parent `P` with `SpawnDecompose` and `RollupGatePolicy=best_effort`
- **And** children `C1` (VerdictPass) and `C2` (VerdictFail), both terminal
- **When** `ReevaluateParentAfterChild` runs after `C2` terminal
- **Then** `ShouldRollupAfterChildren` shall return true
- **And** `P.NeedsRollup=true`

#### Scenario: policy evaluated on each child terminal transition

- **Given** parent `P` with 3 implement children and `RollupGatePolicy=min_coverage` (threshold 0.67)
- **When** only 1 of 3 children is terminal
- **Then** `ShouldRollupAfterChildren` shall return false
- **When** a second child becomes terminal
- **Then** `ShouldRollupAfterChildren` shall return true

---

## ADDED Requirements (Phase 2 — 登记，本 PR 不实现)

### D7-S15-A52: LLM DecomposeProposer + ChildSpec Rationale + ExpectedReturn

When `SpawnPolicyEvaluator` returns `SpawnDecompose`, the orchestrator shall call `DecomposeProposer.Propose` **before** `ApplySpawnPolicy` / `DecomposeChildren` to produce `ChildSpec[]` with per-child `Directive`, `Kind`, `Rationale`, and **`ExpectedReturn`** (text constraint for upward rollup verification — CG4 aligned, no free JSON schema). The resulting `ChildSpec[]` shall be persisted on `parent.LastRound.ChildSpecs` for audit. LLM tools (`todo_write`, `free_fork`) shall not be the authoritative decomposition path.

#### Scenario: SpawnDecompose invokes DecomposeProposer before DecomposeChildren

- **Given** parent round ends with `SpawnPolicy=SpawnDecompose`
- **When** Decide phase runs
- **Then** `DecomposeProposer.Propose(ctx, parent)` shall be called **before** `ApplySpawnPolicy`
- **And** resulting `ChildSpec[]` shall be written to `parent.LastRound.ChildSpecs`
- **And** `DecomposeChildren` shall persist work items from the same `ChildSpec[]`

#### Scenario: ChildSpec ExpectedReturn is text constraint not JSON schema

- **Given** `DecomposeProposer` returns `ChildSpec{ ExpectedReturn: "Summary must list ≥3 P0 items with file paths" }`
- **When** child completes with `ArtifactSummary` containing three P0 entries with paths
- **Then** rollup directive may reference `ExpectedReturn` for synthesize guidance
- **And** `ExpectedReturn` shall be stored as plain text on `LastRound.ChildSpecs[]` (no JSON schema validation)

#### Scenario: FailureCriteria flows parent template to child Verify

- **Given** parent Plan with review-class `FailureCriteria` template
- **When** `DecomposeProposer` produces `ChildSpec.Directive` for child `C1`
- **Then** child `C1` Round 1 Plan shall inherit instantiated `FailureCriteria` from parent template
- **And** child Verify shall evaluate against that `FailureCriteria` (Phase 1 may still fallback to ExitCode with FC present on Plan)

---

### D7-S15-A62: RunParallelExplore → WaveScheduler

`RunParallelExplore` shall dispatch ephemeral probe WorkItems through `WaveScheduler` when `SpawnPolicy=SpawnParallelExplore`, producing Wave schedule spans in trace.

#### Scenario: parallel explore emits Wave schedule span

- **Given** parent with `SpawnPolicy=SpawnParallelExplore` and ExplorationPlan with N>1 steps
- **When** Execute runs `RunParallelExplore`
- **Then** Wave scheduler shall run probes with concurrency ≤ configured slot limit
- **And** trace shall contain Wave schedule operation span (non-zero)

---

**Status:** S4_Development — R1 决议 2026-06-27 已冻结  

---

## R1 决议摘要（编码 SoT）

| ID | 决议 |
|----|------|
| OQ-1 | `WorkItem.NeedsRollup bool` 显式字段 |
| OQ-2 | Rollup 复用 `CommitmentPlan` + rollup FailureCriteria 模板 |
| OQ-3 | Phase 1 不禁止 free_fork |
| R1-V1 | IT：stub LLM；生产：`verifyRollupArtifact` heuristic（len≥500、P0/P1、planning 黑名单） |
| R1-V2 | Rollup 终局 Verdict 触发 Item Learn（覆盖 R1 Fail） |

---

## MODIFIED Requirements

### D7-S1 WorkItem status machine — I3-Rollup exception

**Previous:** Terminal states (`completed`, `failed`, `cancelled`) have no outgoing transitions; terminal items are `Locked=true`.  
**Modified:** When `NeedsRollup=true`, the WorkTree shall allow `ReopenForRollup` to transition `failed|completed → pending`, set `Locked=false`, and preserve `LastRound` history for audit.

#### Scenario: ReopenForRollup unlocks failed goal for rollup round

- **Given** goal WorkItem `G` with `Status=failed`, `Locked=true`, `NeedsRollup=true`
- **When** `ReopenForRollup(sessionID, G.ID)` is called
- **Then** `G.Status` shall become `pending` and `G.Locked` shall become `false`
- **And** `G.LastRound` history shall be preserved

---

### D7-S2-A06 (Turn Loop): Session closure semantics

**Previous:** TurnLoop exits when no open WorkItems; `complete` may emit empty Content.  
**Modified:** TurnLoop exit shall call `extractSessionDeliverable` and populate `complete.Content` from root post-rollup `LastRound.ArtifactSummary`. When `GetPipelineFocus` returns nil, TurnLoop shall invoke rollup fallback gates before exiting.

---

### D7-S15-A60 (addendum): Rollup verify heuristic (R1-V1-C)

Rollup rounds shall use `verifyRollupArtifact` in production: Pass when summary len ≥ 500, contains `P0` or `P1`, and does not match planning-meta denylist. Integration tests may use stub LLM output.

#### Scenario: verifyRollupArtifact passes structured review summary

- **Given** rollup artifact summary containing `P0` section and len ≥ 500 without denylist phrases
- **When** `verifyRollupArtifact` runs
- **Then** `VerdictKind` shall be `pass`

#### Scenario: verifyRollupArtifact fails planning monologue

- **Given** rollup artifact summary containing denylist phrase `parallel explore`
- **When** `verifyRollupArtifact` runs
- **Then** `VerdictKind` shall be `fail`

---

### D7-S15 Learn routing (R1-V2)

When a WorkItem completes a rollup round with `NeedsRollup` cleared, `Learner.Learn` shall use the **rollup round Verdict**, not the prior failed Round 1 Verdict, for Reputation updates.

---

## Cross-References

- Design SoT: `workitem-pipeline-unification-design.md` G1, G5  
- Context SoT: `workitem-context-graph-design.md` §6 bubble levels  
- T Registry: `specs/d7-orchestration/t-registry_delta.md`
