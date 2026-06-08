# Tasks: devrix-multi-agent

**Change ID:** devrix-multi-agent
**Status:** S2 Design
**Based on:** `design.md`, `docs/multi-agent-design.md`, `openspec/specs/multi_agent_layer_delta.md`

---

## Milestone 1: 基础类型与接口定义（P0，6h）

### Definition of Done
- [ ] `contracts.go` — AgentState、AgentConfig、AgentResult、Agent 接口、IAgentFactory、AgentDeps 定义
- [ ] `collaboration/mode.go` + `collaboration/prompt.go` — CollaborationMode 常量 + BuildPromptForMode
- [ ] `internal/shared/errors/multiagent.go` — 9 个 AGT_* 错误码（已完成）
- [ ] `internal/shared/config/multiagent.go` — MultiAgentConfig + Default + Build（已完成）
- [ ] Go 编译通过（零实现，仅类型）

### Tasks

- [ ] **T1**: 创建 `internal/layers/multiagent/contracts.go` — 所有类型、接口、常量
  - L4: AGT-FACTORY
  - L5: L5-4-1-01
  - Estimate: 2h
  - Dependencies: none

- [ ] **T2**: 创建 `collaboration/mode.go` + `collaboration/prompt.go` — CollaborationMode + BuildPromptForMode
  - L4: AGT-COLLAB
  - L5: L5-4-4-01, L5-4-4-02
  - Estimate: 2h
  - Dependencies: T1

- [ ] **T3**: 创建 `observer/adapter.go` + `observer/noop.go` — ObserverAdapter 接口定义
  - L4: AGT-OBSERVER
  - L5: L5-4-5-01
  - Estimate: 2h
  - Dependencies: T1

---

## Milestone 2: Agent 状态机 + 生命周期（P0，10h）

### Definition of Done
- [ ] Agent 状态机 5 状态 + 8 合法转换，非法转换返回 AGT_LIFECYCLE_5001
- [ ] Agent.Run() 主循环委托 IContextEngine.Process()
- [ ] Agent.Terminate() 强制终止 + 取消传播
- [ ] Agent.Wait() 阻塞等待 TERMINATED
- [ ] 状态转换覆盖率 100%

### Tasks

- [ ] **T4**: 创建 `agent/state.go` — AgentState 类型方法 + transition() + ValidTransitions map
  - L4: AGT-LIFECYCLE
  - L5: L5-4-2-01
  - Estimate: 2h
  - Dependencies: T1

- [ ] **T5**: 创建 `agent/agent.go` — agentImpl struct + NewAgent 构造函数
  - L4: AGT-FACTORY, AGT-LIFECYCLE
  - L5: L5-4-1-01, L5-4-2-01
  - Estimate: 3h
  - Dependencies: T4

- [ ] **T6**: 创建 `agent/lifecycle.go` — Run 主循环 + handleEngineEvent + Terminate + Wait
  - L4: AGT-LIFECYCLE
  - L5: L5-4-2-01
  - Estimate: 3h
  - Dependencies: T5

- [ ] **T7**: 编写 `agent/agent_test.go` — TestAgentLifecycle + TestAgentStateTransitions + TestAgentTerminate + TestAgentWait
  - L4: AGT-LIFECYCLE
  - L5: L5-4-2-01, L5-4-2-02
  - Estimate: 2h
  - Dependencies: T6

---

## Milestone 3: Fork/Join 子 Agent（P0，8h）

### Definition of Done
- [ ] Fork 创建子 Agent，共享 `*SessionContext` 指针
- [ ] Fork 检查 MaxChildren=3 硬限制，超限返回 AGT_FACTORY_5002
- [ ] Join 合并子 Agent 结果到 SessionContext.Messages
- [ ] Join 时子 Agent 未完成返回 AGT_FORK_5004
- [ ] 并行 Fork 通过 -race 检测

### Tasks

- [ ] **T8**: 创建 `agent/forkjoin.go` — Fork + Join + collectChildResults
  - L4: AGT-FORKJOIN
  - L5: L5-4-3-01, L5-4-3-02
  - Estimate: 4h
  - Dependencies: T5

- [ ] **T9**: 编写 Fork/Join 测试 — TestAgentForkJoin + TestAgentMaxChildren + TestAgentJoinNotCompleted + TestAgentParallelFork
  - L4: AGT-FORKJOIN
  - L5: L5-4-3-01, L5-4-3-02, L5-4-3-03, L5-4-3-04, L5-4-0-01, L5-4-0-02
  - Estimate: 3h
  - Dependencies: T8

