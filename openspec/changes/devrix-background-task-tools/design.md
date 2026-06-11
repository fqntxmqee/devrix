# Design: Background Task 工具

**Demand ID:** DM-20260611-009

## 1. 数据模型扩展

```go
// query/background.go
type BackgroundTask struct {
    ID        string
    SessionID string
    AgentID   string
    AgentName string
    Status    string // running | completed | failed | cancelled
    Result    string
    Error     string
    StartedAt time.Time
    EndedAt   time.Time
    cancel    context.CancelFunc // 不序列化
}

type BackgroundRegistry struct {
    mu    sync.RWMutex
    tasks map[string]*BackgroundTask
}
```

## 2. Cancel 协议

```go
func (r *BackgroundRegistry) RegisterWithCancel(...) (taskID string, cancel context.CancelFunc)
func (r *BackgroundRegistry) Cancel(taskID string) error  // idempotent
func (r *BackgroundRegistry) List(sessionID string) []*BackgroundTask
```

`RunBackground` 改造：

```go
ctx, cancel := context.WithCancel(ctx)
taskID := reg.RegisterWithCancel(..., cancel)
go func() {
    defer cancel()
    res, err := Run(ctx, deps, params)
    if errors.Is(err, context.Canceled) {
        reg.CompleteCancelled(taskID, q)
        return
    }
    reg.Complete(taskID, ...)
}()
```

## 3. LLM 工具

### task_stop

```json
{ "task_id": "bg_abc123" }
```

→ `registry.Cancel(task_id)` → status=cancelled

### task_output

```json
{
  "task_id": "bg_abc123",
  "block": false,
  "timeout_ms": 30000
}
```

| block | 行为 |
|-------|------|
| false | 立即返回当前 status + partial result |
| true | wait until terminal or timeout |

输出格式（文本 tool result）：

```
task_id: bg_abc123
status: running|completed|failed|cancelled
agent: explore-auth
output: ...
```

## 4. 与 TaskManager 区分

| 工具 | 存储 | 用途 |
|------|------|------|
| task_create/get/list/update | TaskManager 磁盘 | Plan DAG / 用户任务图 |
| task_stop / task_output | BackgroundRegistry 内存 | 异步 SubQuery 生命周期 |

Prompt 中需明确：「task_id 以 `bg_` 开头为 background SubQuery；`task_` 开头为 Plan 任务图」。

## 5. Wave Scheduler 复用

```go
// orchestration/wave/runner.go
type WorkerHandle struct {
    TaskID string
    Cancel context.CancelFunc
}

func (s *WaveScheduler) CancelWorker(taskID string) error {
    return s.registry.Cancel(taskID) // 同一 BackgroundRegistry 或 WorkerRegistry 接口
}
```

Leader `/new`、session 结束 → `WaveScheduler.CancelAll(sessionID)`。

## 6. clawcode 差异

| 点 | clawcode | Devrix |
|----|----------|--------|
| 存储 | AppState + disk output | 内存 registry；P2 可选 persist |
| Task 类型 | local_bash / local_agent / remote | 仅 SubQuery（Wave 扩展 CLI） |
| block poll | 600s max | 默认 30s，max 600s |

## 7. 测试

| 测试 | L5 |
|------|-----|
| stop running task → cancelled | L5-2-9-01 |
| output block=false on running | L5-2-9-02 |
| output block=true waits complete | L5-2-9-03 |
| cancel suppresses completed notification | L5-2-9-04 |
