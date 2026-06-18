# Delta: D7 Orchestration — Unified Work Tree

**Change ID:** `devrix-unified-work-tree`  
**Demand ID:** DM-20260617-009  
**Affects:** D7-S1 Work Model, D7-S2 RunTurn, D7-S3 Wave, D7-S5 PlanMode, D2 todo_write/task tools

---

## ADDED

### Requirement: WorkItem Unified Work Unit Model

The orchestration layer SHALL represent all work semantics as `WorkItem` nodes in a per-session tree owned by D7 `WorkTree`.

Each WorkItem MUST have: `id`, optional `parent_id`, `kind`, `status`, `title`, `directive`, optional `uncertainty`, `policy`, dependency edges, optional `run_ref`, and `ephemeral` flag.

#### Scenario: Session root goal creation
- GIVEN a new session with no work items
- WHEN the first user message is processed
- THEN exactly one `kind=goal` root WorkItem exists with the user directive as `directive`

#### Scenario: Child work item under goal
- GIVEN an existing session goal WorkItem
- WHEN `delegate_implement` is invoked without an explicit work item id
- THEN a new `kind=implement` WorkItem is created with `parent_id` set to the goal id

#### Scenario: Checklist from todo_write
- GIVEN a focus WorkItem (goal or current implement item)
- WHEN `todo_write` receives a full todos snapshot
- THEN ephemeral `kind=checklist` child items under the focus are replaced to match the snapshot
- AND `sc.Todos` in session context reflects the same items for prepare prompts

---

### Requirement: WorkTree Persistence v2 with v1 Migration

Work items SHALL persist to disk when `context_engine.tasks.mode=v2` and `store_dir` is configured.

The on-disk format MUST use `schema_version: 2` with an `items` array.

#### Scenario: Load legacy v1 task file
- GIVEN a session file containing only legacy `tasks[]` flat Task JSON
- WHEN `DiskWorkItemStore.Load` is called
- THEN each Task is migrated to a `kind=implement` WorkItem with empty `parent_id`
- AND subsequent Save writes schema v2

#### Scenario: Round-trip v2 tree
- GIVEN a goal and child WorkItem saved to disk
- WHEN a new process loads the same session file
- THEN the parent-child relationship is preserved

---

### Requirement: WorkTree and RunRegistry Separation

Work semantics (What) and execution handles (How) MUST remain separate stores.

`WorkItem.run_ref` MAY link to a RunRegistry entry (DM-011).

#### Scenario: Spawn registers run reference
- GIVEN a WorkItem in `pending` status
- WHEN execution is spawned via delegate or wave worker
- THEN `run_ref` is set to the RunRegistry entry id
- AND WorkItem status transitions to `in_progress`

#### Scenario: Run terminal updates work item
- GIVEN a running WorkItem with a valid `run_ref`
- WHEN RunRegistry marks the run terminal
- THEN the WorkItem status updates to the terminal status
- AND a completion notification is published for the parent WorkItem if applicable

---

### Requirement: Legacy TaskManager Compatibility Adapter

`TaskManager` SHALL delegate to `WorkTree` internally while preserving the existing flat `Task` API for `task_create`, `task_get`, `task_list`, `task_update`, and `/task` CLI commands.

#### Scenario: Flat task_create unchanged for callers
- GIVEN code calling `TaskManager.Create(session, subject, description)`
- WHEN the call completes
- THEN a `Task` is returned with `subject` and `description` populated
- AND the underlying WorkItem exists in `WorkTree` with `kind=implement`

---

### Requirement: Wave Scheduler Reads WorkTree (v1.1)

Wave parallel dispatch SHALL source ready work from `WorkTree` subtrees rather than maintaining an independent persistent task graph.

#### Scenario: Wave decompose writes implement subtree
- GIVEN an orchestrate intent with a decomposed plan
- WHEN `SynthesizeTaskGraph` completes
- THEN implement WorkItems exist under a batch root with `policy=parallel_ok` and dependency edges
- AND WaveScheduler dispatches workers from ready items in that subtree

---

### Requirement: Unified Task Tool Surface (v2.0)

The LLM-facing task tools SHALL converge to four operations: `task_write`, `task_spawn`, `task_await`, and `task_list`.

Legacy tool names MUST remain as aliases for at least one release.

#### Scenario: task_write checklist mode
- GIVEN the LLM calls `task_write` with `mode=checklist` and a todos array
- THEN behavior is equivalent to legacy `todo_write`

#### Scenario: task_spawn explore mode
- GIVEN `multi_agent.delegate.enabled=true`
- WHEN the LLM calls `task_spawn` with `kind=explore`
- THEN behavior is equivalent to legacy `delegate_explore`

---

### Requirement: RunTurn Focus and Uncertainty Decompose (v2.0)

`DefaultOrchestrator.RunTurn` SHALL select a focus WorkItem via `WorkTree.GetFocus` and MAY decompose high-uncertainty items into child WorkItems before spawning execution.

#### Scenario: High uncertainty triggers decompose
- GIVEN a focus WorkItem with `uncertainty` above configured threshold and decomposable kind
- WHEN RunTurn resolves the focus item
- THEN child WorkItems are created under the focus
- AND execution awaits child completion before re-resolving the parent

---

## MODIFIED

### Requirement: D7-S1 Work Model Ownership

Work model storage ownership SHALL be exclusively D7 `orchestration/workmodel`.

D2 Context Engine MUST NOT own task semantics; it MAY host tool runners that read/write `WorkTree` via injected dependencies.

#### Scenario: No D2 TaskManager for new features
- GIVEN a new task-related feature
- WHEN implemented
- THEN it uses `WorkTree` or `TaskManager.Tree()` in D7
- AND does not introduce new flat task maps in D2

---

### Requirement: QueryWorkPlan Unified Read Model

`LocalWorkModel.QueryWorkPlan` SHALL return a tree-shaped work snapshot combining WorkTree items, RunRegistry background runs, and FlowHub execution flows.

#### Scenario: Query includes parent-child tasks
- GIVEN a session with a goal and two implement children
- WHEN `QueryWorkPlan` is called
- THEN the snapshot includes all three WorkItems with correct parent references

---

## REMOVED

(None in v1.0–v1.2. The following are targeted for v2.0 after alias period:)

- Independent persistent `wave.TaskGraph` as source of truth for work semantics
- Session-scratch `sc.Todos` as authoritative checklist store (demoted to read projection)
- Flat-only `TaskManager` internal map without WorkTree (replaced by delegation)

---

## Cross-Reference

| Related Change | Relationship |
|----------------|--------------|
| devrix-unified-task-registry (DM-011) | How layer; RunRegistry |
| devrix-task-planning | PlanMode writes WorkTree |
| devrix-wave-scheduler | Scheduler reads WorkTree |
| devrix-queryloop-legacy-decommission | RunTurn is canonical engine |