- [ ] **T10**: 创建 `factory/factory.go` — AgentFactory 实现 + validateConfig
  - L4: AGT-FACTORY
  - L5: L5-4-1-01
  - Estimate: 1h
  - Dependencies: T1, T5

---

## Milestone 4: Collaboration Modes + Permission（P1，6h）

### Definition of Done
- [ ] BuildPromptForMode(CoT) 正确增强 prompt
- [ ] BuildPromptForMode(IterativeRefinement) 正确增强 prompt
- [ ] Default 模式不增强
- [ ] 无效 mode 返回 AGT_FACTORY_5006
- [ ] CRITICAL 工具触发 WAITING_PERMISSION 状态
- [ ] 权限批准/拒绝/超时流程正确

### Tasks

- [ ] **T11**: 编写 `collaboration/mode_test.go` — TestBuildCoTPrompt + TestBuildRefinementPrompt + TestDefaultPrompt + TestInvalidMode
  - L4: AGT-COLLAB
  - L5: L5-4-4-01, L5-4-4-02
  - Estimate: 2h
  - Dependencies: T2

- [ ] **T12**: 编写权限处理测试 — TestAgentPermissionFlow + TestAgentPermissionDenied + TestAgentPermissionTimeout
  - L4: AGT-PERMISSION
  - L5: L5-4-2-02, L5-4-2-03
  - Estimate: 2h
  - Dependencies: T6

- [ ] **T13**: 编写 `factory/factory_test.go` — TestAgentFactoryCreate + TestAgentFactoryInvalidConfig + TestAgentFactoryMaxChildren
  - L4: AGT-FACTORY
  - L5: L5-4-1-01
  - Estimate: 2h
  - Dependencies: T10

---

## Milestone 5: Observer 适配器 + 可观测性（P1，4h）

### Definition of Done
- [ ] ObserverAdapter 正确桥接 AgentEvent → IObserver
- [ ] NoOpObserverAdapter 不 panic
- [ ] Span 覆盖所有关键操作（create/run/fork/join/terminate/permission）

### Tasks

- [ ] **T14**: 完善 `observer/adapter.go` — ObserverAdapter 实现 + EmitAgentEvent + 事件类型映射
  - L4: AGT-OBSERVER
  - L5: L5-4-5-01
  - Estimate: 2h
  - Dependencies: T3

- [ ] **T15**: 在 agent.go/lifecycle.go/forkjoin.go 中集成 Span 创建 + Observer 事件上报
  - L4: AGT-OBSERVER
  - L5: L5-4-5-01
  - Estimate: 2h
  - Dependencies: T14, T5

---

## Milestone 6: Bootstrap 集成 + 测试收尾（P1，6h）

### Definition of Done
- [ ] `WireMultiAgent` 引导函数正确注入依赖
- [ ] CommunicationGateway 可选注入 IAgentFactory
- [ ] 集成测试通过（PermissionManager 集成）
- [ ] E2E Fork 场景通过
- [ ] 整体覆盖率 ≥ 80%

### Tasks

- [ ] **T16**: 创建 `internal/bootstrap/multi_agent.go` + 集成测试 + E2E 测试 + 覆盖率验证
  - L4: AGT-BOOTSTRAP
  - L5: L5-4-0-02, L5-4-0-03, L5-4-0-04
  - Estimate: 6h
  - Dependencies: T1-T15

---

## Summary

| Milestone | Tasks | Estimate | Priority |
|-----------|-------|----------|----------|
| M1: Foundation | T1-T3 | 6h | P0 |
| M2: Lifecycle | T4-T7 | 10h | P0 |
| M3: Fork/Join | T8-T10 | 8h | P0 |
| M4: Collab + Permission | T11-T13 | 6h | P1 |
| M5: Observer | T14-T15 | 4h | P1 |
| M6: Bootstrap + Test | T16 | 6h | P1 |
| **Total** | **16 tasks** | **40h** | |

## Dependency Graph

```
T1 ──▶ T2 ──▶ T11
  │
  ├──▶ T3 ──▶ T14 ──▶ T15
  │
  ├──▶ T4 ──▶ T5 ──▶ T6 ──▶ T7 ──▶ T12
  │               │
  │               ├──▶ T8 ──▶ T9
  │               │
  │               └──▶ T10 ──▶ T13
  │
  └──▶ (all) ──▶ T16
```
