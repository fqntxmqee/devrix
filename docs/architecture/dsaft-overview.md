# DSAFT 架构总览

**Capability:** architecture-layering（可读版）
**Version:** 1.1.0
**Canonical:** `openspec/specs/architecture/layering.md` v3.1.0 + `openspec/specs/d7-orchestration/spec.md` v3.8.0
**Last Updated:** 2026-06-19

> **v1.1.0 重大变更：** 域架构从 6 域 + ORCH 升级为 **7 域架构**（D7 升格为独立核心域，2026-06-15 闭环，PR #30+#35+#36）。D7 替代原 ORCH 读模型角色，作为**横向协调层**编排 D2（执行原语）与 D4（委托原语），并向 D1 发布进度事件。

---

## 1. 什么是 DSAFT

Devrix 使用 **DSAFT** 五层编号描述业务架构，从稳定到易变：

| 层 | 名称 | 编号格式 | 稳定性 | 含义 |
|----|------|----------|--------|------|
| **D** | Domain 领域 | D1–D7 | 极高 | 顶层限界上下文，对应 `internal/layers/{domain}/` |
| **S** | Scenario 场景 | D{N}-S{M} | 高 | 域内模块/二级包 |
| **A** | Activity 活动 | D{N}-S{M}-A{XX} | 中 | 可发起的业务动作（输入→输出→状态变更） |
| **F** | Function Point 功能点 | …-F{NN} | 低 | A 编排的最小可测逻辑单元 |
| **T** | Test Point 测试点 | …-T{NN} | 最高 | 确定性验收锚点，注册于 `openspec/t-registry.md` |

**注册表规模（2026-06-19）：** 90+ Activities · 110+ Function Points · 150+ Test Points（按 v3.8.0 推算）

---

## 2. 七域架构（D1–D7）

> 2026-06-15 起 D7 升格为**独立核心域**（PR #30 + PR #36 v1.0 closure）。原 ORCH 读模型由 D7-S4 ExecutionFlowHub 承接；D7-S3 WaveScheduler 替代原 wave 独立包。

```
                    ┌─────────────────────────────────────┐
                    │  D1 Communication (COMM)            │
                    │  Gateway · Adapters · EventBus …    │
                    └──────────────┬──────────────────────┘
                                   │ InboundMessage / EngineEvent
                    ┌──────────────▼──────────────────────┐
                    │  D7 Orchestration (D7) [CORE]       │  ← 主入口（v1.0+）
                    │  coordinator.Entry.ProcessMessage   │
                    │  S1 WorkModel · S2 SessionOrchestr. │
                    │  S3 Wave · S4 Flow · S5 Plan/Decomp │
                    └──┬────────┬────────┬────────┬───────┘
                       │        │        │        │
            ┌──────────▼──┐ ┌───▼────┐ ┌─▼──────┐ ┌▼───────────┐
            │ D2 CTX      │ │D3 LLM  │ │D4 MA   │ │ D1 GW (回) │
            │ Context     │ │Gateway │ │Multi-  │ │            │
            │ Engine      │ │Adapter │ │Agent   │ │            │
            │ (Follower)  │ │        │ │(Follow)│ │            │
            └─────────────┘ └────────┘ └────────┘ └────────────┘

     D5 Observability ──span/metric/log──▶ 全域
     D6 Evolution     ──eval/probe──────▶ D2/D4/D7 质量门禁
     D3 ILLMGateway   ──direct──▶ D7-S2-A07 LLMInvoker (D2→D3 import ban)
```

| 域 | 目录 | 类型 | 核心职责 |
|----|------|------|----------|
| **D1** | `internal/layers/communication/` | 核心 | IM 入站/出站、会话、权限 UI、EventBus；**D1 仍拥有 ingress**（d7_enabled 路由） |
| **D2** | `internal/layers/contextengine/` | 核心 | D7 Follower：`ContextPreparer` / `ToolRoundExecutor` / `SessionPersister`（DM-020 拆面契约） |
| **D3** | `internal/layers/llmgateway/` | 公共 | 模型适配、熔断、重试、Token 计数；**D7 直调**（D2→D3 import ban）|
| **D4** | `internal/layers/multiagent/` | 核心 | Agent 生命周期、Fork/Join、Delegate Worker；D7 通过 hubspoke 编排 |
| **D5** | `internal/layers/observability/` | 公共 | Tracing、Metrics、Logging、Coverage |
| **D6** | `internal/layers/evolution/` | 支撑 | EvalRun、探针、Delta、Layer lint；D6↔D7 milestone bridge |
| **D7** | `internal/layers/orchestration/` | **核心** | **主入口 + 横向协调**：S1 WorkModel / S2 SessionOrch / S3 Wave / S4 Flow / S5 Decision |

