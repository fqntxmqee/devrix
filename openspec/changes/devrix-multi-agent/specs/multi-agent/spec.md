# Multi-Agent Layer V1 — Acceptance Specification

**Change ID:** devrix-multi-agent
**Status:** S2 Design

---

## Feature: Agent Lifecycle State Machine

Agent 遵循严格的状态机驱动的生命周期管理。

### Scenario: Create agent via factory
- GIVEN CommunicationGateway 收到用户消息
- WHEN `AgentFactory.Create(ctx, cfg, sessionCtx)` 被调用
- THEN 返回 Agent 实例，状态 = CREATED
- AND Agent.ID 为非空 UUID
- AND Agent.Config 已正确设置
- AND Agent.SessionCtx 指向传入的指针

### Scenario: Run agent through full lifecycle
- GIVEN Agent 状态 = CREATED
- WHEN `Agent.Run(ctx)` 被调用
- THEN 状态转换: CREATED → RUNNING
- AND 调用 `IContextEngine.Process()`
- AND Process 返回 event channel
- AND 逐条消费 event
- AND `complete` event 后状态: → TERMINATED
- AND 返回 `AgentResult{Messages, ExitCode=0}`

### Scenario: Run agent — iteration with tool calls
- GIVEN Agent 状态 = ITERATING
- WHEN `IContextEngine.Process()` 返回 `tool_call` event
- THEN 工具风险等级被检查
- AND LOW/MEDIUM 等级工具自动执行，状态保持 ITERATING
- AND CRITICAL 等级工具状态: ITERATING → WAITING_PERMISSION

### Scenario: State transition — all valid paths
- GIVEN 状态机 5 个状态
- WHEN 遍历所有合法状态转换对
- THEN 所有合法转换成功
- AND CREATED→RUNNING, RUNNING→ITERATING, ITERATING→ITERATING
- AND ITERATING→WAITING_PERMISSION, ITERATING→TERMINATED
- AND WAITING_PERMISSION→ITERATING, WAITING_PERMISSION→TERMINATED
- AND any→TERMINATED (Terminate/Cancel)

### Scenario: State transition — invalid transition rejected
- GIVEN Agent 状态 = TERMINATED
- WHEN `Run()` 被再次调用
- THEN 返回 `AGT_LIFECYCLE_5003` 错误
- AND Agent 状态保持 TERMINATED

### Scenario: Agent terminates gracefully
- GIVEN Agent 处于 RUNNING 或 ITERATING
- WHEN `Terminate(ctx)` 被调用
- THEN 状态: → TERMINATED
- AND context 取消传播到 ContextEngine
- AND 子 Agent 全部被取消
- AND 发送 `agent.terminated` 事件

### Scenario: Agent timeout
- GIVEN Agent Config.Timeout = 1s
- WHEN Agent 执行超过 1s
- THEN 状态: → TERMINATED
- AND 返回 `AGT_LIFECYCLE_5005` 错误

### Scenario: Context cancellation
- GIVEN Agent 正在 RUNNING
- WHEN 外部 context 被取消
- THEN Agent 终止循环
- AND 状态: → TERMINATED
- AND 返回 `AGT_CONTEXT_5009` 错误

---

## Feature: Fork/Join Parallel Sub-Agents

父 Agent 可创建并行子 Agent 执行独立子任务。

### Scenario: Fork creates child agent with shared context
- GIVEN 父 Agent 需要并行子任务
- WHEN `Agent.Fork(ctx, childCfg)` 被调用
- THEN 子 Agent 状态 = CREATED
- AND 子 Agent.SessionCtx == 父 Agent.SessionCtx（同一指针）
- AND 子 Agent 加入 `childAgents` map
- AND 发送 `agent.forked` 事件

### Scenario: Fork respects MaxChildren limit
- GIVEN 父 Agent 已有 3 个子 Agent（MaxChildren=3）
- WHEN `Fork()` 被再次调用
- THEN 返回 `AGT_FACTORY_5002` 错误
- AND 不创建新 Agent
- AND 现有子 Agent 不受影响

### Scenario: Join merges child result into SessionContext
- GIVEN 子 Agent 状态 = TERMINATED
- WHEN `Join(ctx, child)` 被调用
- THEN 子 Agent.Messages 追加到 SessionContext.Messages
- AND 子 Agent 从 `childAgents` 中移除
- AND 发送 `agent.joined` 事件

### Scenario: Join before child complete returns error
- GIVEN 子 Agent 状态 != TERMINATED
- WHEN `Join(ctx, child)` 被调用
- THEN 返回 `AGT_FORK_5004` 错误
- AND 子 Agent 状态不受影响

### Scenario: Parallel child agents execute concurrently
- GIVEN 父 Agent 创建 2 个子 Agent
- WHEN 父 Agent 调用子 Agent.Run()（分别在 goroutine 中）
- THEN 两个子 Agent 并行执行
- AND 通过 `-race` 检测，无 data race

### Scenario: Wait blocks until child terminal
- GIVEN 子 Agent 正在 RUNNING
- WHEN `Wait(ctx)` 被调用
- THEN 阻塞直到子 Agent 进入 TERMINATED
- AND 返回子 Agent 的 AgentResult

---

## Feature: Collaboration Modes

Agent 支持不同的推理策略，通过 CollaborationMode 配置生效。

### Scenario: Chain-of-Thought prompt enhancement
- GIVEN AgentConfig.Mode = "chain-of-thought"
- WHEN BuildPromptForMode 被调用
- THEN 返回增强 prompt，包含 "请逐步推理，每一步都要明确说明你的理由"

### Scenario: Iterative-Refinement prompt enhancement
- GIVEN AgentConfig.Mode = "iterative-refinement"
- WHEN BuildPromptForMode 被调用
- THEN 返回增强 prompt，包含 "请先给出初始答案，然后进行自我批判"

