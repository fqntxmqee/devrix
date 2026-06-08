# Design: devrix-multi-agent

**Change ID:** devrix-multi-agent
**Canonical design:** `docs/multi-agent-design.md`（可读架构文档，15 章节）
**Delta spec:** `openspec/specs/multi_agent_layer_delta.md`
**Grill Review:** 2026-06-08（6 项决策已合入）

---

## Grill Review 决策记录

| # | 决策 | 影响 |
|---|------|------|
| 1 | **异步权限**：Agent 作为 IPermissionGate 注入 PEVEngine，channel 等待 Gateway 注入用户响应 | Agent 新增 `ResolvePermission()` + `agentPermissionGate` 内部类型 |
| 2 | **contracts.IEngine**：IContextEngine 接口提取到 `shared/contracts/engine.go` | 新增跨层契约文件；Agent 只依赖 contracts |
| 3 | **消息隔离 Fork**：子 Agent 独立消息缓冲区，Join 时合并，不共享 SessionContext 写入 | 消除并发写入风险 |
| 4 | **双层限额**：AgentConfig.MaxChildren(3) + MultiAgentConfig.MaxTotalAgents(5) | 新增配置项 |
| 5 | **AgentPermissionGate**：Agent 实现 IPermissionGate，内部用 channel 阻塞等待 | PEVEngine 零修改，只换注入的实现 |
| 6 | **4 PR 分批**：PR1(contracts) → PR2(perm gate) → PR3(Agent 核心) → PR4(bootstrap) | 16 任务重分组 |

---

## Implementation Notes

| Topic | Decision |
|-------|----------|
| 跨层引擎接口 | `shared/contracts/engine.go` — `IEngine` + `EngineEvent`（含 `ToolRisk` 字段） |
| Package root | `internal/layers/multiagent/` |
| Errors | `internal/shared/errors/multiagent.go` (AGT_* codes, 9 个) |
| Config | `internal/shared/config/multiagent.go` + `multi_agent:` YAML block（含 MaxTotalAgents） |
| Agent 不持有 LLM | 委托 `contracts.IEngine.Process()` 驱动 PEVEngine |
| Tool Registry | 复用 `contextengine/registry`，不在 L4 新建 |
| Permission | Agent 实现 `contextengine.IPermissionGate`，注入 PEVEngine |
| Observer | 适配器桥接到 `contextengine.IObserver`，不在 L4 新建 |
| Fork 并发 | 子 Agent 独立消息缓冲区，Join 时追加到父 Agent SessionContext |

---

## PR 分组

```
PR1: contracts.IEngine 提取（零行为变更）
  shared/contracts/engine.go  ← 从 gateway/gateway.go 提取 IContextEngine → IEngine
  gateway/gateway.go          ← 改为嵌入 contracts.IEngine
  contextengine/contracts.go  ← ILLMGateway 等保持不变

PR2: AgentPermissionGate（权限注入适配）
  multiagent/agent/perm_gate.go  ← agentPermissionGate 实现 IPermissionGate
  multiagent/agent/agent.go      ← 注入 permGate 到 PEVEngine

PR3: Agent 核心实现
  multiagent/contracts.go        ← 所有类型、接口
  multiagent/agent/state.go      ← 状态机
  multiagent/agent/lifecycle.go  ← Run 主循环
  multiagent/agent/forkjoin.go   ← Fork/Join（消息隔离）
  multiagent/factory/factory.go  ← Factory + 双层限额校验
  multiagent/collaboration/*     ← Mode + Prompt
  multiagent/observer/*          ← ObserverAdapter
  所有 *_test.go

PR4: Bootstrap 集成 + 测试收尾
  bootstrap/multi_agent.go       ← WireMultiAgent
  gateway 注入 IAgentFactory     ← CommunicationGateway 可选集成
  tests/integration/*            ← Permission/Concurrency 集成测试
  tests/e2e/*                    ← E2E Fork 场景
```

---

## Package Layout

