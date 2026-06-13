# Delta: Domain D4 (AGENT)

**Change ID:** devrix-foundation → current
**Affects:** agent lifecycle, fork/join, collaboration, delegate, agent tools, session view, observability

---

## Current State Summary

D4 多智能体域已从 V1 基础版本（Agent 状态机 + Fork/Join + Collaboration）演进为完整的 V2 实现能力，新增 Hub-Spoke 委托编排、Agent Tool 子系统、SessionView COW 隔离、WorkerEngine sidechain、Builtin Agents 和可观测性指标。以下记录所有变更：

---

## ADDED

### Requirement: Agent Lifecycle State Machine

Agent 生命周期严格遵循状态机驱动的转换规则。

#### Scenario: Create agent via factory
- GIVEN CommunicationGateway 收到用户消息
- WHEN `AgentFactory.Create(ctx, cfg, session)` 被调用
- THEN 返回新的 Agent 实例，状态 = CREATED
- AND Agent.ID 为非空唯一标识（UUID）
- AND Agent.Config 已设置
- AND Agent.SessionCtx 指向传入的指针

#### Scenario: Run agent — normal flow
- GIVEN Agent 状态 = CREATED
- WHEN `Agent.Run(ctx)` 被调用
- THEN 状态转换: CREATED → RUNNING
- AND 调用 `contracts.IEngine.Process()` 驱动 runLoop
- AND 每次 runLoop 工具轮次时状态 = ITERATING
- AND runLoop 返回 complete event 后状态转换: ITERATING → TERMINATED
- AND 返回 `AgentResult{Messages, ExitCode=0}`

#### Scenario: Agent iteration loop with tool calls
- GIVEN Agent 状态 = ITERATING
- WHEN `contracts.IEngine.Process()` 返回 `tool_call` event
- THEN 工具风险等级被检查
- AND LOW/MEDIUM 等级工具自动执行
- AND CRITICAL 等级工具触发 AgentPermissionGate 权限确认流程

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

### Requirement: Permission Gate

Agent 实现 IPermissionGate 接口，CRITICAL 工具通过 channel 阻塞等待用户响应。

#### Scenario: CRITICAL tool triggers permission
- GIVEN runLoop 返回 tool_call event，工具风险等级 = CRITICAL
- WHEN Agent 处理 tool_call
- THEN 调用 `agentPermissionGate.Request(ctx, sessionID, toolName, input)`
- AND Agent 状态 = WAITING_PERMISSION
- AND 发送 `agent.waiting_permission` 事件

#### Scenario: User grants permission
- GIVEN 权限请求 pending
- WHEN Gateway 调用 `Agent.ResolvePermission(toolName, true)`
- THEN 权限批准，channel 收到 decision
- AND Agent 状态: WAITING_PERMISSION → ITERATING
- AND 工具继续执行

#### Scenario: User denies permission
- GIVEN 权限请求 pending
- WHEN Gateway 调用 `Agent.ResolvePermission(toolName, false)`
- THEN 权限拒绝
- AND Agent 状态: WAITING_PERMISSION → TERMINATED
- AND 返回 `AGT_PERMISSION_5008` 错误

#### Scenario: Permission timeout
- GIVEN 权限请求 pending
- WHEN 60 秒无响应
- THEN 权限超时
- AND Agent 状态: WAITING_PERMISSION → TERMINATED
- AND 返回 `AGT_PERMISSION_5007` 错误

#### Scenario: LOW/MEDIUM risk tools — auto-authorize
- GIVEN runLoop 返回 tool_call event，工具风险等级 != CRITICAL
- WHEN Agent 处理 tool_call
- THEN agentPermissionGate 不触发用户交互
- AND 工具自动授权执行
- AND Agent 状态保持 ITERATING

---

### Requirement: Fork/Join Parallel Sub-Agents

支持父 Agent 创建并行子 Agent 执行独立子任务。

#### Scenario: Fork — create child agent
- GIVEN 父 Agent 需要执行并行子任务
- WHEN `Agent.Fork(ctx, childCfg)` 被调用
- THEN 新的子 Agent 被创建，状态 = CREATED
- AND 子 Agent.session 与父 Agent 共享同一 `*types.Session` 指针
- AND 子 Agent 加入父 Agent 的 `childAgents` 列表
- AND 父 Agent 不阻塞（子 Agent 通过 `Wait()` 等待）
- AND 发送 `agent.forked` 事件
- AND SessionView COW fork 创建子 View 隔离元数据写入

