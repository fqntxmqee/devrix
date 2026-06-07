# Multi-Agent Layer 架构设计（修订版）

**版本:** V1
**创建:** 2026-06-07
**修订:** 2026-06-07
**状态:** Draft
**Layer:** Layer 4

---

## ① 架构目标

### 业务目标

- **多 Agent 协作**：支持主 Agent Fork 子 Agent 进行并行任务处理
- **工具生态**：复用现有 Tool Registry，扩展内置工具集
- **安全执行**：通过现有 PermissionManager 确保 CRITICAL 工具用户知情授权

### 技术指标

| 指标 | 目标值 |
|------|--------|
| 单次 Agent 迭代延迟 | < 5s（不含 LLM 调用） |
| 并行子 Agent 上限 | 单 Session 最多 3 个 |
| 工具风险等级 | LOW / MEDIUM / HIGH / CRITICAL 四级（复用 `types.RiskLevel`） |
| 权限超时默认 | 60 秒（复用 `gateway.PermissionManager`） |

---

## ② 架构原则

1. **状态机驱动**：Agent 生命周期严格遵循 `CREATED → RUNNING → ITERATING → WAITING_PERMISSION → TERMINATED`
2. **复用优于新建**：优先复用现有组件（ContextEngine、PermissionManager、ToolRegistry）
3. **父子共享指针**：Fork 时共享 `*types.SessionContext` 指针，写时复制需要修改的部分
4. **小包高内聚**：按功能拆分 subpackage，单包 < 500 行
5. **接口即契约**：小接口（1-3 方法），接口定义在消费方

---

## ③ 业务流程

### Agent 生命周期

```
Session Start
    │
    ▼
┌─────────────────┐
│  AgentFactory   │
│    .Create()    │
└────────┬────────┘
         │ 创建 Agent 实例
         ▼
    ┌─────────┐
    │ CREATED │ ← 初始状态，SessionContext 已关联
    └────┬────┘
         │ .Run()
         ▼
    ┌─────────┐
    │ RUNNING │ ← 驱动 ContextEngine.Process()
    └────┬────┘
         │ Process() 返回 EngineEvent
         ▼
┌────────────────────────────────┐
│          ITERATING             │ ← 迭代循环（由 PEVEngine 内部处理）
│  ┌──────────────────────────┐  │
│  │  ToolCalls 存在?          │  │
│  └──────────┬───────────────┘  │
│       YES   │   NO             │
│       ▼     │   ▼             │
│  ┌─────────────────┐   ┌──────┴───────┐
│  │ PermissionCheck │   │  返回结果     │
│  └────────┬────────┘   └──────┬───────┘
│           │                   │
│           ▼                   │
│  ┌─────────────────┐          │
│  │ WAITING_PERMISSION│◄── (由 gateway.PermissionManager 处理)
│  └────────┬────────┘          │
│           │                   │
│           └───────────────────┘
│                    │
│              Loop back to ITERATING
│                    │
│            (no more tool calls)
│                    ▼
│           ┌─────────────┐
│           │ TERMINATED  │ ← 最终状态
│           └─────────────┘
```

> **关键修正**：Agent 不直接调用 LLM，而是通过 `IContextEngine.Process()` 驱动 PEVEngine。PEVEngine 内部的迭代循环（Execute→Verify）对 Agent 透明。

### Fork/Join 流程（V1 简化版）

```
主 Agent 正在执行
    │
    │ .Fork(cfg)
    ▼
┌─────────────────────────┐
│ 1. 共享 *SessionContext │ ← 指针共享，非完整 COW
│ 2. 创建新 Agent B       │
│ 3. B 进入 CREATED      │
└──────────┬──────────────┘
           │
           ▼
    ┌──────────────┐
    │ Agent A 等待  │◄────┐
    │ Agent B 完成  │     │
    └───────┬──────┘     │
            │            │
            ▼            │
    ┌──────────────┐     │
    │  .Join()     │     │
    │ 合并结果到 A   │     │
    └───────┬──────┘     │
            │            │
            └────────────┘
```

> **关键修正**：V1 不实现完整 Copy-on-Write，只需共享 `*types.SessionContext` 指针。子 Agent 需要修改 context 时，在写入前复制相关字段。

### Permission 流程

```
工具执行请求
    │
    │ riskLevel = CRITICAL
    ▼
┌─────────────────────────────────────┐
│      gateway.PermissionManager      │ ← 复用现有组件
│         .Request()                 │
└──────────────┬──────────────────────┘
               │ 发送权限请求给用户（通过 EventHandler）
               ▼
        ┌─────────────────┐
        │ WAITING (60s)  │ ← 同步等待用户响应
        └────────┬────────┘
                 │
          ┌──────┴──────┐
          │ GRANTED     │ DENIED        │ TIMEOUT
          └──────┬──────┘    │         │
                 │           │         │
                 ▼           ▼         ▼
          继续执行      抛异常      抛异常
```

> **关键修正**：不新建 `PermissionPipeline`，直接委托给 `gateway.PermissionManager`。

---

## ④ 领域模型

