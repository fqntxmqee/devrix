# Context Engine V6 Delta — QueryLoop Context

**Change:** devrix-queryloop-context (DM-20260610-001)  
**Base:** context-engine spec v5.0.0

## ADDED Requirements

### Requirement: QueryLoop Runtime

When `context_engine.query_loop.enabled=true`, PEV MUST delegate LLM↔Tool rounds to `query.Loop` instead of the legacy fixed-iteration execute loop. When `enabled=false`, behavior MUST remain bit-identical to V5.

**Priority:** P0  
**L3:** L3-BE-CTX-QueryLoop  
**L4:** query_loop

#### Scenario: Multi-turn tool loop

- GIVEN `query_loop.enabled=true` and LLM returns tool_use until final text
- WHEN Process runs
- THEN Loop continues until no pending tool calls or `max_turns` reached
- AND `TurnCount` reflects tool rounds executed

#### Scenario: V5 regression with query loop disabled

- GIVEN `query_loop.enabled=false`
- WHEN any V5 L5 scenario runs
- THEN behavior is unchanged from V5

---

### Requirement: UserContext Prepend Boundary

AGENTS.md and runtime user context MUST be injected via `usercontext.PrependForAPI` at the API boundary only when `user_context.mode=prepend`. They MUST NOT appear in persisted snapshot `Messages`.

**Priority:** P0  
**L4:** user_context

#### Scenario: Prepend not in snapshot

- GIVEN prepend mode and non-empty AGENTS.md
- WHEN Loop builds API messages
- THEN prepend block is present in API call only
- AND snapshot messages exclude the prepend meta-user block

---

### Requirement: Permission Plan Mode

Plan mode MUST restrict writable paths to `PlanFilePath` and filter ToolPool to read-only tools plus plan file writes.

**Priority:** P0  
**L4:** permission_mode

#### Scenario: Write denied outside plan file

- GIVEN `PermissionMode=plan` and configured plan file path
- WHEN `write_file` targets another path
- THEN tool returns plan mode denial without writing

---

### Requirement: Task Disk Persistence

When `tasks.mode=v2`, task_create/update/get/list MUST persist to `tasks.store_dir` and survive process restart.

**Priority:** P0  
**L4:** task_tools

---

### Requirement: SubQuery and Sidechain Transcript

SubQuery MUST run nested agents via the same `query.Loop` with incremented `QueryDepth` and optional `AgentID`. Sidechain transcript MUST append JSONL under `{sessions}/{sessionId}/subagents/{agentId}.jsonl` when enabled.

**Priority:** P1  
**L4:** subquery, sidechain_transcript

#### Scenario: Explore read-only sub-agent

- GIVEN builtin Explore agent invocation
- WHEN SubQuery runs
- THEN `OmitClaudeMd` applies and write tools are excluded

#### Scenario: Sidechain resume

- GIVEN existing sidechain JSONL for agentId
- WHEN SubQuery runs with `Resume=true`
- THEN initial messages include loaded sidechain history

---

### Requirement: Fork Subagent Cache Prefix

When `subquery.fork_subagent_enabled=true`, fork children MUST share identical assistant + placeholder tool_result prefixes; only the per-child directive differs.

**Priority:** P1  
**L4:** subquery

#### Scenario: Identical placeholder tool results

- GIVEN parent assistant message with multiple tool_use blocks
- WHEN two fork children are built with different directives
- THEN placeholder tool_result text is identical across children

---

### Requirement: Streaming Tool Execution

When `query_loop.streaming_tools=true`, concurrency-safe tools in the same batch MAY execute in parallel; results MUST remain ordered in the transcript.

**Priority:** P1  
**L4:** query_loop

---

### Requirement: Background Task Notifications

Background SubQuery completion MUST enqueue `task-notification` commands scoped to the sub-agent's `agentId`; Loop MUST drain matching notifications each iteration.

**Priority:** P1  
**L4:** background_tasks
