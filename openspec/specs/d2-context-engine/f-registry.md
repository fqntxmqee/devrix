# D2 Context Engine Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
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
| D2-S2-A01-F03 | TruncateFallback | F-BE | messages, max_tokens | truncated | `compression/pipeline.go` |

## D2-S3-A01 ManageMemory

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S3-A01-F01 | LoadOrInit | F-BE | session_id | *SessionContext | `memory/manager.go` |
| D2-S3-A01-F02 | AppendMessages | F-BE | session, messages | — | `memory/manager.go` |
| D2-S3-A01-F03 | RecallLongTerm | F-BE | session_id | []Memory | `memory/longterm.go` |

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

## D2-S9-A01 BootstrapSession

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S9-A01-F01 | ScanWorkspace | F-BE | workdir | *WorkspaceContext | `harness/workspace.go` |
| D2-S9-A01-F02 | EvaluatePreflight | F-BE | ctx, messages, budget | *PreflightResult | `harness/preflight.go` |
| D2-S9-A01-F03 | RoutePrompt | F-BE | user_message, tools | routing_hints | `harness/router.go` |

## D2-S9-A02 AssembleSystemPrompt

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S9-A02-F01 | BuildBasePrompt | F-BE | agent_config | base_prompt | `harness/system_prompt_assembler.go` |
| D2-S9-A02-F02 | MergeRulesLayers | F-BE | workspace, user, project | merged_rules | `harness/system_prompt_assembler.go` |

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

## D2-S10-A03 ExecuteBackgroundTask

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S10-A03-F01 | RegisterTask | F-BE | session_id, agent_id | task_id | `query/background.go` |
| D2-S10-A03-F02 | WaitForTask | F-BE | task_id, timeout | terminal_state | `query/background.go` |
| D2-S10-A03-F03 | CancelTask | F-BE | task_id | — | `query/background.go` |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 10 | 27 |
