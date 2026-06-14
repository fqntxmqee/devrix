# Delta Spec: Orchestration — Wave Scheduler

**Demand ID:** DM-20260611-007  
**Capability:** orchestration / wave-scheduler

## ADDED Requirements

### REQ-ORCH-WAVE-01: Worker Pool Quotas

The system SHALL maintain a fixed worker pool per session wave:

| Worker Type | Max Concurrent | Backend |
|-------------|----------------|---------|
| cursor | 1 | call_cursor agent tool |
| claude_code | 1 | call_claude-code agent tool |
| subagent | 3 | in-process SubQuery via LLM Gateway |

Total concurrent workers SHALL NOT exceed 5.

#### Scenario: Pool at capacity

- **GIVEN** 1 cursor, 1 claude_code, and 3 subagents are running
- **WHEN** a new ready task requires any worker type
- **THEN** the task SHALL remain queued until a slot of the required type is released

---

### REQ-ORCH-WAVE-02: Continuous Dispatch

The WaveScheduler SHALL dispatch ready tasks whenever a matching worker slot is available, without waiting for all tasks in the current batch to complete.

#### Scenario: Slot released triggers next dispatch

- **GIVEN** 6 subagent tasks are ready and 3 subagent slots are occupied
- **WHEN** one subagent task completes
- **THEN** the scheduler SHALL immediately attempt to start the next ready subagent task

---

### REQ-ORCH-WAVE-03: DAG Source from Plan Engine

Task DAGs SHALL be produced by the Plan Engine as the primary path, including dependencies, worker_type, and context_policy per node.

#### Scenario: Only ready nodes dispatched

- **GIVEN** a DAG where task B depends on task A
- **WHEN** A is not completed
- **THEN** B SHALL NOT be dispatched

---

### REQ-ORCH-WAVE-04: Context Policy

Each task node SHALL declare a context_policy:

| Policy | Behavior |
|--------|----------|
| fresh | No Leader message history; directive + supplemental system prompt only |
| resume | Resume SubAgent sidechain for the assigned agent_id |
| upstream | Include artifact/summary from dependency task; no full Leader history |

---

### REQ-ORCH-WAVE-05: Shared WorkDir Conflict Guard

Workers SHALL run in the same WorkDir as the Leader session. The scheduler SHALL prevent parallel execution of tasks that would write overlapping file scopes or share the same conflict_group.

#### Scenario: Same conflict group

- **GIVEN** two ready tasks with the same conflict_group
- **WHEN** one is already running
- **THEN** the other SHALL NOT start until the first completes

---

### REQ-ORCH-WAVE-06: IM Worker Card (Dual Block)

For each active worker task, the IM adapter SHALL render an independent card with two streaming blocks:

1. **Thinking** — thinking events
2. **Output** — text, tool progress, and completion summary

Cards SHALL NOT share buffers across workers.

#### Scenario: Five parallel workers

- **GIVEN** 5 workers are running for the same session
- **WHEN** each emits thinking and text events tagged with task_id
- **THEN** the user SHALL see 5 distinct cards updating independently

---

### REQ-ORCH-WAVE-07: Wave Completion Callback

When all tasks in a wave reach a terminal state, the scheduler SHALL notify the Leader with a wave_completed event including per-task artifacts.

---

### REQ-ORCH-WAVE-08: Worker Cancel and Slot Release

The WaveScheduler SHALL support cancelling individual workers and all workers for a session. Cancellation SHALL:

1. Propagate to the worker runtime (context cancel for SubAgent; process terminate for CLI agents)
2. Transition the task to `cancelled` terminal state
3. Release the worker pool slot within 30 seconds
4. Update the IM worker card with a cancelled footer

Cancel MAY be triggered by `task_stop` (DM-009), session reset (`/new`), or `CancelAll(sessionID)`.

#### Scenario: Cancel single running worker

- **GIVEN** a subagent worker is running and holding a slot
- **WHEN** `CancelWorker(task_id)` is invoked
- **THEN** the worker SHALL stop, the slot SHALL be released, and status SHALL be `cancelled`

#### Scenario: Session reset cancels all workers

- **GIVEN** 3 workers are running for session S
- **WHEN** `CancelAll(S)` is invoked
- **THEN** all 3 workers SHALL reach terminal state and all slots SHALL be released
