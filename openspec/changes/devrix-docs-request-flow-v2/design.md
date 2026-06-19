# Design: 链路图文档对齐 D7 v2.0

**Change ID:** devrix-docs-request-flow-v2
**Demand ID:** DM-20260619-001

> docs-only change，3 个文档的逐文件变更映射如下。SoT 不动：`openspec/specs/d7-orchestration/spec.md` v3.8.0。

---

## 1. 设计原则

1. **单向对齐**：`spec.md` v3.8.0 是 SoT；`docs/` 单向对齐 spec，docs 与 spec 矛盾时以 spec 为准
2. **保留架构骨架**：request-flow.md 沿用原 9 节框架（时序图 → Gateway → Process → 内部循环 → LLM → 工具 → Delegate → 配置 → 进一步阅读），只更新内容
3. **DEPRECATED 显式标注**：D2 QueryLoop legacy 路径、`subquery`、`harness/` 等已退役模块在文档中显式标 **DEPRECATED (legacy, fallback only)**，不删除（避免回归断引）
4. **代码锚点 grep 验证**：所有引用的 `xxx.go:Function` 必须能 `git grep` 命中

## 2. 文件级变更映射

### 2.1 W1: `docs/architecture/request-flow.md`

| 旧段落 | 旧内容要点 | 新内容 |
|--------|----------|--------|
| Header 元信息 | "Last Updated: 2026-06-13" + 代码锚点 gateway→engine:runProcess→query/loop:Run→llmgateway | "Last Updated: 2026-06-19" + 锚点 gateway→coordinator/orchestrator:ProcessMessage→turn/orchestrator:RunTurn→bridges/llm + wave/scheduler |
| §1 总览时序 | D1→D2.QueryLoop→D3 LLM | D1→D7-S2.ProcessMessage→4 IntentPath→D3/D2/D4 |
| §2 Gateway 阶段 | 5 步 Gateway 路由 | 5 步 Gateway + `d7_enabled` 路由开关（d7_enabled=true 时 D1→D7；d7_enabled=false 时回退 D1→D2 legacy）|
| §3 Process 管线 | D2 runProcess 7 步骤表 | 替换为 D7-S2 ProcessMessage + ClassifyIntent（规则 → LLM fallback）+ 4 IntentKind dispatch |
| §4 内部循环 | QueryLoop.Run while 循环 | 替换为 `turn.RunTurn` resolve/decompose 循环（v2.0 新增：FocusHint + depth limits + daily limits + ResolveAwaiter blocking）|
| §5 LLM 调用 | D2→D3 query/adapters.go | D7 直调 D3（`coordinator/turn/llm.go::GatewayInvoker.InvokeStream` via `bridges/llm`），D2→D3 import ban（CI 硬阻断）|
| §6 工具与权限 | ToolRunner → sandbox | 保持（仍是 D2 Follower：`ToolRoundExecutor.ExecuteRound`）|
| §7 Delegate / Worker | Hub-Spoke | 保持 + 加 v2.0 unified（`workitem` + `RunRegistry`） |
| §8 配置键 | query_loop.* / harness.* | 加 `d7.enabled` / `workmodel.*` / `wave.*` / `turn.focus.*` |
| §9 进一步阅读 | code-map + dsaft + contracts | code-atlas v1.2.0 + dsaft-overview v1.1.0 + d7 spec v3.8.0 |

### 2.2 W2: `openspec/specs/architecture/code-atlas.md` v1.1.0 → v1.2.0

