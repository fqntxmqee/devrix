# Design: Unified Task Registry

**Change ID:** devrix-unified-task-registry
**Demand ID:** DM-20260612-011

> **归档说明 (2026-06-18):** 设计仅停留在提案草案；变更已取消。

## 1. 设计目标

为后台任务（wave / cron / agent task）提供统一注册与 output delta 订阅抽象。

## 2. 核心接口

```go
type UnifiedTaskRegistry interface {
    Register(spec TaskSpec) (TaskID, error)
    Get(taskID TaskID) (TaskState, error)
    List(filter TaskFilter) ([]TaskState, error)
    Cancel(taskID TaskID) error
    Subscribe(taskID TaskID) (<-chan OutputDelta, error)
}

type TaskSpec struct {
    ID       TaskID
    Type     string  // "wave" / "cron" / "agent"
    Owner    string
    Payload  []byte
    Schedule ScheduleSpec
}

type TaskState struct {
    ID       TaskID
    Status   TaskStatus  // pending / running / completed / failed / cancelled
    Result   []byte
    Error    string
    UpdatedAt time.Time
}

type OutputDelta struct {
    TaskID    TaskID
    Seq       int64
    Delta     []byte
    Timestamp time.Time
}
```

## 3. 适配器设计

### 3.1 WaveAdapter

```go
type WaveAdapter struct {
    waveScheduler WaveScheduler
    store         OutputStore
}

func (a *WaveAdapter) Register(spec TaskSpec) (TaskID, error) {
    return a.waveScheduler.Submit(spec)
}
```

### 3.2 CronAdapter

```go
type CronAdapter struct {
    cronScheduler CronScheduler
}

func (a *CronAdapter) Register(spec TaskSpec) (TaskID, error) {
    return a.cronScheduler.Add(spec)
}
```

### 3.3 AgentAdapter

```go
type AgentAdapter struct {
    agentRunner AgentRunner
}

func (a *AgentAdapter) Register(spec TaskSpec) (TaskID, error) {
    return a.agentRunner.Submit(spec)
}
```

## 4. Output Delta 流

订阅模式：
- 调用 `Subscribe(taskID)` 返回 `<-chan OutputDelta`
- 任务执行过程中，每产生一段输出 → 写入 store → 推送 delta
- 任务完成/取消 → channel 关闭

## 5. 上游依赖（缺失）

- **Wave Scheduler v1.2 T15**：wave 调度抽象（**未实施**）
- **Output Store**：output delta 持久化（**未实施**）

## 6. 取消决策

**Decision (2026-06-18):** 6 天未推进；依赖项未实施；变更取消。

## 7. 后续路径

- Wave Scheduler v1.2 实施 → 重开本 change
- 可与 devrix-wave-worktree-isolation 协同（但后者也已取消）
- 引用：demand-archive-index.md DM-20260612-011 行