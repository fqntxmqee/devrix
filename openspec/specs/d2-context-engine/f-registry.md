# D2 Context Engine Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.2.0
**Last Updated:** 2026-06-29
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d2-context-engine/a-registry.md`
**Domain SoT:** `openspec/specs/d2-context-engine/d2-domain.md`

> **DM-20260629-002 (devrix-d2-dsaft-restructuring) PR-4** — registry-sync:
> F IDs re-keyed from legacy `D2-S1..S5` numbering to canonical
> `D2-S15..S18` numbering to match `a-registry.md` (v4.1.0). Historical S
> (S1 PEV, S9 Harness, S10 QueryLoop, S19 NestedExecution, S20 LegacyHarness)
> moved to a single §Historical Appendix. The legacy `D2-S1..S5` aliases
> are kept as tombstone cross-references in the appendix for trace purposes.

---

## Overview

D2 F 层注册表 v3.2。F 点按 Canonical S15–S18 重新索引（与 `a-registry.md` v4.1.0 一致）。

---

## D2-S15 PrepareExecutionContext

### D2-S15-A01 LoadSession

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S15-A01-F01 | LoadOrInit | session_id, model | *SessionContext | `prepare/memory/manager.go` |
| D2-S15-A01-F02 | AppendMessages | session, messages | — | `prepare/memory/manager.go` |

### D2-S15-A02 RecallMemory

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S15-A02-F01 | RecallLongTerm | session_id, query | []MemoryEntry | `prepare/memory/longterm.go` |
| D2-S15-A02-F02 | FormatMemoryContext | entries, max_tokens | string | `prepare/memory/longterm_format.go` |

### D2-S15-A03 CompressContext

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S15-A03-F01 | RunPipeline | messages, budget | compressed, report | `prepare/compression/pipeline.go` |
| D2-S15-A03-F02 | RepairToolChain | messages | valid_messages | `prepare/conversation/repair.go` |
| D2-S15-A03-F03 | AutoCompact | messages, llm | compacted | `prepare/compression/autocompact.go` |
| D2-S15-A03-F04 | AsyncCompact | messages, llm | placeholder+async | `prepare/compression/async_compact.go` |
| D2-S15-A03-F05 | MessagesAfterCompactBoundary | messages | tail_messages | `prepare/conversation/boundary.go` |

### D2-S15-A04 AssemblePrompt

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S15-A04-F01 | BuildSystemPrompt | build_input | prompt, report | `prepare/prompt/assembler.go` |
| D2-S15-A04-F02 | LoadPromptSections | workdir | sections | `prepare/prompt/loader.go` |
| D2-S15-A04-F03 | CountTokens | text | int | `token/counter.go` |

---

## D2-S16 RunQueryLoop — REMOVED (DM-20260618-010)

> **DM-20260618-010：** LLM↔Tool 循环归 D7-S2-A06；下列 F 点仅作追溯，全部 REMOVED。

### D2-S16-A01 RunLoop

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S16-A01-F01 | RunLoop | — | — | **REMOVED** → `orchestration/turn/orchestrator.go` |
| D2-S16-A01-F02 | CallLLM | — | — | **REMOVED** → D7 `GatewayInvoker` |
| D2-S16-A01-F03 | RecoverFrom413 | — | — | **REMOVED** → `orchestration/turn/recovery.go` |
| D2-S16-A01-F04 | FallbackOnOverload | — | — | **REMOVED** → D7 turn runtime |

### D2-S16-A02 ExecuteToolRound

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S16-A02-F01 | ExecuteTools | []ToolCall | []ToolResult | `enforce/toolrunner/tool_runner.go` |
| D2-S16-A02-F02 | PersistToolHistory | tool_calls | sc.Messages | `engine_persist.go` (legacy → `persist/` per DM-20260629-002 PR-1) |

### D2-S16-A03 StreamResponse

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S16-A03-F01 | StreamEmit | — | — | **REMOVED** → D7 turn runtime |

---

## D2-S17 PersistSessionState

### D2-S17-A01 SaveSnapshot

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S17-A01-F01 | Serialize | SessionContext | bytes | `persist/snapshot/store.go` |
| D2-S17-A01-F02 | Deserialize | bytes | SessionContext | `persist/snapshot/store.go` |

### D2-S17-A02 WriteTranscript

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S17-A02-F01 | AppendMainTranscript | session_id, msgs | — | `persist/transcript/main_thread.go` |
| D2-S17-A02-F02 | AppendSidechain | session_id, msgs | — | `persist/transcript/sidechain.go` |

### D2-S17-A03 StoreLongTerm

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S17-A03-F01 | AutoStore | session, query, summary | — | `prepare/memory/longterm.go` |

### D2-S17-A04 CommitWindow

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S17-A04-F01 | CommitActiveWindow | session, budget | active | `kernel/context_engine_commit_window_adapter.go` (was `engine_persist.go`, migrated by DM-20260629-002 PR-1) |
| D2-S17-A04-F02 | TrimMessages | session, max | trimmed | `prepare/memory/manager.go` |

---

## D2-S18 EnforceExecutionPolicy

### D2-S18-A01 CheckPermission

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S18-A01-F01 | CheckPermission | tool_call, mode | allow/deny | `enforce/permission/mode.go` |
| D2-S18-A01-F02 | PlanModeWriteGate | path, plan_file | allowed | `enforce/permission/mode.go` |

### D2-S18-A02 FilterTools

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S18-A02-F01 | FilterByPermissionMode | tools, mode | visible | `enforce/tool_filter.go` |
| D2-S18-A02-F02 | FilterByAgentRole | tools, role | visible | `enforce/agent_role_filter.go` |

### D2-S18-A03 SandboxExecution

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S18-A03-F01 | SandboxBash | command, workdir | result | `enforce/toolrunner/sandbox.go` |

### D2-S18-A04 RegisterTools

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S18-A04-F01 | RegisterTool | tool_spec | — | `enforce/toolrunner/tool_runner.go` |
| D2-S18-A04-F02 | ListTools | — | []ToolSchema | `enforce/toolrunner/tool_runner.go` |
| D2-S18-A04-F03 | RegisterBuiltinTools | config | registry | `registry/builtin.go` |

### D2-S18-A06 SpawnSubquery

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S18-A06-F01 | RunSubQuery | spec | task_id | `enforce/subquery.go` |

### D2-S18-A07 RunBackgroundTask

| F ID | Name | Input | Output | Code Location |
|------|------|-------|--------|---------------|
| D2-S18-A07-F01 | RegisterTask | spec | bg_id | `enforce/registry.go` (was `background.go`, split by DM-20260629-002 PR-3) |
| D2-S18-A07-F02 | WaitForTask | bg_id, timeout | result | `enforce/run.go` |
| D2-S18-A07-F03 | CancelTask | bg_id | ok | `enforce/registry.go` |

---

## Historical Appendix

> **DM-20260629-002 (devrix-d2-dsaft-restructuring) PR-4:** historical S
> previously scattered across the legacy `## D2-S5` and `## Legacy F`
> sections, plus the t-registry `~~S1 / S9 / S10 / S19 / S20~~` rows, now
> consolidated here as tombstone cross-references for trace purposes only.
> No active code references these S/F/T — see `d2_layout_test.go` for the
> "no legacy.Process() callers" guard (D2-STRUCT-T07).

