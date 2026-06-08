# Communication Layer V3 Tasks

**Change ID:** devrix-v3
**Domain:** D1 - Communication (COMM)
**Status:** Completed (S7 Archived)
**Version:** 3.0
**Based on:** delta.md, devrix-v2 design

---

## Task Overview

实现 V3 功能集：Milestone DAG、Task Flow、钉钉 Adapter、UI 组件、多实例支持。

共 **50 个子任务**，分为 6 个阶段。

---

## Phase 1: Milestone DAG

### 1.1 Milestone Types

**Owner:** internal/shared/types

- [ ] 1.1.1 Define MilestoneStatus type
  - Location: `internal/shared/types/milestone.go`
  - Status: MilestoneStatusPending, MilestoneStatusInProgress, MilestoneStatusCompleted, MilestoneStatusFailed

- [ ] 1.1.2 Define Milestone struct
  - Location: `internal/shared/types/milestone.go`
  - Fields: ID, TaskID, Name, Description, Status, Progress, Dependencies, CreatedAt, UpdatedAt

- [ ] 1.1.3 Define MilestoneDAG struct
  - Location: `internal/shared/types/milestone.go`
  - Fields: Milestones map, RootMilestoneID

### 1.2 Milestone Operations

**Owner:** internal/layers/communication/milestone

- [ ] 1.2.1 Create IMilestoneService interface
  - Location: `internal/layers/communication/milestone/service.go`
  - Methods: Create, Get, UpdateProgress, Complete, Fail, GetDependencies

- [ ] 1.2.2 Create MilestoneService implementation
  - Location: `internal/layers/communication/milestone/service.go`

- [ ] 1.2.3 Implement Create(milestone)
  - Validate no circular dependencies
  - Add to DAG
  - Emit milestone.created event

- [ ] 1.2.4 Implement UpdateProgress(id, progress)
  - Update progress (0.0-1.0)
  - Emit milestone.updated event

- [ ] 1.2.5 Implement Complete(id)
  - Set status to completed
  - Emit milestone.completed event
  - Unblock dependent milestones

- [ ] 1.2.6 Implement Fail(id, error)
  - Set status to failed
  - Emit milestone.failed event
  - Mark dependents as blocked

- [ ] 1.2.7 Implement GetExecutionOrder()
  - Return milestones in topological order
  - Respect dependencies

### 1.3 Cycle Detection

- [ ] 1.3.1 Detect circular dependencies
  - When AddDependency is called
  - Return error if cycle detected

### 1.4 Milestone Tests

- [ ] 1.4.1 Test Milestone creation
- [ ] 1.4.2 Test Progress update
- [ ] 1.4.3 Test Completion
- [ ] 1.4.4 Test Failure propagation
- [ ] 1.4.5 Test Cycle detection
- [ ] 1.4.6 Test Execution order

---

## Phase 2: Task Flow Manager

### 2.1 TaskFlow Types

**Owner:** internal/shared/types

- [ ] 2.1.1 Define TaskFlow struct
  - Location: `internal/shared/types/taskflow.go`
  - Fields: ID, Name, Milestones, RootMilestoneID, Status, CreatedAt

- [ ] 2.1.2 Define TaskFlowStatus type
  - Status: TaskFlowStatusPending, TaskFlowStatusRunning, TaskFlowStatusCompleted, TaskFlowStatusFailed

### 2.2 TaskFlow Service

**Owner:** internal/layers/communication/milestone

- [ ] 2.2.1 Create ITaskFlowService interface
  - Location: `internal/layers/communication/milestone/taskflow.go`
  - Methods: Create, Execute, GetProgress, GetStatus, Abort

- [ ] 2.2.2 Create TaskFlowService implementation
  - Manages milestone execution

- [ ] 2.2.3 Implement Execute()
  - Process milestones in order
  - Update progress on completion
  - Handle failures

- [ ] 2.2.4 Implement GetProgress()
  - Calculate overall progress
  - Return per-milestone status

- [ ] 2.2.5 Implement Abort()
  - Stop running task flow
  - Mark remaining as failed

### 2.3 TaskFlow Events

- [ ] 2.3.1 Emit taskflow.started
- [ ] 2.3.2 Emit taskflow.progress
- [ ] 2.3.3 Emit taskflow.completed
- [ ] 2.3.4 Emit taskflow.failed

---

## Phase 3: 钉钉 Adapter

### 3.1 DingTalk Adapter

**Owner:** internal/layers/communication/adapters

- [ ] 3.1.1 Create DingTalkAdapter struct
  - Location: `internal/layers/communication/adapters/dingtalk.go`
  - Based on Feishu adapter architecture

- [ ] 3.1.2 Implement Connect()
  - Establish WebSocket connection
  - Register event handlers

- [ ] 3.1.3 Implement SendMessage()
  - Send text messages

- [ ] 3.1.4 Implement SendCard()
  - Send interactive cards

- [ ] 3.1.5 Implement onMessage()
  - Handle incoming messages

### 3.2 DingTalk Event Types

- [ ] 3.2.1 Handle text message
- [ ] 3.2.2 Handle card action callback
- [ ] 3.2.3 Handle markdown message

