# Design: D7 Orchestration Domain

## 1. Root Cause Analysis

### 1.1 架构漂移

DSAFT 架构要求每个 D 层有独立的业务能力边界、数据模型、通过 A 层交互。当前系统在 V5-V6 的快速迭代中，D2（Context Engine）承担了超出其域边界的职责：

```
D2 当前承担的非 D2 职责：
  1. Task 持久化（tasks/）           → 应属 D7-S1
  2. BackgroundTask 注册表           → 应属 D7-S1
  3. Delegate 回调注入（loop.Hooks） → 应属 D7-S2 编排
  4. SessionQueue 排空               → 应属 D7-S2
  5. Plan attachments 注入            → 应属 D7-S2
  6. Delegate 工具过滤（filterDelegateTools） → 应属 D7-S2
  7. import multiagent/delegate       → DSAFT 跨域违规
```

### 1.2 分层侵蚀

```
S3 设计时的分层承诺（V6）          → 实现现状

D1 → D2.Process                     → D1 → D2.Process（正确但入口过深）
D2 → D3.LLM.ChatStream             → ✓ 正确
D2 → D4 via interface              → ✗ 直接 import delegate 包
ORCH 作为读模型                     → ✗ ORCH 有能力但被限制
D2 作为上下文引擎                    → ✗ D2 做了编排的事
```

### 1.3 Plan 场景的真空

PEV（D2-S1）退役后，系统丢失了结构化的规划能力。当前规划完全依赖 LLM 在 prompt 内自行完成，系统层无 Plan 数据模型、无阶段追踪、无产物管理。

## 2. Solution Design

### 2.1 D7 顶层设计

```
┌──────────────────────────────────────────────────────────────────┐
│                       D7 Orchestration Domain                     │
│                                                                  │
│  D1 → D7-S2-A01 ProcessMessage  (新主入口)                       │
│         │                                                         │
│         ├── 快速路径: → D2 RunQueryLoop (简单问答)               │
│         │                                                         │
│         └── 编排路径:                                             │
│               D7-S5-A01 ClassifyIntent                            │
│               D7-S5-A02 SynthesizeTaskGraph                       │
│               D7-S1-A01 CreateWorkPlan                            │
│               loop:                                               │
│                 D7-S1-A02-F01 DispatchTask                        │
│                   → D2 RunQueryLoop (执行)                       │
│                   → D4 RunAgent (委托)                            │
│                 D7-S1-A02-F03 CollectArtifacts                    │
│               D7-S1-A03 QueryWorkPlan (报告)                     │
│                                                                  │
│  D7-S3 WaveScheduler (DAG 调度, 多 agent)                        │
│  D7-S4 ExecutionFlow (事件聚合, 进度广播)                        │
└──────────────────────────────────────────────────────────────────┘
```

### 2.2 域间交互契约

D7 通过以下契约与各域交互（不 import 实现包）：

```go
// D7 → D2: 调用 LLM↔Tool 交互原语
type D2Executor interface {
    RunQueryLoop(ctx context.Context, req QueryRequest) (<-chan EngineEvent, error)
}

// D7 → D4: 调用 agent 创建和运行
type D4Executor interface {
    CreateAgent(ctx context.Context, spec AgentSpec) (AgentHandle, error)
    RunAgent(ctx context.Context, handle AgentHandle) (<-chan AgentEvent, error)
}

// D7 → D1: 发布事件到通信层
type D1EventSink interface {
    Publish(ctx context.Context, ev *EngineEvent)
}

// D7 → D6: 请求编排决策验证
type D6Validator interface {
    ValidateOrchestration(ctx context.Context, decision OrchestrationDecision) ValidationResult
}
```

### 2.3 Task 数据模型（单一权威来源）

```go
type Task struct {
    ID           string        `json:"id"`
    SessionID    string        `json:"session_id"`
    Type         TaskType      `json:"type"`        // explore | plan | execute | background
    Status       TaskStatus    `json:"status"`       // created → assigned → running → completed | failed | cancelled
    Goal         string        `json:"goal"`
    AgentID      string        `json:"agent_id,omitempty"`
    Dependencies []string      `json:"dependencies,omitempty"` // TaskID 列表 (DAG)
    Input        *TaskSpec     `json:"input"`
    Output       *TaskResult   `json:"output,omitempty"`
    Artifacts    []ArtifactRef `json:"artifacts,omitempty"`
    CreatedAt    time.Time     `json:"created_at"`
    UpdatedAt    time.Time     `json:"updated_at"`
}

type Plan struct {
    ID        string    `json:"id"`
    SessionID string    `json:"session_id"`
    Tasks     []Task    `json:"tasks"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### 2.4 快速路径 vs 编排路径

