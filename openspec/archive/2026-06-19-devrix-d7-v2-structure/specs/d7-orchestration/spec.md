# D7 Orchestration — Spec Delta (devrix-d7-v2-structure)

**Change ID:** devrix-d7-v2-structure  
**Demand ID:** DM-20260619-005  
**Base:** `openspec/specs/d7-orchestration/spec.md` v3.8.0

---

## MODIFIED: Physical Path Registration

D7 S-layer code locations MUST align with `architecture/code-layout.md` §4.2 scenario-slug paths.

#### Scenario: S2 Session Orchestrator at sessionorchestrator slug

- GIVEN `devrix-d7-v2-structure` Phase B3 is merged
- WHEN a developer locates D7-S2-A01 ProcessMessage
- THEN the implementation lives under `internal/layers/orchestration/sessionorchestrator/`
- AND `coordinator/` contains only deprecated type aliases

<!-- T: D7-S2-A01-T01 -->

#### Scenario: S5 Decision at decisionplanning slug

- GIVEN Phase B3 is merged
- WHEN a developer locates D7-S5-A02 SynthesizeTaskGraph
- THEN the implementation lives under `internal/layers/orchestration/decisionplanning/`
- AND S5 does NOT own ProcessMessage ingress

<!-- T: D7-S5-A02-T01 -->

#### Scenario: S3 Wave at wavescheduler slug

- GIVEN Phase B1 is merged
- WHEN WaveScheduler is imported
- THEN the package path is `orchestration/wavescheduler`
- AND `orchestration/wave` does not exist (or is shim only)

<!-- T: D7-S3-T01 -->

#### Scenario: S4 Execution Flow at executionflow slug

- GIVEN Phase B2 is merged
- WHEN ExecutionFlowHub is imported
- THEN hub lives at `orchestration/executionflow/hub/`
- AND workplan and imsink are subdirectories of executionflow

<!-- T: D7-S4-A01-T01 -->

---

## MODIFIED: hubspoke Physical Split

D7-S2-A04 DispatchWorker and D7-S4-A05 SpokeBridge MUST NOT share the same Go package after Phase B4.

#### Scenario: Dispatch belongs to S2

- GIVEN Phase B4 is merged
- WHEN Dispatcher.DispatchWorker is located
- THEN it lives under `sessionorchestrator/dispatch.go`

#### Scenario: Bridge belongs to S4

- GIVEN Phase B4 is merged
- WHEN SpokeBridge is located
- THEN it lives under `executionflow/bridge.go`

---

## ADDED: WorkTree Single Semantic SoT (Phase C)

Wave scheduling MUST read WorkTree as the semantic source of truth; TaskNode is a dispatch projection only.

**Package:** `orchestration/wavescheduler/` + `orchestration/workmodel/`
**TD:** TD-WT-02

#### Scenario: TaskNode is projection not SoT

- GIVEN a WorkTree with implement children and BlockedBy edges
- WHEN WaveScheduler.Start is called
- THEN TaskGraph nodes are derived from WorkTree via SyncWaveNodes
- AND TaskNode is not independently persisted to disk

<!-- T: D7-S1-T07 -->

#### Scenario: sc.Todos is read projection only

- GIVEN todo_write updates WorkTree checklist
- WHEN D2 prepare reads session todos
- THEN sc.Todos is populated as read projection from WorkTree
- AND no code path writes sc.Todos as authoritative SoT

**TD:** TD-WT-03

---

## MODIFIED: Documentation Status

The following documents MUST reflect IMPLEMENTED status after Phase A:

| Document | Required state |
|----------|----------------|
| `design.md` | S1–S5 IMPLEMENTED; no stale PLANNED for ProcessMessage/Classify |
| `layer-delta.md` | §v2.0-Structure added |
| `d7-boundary.md` | Task/PlanMode ✅ D7-S1/S5 |
| `code-layout.md` §4.2 | 5/5 S layers ✅ |

---

## UNCHANGED

- North Star (`d7-domain.md`)
- D-S-A-F-T ID numbering
- 66 T point IDs and acceptance contracts
- `IOrchestrationEntry` public contract shape
- D2 scope (no engine changes)
