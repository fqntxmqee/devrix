# Design: Wave Git Worktree Isolation

**Change ID:** devrix-wave-worktree-isolation
**Demand ID:** DM-20260612-010

> **归档说明 (2026-06-18):** 设计仅停留在提案草案；变更已取消。

## 1. 设计目标

为 Wave worker 提供**可选的 git worktree 隔离**，解决写并发的文件竞争问题。

## 2. 核心抽象

### 2.1 TaskNode.Isolation

```go
type TaskNode struct {
    ID         string
    Capability string
    Inputs     map[string]any
    Isolation  IsolationMode  // 新增字段
}

type IsolationMode string

const (
    IsolationShared   IsolationMode = "shared"   // 默认
    IsolationWorktree IsolationMode = "worktree"
)
```

### 2.2 GitWorktreeAdapter

```go
type GitWorktreeAdapter interface {
    // Enter 创建 worktree 并返回 workDir
    Enter(ctx context.Context, opts EnterOptions) (EnterResult, error)
    
    // HasChanges 检查 worktree 是否有未合并的变更
    HasChanges(ctx context.Context, workDir string) (bool, error)
    
    // Exit 清理 worktree，可选 merge
    Exit(ctx context.Context, workDir string, opts ExitOptions) error
}

type EnterOptions struct {
    RepoPath string
    Branch   string  // worktree 的分支名
    BaseRef  string  // 从哪个 ref 创建 worktree
}

type EnterResult struct {
    WorkDir   string
    Branch    string
    WorktreePath string
}

type ExitOptions struct {
    Merge     bool   // 是否合并回主分支
    CleanupBranch bool  // 是否清理 worktree 分支
    Force     bool   // 强制清理（即使有冲突）
}
```

### 2.3 Scheduler 集成

```go
func (s *Scheduler) dispatchWorker(ctx context.Context, node *TaskNode) error {
    var workDir string
    var cleanup func() error
    
    if node.Isolation == IsolationWorktree {
        result, err := s.worktreeAdapter.Enter(ctx, EnterOptions{
            RepoPath: s.repoPath,
            Branch:   slugify(node.ID),
            BaseRef:  "HEAD",
        })
        if err != nil {
            return fmt.Errorf("worktree enter: %w", err)
        }
        workDir = result.WorkDir
        cleanup = func() error {
            return s.worktreeAdapter.Exit(ctx, workDir, ExitOptions{
                Merge: true,
                CleanupBranch: true,
            })
        }
    } else {
        workDir = s.sharedWorkDir
        cleanup = func() error { return nil }
    }
    
    defer func() { _ = cleanup() }()
    
    // ... 执行 worker
    return s.executeInWorkDir(ctx, node, workDir)
}
```

## 3. Artifact 扩展

```go
type Artifact struct {
    // ...existing fields
    WorktreePath string  // worktree 路径（仅 worktree 模式）
    Branch       string  // worktree 分支名（仅 worktree 模式）
    PRURL        string  // 若通过 PR 合并，PR URL（仅 worktree 模式）
    Isolation    IsolationMode  // 实际采用的隔离模式
}
```

## 4. 错误处理

| 场景 | 处理 |
|------|------|
| Enter 失败（git 错误） | 返回 error；wave 标记失败 |
| Worker 执行失败 | defer cleanup；不 merge；记录 dirty worktree |
| Merge 冲突 | 返回 error；保留 worktree；标记 `isolation_degraded` |
| 非 git 仓库 | 降级为 `shared`；Artifact.isolation = shared + degraded=true |

## 5. 安全考虑

- slug 校验：分支名仅允许 `[a-z0-9-]`，长度 ≤ 64
- 路径防逃逸：worktree 路径必须在 `.devrix/worktrees/` 下
- 权限控制：仅 devrix 进程可创建/清理 worktree

## 6. 上游依赖（缺失）

- **Wave Scheduler v1.0**：wave 抽象稳定版本（**未完成**）
- 需 devrix-d7-sa-refine 完成后 wave 层稳定

## 7. 取消决策

**Decision (2026-06-18):** 6 天未推进；依赖项未完成；写并发未达痛点；变更取消。

## 8. 后续路径

- Wave Scheduler v1.0 完成 → 重开本 change
- 可与 devrix-unified-task-registry 协同（但后者也已取消）
- 引用：demand-archive-index.md DM-20260612-010 行