### 3.3 DingTalk Renderer

- [ ] 3.3.1 Create DingTalkCardRenderer
  - Location: `internal/layers/communication/renderers/dingtalk_card.go`

---

## Phase 4: UI Component Library

### 4.1 Base Components

**Owner:** internal/layers/communication/renderers

- [ ] 4.1.1 Create Component interface
  - Location: `internal/layers/communication/renderers/components.go`

- [ ] 4.1.2 Create MilestoneCard component
  - Shows milestone name, progress, status

- [ ] 4.1.3 Create ProgressBar component
  - Visual progress indicator

- [ ] 4.1.4 Create StatusBadge component
  - Colored status indicator

- [ ] 4.1.5 Create PermissionCard component
  - Permission request with approve/deny buttons

### 4.2 Platform Renderers

- [ ] 4.2.1 Implement CLIRenderer (enhance existing)
  - Add milestone rendering
  - Add progress bar rendering

- [ ] 4.2.2 Implement FeishuCardRenderer (enhance existing)
  - Add milestone card
  - Add progress update

- [ ] 4.2.3 Implement DingTalkCardRenderer (new)

### 4.3 Component Tests

- [ ] 4.3.1 Test MilestoneCard rendering
- [ ] 4.3.2 Test ProgressBar rendering
- [ ] 4.3.3 Test StatusBadge rendering

---

## Phase 5: Multi-Instance Support

### 5.1 Instance Manager

**Owner:** internal/layers/communication/instance

- [ ] 5.1.1 Create InstanceConfig
  - Location: `internal/shared/config/instance.go`
  - Fields: InstanceID, ClusterName, RegistryURL

- [ ] 5.1.2 Create IInstanceRegistry interface
  - Location: `internal/layers/communication/instance/registry.go`
  - Methods: Register, Unregister, GetInstances, HealthCheck

- [ ] 5.1.3 Create InstanceRegistry implementation
  - In-memory registry for V3
  - Can be extended to etcd/consul

- [ ] 5.1.4 Implement HealthCheck()
  - Periodic health status update

### 5.2 Prometheus Metrics

- [ ] 5.2.1 Create MetricsCollector
  - Location: `internal/layers/communication/metrics/collector.go`
  - Counters: requests_total, session_created_total
  - Gauges: active_sessions, response_time

- [ ] 5.2.2 Expose /metrics endpoint
  - Prometheus scrape endpoint

- [ ] 5.2.3 Define metrics
  - requests_total{adapter, status}
  - session_count{adapter}
  - response_time_seconds{adapter, quantile}

### 5.3 Load Balancer Integration

- [ ] 5.3.1 Support X-Forwarded-For header
- [ ] 5.3.2 Support sticky sessions

---

## Phase 6: Integration & Tests

### 6.1 Integration Tests

- [ ] 6.1.1 Test Milestone DAG end-to-end
- [ ] 6.1.2 Test TaskFlow execution
- [ ] 6.1.3 Test DingTalk adapter
- [ ] 6.1.4 Test UI component rendering
- [ ] 6.1.5 Test multi-instance metrics

### 6.2 Performance Tests

- [ ] 6.2.1 Test milestone creation performance
- [ ] 6.2.2 Test concurrent session handling

---

## Quality Gates

- [ ] All 50 tasks complete
- [ ] All tests pass
- [ ] Coverage ≥ 80%
- [ ] No critical code analysis issues
- [ ] go vet and staticcheck clean

---

## File Checklist

```
devrix/
├── internal/
│   ├── shared/
│   │   ├── types/
│   │   │   ├── milestone.go   # V3: Milestone 类型
│   │   │   ├── taskflow.go   # V3: TaskFlow 类型
│   │   │   └── events.go     # V3: 新增 milestone 事件
│   │   └── config/
│   │       └── instance.go    # V3: 实例配置
│   └── layers/
│       └── communication/
│           ├── milestone/
│           │   ├── service.go    # V3: Milestone 服务
│           │   ├── taskflow.go   # V3: TaskFlow 服务
│           │   └── events.go     # V3: Milestone 事件
│           ├── adapters/
│           │   ├── dingtalk.go   # V3: 钉钉适配器
│           │   └── dingtalk_test.go
│           ├── renderers/
│           │   ├── components.go # V3: UI 组件接口
│           │   ├── milestone_card.go # V3: Milestone 卡片
│           │   ├── progress_bar.go   # V3: 进度条
│           │   ├── status_badge.go   # V3: 状态徽章
│           │   └── dingtalk_card.go  # V3: 钉钉卡片
│           ├── instance/
│           │   ├── registry.go   # V3: 实例注册
│           │   └── health.go    # V3: 健康检查
│           └── metrics/
│               ├── collector.go # V3: 指标收集
│               └── exporter.go  # V3: Prometheus 导出器
```

---

## Completion Checklist

- [x] P1 能力全部落地（Milestone / TaskFlow / 钉钉 Webhook / UI / Registry）
- [x] 关联单元测试通过
- [x] demand.md + acceptance-report.md 完成
- [ ] P2 延后：InstanceConfig、`/metrics` 端点、LB sticky session
- [x] Ready for production（钉钉 Webhook 模式）