---

## 3. D7 编排层（核心域，v3.8.0）

> D7 回答 **"做什么、按什么顺序做、谁来做、做得怎么样了"**。2026-06-15 升格为核心域，2026-06-18 闭环 v2.0 unified task tools（PR #83-#87）。

### 3.1 D7 S 层 5 子层（切法 A — 按用户价值流）

| S 层 | 博弈角色 | North Star | 实现状态 | 关键包 |
|------|---------|------------|----------|--------|
| **D7-S1** | **State Authority**（非博弈）| Task/Plan 持久化与状态机；产"事实"而非"决策" | ✅ IMPLEMENTED (v1.1 closure, DM-20260614-009) | `orchestration/workmodel/` |
| **D7-S2** | **Screening Mechanism** + **Turn Leader (Stackelberg)** | 用户消息统一入口 + Turn 主循环；元层 | ✅ IMPLEMENTED (v2.0 Structure, DM-20260619-005) | `orchestration/sessionorchestrator/` + `turn/` |
| **D7-S3** | **Mechanism Designer** | 多任务并行执行，冲突避免，上下文隔离 | ✅ IMPLEMENTED (5-slot WorkerPool + ConflictGuard) | `orchestration/wavescheduler/` |
| **D7-S4** | **Costly Signaler** | 执行进度透明，WorkPlan 可追溯 | ✅ IMPLEMENTED (Flow Hub + WorkPlan + IM Sink) | `orchestration/executionflow/{hub,workplan,imsink,bridge}/` |
| **D7-S5** | **Information Producer** | 把用户 goal 转化为可执行的任务结构 | ✅ IMPLEMENTED (ClassifyIntent + LLMDecomposer + PlanMode) | `orchestration/decisionplanning/` + `workmodel/plan_*.go` |

### 3.2 D2 Follower 契约（DM-020）

D2 不再是主入口，是 D7 编排的 **Follower**：

| D7 动作 | D2 响应（拆面） | D2 Canonical S |
|---------|----------------|----------------|
| `ContextPreparer.Prepare` | 组装合法上下文 + CompressHint（若需压缩）| D2-S15 |
| `ToolRoundExecutor.ExecuteRound` | 权限门控 + 沙箱执行 | D2-S18 |
| `SessionPersister.PersistTurn` | 快照 + transcript + commit | D2-S17 |
| Wave 调度 D2 Worker | S16 Loop + S18 policy | D2-S16 |
| SubQuery / Background (fallback) | S19 NestedExecution | D2-S19 (DEPRECATED) |

> **LLM 调用权（DM-020 产权）：** D7 是唯一有权决定何时调用 D3 的域。D2 拥有"请求 LLM 结果"权利（通过 CompressHint），但不拥有"执行 LLM 调用"权利。该产权通过 `internal/lint/layer/d2_d3_ban_test.go` CI 硬阻断（4/4 白名单已满）。

### 3.3 D2 域现状（非主入口，仅作 Follower）

> D2-S1 PEV 已 **RETIRED**（2026-06-13）。生产路径为 **D2-S10 QueryLoop**（**DEPRECATED**, fallback only, DM-20260616-004）+ **D2-S15/17/18 Follower 拆面**（生产主路径，被 D7-S2 编排）。

| S ID | Scenario | 包路径 | 状态 |
|------|----------|--------|------|
| D2-S1 | PEV | — | **RETIRED** |
| D2-S2 | Compression | `compression/` | IMPLEMENTED（CompressHint 来源）|
| D2-S3 | Memory | `memory/` | IMPLEMENTED |
| D2-S4 | Token | `token/` + shared counter | IMPLEMENTED |
| D2-S5 | Registry | `registry/` | IMPLEMENTED |
| D2-S6 | Snapshot | `snapshot/` | IMPLEMENTED |
| D2-S7 | Prompt | `prompt/` | IMPLEMENTED |
| D2-S8 | Sandbox | `toolrunner/`（命令策略）| IMPLEMENTED |
| D2-S9 | Harness | `harness/` | **DEPRECATED**（legacy fallback）|
| D2-S10 | QueryLoop | `query/` | **DEPRECATED**（DM-20260616-004 落地，仅 fallback）|
| D2-S11 | Queue | `queue/` | IMPLEMENTED |
| D2-S12 | Worktree | `worktree/` | IMPLEMENTED（per-handle wt path, v2.0 unified）|
| D2-S13 | Conversation | `conversation/` | IMPLEMENTED |
| D2-S14 | Mock | `mock/` | 测试辅助 |
| **D2-S15** | **Context Preparer** | `prepare/` | **IMPLEMENTED (Follower)** |
| **D2-S16** | **Loop Follower** | `query/loop.go` (239 行瘦身) | **IMPLEMENTED (Follower)** |
| **D2-S17** | **Session Persister** | `persist/` | **IMPLEMENTED (Follower)** |
| **D2-S18** | **Tool Round Executor** | `enforce/` | **IMPLEMENTED (Follower)** |
| D2-S19 | NestedExecution | `query/subquery.go` | **DEPRECATED**（legacy fallback）|

