# Tasks: Unified Task Registry

**Change ID:** devrix-unified-task-registry
**Demand ID:** DM-20260612-011
**Status:** S7_Archived (2026-06-18; S2_Cancelled; not implemented)

> **归档说明 (2026-06-18):** 变更在 S2 阶段取消；依赖项 "Wave Scheduler v1.2 T15" 未实施。本文件保留原 S2_Clarified 任务表作为历史参考。

## S0 — Demand 创建（已完成）

| ID | 任务 | 状态 | 日期 |
|----|------|------|------|
| D01 | 创建 demand.md | ✅ DONE | 2026-06-12 |
| D02 | 创建 proposal.md | ✅ DONE (cancelled) | 2026-06-12 |
| D03 | 创建 tasks.md（S2_Clarified） | ✅ DONE (cancelled) | 2026-06-12 |

## 原 S2_Clarified 任务（已取消）

以下任务在 S2 阶段清晰化，但变更已取消，不实施：

### Phase 1 — TaskRegistry 核心

| ID | 任务 | L4 | L5 | 估行 | 状态 |
|----|------|-----|-----|------|------|
| T1 | 定义 `TaskRegistry` 接口 + 内存模型 | L4-BE-CTX-TASK-REGISTRY | {T}-CTX-REG-01 | ~80 | ❌ CANCELLED |
| T2 | disk output：`AppendOutput` / `GetOutputDelta` | L4-BE-CTX-TASK-REGISTRY | {T}-CTX-REG-02 | ~100 | ❌ CANCELLED |
| T3 | `SetTerminal` + SessionQueue 入队 + notified 去重 | L4-BE-CTX-TASK-REGISTRY | {T}-CTX-REG-03, {T}-CTX-REG-04 | ~80 | ❌ CANCELLED |

### Phase 2 — 适配层

| ID | 任务 | L4 | L5 | 估行 | 状态 |
|----|------|-----|-----|------|------|
| T4 | BackgroundRegistry → TaskRegistry 适配（RunBackground） | L4-BE-CTX-TASK-REGISTRY | {T}-CTX-REG-01 | ~60 | ❌ CANCELLED |
| T5 | Wave SubAgentRunner / AgentToolRunner 注册 terminal | L4-BE-CTX-TASK-REGISTRY | {T}-CTX-REG-05 | ~80 | ❌ CANCELLED |
| T6 | DM-009 `task_output` / `task_stop` 改读 TaskRegistry | L4-BE-CTX-BG-OUTPUT | D2-S9-T17 | ~60 | ❌ CANCELLED |

### Phase 3 — QueryLoop 与 wave_completed

| ID | 任务 | L4 | L5 | 估行 | 状态 |
|----|------|-----|-----|------|------|
| T7 | `collectBackgroundTaskAttachments`（per-turn delta） | L4-BE-CTX-BG-ATTACHMENTS | — | ~80 | ❌ CANCELLED |
| T8 | `BuildWaveCompletedAttachment(sessionID)` 供 DM-007 T23 | L4-BE-CTX-TASK-REGISTRY | {T}-ORCH-22 | ~60 | ❌ CANCELLED |
| T9 | bootstrap 注册 GlobalTaskRegistry + devrix.yaml | L4-BE-CTX-TASK-REGISTRY | — | ~40 | ❌ CANCELLED |

### Phase 4 — 测试与 T 层登记

| ID | 任务 | L5 | 状态 |
|----|------|-----|------|
| T10 | 单元：delta / notified / List | {T}-CTX-REG-01~04 | ❌ CANCELLED |
| T11 | 集成：SubQuery background + task_output | {T}-CTX-REG-02 | ❌ CANCELLED |
| T12 | 登记 {T}-CTX-REG-01~05 → t-registry.md | ALL | ❌ CANCELLED |

## 取消原因

1. 6 天（2026-06-12 → 2026-06-18）未推进
2. 依赖项 "Wave Scheduler v1.2 T15" 未实施
3. 资源优先级 → 让位给其他活跃变更

## 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S2_Cancelled → Archived；12 个 T 点全部 CANCELLED。