```
internal/
├── shared/
│   └── contracts/
│       └── engine.go            # PR1: IEngine + EngineEvent（跨层契约）
├── layers/
│   └── multiagent/
│       ├── contracts.go         # PR3: AgentState, AgentConfig, AgentResult, Agent, IAgentFactory, AgentDeps
│       ├── agent/
│       │   ├── agent.go         # PR2+PR3: agentImpl + NewAgent + agentPermissionGate 注入
│       │   ├── perm_gate.go     # PR2: agentPermissionGate 实现 IPermissionGate
│       │   ├── state.go         # PR3: AgentState 方法 + transition()
│       │   ├── lifecycle.go     # PR3: Run 主循环 + handleEngineEvent + Terminate + Wait
│       │   ├── forkjoin.go      # PR3: Fork（消息隔离）+ Join（合并缓冲区）
│       │   └── agent_test.go
│       ├── factory/
│       │   ├── factory.go       # PR3: AgentFactory + validateConfig + 双层限额
│       │   └── factory_test.go
│       ├── collaboration/
│       │   ├── mode.go          # PR3: CollaborationMode + Validate
│       │   ├── prompt.go        # PR3: BuildPromptForMode
│       │   └── mode_test.go
│       └── observer/
│           ├── adapter.go       # PR3: ObserverAdapter
│           └── noop.go          # PR3: NoOpObserverAdapter
└── bootstrap/
    └── multi_agent.go           # PR4: WireMultiAgent
```

---

## Core Types（Grill 修正版）

```go
// ============================================
// shared/contracts/engine.go（PR1）
// ============================================

// IEngine 跨层引擎契约（从 gateway.IContextEngine 提取）
type IEngine interface {
    Process(ctx context.Context, session *types.Session, message string) <-chan *EngineEvent
}

// EngineEvent 引擎事件（扩展 ToolRisk 字段）
type EngineEvent struct {
    Type      string            // thinking | text | tool_call | tool_result | permission | complete | error
    Content   string
    ToolName  string
    ToolInput string
    ToolRisk  types.RiskLevel   // PR1 新增：Agent 需要知道风险等级
    SessionID string
    Metadata  map[string]string
}

// ============================================
// multiagent/contracts.go（PR3）
// ============================================

type AgentState int
const (
    AgentStateCreated           AgentState = iota
    AgentStateRunning
    AgentStateIterating
    AgentStateWaitingPermission
    AgentStateTerminated
)

type CollaborationMode string
const (
    ModeChainOfThought      CollaborationMode = "chain-of-thought"
    ModeIterativeRefinement CollaborationMode = "iterative-refinement"
    ModeDefault             CollaborationMode = "default"
)

type AgentConfig struct {
    SessionID    string
    WorkDir      string
    Mode         CollaborationMode
    MaxIter      int
    MaxChildren  int            // 单父 Agent 子 Agent 上限，默认 3
    Timeout      time.Duration
    PermissionTimeout time.Duration  // 权限等待超时，默认 60s
    ParentID     string
}

type AgentResult struct {
    Messages []types.Message
    ExitCode int
    Error    error
    Duration time.Duration
}

// Agent 主接口（Grill 修正：新增 ResolvePermission + GetMessages）
type Agent interface {
    ID() string
    State() AgentState
    Config() AgentConfig

    Run(ctx context.Context) (*AgentResult, error)
    Fork(ctx context.Context, cfg AgentConfig) (Agent, error)
    Join(ctx context.Context, child Agent) error
    Terminate(ctx context.Context) error
    Wait(ctx context.Context) (*AgentResult, error)

    // ResolvePermission 由 Gateway 调用，注入用户权限响应（PR2 引入）
    ResolvePermission(toolName string, granted bool)

    // GetMessages 返回 Agent 独立的消息缓冲区（PR3：消息隔离）
    GetMessages() []types.Message
}

type IAgentFactory interface {
    Create(ctx context.Context, cfg AgentConfig, session *types.Session) (Agent, error)
}

// AgentDeps 依赖注入（Grill 修正：Engine 类型改为 contracts.IEngine，移除 PermissionMgr）
type AgentDeps struct {
    Engine   contracts.IEngine          // PR1: 跨层契约
    Observer contextengine.IObserver
}
```

---

## State Machine（不变）

```
CREATED ──Run()──▶ RUNNING ──Process()──▶ ITERATING
                      ▲                      │
                      │              ┌───────┼───────┐
                      │              │               │
                      │       permission       no tools
                      │       _required           │
                      │              │               │
                      │              ▼               ▼
                      │     WAITING_PERMISSION   TERMINATED
                      │         │    │     │
                      │    GRANTED DENIED TIMEOUT
                      │         │    │     │
                      └─────────┘    └─────┘
```

> 注：WAITING_PERMISSION 触发条件由 "CRITICAL tool_call" 改为 "permission_required 事件"，因为 Agent 作为 IPermissionGate 在 PEVEngine 内部判断风险等级。

---

## Key Flows（Grill 修正版）

### Agent.Run() 主循环