#### Scenario: Fork — max children exceeded
- GIVEN 父 Agent 已有 `MaxChildren=3` 个子 Agent
- WHEN `Fork()` 被再次调用
- THEN 返回 `AGT_MAX_CHILDREN` 错误
- AND 不创建新 Agent

#### Scenario: Fork — worker cannot fork
- GIVEN Agent 是 Worker（cfg.ParentID != ""）
- WHEN `Fork()` 被调用
- THEN 返回 `AGT_INVALID_CONFIG` 错误
- AND 不创建子 Agent

#### Scenario: Join — merge child result
- GIVEN 子 Agent 已完成（状态 = TERMINATED）
- WHEN `Agent.Join(ctx, child)` 被调用
- THEN 子 Agent 的 Messages 追加到父 Agent messageBuffer
- AND tool_call 消息通过 tool_call_id 去重（dedupToolCallMessages）
- AND 子 Agent 从父 Agent 的 `childAgents` 列表中移除
- AND 发送 `agent.joined` 事件

#### Scenario: Join — child not completed
- GIVEN 子 Agent 尚未 TERMINATED
- WHEN `Join(ctx, child)` 被调用
- THEN 返回 `AGT_JOIN_NOT_COMPLETED` 错误
- AND 子 Agent 不受影响

#### Scenario: Fork — parallel execution
- GIVEN 父 Agent 创建了 2 个子 Agent
- WHEN 两个子 Agent 同时 `Run()`
- THEN 两个子 Agent 并行执行
- AND 父 Agent 通过 `Wait()` 等待两者完成
- AND 首次 `Wait()` 阻塞直到对应子 Agent TERMINATED

---

### Requirement: SessionView COW Isolation

DM-20260611-005: 子 Agent 通过 Copy-on-Write View 隔离元数据写入，不污染父 Session。

#### Scenario: Fork creates isolated view
- GIVEN 父 Agent 的 session 包含 context_snapshot
- WHEN `sessionview.Fork(parent)` 被调用
- THEN 返回新 View，ID 为唯一标识
- AND 不可变字段（model, token_budget）与父 Session 共享
- AND 可变字段（metadata, snapshot）初始为空

#### Scenario: Child writes to view do not pollute parent
- GIVEN 子 Agent 通过 Fork 获得隔离 View
- WHEN 子 Agent 写入 `view.SetMetadata(key, value)`
- THEN 写入仅影响子 View
- AND 父 Session.Metadata 不受影响

#### Scenario: MergeToParent on Join
- GIVEN 子 Agent 已完成，view 中有 metadata
- WHEN `view.MergeToParent(parent)` 被调用
- THEN 子 View 的 metadata 合并到父 Session
- AND 父 Session 持久化更新后的状态

#### Scenario: Snapshot isolation
- GIVEN 子 Agent 通过 Fork 获得隔离 View
- WHEN 子 Agent 调用 `view.SetSnapshot(snap)`
- THEN 快照仅存储于子 View
- AND 父 Session.ContextSnapshot 不受影响

---

### Requirement: Collaboration Modes

Agent 支持不同的推理策略，通过 CollaborationMode 配置生效。

#### Scenario: Chain-of-Thought mode
- GIVEN AgentConfig.Mode = "chain-of-thought"
- WHEN `BuildPromptForMode(CoT, basePrompt)` 被调用
- THEN system prompt 被增强：添加逐步推理指令
- AND 增强后的 prompt 包含 "请逐步推理，每一步都要明确说明你的理由"

#### Scenario: Iterative-Refinement mode
- GIVEN AgentConfig.Mode = "iterative-refinement"
- WHEN `BuildPromptForMode(IR, basePrompt)` 被调用
- THEN system prompt 被增强：添加自我批判指令
- AND 增强后的 prompt 包含 "请先给出初始答案，然后进行自我批判"

#### Scenario: Default mode
- GIVEN AgentConfig.Mode = "default" 或空
- WHEN `BuildPromptForMode(default, basePrompt)` 被调用
- THEN system prompt 不做额外增强
- AND 直接使用原始 system prompt

#### Scenario: Invalid mode
- GIVEN AgentConfig.Mode = "invalid-mode"
- WHEN `ValidateMode(mode)` 被调用
- THEN 返回错误

---

### Requirement: Worker Engine

Agent 通过 WorkerEngine 为 worker 注入 sidechain context 覆盖。

#### Scenario: NewWorkerEngine wraps inner engine
- GIVEN contracts.IEngine 实例和 AgentConfig
- WHEN `NewWorkerEngine(inner, cfg, agentID)` 被调用
- THEN 返回 *WorkerEngine 实现 contracts.IEngine
- AND WorkerEngine 持有 AgentID / WorkerRole / SystemPrompt / ModelTier