```
快速路径（FastPath）：
  语义：简单问答、无需多步骤
  路径：D7-S2-A01 → D2 RunQueryLoop → 返回
  开销：零分配 proxy，≤2ms

编排路径（OrchestratePath）：
  语义：多步骤、需要规划或委托
  路径：D7-S2-A01 → D7-S5 分类 → D7-S1 创建 Plan → 循环 dispatch → 返回
```

### 2.5 决策分层

```
D7-S5 的决策采用"规则优先、LLM 兜底、合并裁决"三层：

Layer 1: 规则引擎
  - 正则、关键字匹配
  - 历史模式匹配（同一 session 之前的决策）
  - 成本：纳秒级

Layer 2: LLM 分类
  - 仅当规则置信度 < threshold 时触发
  - 成本和延迟高于规则
  - 可异步执行

Layer 3: 合并
  - 规则 + LLM 的置信度加权
  - 高置信度 → 快速路径
  - 低置信度 → 编排路径
```

## 3. Key Interfaces / Types

核心接口（位置：`internal/layers/orchestration/` 迁移到 `internal/layers/d7/`）：

```go
package d7

// SessionOrchestrator — D7-S2 核心入口
type SessionOrchestrator interface {
    ProcessMessage(ctx context.Context, session *types.Session, message string) <-chan *contracts.EngineEvent
}

// WorkModel — D7-S1 工作模型
type WorkModel interface {
    CreatePlan(ctx context.Context, sessionID string, tasks []TaskSpec) (*Plan, error)
    CreateTask(ctx context.Context, spec TaskSpec) (*Task, error)
    UpdateTaskStatus(ctx context.Context, taskID string, status TaskStatus) error
    GetPlan(ctx context.Context, sessionID string) (*Plan, error)
}

// IntentClassifier — D7-S5 意图分类
type IntentClassifier interface {
    Classify(ctx context.Context, message string, opts ClassifyOptions) (*IntentClassification, error)
}

// TaskDecomposer — D7-S5 任务拆解
type TaskDecomposer interface {
    Decompose(ctx context.Context, goal string, context map[string]string) ([]TaskSpec, error)
}
```

D2 瘦身后的接口（`internal/shared/contracts/`）：

```go
// PureQueryExecutor — D2 瘦身后的契约
type PureQueryExecutor interface {
    RunQueryLoop(ctx context.Context, req QueryRequest) (<-chan EngineEvent, error)
}

type QueryRequest struct {
    SystemPrompt string
    Messages     []types.Message
    Tools        []ToolSchema
    MaxTurns     int
}
```

## 4. Data Flow

### 4.1 ProcessMessage 快速路径

```
D1 → D7.ProcessMessage
  → D7.A02 EvaluateIntent (规则匹配, 置信度 95%)
  → D7.A01 RouteByIntent → FastPath
  → D2.RunQueryLoop(ctx, req)
  → D7 emit events → D1
```

### 4.2 ProcessMessage 编排路径

```
D1 → D7.ProcessMessage
  → D7-S5.A01 ClassifyIntent (置信度 60%, 需编排)
  → D7-S5.A02 SynthesizeTaskGraph
     → [Explore("调研模块"), Plan("制定方案"), Execute("执行重构")]
     → DAG: Explore → Plan → Execute
  → D7-S1.A01 CreateWorkPlan
  → D7-S1.A02 ManageTask (dispatch Explore)
     → D2.RunQueryLoop(readonly_tools)
     → CollectArtifacts(explore_result)
  → D7-S1.A02 ManageTask (dispatch Plan)
     → D2.RunQueryLoop(plan_mode)
     → CollectArtifacts(plan)
  → D7-S1.A02 ManageTask (dispatch Execute)
     → D4.RunAgent(worker_spec)  // 需要多 agent 并行
     → CollectArtifacts(changes)
  → D7-S1.A03 QueryWorkPlan → snapshot
  → D1 汇总报告
```

### 4.3 D2 回调消除

```
当前（Phase 0）:
  D2 Loop内部:
     1. EnsureParallelAsyncBatch → D4 delegate 启动
     2. LLM Call
     3. Execute Tools
     4. AfterToolRound → seal + wait delegate
     5. SessionQueue.Drain

重构后（Phase 6）:
  D7 SessionOrchestrator:
     Phase 1: D4.DelegateTask(spec)         ← D7 显式编排 D4
     Phase 2: D2.RunQueryLoop(messages)      ← D2 不知道 delegate
       → 返回
     Phase 3: D4.JoinResult(taskID)          ← D7 显式等待
     Phase 4: D2.RunQueryLoop(continue)      ← D2 不知道 join
```

