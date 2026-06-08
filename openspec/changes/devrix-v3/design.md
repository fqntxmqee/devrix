# Communication Layer V3 Design - Feature Completion

**Change ID:** devrix-v3
**Domain:** D1 - Communication (COMM)
**Status:** Draft
**Version:** 3.0

---

## 一、架构目标

### V3 业务目标

| 业务目标 | V1 | V2 | V3 |
|---------|-----|-----|-----|
| **Milestone DAG** | ❌ | ❌ | ✅ |
| **Task Flow** | ❌ | ❌ | ✅ |
| **钉钉 Adapter** | ❌ | ❌ | ✅ |
| **UI 组件体系** | ❌ | ❌ | ✅ |
| **多实例部署** | ❌ | ❌ | ✅ |

---

## 二、Milestone DAG 设计

### 2.1 领域模型

```go
// internal/shared/types/milestone.go

// MilestoneStatus represents the status of a milestone
type MilestoneStatus string

const (
    MilestoneStatusPending    MilestoneStatus = "pending"
    MilestoneStatusInProgress MilestoneStatus = "in_progress"
    MilestoneStatusCompleted  MilestoneStatus = "completed"
    MilestoneStatusFailed    MilestoneStatus = "failed"
)

// Milestone represents a task milestone (Entity)
type Milestone struct {
    ID            string           // 唯一标识
    TaskID        string           // 所属任务 ID
    Name          string           // 里程碑名称
    Description   string           // 详细描述
    Status        MilestoneStatus  // pending | in_progress | completed | failed
    Progress      float64          // 进度 0.0-1.0
    Dependencies  []string        // 依赖的里程碑 IDs
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// MilestoneDAG represents a directed acyclic graph of milestones
type MilestoneDAG struct {
    ID               string
    RootMilestoneID  string
    Milestones       map[string]*Milestone
}
```

### 2.2 DAG 操作

```go
// internal/layers/communication/milestone/service.go

type IMilestoneService interface {
    Create(milestone *Milestone) error
    Get(id string) (*Milestone, error)
    UpdateProgress(id string, progress float64) error
    Complete(id string) error
    Fail(id string, reason string) error
    AddDependency(milestoneID, dependencyID string) error
    GetExecutionOrder() ([]*Milestone, error)
}
```

### 2.3 循环检测

```go
// 使用 DFS 检测循环依赖
func hasCycle(dag *MilestoneDAG, from, to string) bool {
    visited := make(map[string]bool)
    return dfs(dag, from, to, visited)
}

func dfs(dag *MilestoneDAG, current, target string, visited map[string]bool) bool {
    if current == target {
        return true
    }
    if visited[current] {
        return false
    }
    visited[current] = true

    for _, dep := range dag.Milestones[current].Dependencies {
        if dfs(dag, dep, target, visited) {
            return true
        }
    }
    return false
}
```

---

## 三、Task Flow 设计

### 3.1 领域模型

```go
// internal/shared/types/taskflow.go

// TaskFlowStatus represents the status of a task flow
type TaskFlowStatus string

const (
    TaskFlowStatusPending    TaskFlowStatus = "pending"
    TaskFlowStatusRunning   TaskFlowStatus = "running"
    TaskFlowStatusCompleted TaskFlowStatus = "completed"
    TaskFlowStatusFailed   TaskFlowStatus = "failed"
)

// TaskFlow represents a task execution flow (Aggregate)
type TaskFlow struct {
    ID               string
    Name             string
    Description      string
    Milestones       []*Milestone        // 有序的里程碑列表
    RootMilestoneID  string
    Status           TaskFlowStatus
    OverallProgress  float64            // 0.0-1.0
    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

### 3.2 执行流程

```
┌──────────────────────────────────────────────────────────────┐
│                      TaskFlow Execution                       │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  Execute() ──▶ Milestone[0] ──▶ Milestone[1] ──▶ ...        │
│       │              │               │                       │
│       │              ▼               ▼                       │
│       │         in_progress     pending (waiting deps)       │
│       │              │               │                       │
│       │              ▼               ▼                       │
│       │          completed      in_progress                  │
│       │              │               │                       │
│       └───────▶ Progress Updated ◀─────┘                       │
│                                                               │
│  On Milestone[0] Complete:                                   │
│    - Update OverallProgress                                   │
│    - Emit milestone.completed                                 │
│    - Unblock Milestone[1]                                    │
│    - Start Milestone[1]                                       │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

---

## 四、钉钉 Adapter 设计

### 4.1 架构

```
┌─────────────────────────────────────────────────────────────┐
│                    DingTalkAdapter                           │
├─────────────────────────────────────────────────────────────┤
│  client: *dingtalk.Client                                   │
│  wsClient: *dingtalk.WSClient                              │
│  sessionMap: sync.Map                                       │
├─────────────────────────────────────────────────────────────┤
│  Start(ctx)                                                 │
│    ├─ fetchBotInfo() → 获取 robot code                      │
│    └─ startStreamMode() → 启动长连接                        │
│                                                               │
│  onMessage(topic, payload)                                  │
│    ├─ parseMessage()                                        │
│    └─ routeToGateway()                                      │
│                                                               │
│  SendMessage(chatID, content)                              │
│    └─ client.Message.Send()                                 │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 消息格式

```go
// 接收消息格式
type DingTalkMessage struct {
    Topic string          `json:"topic"`
    Data  DingTalkData   `json:"data"`
}