```go
func (a *agentImpl) Run(ctx context.Context) (*AgentResult, error) {
    startTime := time.Now()
    a.transition(AgentStateCreated, AgentStateRunning)
    a.emitEvent("agent.started")

    eventCh := a.engine.Process(ctx, a.session, a.userContent)

    for event := range eventCh {
        select {
        case <-ctx.Done():
            return a.terminateWithError(NewAgentContextCancelledError(a.id))
        default:
        }

        a.transition(a.state, AgentStateIterating)

        switch event.Type {
        case "permission":
            // PEVEngine 通过 AgentPermissionGate 触发的权限事件
            // AgentPermissionGate.Request() 内部阻塞等待 ResolvePermission()
            // Gateway 需要在此期间调用 Agent.ResolvePermission()
            // 如果超时/拒绝，PEVEngine 会收到 false，产生 error event
            continue

        case "complete":
            result := a.buildResult(event, time.Since(startTime))
            a.transition(a.state, AgentStateTerminated)
            a.emitEvent("agent.terminated")
            return result, nil

        case "error":
            return a.terminateWithError(event.Error)
        }
    }
    return a.buildResult(nil, time.Since(startTime)), nil
}
```

> 关键变化：Agent.Run() 不再自己处理 tool_call → 权限判断。PEVEngine 内部通过 AgentPermissionGate 完成，Agent 只响应 `permission` 事件做状态转换。

### AgentPermissionGate（PR2，注入 PEVEngine）

```go
// agent/perm_gate.go — Agent 实现 contextengine.IPermissionGate

type agentPermissionGate struct {
    agent        *agentImpl
    pendingTools map[string]chan bool  // toolName → result channel
    mu           sync.Mutex
}

func (g *agentPermissionGate) Request(ctx context.Context, sessionID, toolName, input string, risk types.RiskLevel) bool {
    if risk != types.RiskLevelCritical {
        return true  // 非 CRITICAL 自动批准
    }

    ch := make(chan bool, 1)
    g.mu.Lock()
    g.pendingTools[toolName] = ch
    g.mu.Unlock()

    // 异步通知：Agent 进入 WAITING_PERMISSION 状态
    g.agent.transition(g.agent.state, AgentStateWaitingPermission)
    g.agent.emitEvent("permission_required")  // → Gateway → Adapter → 用户 UI

    // 同步等待：Gateway 调用 ResolvePermission 后 channel 被写入
    select {
    case granted := <-ch:
        if granted {
            g.agent.transition(AgentStateWaitingPermission, AgentStateIterating)
        } else {
            g.agent.transition(AgentStateWaitingPermission, AgentStateTerminated)
        }
        return granted
    case <-ctx.Done():
        g.agent.transition(AgentStateWaitingPermission, AgentStateTerminated)
        return false
    case <-time.After(g.agent.config.PermissionTimeout):
        g.agent.transition(AgentStateWaitingPermission, AgentStateTerminated)
        return false
    }
}

// ResolvePermission 由 Gateway 调用（通过 Agent 接口暴露）
func (a *agentImpl) ResolvePermission(toolName string, granted bool) {
    a.permGate.mu.Lock()
    defer a.permGate.mu.Unlock()
    if ch, ok := a.permGate.pendingTools[toolName]; ok {
        ch <- granted
        delete(a.permGate.pendingTools, toolName)
    }
}
```

**Gateway 侧调用链：**
```
1. Gateway 收到 EngineEvent{Type: "permission"}  → 通过 Adapter 展示确认 UI
2. 用户响应
3. Gateway 调用 Agent.ResolvePermission(toolName, granted)
4. agentPermissionGate.Request() 的 select 收到 channel 值 → 返回给 PEVEngine
```

### Fork/Join（消息隔离）

```go
// agent/forkjoin.go

func (a *agentImpl) Fork(ctx context.Context, cfg AgentConfig) (Agent, error) {
    a.mu.Lock()
    defer a.mu.Unlock()

    // 双层限额检查
    if len(a.childAgents) >= a.config.MaxChildren {
        return nil, NewAgentMaxChildrenError(...)
    }
    if a.countAllDescendants() >= a.globalCfg.MaxTotalAgents {
        return nil, NewAgentMaxTotalError(...)
    }

    cfg.ParentID = a.id
    // 子 Agent 不能继续 Fork（V1 限制）
    cfg.MaxChildren = 0

    // 子 Agent 获得 Session 元信息，但使用独立消息缓冲区
    child, _ := a.factory.Create(ctx, cfg, a.session)
    a.childAgents[child.ID()] = child
    a.emitEvent("agent.forked")
    return child, nil
}

func (a *agentImpl) Join(ctx context.Context, child Agent) error {
    result, err := child.Wait(ctx)
    if err != nil {
        return err
    }

    a.mu.Lock()
    defer a.mu.Unlock()

    // 合并子 Agent 的独立消息缓冲区到 SessionContext
    childMessages := child.GetMessages()
    a.messageBuffer = append(a.messageBuffer, childMessages...)
    delete(a.childAgents, child.ID())
    a.emitEvent("agent.joined")
    return nil
}

// GetMessages 返回 Agent 独立的消息缓冲区
func (a *agentImpl) GetMessages() []types.Message {
    a.mu.RLock()
    defer a.mu.RUnlock()
    return append([]types.Message{}, a.messageBuffer...)
}
```

