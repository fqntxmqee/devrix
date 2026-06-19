# Design: D2 QueryLoop 拆解

**Change ID:** devrix-d2-queryloop-dismantle  
**Demand ID:** DM-20260618-010

---

## 1. 设计原则

1. **D2 无循环** — 任何 `for { llm; tools }` 只能在 D7
2. **D2 无 LLM** — D2 不持有 `LLMCaller`；D7→D3 直调
3. **D2 无编排** — 不 import `orchestration/`；SubQuery Flow 由 D7 注入
4. **最小 SubTurn** — 复用现有 `turn_adapter` 三拆面，不新造 D2 API

---

## 2. D7 SubTurn 扩展

### 2.1 TurnScope

```go
type TurnScope string

const (
    TurnScopeMain         TurnScope = "main"
    TurnScopeSub          TurnScope = "sub"          // delegate_subquery
    TurnScopeBackground   TurnScope = "background"  // async subquery
    TurnScopeWaveWorker   TurnScope = "wave_worker" // wave subagent
)
```

### 2.2 SubTurnRequest（扩展 TurnRequest）

| 字段 | 用途 |
|------|------|
| `Scope` | sub / background / wave_worker |
| `ParentSessionID` | fork 来源 session |
| `AgentID` / `AgentName` | sidechain 键 |
| `ReadOnlyTools` | explore/plan 过滤 |
| `FlowReporter` | D7-S4 SubQuery flow |
| `Sidechain` | transcript 追加 |
| `MaxTurns` | 子 agent 上限（默认 30） |

### 2.3 SubTurnExecutor 接口

```go
type SubTurnExecutor interface {
    RunSubTurn(ctx context.Context, req SubTurnRequest) (*SubTurnResult, error)
}
```

实现：`DefaultOrchestrator.RunTurn` 已支持 scope 参数时直接复用；background 路径用 goroutine + RunRegistry/BackgroundRegistry 包装（与现有 `RunBackground` 语义对齐）。

---

## 3. 迁移映射

| 现状 | 目标 |
|------|------|
| `enforce.Run(..., LoopDeps{Loop})` | `SubTurnExecutor.RunSubTurn(scope=sub)` |
| `enforce.RunBackground(..., LoopDeps{Loop})` | `go SubTurnExecutor.RunSubTurn(scope=background)` + registry |
| `wire_wave buildSubAgentDeps → RunBackground` | `SubTurnExecutor.RunSubTurn(scope=wave_worker)` |
| `engine.Process → queryLoop.Run` | **删除**或 facade 转发 D7（过渡期 1 PR） |
| `d2Executor.RunQueryLoop` | **删除** |
| `turn.QueryLLMCaller` | **删除**（D7 已有 `GatewayInvoker`） |

---

## 4. D2 Engine 瘦身后形态

```go
type ContextEngine struct {
    // 保留
    memory, counter, prompt, toolsReg, permission, ...
    // 删除
    // queryLoop *query.Loop
}

// 对外仅 Facade：
func (e *ContextEngine) Prepare(...) (...)   // → turn_adapter 已有
func (e *ContextEngine) ExecuteToolRound(...)
func (e *ContextEngine) PersistTurn(...)
func (e *ContextEngine) SessionContext(...)
// Process() → Deprecated 或删除
```

`NewContextEngine` 不再 panic on nil `QueryLLMCaller`。

---

## 5. 错误恢复（TD-QL-01~03）归属

| TD | 现状位置 | 目标位置 |
|----|---------|---------|
| TD-QL-01 413 compress | `query/loop.go` | `turn/orchestrator.go` runCompress 扩展 |
| TD-QL-02 max_output recovery | 无 | `turn/recovery.go` 新文件 |
| TD-QL-03 fallback model | `loop.FallbackLLM` | `turn/llm.go` GatewayInvoker retry |

Phase 4 实现；若工期紧可 AC8 登记 defer 并更新 `queryloop-error-recovery.md`。

---

## 6. 测试策略

| T ID | 描述 | 文件 |
|------|------|------|
| D7-S2-T20 | SubTurn scope=sub 等价原 SubQuery 单测场景 | `turn/subturn_test.go` |
| D7-S2-T21 | Wave worker SubTurn 取消传播 | `wave/runners/subagent_test.go` |
| D7-S2-T22 | Background SubTurn 完成通知 | `enforce/background_test.go` |
| D2-S15-T99 | D2 无 Loop.Run 静态断言 | `query/loop_removed_test.go` |
| D7-S2-A06-T09/T10 | 已有 — 保持 PASS | path regression |

---

## 7. 回滚

Phase 3 删除前保留 git tag `pre-queryloop-dismantle`。若 Phase 2 回归，可 revert Phase 2 PR 而不动 Phase 1 API。

---

## 8. 文档同步

| 文件 | 变更 |
|------|------|
| `d2-domain.md` | S16 → REMOVED；North Star 去掉 Legacy freeze 脚注 |
| `d7-boundary.md` | 删除 Legacy freeze 行；SubTurn 矩阵 |
| `queryloop-location.md` | Status → CLOSED，指向本 change |
| `d2-context-engine/spec.md` | D2-S10/D2-S16 LEGACY → REMOVED |
