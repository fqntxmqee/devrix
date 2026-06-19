# Delta: Domain D2 (CTX)

**Change ID:** devrix-foundation → devrix-queryloop-context (archived) → devrix-d2-queryloop-dismantle (archived)
**Canonical spec:** `openspec/specs/d2-context-engine/spec.md` (v8.0.0)
**Status:** Merged — D7 owns LLM↔Tool loop as of DM-20260618-010
**Affects:** D7 turn runtime, compression pipeline, layered memory, conversation repair, main transcript

---

## REMOVED (DM-20260618-010)

- `query.Loop.Run` and `query_loop.enabled` config
- Runtime label renamed: `PathD7Turn` (`d7_turn`) replaces `PathQueryLoop`

---

## ADDED

### Requirement: D7 Turn Primary Path

`ContextEngine.Process` MUST delegate LLM↔Tool rounds to D7 `PreparedTurnRunner` and record `obsruntime.PathD7Turn`.

#### Scenario: Multi-turn tool loop via D7
- GIVEN LLM returns tool_use until final text
- WHEN Process runs
- THEN D7 RunTurn continues until no pending tool calls or `max_turns` reached

#### Scenario: Per-turn compression
- GIVEN `query_loop.compress_per_turn=true` (default)
- WHEN Process starts
- THEN entry compression is skipped (`skipEntryCompress`)
- AND `commitActiveWindow` runs messages-only seven-step pipeline after each successful turn when budget exceeded

#### Scenario: Deferred complete event
- GIVEN D7 turn completes successfully
- WHEN snapshot and `sc.Messages` are persisted
- THEN `complete` EngineEvent is emitted only after durable writes

### Requirement: Conversation Integrity

#### Scenario: Repair tool message chain before LLM
- GIVEN snapshot messages contain orphan tool results or incomplete tool rounds
- WHEN Process prepares API messages
- THEN `conversation.RepairToolMessageChain` removes invalid pairs

#### Scenario: Compact boundary preserves tail
- GIVEN prior compaction inserted a `compact_boundary` system marker
- WHEN messages are read for LLM or compression
- THEN only messages after the last boundary are used

### Requirement: Main Thread Transcript

When `context_engine.main_transcript.enabled=true`, full message deltas MUST append to `{base_dir}/{sessionId}/transcript.jsonl`.

### Requirement: System Prompt Assembly

Four-layer assembly via `prompt.SystemPromptAssembler`.

### Requirement: Seven-Step Compression Pipeline

Physical order: 1 → 2 → 3 → 4 → [6] → 5 → 7. Operates on **messages only**.

---

## Historical (pre-20260618)

<details>
<summary>Legacy QueryLoop / Harness requirements (superseded)</summary>

Prior versions routed Process through `query.Loop.Run` when `query_loop.enabled=true`. Harness bootstrap ran when `query_loop.enabled=false`. Both paths removed in v8.0.0.

</details>