---

## 4. 主入口路径（D7 v2.0）

> 旧 QueryLoop 时代（2026-06-13）的 `D1→D2.runProcess→QueryLoop.Run` 已**退役**。当前主入口：`**D1→D7-S2 ProcessMessage→4 IntentKind→4 真实执行链**`。

| 配置 | 行为 |
|------|------|
| `orchestration.enabled: true`（**默认**）| `D1→D7-S2 coordinator.Entry.ProcessMessage`；按 IntentKind dispatch 到 Skip/Command/Fast/Orchestrate |
| `orchestration.enabled: false` | 回退 `D1→D2.QueryLoop.Run`（**DEPRECATED** fallback，D6 PathRegressionProbe 监控）|

**4 IntentKind 真实链（v1.1.0+ 正交分发，DM-20260615-004）：**

| IntentKind | 触发场景 | 执行链 | 是否调 LLM |
|------------|----------|--------|------------|
| `IntentSkip` | 寒暄/无效输入 | `close channel` | ❌ 零 LLM 成本 |
| `IntentCommand` | `/plan` / `/task` / `/stop` / `/help` | `CommandHandler` → `workmodel.CLICommands.Handle` | ❌ 零 LLM 成本 |
| `IntentFast` | 普通 LLM 对话 | `FastPath.Run` → `turn.RunTurn` resolve/decompose | ✅ D7 直调 D3 |
| `IntentOrchestrate` | 多步/可拆解任务 | `OrchestratePath.Run` → `LLMDecomposer` + `WaveScheduler` | 部分（D7-S5 LLM Decomposer）|

**v2.0 unified task tools（PR #83-#87）：** `task_write` / `task_spawn` / `task_await` 统一 alias + `WorkItem`/`WorkTree` 统一抽象 + `RunRegistry` + `FocusHint` + `ResolveAwaiter` blocking。

详见 [`request-flow.md`](./request-flow.md) v1.2.0 + `openspec/specs/d7-orchestration/spec.md` v3.8.0。

---

## 5. 与其他文档的关系

| 需求 | 去哪里 |
|------|--------|
| D7 SoT（权威）| `openspec/specs/d7-orchestration/spec.md` v3.8.0 |
| D7 S 层博弈角色 | `openspec/specs/d7-orchestration/d7-domain.md` v1.1.0 |
| D2 Follower 契约 | `openspec/specs/d2-context-engine/d7-boundary.md` |
| 改代码放哪 | [`code-map.md`](./code-map.md) |
| **D-S Index 索引** | [`openspec/specs/architecture/code-atlas.md` v1.2.0](../../openspec/specs/architecture/code-atlas.md) |
| **端到端链路图（D7 v2.0）** | [`request-flow.md`](./request-flow.md) v1.2.0 |
| 跨层接口在哪定义 | [`contracts-and-boundaries.md`](./contracts-and-boundaries.md) |
| DSAFT 方法论（本文件） | `docs/methodology/dsaft-methodology.md` |
| 新增 T 测试点 | `openspec/t-registry.md` |
| 新增 Activity | `openspec/a-registry.md` |
| CI 层违规检测 | `internal/lint/layer/` + D6 LayerViolationProbe |

---

## 6. Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初版（6 域 + ORCH 读模型，QueryLoop 视角）|
| **1.1.0** | **2026-06-19** | **升级 7 域架构**：D7 升格独立核心域；D-S Index 切到 D7 v2.0 unified；D7-S 层 5 子层 IMPLEMENTED 状态表；D2 Follower 契约章节；4 IntentKind 真实链；QueryLoop/SubQuery/Harness 标 DEPRECATED；引用 code-atlas v1.2.0 + request-flow v1.2.0 + d7 spec v3.8.0（DM-20260619-001, docs-only）|
