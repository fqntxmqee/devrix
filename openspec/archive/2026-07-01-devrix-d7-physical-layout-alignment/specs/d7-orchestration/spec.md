# D7 Orchestration Delta — Physical Layout Alignment

**Change ID:** `devrix-d7-physical-layout-alignment`
**Demand ID:** DM-20260701-004
**Status:** Proposed / Implemented in source specs during S4
**Parent Proposal:** `proposal.md`
**Parent Design:** `design.md`

---

## ADDED Requirements

### Requirement: D7 physical-layout-alignment change package MUST be complete

The OpenSpec change package for `devrix-d7-physical-layout-alignment` MUST contain `.openspec.yaml`, `demand.md`, `proposal.md`, `design.md`, `tasks.md`, and `specs/d7-orchestration/spec.md` (delta spec).

<!-- T: D7-PL-T01 -->

### Requirement: D7 a-registry Canonical section MUST enumerate every ValueFlow Activity

`a-registry.md` Canonical 段 SHALL list every Activity registered in its ValueFlow section (47 Activities + Hardening 2), with a resolvable Code Location per row.

#### Scenario: Canonical A row count matches ValueFlow commitment

- GIVEN a reader opens `a-registry.md`
- WHEN they count the `^### D7-S{N}` Canonical sections
- THEN it SHALL list 6 sections (D7-S1 through D7-S6)
- AND the total Activity row count SHALL be ≥ 47 + Hardening 2.

#### Scenario: Every Canonical A Code Location points to an existing file

- GIVEN the layout guard test runs
- WHEN it scans every `D7-S{N}-A{NN}` row's Code Location column
- THEN each path SHALL point to an existing `.go` file under `internal/layers/orchestration/`.

<!-- T: D7-PL-T02, D7-PL-T03 -->

### Requirement: D7 f-registry Canonical section MUST cover all 6 scenarios

`f-registry.md` Canonical 段 SHALL list Function rows for every S1-S6 Scenario that owns Activities, mirroring the A registry's 1-to-N F breakdown.

#### Scenario: Canonical F section covers S1 through S6

- GIVEN a reader opens `f-registry.md`
- WHEN they count `^### D7-S{N}` Canonical F sections
- THEN it SHALL list 6 sections (D7-S1 through D7-S6)
- AND no retired path (`orchestration/observe/`, `orchestration/execute/`, `orchestration/verify/`, `orchestration/learn/`) SHALL appear in the `Current Path` column; references in `Legacy Path` / `Historical Path` columns or in `historical-s-mapping.md` are out of scope.

<!-- T: D7-PL-T04 -->

### Requirement: code-layout.md MUST mirror the live `internal/layers/orchestration/` directory tree

`code-layout.md §4.2` SHALL contain a row for every directory present in `internal/layers/orchestration/`, and SHALL NOT list any directory that has been removed from the repository.

#### Scenario: Ghost directories are absent from code-layout.md

- GIVEN the layout guard test runs
- WHEN it greps `code-layout.md` for retired directory names
- THEN `coordinator/`, `hubspoke/`, `observe/`, `turn/` SHALL match 0 rows
- AND every directory listed in `ls internal/layers/orchestration/` SHALL have a corresponding row.

<!-- T: D7-PL-T05 -->

### Requirement: D7 layout guard MUST reject retired directory resurrection

`internal/layers/orchestration/layout/guard_test.go::TestNoResurrectRetiredDirs` SHALL FAIL if any retired directory reappears in `internal/layers/orchestration/`.

#### Scenario: Resurrecting `coordinator/` breaks CI

- GIVEN a developer accidentally runs `mkdir internal/layers/orchestration/coordinator`
- WHEN CI runs `go test ./internal/layers/orchestration/layout/...`
- THEN the test SHALL fail
- AND emit `ORCH_LAYOUT_6002` error code with the resurrected path.

#### Scenario: Resurrecting `observe/` breaks CI

- GIVEN a developer adds a file under `internal/layers/orchestration/observe/`
- WHEN CI runs the layout guard test
- THEN the test SHALL fail
- AND emit `ORCH_LAYOUT_6002`.

<!-- T: D7-PL-T06 -->

### Requirement: D7 layout guard MUST reject unregistered orphan directories

`internal/layers/orchestration/layout/guard_test.go::TestOrphanDirs` SHALL FAIL if any new directory appears under `internal/layers/orchestration/` without a matching row in `code-layout.md §4.2`.

