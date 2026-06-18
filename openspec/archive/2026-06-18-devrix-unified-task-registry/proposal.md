# Proposal: Unified Task Registry — 后台任务统一注册与 output delta

**Change ID:** devrix-unified-task-registry
**Demand ID:** DM-20260612-011
**Status:** S7_Archived (2026-06-18; S2_Cancelled; not implemented)
**Author:** Devrix Team
**Date:** 2026-06-12 → Cancelled 2026-06-18

> **取消原因 (2026-06-18):** 创建 6 天未推进；依赖项 "Wave Scheduler v1.2 T15" 未实施，导致本 change 缺少可挂载的调度抽象。归档为 S7_Archived（S2_Cancelled → Archived）。

## 1. Background

Devrix 后台任务（wave 调度、cron、用户发起的长任务）目前分散在多个 registry 中（wave registry、cron registry、agent task registry），缺乏统一抽象。期望建立 **Unified Task Registry**：
- 统一注册接口（不同后端可挂载）
- 统一查询接口（按 task_id / status / owner）
- 统一 output delta 接口（任务增量输出订阅）

## 2. Problem Statement

| 问题 | 影响 |
|------|------|
| 多个 task registry 并存 | 跨 registry 查询需写胶水代码 |
| 无统一 output delta | 长任务的增量输出无法实时订阅 |
| 无统一生命周期管理 | 任务取消、暂停、恢复 API 不一致 |
| 监控困难 | 任务指标分散 |

## 3. 提案范围（未实施）

### 3.1 UnifiedTaskRegistry 接口

```go
// internal/layers/contextengine/taskregistry/registry.go
type UnifiedTaskRegistry interface {
    Register(spec TaskSpec) (TaskID, error)
    Get(taskID TaskID) (TaskState, error)
    List(filter TaskFilter) ([]TaskState, error)
    Cancel(taskID TaskID) error
    Subscribe(taskID TaskID) (<-chan OutputDelta, error)
}
```

### 3.2 适配器

- WaveAdapter：适配现有 wave registry
- CronAdapter：适配 cron scheduler
- AgentAdapter：适配 agent task runner

### 3.3 Output Delta 流

```go
type OutputDelta struct {
    TaskID    TaskID
    Seq       int64
    Delta     []byte
    Timestamp time.Time
}
```

## 4. Non-Goals

- 不替换底层调度器（wave / cron / agent）
- 不引入新调度算法
- 不修改任务执行逻辑

## 5. 上游依赖

- **Wave Scheduler v1.2 T15**：抽象 wave 调度接口（**未实施**）

## 6. 取消决策

**Decision (2026-06-18):**
1. 6 天（2026-06-12 → 2026-06-18）未推进
2. 依赖项 "Wave Scheduler v1.2 T15" 未实施
3. 资源优先级 → 让位给 devrix-wave-worktree-isolation / devrix-tool-surface-contract 等活跃变更（但 wave-worktree-isolation 也已取消）

## 7. 后续路径

- 如 Wave Scheduler v1.2 实施 → 可重开本 change
- 引用：demand-archive-index.md DM-20260612-011 行

## 8. 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S2_Cancelled → Archived；不实施；依赖项就绪后可重开。