# Design: devrix-multi-agent

**Change ID:** devrix-multi-agent
**Canonical design:** `docs/multi-agent-design.md`（可读架构文档，15 章节）
**Delta spec:** `openspec/specs/multi_agent_layer_delta.md`

---

## Implementation Notes (vs design doc)

| Topic | Decision |
|-------|----------|
| Shared context | `*types.Session` pointer（与 `IContextEngine.Process` 契约一致） |
| Package root | `internal/layers/multiagent/` |
| Errors | `internal/shared/errors/multiagent.go` (AGT_* codes) |
| Config | `internal/shared/config/multiagent.go` + `multi_agent:` YAML block |
| Agent 不持有 LLM | 委托 `IContextEngine.Process()` 驱动 PEVEngine |
| Tool Registry | 复用 `contextengine/registry`，不在 L4 新建 |
| Permission | 委托 `gateway.PermissionManager`，不在 L4 新建 |
| Observer | 适配器桥接到 `contextengine.IObserver`，不在 L4 新建 |

---

## Package Layout

```
internal/layers/multiagent/
├── contracts.go              # AgentState, AgentConfig, AgentResult, Agent 接口, IAgentFactory, AgentDeps
├── agent/
│   ├── agent.go              # agentImpl struct + NewAgent 构造函数
│   ├── state.go              # AgentState 类型方法 + transition() + ValidTransitions
│   ├── lifecycle.go          # Run 主循环 + handleEngineEvent + handleToolCall + Terminate + Wait
│   ├── forkjoin.go           # Fork + Join + collectChildResults
│   └── agent_test.go         # 单元测试
├── factory/
│   ├── factory.go            # AgentFactory 实现 + validateConfig
│   └── factory_test.go
├── collaboration/
│   ├── mode.go               # CollaborationMode 常量 + Validate
│   ├── prompt.go             # BuildPromptForMode + BuildChainOfThoughtPrompt + BuildIterativeRefinementPrompt
│   └── mode_test.go
└── observer/
    ├── adapter.go            # ObserverAdapter + EmitAgentEvent + 事件类型映射
    └── noop.go               # NoOpObserverAdapter

internal/bootstrap/multi_agent.go  # WireMultiAgent 引导函数
```

---

## Core Types

```go
// AgentState 生命周期状态（int enum）
type AgentState int
const (
    AgentStateCreated           AgentState = iota // 0
    AgentStateRunning                              // 1
    AgentStateIterating                            // 2
    AgentStateWaitingPermission                    // 3
    AgentStateTerminated                           // 4
)
func (s AgentState) String() string
func (s AgentState) IsActive() bool     // Running/Iterating/WaitingPermission
func (s AgentState) IsTerminal() bool    // Terminated

// CollaborationMode 协作模式（string enum）
type CollaborationMode string
const (
    ModeChainOfThought      CollaborationMode = "chain-of-thought"
    ModeIterativeRefinement CollaborationMode = "iterative-refinement"
    ModeDefault             CollaborationMode = "default"
)

// AgentConfig 创建配置（值对象）
type AgentConfig struct {
    SessionID    string
    WorkDir      string
    Mode         CollaborationMode
    MaxIter      int
    MaxChildren  int
    Timeout      time.Duration
    ParentID     string
}

// AgentResult 执行结果（值对象）
type AgentResult struct {
    Messages []types.Message
    ExitCode int
    Error    error
    Duration time.Duration
}

// Agent 主接口
type Agent interface {
    ID() string
    State() AgentState
    Config() AgentConfig
    Run(ctx context.Context) (*AgentResult, error)
    Fork(ctx context.Context, cfg AgentConfig) (Agent, error)
    Join(ctx context.Context, child Agent) error
    Terminate(ctx context.Context) error
    Wait(ctx context.Context) (*AgentResult, error)
}

// IAgentFactory 工厂接口
type IAgentFactory interface {
    Create(ctx context.Context, cfg AgentConfig, session *types.Session) (Agent, error)
}

// AgentDeps 依赖注入
type AgentDeps struct {
    Engine        gateway.IContextEngine
    PermissionMgr gateway.PermissionManager
    Observer      contextengine.IObserver
}
```

---

## State Machine

```
CREATED ──Run()──▶ RUNNING ──Process()──▶ ITERATING
                      ▲                      │
                      │              ┌───────┼───────┐
                      │              │               │
                      │         tool_call       no tools
                      │         (CRITICAL)          │
                      │              │               │
                      │              ▼               ▼
                      │     WAITING_PERMISSION   TERMINATED
                      │         │    │     │
                      │    GRANTED DENIED TIMEOUT
                      │         │    │     │
                      └─────────┘    └─────┘
```

**合法转换：**

| From | To | Trigger |
|------|----|---------|
| CREATED | RUNNING | Run() |
| RUNNING | ITERATING | Process() 返回 |
| RUNNING | TERMINATED | Terminate() / Cancel |
| ITERATING | ITERATING | 继续迭代 |
| ITERATING | WAITING_PERMISSION | CRITICAL tool_call |
| ITERATING | TERMINATED | 完成 / Terminate |
| WAITING_PERMISSION | ITERATING | 用户批准 |
| WAITING_PERMISSION | TERMINATED | 用户拒绝 / 超时 |

