# Tasks: Wave Git Worktree Isolation

**Demand ID:** DM-20260612-010  
**Status:** S2_Clarified

## Phase 1 — 模型与 Git 适配

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T1 | `TaskNode.Isolation` 字段 + Validate（shared\|worktree） | L4-BE-ORCH-WAVE-ISOLATION | L5-CTX-WT-04 | ~40 |
| T2 | `GitWorktreeAdapter`：Enter / HasChanges / Exit | L4-BE-CTX-GIT-WORKTREE | L5-CTX-WT-01~03 | ~150 |
| T3 | slug 校验 + 路径防逃逸 | L4-BE-CTX-GIT-WORKTREE | L5-CTX-WT-01 | ~40 |

## Phase 2 — Wave 集成

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T4 | Scheduler dispatch：解析 Isolation，设置 WorkDir | L4-BE-ORCH-WAVE-ISOLATION | L5-CTX-WT-04 | ~60 |
| T5 | AgentToolRunner worktree WorkDir + defer Exit | L4-BE-ORCH-WAVE-ISOLATION | L5-CTX-WT-02, L5-CTX-WT-03 | ~80 |
| T6 | Delegate forkWorker 可选 worktree（与 D4 对齐） | L4-BE-CTX-GIT-WORKTREE | L5-CTX-WT-01 | ~60 |
| T7 | 非 git 降级 shared + Artifact.isolation_degraded | L4-BE-ORCH-WAVE-ISOLATION | L5-CTX-WT-05 | ~40 |

## Phase 3 — Artifact 与汇总

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T8 | Artifact 扩展 WorktreePath / Branch / PRURL | L4-BE-ORCH-WAVE-ISOLATION | L5-CTX-WT-03 | ~40 |
| T9 | 从 worker result 解析 `PR:` 行 | L4-BE-ORCH-WAVE-ISOLATION | L5-CTX-WT-06 | ~40 |
| T10 | wave_completed PR 汇总表（配合 DM-007 / DM-011） | L3-BE-ORCH-DISPATCH | L5-CTX-WT-06 | ~60 |

## Phase 4 — 测试与 L5 登记

| ID | 任务 | L5 |
|----|------|-----|
| T11 | 单元：Enter/Exit/HasChanges（temp git repo） | L5-CTX-WT-01~03 |
| T12 | 集成：Wave stub runner + isolation=worktree | L5-CTX-WT-04, L5-CTX-WT-05 |
| T13 | 登记 L5-CTX-WT-01~06 → l5-registry.md | ALL |

## 依赖顺序

```
T1 → T2 → T3 → T4 → T5
              ↓
         T8 → T9 → T10
T6 可与 T5 并行
T7 在 T4 后
依赖 DM-007 WaveScheduler v1.0
```

## 建议 PR 拆分

1. **PR-1**: T1–T3 + T13（模型 + GitWorktreeAdapter）
2. **PR-2**: T4–T7（Wave/Delegate 集成）
3. **PR-3**: T8–T10 + T11–T12（Artifact + 测试）