### 聚合边界

| 聚合根 | 包含 | 职责 |
|--------|------|------|
| `Agent` | ID, State, Config, SessionContext | Agent 生命周期管理 |
| `AgentFactory` | 依赖 ContextEngine、ToolRegistry、PermissionManager | 创建 Agent 实例 |

### 核心结构

```go
// Agent 主实体（关键修正：不直接持有 LLM/ToolRunner）
type Agent struct {
    id          string
    state       AgentState
    config      AgentConfig
    sessionCtx  *types.SessionContext  // 共享指针
    engine      gateway.IContextEngine // 委托 ContextEngine
    observer    contextengine.IObserver // 复用现有接口
    childAgents map[string]Agent       // 子 Agent 列表
    mu          sync.RWMutex
}

// AgentConfig Agent 配置
type AgentConfig struct {
    SessionID   string
    WorkDir     string
    Mode        CollaborationMode  // CoT / IterativeRefinement / Default
    MaxIter     int
    Timeout     time.Duration
}

// CollaborationMode 协作模式（配置选项，非消息处理器）
type CollaborationMode string

const (
    ModeChainOfThought      CollaborationMode = "chain-of-thought"
    ModeIterativeRefinement CollaborationMode = "iterative-refinement"
    ModeDefault             CollaborationMode = "default"
)
```

> **关键修正**：
> - `Agent` 不直接持有 `ILLMGateway`/`IToolRunner`，而是通过 `IContextEngine` 委托
> - 复用 `types.SessionContext` 而非新建 `SharedContext`
> - `CollaborationMode` 是配置类型，不是独立的消息处理器接口

---

## ⑤ 核心链路图

### 完整数据流（修正版）

```
Communication Layer
        │
        │ RouteInbound(session, message)
        ▼
┌───────────────────────────────────────────────────────────┐
│                    Communication Gateway                   │
│                                                           │
│  ┌─────────────────────────────────────────────────────┐  │
│  │           gateway.IContextEngine.Process()          │  │
│  │                    (L2, PEVEngine)                 │  │
│  └─────────────────────────────────────────────────────┘  │
│                            │                              │
│                            │ EngineEvent                  │
└────────────────────────────┼──────────────────────────────┘
                             │
                             ▼
              ┌──────────────────────────────┐
              │      Multi-Agent Layer      │
              │                              │
              │  Agent.Run()                 │
              │  - 驱动 ContextEngine        │
              │  - 管理 Agent 生命周期       │
              │  - 处理 Fork/Join           │
              │  - 应用 Collaboration Mode  │
              │                              │
              │  委托组件（不复建）：          │
              │  - gateway.PermissionManager │
              │  - contextengine.Registry    │
              │  - contextengine.IObserver   │
              │                              │
              └──────────────────────────────┘
```

> **关键修正**：Multi-Agent 层是 **L2 ContextEngine 的调用方和协调者**，不替代 PEVEngine。

---

## ⑥ 接口 / API 设计

### AgentFactory 接口

```go
// IAgentFactory 创建 Agent 实例
type IAgentFactory interface {
    Create(ctx context.Context, cfg AgentConfig, sessionCtx *types.SessionContext) (Agent, error)
}
```

### Agent 接口

```go
// Agent 主接口
type Agent interface {
    ID() string
    State() AgentState

    // Run 执行 Agent 主循环（驱动 ContextEngine）
    Run(ctx context.Context) error

    // Fork 创建子 Agent（V1 简化版：同步等待结果后 Join）
    Fork(ctx context.Context, cfg AgentConfig) (Agent, error)

    // Terminate 强制终止 Agent
    Terminate(ctx context.Context) error

    // Wait 等待 Agent 完成并获取结果
    Wait(ctx context.Context) (*AgentResult, error)
}

// AgentResult Agent 执行结果
type AgentResult struct {
    Messages []types.Message
    ExitCode int
    Error    error
}
```

### CollaborationMode（配置类型，非接口）

```go
// CollaborationMode 作为配置传递给 AgentFactory
// 不再是独立接口，简化实现

// BuildPromptForMode 根据 mode 构建 system prompt
func BuildPromptForMode(mode CollaborationMode, basePrompt string) string {
    switch mode {
    case ModeChainOfThought:
        return basePrompt + "\n\n请逐步推理，每一步都要说明理由。"
    case ModeIterativeRefinement:
        return basePrompt + "\n\n请先给出初始答案，然后自我批判并改进。"
    default:
        return basePrompt
    }
}
```

> **关键修正**：`CollaborationMode` 从独立接口简化为配置类型，通过 `BuildPromptForMode()` 生成不同模式对应的 system prompt。

---

## ⑦ 目录结构（修正版）

