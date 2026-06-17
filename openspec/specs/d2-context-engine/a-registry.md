# D2 Context Engine Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 4.1.0
**Last Updated:** 2026-06-18
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `d2-domain.md`

---

## Overview

D2 上下文引擎域 A 层注册表。**Canonical 全局编号 D2-S15–S18**（与 `d2-domain.md` 一致）。

> **终态流程 / D7 Follower 拆面：** 见 `terminal-state-guide.md` §3–§6  
> **Span↔T Runbook：** 见 `observability-guide.md`

---

## Canonical S/A — D2-S15–S18

### D2-S15: PrepareExecutionContext ✅

> North Star: Turn 前上下文合法、在预算内

| A ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S15-A01 | LoadSession | A-BE | session, model | SessionContext | `prepare/memory/manager.go` |
| D2-S15-A02 | RecallMemory | A-BE | query | memory_entries | `prepare/memory/longterm.go` |
| D2-S15-A03 | CompressContext | A-BE | messages, budget | compressed, report | `prepare/compression/pipeline.go` |
| D2-S15-A04 | AssemblePrompt | A-BE | build_input | system_prompt | `prepare/prompt/assembler.go` |

> **S19 拆解迁入：** `prepare/conversation/fork.go` + `fork_worker.go`（fork 侧车，仍属 S15 准备面）

### D2-S16: RunQueryLoop — LEGACY FREEZE ✅

> **DM-020：** Turn 主循环迁 **D7-S2-A06**；本 S 保留 Thin Loop + Legacy `engine.Process` 追溯。

| A ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S16-A01 | RunLoop | A-BE | session, params | loop_result | `query/loop.go` |
| D2-S16-A02 | StreamResponse | A-BE | text_chunks | events | `query/loop.go` |

### D2-S17: PersistSessionState ✅

> North Star: Turn 后状态 durable + deferred complete

| A ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S17-A01 | SaveSnapshot | A-BE | session | snapshot_bytes | `persist/snapshot/store.go` |
| D2-S17-A02 | WriteTranscript | A-BE | session_id, delta | jsonl | `persist/transcript/main_thread.go` |
| D2-S17-A03 | StoreLongTerm | A-BE | session, query, summary | — | `prepare/memory/longterm.go` |
| D2-S17-A04 | CommitWindow | A-BE | session, budget | trimmed | `engine_persist.go` |

### D2-S18: EnforceExecutionPolicy ✅

> North Star: 权限 / 沙箱 / 工具面先于执行；SubQuery/Background 执行体
>
> **v3 增量 (DM-20260618-001/002/003):**
> - D2-S18-A01 拆分为 **Surface.CheckPermission** (per-tool hook, surface 内置) +
>   **IPermissionGate.CheckPermission** (D7 orchestration 提供, plan_mode 自动 deny) +
>   **PlanModeOpenWorldPolicy.ShouldDefer** (runtime 懒加载过滤)
> - D2-S18-A02 拆分为 **PerAgentFilter.Apply** + **PerRiskFilter.Apply** +
>   **ToolFilter.ShouldDefer** (DM-20260618-003 新增)
> - D2-S18-A04 扩展为 **BuildSurfaces(SurfaceBuildOpts) → []ToolSurface**（8 surface
>   含 ToolSearchSurface）+ 排序稳定 + 4 bool 字段填充
> - D2-S18-A05 升级为 2-phase dispatch (Phase 1 CheckPermission sequential,
>   Phase 2 parallel via ConcurrencySafe)

| A ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S18-A01 | CheckPermission (3-tier) | A-BE | tool_call, ctx | Decision (allow/deny/ask) | `contracts/permission.go` + `surface/{bash_ast,permission}.go` + `orchestration/toolpolicy/plan_mode.go` |
| D2-S18-A02 | FilterTools + ShouldDefer | A-BE | all_tools, mode, ctx | visible (defer-aware) | `enforce/toolrunner/filter/{per_agent,per_risk}.go` + `toolpolicy/filter_adapter.go` |
| D2-S18-A03 | SandboxExecution | A-BE | tool_call, workdir | sandboxed | `enforce/toolrunner/sandbox.go` |
| D2-S18-A04 | BuildSurfaces (8 surface) | A-BE | config, opts | sorted []ToolSurface | `bootstrap/surfaces.go` + `surface/{builtin,lsptool,freefork,tracker,verify,delegate,background_task,tool_search}_surface.go` |
| D2-S18-A05 | ExecuteToolRound (2-phase) | A-BE | tool_calls, ctx | tool_results (indexed) | `bootstrap/turn_adapter.go` |
| D2-S18-A06 | SpawnSubquery | A-BE | spec | task_id | `enforce/subquery.go` |
| D2-S18-A07 | RunBackgroundTask | A-BE | spec | bg_id | `enforce/background.go` |
| D2-S18-A08 | MergeSubResult | A-BE | sub_result | messages | `enforce/subquery.go` |

---

## Retired — D2-S19 / S20

| S | 状态 | 去向 |
|---|------|------|
| D2-S19 NestedExecution | **DISMANTLED** v6.4.0 | fork→S15；subquery/background→S18 |
| D2-S20 LegacyHarnessFallback | **REMOVED** v6.5.0 | `fallback/` 已删除 |

---

## Legacy Module Index — D2-S1–S14（冻结）

> v2.x 模块编号，追溯用。Canonical 映射见 `d2-domain.md`。

| Legacy S | 映射 |
|----------|------|
| S2 Compression | → S15-A03 |
| S3 Memory | → S15-A01/A02, S17-A03 |
| S5 Registry | → S18-A04 |
| S6 Snapshot | → S17-A01 |
| S7 Prompt | → S15-A04 |
| S8 Sandbox | → S18-A03 |
| S10 QueryLoop | → S16, S18-A05 |
| S11 Queue | → **D7-S4** |
| S13 Conversation | → S15-A03（RepairToolChain） |

### 域内 v3.0 本地编号追溯（已废弃）

| 本地 S（v3.0） | 全局 Canonical |
|----------------|----------------|
| D2-S1 PrepareContext | **D2-S15** |
| D2-S2 ExecuteQuery | **D2-S16** + S18-A05 |
| D2-S3 EnforcePolicy | **D2-S18** A01–A04 |
| D2-S4 PersistState | **D2-S17** |
| D2-S5 Nested | **DISMANTLED** → S15+S18 |

---

## Statistics

| Canonical S | A 数 | 状态 |
|-------------|------|------|
| S15 Prepare | 4 | IMPLEMENTED |
| S16 QueryLoop | 2 | LEGACY FREEZE |
| S17 Persist | 4 | IMPLEMENTED |
| S18 Enforce | 8 | IMPLEMENTED |
| **合计** | **18** | |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 3.1.0 | 2026-06-16 | S19 拆解、S20 移除 |
| 4.0.0 | 2026-06-16 | Canonical 全局编号 S15–S18；ExecuteToolRound 归 S18；S19 活动并入 S18；Guides 指针 |
| **4.1.0** | **2026-06-18** | **S18 v3 — 3-tier CheckPermission (Surface + IPermissionGate + PlanMode); 8 surface (含 ToolSearchSurface); 2-phase ExecuteToolRound; ToolSpec 4 bool + DeferLoading 字段；ToolFilter.ShouldDefer runtime hook；A01/A02/A04/A05 拆解细化** (DM-20260618-001/002/003) |
