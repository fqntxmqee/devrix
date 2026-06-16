# D2 Context Engine Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.1.0
**Last Updated:** 2026-06-16
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `openspec/specs/d2-context-engine/d2-domain.md`

---

## Overview

D2 上下文引擎域 A 层注册表。v3.0 重构为 5 个活跃 S + 1 个 Legacy（DM-20260616-001）。

---

## Canonical S/A 层 — v3.0

### D2-S1: PrepareContext（执行前准备）

| A ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S1-A01 | LoadSession | A-BE | session, model | SessionContext | `prepare/memory/manager.go` |
| D2-S1-A02 | RecallMemory | A-BE | query | memory_entries | `prepare/memory/longterm.go` |
| D2-S1-A03 | CompressContext | A-BE | messages, budget | compressed, report | `prepare/compression/pipeline.go` |
| D2-S1-A04 | AssemblePrompt | A-BE | build_input | system_prompt | `prepare/prompt/assembler.go` |

### D2-S2: ExecuteQuery（LLM↔Tool 执行循环）

| A ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S2-A01 | RunLoop | A-BE | session, params | loop_result | `query/loop.go` |
| D2-S2-A02 | ExecuteToolRound | A-BE | tool_calls | tool_results | `enforce/toolrunner/tool_runner.go` |
| D2-S2-A03 | StreamResponse | A-BE | text_chunks | events | `query/loop.go` |

### D2-S3: EnforcePolicy（安全策略执行）

| A ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S3-A01 | CheckPermission | A-BE | tool_call | allow/deny | `enforce/permission/mode.go` |
| D2-S3-A02 | FilterTools | A-BE | all_tools, mode | visible | `enforce/tool_filter.go` |
| D2-S3-A03 | SandboxExecution | A-BE | tool_call, workdir | sandboxed | `enforce/toolrunner/sandbox.go` |
| D2-S3-A04 | RegisterTools | A-BE | config | tool_registry | `enforce/toolrunner/tool_runner.go` |

### D2-S4: PersistState（执行后持久化）

| A ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S4-A01 | SaveSnapshot | A-BE | session | snapshot_bytes | `persist/snapshot/store.go` |
| D2-S4-A02 | WriteTranscript | A-BE | session_id, delta | jsonl | `persist/transcript/main_thread.go` |
| D2-S4-A03 | StoreLongTerm | A-BE | session, query, summary | — | `prepare/memory/longterm.go` |
| D2-S4-A04 | CommitWindow | A-BE | session, budget | trimmed | `engine_persist.go` |

### D2-S5: ~~NestedExecution~~ → S2+S3 拆解（DISMANTLED v3.1.0）

> **2026-06-16**: S19 拆解，fork 归 S1（PrepareContext），subquery+background 归 S2（ExecuteQuery）。
> 原 nested/ 目录已删除。详见 `d2-domain.md` v6.4.0。

| A ID | Name | Type | 原 Code Location | 新 Code Location |
|------|------|------|-----------------|-----------------|
| D2-S5-A01 | ~~SpawnSubquery~~ → S2 | A-BE | `nested/subquery.go` | `enforce/subquery.go` |
| D2-S5-A02 | ~~RunBackgroundTask~~ → S2 | A-BE | `nested/background.go` | `enforce/background.go` |
| D2-S5-A03 | ~~MergeSubResult~~ → S2 | A-BE | `nested/subquery.go` | `enforce/subquery.go` |

---

## Legacy: D2 Harness（REMOVED v6.5.0）

> `fallback/` 目录已删除。harness 路径不再可用。

| A ID | Name | Type | Status | Code Location |
|------|------|------|--------|---------------|
| D2-HARNESS-A01 | Bootstrap | A-BE | **REMOVED** | ~~`fallback/bootstrap.go`~~ |
| D2-HARNESS-A02 | Preflight | A-BE | **REMOVED** | ~~`fallback/preflight.go`~~ |
| D2-HARNESS-A03 | Route | A-BE | **REMOVED** | ~~`fallback/router.go`~~ |
| D2-HARNESS-A04 | ManageToolPool | A-BE | **REMOVED** | ~~`fallback/toolpool.go`~~ |

---

## Legacy Module Index — D2-S1–S14（冻结）

> v2.x 遗留编号，保留用于追溯。主路径已迁移至 v3.0 Canonical S1–S5。

### D2-S1: PEV (RETIRED)
> Execute / Verify / Plan 已由 D2-S2 ExecuteQuery 承接。

### D2-S2: Compression → D2-S1-A03
### D2-S3: Memory → D2-S1-A01/A02
### D2-S4: Token → 共享组件（不计入 S）
### D2-S5: Registry → D2-S3-A04
### D2-S6: Snapshot → D2-S4-A01
### D2-S7: Prompt → D2-S1-A04
### D2-S8: Sandbox → D2-S3-A03
### D2-S9: Harness → Legacy
### D2-S10: QueryLoop → D2-S2
### D2-S11: Queue → D2-S1-A04（Hub-Spoke drain 已合并入 AssemblePrompt）
### D2-S12: Worktree → 共享组件（不计入 S）
### D2-S13: Conversation → D2-S1-A03（RepairToolChain 合并入 CompressContext）
### D2-S14: Mock → 测试支撑

---

## Statistics

| Canonical Scenarios | Canonical Activities | Legacy Scenarios |
|---------------------|---------------------|------------------|
| 5 (S1–S5) | ~~17~~ 14（S5 拆解） | 14 (frozen) |
