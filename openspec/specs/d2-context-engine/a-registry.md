# D2 Context Engine Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D2 上下文引擎域 A 层活动注册表。

---

## D2-S1: PEV (RETIRED)

> **2026-06-13**：PEV 引擎已下线。Execute / Verify / Plan 活动由 **D2-S10 QueryLoop** 承接。

| A ID | Name | Type | Status | Successor |
|------|------|------|--------|-----------|
| D2-S1-A01 | ExecutePEV | A-BE | RETIRED | D2-S10-A01 RunQueryLoop |
| D2-S1-A02 | VerifyExecution | A-BE | RETIRED | — |
| D2-S1-A03 | PlanExecution | A-BE | RETIRED | — |

## D2-S2: Compression

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S2-A01 | CompressContext | A-BE | messages, budget | compressed_messages, report | context.compressed | `contextengine/compression/pipeline.go` |

## D2-S3: Memory

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S3-A01 | ManageMemory | A-BE | session, messages | updated_session | memory.{appended,recalled,persisted} | `contextengine/memory/manager.go` |

## D2-S4: Token

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S4-A01 | CountTokens | A-BE | text/messages | token_count, budget | — | `contextengine/token/` |

## D2-S5: Registry

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S5-A01 | RegisterOperation | A-BE | tool_spec | operation_id | operation.registered | `contextengine/registry/builtin.go` |

## D2-S6: Snapshot

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S6-A01 | SnapshotContext | A-BE | session | snapshot_data | snapshot.created | `contextengine/snapshot/` |

## D2-S7: Prompt

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S7-A01 | ManagePromptTemplate | A-BE | template_name, params | assembled_prompt | — | `contextengine/prompt/` (PLANNED) |

## D2-S8: Sandbox

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S8-A01 | IsolateTool | A-BE | tool_call, workdir | sandboxed_command | — | PLANNED (test only) |

## D2-S9: Harness

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S9-A01 | BootstrapSession | A-BE | session, config | bootstrap_result | session.bootstrapped | `contextengine/harness/bootstrap.go` |
| D2-S9-A02 | AssembleSystemPrompt | A-BE | layers (base/workspace/rules/tool) | assembled_prompt | — | `contextengine/harness/system_prompt_assembler.go` |
| D2-S9-A03 | FilterToolPool | A-BE | all_tools, mode | visible_tools | — | `contextengine/harness/toolpool.go` |

## D2-S10: QueryLoop

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S10-A01 | RunQueryLoop | A-BE | session, params | result (messages, usage) | loop.{iterating,completed,failed} | `contextengine/query/loop.go` |
| D2-S10-A02 | AttachUserContext | A-BE | session | enriched_session | session.user_context_set | `contextengine/usercontext/` |
| D2-S10-A03 | ExecuteBackgroundTask | A-BE | task_spec | task_id | task.{registered,running,completed} | `contextengine/query/background.go` |

## D2-S11: Queue

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S11-A01 | ManageSessionQueue | A-BE | session_id, events | drained_events | queue.{enqueued,drained} | `contextengine/queue/` |

## D2-S12: Worktree

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S12-A01 | EnterWorktree | A-BE | session, spec | worktree_path | worktree.{entered,exited} | `contextengine/worktree/` |

## D2-S13: Conversation

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S13-A01 | ManageConversation | A-BE | session, messages | conversation_state | conversation.updated | `contextengine/conversation/` |

## D2-S14: Mock

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S14-A01 | MockEngine | A-BE | mock_config | mock_engine | — | `contextengine/mock/` |

---

## Statistics

| Scenarios | Activities | RETIRED |
|-----------|------------|---------|
| 14 | 18 | 3 |