## 5. File Manifest

### 新增文件

| 文件 | 归属 | 说明 |
|------|------|------|
| `internal/layers/d7/orchestrator.go` | D7-S2 | SessionOrchestrator 实现 |
| `internal/layers/d7/workmodel.go` | D7-S1 | Task/Plan 数据模型 |
| `internal/layers/d7/workmodel_store.go` | D7-S1 | Task 持久化（从 D2 tasks/ 迁入） |
| `internal/layers/d7/classifier.go` | D7-S5 | 规则+LLM 意图分类器 |
| `internal/layers/d7/decomposer.go` | D7-S5 | 目标拆解器 |
| `internal/layers/d7/executor.go` | D7-S5 | Task 分派器 |
| `internal/layers/d7/types.go` | D7 | 域公共类型 |
| `internal/layers/d7/fastpath.go` | D7-S2 | 快速路径 proxy |
| `openspec/specs/orchestration/d7-domain.md` | 规范 | D7 域规范 |

### 迁移文件

| 文件 | 从 | 到 |
|------|----|----|
| `orchestration/workplan/service.go` | ORCH-S1 | D7-S1 WorkModel |
| `orchestration/flow/hub.go` | ORCH-S2 | D7-S4 ExecutionFlow |
| `orchestration/flow/hub.go` (部分) | ORCH-S2 | D7-S4-A01 PublishFlowEvent |
| `orchestration/wave/scheduler.go` | ORCH-S3 | D7-S3 WaveScheduler |
| `orchestration/wave/pool.go` | ORCH-S3 | D7-S3 |
| `orchestration/wave/taskgraph.go` | ORCH-S3 | D7-S3 |
| `orchestration/wave/context.go` | ORCH-S3 | D7-S3 |
| `orchestration/wave/conflict.go` | ORCH-S3 | D7-S3 |
| `orchestration/wave/artifact.go` | ORCH-S3 | D7-S3 |
| `contextengine/tasks/` | D2-S10 | D7-S1 WorkModel |
| `contextengine/query/background.go` | D2-S10 | D7-S1 (BackgroundTask) |

### 修改文件

| 文件 | 变更 |
|------|------|
| `communication/gateway/gateway.go` | D1 `RouteInbound` 调用目标从 D2.Process 改为 D7.ProcessMessage |
| `contextengine/engine.go` | 移除 process 中编排逻辑（attachments、queue drain、pre-loop 决策） |
| `contextengine/query/loop.go` | 瘦身：移除 EnsureParallelAsyncBatch、WaitPendingAsyncBatch、SessionQueue、Attachments、UserContext、AfterToolRound |
| `contextengine/query/types.go` | 移除编排相关字段 |
| `bootstrap/delegate.go` | 不再向 Loop 注入回调；改为 D7 直接调用 D4 |
| `openspec/a-registry.md` | 添加 D7 A 层注册 |
| `openspec/f-registry.md` | 添加 D7 F 层注册 |
| `openspec/specs/architecture/layering.md` | 添加 D7 域定义 |
| `openspec/specs/project/dsaft-methodology.md` | 更新域映射表 |

### 删除文件

| 文件 | 理由 |
|------|------|
| `internal/layers/orchestration/` (目录) | 整体迁入 D7；路径 `internal/layers/d7/` |

## 6. Regression Risk Assessment

| 风险区域 | 影响 | 保护措施 |
|----------|------|----------|
| D1→D7 入口变更 | 所有用户请求入口 | Phase 4 feature flag: 双路由并存，灰度切换 |
| D2.Process 逻辑精简 | 现有 D2 测试失效 | Phase 6 前保持 D2.Process 向后兼容 wrapper |
| Task 数据迁移 | 存量 task 数据 | 向后兼容的序列化 + 读时迁移 |
| ORCH 包路径变更 | 所有 import | 迁移期内保留 `internal/layers/orchestration/` 作为 re-export 桥接 |
| delegate 回调重构 | D4 worker 生命周期 | Phase 2 adapter wrapper: `D7DelegateAdapter` 模拟当前 Hook 行为 |

## 7. Rollback Plan

| 场景 | 回滚操作 |
|------|----------|
| D1 入口变更有问题 | Feature flag 切回 `D1→D2.Process` 路径 |
| D7-S5 分类错误 | 降级为 `always_fast_path` 模式，跳过编排 |
| Task 数据模型不兼容 | 保留 D2 tasks/ 作为读兼容层 |
| 性能退化超过 5% | 回退 Phase 4（入口变更）+ Phase 6（D2 瘦身），保留 D7 定义和注册表 |
