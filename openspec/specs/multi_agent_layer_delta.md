# Delta: Multi-Agent Layer (Layer 4)

**Change ID:** devrix-multi-agent
**Demand ID:** DM-20260608-005
**版本:** 1.0.0
**父设计:** `docs/multi-agent-design.md`
**实施设计:** `openspec/changes/devrix-multi-agent/design.md`（Grill Review 2026-06-08）
**关联 L5 Registry:** `openspec/l5-registry.md` (L5-4-*)

> **Grill Review 修正（2026-06-08）：** 权限改为 AgentPermissionGate（Agent 实现 IPermissionGate）；Fork 改为消息隔离 + Join 合并；新增 MaxTotalAgents 双层限额；跨层契约提取为 `contracts.IEngine`。详见 change design.md。

---

## ADDED

### Requirement: Agent Lifecycle State Machine

Agent 生命周期严格遵循状态机驱动的转换规则。

#### Scenario: Create agent via factory
- GIVEN CommunicationGateway 收到用户消息
- WHEN `AgentFactory.Create(ctx, cfg, sessionCtx)` 被调用
- THEN 返回新的 Agent 实例，状态 = CREATED
- AND Agent.ID 为非空唯一标识（UUID）
- AND Agent.Config 已设置
- AND Agent.SessionCtx 指向传入的指针

#### Scenario: Run agent — normal flow
- GIVEN Agent 状态 = CREATED
- WHEN `Agent.Run(ctx)` 被调用
- THEN 状态转换: CREATED → RUNNING
- AND 调用 `IContextEngine.Process()` 驱动 PEVEngine
- AND 每次 PEV 迭代时状态 = ITERATING
- AND PEVEngine 返回 complete event 后状态转换: ITERATING → TERMINATED
- AND 返回 `AgentResult{Messages, ExitCode=0}`

#### Scenario: Agent iteration loop with tool calls
- GIVEN Agent 状态 = ITERATING
- WHEN `IContextEngine.Process()` 返回 `tool_call` event
- THEN 工具风险等级被检查
- AND LOW/MEDIUM 等级工具自动执行
- AND CRITICAL 等级工具触发权限确认流程

#### Scenario: State transition — valid transitions
- GIVEN Agent 状态 = CREATED
- WHEN Run() 被调用 → RUNNING
- WHEN Process() 返回 → ITERATING
- WHEN CRITICAL tool_call → WAITING_PERMISSION
- WHEN permission granted → ITERATING
- WHEN permission denied → TERMINATED
- WHEN 无更多 tool_call → TERMINATED

#### Scenario: State transition — invalid transition rejected
- GIVEN Agent 状态 = TERMINATED
- WHEN `Run()` 被再次调用
- THEN 返回 `AGT_LIFECYCLE_5003` 错误
- AND Agent 状态保持 TERMINATED

#### Scenario: Terminate agent
- GIVEN Agent 处于任意活跃状态（RUNNING / ITERATING / WAITING_PERMISSION）
- WHEN `Agent.Terminate(ctx)` 被调用
- THEN 状态转换: → TERMINATED
- AND context 取消信号传播到 ContextEngine
- AND 所有子 Agent 被传播取消
- AND 发送 `agent.terminated` 事件

#### Scenario: Agent timeout
- GIVEN Agent 配置 `Timeout = 5m`
- WHEN Agent 执行超过 5 分钟
- THEN 状态转换: → TERMINATED
- AND 返回 `AGT_LIFECYCLE_5005` 错误
- AND 子 Agent 被传播取消

#### Scenario: Context cancellation
- GIVEN Agent 正在 RUNNING 或 ITERATING
- WHEN 外部 context 被取消（用户 `/stop` 命令）
- THEN Agent 终止当前循环
- AND 状态转换: → TERMINATED
- AND 返回 `AGT_CONTEXT_5009` 错误

---

### Requirement: Fork/Join Parallel Sub-Agents

支持父 Agent 创建并行子 Agent 执行独立子任务。

#### Scenario: Fork — create child agent
- GIVEN 父 Agent 需要执行并行子任务
- WHEN `Agent.Fork(ctx, childCfg)` 被调用
- THEN 新的子 Agent 被创建，状态 = CREATED
- AND 子 Agent.SessionCtx 与父 Agent 共享同一 `*types.SessionContext` 指针
- AND 子 Agent 加入父 Agent 的 `childAgents` 列表
- AND 父 Agent 不阻塞（子 Agent 通过 `Wait()` 等待）
- AND 发送 `agent.forked` 事件

#### Scenario: Fork — max children exceeded
- GIVEN 父 Agent 已有 `MaxChildren=3` 个子 Agent
- WHEN `Fork()` 被再次调用
- THEN 返回 `AGT_FACTORY_5002` 错误
- AND 不创建新 Agent

#### Scenario: Join — merge child result
- GIVEN 子 Agent 已完成（状态 = TERMINATED）
- WHEN `Agent.Join(ctx, child)` 被调用
- THEN 子 Agent 的 `AgentResult.Messages` 追加到 SessionContext
- AND 子 Agent 从父 Agent 的 `childAgents` 列表中移除
- AND 发送 `agent.joined` 事件

#### Scenario: Join — child not completed
- GIVEN 子 Agent 尚未 TERMINATED
- WHEN `Join(ctx, child)` 被调用
- THEN 返回 `AGT_FORK_5004` 错误
- AND 子 Agent 不受影响

