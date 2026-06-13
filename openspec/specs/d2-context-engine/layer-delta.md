# Delta: Domain D2 (CTX)

**Change ID:** devrix-foundation → devrix-context-engine (archived) → devrix-queryloop-context (archived) → devrix-unified-task-registry (archived)
**Canonical spec:** `openspec/specs/d2-context-engine/spec.md` (v7.1.0)
**Status:** Merged — reflects production path as of 2026-06-13
**Affects:** QueryLoop runtime, compression pipeline, layered memory, harness bootstrap (legacy fallback), conversation repair, main transcript

---

## ADDED

### Requirement: QueryLoop Primary Runtime

When `context_engine.query_loop.enabled=true` (default since DM-20260611-004), `ContextEngine.Process` MUST route all LLM↔Tool rounds through `query.Loop.Run` instead of the retired PEV engine.

#### Scenario: Multi-turn tool loop
- GIVEN `query_loop.enabled=true` and LLM returns tool_use until final text
- WHEN Process runs
- THEN Loop continues until no pending tool calls or `max_turns` reached
- AND tool results are ordered in transcript

#### Scenario: Per-turn compression
- GIVEN `query_loop.compress_per_turn=true` (default)
- WHEN Process starts
- THEN entry compression is skipped (`skipEntryCompress`)
- AND `commitActiveWindow` runs messages-only seven-step pipeline after each successful turn when budget exceeded

#### Scenario: Deferred complete event
- GIVEN QueryLoop completes successfully
- WHEN snapshot and `sc.Messages` are persisted
- THEN `complete` EngineEvent is emitted only after durable writes
- AND Metadata includes `duration`, `usage`, `model`, optional `ctx_pct`

### Requirement: Conversation Integrity

#### Scenario: Repair tool message chain before LLM
- GIVEN snapshot messages contain orphan tool results or incomplete tool rounds
- WHEN Process prepares API messages
- THEN `conversation.RepairToolMessageChain` removes invalid pairs
- AND provider APIs receive valid assistant/tool_call_id sequences

#### Scenario: Compact boundary preserves tail
- GIVEN prior compaction inserted a `compact_boundary` system marker
- WHEN messages are read for LLM or compression
- THEN only messages after the last boundary are used
- AND full history remains in snapshot for audit

### Requirement: Main Thread Transcript

When `context_engine.main_transcript.enabled=true`, full message deltas MUST append to `{base_dir}/{sessionId}/transcript.jsonl`.

#### Scenario: Append-only main transcript
- GIVEN main transcript enabled and Process completes
- WHEN assistant/tool messages are persisted
- THEN delta messages append to JSONL without replacing snapshot
- AND Worker overlay sessions do not write main transcript

### Requirement: Harness Bootstrap (Legacy Fallback)

Bootstrap stages run only when `harness.enabled=true` AND `query_loop.enabled=false` (explicit legacy path).

#### Scenario: Production default skips legacy harness path
- GIVEN default config (`query_loop.enabled=true`)
- WHEN Process runs
- THEN `obsruntime.PathQueryLoop` is recorded
- AND full bootstrap/preflight/routing harness branch is not taken on primary path

#### Scenario: Bootstrap when explicitly legacy
- GIVEN `query_loop.enabled=false` and `harness.enabled=true`
- WHEN first Process for session
- THEN prefetch → guards → setup → deferred_init → tool_pool stages run
- AND session is marked harness-initialized

### Requirement: System Prompt Assembly

Four-layer assembly via `prompt.SystemPromptAssembler` (not harness package).

#### Scenario: Build before QueryLoop
- GIVEN non-worker Process turn
- WHEN memory recall and optional preflight complete
- THEN assembler builds Layer 0–3 prompt into `sc.SystemPrompt`
- AND CompressedView system message equals assembler output

### Requirement: Seven-Step Compression Pipeline

Physical order: 1 → 2 → 3 → 4 → [6] → 5 → 7. Operates on **messages only** (system prompt excluded on QueryLoop path).

#### Scenario: Step 6 Autocompact async placeholder
- GIVEN async autocompact wired
- WHEN step 6 runs under budget pressure
- THEN placeholder returns within 50ms
- AND background goroutine may replace with LLM summary via `OnAutocompactComplete`

#### Scenario: Step 7 Token Block
- GIVEN compressed context still exceeds max budget
- WHEN TokenBlock runs
- THEN `ContextExceededError` is returned

### Requirement: Layered Memory

#### Scenario: Short-term snapshot with Snappy compression
- GIVEN snapshot compression enabled and payload exceeds threshold
- WHEN Serialize runs
- THEN output uses Snappy magic header `\xfe\x53`
- AND Deserialize accepts legacy raw JSON

#### Scenario: LongTerm recall and auto_store
- GIVEN `longterm.enabled=true`
- WHEN Process starts / completes
- THEN Recall injects entries into assembler Layer 3 within token budget
- AND optional auto_store persists SQLite entries on success

### Requirement: UserContext Prepend Boundary

#### Scenario: AGENTS.md not in snapshot
- GIVEN `user_context.mode=prepend`
- WHEN Loop builds API messages
- THEN prepend block appears in API call only
- AND snapshot `Messages` exclude prepend meta-user block

### Requirement: Permission Plan Mode

#### Scenario: Write restricted to plan file
- GIVEN `PermissionMode=plan`
- WHEN write tools target paths outside `PlanFilePath`
- THEN tool returns plan-mode denial without writing

### Requirement: SubQuery and Sidechain

#### Scenario: Sidechain JSONL resume
- GIVEN existing `{sessions}/{sessionId}/subagents/{agentId}.jsonl`
- WHEN SubQuery runs with `Resume=true`
- THEN initial messages include loaded sidechain history

### Requirement: Worktree Sandbox

#### Scenario: Isolated worker writes
- GIVEN worktree enabled
- WHEN Worker runs write tools
- THEN files are created under worktree path only
- AND primary session WorkDir is unchanged

---

## MODIFIED

### Requirement: IContextEngine Ownership

`ContextEngine` implements `contracts.IEngine` in `internal/layers/contextengine/engine.go`. Communication layer depends on interface only.

### Requirement: Gateway Event Contract

`complete` event Metadata MUST include millisecond `duration` and total token `usage` for gateway completion summary.

---

## REMOVED

| Item | Reason | Successor |
|------|--------|-----------|
| PEV Engine (Plan/Execute/Verify) | Retired 2026-06-13 | D2-S10 QueryLoop |
| PEVEngine.Run in Process hot path | DM-20260611-004 | `query.Loop.Run` |
| `harness/system_prompt_assembler.go` | Moved to `prompt/assembler.go` | `prompt.SystemPromptAssembler` |
| Entry compression on every Process when QueryLoop enabled | `compress_per_turn` defers to post-turn `commitActiveWindow` | D2-S11 harness unification |
