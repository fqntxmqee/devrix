# Delta: Communication Layer V3 - Feature Completion

**Change ID:** devrix-v3
**Affects:** communication layer, milestone, task flow, adapters, UI
**Based on:** devrix-v2 (V2)

---

## ADDED

### Requirement: Milestone DAG

任务里程碑有向无环图，支持复杂任务的拆解和进度追踪。

#### Scenario: Create Milestone
- GIVEN task is being planned
- WHEN CreateMilestone is called
- THEN milestone is created with unique ID
- AND dependencies are recorded
- AND status is 'pending'

#### Scenario: Update Milestone Progress
- GIVEN milestone is in progress
- WHEN UpdateProgress is called with 0.0-1.0
- THEN progress is updated
- AND parent milestones are notified

#### Scenario: Milestone Completion
- GIVEN milestone progress reaches 1.0
- WHEN CheckCompletion is called
- THEN status changes to 'completed'
- AND dependent milestones are unblocked

#### Scenario: Cycle Detection
- GIVEN new milestone has circular dependency
- WHEN AddDependency is called
- THEN error is returned
- AND dependency is not added

```
Milestone Structure:
- ID: unique identifier
- TaskID: parent task identifier
- Name: milestone name
- Description: detailed description
- Status: pending | in_progress | completed | failed
- Progress: 0.0 - 1.0
- Dependencies: []string (milestone IDs that must complete first)
- CreatedAt: timestamp
- UpdatedAt: timestamp
```

---

### Requirement: Task Flow Manager

基于 Milestone 的任务流程管理器。

#### Scenario: Create TaskFlow
- GIVEN multiple milestones need coordination
- WHEN NewTaskFlow is called with milestones
- THEN TaskFlow is created
- AND execution order is computed

#### Scenario: Execute TaskFlow
- GIVEN TaskFlow is created
- WHEN Execute is called
- THEN milestones are processed in dependency order
- AND progress is updated on each completion

#### Scenario: TaskFlow Progress
- GIVEN TaskFlow is executing
- WHEN GetProgress is called
- THEN overall progress (0.0-1.0) is returned
- AND per-milestone status is included

#### Scenario: TaskFlow Failure
- GIVEN milestone fails during execution
- WHEN milestone status changes to 'failed'
- THEN TaskFlow status becomes 'failed'
- AND remaining dependent milestones are marked as 'blocked'

---

### Requirement: Milestone Events

任务里程碑相关的事件。

#### Scenario: Emit milestone.created
- GIVEN new milestone is created
- WHEN CreateMilestone completes
- THEN event 'milestone.created' is emitted

#### Scenario: Emit milestone.updated
- GIVEN milestone progress changes
- WHEN UpdateProgress is called
- THEN event 'milestone.updated' is emitted

#### Scenario: Emit milestone.completed
- GIVEN milestone reaches 100%
- WHEN status changes to 'completed'
- THEN event 'milestone.completed' is emitted

#### Scenario: Emit milestone.failed
- GIVEN milestone execution fails
- WHEN status changes to 'failed'
- THEN event 'milestone.failed' is emitted

---

### Requirement: 钉钉 Adapter

钉钉消息平台适配器。

#### Scenario: DingTalk WebSocket connection
- GIVEN DingTalk adapter starts
- WHEN Connect is called
- THEN WebSocket connection is established
- AND bot info is fetched

#### Scenario: Receive DingTalk message
- GIVEN user sends message in DingTalk
- WHEN message event is received
- THEN message is extracted
- AND routed to gateway

#### Scenario: Send message to DingTalk
- GIVEN gateway sends outbound message
- WHEN SendMessage is called
- THEN message is sent to DingTalk user

---

### Requirement: UI Component Library

统一的 UI 组件，用于跨平台展示。

#### Scenario: Render MilestoneCard
- GIVEN milestone data
- WHEN RenderMilestoneCard is called
- THEN card with name, progress, status is rendered
- AND platform-specific format is returned

#### Scenario: Render ProgressBar
- GIVEN progress value (0.0-1.0)
- WHEN RenderProgressBar is called
- THEN progress bar string is returned
- AND format is platform-appropriate

#### Scenario: Render StatusBadge
- GIVEN status string
- WHEN RenderStatusBadge is called
- THEN styled status badge is returned
- AND color-coding is applied

---

### Requirement: Multi-Instance Support

多实例部署支持。

#### Scenario: Instance Registration
- GIVEN new instance starts
- WHEN RegisterInstance is called
- THEN instance is registered in cluster
- AND heartbeat starts

#### Scenario: Instance Health Check
- GIVEN instance is running
- WHEN health check interval elapses
- THEN health status is updated
- AND unhealthy instances are flagged

#### Scenario: Prometheus Metrics
- GIVEN instances are running
- WHEN /metrics endpoint is called
- THEN Prometheus-formatted metrics are returned
- AND includes: requests_total, session_count, response_time

---

## MODIFIED

### Modified: Session Entity

V3 新增 MilestoneID 字段。

```go
type Session struct {
    SessionID    string
    ShortID     string
    RequestID   string
    AdapterID   string
    MilestoneID string      // V3: 当前里程碑 ID
    // ... existing fields
}
```

### Modified: PermissionRequest Entity

V3 新增 ToolDescription 字段。

```go
type PermissionRequest struct {
    ID              string
    SessionID       string
    ToolName        string
    ToolDescription string // V3: 工具描述
    InputPreview    string
    RiskLevel       RiskLevel
    // ... existing fields
}
```

---

## REMOVED

(None)

---

## V1/V2/V3 Feature Matrix

| Feature | V1 | V2 | V3 |
|---------|-----|-----|-----|
| CLI Adapter | ✅ | ✅ | ✅ |
| IM Adapter (飞书) | ❌ | ✅ | ✅ |
| **IM Adapter (钉钉)** | ❌ | ❌ | ✅ |
| WebSocket | ❌ | ✅ | ✅ |
| ShortId | ❌ | ✅ | ✅ |
| Auth | ❌ | ✅ | ✅ |
| Heartbeat | ❌ | ✅ | ✅ |
| connection.lost/restored | ❌ | ✅ | ✅ |
| FileSessionStore | ✅ | ✅ | ✅ |
| Session idle timeout | ✅ | ✅ | ✅ |
| Command Handler | ✅ | ✅ | ✅ |
| Permission pipeline | ✅ | ✅ | ✅ |
| **Milestone DAG** | ❌ | ❌ | ✅ |
| **Task Flow** | ❌ | ❌ | ✅ |
| **UI 组件体系** | ❌ | ❌ | ✅ |
| **多实例部署** | ❌ | ❌ | ✅ |
| Rate Limiting | ❌ | ✅ | ✅ |

---

## Dependencies

- V3 依赖 V2 的 Auth、Connection Manager
- V3 依赖 `github.com/prometheus/client_golang` 用于 metrics
- V3 的钉钉适配器需要：`github.com/open-dingtalk/dingtalk-stream-sdk-go`

---

## Backward Compatibility

V3 保持向后兼容：
- V2 Session 仍可使用（MilestoneID 字段可选）
- V2 Adapter 继续工作
- API 保持不变