#### Scenario: Process injects overlay
- GIVEN WorkerEngine 包装了 inner engine
- WHEN `Process(ctx, session, message)` 被调用
- THEN 通过 ProcessOverlay 向 session 注入 worker sidechain context
- AND 调用 inner.Process(ctx, session, message) 驱动 runLoop
- AND 返回 engine event channel

---

### Requirement: Hub-Spoke Delegate Service

DM-20260610-012: Leader Agent 通过 Delegate Service 将子任务委派给 Worker Agent 执行。

#### Scenario: DelegateSync creates and runs worker
- GIVEN Leader Agent 调用 delegate_explore 工具
- WHEN `Service.DelegateSync(ctx, leader, spec)` 被调用
- THEN Service.Fork 创建 Worker Agent
- AND Worker Agent 被 Run 执行
- AND 返回 DelegateResult{Summary, Messages}
- AND FlowBridge 发布 FlowStarted → FlowCompleted 事件

#### Scenario: DelegateAsync runs worker in background
- GIVEN Leader Agent 调用 delegate_implement 工具
- WHEN `Service.DelegateAsync(ctx, leader, spec)` 被调用
- THEN Worker Agent 被创建并异步运行
- AND 返回 task_id
- AND 完成时通过 SessionQueue 通知 Leader 主线程

#### Scenario: DelegateOrFallback when delegate disabled
- GIVEN Delegate Service 未启用
- WHEN `Service.DelegateOrFallback(ctx, leader, spec)` 被调用
- THEN 降级调用 SubQuery fallback
- AND 不创建 Worker Agent

#### Scenario: Worker runs in worktree sandbox
- GIVEN DelegateSync 被调用
- WHEN Worker 需要文件系统隔离
- THEN Worker 在 worktree sandbox 中运行
- AND worktree 在完成/失败时清理

#### Scenario: Worker cannot delegate or fork
- GIVEN Worker Agent 通过 Delegate Service 创建
- WHEN Worker 尝试调用 delegate_* 工具或 Fork
- THEN 操作被拒绝
- AND Worker 只能执行分配的单一任务

---

### Requirement: Flow Bridge

Delegate Service 通过 FlowBridge 将 Agent 生命周期事件发布到 ExecutionFlowHub (D7)。

#### Scenario: Emit agent event to flow hub
- GIVEN FlowBridge 绑定到 ExecutionFlowHub
- WHEN `FlowBridge.EmitAgentEvent(ev)` 被调用
- THEN AgentEvent 映射为 FlowEventKind
- AND 发布到 ExecutionFlowHub

#### Scenario: Engine event sink forwards to flow
- GIVEN FlowBridge 绑定到 Worker Agent
- WHEN Worker 的 runLoop 产生 engine event
- THEN EngineEventSink 将 engine event 转发到 FlowBridge
- AND FlowBridge 发布 FlowProgress 事件到 Hub

---

### Requirement: Agent Tools

CLI 和 Cursor 子进程 Agent Tool 注册与执行。

#### Scenario: Register agent tool
- GIVEN AgentTool 实现 Info/Execute/ExecutionTimeout 接口
- WHEN `Registry.Register(tool)` 被调用
- THEN tool 按名称注册
- AND 可通过 Get(name) 查找
- AND 可通过 List() 枚举所有已注册 tool

#### Scenario: Execute CLI agent tool
- GIVEN CLI Agent Tool 已注册
- WHEN `CLIAgentTool.Execute(ctx, sessionID, req)` 被调用
- THEN 启动子进程（首次调用创建 session）
- AND 后续调用复用同一进程
- AND 子进程 stdout 通过 ParseStreamJSONLine 解析
- AND 返回 Event channel

#### Scenario: CLI session lifecycle
- GIVEN CLI session 已创建
- WHEN session 空闲超过 IdleTimeout
- THEN idleSweeper 回收子进程
- WHEN D1 Session 销毁
- THEN CleanupBySessionID 清理关联 Agent Tool 子进程

#### Scenario: Execute Cursor agent tool
- GIVEN Cursor Agent Tool 已注册
- WHEN `CursorAgentTool.Execute(ctx, sessionID, req)` 被调用
- THEN 启动子进程，解析 stream-json 输出
- AND 支持 assistant/text/thinking/tool_call/result 事件类型
- AND session 复用与并发隔离