#### Scenario: Fork — parallel execution
- GIVEN 父 Agent 创建了 2 个子 Agent
- WHEN 两个子 Agent 同时 `Run()`
- THEN 两个子 Agent 并行执行
- AND 父 Agent 通过 `Wait()` 等待两者完成
- AND 首次 `Wait()` 阻塞直到对应子 Agent TERMINATED

---

### Requirement: Collaboration Modes

Agent 支持不同的推理策略，通过 CollaborationMode 配置生效。

#### Scenario: Chain-of-Thought mode
- GIVEN AgentConfig.Mode = "chain-of-thought"
- WHEN AgentFactory.Create 被调用
- THEN system prompt 被增强：添加逐步推理指令
- AND 增强后的 prompt 包含 "请逐步推理，每一步都要明确说明你的理由"
- AND Agent.Run 使用增强后的 prompt 调用 ContextEngine

#### Scenario: Iterative-Refinement mode
- GIVEN AgentConfig.Mode = "iterative-refinement"
- WHEN AgentFactory.Create 被调用
- THEN system prompt 被增强：添加自我批判指令
- AND 增强后的 prompt 包含 "请先给出初始答案，然后进行自我批判"
- AND Agent.Run 使用增强后的 prompt 调用 ContextEngine

#### Scenario: Default mode
- GIVEN AgentConfig.Mode = "default" 或空
- WHEN AgentFactory.Create 被调用
- THEN system prompt 不做额外增强
- AND 直接使用原始 system prompt

#### Scenario: Invalid mode
- GIVEN AgentConfig.Mode = "invalid-mode"
- WHEN AgentFactory.Create 被调用
- THEN 返回 `AGT_FACTORY_5006` 错误

---

### Requirement: Permission Delegation

CRITICAL 风险等级工具的执行必须经用户确认，委托给 gateway.PermissionManager。

#### Scenario: CRITICAL tool triggers permission
- GIVEN PEVEngine 返回 tool_call event，工具风险等级 = CRITICAL
- WHEN Agent 处理 tool_call
- THEN 调用 `PermissionManager.Request(ctx, sessionID, toolName, input, risk)`
- AND Agent 状态 = WAITING_PERMISSION
- AND 发送 `agent.waiting_permission` 事件

#### Scenario: User grants permission
- GIVEN 权限请求 pending
- WHEN 用户响应 "yes"
- THEN 权限批准
- AND Agent 状态: WAITING_PERMISSION → ITERATING
- AND 工具继续执行

#### Scenario: User denies permission
- GIVEN 权限请求 pending
- WHEN 用户响应 "no"
- THEN 权限拒绝
- AND Agent 状态: WAITING_PERMISSION → TERMINATED
- AND 返回 `AGT_PERMISSION_5008` 错误

#### Scenario: Permission timeout
- GIVEN 权限请求 pending
- WHEN 60 秒无响应
- THEN 权限超时
- AND Agent 状态: WAITING_PERMISSION → TERMINATED
- AND 返回 `AGT_PERMISSION_5007` 错误

#### Scenario: LOW/MEDIUM/HIGH risk tools — auto-authorize
- GIVEN PEVEngine 返回 tool_call event，工具风险等级 != CRITICAL
- WHEN Agent 处理 tool_call
- THEN PermissionManager 不触发用户交互
- AND 工具自动授权执行
- AND Agent 状态保持 ITERATING

---

### Requirement: Observer Adapter

Agent 生命周期事件通过适配器桥接到 contextengine.IObserver。

#### Scenario: Agent created event
- GIVEN AgentFactory.Create 成功
- WHEN Agent 实例创建
- THEN 调用 `ObserverAdapter.EmitAgentEvent("agent.created", ...)`
- AND 事件包含 agent_id, session_id, mode

#### Scenario: Agent terminated event
- GIVEN Agent 进入 TERMINATED 状态
- WHEN 状态转换完成
- THEN 调用 `ObserverAdapter.EmitAgentEvent("agent.terminated", ...)`
- AND 事件包含 duration_ms, exit_code, error（如有）

#### Scenario: NoOp when observer is nil
- GIVEN AgentDeps.Observer = nil（注入 NoOpObserverAdapter）
- WHEN Agent 发送事件
- THEN 不会 panic
- AND 事件被静默丢弃

---

### Requirement: SessionContext Concurrency Safety

多 Agent 共享 SessionContext 时必须线程安全。

#### Scenario: Concurrent Fork + Join
- GIVEN 2 个子 Agent 并行执行
- WHEN 两个子 Agent 同时 Join（追加消息到 SessionContext）
- THEN 不产生 data race（通过 -race 检测）
- AND 所有消息被正确追加（无丢失、无乱序）

#### Scenario: Shared pointer mutation safety
- GIVEN 父 Agent 和子 Agent 共享同一个 `*types.SessionContext`
- WHEN 任一 Agent 追加消息到 Messages[]
- THEN 使用 `sync.RWMutex` 写锁保护
- AND 其他 Agent 的读操作使用读锁保护

---

## MODIFIED

(None — initial layer specification)

---

## REMOVED

| Item | Reason |
|------|--------|
| Supervisor-Worker mode (task decomposition) | V2 功能，V1 不实现 |
| Peer-Review mode (multi-agent review) | V3 功能 |
| Vote-Consensus mode (multi-agent voting) | V3 功能 |
| Full Fork/Merge with Milestone DAG | V3 功能 |
| 自建 Tool Registry | 复用 `contextengine/registry` |
| 自建 Permission Pipeline | 委托 `gateway.PermissionManager` |
| 自建 Observer/Event Bus | 适配器桥接到 `contextengine.IObserver` |
| Agent 持久化 | V2 功能 |
| 完整 COW SessionContext | V2 功能 |