```
internal/layers/multiagent/
├── contracts.go              # 类型定义与常量（约 100 行）
│                              # - AgentState
│                              # - AgentConfig
│                              # - CollaborationMode
│                              # - AgentResult
│                              # - IAgentFactory
│                              # - Agent
│
├── agent/
│   ├── state.go             # AgentState 状态机
│   ├── lifecycle.go          # Run / Terminate / Wait 实现
│   ├── forkjoin.go          # Fork / Join 实现（简化版 COW）
│   ├── agent.go             # Agent 主实体（约 150 行）
│   └── agent_test.go
│
├── factory/
│   ├── factory.go            # AgentFactory 实现
│   └── factory_test.go
│
├── collaboration/
│   ├── mode.go              # CollaborationMode 配置类型
│   ├── prompt.go            # BuildPromptForMode() 生成不同模式的 prompt
│   └── mode_test.go
│
└── observer/
    ├── adapter.go           # multiagent → contextengine.IObserver 适配器
    └── noop.go              # NoOp 实现
```

> **关键修正**：
> - 删除 `registry/`（复用 `contextengine/registry`）
> - 删除 `permission/`（复用 `gateway.PermissionManager`）
> - `CollaborationMode` 从接口改为配置类型 + prompt 生成器
> - 新增 `observer/adapter.go` 桥接到现有 `contextengine.IObserver`

### 对比：原设计 vs 修正版

| 原设计 | 修正版 | 原因 |
|--------|--------|------|
| `multiagent/registry/` | 复用 `contextengine/registry` | 避免重复建设 |
| `multiagent/permission/` | 复用 `gateway.PermissionManager` | 避免重复建设 |
| `CollaborationMode` 接口 | 配置类型 + prompt 生成器 | 无需独立消息处理器 |
| `SharedContext` (COW) | 直接共享 `*types.SessionContext` | V1 简化实现 |
| `multiagent/IObserver` | `contextengine.IObserver` 适配器 | 复用现有接口 |
| `Agent` 直接调用 LLM | Agent 通过 `IContextEngine` 委托 | 复用 PEVEngine |

---

## ⑧ 风险等级定义（复用现有）

复用 `types.RiskLevel`：

| 等级 | 值 | 工具 | 说明 |
|------|------|------|------|
| LOW | `"LOW"` | `read_file`, `glob`, `ls`, `grep` | 只读操作 |
| MEDIUM | `"MEDIUM"` | `write_file`, `fetch` | 可能有副作用 |
| HIGH | `"HIGH"` | `edit`, `bash` | 需用户确认 |
| CRITICAL | `"CRITICAL"` | (扩展) | 强制确认 |

> 工具注册在 `contextengine/registry/builtin.go`，V1 扩展工具在 `openspec/changes/devrix-multi-agent/` 中定义。

---

## ⑨ Collaboration Modes（配置实现）

### Chain-of-Thought (V1)

```go
// collaboration/prompt.go
func BuildChainOfThoughtPrompt(base string) string {
    return base + "\n\n请一步一步推理，在回复中包含你的思考过程。"
}
```

### Iterative-Refinement (V1)

```go
func BuildIterativeRefinementPrompt(base string) string {
    return base + "\n\n请先给出你认为的最佳答案，然后进行自我批判（指出不足），最后给出改进版本。"
}
```

### Default (V1)

直接使用基础 system prompt，无额外指令。

---

## ⑩ 与其他层的关系（修正版）

| 层 | 关系 | 接口/组件 |
|----|------|----------|
| L1 Communication | 启动 Agent，展示结果 | `AgentFactory.Create()` |
| L2 Context Engine | **委托**：Agent 通过 `IContextEngine.Process()` 驱动 PEVEngine | `gateway.IContextEngine` |
| L2 Tool Registry | **复用** | `contextengine.IToolRegistry` |
| L2 Observer | **复用 + 适配** | `contextengine.IObserver` → `multiagent.ObserverAdapter` |
| L1 Permission | **委托** | `gateway.PermissionManager` |
| L3 LLM Gateway | **间接**：通过 PEVEngine 调用 | `contextengine.ILLMGateway` |

---

## ⑪ 关键设计决策汇总

| 决策 | 选择 | 理由 |
|------|------|------|
| ToolRegistry | 复用 `contextengine/registry` | 避免重复 |
| Permission | 委托 `gateway.PermissionManager` | 已有完整实现 |
| Agent ↔ PEVEngine | Agent 封装/委托 PEVEngine，非替代 | 复用 PEVEngine 循环 |
| RiskLevel | 复用 `types.RiskLevel` (string) | 避免类型不匹配 |
| SharedContext | 直接共享 `*types.SessionContext` | V1 简化 COW |
| CollaborationMode | 配置类型 + prompt 生成器 | 无需独立消息处理器 |
| Observer | 适配器桥接到 `contextengine.IObserver` | 复用现有接口 |

---

## ⑫ 后续规划

| 阶段 | 功能 | 前置条件 |
|------|------|----------|
| V2 | Supervisor-Worker 模式（任务分解） | Agent 可嵌套 |
| V2 | 完整 COW 实现 | SessionContext 支持 |
| V3 | Peer-Review 模式（多 Agent 审查） | Milestone DAG |
| V3 | Vote-Consensus 模式（多 Agent 投票） | Agent 组管理 |
| V3 | Full Fork/Merge + Milestone DAG | 复杂依赖图 |
