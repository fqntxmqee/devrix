# D7 Orchestration — Uncertainty Spawn Contract (CC-U1～U6)

**Status:** Active  
**Demand ID:** DM-20260704-001  
**Change ID:** `d7-uncertainty-spawn-decouple`  
**Affects:** SpawnPolicy, RollupGate, StrategicPlan, Observe signals, Session complete  
**Design SoT:** `openspec/archive/2026-07-04-d7-uncertainty-spawn-decouple/design.md`  
**Depends on:** CC-1～CC-5 (`pipeline-architecture.md` §4.1)

---

## ADDED

### Requirement: CC-U1 Uncertainty-Primary Spawn Continuation

The SpawnPolicyEvaluator MUST treat uncertainty, child topology, and evidence progress as the primary signals for choosing among `SpawnInline`, `SpawnDecompose`, rollup synthesis, and `SpawnEscalateHuman`. Deliverable format incompleteness alone MUST NOT be the sole trigger for consecutive inline retries at depth 0 when evidence progress indicates exploration is sufficient and uncertainty is below the rollup synthesis threshold.

#### Scenario: Format failure after sufficient exploration at depth 0
- GIVEN a WorkItem at depth 0 with no running children
- AND the Execute artifact records `tool_calls >= 2` covering paths in `ScopeContract.InScope`
- AND `DeliverableStatus == incomplete` with reason in `{planning_meta, findings_json_incomplete, findings_json_required, partial deliverable}`
- AND `UncertaintyMean < RollupSynthThreshold` (default 0.50)
- WHEN `SpawnPolicyEvaluator` runs after Verify
- THEN the evaluator MUST NOT select `SpawnInline` solely because `DeliverableContinuationRequired` is true
- AND MUST trigger rollup synthesis per CC-U3 or mark the WorkItem for rollup before exhausting inline retry budget
- AND MUST NOT escalate to human review solely for deliverable format at depth 0

#### Scenario: Evidence insufficient — exploration continues
- GIVEN `EvidenceProgress < EvidenceSufficientThreshold`
- OR `UncertaintyMean >= DefaultUncertaintyDecomposeThreshold` (default 0.60)
- WHEN Verify returns Partial with deliverable incomplete
- THEN existing exploratory spawn rules (decompose / inline per DM-20260703-001) MUST apply

---

### Requirement: CC-U2 Deliverable Gate Presentation Layer

DeliverableContract verification MUST remain responsible for structured extraction and user-facing presentation quality, but MUST NOT be the sole spawn continuation driver when CC-U1 rollup synthesis conditions are met.

#### Scenario: Terminalization unchanged
- GIVEN `DeliverableStatus == complete` for an applicable contract
- WHEN `SpawnPolicyEvaluator` runs
- THEN `SpawnPolicy == none` per CC-1.1 (DM-20260703-001)

#### Scenario: Session complete salvage
- GIVEN rollup synthesis or final round leaves `DeliverableStatus == incomplete`
- AND salvageable content exists in the artifact or structured payload
- WHEN `buildSessionCompleteEvent` runs
- THEN `ExtractSessionDeliverable` MUST attempt best-effort salvage before emitting `task_incomplete`

---

### Requirement: CC-U3 Rollup Synthesis on Format Failure

When CC-U1 conditions are met and deliverable verification fails for format or synthesis reasons, the system MUST enter a convergence-phase rollup synthesis round instead of repeating the same leaf inline execution.

#### Scenario: Depth-0 goal without decomposed children
- GIVEN a Goal WorkItem at depth 0 with no non-terminal children
- AND CC-U1 conditions are met
- WHEN deliverable verification fails
- THEN the system MUST set `NeedsRollup=true` on the WorkItem OR create an ephemeral synthesis path equivalent to rollup mode
- AND the next Execute MUST use rollup synthesis materialize policy (`ModeRollupSynth`) with prior tool evidence

#### Scenario: Decompose parent with terminal children
- GIVEN a parent WorkItem whose last spawn policy was decompose or await
- AND all direct non-checklist children are terminal
- WHEN the parent round has deliverable incomplete but CC-U1 evidence is sufficient
- THEN CC-1.3 rollup gate (DM-20260703-001) MUST apply in preference to inline retry

---

### Requirement: CC-U4 Strategic Single Mode Uncertainty Gate

Strategic Plan proposals with `execution_mode=single` MUST be rejected or coerced when uncertainty remains above the single-mode threshold.

#### Scenario: High uncertainty rejects single
- GIVEN `UncertaintyMean >= SingleModeThreshold` (default 0.45)
- WHEN the Strategic Plan proposer returns `execution_mode=single`
- THEN the proposal MUST be rejected with a structured reason OR coerced to decompose
- AND the subsequent DefaultPlanner input MUST allow exploratory spawn rules

---

### Requirement: CC-U5 Observe Verify Signals

The per-WorkItem Observe phase MUST emit structured observation signals derived from the previous round's deliverable and evidence metadata without requiring an LLM call.

#### Scenario: Deliverable incomplete signal
- GIVEN `item.LastRound.DeliverableStatus == incomplete`
- WHEN `observeWorkItem` runs
- THEN at least one `ObsSignal` MUST record deliverable incompleteness and the verify reason category
- AND tool call count from the prior artifact MUST be available to uncertainty reconciliation

---

### Requirement: CC-U6 Accurate Spawn Rationale Labels

Spawn rationale strings MUST accurately identify the spawn rule that fired.

#### Scenario: Inline retry budget at depth 0
- GIVEN escalation caused by `InlineRetriesAtMaxDepth >= MaxInlineRetriesAtMaxDepth` with deliverable continuation
- WHEN `spawnRationale` is computed for `SpawnEscalateHuman`
- THEN the rationale MUST reference CC-1.2 or inline retry exhaustion
- AND MUST NOT label the event as R7 indeterminate exhaustion unless `VerdictKind == indeterminate`

---

## MODIFIED

### Requirement: SpawnPolicy Partial Branch Ordering

The Partial verdict branch in SpawnPolicyEvaluator MUST evaluate CC-U1 rollup synthesis eligibility before deliverable-driven inline continuation.

#### Scenario: CC-U1 precedes deliverable inline
- GIVEN `VerdictKind == partial` and `DeliverableContinuationRequired == true`
- AND CC-U1 rollup synthesis conditions are met
- WHEN spawn policy is evaluated
- THEN rollup synthesis MUST be chosen before incrementing inline retry counters for format-only failure

---

### Requirement: Strategic Plan Execution Mode Defaults

Strategic Plan `execution_mode=single` MUST be treated as a narrowing proposal subject to uncertainty gating, not as an unconditional topology lock.

#### Scenario: Single mode does not bypass decompose when U is high
- GIVEN high uncertainty per Observe
- WHEN strategic plan returns single mode
- THEN child decomposition MUST remain available on subsequent Partial or Indeterminate rounds

---

## Non-Goals (Explicit)

- Task-type-specific spawn rules (review, edit, test, etc.)
- LLM tactical prose in Execute directives
- Replacing DeliverableContract verification entirely
