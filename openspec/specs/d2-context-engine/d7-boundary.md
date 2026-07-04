# D2 ↔ D7 跨域边界规范

**Capability:** d2-d7-boundary  
**Status:** Active  
**Version:** 2.1.0  
**Last Updated:** 2026-07-04  
**Change ID:** devrix-d2-sa-refine（DM-009）+ devrix-d7-turn-orchestration（DM-020）+ devrix-d2-queryloop-dismantle（DM-20260618-010）+ devrix-d7-mups-v4-phase3-execute（DM-20260625-001，PR-C1 跨域类型上提 shared/types）+ **mups-d2-context-tools-ownership（DM-20260704-001）**  
**Demand ID:** DM-20260614-009 / DM-20260614-020 / DM-20260618-010 / DM-20260625-001  
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
| 4 | ~~`contextengine/queue/`~~ delegate-progress | Flow drain | D7-S4 `executionflow/` (formerly `sessionqueue/`) | ✅ DM-013 + DM-20260625-018 PR-3b |
| 5 | ~~`contextengine/worker_tools.go`~~ | Worker 编排面 | D7 `decisionplanning/` (filter_adapter) | ✅ DM-015 |
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

D2 `contextengine` production packages MUST NOT import D4 multi-agent or D7 orchestration packages. Orchestration routing MUST remain in D7.

#### Scenario: Static import boundary

- GIVEN `internal/layers/contextengine/` production sources
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
| **DM-20260625-001** | **MUPS Phase 3 PR-C1（跨域类型上提 shared/types.Artifact）** |
| `openspec/specs/d7-orchestration/d3-boundary.md` | **D7↔D3 边界 SoT** |

---

## 9. MUPS 5 节点管道 × D2 跨域边界（2026-06-25 落地）

> MUPS v4.3 5 节点管道（Observe / Plan / Execute / Verify / Learn）落地后，D2 与 D7 的跨域边界新增 1 个共享类型 + 4 类 Artifact 的 D2 消费路径。

### 9.1 共享类型上提（PR-C1）

`Artifact` 类型从 D7 Execute 节点独占升格为**跨域共享类型**，由 D2/D4/D7 共同消费：

```go
// internal/shared/types/artifact.go
type Artifact struct {
    ID             string
    Kind           ArtifactKind  // state_change / response / probe / experiment
    Payload        []byte
    Evidence       Evidence
    SourcePlanID   string        // 反向追溯 Plan
}

// 4 类 Artifact
const (
    ArtifactStateChange ArtifactKind = "state_change"
    ArtifactResponse    ArtifactKind = "response"
    ArtifactProbe       ArtifactKind = "probe"
    ArtifactExperiment  ArtifactKind = "experiment"
)
```

**D2 消费路径：**

| Artifact Kind | D2 消费者 | 用途 |
|--------------|----------|------|
| `state_change` | D2-S17 PersistTurn | 状态变更落盘 |
| `response` | D2-S16 Loop | 直接返回用户 |
| `probe` | D2-S19 SubQuery | 试探结果合并 |
| `experiment` | D2-S19 SubQuery | 实验数据汇总 |

### 9.2 D2 → D7 Verify 节点接口

D7-S10 Verify 节点（`verify/verifier.go::Verify`）是 D2 投递 Artifact 的目标。**D2 → D7 Verify 的调用模式**与 D7 → D2 拆面契约对称：

```
D2 SubQuery / D4 Worker
    └── 投递 Artifact 到 D7 Verify (S10-A32)
            ├── VerifyWithRetry (3 次重试)
            ├── ExtractEvidence (与 Plan.FailureCriteria 对齐)
            ├── VerdictKind 提取（compliance/timeliness/root_cause/statistical）
            └── 返回 Verdict + ExitReason (14 态之一)
```

### 9.3 D2 ↔ MUPS Plan 节点

MUPS Plan 节点（D7-S8 PR-B1）的输入是 `UncertaintyReport`，D2 不直接产出 UncertaintyReport；但 D2-S15 PrepareContext 输出的 context snapshot 是 Observe 节点的输入信号之一（Observation.Kind=ObsFact）。

### 9.4 D2 ↔ MUPS Learn 节点

MUPS Learn 节点（D7-S11）通过 `Memory.Persist` 3 通道（skill / feedback / scheduled）落 LearningAsset。**D2 不参与** Learn 节点，但 D2-S17 PersistTurn 输出的 transcript 是 Learn 节点的输入信号之一（追溯链 Verification）。

