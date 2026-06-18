---
demand-id: DM-20260612-010
title: Wave Git Worktree — 写并行可选 git 隔离与合并收尾
source: clawcode 能力对照（createAgentWorktree / hasWorktreeChanges / batch PR）
priority: P1
status: S2_Clarified
dsaft_domain: context-engine
created: 2026-06-12
---

# Wave Git Worktree — 写并行可选 git 隔离与合并收尾

## 1. 背景

DM-007（Wave Scheduler）澄清 Q3 选择 **3B：CLI Agent 与 SubAgent 默认同 WorkDir**，写冲突由 `ConflictGuard`（`conflict_group` + `file_scope`）在调度层规避。

该策略适合 **飞书 IM 多 Worker 协作**（共享代码库视图、卡片流式输出）。但对 **大规模机械 refactor**（clawcode `/batch` 类：5–30 路并行改码、各自开 PR），仅有 ConflictGuard 不够：

- glob 重叠检测是保守近似，Plan 标注错误会导致并行写冲突
- 同 WorkDir 下 CLI Agent 进程无法做 git 级隔离
- 合并收尾依赖人工 review，但 devrix 缺少 **worktree path/branch 回传** 协议

clawcode 模式：

- `isolation: "worktree"` → `createAgentWorktree(slug)`
- 完成 → `hasWorktreeChanges(headCommit)` → 无改动自动删除，有改动保留并 notification 带 `worktreePath`/`branch`
- Worker prompt 要求 `PR: <url>`，Coordinator 汇总

Devrix 现有 D2-S12 `worktree.Manager` 仅为 **mkdir 沙箱**（`~/.devrix/worktrees/{session}/{slug}`），不是 git worktree。

## 2. 问题陈述

| 场景 | DM-007 3B | 本需求 |
|------|-----------|--------|
| IM 5 卡并行协作 | ✅ 默认 shared + ConflictGuard | 不变 |
| Plan 标注 write-heavy / batch refactor | ⚠️ 仅靠 ConflictGuard | ✅ TaskNode 可选 `isolation: worktree` |
| Worker 无改动 | N/A | 自动 remove worktree |
| Worker 有 commit | N/A | keep + Artifact 带 path/branch/PR |
| 非 git 仓库 | N/A | 降级 shared + 告警 |

## 3. 澄清记录

### Q1: 是否推翻 DM-007 3B 默认策略？

**A**: **否**。`TaskNode.Isolation` 默认 `shared`；仅 Plan Engine 或 Leader 显式标注 `worktree` 时启用 git 隔离。

### Q2: 与 D2-S12 mkdir worktree 的关系？

**A**: **并存**：

- `shared` / Delegate 轻量沙箱 → 现有 `worktree.Manager.Enter`（mkdir）
- `worktree` 隔离模式 → 新增 `GitWorktreeAdapter`（或扩展 Manager），路径建议 `{BaseDir}/git/{session}/{slug}`

### Q3: 自动 git merge？

**A**: **Out of Scope**。合并策略为 **PR 级**（Worker 可选 `gh pr create`，Leader wave_completed 汇总 PR URL），不做 merge bot。

### Q4: 与 ConflictGuard 的关系？

**A**: **互补**。worktree 模式降低 file_scope 标注压力；shared 模式仍依赖 ConflictGuard。同一 wave 可混用两种 isolation。

### Q5: 无 git repo 时行为？

**A**: 降级为 `shared` + 结构化 warn；Task 标记 `isolation_degraded=true` 写入 Artifact。

## 4. L1–L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | context-engine | 上下文引擎 | 已有 |
| L2 | L2-CTX-WORKTREE-GIT | Git 级并行隔离 | **新增** |
| L3-BE | L3-BE-ORCH-WAVE-WORKTREE | Wave Worker worktree 生命周期 | **新增** |
| L4-BE | L4-BE-CTX-GIT-WORKTREE | GitWorktreeAdapter enter/exit/hasChanges | **新增** |
| L4-BE | L4-BE-ORCH-WAVE-ISOLATION | TaskNode.Isolation 解析 | **新增** |
| T 层 | {T}-CTX-WT-01 ~ 06 | 见 §6 | **草拟** |

