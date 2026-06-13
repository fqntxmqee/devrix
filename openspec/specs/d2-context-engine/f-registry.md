# D2 Context Engine Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.1.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d2-context-engine/a-registry.md`

---

## Overview

D2 上下文引擎域 F 层功能点注册表。

---

## D2-S1 (RETIRED): PEV

> **2026-06-13**：PEV 功能点已移除。QueryLoop 见 D2-S10。

| F ID | Name | Status | Successor |
|------|------|--------|-----------|
| D2-S1-A01-F03 | RunPEVCycle | RETIRED | D2-S10 QueryLoop |
| D2-S1-A02-F01 | VerifyCommands | RETIRED | — |
| D2-S1-A03-F01–F03 | Plan/Milestone | RETIRED | — |

## D2-S2-A01 CompressContext

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S2-A01-F01 | RunPipeline | F-BE | messages, budget | compressed, report | `compression/pipeline.go` |
| D2-S2-A01-F02 | AutoCompact | F-BE | messages, llm | compacted | `compression/autocompact.go` |
| D2-S2-A01-F03 | AsyncCompact | F-BE | messages, llm | placeholder + async | `compression/async_compact.go` |
| D2-S2-A01-F04 | TruncateFallback | F-BE | messages, max_tokens | truncated | `compression/pipeline.go` |

## D2-S3-A01 ManageMemory

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S3-A01-F01 | LoadOrInit | F-BE | session_id | *SessionContext | `memory/manager.go` |
| D2-S3-A01-F02 | AppendMessages | F-BE | session, messages | — | `memory/manager.go` |
| D2-S3-A01-F03 | RecallLongTerm | F-BE | session_id | []Memory | `memory/longterm.go` |
| D2-S3-A01-F04 | CommitActiveWindow | F-BE | session, budget | active_messages | `engine.go` (`commitActiveWindow`) |

## D2-S4-A01 CountTokens

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S4-A01-F01 | CountText | F-BE | text | int | `contracts/tokencounter.go` |
| D2-S4-A01-F02 | CountMessages | F-BE | []Message | int | `contracts/tokencounter.go` |
| D2-S4-A01-F03 | CountWithSystemPrompt | F-BE | prompt, messages | int | `contracts/tokencounter.go` |
| D2-S4-A01-F04 | TruncateToTokens | F-BE | text, max_tokens | string | `contracts/tokencounter.go` |
| D2-S4-A01-F05 | EncodingForModel | F-BE | model | encoding_name | `contracts/tokencounter.go` |

## D2-S5-A01 RegisterOperation

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S5-A01-F01 | RegisterTool | F-BE | tool_spec | — | `registry/builtin.go` |
| D2-S5-A01-F02 | ListTools | F-BE | — | []ToolSpec | `registry/builtin.go` |

## D2-S6-A01 SnapshotContext

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S6-A01-F01 | Serialize | F-BE | SessionContext | bytes | `snapshot/store.go` |
| D2-S6-A01-F02 | Deserialize | F-BE | bytes | SessionContext | `snapshot/store.go` |

## D2-S6-A02 PersistMainTranscript

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S6-A02-F01 | AppendBatch | F-BE | session_id, messages | — | `transcript/main_thread.go` |
| D2-S6-A02-F02 | Load | F-BE | session_id | []Message | `transcript/main_thread.go` |

## D2-S7-A01 LoadPromptSections

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S7-A01-F01 | LoadAsSections | F-BE | workdir | []string | `prompt/loader.go` |
| D2-S7-A01-F02 | LoadWithDynamic | F-BE | workdir | sections + boundary | `prompt/loader.go` |

## D2-S7-A02 AssembleSystemPrompt

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S7-A02-F01 | Build | F-BE | SystemPromptBuildInput | prompt, report | `prompt/assembler.go` |
| D2-S7-A02-F02 | BuildLegacy | F-BE | agents_raw, memory | prompt | `prompt/assembler.go` |

## D2-S8-A01 IsolateTool

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S8-A01-F01 | SandboxBash | F-BE | command, workdir | result | `toolrunner/sandbox.go` |

## D2-S9-A01 BootstrapSession

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S9-A01-F01 | ScanWorkspace | F-BE | workdir | *WorkspaceContext | `harness/workspace.go` |
| D2-S9-A01-F02 | RoutePrompt | F-BE | user_message, tools | routing_hints | `harness/router.go` |

## D2-S9-A02 EvaluatePreflight

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S9-A02-F01 | Evaluate | F-BE | ctx, messages, budget | *PreflightResult | `harness/preflight.go` |
| D2-S9-A02-F02 | FilterVisibleTools | F-BE | message, tools | filtered_tools | `harness/preflight.go` |

## D2-S9-A03 FilterToolPool

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S9-A03-F01 | FilterByMode | F-BE | all_tools, mode | visible_tools | `harness/toolpool.go` |
| D2-S9-A03-F02 | FilterByConfig | F-BE | tools, deny_list | filtered_tools | `harness/toolpool.go` |

## D2-S10-A01 RunQueryLoop

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S10-A01-F01 | RunLoop | F-BE | ctx, session, params | *Result | `query/loop.go` |
| D2-S10-A01-F02 | CallLLM | F-BE | request | <-chan LLMChunk | `query/loop.go` |
| D2-S10-A01-F03 | ExecuteTools | F-BE | []ToolCall | []ToolResult | `query/streaming_executor.go` |
| D2-S10-A01-F04 | RecoverFrom413 | F-BE | oversized request | retry messages | `query/recovery.go` |
| D2-S10-A01-F05 | FallbackOnOverload | F-BE | primary error | fallback LLM | `query/recovery.go` |

## D2-S10-A03 ExecuteBackgroundTask

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S10-A03-F01 | RegisterTask | F-BE | session_id, agent_id | task_id | `query/background.go` |
| D2-S10-A03-F02 | WaitForTask | F-BE | task_id, timeout | terminal_state | `query/background.go` |
| D2-S10-A03-F03 | CancelTask | F-BE | task_id | — | `query/background.go` |

## D2-S13-A01 RepairToolChain

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S13-A01-F01 | RepairToolMessageChain | F-BE | messages | valid_messages | `conversation/repair.go` |
| D2-S13-A01-F02 | FilterIncompleteToolCalls | F-BE | messages | trimmed | `conversation/filter.go` |

## D2-S13-A02 ManageCompactBoundary

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S13-A02-F01 | MessagesAfterCompactBoundary | F-BE | messages | tail_messages | `conversation/boundary.go` |
| D2-S13-A02-F02 | NewCompactBoundaryMessage | F-BE | trigger, count | system_marker | `conversation/boundary.go` |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 18 | 38 |
