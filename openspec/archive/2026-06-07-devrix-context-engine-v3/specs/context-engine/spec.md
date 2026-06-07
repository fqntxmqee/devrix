# Delta: Context Engine V3

**Change ID:** devrix-context-engine-v3
**Base Spec:** `openspec/specs/context-engine/spec.md` v2.0.0
**Target Version:** 3.0.0
**Demand:** DM-20260607-006

---

## ADDED Requirements

### Requirement: PEV Plan Phase

系统 MUST 在 `plan.enabled=true` 时支持 PEV Plan 阶段，将用户意图分解为 Milestone DAG。

**Priority**: P0
**L4**: L4-CTX-PLAN, L4-CTX-PEV

#### Scenario: Plan generates milestone DAG

- GIVEN `plan.enabled=true` and user message requires multi-step work
- WHEN PEVEngine enters Plan phase
- THEN LLM produces structured milestone JSON
- AND milestones are validated (id format, dependency refs, acyclic DAG)
- AND milestones are created via `IMilestonePlanner`

#### Scenario: Plan validation failure degrades to V2

- GIVEN Plan LLM output fails validation
- WHEN validation detects cycle or invalid refs
- THEN error `CTX_PLAN_4020` is logged
- AND execution continues as V2 Execute→Verify without DAG

#### Scenario: Plan disabled preserves V2 behavior

- GIVEN `plan.enabled=false`
- WHEN Process handles user message
- THEN Plan phase is skipped
- AND Execute→Verify loop runs as V2

---

### Requirement: Milestone-Driven Execution

系统 MUST 按 Milestone DAG 拓扑序驱动 Execute→Verify，并更新进度。

**Priority**: P0
**L4**: L4-CTX-PEV

#### Scenario: Milestones execute in dependency order

- GIVEN a valid Milestone DAG for a task
- WHEN PEV runs after Plan
- THEN milestones execute in topological order
- AND blocked milestones wait until dependencies complete

#### Scenario: Milestone progress events emitted

- GIVEN milestone progress changes during PEV
- WHEN UpdateProgress or Complete is called
- THEN `milestone_progress` EngineEvent is emitted
- AND metadata includes `milestone_id`, `progress`, `task`

#### Scenario: Milestone verify failure marks failed

- GIVEN verify fails after max iterations for a milestone
- WHEN retry budget is exhausted
- THEN `IMilestonePlanner.Fail` is called with reason
- AND subsequent milestones are skipped or handled per config

---

### Requirement: Long-Term Memory

系统 MUST 提供 SQLite 持久化的跨 Session 长期记忆。

**Priority**: P0
**L4**: L4-CTX-MEMORY

#### Scenario: LongTerm recall injects context

- GIVEN `longterm.enabled=true` and entries exist for query topic
- WHEN Process starts
- THEN Recall returns matching entries
- AND recalled content is injected into context within token budget

#### Scenario: LongTerm store on completion

- GIVEN `longterm.auto_store=true` and topic in whitelist
- WHEN Process completes successfully
- THEN a memory entry is persisted to SQLite

#### Scenario: LongTerm disabled returns not implemented

- GIVEN `longterm.enabled=false`
- WHEN Recall is called
- THEN `CTX_MEMORY_4005` FeatureNotImplemented is returned

---

### Requirement: IMilestonePlanner Contract

Layer 2 MUST depend on `shared/contracts.IMilestonePlanner` rather than Communication layer internals.

**Priority**: P0
**L4**: L4-CTX-PLAN

#### Scenario: Context engine uses planner contract

- GIVEN context engine Plan phase
- WHEN milestones are created or updated
- THEN only `IMilestonePlanner` interface methods are invoked
- AND communication `milestone` package is not imported by L2

---

## MODIFIED Requirements

### Requirement: PEV Engine

PEV Engine MUST support three phases when Plan is enabled: Plan → Execute → Verify.

**Priority**: P0

#### Scenario: Full PEV loop

- GIVEN `plan.enabled=true` and shouldPlan returns true
- WHEN PEVEngine runs
- THEN phases proceed Plan → Execute → Verify per milestone
- AND `types.PEVPhase` includes `plan`

#### Scenario: PEV observer plan events

- GIVEN IPEVObserver is wired
- WHEN Plan completes or milestone progress updates
- THEN `EmitPlanCompleted` and `EmitMilestoneProgress` are invoked

---

### Requirement: Task flow milestone progress (V3 ready)

- GIVEN milestone progress update from PEV Plan/Execute
- WHEN milestone state changes
- THEN `milestone_progress` events are emitted (no longer V3-ready placeholder)
- AND metadata includes milestone_id and progress

---

## REMOVED Requirements

_None._