---

## Config（Grill 修正：新增 MaxTotalAgents）

```yaml
# devrix.yaml
multi_agent:
  max_children: 3         # 单父 Agent 子 Agent 上限
  max_total_agents: 5     # 单 Session Agent 总数上限（兜底保护）
  default_timeout: "5m"
  default_max_iter: 50
  permission_timeout: "60s"
  default_mode: "default"
```

```go
type MultiAgentConfig struct {
    MaxChildren       int
    MaxTotalAgents    int           // 新增
    DefaultTimeout    time.Duration
    DefaultMaxIter    int
    PermissionTimeout time.Duration
    DefaultMode       string
}

func DefaultMultiAgentConfig() *MultiAgentConfig {
    return &MultiAgentConfig{
        MaxChildren:       3,
        MaxTotalAgents:    5,        // 新增
        DefaultTimeout:    5 * time.Minute,
        DefaultMaxIter:    50,
        PermissionTimeout: 60 * time.Second,
        DefaultMode:       "default",
    }
}
```

---

## Error Codes（Grill 修正：新增 AGT_FACTORY_5010）

| Code | Constant | Meaning | Recoverable |
|------|----------|---------|-------------|
| AGT_LIFECYCLE_5001 | ErrAgentInvalidTransition | 非法状态转换 | No |
| AGT_FACTORY_5002 | ErrAgentMaxChildren | 子 Agent 数超限 | No |
| AGT_LIFECYCLE_5003 | ErrAgentAlreadyTerminated | 已终止再操作 | No |
| AGT_FORK_5004 | ErrAgentJoinNotCompleted | Join 时未完成 | Yes |
| AGT_LIFECYCLE_5005 | ErrAgentTimeout | 执行超时 | Yes |
| AGT_FACTORY_5006 | ErrAgentInvalidConfig | 配置不合法 | No |
| AGT_PERMISSION_5007 | ErrAgentPermissionTimeout | 权限超时 | Yes |
| AGT_PERMISSION_5008 | ErrAgentPermissionDenied | 权限被拒 | Yes |
| AGT_CONTEXT_5009 | ErrAgentContextCancelled | Context 取消 | Yes |
| AGT_FACTORY_5010 | ErrAgentMaxTotal | Session Agent 总数超限 | No |

---

## Bootstrap（Grill 修正：Engine 类型 + 移除 PermissionMgr）

```go
// internal/bootstrap/multi_agent.go
func WireMultiAgent(
    engine contracts.IEngine,         // PR1: 跨层契约类型
    observer contextengine.IObserver,
    obsBridge *observability.Bridge,
    cfg *config.MultiAgentConfig,
) multiagent.IAgentFactory {
    deps := multiagent.AgentDeps{
        Engine:   engine,
        Observer: observer,
    }
    return multiagent.NewAgentFactory(deps, obsBridge, cfg)
}
```

---

## Test Strategy

| PR | L5 覆盖 | 类型 |
|----|---------|------|
| PR1 | — | 重构验证：L1/L2 编译 + 现有测试全绿 |
| PR2 | L5-4-2-02, L5-4-2-03 | 单元：AgentPermissionGate 批准/拒绝/超时 |
| PR3 | L5-4-1-01, L5-4-2-01, L5-4-3-01~04, L5-4-4-01~02, L5-4-5-01, L5-4-0-01~02 | 单元：全 Agent 功能 |
| PR4 | L5-4-0-03, L5-4-0-04 | 集成+E2E：Permission + Fork 端到端 |

- MockEngine 实现 `contracts.IEngine`，模拟 Process() 返回可控事件流
- MockPermissionManager 不再需要（Agent 自己就是 IPermissionGate）
- `-race` 作为 CI 强制门禁
