# Tasks: Wave Git Worktree Isolation

**Change ID:** devrix-wave-worktree-isolation
**Demand ID:** DM-20260612-010
**Status:** S7_Archived (2026-06-18; S2_Cancelled; not implemented)

> **归档说明 (2026-06-18):** 变更在 S2 阶段取消；依赖项 "Wave Scheduler v1.0" 未完成。本文件保留原 S2_Clarified 任务表作为历史参考。

## S0 — Demand 创建（已完成）

| ID | 任务 | 状态 | 日期 |
|----|------|------|------|
| D01 | 创建 demand.md | ✅ DONE | 2026-06-12 |
| D02 | 创建 proposal.md | ✅ DONE (cancelled) | 2026-06-12 |
| D03 | 创建 tasks.md（S2_Clarified） | ✅ DONE (cancelled) | 2026-06-12 |

## 原 S2_Clarified 任务（已取消）

以下任务在 S2 阶段清晰化，但变更已取消，不实施：

### Phase 1 — 模型与 Git 适配

| ID | 任务 | L4 | L5 | 估行 | 状态 |
|----|------|-----|-----|------|------|
| T1 | `TaskNode.Isolation` 字段 + Validate（shared\|worktree） | L4-BE-ORCH-WAVE-ISOLATION | {T}-CTX-WT-04 | ~40 | ❌ CANCELLED |
| T2 | `GitWorktreeAdapter`：Enter / HasChanges / Exit | L4-BE-CTX-GIT-WORKTREE | {T}-CTX-WT-01~03 | ~150 | ❌ CANCELLED |
| T3 | slug 校验 + 路径防逃逸 | L4-BE-CTX-GIT-WORKTREE | {T}-CTX-WT-01 | ~40 | ❌ CANCELLED |

### Phase 2 — Wave 集成

| ID | 任务 | L4 | L5 | 估行 | 状态 |
|----|------|-----|-----|------|------|
| T4 | Scheduler dispatch：解析 Isolation，设置 WorkDir | L4-BE-ORCH-WAVE-ISOLATION | {T}-CTX-WT-04 | ~60 | ❌ CANCELLED |
| T5 | AgentToolRunner worktree WorkDir + defer Exit | L4-BE-ORCH-WAVE-ISOLATION | {T}-CTX-WT-02, {T}-CTX-WT-03 | ~80 | ❌ CANCELLED |
| T6 | Delegate forkWorker 可选 worktree（与 D4 对齐） | L4-BE-CTX-GIT-WORKTREE | {T}-CTX-WT-01 | ~60 | ❌ CANCELLED |
| T7 | 非 git 降级 shared + Artifact.isolation_degraded | L4-BE-ORCH-WAVE-ISOLATION | {T}-CTX-WT-05 | ~40 | ❌ CANCELLED |

### Phase 3 — Artifact 与汇总

| ID | 任务 | L4 | L5 | 估行 | 状态 |
|----|------|-----|-----|------|------|
| T8 | Artifact 扩展 WorktreePath / Branch / PRURL | L4-BE-ORCH-WAVE-ISOLATION | {T}-CTX-WT-03 | ~40 | ❌ CANCELLED |
| T9 | 从 worker result 解析 `PR:` 行 | L4-BE-ORCH-WAVE-ISOLATION | {T}-CTX-WT-06 | ~40 | ❌ CANCELLED |
| T10 | wave_completed PR 汇总表（配合 DM-007 / DM-011） | L3-BE-ORCH-DISPATCH | {T}-CTX-WT-06 | ~60 | ❌ CANCELLED |

### Phase 4 — 测试与 T 层登记

| ID | 任务 | L5 | 状态 |
|----|------|-----|------|
| T11 | 单元：Enter/Exit/HasChanges（temp git repo） | {T}-CTX-WT-01~03 | ❌ CANCELLED |
| T12 | 集成：Wave stub runner + isolation=worktree | {T}-CTX-WT-04, {T}-CTX-WT-05 | ❌ CANCELLED |
| T13 | 登记 {T}-CTX-WT-01~06 → t-registry.md | ALL | ❌ CANCELLED |

## 取消原因

1. 6 天（2026-06-12 → 2026-06-18）未推进
2. 依赖项 "Wave Scheduler v1.0" 未完成（devrix-d7-sa-refine 已重命名 D7 域，但 Wave Scheduler 自身未独立版本化）
3. 资源优先级 → 让位给其他活跃变更

## 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S2_Cancelled → Archived；13 个 T 点全部 CANCELLED。