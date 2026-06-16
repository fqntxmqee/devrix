# D2 Context Engine Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.0.0
**Last Updated:** 2026-06-16
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d2-context-engine/a-registry.md`
**Domain SoT:** `openspec/specs/d2-context-engine/d2-domain.md`

---

## Overview

D2 F 层注册表 v3.0。F 点按 Canonical S1–S5 重新索引。

---

## D2-S1 PrepareContext

### D2-S1-A01 LoadSession

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S1-A01-F01 | LoadOrInit | session_id, model | *SessionContext | `prepare/memory/manager.go` |
| D2-S1-A01-F02 | AppendMessages | session, messages | — | `prepare/memory/manager.go` |

### D2-S1-A02 RecallMemory

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S1-A02-F01 | RecallLongTerm | session_id, query | []MemoryEntry | `prepare/memory/longterm.go` |
| D2-S1-A02-F02 | FormatMemoryContext | entries, max_tokens | string | `prepare/memory/longterm_format.go` |

### D2-S1-A03 CompressContext

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S1-A03-F01 | RunPipeline | messages, budget | compressed, report | `prepare/compression/pipeline.go` |
| D2-S1-A03-F02 | RepairToolChain | messages | valid_messages | `prepare/conversation/repair.go` |
| D2-S1-A03-F03 | AutoCompact | messages, llm | compacted | `prepare/compression/autocompact.go` |
| D2-S1-A03-F04 | AsyncCompact | messages, llm | placeholder+async | `prepare/compression/async_compact.go` |
| D2-S1-A03-F05 | MessagesAfterCompactBoundary | messages | tail_messages | `prepare/conversation/boundary.go` |

### D2-S1-A04 AssemblePrompt

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S1-A04-F01 | BuildSystemPrompt | build_input | prompt, report | `prepare/prompt/assembler.go` |
| D2-S1-A04-F02 | LoadPromptSections | workdir | sections | `prepare/prompt/loader.go` |
| D2-S1-A04-F03 | CountTokens | text | int | `token/counter.go` |

---

## D2-S2 ExecuteQuery

### D2-S2-A01 RunLoop

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S2-A01-F01 | RunLoop | ctx, session, params | *Result | `query/loop.go` |
| D2-S2-A01-F02 | CallLLM | request | <-chan LLMChunk | `query/loop.go` |
| D2-S2-A01-F03 | RecoverFrom413 | oversized | retry_msgs | `query/recovery.go` |
| D2-S2-A01-F04 | FallbackOnOverload | error | fallback_llm | `query/recovery.go` |

### D2-S2-A02 ExecuteToolRound

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S2-A02-F01 | ExecuteTools | []ToolCall | []ToolResult | `engine.go` |
| D2-S2-A02-F02 | PersistToolHistory | tool_calls | sc.Messages | `engine.go` |

### D2-S2-A03 StreamResponse

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S2-A03-F01 | StreamEmit | text_chunk | EngineEvent | `query/loop.go` |

---

## D2-S3 EnforcePolicy

### D2-S3-A01 CheckPermission

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S3-A01-F01 | CheckPermission | tool_call, mode | allow/deny | `enforce/permission/mode.go` |
| D2-S3-A01-F02 | PlanModeWriteGate | path, plan_file | allowed | `enforce/permission/mode.go` |

### D2-S3-A02 FilterTools

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S3-A02-F01 | FilterByPermissionMode | tools, mode | visible | `permission_tools.go` |
| D2-S3-A02-F02 | FilterByAgentRole | tools, role | visible | `agent_role_filter.go` |

### D2-S3-A03 SandboxExecution

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S3-A03-F01 | SandboxBash | command, workdir | result | `enforce/toolrunner/sandbox.go` |

### D2-S3-A04 RegisterTools

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S3-A04-F01 | RegisterTool | tool_spec | — | `enforce/toolrunner/tool_runner.go` |
| D2-S3-A04-F02 | ListTools | — | []ToolSchema | `enforce/toolrunner/tool_runner.go` |
| D2-S3-A04-F03 | RegisterBuiltinTools | config | registry | `registry/builtin.go` |

---

## D2-S4 PersistState

### D2-S4-A01 SaveSnapshot

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S4-A01-F01 | Serialize | SessionContext | bytes | `persist/snapshot/store.go` |
| D2-S4-A01-F02 | Deserialize | bytes | SessionContext | `persist/snapshot/store.go` |

### D2-S4-A02 WriteTranscript

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S4-A02-F01 | AppendMainTranscript | session_id, msgs | — | `persist/transcript/main_thread.go` |
| D2-S4-A02-F02 | AppendSidechain | session_id, msgs | — | `persist/transcript/sidechain.go` |

### D2-S4-A03 StoreLongTerm

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S4-A03-F01 | AutoStore | session, query, summary | — | `prepare/memory/longterm.go` |

### D2-S4-A04 CommitWindow

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S4-A04-F01 | CommitActiveWindow | session, budget | active | `engine.go` |
| D2-S4-A04-F02 | TrimMessages | session, max | trimmed | `prepare/memory/manager.go` |

---

## D2-S5 NestedExecution

### D2-S5-A01 SpawnSubquery

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S5-A01-F01 | RunSubQuery | spec | result | `nested/subquery.go` |

### D2-S5-A02 RunBackgroundTask

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S5-A02-F01 | RegisterTask | session_id, agent_id | task_id | `nested/background.go` |
| D2-S5-A02-F02 | WaitForTask | task_id, timeout | state | `nested/background.go` |
| D2-S5-A02-F03 | CancelTask | task_id | — | `nested/background.go` |

---

## Legacy F（冻结，保留追溯）

> 以下 F 点映射到 D2-S9 Harness（#deprecated）。

| F ID | Name | Code Location |
|------|------|---------------|
| D2-S9-A01-F01 | ScanWorkspace | `harness/workspace.go` |
| D2-S9-A01-F02 | RoutePrompt | `harness/router.go` |
| D2-S9-A02-F01 | EvaluatePreflight | `harness/preflight.go` |
| D2-S9-A02-F02 | FilterVisibleTools | `harness/preflight.go` |
| D2-S9-A03-F01 | FilterByMode | `harness/toolpool.go` |
| D2-S9-A03-F02 | FilterByConfig | `harness/toolpool.go` |
| D2-S1-A01-F03 | RunPEVCycle | RETIRED |
| D2-S1-A02-F01 | VerifyCommands | RETIRED |
| D2-S1-A03-F01–F03 | Plan/Milestone | RETIRED |

---

## Statistics

| Canonical S | F Points |
|-------------|----------|
| S1 PrepareContext | 12 |
| S2 ExecuteQuery | 7 |
| S3 EnforcePolicy | 8 |
| S4 PersistState | 7 |
| S5 NestedExecution | 4 |
| **Total** | **38** |
