# D2 ↔ D7 跨域边界规范

**Capability:** d2-d7-boundary  
**Status:** Active  
**Version:** 1.0.0  
**Last Updated:** 2026-06-14  
**Change ID:** devrix-d2-sa-refine  
**Demand ID:** DM-20260614-009  
**Parent (D2):** `openspec/specs/d2-context-engine/d2-domain.md`  
**Parent (D7):** `openspec/specs/d7-orchestration/d7-domain.md`

---

## 1. 关系摘要

| 域 | 角色 | 一句话 |
|----|------|--------|
| **D7** | Orchestration Mediator / **Leader** | 决定做什么、顺序、谁做、进度如何广播 |
| **D2** | Execution Follower | 在给定参数下完成 Prepare → QueryLoop → Persist |

**ingress：** D1 → D7 `ProcessMessage` only（DM-20260614-007）。D2 **不被** D1 直接调用。

---

## 2. 调用链 SoT

```text
D1.Gateway.RouteInbound
    └── D7.IOrchestrationEntry.ProcessMessage
            ├── [S2/S5] 路由 / 分类 / TaskGraph
            ├── [S3] WaveScheduler → Worker(D2|D4)
            ├── [S4] ExecutionFlowHub → D1 IM
            └── d2Executor.RunQueryLoop (bootstrap)
                    └── D2.IEngine.Process
                            ├── D2-S15 PrepareExecutionContext
                            ├── D2-S16 RunQueryLoop
                            └── D2-S17 PersistSessionState
```

**实现：** `internal/bootstrap/wire_coordinator.go`

---

## 3. 职责矩阵

| 能力 | D7 | D2 | D1 | D4 | D6 |
|------|----|----|----|----|-----|
| 消息 ingress | 接收（自 D1） | — | ✅ SoT | — | — |
| ClassifyIntent / Command-first | ✅ S5 | ❌ | — | — | — |
| Wave / DAG 调度 | ✅ S3 | ❌ | — | — | — |
| FlowEvent / WorkPlan | ✅ S4 | ❌ | 展示 | 产出事件 | — |
| Task 写模型 | ✅ S1（目标） | 🔶 暂托管 | — | — | — |
| PlanMode / PlanAgent | ✅ S5（目标） | 🔶 暂托管 | — | — | — |
| delegate_* 路由 | ✅ F（目标） | 🔶 暂存 | — | ✅ 执行 | — |
| Session 上下文 / 压缩 | ❌ | ✅ S15/S17 | — | — | — |
| QueryLoop LLM↔Tool | 编排调用 | ✅ S16 | — | — | — |
| Permission / Sandbox | 策略下发 | ✅ S18 机制 | — | — | — |
| SubQuery / Background | 触发 | ✅ S19 | — | — | — |
| EngineEvent | 转发 | ✅ 产出 | ✅ 展示 | — | — |
| 结论质量 Judge | ❌ | ❌ | — | — | ✅ |

图例：✅ SoT · 🔶 代码暂存、规格已迁出 · ❌ Out of Scope

---

## 4. 契约接口

| 接口 | 定义位置 | 实现 | 消费 |
|------|----------|------|------|
| `IOrchestrationEntry` | `shared/contracts` | D7 `coordinator.Entry` | D1 Gateway |
| `QueryLoopExecutor` | `coordinator` | `bootstrap.d2Executor` | D7 Orchestrator |
| `IEngine` | `shared/contracts` | `contextengine.Engine` | D7 via adapter |
| `LoopHooks` | `query/loop.go` | D7 注入 | D2 Loop |
| `ExecutionFlowHub` | `shared/contracts` | D7 flow | D2/D4 发布 |

### 4.1 依赖规则

```text
✅ D7 → D2（IEngine / QueryLoopExecutor）
✅ D2 → D3, shared/contracts, toolrunner…
❌ D2 → orchestration（禁止）
❌ D2 → communication/adapters（禁止）
```

---

## 5. Canonical S 对照