**非法转换返回:** `AGT_LIFECYCLE_5001`（非终端状态）或 `AGT_LIFECYCLE_5003`（TERMINATED 后再操作）

---

## Key Flows

### Agent.Run() 主循环（伪代码）

```go
func (a *agentImpl) Run(ctx context.Context) (*AgentResult, error) {
    startTime := time.Now()
    a.transition(AgentStateCreated, AgentStateRunning)
    a.emitEvent("agent.started")

    // 委托 ContextEngine
    eventCh, err := a.engine.Process(ctx, a.session, a.userContent)
    if err != nil {
        return a.terminateWithError(err)
    }

    for event := range eventCh {
        select {
        case <-ctx.Done():
            return a.terminateWithError(NewAgentContextCancelledError(a.id))
        default:
        }

        a.transition(a.state, AgentStateIterating)
        a.emitEvent("agent.iterating")

        switch event.Type {
        case "tool_call":
            a.handleToolCall(ctx, event)
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

### Fork/Join

```go
// Fork 创建子 Agent，共享 SessionContext 指针
func (a *agentImpl) Fork(ctx context.Context, cfg AgentConfig) (Agent, error) {
    a.mu.Lock()
    defer a.mu.Unlock()
    if len(a.childAgents) >= a.config.MaxChildren {
        return nil, NewAgentMaxChildrenError(...)
    }
    child, _ := a.factory.Create(ctx, cfg, a.sessionCtx)
    a.childAgents[child.ID()] = child
    a.emitEvent("agent.forked")
    return child, nil
}

// Join 合并子 Agent 结果到 SessionContext
func (a *agentImpl) Join(ctx context.Context, child Agent) error {
    result, err := child.Wait(ctx)
    if err != nil { return err }
    a.mu.Lock()
    defer a.mu.Unlock()
    // 追加消息到 SessionContext（写锁保护）
    a.sessionCtx.Messages = append(a.sessionCtx.Messages, result.Messages...)
    delete(a.childAgents, child.ID())
    a.emitEvent("agent.joined")
    return nil
}
```

### 权限处理

```go
func (a *agentImpl) handleToolCall(ctx context.Context, event *EngineEvent) {
    riskLevel := a.resolveRiskLevel(event.ToolName)
    if riskLevel == types.RiskLevelCritical {
        a.transition(a.state, AgentStateWaitingPermission)
        granted := a.permissionMgr.Request(ctx, a.session.ID, event.ToolName, event.ToolInput, riskLevel)
        if !granted {
            a.transition(a.state, AgentStateTerminated)
            return
        }
        a.transition(AgentStateWaitingPermission, AgentStateIterating)
    }
    // LOW/MEDIUM/HIGH: 自动授权
}
```

---

## Error Codes

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

---

## Config

```yaml
# devrix.yaml
multi_agent:
  max_children: 3
  default_timeout: "5m"
  default_max_iter: 50
  permission_timeout: "60s"
  default_mode: "default"
```

```go
// internal/shared/config/multiagent.go
type MultiAgentFileConfig struct { ... }  // YAML shape
type MultiAgentConfig struct { ... }       // runtime
func DefaultMultiAgentConfig() *MultiAgentConfig { ... }
func BuildMultiAgentConfig(file *MultiAgentFileConfig) *MultiAgentConfig { ... }
```

---

## Bootstrap

```go
// internal/bootstrap/multi_agent.go
func WireMultiAgent(
    engine gateway.IContextEngine,
    permMgr gateway.PermissionManager,
    observer contextengine.IObserver,
    obsBridge *observability.Bridge,
    cfg *config.MultiAgentConfig,
) multiagent.IAgentFactory {
    deps := multiagent.AgentDeps{Engine: engine, PermissionMgr: permMgr, Observer: observer}
    return multiagent.NewAgentFactory(deps, obsBridge, cfg)
}
```

---

## Test Strategy

| Priority | Count | Type |
|----------|-------|------|
| P0 | 6 | 单元：Factory/StateMachine/ForkJoin/Concurrency |
| P1 | 7 | 单元+集成：Permission/Collaboration/Observer/SessionContext |
| P2 | 2 | 集成+E2E：Observer 桥接/E2E Fork 场景 |

- MockContextEngine 模拟 Process() 返回可控事件流
- MockPermissionManager 模拟批准/拒绝/超时
- 所有测试文件用 `package multiagent_test`（黑盒测试）
- `-race` 作为 CI 强制门禁

---

## Milestone Scope (this change)

| Milestone | Tasks | Scope | Estimate |
|-----------|-------|-------|----------|
| M1: Foundation | T1-T3 | contracts + collaboration + observer 桩 | 6h |
| M2: Lifecycle | T4-T7 | 状态机 + agent + lifecycle + 测试 | 10h |
| M3: Fork/Join | T8-T10 | forkjoin + factory + 测试 | 8h |
| M4: Collab+Perm | T11-T13 | mode 测试 + permission 测试 + factory 测试 | 6h |
| M5: Observer | T14-T15 | observer 实现 + span 集成 | 4h |
| M6: Bootstrap | T16 | wire + 集成测试 + E2E + 覆盖率 | 6h |
| **Total** | **T1-T16** | | **40h** |
