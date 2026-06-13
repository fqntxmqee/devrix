# D2 Context Engine Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.1.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D2 上下文引擎域 A 层活动注册表。主路径：**D2-S10 QueryLoop**（`query_loop.enabled` 默认 true）。D2-S1 PEV 已退役。

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
| D2-S2-A01 | CompressContext | A-BE | messages, budget | compressed_messages, report | context.compressed | `compression/pipeline.go` |

## D2-S3: Memory

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S3-A01 | ManageMemory | A-BE | session, messages | updated_session | memory.{appended,recalled,persisted} | `memory/manager.go` |

## D2-S4: Token

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S4-A01 | CountTokens | A-BE | text/messages | token_count, budget | — | `token/counter.go`, `contracts/tokencounter.go` |

## D2-S5: Registry

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S5-A01 | RegisterOperation | A-BE | tool_spec | operation_id | operation.registered | `registry/builtin.go` |

## D2-S6: Snapshot

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S6-A01 | SnapshotContext | A-BE | session | snapshot_data | snapshot.created | `snapshot/store.go` |
| D2-S6-A02 | PersistMainTranscript | A-BE | session_id, message_delta | jsonl_lines | transcript.appended | `transcript/main_thread.go` |

## D2-S7: Prompt

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S7-A01 | LoadPromptSections | A-BE | workdir, config | sections[] | — | `prompt/loader.go` |
| D2-S7-A02 | AssembleSystemPrompt | A-BE | build_input | assembled_prompt | — | `prompt/assembler.go` |

## D2-S8: Sandbox

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S8-A01 | IsolateTool | A-BE | tool_call, workdir | sandboxed_command | — | `toolrunner/sandbox.go` |

## D2-S9: Harness

> Legacy fallback：仅 `query_loop.enabled=false` 且 `harness.enabled=true` 时执行 Bootstrap / Preflight / Routing。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S9-A01 | BootstrapSession | A-BE | session, config | bootstrap_result | session.bootstrapped | `harness/bootstrap.go` |
| D2-S9-A02 | EvaluatePreflight | A-BE | session, tools, context | preflight_result | — | `harness/preflight.go` |
| D2-S9-A03 | FilterToolPool | A-BE | all_tools, mode | visible_tools | — | `harness/toolpool.go` |

## D2-S10: QueryLoop

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S10-A01 | RunQueryLoop | A-BE | session, params | result (messages, usage) | loop.{iterating,completed,failed} | `query/loop.go` |
| D2-S10-A02 | AttachUserContext | A-BE | session | enriched_messages | session.user_context_set | `usercontext/provider.go` |
| D2-S10-A03 | ExecuteBackgroundTask | A-BE | task_spec | task_id | task.{registered,running,completed} | `query/background.go` |

## D2-S11: Queue

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S11-A01 | ManageSessionQueue | A-BE | session_id, events | drained_events | queue.{enqueued,drained} | `queue/session_queue.go` |

## D2-S12: Worktree

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S12-A01 | EnterWorktree | A-BE | session, spec | worktree_path | worktree.{entered,exited} | `worktree/manager.go` |

## D2-S13: Conversation

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S13-A01 | RepairToolChain | A-BE | messages | valid_messages | conversation.repaired | `conversation/repair.go` |
| D2-S13-A02 | ManageCompactBoundary | A-BE | messages, trigger | boundary_marker | conversation.compacted | `conversation/boundary.go` |
| D2-S13-A03 | TrimActiveMessages | A-BE | session, max | trimmed_messages | conversation.trimmed | `conversation/trim.go`, `memory/manager.go` |

## D2-S14: Mock

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S14-A01 | MockEngine | A-BE | mock_config | mock_engine | — | `mock/llm.go` |

---

## Statistics

| Scenarios | Activities | RETIRED |
|-----------|------------|---------|
| 14 | 22 | 3 |
