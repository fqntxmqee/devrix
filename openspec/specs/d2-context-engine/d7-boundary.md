# D2 ↔ D7 跨域边界规范

**Capability:** d2-d7-boundary  
**Status:** Active  
**Version:** 2.0.0  
**Last Updated:** 2026-06-19  
**Change ID:** devrix-d2-sa-refine（DM-009）+ devrix-d7-turn-orchestration（DM-020）+ devrix-d2-queryloop-dismantle（DM-20260618-010）  
**Demand ID:** DM-20260614-009 / DM-20260614-020 / DM-20260618-010  
**Parent (D2):** `openspec/specs/d2-context-engine/d2-domain.md`  
**Parent (D7):** `openspec/specs/d7-orchestration/d7-domain.md`

---

## 1. 关系摘要

| 域 | 角色 | 一句话 |
|----|------|--------|
| **D7** | Orchestration Mediator / **Leader** | 决定做什么、顺序、谁做、进度如何广播；**拥有 Turn 主循环与 LLM 调用权** |
| **D2** | Execution Follower | 在给定参数下完成 Prepare → ToolRound → Persist |

**ingress：** D1 → D7 `ProcessMessage` only（DM-20260614-007）。D2 **不被** D1 直接调用。

---

## 2. 调用链 SoT（v8.0.0）

```text
D1.Gateway.RouteInbound
    └── D7.IOrchestrationEntry.ProcessMessage
            ├── [S2/S5] 路由 / 分类 / TaskGraph
            ├── [S2-A06] RunTurn（D7 Turn Leader）
            │       ├── D2.Prepare（D2-S15）
            │       ├── D7.InvokeLLM → D3.StreamChat（D7 直调 D3）
            │       ├── D2.ExecuteToolRound（D2-S18）
            │       └── D2.PersistTurn（D2-S17）
            ├── [FastPath] TurnExecutor.RunTurn → turn.DefaultOrchestrator
            ├── [S3] WaveScheduler → Worker(D2|D4)
            ├── [S4] ExecutionFlowHub → D1 IM
            └── D2.IEngine.Process（PreparedTurnRunner 委托 D7）
                    ├── D2-S15 PrepareExecutionContext
                    ├── D7 RunTurn / SubTurn（经 PreparedTurnRunner）
                    └── D2-S17 PersistSessionState
```

**实现：** `internal/bootstrap/wire_coordinator.go`

> **DM-20260618-010：** `query/loop.go`、`QueryLLMCaller`、`d2Executor.RunQueryLoop` 已删除。FastPath 经 `TurnExecutor`（`turnOrchExecutor`）调用 D7 `RunTurn`。

---

## 3. 职责矩阵

| 能力 | D7 | D2 | D1 | D4 | D6 |
|------|----|----|----|----|-----|
| 消息 ingress | 接收（自 D1） | — | ✅ SoT | — | — |
| ClassifyIntent / Command-first | ✅ S5 | ❌ | — | — | — |
| Wave / DAG 调度 | ✅ S3 | ❌ | — | — | — |
| FlowEvent / WorkPlan | ✅ S4 | ❌ | 展示 | 产出事件 | — |
| Task 写模型 | ✅ S1 `workmodel/` | — | — | — | — |
| PlanMode / PlanAgent | ✅ S1 `workmodel/plan_*.go` | — | — | — | — |
| delegate_* 路由 | ✅ F（目标） | 🔶 暂存 | — | ✅ 执行 | — |
| Session 上下文 / 压缩 | ❌ | ✅ S15/S17 | — | — | — |
| Turn 主循环 LLM↔Tool | ✅ S2-A06 RunTurn | ❌（**REMOVED S16**） | — | — | — |
| LLM 调用（StreamChat） | ✅ S2-A07 InvokeLLM → D3 | ❌（禁止 D2→D3） | — | — | — |
| Permission / Sandbox | 策略下发 | ✅ S18 机制 | — | — | — |
| SubQuery / Background | 触发 | ✅ S19 机制 | — | — | — |
| EngineEvent | 转发 | ✅ 产出 | ✅ 展示 | — | — |
| 结论质量 Judge | ❌ | ❌ | — | — | ✅ |

图例：✅ SoT · 🔶 代码暂存、规格已迁出 · ❌ Out of Scope

---

## 4. 契约接口

| 接口 | 定义位置 | 实现 | 消费 | 状态 |
|------|----------|------|------|------|
| `IOrchestrationEntry` | `shared/contracts` | D7 `sessionorchestrator.Entry`（`coordinator.Entry` shim） | D1 Gateway | ACTIVE |
| `TurnExecutor` | `sessionorchestrator` | `bootstrap.turnOrchExecutor` → `turn.TurnOrchestrator` | D7 FastPath | ACTIVE |
| `PreparedTurnRunner` | `shared/contracts` | `turn.PreparedTurnAdapter` | D2 `engine.Process` | ACTIVE |
| `IEngine` | `shared/contracts` | `contextengine.Engine` | D7 via adapter | ACTIVE |
| `Loop.Run` | ~~`query/loop.go`~~ | — | — | **REMOVED** (DM-20260618-010) |
| `QueryLoopExecutor` | ~~`coordinator`~~ | — | — | **REMOVED** → `TurnExecutor` |
| `ExecutionFlowHub` | `shared/contracts` | D7 `executionflow/hub` | D2/D4 发布 | ACTIVE |

### 4.1 依赖规则（DM-020 + DM-20260618-010）

