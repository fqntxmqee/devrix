# Spec: Unified Task Registry

**Change ID:** devrix-unified-task-registry
**Demand ID:** DM-20260612-011
**Status:** S7_Archived (2026-06-18; S2_Cancelled)

## 1. 变更性质

为后台任务（wave / cron / agent）建立统一注册 + output delta 订阅抽象。变更在 S2 阶段取消。

## 2. 核心接口

```go
type UnifiedTaskRegistry interface {
    Register(spec TaskSpec) (TaskID, error)
    Get(taskID TaskID) (TaskState, error)
    List(filter TaskFilter) ([]TaskState, error)
    Cancel(taskID TaskID) error
    Subscribe(taskID TaskID) (<-chan OutputDelta, error)
}
```

## 3. 适配器范围

- WaveAdapter：wave 调度任务
- CronAdapter：cron 定时任务
- AgentAdapter：agent 执行任务

## 4. 上游约束

- 不替换底层调度器
- 不修改任务执行逻辑

## 5. 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S2_Cancelled → Archived；草案保留作为未来重开参考。