## 5. 范围

### In Scope

- `TaskNode.Isolation`: `shared | worktree`（默认 `shared`）
- `GitWorktreeAdapter`：
  - `Enter(ctx, sessionID, slug, gitRoot)` → path, branch, headCommit
  - `HasChanges(path, headCommit)` → bool
  - `Exit(path, branch, keep)` → remove worktree + 可选删 branch
- slug 校验（防 path traversal，对齐 clawcode `validateWorktreeSlug`）
- Wave `AgentToolRunner` / Delegate `forkWorker`：worktree 模式设置 `WorkerRunSpec.WorkDir`
- `Artifact` 扩展：`WorktreePath`, `Branch`, `PRURL`（从 worker result 解析 `PR:` 行）
- Plan Engine prompt 约束：write-heavy 单元应标注 `isolation: worktree` + 独立 file_scope
- wave_completed / task notification 携带 worktree 信息（供 Leader 汇总）

### Out of Scope

- 自动 git merge / conflict resolution bot
- 替代 ConflictGuard（shared 模式仍必需）
- 跨 session worktree 复用（resume 已有 worktree 为 P2）
- hook-based worktree（clawcode WorktreeCreate hook）— P2

## 6. 验收标准

### P0

| ID | 标准 |
|----|------|
| AC1 | `Isolation=worktree` 的 Task 在独立 git worktree 目录执行，主 WorkDir 无未预期改动 |
| AC2 | Worker 无 git 改动时 Exit 自动 remove worktree 与临时 branch |
| AC3 | Worker 有改动时 Exit keep，Artifact 含 `WorktreePath` + `Branch` |
| AC4 | `Isolation=shared` 行为与 DM-007 v1.0 一致（回归） |
| AC5 | 非 git 仓库降级 shared，Artifact 含 `isolation_degraded` |

### P1

| ID | 标准 |
|----|------|
| AC6 | Worker result 含 `PR: https://...` 时 Artifact.PRURL 正确解析 |
| AC7 | wave_completed 附件含 PR 汇总表（task_id / status / pr_url） |
| AC8 | stale agent worktree 清理策略（可配置 retention 天数） |

### T 层测试点（草案，S3 登记 t-registry）

| T ID | Given-When-Then | 优先级 |
|-------|-----------------|--------|
| {T}-CTX-WT-01 | Given git repo When Enter worktree Then write 不污染主 WorkDir | P0 |
| {T}-CTX-WT-02 | Given worktree 无改动 When Exit Then 目录与 branch 已删除 | P0 |
| {T}-CTX-WT-03 | Given worktree 有新 commit When Exit Then path/branch 保留且 Artifact 可查 | P0 |
| {T}-CTX-WT-04 | Given Isolation=shared When dispatch Then 不使用 git worktree | P0 |
| {T}-CTX-WT-05 | Given 非 git repo When Isolation=worktree Then 降级 shared + warn | P0 |
| {T}-CTX-WT-06 | Given result 含 PR URL When 解析 Artifact Then PRURL 正确 | P1 |

## 7. 依赖

| 方向 | 需求 | 说明 |
|------|------|------|
| 前置 | DM-007 Wave Scheduler | TaskNode、AgentToolRunner、ArtifactStore |
| 前置 | D2-S12 worktree 模块 | mkdir 沙箱保留；本需求扩展 git 路径 |
| 下游 | DM-011 Task Registry | terminal notification 携带 worktree 字段 |
| 关联 | DM-006 Plan Engine | 物化 DAG 时写入 isolation 字段 |

## 8. clawcode 参照

- `clawcode/src/utils/worktree.ts` — createAgentWorktree, hasWorktreeChanges, removeAgentWorktree
- `clawcode/src/tools/AgentTool/AgentTool.tsx` — cleanupWorktreeIfNeeded
- `clawcode/src/skills/bundled/batch.ts` — 并行 worktree + PR 汇总流程
