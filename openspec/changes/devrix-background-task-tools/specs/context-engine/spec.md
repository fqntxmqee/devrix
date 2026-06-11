# Delta Spec: Context Engine — Background Task Tools

**Demand ID:** DM-20260611-009  
**Capability:** context-engine / background-tasks

## ADDED Requirements

### REQ-CTX-BG-01: task_stop

The system SHALL provide an LLM tool `task_stop` that cancels a running background SubQuery by `task_id` (prefix `bg_`).

#### Scenario: Stop running background task

- **GIVEN** a background SubQuery with task_id `bg_abc` is running
- **WHEN** the Leader invokes `task_stop` with `task_id=bg_abc`
- **THEN** the SubQuery context SHALL be cancelled and registry status SHALL become `cancelled`

---

### REQ-CTX-BG-02: task_output

The system SHALL provide an LLM tool `task_output` that returns background task status and output.

Parameters:

- `task_id` (required)
- `block` (default true) — wait for terminal state
- `timeout_ms` (default 30000, max 600000)

#### Scenario: Non-blocking status check

- **GIVEN** a background task is still running
- **WHEN** `task_output` is called with `block=false`
- **THEN** the tool SHALL return immediately with `status=running` and any partial output

#### Scenario: Blocking wait with timeout

- **GIVEN** a background task completes within 5 seconds
- **WHEN** `task_output` is called with `block=true` and `timeout_ms=30000`
- **THEN** the tool SHALL return `status=completed` and the final output

---

### REQ-CTX-BG-03: Cancel Protocol Reuse

The background task cancel mechanism (context.CancelFunc registry) SHALL be reusable by Wave Scheduler worker cancellation (DM-007 REQ-ORCH-WAVE-08).

#### Scenario: Wave worker uses same cancel registry

- **GIVEN** a Wave SubAgent worker is registered with a cancel function
- **WHEN** `CancelWorker(task_id)` is invoked
- **THEN** the same cancel path as `task_stop` SHALL be used