### Scenario: Default mode — no enhancement
- GIVEN AgentConfig.Mode = "default" 或空字符串
- WHEN BuildPromptForMode 被调用
- THEN 返回原始 basePrompt 不做修改

### Scenario: Invalid mode rejected
- GIVEN AgentConfig.Mode = "invalid-mode"
- WHEN AgentFactory.Create 被调用
- THEN 返回 `AGT_FACTORY_5006` 错误

---

## Feature: Permission Delegation

CRITICAL 风险等级工具委托 PermissionManager 进行用户确认。

### Scenario: CRITICAL tool triggers permission state
- GIVEN PEVEngine 返回 tool_call，riskLevel = CRITICAL
- WHEN Agent 处理 tool_call
- THEN Agent 状态: ITERATING → WAITING_PERMISSION
- AND 调用 `PermissionManager.Request(ctx, sessionID, toolName, input, CRITICAL)`
- AND 发送 `agent.waiting_permission` 事件

### Scenario: Grant — resume iteration
- GIVEN Agent 状态 = WAITING_PERMISSION
- WHEN PermissionManager 返回 true（批准）
- THEN Agent 状态: WAITING_PERMISSION → ITERATING
- AND 工具正常执行

### Scenario: Deny — terminate agent
- GIVEN Agent 状态 = WAITING_PERMISSION
- WHEN PermissionManager 返回 false（拒绝）
- THEN Agent 状态: WAITING_PERMISSION → TERMINATED
- AND 返回 `AGT_PERMISSION_5008` 错误

### Scenario: Timeout — terminate agent
- GIVEN Agent 状态 = WAITING_PERMISSION
- WHEN 60s 超时
- THEN PermissionManager 返回 false
- AND Agent 状态: WAITING_PERMISSION → TERMINATED
- AND 返回 `AGT_PERMISSION_5007` 错误

### Scenario: Non-CRITICAL tools auto-authorize
- GIVEN PEVEngine 返回 tool_call，riskLevel = LOW / MEDIUM / HIGH
- WHEN Agent 处理 tool_call
- THEN 不调用 PermissionManager.Request
- AND Agent 状态保持 ITERATING
- AND 工具自动执行

---

## Feature: Observer Adapter

Agent 生命周期事件通过适配器桥接到 contextengine.IObserver。

### Scenario: Agent lifecycle events forwarded to IObserver
- GIVEN AgentDeps.Observer 非 nil
- WHEN Agent 经历 CREATED → RUNNING → ITERATING → TERMINATED
- THEN 每个状态转换触发对应 AgentEvent
- AND IObserver 被调用（验证 mock observer 收到事件）

### Scenario: NoOp observer — no panic
- GIVEN AgentDeps.Observer = NoOpObserverAdapter
- WHEN Agent 经历完整生命周期
- THEN 不 panic
- AND 事件被静默丢弃

---

## Feature: Bootstrap Integration

WireMultiAgent 正确构建依赖并注入到 CommunicationGateway。

### Scenario: WireMultiAgent returns IAgentFactory
- GIVEN 所有依赖可用（IContextEngine, PermissionManager, IObserver, ObservabilityBridge, MultiAgentConfig）
- WHEN `WireMultiAgent(...)` 被调用
- THEN 返回非 nil 的 IAgentFactory 实例
- AND Factory 可正常创建 Agent

### Scenario: Gateway injects AgentFactory
- GIVEN CommunicationGateway 构造时注入 IAgentFactory
- WHEN Gateway 处理用户消息
- THEN 可通过 AgentFactory 创建 Agent（V1 由 `/fork` 命令触发）

---

## Feature: Error Codes

所有错误使用 AGT_* 前缀错误码体系。

### Scenario: Error wrapped with SentinelError
- GIVEN 任何 AGT_* 错误被创建
- WHEN 调用 `errors.Is(err, ErrAgentXxx)`
- THEN 返回 true
- AND `err.(*SentinelError).Code` 返回正确的 AGT_* 错误码

---

## Golden Test Cases

### State Transition Matrix

```
From          → To                     Expected
CREATED       → RUNNING                OK
CREATED       → ITERATING              REJECT (AGT_LIFECYCLE_5001)
CREATED       → WAITING_PERMISSION     REJECT (AGT_LIFECYCLE_5001)
CREATED       → TERMINATED             REJECT (AGT_LIFECYCLE_5001)
RUNNING       → ITERATING              OK
RUNNING       → TERMINATED             OK (Terminate)
RUNNING       → CREATED                REJECT (AGT_LIFECYCLE_5001)
ITERATING     → ITERATING              OK
ITERATING     → WAITING_PERMISSION     OK
ITERATING     → TERMINATED             OK
WAITING_PERMISSION → ITERATING         OK
WAITING_PERMISSION → TERMINATED        OK
TERMINATED    → RUNNING                REJECT (AGT_LIFECYCLE_5003)
TERMINATED    → ITERATING              REJECT (AGT_LIFECYCLE_5003)
```

### Fork/Join Integration Flow

```
1. Factory.Create(parentCfg, sessionCtx) → parentAgent(CREATED)
2. parentAgent.Run() → RUNNING → ITERATING
3. parentAgent.Fork(childCfg) → childAgent(CREATED)
4. parentAgent.Fork(childCfg2) → childAgent2(CREATED)
5. go childAgent.Run()  ┐
6. go childAgent2.Run() ┤ 并行执行
7. parentAgent.Wait(child) → blocks
8. parentAgent.Wait(child2) → blocks
9. parentAgent.Join(child) → merge messages
10. parentAgent.Join(child2) → merge messages
11. parentAgent.Terminate() → TERMINATED
```