#### Scenario: Parse stream JSON output
- GIVEN 子进程 stdout 输出一行 JSON
- WHEN `ParseStreamJSONLine(line)` 被调用
- THEN 解析 devrix 格式: `{"type":"...","content":"..."}`
- AND 解析 Claude 格式: `{"type":"assistant","message":{"content":[...]}}`
- AND 解析 Claude result 格式（成功/错误）
- AND 忽略 Claude system 事件

---

### Requirement: Builtin Agents

SubQuery 内置 Agent 作为 delegate 降级回退。

#### Scenario: Run explore agent
- GIVEN delegate 不可用或降级
- WHEN `RunExplore(ctx, deps, prompt, tools, maxTurns)` 被调用
- THEN 创建 SubQuery 执行探索任务
- AND 返回 SubQueryResult

#### Scenario: Run plan agent
- GIVEN delegate 不可用或降级
- WHEN `RunPlan(ctx, deps, prompt, tools, maxTurns)` 被调用
- THEN 创建 SubQuery 执行规划任务
- AND 返回 SubQueryResult

#### Scenario: Run implement agent
- GIVEN delegate 不可用或降级
- WHEN `RunImplement(ctx, deps, prompt, tools, maxTurns)` 被调用
- THEN 创建 SubQuery 执行实现任务
- AND 返回 SubQueryResult

---

### Requirement: Observability Metrics

Agent 可观测性计数器与 D5 集成。

#### Scenario: Record fork policy metrics
- GIVEN Fork 创建子 Agent
- WHEN `observability.IncForkSessionView(policy)` 被调用
- THEN 按 policy 标签递增计数器
- AND 支持 "cow" / "snapshot" 策略标签

#### Scenario: Concurrent counter atomicity
- GIVEN 多个 Fork 并发执行
- WHEN 并发调用 IncForkSessionView
- THEN 计数器通过 sync/atomic 保证原子性
- AND 最终计数准确

#### Scenario: D5 sink integration
- GIVEN D5 observability.Bridge 已初始化
- WHEN `observability.SetD5Sink(sink)` 被调用
- THEN 指标可通过 D5Sink 导出到外部可观测性系统

---

### Requirement: Agent Observer Bridge

Agent 生命周期事件通过 Observer 桥接到外部系统。

#### Scenario: Emit agent event to observer chain
- GIVEN Agent 状态转换
- WHEN `AgentObserverChain.EmitAgentEvent(event)` 被调用
- THEN 事件依次传递给链中所有 Observer
- AND 事件包含 agent_id, session_id, event_type, metadata

#### Scenario: NoOp when observer is nil
- GIVEN AgentDeps.AgentObserver 为 NoOpAgentObserver
- WHEN Agent 发送事件
- THEN 不会 panic
- AND 事件被静默丢弃

---

## MODIFIED

(None — 所有 V1 基础需求在 V2 中保留并增强，无破坏性变更)

---

## REMOVED

(None — 原 REMOVED 节的 Supervisor-Worker mode、完整 COW SessionContext 等已在 V2 实现)

---

## V1 → V2 Feature Matrix

| Feature | V1 | V2 |
|---------|-----|-----|
| Agent 状态机 | CREATED→RUNNING→ITERATING→TERMINATED | +WAITING_PERMISSION 状态 |
| Permission Gate | 预留接口 | agentPermissionGate + channel 异步确认 |
| Fork/Join | 消息隔离 + Join 合并 | +SessionView COW 隔离 + tool_call_id 去重 |
| Collaboration | CoT / IR 提示词增强 | 无变更 |
| Observer | NoOpAgentObserver | +AgentObserverChain 多播 |
| Delegate Service | — | Hub-Spoke 同步/异步/回退 |
| FlowBridge | — | AgentEvent → ExecutionFlowHub |
| Agent Tools | — | CLI + Cursor 子进程适配器 |
| Stream JSON | — | ParseStreamJSONLine (devrix + Claude 格式) |
| Builtin Agents | — | Explore / Plan / Implement SubQuery |
| Worker Engine | — | NewWorkerEngine + ProcessOverlay sidechain |
| SessionView COW | — | Fork / SetMetadata / SetSnapshot / MergeToParent |
| Observability | — | IncForkSessionView + D5Sink |
| 跨域依赖 | D1, D2, D5 | +D7 (ExecutionFlowHub) |

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-08 | V1 foundation delta (DM-20260608-005) |
| 2.0.0 | 2026-06-14 | Complete V1→V2 rewrite; added SessionView COW, Delegate, Agent Tools, WorkerEngine, Builtin Agents, FlowBridge, Observability |