#### Scenario: New orphan directory breaks CI

- GIVEN a developer adds `internal/layers/orchestration/foo/`
- WHEN CI runs the layout guard test
- THEN the test SHALL fail
- AND emit `ORCH_LAYOUT_6003`.

<!-- T: D7-PL-T06 -->

### Requirement: `plan/` MUST be registered under D7-S5 Decision & Planning

`plan/` SHALL be registered in `a-registry.md` and `code-layout.md §4.2` as a S5 Decision & Planning directory via doc-only dual registration (design §④ Q1 Decision — option B: 0 物理改动, 0 importer 改).

#### Scenario: plan/ appears in both registries

- GIVEN a reader inspects `a-registry.md`
- WHEN they search for `PlanValidate` or `PlanGenerate`
- THEN the Code Location SHALL reference `plan/` OR `decisionplanning/` (doc-only dual registration per design §④ Q1 Decision — option B: 0 物理改动, 0 importer 改)
- AND `code-layout.md §4.2` SHALL list `plan/` with S5 Decision & Planning ownership.

<!-- T: D7-PL-T07 -->

### Requirement: `orchtypes/` MUST be registered as D7 Cross-S Kernel

`orchtypes/` SHALL be registered in `a-registry.md`, `f-registry.md`, and `code-layout.md §4.2` as the cross-S governance kernel (types, sentinels, intent/observation primitives).

#### Scenario: orchtypes/ has explicit Cross-S ownership

- GIVEN a reader inspects `code-layout.md §4.2`
- WHEN they search for `orchtypes/`
- THEN the row SHALL declare Cross-S ownership (S5 intent + S6 types + S1-S6 共享)
- AND `d7-domain.md §North Star` SHALL list `Cross-S Kernel (orchtypes/)` alongside the 6 canonical Scenarios.

<!-- T: D7-PL-T08 -->

### Requirement: D7 Cross-S Kernel MUST enumerate 6 Activities in `orchtypes/`

`a-registry.md` SHALL list 6 Cross-S Kernel Activities (D7-X-A01..A06) under a dedicated `## D7 Cross-S Kernel (orchtypes/)` section, with Code Location per row pointing to existing `.go` files in `internal/layers/orchestration/orchtypes/`.

#### Scenario: Cross-S Kernel A row count matches design §④ Decision D8

- GIVEN a reader opens `a-registry.md`
- WHEN they count `^### D7-X-A` rows in the Cross-S Kernel section
- THEN it SHALL list 6 Activities (A01 OrchestrationTypes, A02 BoundaryDecisions, A03 AdaptivePrior, A04 AnomalyDetector, A05 ProcessConfig, A06 LLMInvoker)
- AND every Code Location SHALL resolve to an existing `.go` file under `internal/layers/orchestration/orchtypes/`.

#### Scenario: Cross-S Kernel F layer mirrors A registry

- GIVEN a reader opens `f-registry.md`
- WHEN they look for the `## D7 Cross-S Kernel F 层 (orchtypes/)` section
- THEN it SHALL enumerate Function rows for A01..A06 (or declare "F 层 deferred to v1.1 follow-up" with rationale).

<!-- T: D7-PL-T08, D7-PL-T12 -->

### Requirement: S6 MUPS Governance SHALL expose an activity-to-path matrix

`design.md §④` SHALL contain a table mapping every S6-governance Activity (D7-S6-A{NN} + Hardening-A{NN}) to a path from one of 3 categories: (a) 5 S6 overlay paths (`sessionorchestrator/`, `mups/execute/`, `mups/learn/`, `escape/`, `interfaces/`), (b) 2 Cross-S paths (`orchtypes/`, `executionflow/verify/` for shared verdict wiring), or (c) 1 cross-cutting path (`hardening/` + `wavescheduler/` for metrics+conflict). Total: 5+2+1=8 physical locations.

**Note (S5 carve-out)**: D7-S6-A03 (PlanValidate) and D7-S6-A04 (PlanGenerate) are S6-governance Activities that physically live in S5 paths (`decisionplanning/plan_mode.go` and `plan/planner.go` respectively) per DM-20260626-001 governance overlay decision. They are not part of the 8-path enumeration; their Code Location is recorded as an "S5 sub-registration" exception in the matrix.

#### Scenario: D7-S6-A14 (HardenMetricsAndConcurrency) resolves to a real path

