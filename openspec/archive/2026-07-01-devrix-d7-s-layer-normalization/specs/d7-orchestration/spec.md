# D7 Orchestration Delta — S Layer Normalization

**Change ID:** `devrix-d7-s-layer-normalization`  
**Demand ID:** DM-20260701-002  
**Status:** Proposed / Implemented in source specs during S4

---

## ADDED Requirements

### Requirement: D7 current canonical S MUST be S1-S6 only

D7 current source-of-truth documents SHALL present exactly these canonical S layers:

- D7-S1 Work Model
- D7-S2 Session Orchestrator
- D7-S3 Wave Scheduler
- D7-S4 Execution Flow
- D7-S5 Decision & Planning
- D7-S6 MUPS Governance

Historical S identifiers such as D7-S7-S14 and contract identifiers such as D7-S20/S21 SHALL NOT be presented as current canonical S layers.

#### Scenario: Current S list is canonical only

- GIVEN a reader opens `spec.md`, `a-registry.md`, or `code-layout.md`
- WHEN they inspect the current D7 S-layer table
- THEN it SHALL list only D7-S1 through D7-S6
- AND S7-S14/S20/S21 SHALL appear only in historical or contract mapping sections.

### Requirement: MUPS five-node pipeline MUST be modeled as A/F chain

Observe, Plan, Execute, Verify, and Learn SHALL be represented as A/F activities under D7-S5 and D7-S6, not as independent S layers.

#### Scenario: MUPS nodes map to activities

- GIVEN a developer traces WorkTree+MUPS
- WHEN they inspect the A registry
- THEN Observe and Plan SHALL map to D7-S5 activities
- AND Execute, Verify, Learn, Escape, and convergence governance SHALL map to D7-S6 activities.

### Requirement: TaskSpec and TaskReport MUST be contract assets

TaskSpec and TaskReport SHALL be documented as downlink/uplink contracts, with canonical targets in D7-S1 and D7-S6, not as D7-S20/S21 scenarios.

#### Scenario: Task contracts do not create new S layers

- GIVEN a reader inspects TaskSpec / TaskReport documentation
- WHEN they look for canonical S ownership
- THEN the contract section SHALL point to D7-S1 and D7-S6
- AND no current canonical S20/S21 SHALL be listed.

### Requirement: StrategicPlanReject MUST feedback into next planning attempt

When the strategic proposer rejects an over-budget proposal, D7 SHALL persist a structured rejection rationale in the current WorkItem round so the next inline attempt can include it as prior feedback.

#### Scenario: Over-budget proposal is visible to next prompt

- GIVEN a strategic proposer returns `StrategicPlanReject`
- WHEN ItemPipeline continues the round
- THEN the round SHALL contain a rejection rationale
- AND the next retry prompt SHALL include that rationale through `PriorVerifyReason`.

### Requirement: Parent uncertainty reevaluation MUST use child-stats signal

Parent reevaluation after child terminal events SHALL derive the current round signal from child outcome stats, not from the previously stored parent uncertainty.

#### Scenario: All-pass children reduce parent uncertainty

- GIVEN a parent has high stored uncertainty
- AND all children are terminal completed
- WHEN `ReevaluateParentAfterChild` recomputes uncertainty
- THEN the stored uncertainty SHALL drop according to child-stats-driven convergence.

---

## MODIFIED Requirements

### Requirement: D7 registry paths MUST reflect current runtime

A/F registry code locations SHALL point to current runtime paths (`sessionorchestrator/`, `workmodel/`, `mups/learn/`, `interfaces/`) rather than retired `observe/`, `execute/`, `verify/`, or old ingress paths.