| D7 Canonical S | 与 D2 关系 | D2 Canonical |
|----------------|-----------|----------------|
| D7-S1 Work Model | 调用 D2 task tools（v2.0 代码归 D7） | Legacy S10 task F |
| D7-S2 Session Orchestrator | 调用 `d2Executor` | S15–S17 整链 |
| D7-S3 Wave Scheduler | 调度 D2 Worker Loop | S16 per worker |
| D7-S4 Execution Flow | 聚合 D2 SubQuery FlowEvent | S19 产出；S11→D7 |
| D7-S5 Decision & Planning | 下发 permission/plan 参数 | S18 执行约束 |

| D2 Canonical S | 与 D7 关系 |
|----------------|-----------|
| D2-S15 | 每次 Process 前；D7 不替代 |
| D2-S16 | D7 主要 Follower 调用点 |
| D2-S17 | complete 前持久化；与 D7-S4 进度正交 |
| D2-S18 | 接收 D7 下发的 mode/tools |
| D2-S19 | 嵌套执行；Flow 归 D7-S4 |
| D2-S20 | 不经 D7 的 legacy 配置路径 |

---

## 6. 跨域漂移清单（v2.0 迁移）

| # | 路径 | 行为 | 目标 | Phase |
|---|------|------|------|-------|
| 1 | ~~`contextengine/tasks/`~~ | Task CRUD | D7-S1 `workmodel/` | ✅ DM-012 |
| 2 | ~~`contextengine/tasks/plan_*.go`~~ | Plan 策略 | D7-S5 `workmodel/plan_*.go` | ✅ DM-012 |
| 3 | ~~`contextengine/delegate_tools.go`~~ | D4 路由 | `orchestration/delegatetools/` | ✅ DM-011 |
| 4 | ~~`contextengine/queue/`~~ delegate-progress | Flow drain | D7-S4 `sessionqueue/` | ✅ DM-013 |
| 5 | ~~`contextengine/worker_tools.go`~~ | Worker 编排面 | D7 `toolpolicy/` | ✅ DM-015 |
| 6 | `contextengine/nested/flow_report.go` | SubQuery FlowEvent 发布 | D7-S4 `hubspoke/subquery_bridge.go` | ⬜ DM-018 slice-c |

v1.0：**仅登记**，不移动代码。  
v2.0 slice-1（DM-011）：`delegate_tools` **已迁移**。  
v2.0 slice-2（DM-012）：`contextengine/tasks/` → `orchestration/workmodel/` **已迁移**。  
v2.0 slice-3（DM-013）：`contextengine/queue/` → `orchestration/sessionqueue/` **已迁移**。  
v2.0 slice-4（DM-015）：`worker_tools.go` → `orchestration/toolpolicy/` **已迁移**。  
v2.0 slice-5（DM-014）：D2 物理目录 `prepare/` `persist/` `policy/` `nested/` **已收敛**。  
v2.0 slice-c（DM-018）：`nested/flow_report.go` SubQuery Flow 发布 **待迁** D7 `hubspoke/subquery_bridge.go`（D2-S19 仅保留嵌套 QueryLoop 执行机制）。

---

## 7. Requirements

### Requirement: D7-Only Ingress to D2

D2 `IEngine.Process` MUST be invoked by D7 (via `QueryLoopExecutor`) or test/bootstrap harness. D1 MUST NOT call `D2.Process` directly.

#### Scenario: Gateway routes to D7 not D2

- GIVEN `d7.enabled=true` (mandatory)
- WHEN D1 `RouteInbound` handles user message
- THEN `IOrchestrationEntry.ProcessMessage` is called
- AND D2.Process is NOT called from D1 capture layer

### Requirement: D2 Thin Execution Primitive

D2 `query` package MUST NOT import D4 multi-agent orchestration packages. Orchestration routing MUST remain in D7.

#### Scenario: Static import boundary

- GIVEN `internal/layers/contextengine/query/` sources
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
| `archive/2026-06-14-devrix-d2-sa-refine/gaming-analysis.md` | 博弈推导 |
| `archive/2026-06-14-devrix-d2-sa-refine/design.md` §12 | 设计 Decision |
| `openspec/archive/2026-06-14-devrix-d2-sa-refine/` | Change 包（S7 已归档） |
| DM-20260614-008 | D7 Leader 规格 |
| DM-20260614-007 | D1→D7 ingress |