- GIVEN a developer reads `design.md §④ S6 MUPS Governance matrix`
- WHEN they look up D7-S6-A14
- THEN the row SHALL name a path from one of 3 categories: (a) 5 S6 overlay paths (`sessionorchestrator/`, `mups/execute/`, `mups/learn/`, `escape/`, `interfaces/`), (b) 2 Cross-S paths (`orchtypes/`, `executionflow/verify/` for shared verdict wiring), or (c) a cross-cutting pair (`hardening/` + `wavescheduler/`, where A14's ConflictGuard owner is `wavescheduler/` and `hardening/` is the Discipline Keeper namespace for metrics)
- AND the path SHALL exist on disk.

<!-- T: D7-PL-T09 -->

### Requirement: hardening/ MUST map to Cross-cutting Discipline Keeper

`hardening/` SHALL be registered in `a-registry.md` Hardening 段 with explicit Cross-cutting ownership, mapping to D7-S6-A14 (HardenMetricsAndConcurrency) and Hardening-A01/A02.

#### Scenario: hardening/ activities link to S6 + Cross-cutting

- GIVEN a reader inspects `a-registry.md Hardening` section
- WHEN they read the Code Location column
- THEN each row SHALL reference `hardening/` package files AND/OR `wavescheduler/` (for cross-cutting ConflictGuard ownership, e.g. Hardening-A02 in `wavescheduler/conflict.go`)
- AND the row SHALL declare Cross-cutting Discipline Keeper ownership.

<!-- T: D7-PL-T10 -->

### Requirement: D7 layout guard MUST emit Span and Metric on every run

Every layout guard test execution SHALL emit a `D7_Code_Layout_Guard_Pass` or `D7_Code_Layout_Guard_Fail` Span, and increment `devrix_orch_layout_guard_failures_total` on failure.

#### Scenario: Layout guard failure emits Span + Metric

- GIVEN a layout guard test failure
- WHEN CI runs the test
- THEN a `D7_Code_Layout_Guard_Fail` Span SHALL be emitted with `reason` attribute
- AND `devrix_orch_layout_guard_failures_total{reason=...}` SHALL be incremented by 1.

<!-- T: D7-PL-T06, D7-PL-T11 -->

## MODIFIED Requirements

### Requirement: D7 registry paths MUST reflect current runtime

A/F registry code locations SHALL point to current runtime paths (`sessionorchestrator/`, `workmodel/`, `mups/learn/`, `interfaces/`, `decisionplanning/`) rather than retired `observe/`, `execute/`, `verify/`, or old ingress paths.

#### Scenario: Registry paths resolve to existing files

- GIVEN the layout guard test scans A/F registries
- WHEN it checks every Code Location against `internal/layers/orchestration/`
- THEN every path SHALL resolve to an existing `.go` file
- AND `ORCH_LAYOUT_6001` SHALL NOT be emitted for any current-path row.

<!-- T: D7-PL-T03, D7-PL-T04 -->

### Requirement: a-registry version MUST bump with each registry change

`a-registry.md` version SHALL bump from v5.3.0 to v5.4.0 in this change.

#### Scenario: a-registry version reflects the change

- GIVEN `a-registry.md` is updated to include 47+Hardening 2 Activity rows
- WHEN a reader inspects the version header
- THEN it SHALL read `Version: 5.4.0`
- AND `CHANGELOG.md` SHALL contain a one-line summary referencing `devrix-d7-physical-layout-alignment`.

<!-- T: D7-PL-T02 -->

### Requirement: f-registry version MUST bump with each registry change

`f-registry.md` version SHALL bump from v5.3.0 to v5.4.0 in this change.

#### Scenario: f-registry version reflects the change

- GIVEN `f-registry.md` is updated to cover S1-S6 canonical F sections
- WHEN a reader inspects the version header
- THEN it SHALL read `Version: 5.4.0`
- AND `CHANGELOG.md` SHALL contain a one-line summary referencing this change.

<!-- T: D7-PL-T04 -->

### Requirement: D7 acceptance report MUST be generated on PR-1..PR-4 completion

After all 4 PRs are squash auto-merged, `openspec/archive/2026-07-01-devrix-d7-physical-layout-alignment/acceptance-report.md` MUST record: 12 T points IMPLEMENTED, AC1-AC9 PASS verification, 22 orchestration packages `-race -count=1` PASS, and PR-1/2/3/4 commit hashes.

<!-- T: D7-PL-T12 -->

## REMOVED Requirements

(None)