# Proposal: Wave Git Worktree — 写并行可选 git 隔离与合并收尾

**Change ID:** devrix-wave-worktree-isolation
**Demand ID:** DM-20260612-010
**Status:** S7_Archived (2026-06-18; S2_Cancelled; not implemented)
**Author:** Devrix Team
**Date:** 2026-06-12 → Cancelled 2026-06-18

> **取消原因 (2026-06-18):** 创建 6 天未推进；依赖项 "Wave Scheduler v1.0" 未完成，导致 wave 抽象未稳定，本 change 缺乏可挂载点。归档为 S7_Archived（S2_Cancelled → Archived）。

## 1. Background

Devrix Wave 调度支持多个 worker 并行执行写任务（如多个 Agent 同时改文件）。当前所有 worker 共享同一 WorkDir，**写并发需靠锁/序列化保证正确性**，效率受限。

期望引入**可选 git worktree 隔离**：
- Worker 可声明 `isolation: worktree`
- Scheduler 为该 worker 创建独立 git worktree
- Worker 在自己的 worktree 中写文件，互不冲突
- 完成后由 scheduler 合并回主分支

## 2. Problem Statement

| 问题 | 影响 |
|------|------|
| 写并发需序列化 | wave 吞吐受限 |
| worker 间文件竞争 | 需复杂锁机制 |
| 无法回滚单个 worker 的修改 | 出错需全量 rebase |
| worktree 是 git 原生方案 | 但缺乏 wave 层抽象 |

## 3. 提案范围（未实施）

### 3.1 TaskNode 扩展

```go
type TaskNode struct {
    // ...existing fields
    Isolation IsolationMode  // shared | worktree
}

type IsolationMode string

const (
    IsolationShared   IsolationMode = "shared"
    IsolationWorktree IsolationMode = "worktree"
)
```

### 3.2 GitWorktreeAdapter

```go
type GitWorktreeAdapter interface {
    Enter(ctx context.Context, branch string) (workDir string, err error)
    HasChanges(ctx context.Context, workDir string) (bool, error)
    Exit(ctx context.Context, workDir string, merge bool) error
}
```

### 3.3 Scheduler 集成

```go
// dispatch 时
if node.Isolation == IsolationWorktree {
    workDir, err := worktreeAdapter.Enter(ctx, slug)
    if err != nil { return err }
    defer worktreeAdapter.Exit(ctx, workDir, merge=true)
}
// ...继续 dispatch
```

### 3.4 Artifact 扩展

```go
type Artifact struct {
    // ...existing fields
    WorktreePath string
    Branch       string
    PRURL        string
}
```

## 4. Non-Goals

- 不替换 Wave Scheduler
- 不修改 git 协议
- 不强制所有 worker 使用 worktree（保留 `shared` 模式）

## 5. 上游依赖（缺失）

- **Wave Scheduler v1.0**：wave 抽象的稳定版本（**未完成**）

## 6. 取消决策

**Decision (2026-06-18):**
1. 6 天（2026-06-12 → 2026-06-18）未推进
2. 依赖项 "Wave Scheduler v1.0" 未完成
3. 写并发场景在当前 devrix 流量下未达痛点阈值
4. 资源优先级 → 让位给其他活跃变更

## 7. 后续路径

- 如 Wave Scheduler 稳定后 → 重开本 change
- 引用：demand-archive-index.md DM-20260612-010 行

## 8. 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S2_Cancelled → Archived；不实施；依赖项完成后可重开。