### ~~D2-S1 PrepareContext~~ (was D2-S1 PEV) — RETIRED

PEV (Plan / Execute / Verify) cycle predates the canonical S15–S18 split. F points
migrated to S15 in v2.2 structure closure (DM-20260619-007).

| F ID | Name | Final destination |
|------|------|-------------------|
| ~~D2-S1-A01-F03~~ | RunPEVCycle | RETIRED |
| ~~D2-S1-A02-F01~~ | VerifyCommands | RETIRED |
| ~~D2-S1-A03-F01..F03~~ | Plan/Milestone | RETIRED |

### ~~D2-S5 NestedExecution~~ — DISMANTLED v3.1.0

Fork → S15-A04 (PrepareContext); subquery + background → S18-A06/A07
(EnforceExecutionPolicy).

| F ID | Name | Final destination |
|------|------|-------------------|
| ~~D2-S5-A01-F01~~ | RunSubQuery | → D2-S18-A06-F01 |
| ~~D2-S5-A02-F01~~ | RegisterTask | → D2-S18-A07-F01 |
| ~~D2-S5-A02-F02~~ | WaitForTask | → D2-S18-A07-F02 |
| ~~D2-S5-A02-F03~~ | CancelTask | → D2-S18-A07-F03 |

### ~~D2-S9 Harness~~ — REMOVED v6.5.0

The legacy Harness fallback (ScanWorkspace / RoutePrompt / EvaluatePreflight /
FilterVisibleTools / FilterByMode / FilterByConfig) was removed in v6.5.0 when
the canonical hot path moved to D7 SessionOrchestrator + D2 turn adapters.

| F ID | Name | Code Location (at retirement) |
|------|------|-------------------------------|
| ~~D2-S9-A01-F01~~ | ScanWorkspace | ~~`fallback/workspace.go`~~ |
| ~~D2-S9-A01-F02~~ | RoutePrompt | ~~`fallback/router.go`~~ |
| ~~D2-S9-A02-F01~~ | EvaluatePreflight | ~~`fallback/preflight.go`~~ |
| ~~D2-S9-A02-F02~~ | FilterVisibleTools | ~~`fallback/preflight.go`~~ |
| ~~D2-S9-A03-F01~~ | FilterByMode | ~~`fallback/toolpool.go`~~ |
| ~~D2-S9-A03-F02~~ | FilterByConfig | ~~`fallback/toolpool.go`~~ |

### ~~D2-S10 QueryLoop~~ — REMOVED (DM-20260618-010)

The D2-owned QueryLoop subsystem was retired in DM-20260618-010 and the
`internal/layers/contextengine/query/` directory was deleted (guarded by
D2-STRUCT-T08 in `d2_layout_test.go`).

### ~~D2-S19 NestedExecution Fork~~ — DISMANTLED (DM-20260614-009)

Fork sidecar functionality originally tracked under S19 was split into
S15-A04 (Prepare sidecar, `prepare/conversation/fork.go` + `fork_worker.go`)
and S18-A06 (Execute sidecar, `enforce/subquery.go`) in v2.2.

### ~~D2-S20 LegacyHarnessFallback~~ — REMOVED v6.5.0

The LegacyHarness fallback counter (`legacy_harness` metric) was retired in
v6.5.0 alongside the D2 harness subsystem. The `runtime.Snapshot().LegacyHarness`
field on the observability side is preserved (always 0) for backward
compatibility with dashboards.

---

## Statistics

| Canonical S | Active F Points |
|-------------|----------------|
| S15 PrepareExecutionContext | 12 |
| S16 RunQueryLoop | 0 (REMOVED, 7 historical) |
| S17 PersistSessionState | 6 |
| S18 EnforceExecutionPolicy | 12 |
| **Total active** | **30** |
| Historical (S1+S5+S9+S10+S19+S20) | 17 (tombstone only) |