type DingTalkData struct {
    ConversationID string `json:"conversationId"`
    SenderNick    string `json:"senderNick"`
    Content       string `json:"content"`
    msgId         string `json:"msgId"`
}
```

---

## 五、UI 组件体系

### 5.1 组件接口

```go
// internal/layers/communication/renderers/components.go

type Component interface {
    Render(ctx context.Context) (string, error)
    RenderPlatform(platform string) (string, error)
}

type MilestoneCard struct {
    Milestone *types.Milestone
    OnAction func(action string)
}

type ProgressBar struct {
    Progress float64
    Width    int
    ShowText bool
}

type StatusBadge struct {
    Status  string
    Color   string
}
```

### 5.2 CLI 渲染

```
┌──────────────────────────────────────────────────────────────┐
│  Milestone: [████████████████████░░░░░░░░] 80%                │
│  └─ 设计文档已完成                                           │
│  └─ 代码实现进行中                                          │
│                                                               │
│  Task Flow: ████████████████████░░░░ 4/5 milestones        │
└──────────────────────────────────────────────────────────────┘
```

### 5.3 飞书卡片

```json
{
  "elements": [
    {
      "tag": "markdown",
      "content": "**Milestone: 代码实现** 80%"
    },
    {
      "tag": "hr"
    },
    {
      "tag": "column_set",
      "flex_mode": "baseline",
      "columns": [
        {"tag": "column", "width": "stretch", "content": "▓▓▓▓▓▓▓▓▓▓░░░░"},
        {"tag": "column", "width": "auto", "content": "80%"}
      ]
    }
  ]
}
```

---

## 六、多实例支持

### 6.1 实例注册

```go
// internal/layers/communication/instance/registry.go

type InstanceInfo struct {
    ID        string
    Name      string
    Address   string
    Port      int
    Status    string // "healthy" | "unhealthy"
    StartedAt time.Time
    LastSeen  time.Time
}

type InstanceRegistry interface {
    Register(ctx context.Context, info *InstanceInfo) error
    Unregister(ctx context.Context, id string) error
    GetInstances(ctx context.Context) ([]*InstanceInfo, error)
    HealthCheck(ctx context.Context, id string) error
}
```

### 6.2 Prometheus Metrics

```
# HELP devrix_requests_total Total number of requests
# TYPE devrix_requests_total counter
devrix_requests_total{adapter="feishu",status="success"} 1234
devrix_requests_total{adapter="cli",status="success"} 5678

# HELP devrix_active_sessions Current number of active sessions
# TYPE devrix_active_sessions gauge
devrix_active_sessions{adapter="feishu"} 42
devrix_active_sessions{adapter="cli"} 15

# HELP devrix_response_time_seconds Response time in seconds
# TYPE devrix_response_time_seconds histogram
devrix_response_time_seconds_bucket{adapter="feishu",le="0.1"} 1000
devrix_response_time_seconds_bucket{adapter="feishu",le="0.5"} 1500
devrix_response_time_seconds_sum{adapter="feishu"} 1234.5
devrix_response_time_seconds_count{adapter="feishu"} 2000
```

### 6.3 指标端点

```
GET /metrics
```

返回 Prometheus 格式的指标数据。

---

## 七、项目结构

```
devrix/
├── internal/
│   ├── shared/
│   │   ├── types/
│   │   │   ├── milestone.go     # Milestone 类型
│   │   │   ├── taskflow.go      # TaskFlow 类型
│   │   │   └── events.go        # 新增 milestone 事件
│   │   └── config/
│   │       └── instance.go      # 实例配置
│   └── layers/
│       └── communication/
│           ├── milestone/
│           │   ├── service.go   # Milestone 服务
│           │   ├── dag.go       # DAG 操作
│           │   └── taskflow.go  # TaskFlow 服务
│           ├── adapters/
│           │   ├── dingtalk.go  # 钉钉适配器
│           │   └── dingtalk_test.go
│           ├── renderers/
│           │   ├── components.go    # UI 组件接口
│           │   ├── milestone_card.go # Milestone 卡片
│           │   ├── progress_bar.go # 进度条
│           │   └── status_badge.go  # 状态徽章
│           ├── instance/
│           │   ├── registry.go   # 实例注册
│           │   └── health.go    # 健康检查
│           └── metrics/
│               ├── collector.go  # 指标收集
│               └── exporter.go   # Prometheus 导出
```

---

## 八、错误处理

### 8.1 新增错误码

| 错误码 | 说明 |
|--------|------|
| COMM_MILESTONE_NOT_FOUND | 里程碑不存在 |
| COMM_MILESTONE_CYCLE | 循环依赖检测 |
| COMM_MILESTONE_BLOCKED | 依赖未完成 |
| COMM_TASKFLOW_RUNNING | TaskFlow 已在运行 |
| COMM_INSTANCE_NOT_FOUND | 实例不存在 |
| COMM_METRICS_COLLECT | 指标收集失败 |

---

## 九、测试策略

### 9.1 单元测试

- Milestone DAG 测试
- TaskFlow 执行测试
- UI 组件渲染测试

### 9.2 集成测试

- 多 Milestone 协作测试
- TaskFlow 失败恢复测试
- 钉钉消息流测试

### 9.3 性能测试

- 大量 Milestone 创建性能
- 并发 Session 性能
