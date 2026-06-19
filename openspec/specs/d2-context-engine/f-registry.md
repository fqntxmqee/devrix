# D2 Context Engine Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.1.0
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

## D2-S2 ExecuteQuery — REMOVED (Legacy)

> **DM-20260618-010：** LLM↔Tool 循环归 D7-S2-A06；下列 F 点仅作追溯。

### D2-S2-A01 RunLoop

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S2-A01-F01 | RunLoop | — | — | **REMOVED** → `orchestration/turn/orchestrator.go` |
| D2-S2-A01-F02 | CallLLM | — | — | **REMOVED** → D7 `GatewayInvoker` |
| D2-S2-A01-F03 | RecoverFrom413 | — | — | **REMOVED** → `orchestration/turn/recovery.go` |
| D2-S2-A01-F04 | FallbackOnOverload | — | — | **REMOVED** → D7 turn runtime |

### D2-S2-A02 ExecuteToolRound

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S2-A02-F01 | ExecuteTools | []ToolCall | []ToolResult | `enforce/toolrunner/tool_runner.go` |
| D2-S2-A02-F02 | PersistToolHistory | tool_calls | sc.Messages | `engine_persist.go` |

### D2-S2-A03 StreamResponse

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S2-A03-F01 | StreamEmit | — | — | **REMOVED** → D7 turn runtime |

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
| D2-S3-A02-F01 | FilterByPermissionMode | tools, mode | visible | `enforce/tool_filter.go` |
| D2-S3-A02-F02 | FilterByAgentRole | tools, role | visible | `enforce/agent_role_filter.go` |

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
| D2-S4-A04-F01 | CommitActiveWindow | session, budget | active | `engine_persist.go` |
| D2-S4-A04-F02 | TrimMessages | session, max | trimmed | `prepare/memory/manager.go` |

---

## D2-S5 ~~NestedExecution~~ → S1+S2 拆解（DISMANTLED v3.1.0）

> **2026-06-16**: S19 拆解。fork 归 S1 PrepareContext，subquery+background 归 S2 ExecuteQuery。

### ~~D2-S5-A01 SpawnSubquery~~ → S2 ExecuteQuery

| F ID | Name | 原 Code Location | 新 Code Location |
|------|------|-----------------|-----------------|
| D2-S5-A01-F01 | RunSubQuery | `nested/subquery.go` | `enforce/subquery.go` |

### ~~D2-S5-A02 RunBackgroundTask~~ → S2 ExecuteQuery

| F ID | Name | 原 Code Location | 新 Code Location |
|------|------|-----------------|-----------------|
| D2-S5-A02-F01 | RegisterTask | `nested/background.go` | `enforce/background.go` |
| D2-S5-A02-F02 | WaitForTask | `nested/background.go` | `enforce/background.go` |
| D2-S5-A02-F03 | CancelTask | `nested/background.go` | `enforce/background.go` |

---

## Legacy F（冻结，保留追溯）

> 以下 F 点映射到 D2-S9 Harness（**REMOVED v6.5.0**）。

| F ID | Name | Code Location | Status |
|------|------|---------------|--------|
| D2-S9-A01-F01 | ScanWorkspace | ~~`fallback/workspace.go`~~ | **REMOVED** |
| D2-S9-A01-F02 | RoutePrompt | ~~`fallback/router.go`~~ | **REMOVED** |
| D2-S9-A02-F01 | EvaluatePreflight | ~~`fallback/preflight.go`~~ | **REMOVED** |
| D2-S9-A02-F02 | FilterVisibleTools | ~~`fallback/preflight.go`~~ | **REMOVED** |
| D2-S9-A03-F01 | FilterByMode | ~~`fallback/toolpool.go`~~ | **REMOVED** |
| D2-S9-A03-F02 | FilterByConfig | ~~`fallback/toolpool.go`~~ | **REMOVED** |
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
| S5 NestedExecution | 4 | **DISMANTLED** |
| **Total** | **38** | |