| 旧段 | 旧内容 | 新内容 |
|------|--------|--------|
| Version + Last Updated | v1.1.0 / 2026-06-13 | v1.2.0 / 2026-06-19 |
| Demand | DM-20260610-012 (QueryLoop v2) | DM-20260619-001 (docs alignment to D7 v2.0) |
| D-S Index | QueryLoop v2 主索引（contextengine/query/*, SubQuery, FlowTap）| **替换为 D7 v2.0 unified 主索引**（见下表）|
| `subquery` / `sidechain_transcript` | IMPLEMENTED | **DEPRECATED**（DM-20260616-004 queryloop legacy decommission 落地，subquery 降级为 fallback-only）|
| `query_loop` | D2-S10 主入口 | **DEPRECATED**（D2.QueryLoop.Run 标记 DEPRECATED，emit legacy invocation metric；D7 取代）|
| `workplan` | ORCH-S1 读模型 | 改归 D7-S4，状态：IMPLEMENTED |
| `execution_flow` (Flow Hub) | ORCH-S2 | 改归 D7-S4，状态：IMPLEMENTED |
| `im_flow_sink` | ORCH-S2 | 改归 D7-S4，状态：IMPLEMENTED |
| `delegate` | D4-S10 | 保持 + 加 D7-S2-A04 DispatchWorker 索引（v1.1 闭环）|
| `worker_engine` | D4-S10 | 保持 |
| `worktree` | D2-S12 | 保持 + 加 v2.0 unified worktree 路径（per-handle wt path）|
| Shared Contracts | 5 契约 + 5 配置 | 加 4 契约：`RunRegistry` / `ResolveAwaiter` / `WorkItem` v2 / `FocusHint`；加 2 配置：`workmodel.*` / `wave.*` |
| Bootstrap Wiring | 3 文件（execution_flow/delegate/cli_events）| 4 文件：加 `wire_coordinator.go::WireD7`（coordinator/turn/workmodel/wave 全 wiring）|
| Dependency Direction | D1→D2→D4 旧图 | D1→D7(coordinator)→D2/D3/D4 + D7 wave/flow/workplan 内闭环 |
| Test Placement | 4 T 层目录 | 保持 + 加 turn/ 目录：`turn/orchestrator_test.go` + `turn/llm_test.go` |

**D7 v2.0 unified D-S Index 新表**：

| L4 ID | 名称 | D-S | 包路径 | 关键类型 |
|-------|------|-----|--------|----------|
| coordinator_entry | D7 主入口 | D7-S2 | `orchestration/coordinator/` | `Entry`, `ProcessMessage` |
| classifier | Intent 分类器 | D7-S2-A02 | `orchestration/coordinator/classifier.go` | `RuleClassifier`, `LLMClassifier` |
| command_handler | IntentCommand 调度 | D7-S2-A03 | `orchestration/coordinator/command_handler.go` | `Handle` |
| orchestrate_path | IntentOrchestrate 调度 | D7-S2-A03 | `orchestration/coordinator/orchestrate_path.go` | `Run` |
| fast_path | IntentFast 调度 | D7-S2-A03 | `orchestration/coordinator/fastpath.go` | `Run` |
| turn_orchestrator | RunTurn 主循环 | D7-S2-A06 | `orchestration/turn/orchestrator.go` | `RunTurn`, `ResolveHint` |
| llm_invoker | D7 直调 D3 | D7-S2-A07 | `orchestration/turn/llm.go` | `GatewayInvoker.InvokeStream` |
| workitem | 任务统一抽象 | D7-S1 | `orchestration/workmodel/workitem.go` | `WorkItem`, `WorkTree` |
| run_registry | 任务注册表 | D7-S1 | `orchestration/workmodel/run_registry.go` | `RunRegistry` |
| resolve_awaiter | 阻塞等待 | D7-S1 | `orchestration/workmodel/awaiter.go` | `ResolveAwaiter` |
| task_manager | Task CRUD | D7-S1 | `orchestration/workmodel/task_manager.go` | `TaskManager` |
| plan_mode | /plan 工作流 | D7-S5 | `orchestration/workmodel/plan_mode.go` | `Enter`, `Approve` |
| llm_decomposer | LLM 拆 DAG | D7-S5-A03 | `orchestration/coordinator/llm_decomposer.go` | `LLMDecomposer` |
| wave_scheduler | DAG 调度 | D7-S3 | `orchestration/wave/scheduler.go` | `Start`, `WaitForCompletion` |
| execution_hub | FlowEvent 聚合 | D7-S4 | `orchestration/flow/hub.go` | `GlobalHub` |
| workplan_service | WorkPlan 读模型 | D7-S4 | `orchestration/workplan/service.go` | `Service` |
| im_sink | 飞书 worker_progress | D7-S4 | `orchestration/imsink/gateway_sink.go` | `GatewaySink` |
| context_preparer | D2 Follower | D2-S15 | `contextengine/prepare/` | `ContextPreparer` |
| tool_round_executor | D2 Follower | D2-S18 | `contextengine/enforce/` | `ToolRoundExecutor` |
| session_persister | D2 Follower | D2-S17 | `contextengine/persist/` | `SessionPersister` |

### 2.3 W3: `docs/architecture/dsaft-overview.md` v1.0.0 → v1.1.0

| 旧段 | 旧内容 | 新内容 |
|------|--------|--------|
| Header | Last Updated 2026-06-13 | Last Updated 2026-06-19 |
| §1 5 层 | 6 行表（D-S-A-F-T）| 保持 |
| §2 7 域 + ORCH | 6 域 + "ORCH 读模型，非 D7" | **7 域**：D1-D7 全列，ORCH → D7 升级为核心域 |
| 域架构图 | ASCII 6 域图 + "ORCH 读模型" | 重画为 7 域图，D7 含 S1-S5 子层标注 |
| §3 D2 上下文域 | "现行重点" | 改为"D2 Follower 契约"：D2 不再是入口，是 D7 编排的 Follower |
| §4 生产路径 | QueryLoop vs legacy | 改为"主入口路径"：D7 ProcessMessage + 4 IntentKind |
| §5 与其他文档 | 6 项导航 | 加 d7 spec v3.8.0 入口 + code-atlas v1.2.0 |

**D7 S 层 5 个子层实现状态**（切法 A 博弈角色）：

| S 层 | 博弈角色 | 实现状态 |
|------|---------|---------|
| D7-S1 | State Authority | ✅ IMPLEMENTED (v1.1 closure, DM-20260614-009) |
| D7-S2 | Screening + Turn Leader | ✅ IMPLEMENTED (v1.0 closure, DM-020) |
| D7-S3 | Mechanism Designer | ✅ IMPLEMENTED (WaveScheduler + ConflictGuard + 5-slot pool) |
| D7-S4 | Costly Signaler | ✅ IMPLEMENTED (ExecutionFlowHub + WorkPlan + IMSink) |
| D7-S5 | Information Producer | ✅ IMPLEMENTED (ClassifyIntent + LLMDecomposer + PlanMode) |

## 3. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 新人基于旧文档理解 D2 仍是入口 | dsaft-overview.md §3 显式标"D2 Follower"，request-flow.md 头部加 "主入口已迁 D7（v1.0 closure 2026-06-15）" banner |
| 退役模块路径仍被引用 | DEPRECATED 显式标 + 给替代路径（如 subquery → turn.RunTurn）|
| 代码锚点漂移 | 提交前 `git grep` 验证所有 `:Function` 引用，PR review 检查 |
| docs 与 spec 不一致 | docs 单向对齐 spec（spec v3.8.0 不动），如发现 spec 错则开独立 change 改 spec |

## 4. 不变更（边界声明）

- D7 域代码（`internal/layers/orchestration/**`）— 完全不动
- D2 域代码（`internal/layers/contextengine/**`）— 完全不动
- D1 域代码（`internal/layers/communication/**`）— 完全不动
- `openspec/specs/d7-orchestration/spec.md` v3.8.0 — 不动
- D-S 编号体系（D1-D7 + S/A/F/T）— 不增不改