```text
✅ D7 → D2（ContextPreparer / ToolRoundExecutor / SessionPersister）
✅ D7 → D3（ILLMGateway via bridges/llm）— D7 拥有 LLM 调用权
✅ D2 → shared/contracts, toolrunner…
❌ D2 → D3（禁止：llmgateway, bridges/llm — DM-020 v1.0 Registry，v2.0-d CI 硬阻断）
❌ D2 → orchestration（禁止）
❌ D2 → communication/adapters（禁止）
```

---

## 5. Canonical S 对照

| D7 Canonical S | 与 D2 关系 | D2 Canonical |
|----------------|-----------|----------------|
| D7-S1 Work Model | 调用 D2 task tools（v2.0 代码归 D7） | Legacy S10 task F |
| D7-S2 Session Orchestrator | 拆面调用 D2 + 自持 RunTurn | S15–S17 |
| D7-S3 Wave Scheduler | 调度 D2 Worker | S18 per worker |
| D7-S4 Execution Flow | 聚合 D2 SubQuery FlowEvent | S19 产出；S11→D7 |
| D7-S5 Decision & Planning | 下发 permission/plan 参数 | S18 执行约束 |

| D2 Canonical S | 与 D7 关系 | 状态 |
|----------------|-----------|------|
| D2-S15 | 每次 Process / Turn 前；D7 不替代 | ACTIVE |
| D2-S16 | ~~RunQueryLoop~~ | **REMOVED → D7-S2-A06** |
| D2-S17 | complete 前持久化；与 D7-S4 进度正交 | ACTIVE |
| D2-S18 | 接收 D7 下发的 mode/tools | ACTIVE |
| D2-S19 | 嵌套执行；Flow 归 D7-S4 | ACTIVE |
| D2-S20 | Legacy Harness | **REMOVED v6.5.0** |

---

## 6. 跨域漂移清单（v2.0 迁移）

| # | 路径 | 行为 | 目标 | Phase |
|---|------|------|------|-------|
| 1 | ~~`contextengine/tasks/`~~ | Task CRUD | D7-S1 `workmodel/` | ✅ DM-012 |
| 2 | ~~`contextengine/tasks/plan_*.go`~~ | Plan 策略 | D7-S5 `workmodel/plan_*.go` | ✅ DM-012 |
| 3 | ~~`contextengine/delegate_tools.go`~~ | D4 路由 | `orchestration/delegatetools/` | ✅ DM-011 |
| 4 | ~~`contextengine/queue/`~~ delegate-progress | Flow drain | D7-S4 `sessionqueue/` | ✅ DM-013 |
| 5 | ~~`contextengine/worker_tools.go`~~ | Worker 编排面 | D7 `toolpolicy/` | ✅ DM-015 |
| 6 | `contextengine/nested/flow_report.go` | SubQuery FlowEvent 发布 | D7-S4 `executionflow/bridge/subquery_bridge.go` | ⬜ DM-018 slice-c |
| 7 | ~~`contextengine/query/loop.go`~~ | LLM↔Tool loop | D7 `turn/orchestrator.go` | ✅ DM-20260618-010 |

---

## 7. Requirements

### Requirement: D7-Only Ingress to D2

D2 `IEngine.Process` MUST be invoked by D7 (via `PreparedTurnRunner` / turn adapters) or test/bootstrap harness. D1 MUST NOT call `D2.Process` directly.

#### Scenario: Gateway routes to D7 not D2

- GIVEN `d7.enabled=true` (mandatory)
- WHEN D1 `RouteInbound` handles user message
- THEN `IOrchestrationEntry.ProcessMessage` is called
- AND D2.Process is NOT called from D1 capture layer

### Requirement: D2 Thin Execution Primitive

D2 `query` package MUST NOT import D4 multi-agent orchestration packages. Orchestration routing MUST remain in D7.

#### Scenario: Static import boundary

- GIVEN `internal/layers/contextengine/` sources (excluding `query/types.go` legacy types)
- WHEN package import graph is analyzed
- THEN `multiagent` and `orchestration` packages are not imported
- AND regression test `internal/lint/layer/d2_thin_test.go` passes (DM-20260614-010)

#### Scenario: D1 capture does not import D2 directly

- GIVEN `internal/layers/communication/capture/` sources
- WHEN package import graph is analyzed
- THEN `contextengine` package is not imported
- AND regression test `internal/lint/layer/d7_boundary_test.go` passes

### Requirement: FlowEvent Ownership

Unified `FlowEvent` aggregation and delegate-progress drain Canonical ownership MUST be D7-S4. D2 MAY publish FlowEvent but MUST NOT own Hub aggregation SoT.

#### Scenario: Hub interface implemented in D7

- GIVEN execution flow enabled
- WHEN FlowEvent is aggregated to WorkPlan
- THEN `ExecutionFlowHub` implementation lives under `orchestration/`
- AND D2 SubQuery only publishes events

---

## 8. 相关文档

| 文档 | 用途 |
|------|------|
| `archive/2026-06-18-devrix-d2-queryloop-dismantle/` | QueryLoop 物理删除变更包 |
| `archive/2026-06-14-devrix-d2-sa-refine/gaming-analysis.md` | 博弈推导 |
| DM-20260614-008 | D7 Leader 规格 |
| DM-20260614-007 | D1→D7 ingress |
| **DM-20260614-020** | **D7 Turn 编排上移（D2→D3 禁止）** |
| **DM-20260618-010** | **QueryLoop dismantle（S16 REMOVED）** |
| `openspec/specs/d7-orchestration/d3-boundary.md` | **D7↔D3 边界 SoT** |