### 9.5 边界硬约束

| 约束 | 含义 |
|------|------|
| D2→D7 import | **允许**（D7-S10 Verify 消费 D2 投递的 Artifact） |
| D7→D2 import | **允许**（拆面调用 D2-S15/S17/S18） |
| D2→D3 import | **白名单 4/4 已满，禁止新增**（DM-020 + DM-20260618-010） |
| D7→D3 import | **允许**（D7-S2-A07 InvokeLLM 直调 D3） |
| shared/types/Artifact import | **所有域允许**（PR-C1 跨域共享类型） |

详见 `openspec/specs/d7-orchestration/d7-domain.md` §MUPS 5 节点管道 与 `design.md` §⑦ 跨域类型上提。

---

## 10. MUPS 上下文与工具决策 — D2 统一负责（DM-20260704-001）

> **Change:** `mups-d2-context-tools-ownership` — D2 拥有 MUPS LLM 节点的 context + tools 决策；D7 只传 phase 参数并调 D3。

### 10.1 职责 reaffirmation

| 能力 | D7 (Leader) | D2 (Follower) |
|------|-------------|---------------|
| MUPS 节点顺序 / WorkItem 状态机 | ✅ | ❌ |
| InvokeLLM → D3 | ✅ | ❌（禁止 D2→D3） |
| `MaterializeForMUPS` 调用 | 发起（传 `MUPSContextRequest`） | 实现（返回 `MUPSPreparedContext`） |
| System prompt 组装（MUPS 节点） | ❌ | ✅（PromptAssembler + phase registry） |
| Tool filter pipeline（7 步） | ❌ | ✅（含 Filter v2） |
| ExecuteToolRound + ToolChannel | 传 TaskKind + budget | ✅（`enforce/toolround/`） |
| PlanChannel（per-PlanKind 策略） | ✅ | ❌ |
| Verify / Learn / Decide | ✅ Go-only | ❌ 不调用 Materialize |

### 10.2 契约接口

```go
// shared/contracts/mups_context.go
type IMUPSContextMaterializer interface {
    MaterializeForMUPS(ctx context.Context, req MUPSContextRequest) (MUPSPreparedContext, error)
}
```

**挂载：** `bootstrap/turn_adapter.go` → `contextEngineAdapter`；D7 `sessionorchestrator` 通过 `MUPS` 字段注入。

### 10.3 D7 → D2 调用矩阵

| MUPS 节点 | D2 调用 | LLM | Tools |
|-----------|---------|-----|-------|
| Observe (LLM) | `MaterializeForMUPS(phase=observe)` | D7→D3 | 空 |
| Plan (LLM) | `MaterializeForMUPS(phase=plan)` | D7→D3 | 空 |
| Execute | `MaterializeForMUPS(phase=execute)` 每轮 | D7→D3 | 已过滤 |
| Verify / Learn / Decide | **不调用** | — | — |

### 10.4 Tool Filter Pipeline（D2 专属，有序）

```text
ListTools → Permission → AgentRole → PerEmissionClass → PerTaskKind → ToolProfile → i18n descriptors
```

实现：`internal/layers/contextengine/materialize/filter_pipeline.go`

### 10.5 边界硬约束（新增）

| 约束 | 含义 |
|------|------|
| D7 **MUST NOT** import `contextengine/enforce/tools/filter` | CI: `d7_no_tool_filter_test.go` |
| D7 **MUST NOT** 硬编码 tool name 列表用于 MUPS 过滤 | 同上 |
| D7 **MUST NOT** 在 proposer/executor 组装 phase appendix | Observe/Plan appendix 在 D2 `phase_prompts.go` |
| D2 **MUST NOT** import D7 orchestration 包 | 现有 `d2_thin_test.go` |
| D2 **MUST NOT** 调用 D3 | DM-020 不变 |

### 10.6 相关文档

| 文档 | 用途 |
|------|------|
| `openspec/changes/mups-d2-context-tools-ownership/design.md` | 完整架构设计 |
| `openspec/changes/mups-d2-context-tools-ownership/acceptance-report.md` | S5 验收报告 |
| `openspec/specs/d2-context-engine/t-registry.md` §D2-S15-A90 | D2 T 点 |
| `openspec/specs/d7-orchestration/t-registry.md` §D7-S2-A90 | D7 T 点 |
