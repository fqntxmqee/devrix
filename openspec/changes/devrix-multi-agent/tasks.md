# Tasks: devrix-multi-agent

**Change ID:** devrix-multi-agent
**Status:** S4 Development
**Grill Review:** 2026-06-08（6 决策合入，16 任务重组为 4 PR）

---

## PR1: contracts.IEngine 跨层契约提取（P0，4h）✅

### Definition of Done
- [x] `shared/contracts/engine.go` — IEngine + EngineEvent（含 ToolRisk 字段）
- [x] `gateway/gateway.go` — IContextEngine 改为嵌入 contracts.IEngine
- [x] `contextengine/pev_engine.go` — tool_call 事件填充 ToolRisk
- [x] 所有现有测试全绿

### Tasks

- [x] **T1**: 创建 `internal/shared/contracts/engine.go`
- [x] **T2**: 更新 `gateway/gateway.go` — type alias
- [x] **T3**: 更新 `pev_engine.go` — ToolRisk 字段

---

## PR2: AgentPermissionGate（P0，6h）✅

### Definition of Done
- [x] `agent/perm_gate.go` — agentPermissionGate 实现 IPermissionGate
- [x] channel 阻塞等待 + ResolvePermission 外部注入 + 超时兜底
- [x] `agent/perm_gate_test.go` 全绿

### Tasks

- [x] **T4**: 创建 `agent/perm_gate.go`
- [x] **T5**: `agent/agent.go` — permGate + ResolvePermission
- [x] **T6**: `agent/perm_gate_test.go`

---

## PR3: Agent 核心实现（P0+P1，24h）✅

### Definition of Done
- [x] contracts.go — ResolvePermission + GetMessages + contracts.IEngine
- [x] 状态机 + Run/Wait/Terminate
- [x] Fork/Join（消息隔离模型）
- [x] Factory + 双层限额（MaxChildren + MaxTotalAgents）
- [x] CollaborationMode + Observer 骨架
- [x] 单元测试全绿 + `-race` 通过

### Tasks

- [x] **T7**: `contracts.go`
- [x] **T8**: `agent/state.go`
- [x] **T9**: `agent/agent.go`
- [x] **T10**: `agent/lifecycle.go`
- [x] **T11**: `agent/forkjoin.go`
- [x] **T12**: `factory/factory.go`
- [x] **T13**: `collaboration/*`
- [x] **T14**: `observer/*`
- [x] **T15**: 全部单元测试

---

## PR4: Bootstrap 集成 + 测试收尾（P1，8h）⏳

### Definition of Done
- [ ] `bootstrap/multi_agent.go` — WireMultiAgent 正确注入
- [ ] CommunicationGateway 可选注入 IAgentFactory + ResolvePermission 路由
- [ ] Permission 集成测试通过
- [ ] E2E Fork 场景通过

### Tasks

- [ ] **T16**: Bootstrap + Gateway 注入 + 集成/E2E 测试
  - L5: L5-4-0-03, L5-4-0-04

---

## Summary

| PR | 状态 | Tasks |
|----|------|-------|
| PR1 contracts | ✅ | T1-T3 |
| PR2 perm gate | ✅ | T4-T6 |
| PR3 Agent 核心 | ✅ | T7-T15 |
| PR4 bootstrap | ⏳ | T16 |
