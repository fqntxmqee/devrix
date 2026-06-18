# Spec: Wave Git Worktree Isolation

**Change ID:** devrix-wave-worktree-isolation
**Demand ID:** DM-20260612-010
**Status:** S7_Archived (2026-06-18; S2_Cancelled)

## 1. 变更性质

为 Wave worker 提供可选 git worktree 隔离，解决写并发文件竞争。变更在 S2 阶段取消。

## 2. 核心抽象

```go
type IsolationMode string

const (
    IsolationShared   IsolationMode = "shared"
    IsolationWorktree IsolationMode = "worktree"
)

type GitWorktreeAdapter interface {
    Enter(ctx context.Context, opts EnterOptions) (EnterResult, error)
    HasChanges(ctx context.Context, workDir string) (bool, error)
    Exit(ctx context.Context, workDir string, opts ExitOptions) error
}
```

## 3. 涉及域

- D2 Context Engine（wave 调度）
- D1 Orchestration（wave 抽象）
- D4 Multi-Agent（forkWorker 可选 worktree）

## 4. 上游约束

- 不替换 Wave Scheduler
- 保留 `shared` 模式作为默认
- 非 git 仓库降级为 `shared`

## 5. 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S2_Cancelled → Archived；草案保留作为未来重开参考。