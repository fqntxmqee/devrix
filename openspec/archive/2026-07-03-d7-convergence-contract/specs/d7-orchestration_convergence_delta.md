# Delta: D7 Orchestration — Convergence Contract

**Change ID:** `d7-convergence-contract`  
**Affects:** SpawnPolicy, ItemPipeline terminalization, RollupGate, Scope validation, Session exit  
**Design SoT:** [`../design.md`](../design.md)

---

## ADDED

### Requirement: CC-1 Round Terminalization Gate

When a MUPS round completes Verify with no deliverable continuation obligation, the system MUST assign SpawnNone and transition the WorkItem to a terminal TaskStatus consistent with `StatusAfterSpawnNone`.

#### Scenario: Deliverable complete at max depth
- GIVEN a WorkItem at `Depth >= MaxDecomposeDepth`
- AND `DeliverableStatus == complete` for an applicable schema
- WHEN `SpawnPolicyEvaluator` runs
- THEN `SpawnPolicy == none`
- AND `TaskStatus` becomes `completed` (or `failed` if verdict fail with complete deliverable per existing rules)
- AND the WorkItem MUST NOT be selected again by `GetPipelineFocus` for inline retry

#### Scenario: Max depth inline retry budget exhausted
- GIVEN a WorkItem at max depth with `DeliverableContinuationRequired == true`
- AND `InlineRetriesAtMaxDepth >= MaxInlineRetriesAtMaxDepth`
- WHEN `SpawnPolicyEvaluator` runs
- THEN `SpawnPolicy` is `escalate_human` OR the WorkItem becomes `failed` with a recorded `TerminalReason`
- AND a decompose parent with `RollupGateBestEffort` MAY proceed to rollup when all siblings are terminal

---

### Requirement: CC-2 Scope Validation Before Decompose

Before `DecomposeChildren` creates persistent child WorkItems, each proposed `ScopeIn` path MUST be validated against the session work directory.

#### Scenario: Reject non-existent paths
- GIVEN a StrategicPlan or DefaultDecomposeProposer emits `ScopeIn` containing paths that do not exist under the session WorkDir
- WHEN `PrepareDecomposeSpecs` runs
- THEN those paths are rejected
- AND the system falls back to valid paths or `DefaultDecomposeProposer` structural split
- AND MUST NOT spawn parallel siblings with identical or overlapping invalid scopes

#### Scenario: Monotonic scope narrowing
- GIVEN a parent WorkItem with `ScopeContract.InScope`
- WHEN child specs are validated
- THEN each child `ScopeIn` MUST remain a subset of the parent scope (existing `ValidateChildScopes` semantics preserved)

---

### Requirement: CC-3 Upward Rollup on All Decompose Parents

When all direct non-checklist children of a decompose parent reach terminal TaskStatus, the parent MUST receive `NeedsRollup=true` and execute at least one rollup MUPS round before the session completes.

#### Scenario: Four-level decompose chain
- GIVEN a Goal → Implement → Implement → Implement tree where each level decomposed once
- AND the deepest leaf completes with a valid deliverable
- WHEN intermediate parents' children all reach terminal status
- THEN each decompose parent MUST trigger rollup in bottom-up order
- AND the root Goal MUST expose a non-empty `ExtractSessionDeliverable`

#### Scenario: Sibling best-effort when one completes and one stuck
- GIVEN two sibling Implement WorkItems under the same parent
- AND one sibling is terminal with `deliverable complete`
- AND the other exhausted max-depth inline retries with deliverable incomplete
- WHEN `MaybeSiblingBestEffortRollup` runs
- THEN the stuck sibling is marked failed with reason `inline_retries_exhausted_at_max_depth`
- AND the parent receives `NeedsRollup=true` under `RollupGateBestEffort`

---

### Requirement: CC-4 Session Exit Boundedness

The session turn loop MUST terminate within a bounded number of MUPS rounds for any work tree topology, via natural completion, escalation, anomaly exit, or explicit best-effort completion.

#### Scenario: No infinite inline at max depth
- GIVEN only open WorkItems are max-depth leaves on inline retry with incomplete deliverable
- WHEN inline retry budget is exhausted
- THEN the session loop MUST NOT schedule unbounded additional MUPS rounds on those items
- AND MUST proceed to rollup fallback or session exit with user-visible outcome

---

## MODIFIED

### Requirement: SpawnPolicy Rule Ordering (R0.5)

The SpawnPolicyEvaluator MUST evaluate deliverable continuation before max-depth inline forcing (R1).

#### Scenario: R0.5 precedes R1
- GIVEN `DeliverableContinuationRequired(round) == false`
- WHEN `Depth >= MaxDepth`
- THEN the evaluator MUST NOT return `SpawnInline` solely due to R1
- AND MUST return `SpawnNone` per CC-1

---

### Requirement: MaybeDecomposeParentRollup Scope

The emergency rollup trigger MUST apply to any WorkItem that previously spawned with decompose/await policy and has all direct children terminal, not only root Goal WorkItems.

#### Scenario: Intermediate implement parent
- GIVEN an Implement WorkItem at depth 2 with `LastRound.SpawnPolicy == decompose`
- AND all direct children terminal
- WHEN session focus is nil and rollup has not yet run
- THEN the system MUST set `NeedsRollup` on that Implement WorkItem and schedule it for focus

---

### Requirement: Session Stagnation Detection

Session-level stagnation detection MUST consider transitive descendant stuck state, not only direct children of `await_child` parents.

#### Scenario: Grandchild inline stuck
- GIVEN a parent in `await_child` whose direct child is also `await_child`
- AND all non-terminal descendants are inline-stuck with deliverable continuation required
- WHEN `EvaluateSessionExit` runs
- THEN the session MUST NOT treat the tree as having forward progress indefinitely

---

## REMOVED

(None — behavioral tightening only; PR #379 session loop cap removal remains, complemented by CC-1/CC-4.)

---

## Traceability

| Requirement | Code target (planned) | Test |
|-------------|----------------------|------|
| CC-1 | `spawn_policy.go`, `terminalize.go` | T1, T2 |
| CC-2 | `scope_validator.go` | T5 |
| CC-3 | `rollup_gate.go`, `resolve.go` | T3, T4 |
| CC-4 | `session_loop_signals.go` | T3, T7 |
| R0.5 ordering | `spawn_policy.go` | T1 |
| Parent rollup scope | `rollup_gate.go` | T4 |
| Subtree stagnation | `session_loop_signals.go` | T3 |
