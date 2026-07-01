# Devrix 代码路径布局规范（Domain / Scenario）

**Capability:** architecture-code-layout  
**Status:** Active  
**Version:** 1.14.0
**Last Updated:** 2026-07-01 (devrix-d7-physical-layout-alignment DM-20260701-004 PR-3: §4.2 D7 S5 sub 行 "Plan agent" → "Plan Generation" 命名收敛 + doc-only 双登记 wording 收敛（与 a-registry D7-S6-A03/A04 加 cross-reference 对齐）)
**Parent:** `openspec/specs/architecture/layering.md`

---

## 1. 目标

代码目录 **必须先能回答「属于哪个领域、哪个场景」**，再体现技术分层。  
路径是 DSAFT 资产（D/S）在仓库中的 **物理锚点**；`openspec/specs/{domain}/` 是同一资产的 **规格锚点**。

```
L1 领域 (D)  →  internal/layers/{domain-slug}/
L2 场景 (S)  →  …/{scenario-slug}/
L4 功能点 (F) →  …/{scenario-slug}/*.go  （或 activity 子目录）
```

> **Go 包名与目录名** 遵循 `openspec/specs/project/coding.md` §2.2：**全小写、无下划线、单数，`package` 与目录叶子名严格一致**。禁止 `d1`、`s13` 等 DSAFT 编号进入包名（见 `layering.md` §命名规约）。

---

## 2. 命名规则

| 层级    | 路径段                | 规则                                            | 示例                                                |
| ----- | ------------------ | --------------------------------------------- | ------------------------------------------------- |
| L1 领域 | `{domain-slug}/`   | 全小写复合词，无下划线；与 `layering.md` D 表一致             | `communication`, `contextengine`, `orchestration` |
| L2 场景 | `{scenario-slug}/` | **Go 合法目录名**（同 §2.2）；在本文 §4 登记，**禁止自造**       | `capture`, `thinking`, `taskprogress`             |
| L3 活动 | `{activity-slug}/` | **可选**；仅当单 S 下 A 组 >1 且需物理隔离时使用；同样遵守 Go 目录名规则 | `dispatch`, `encode/feishu`                       |
| 域内核   | `kernel/`          | 非 S 的共享模型（Card、Session 值对象等）                  | `communication/kernel/`                           |
| 跨域契约  | —                  | `internal/shared/contracts/`                  | 禁止域内 duplicated 契约                                |

**禁止作为 L2 场景目录名：**

- 技术角色词：`gateway`, `adapters`, `eventbus`, `handler`, `service`（可作 **文件名** 或 L3 activity）
- DSAFT 编号：`d1`, `s13`, `d7-s2`
- 动词堆叠无场景语义：`utils`, `common`, `internal`
- **下划线**：Go 包名不允许，`scenario-slug` 目录亦不允许（如 ~~`present_thinking`~~）

---

## 3. 目录决策树（新文件放哪）

```text
1. 是否跨域共享契约/类型？
   YES → internal/shared/{contracts,types,config}/
2. 属于哪个领域 D？
   → internal/layers/{domain-slug}/
3. 属于哪个场景 S？（查 §4 注册表）
   → …/{scenario-slug}/
4. 是否域内核（无 S 归属）？
   → …/kernel/
5. 是否仅编排/启动接线？
   → internal/bootstrap/ 或 cmd/
6. 文件名体现 L4 F 语义（camelCase 导出 / snake 文件）
```

**测试文件** 与实现同目录：`{scenario-slug}/*_test.go`；E2E 在 `tests/acceptance/` 按域标签组织。

---

## 4. Scenario 路径注册表

### 4.1 D1 Communication（canonical S13–S18）

| S ID   | Scenario            | scenario-slug  | 目标路径                          | 职责摘要                                   |
| ------ | ------------------- | -------------- | ----------------------------- | -------------------------------------- |
| D1-S13 | CaptureUserIntent   | `capture`      | `communication/capture/`      | 入站、Persist、Dispatch、Permission、Command |
| D1-S14 | PresentThinking     | `thinking`     | `communication/thinking/`     | Thinking 信号映射与 emit                    |
| D1-S15 | PresentTaskProgress | `taskprogress` | `communication/taskprogress/` | Task/Tool/Worker/Milestone **展示**      |
| D1-S16 | DeliverConclusion   | `conclusion`   | `communication/conclusion/`   | Conclusion 流式/终态/摘要                    |
| D1-S17 | ConnectChannel      | `channel`      | `communication/channel/`      | IM 适配、连接、实例、限流、Encode                  |
| D1-S18 | GuaranteeDelivery   | `delivery`     | `communication/delivery/`     | EventBus、Critical、Drain                |
| —      | Domain Kernel       | `kernel`       | `communication/kernel/`       | Card、平台无关消息模型                          |

**横切（暂存，随迁移收敛）：**

| 组件                  | 当前路径                                     | 收敛目标      |
| ------------------- | ---------------------------------------- | --------- |
| Turn tracker / 信号锚点 | `communication/capture/signal/`          | ✅ 已收敛     |
| 契约映射                | `shared/contracts/im_outbound_signal.go` | 保持 shared |

### 4.2 D7 Orchestration

> D7 待迁移目录须遵守 §2.2（无下划线）。DSAFT 场景名与 slug 映射示例：`Wave Scheduler` → `wavescheduler/`。

| S ID          | Scenario                | scenario-slug         | 目标路径                                 | 当前路径（迁移中）                          |
| ------------- | ----------------------- | --------------------- | ------------------------------------ | ---------------------------------- |
| D7-S1         | Work Model              | `workmodel`           | `orchestration/workmodel/`           | ✅ DM-012                           |
| D7-S2         | Session Orchestrator    | `sessionorchestrator` | `orchestration/sessionorchestrator/` | ✅ DM-20260619-005                  |
| D7-S3         | Wave Scheduler          | `wavescheduler`       | `orchestration/wavescheduler/`       | ✅ DM-20260619-005                  |
| D7-S4         | Execution Flow          | `executionflow`       | `orchestration/executionflow/`       | ✅ DM-20260619-005                  |
| D7-S5         | Decision & Planning     | `decisionplanning`    | `orchestration/decisionplanning/`    | ✅ DM-20260619-005                  |
| D7-S6         | MUPS Governance         | `mupsgov`             | `orchestration/sessionorchestrator/` + `orchestration/mups/` + `orchestration/escape/` + `orchestration/interfaces/` | ✅ governance overlay (DM-20260701-002; no forced directory migration) |
| —             | Hub-Spoke dispatch (S2) | —                     | `sessionorchestrator/dispatch.go`    | ✅ DM-20260619-005（自 `hubspoke/`） |
| —             | Hub-Spoke bridge (S4)   | `bridge`              | `executionflow/bridge/`              | ✅ DM-20260619-005（自 `hubspoke/`） |
| —             | Delegate routing F      | `delegatetools`       | `orchestration/delegatetools/`       | ✅ DM-011                           |
| —             | Session command queue F | `executionflow`       | `orchestration/executionflow/`       | ✅ DM-013 + DM-20260625-018 PR-3b（物理合并到 executionflow 父级）|
| D7-S5 sub     | Plan Generation         | `plan`                | `orchestration/plan/`                | ✅ DM-20260625-019 PR-2（物理独立成包，S5 sub-registration carve-out；doc-only 双登记（与 decisionplanning/ 并列 S5）：物理在 `decisionplanning/` 路径下，但 a/f-registry 同时登记 `plan/` 与 `decisionplanning/`，**0 shim / 0 alias / 0 git mv**；DM-20260701-004 PR-3 收敛命名 "Plan agent"→"Plan Generation" + doc-only 双登记 wording）|
| —             | Cross-S Kernel          | `orchtypes`           | `orchestration/orchtypes/`           | ✅ DM-20260701-004 PR-1（共享类型 / 哨兵 / 边界决策 / 先验 / 异常检测 / LLM 调用契约 6 A + 6 F；**no Go shim, no re-export, 直接 import**）|
| —             | Cross-cutting Hardening | `hardening`           | `orchestration/hardening/`           | ✅ DM-20260701-004 PR-1（emitter 集中点 namespace，**不是 owner**；ConflictGuard 实际 owner 是 `wavescheduler/`）|
| —             | TaskContract contracts  | `interfaces`          | `orchestration/interfaces/`          | ✅ DM-20260629-007 (v7.0 PR-A) + DM-20260629-008 (v7.0 PR-B)；pure types 包，0 import D7 任何子包 |
| **D7-S2-A06** | **RunTurnLoop**         | `sessionorchestrator` | `sessionorchestrator/turn_orchestrator.go` | ✅ DM-020 v2.3（`turn/` 子包已物理合并入 `sessionorchestrator/`，见 DM-20260626-004 6S Package Merge） |
| **D7-S2-A07** | **InvokeLLM**           | `sessionorchestrator` | `sessionorchestrator/llm.go`         | ✅ DM-020 v2.3（同上）|

> **DM-020 bootstrap 注释（v1.0 Registry / v2.3 实施）：** `bootstrap/main.go` 目标接线：
> 
> ```text
> WireContextLLM(obs) → TurnOrchestrator deps  # 迁出 context_engine，D7 持有 ILLMGateway
> WireContextEngine() → ContextPreparer only   # 无 LLM 字段
> WireCoordinator(turnOrch, ctxPrep, ...)      # D7 持有 Turn Leader + Hub-Spoke
> ```
> 
> **v6.0.0 6 S 终态（DM-20260626-001 + 后续 package cleanup sprint）：**
> - **`turn/` 子包已物理合并入 `sessionorchestrator/`**（DM-20260626-004 6S Package Merge），故 D7-S2-A06/A07 当前路径表 1 行（统一归属 `sessionorchestrator/turn_orchestrator.go` + `llm.go`）
> - **`hubspoke/` 已退役**（DM-20260625-018 Package Cleanup Sprint），dispatch.go / agent_bridge.go / subquery_bridge.go 物理迁入 `sessionorchestrator/` + `executionflow/bridge/`
> - **`milestone/` 已退役**（早期 v2.x 迁入 `executionflow/workplan/`）
> - **`coordinator/` 已退役**（v1.1 closure 同步后完全删除）
> 
> **v6.0.0+ 新增归属（DM-20260701-004 PR-1 登记）：**
> - **`plan/`（S5 sub-registration）**：物理独立子包，但 a/f-registry 双登记（同时归属 `decisionplanning/` 路径下的代码），是 **doc-only dual-registration**，**0 物理 shim**
> - **`orchtypes/`（Cross-S Kernel）**：物理即 kernel，**no Go shim, no re-export, 直接 import**，S1-S6 任意 A 可直接 import
> - **`hardening/`（Cross-cutting Discipline Keeper）**：emitter 集中点 namespace，**ConflictGuard 实际 owner 是 `wavescheduler/`**（物理位置与 hardener 命名解耦）
> - **`interfaces/`（TaskContract contracts）**：pure types 包，0 import D7 任何子包

### 4.3 D2 Context Engine

> **Canonical S15–S20**（DM-20260614-009 + DM-20260619-007 v2.2 closure）。v2.2 终态路径已锁定。

| S ID       | Scenario                        | scenario-slug   | 终态路径（v2.2, DM-20260619-007）                                                    | Status |
| ---------- | ------------------------------- | --------------- | --------------------------------------------------------------------------------- | ------ |
| D2-S15     | PrepareExecutionContext         | `prepare`       | `prepare/{memory,compression,prompt,conversation,token,attachments,usercontext}/` + `prepare/adapters/` + `prepare/orchestrator.go` | ✅ |
| D2-S16     | ~~RunQueryLoop~~                | —               | **REMOVED**（DM-20260618-010）；`contextengine/query/` 已删除 → D7 `orchestration/turn/` | ✅ |
| D2-S17     | PersistSessionState             | `persist`       | `persist/{snapshot,transcript}/` + `persist/{commit,commit_window,orchestrator}.go` + `persist/memory/store.go` (P4 split) | ✅ |
| D2-S18     | EnforceExecutionPolicy          | `enforce`       | `enforce/{permission,tools,registry,sandbox}/` + `enforce/{tool_filter,agent_role_filter,background_task_tools,planmode_tools}.go` | ✅ |
| —          | Process legacy (P5 retired)     | `legacy`        | `legacy/{engine,engine_builder,engine_persist_v2,engine_*,prepared_turn_*}.go` — **Deprecated**，slog.Warn + D2-STRUCT-T07 guard | ✅ |
| —          | Domain kernel                   | `kernel`        | `kernel/{contracts,spans}.go`                                                       | ✅ |
| —          | Public API re-exports           | —               | `contracts.go`, `aliases.go` (type aliases → legacy), `tool_context.go` (type alias → `enforce/tools/context.go`) | ✅ |
| D2-S19     | ~~NestedExecution~~ → S15+S18           | —               | **DISMANTLED**: fork→`prepare/conversation/`, subquery+background→`enforce/` | ✅ |
| D2-S20     | ~~LegacyHarnessFallback~~              | —               | **REMOVED**: harness 路径已移除，`fallback/` 目录已删除，`query_loop.enabled=false` 不再有效 | ✅ |

> **v2.2 深度规则（DM-20260619-007 P3-T5）：**
> - `enforce/tools/surface/` 是允许的 2 层嵌套（scenario → scenario 内部 sub-scenario）。其他 scenario 子目录深度 ≤ 1。
> - `enforce/` 与 `prepare/` 与 `persist/` 是 peer；不存在互相 import。
> - `prepare/memory/` 与 `persist/memory/` 必须保持解耦：Recall (S15) 在前者，Store (S17) 在后者；端口接口在 `internal/shared/contracts/memory.go`（D2-STRUCT-T04）。
> - `legacy/` 仅由 `contextengine/aliases.go` 反向引用；其他生产代码必须经 D7 SessionOrchestrator 或 turn_adapter 走 D2 路径。
>
> **D2-STRUCT Layout Guards（`internal/lint/layer/d2_layout_test.go`）：**
> - **D2-STRUCT-T01** 根目录生产文件白名单（`contracts.go` + `aliases.go` + `tool_context.go` + `doc.go`）
> - **D2-STRUCT-T02** 无 `engine_persist.go` 在根或 `facade/` 外（`facade/` 已不存在，等价于全工程除 `legacy/` 外禁用）
> - **D2-STRUCT-T03** `enforce/tools/` 包名 = `package tools`，禁止 `package toolrunner` 残留
> - **D2-STRUCT-T04** `prepare/memory/` ↔ `persist/memory/` 无循环导入
> - **D2-STRUCT-T05** `enforce/orchestrator.go` 已删除（stub 移除，dispatch 由 `turn_adapter` 接管）
> - **D2-STRUCT-T06** scenario 下目录深度 ≤ 2 层
> - **D2-STRUCT-T07** P5 退役：禁止新增 `legacy.ContextEngine.Process()` 生产引用（CI 硬阻断）

> **DM-020 bootstrap 注释（v1.0 Registry）：** `WireContextLLM` 当前在 `internal/bootstrap/` 为 D2 接线 ILLMGateway。v2.0-b 迁移后，D7 持有 ILLMGateway（`WireContextLLM → TurnOrchestrator deps`），D2 仅保留 ContextPreparer（`WireContextEngine → ContextPreparer only，无 LLM 字段`）。

**Legacy module 路径（冻结追溯）：**

| Legacy S | scenario-slug | 路径                                                                      |
| -------- | ------------- | ----------------------------------------------------------------------- |
| D2-S2    | `compression` | ~~`contextengine/compression/`~~ → `contextengine/prepare/compression/` |
| D2-S3    | `memory`      | ~~`contextengine/memory/`~~ → `contextengine/prepare/memory/` (read) + `contextengine/persist/memory/` (write, v2.2) |
| D2-S4    | `token`       | ~~`contextengine/token/`~~ → `contextengine/prepare/token/`              |
| D2-S11   | `queue`       | ~~`contextengine/queue/`~~ → `orchestration/executionflow/` (D7-S4, formerly `sessionqueue/`, DM-20260625-018 PR-3b)      |
| D2-S12   | `sandbox`    | `contextengine/sandbox/` → `contextengine/enforce/sandbox/` (v2.2, P3-T1) |
| D2-S5/S8 | `toolrunner`  | `contextengine/enforce/toolrunner/` → `contextengine/enforce/tools/` (v2.2, P3-T2, package renamed) |
| —        | `facade`      | `contextengine/facade/` → `contextengine/legacy/` (v2.2, P5 retired) |

### 4.4 D3 LLM Gateway（canonical S1–S6 / 5+1 价值流）

> **Canonical S1–S6**（DM-20260614-016 / devrix-d3-sa-refine / DM-20260614-019 v2.0）。
> v1.0 注册表重排，0 行为变更（已 ACCEPTED commit 199ad18）。
> **v1.1** 韧性可见性 + D6 3 probe + IAdapter.Protocol() + obs nil fail-fast（已 ACCEPTED commit 3a6970b）。
> **v2.0** 物理路径按 scenario-slug 迁移（DM-20260614-019 / devrix-d3-sa-refine-v2.0 **ACCEPTED** commit d222328）。
> 详见 `openspec/archive/2026-06-14-devrix-d3-sa-refine-v2.0/acceptance-report.md`。

| S ID  | Scenario         | scenario-slug (v2.0)           | v1.0 当前路径                                             | v2.0 目标                                       | 状态         |
| ----- | ---------------- | ------------------------------ | ----------------------------------------------------- | --------------------------------------------- | ---------- |
| D3-S1 | RouteModel       | `route`                        | `gateway/router.go` (路由解析部分)                          | `llmgateway/route/`                           | ✅ 完成       |
| D3-S2 | StreamChat       | `stream`                       | `adapter/` (全部) + `gateway/gateway.go` Stream 主实现     | `llmgateway/stream/`（含 `stream/adapter/` 子目录） | ✅ 完成       |
| D3-S3 | ProtectCall      | `protect`                      | `breaker/` + `retry/` + `gateway/breaker_observer.go` | `llmgateway/protect/`（两机制独立 .go）              | ✅ 完成       |
| D3-S4 | BudgetTokens     | `budget`                       | `token/`                                              | `llmgateway/budget/`                          | ✅ 完成       |
| D3-S5 | GuardContent     | `guard`                        | `safety/`                                             | `llmgateway/guard/`                           | ✅ 完成       |
| D3-S6 | ConfigureGateway | `configure`                    | `config/` + `shared/config/llmgateway.go`             | `llmgateway/configure/`（合并 shared）            | ✅ 完成       |
| —     | Domain Kernel    | (根 `contracts.go` 拆分后 < 200 行) | `llmgateway/contracts.go`                             | `llmgateway/contracts.go` (145 行)             | ✅ AC-09 达成 |
| D3-X  | CROSS 跨域锚点       | (Bridge 不变)                    | `internal/bridges/llm/`                               | `internal/bridges/llm/`                       | ✅ 不动       |

**Scenario-slug 命名依据**（与 code-layout §2 规则一致）：

- `route` — 路由解析（语义：用户给 model 名 → 返回 provider+实际 model）
- `stream` — 流式调用（语义：流式 chat completion；非 `adapter`/`gateway` 等技术角色词）
- `protect` — 韧性保护（语义：Breaker + Retry + Fallback 合并后承诺；非 `breaker`/`retry`）
- `budget` — 预算控制（语义：Token 预算检查 + 截断；非 `token`）
- `guard` — 内容守卫（语义：Safety 内容过滤；非 `safety`/`filter`）
- `configure` — 配置加载（语义：横切配置；非 `config`/`loader`）

**v1.0 兼容性约束**：

1. **运行时字符串不变**：`llm.stream` / `llm.provider.route` / `llm.circuit_breaker` / `llm.retry` / `llm.adapter.stream` 5 个 span 名保持字面量（R1 Q3 决议）
2. **配置 key 不变**：`llm_gateway:` / `circuit_breaker:` / `model_tiers:` 等 YAML key 在 v1.0 / v2.0 都不变
3. **metric 名不变**：`llm_requests_total` / `llm_errors_total` / `llm_latency_seconds` 在 v1.0 / v2.0 都不变
4. **Bridge 路径不变**：`internal/bridges/llm/` 是 D3 → D2 的跨域锚点（R1 D2 决议），v1.0 / v2.0 都不迁移到 D3 内部

**contracts.go 拆分粒度**（v2.0 ACCEPTED；F9 完整拆分 deferred 至下 release）：

- **留根**（kernel 性质，跨 S / 跨域共享）：`Request` / `Chunk` / `TokenUsage` / `ToolCall` / `AdapterChunk` / `CircuitState` / `BreakerStateObserver` 等
- **re-export 桥接**：v2.0 迁移时 8 个旧路径 `bridge.go` 保留 type alias，1 发布周期后物理删除

**R3 NQ-6 决策**（v2.0）：不引入 `kernel/` 子包；kernel 类型继续留根 `contracts.go` 中。

**跨域漂移（v2.0 迁出 D2）：**

| 组件                      | 当前路径                                  | 目标                                 | 状态       |
| ----------------------- | ------------------------------------- | ---------------------------------- | -------- |
| ~~delegate_tools~~      | ~~`contextengine/delegate_tools.go`~~ | `orchestration/delegatetools/`     | ✅ DM-011 |
| TaskManager             | ~~`contextengine/tasks/`~~            | `orchestration/workmodel/` (D7-S1) | ✅ DM-012 |
| queue delegate-progress | ~~`contextengine/queue/`~~            | D7-S4 `executionflow/` (formerly `sessionqueue/`) | ✅ DM-013 + DM-20260625-018 PR-3b |

### 4.5 D4 Multi-Agent（canonical S11–S16 / 5+1 价值流）

> **Canonical S11–S16**（DM-20260614-018 / devrix-d4-sa-refine）。v2.0-d 物理迁移完成；v2.0-e re-export shim 已删除（`factory/` `agent/` `sessionview/` `observer/` `tool/`）；Hub-Spoke 编排代码 v2.0 迁 D7 `hubspoke/`。

| S ID   | Scenario            | scenario-slug (v2.0) | v1.0 当前路径                                                 | v2.0 目标                                   | 状态       |
| ------ | ------------------- | -------------------- | --------------------------------------------------------- | ----------------------------------------- | -------- |
| D4-S11 | ProvisionAgent      | `provision`          | `factory/` + `collaboration/` + `builtin/`                | `multiagent/provision/`                   | ✅ v2.0-d |
| D4-S12 | RunAgentLoop        | `run`                | `agent/`（lifecycle, state, perm_gate）                     | `multiagent/run/`                         | ✅ v2.0-d |
| D4-S13 | IsolateAndMerge     | `isolate`            | `agent/forkjoin.go` + `sessionview/` + `worker_engine.go` | `multiagent/isolate/` + `multiagent/run/` | ✅ v2.0-d |
| D4-S14 | ExecuteWorker       | `execute`            | `delegate/service.go`                                     | `multiagent/execute/`                     | ✅ v2.0-d |
| D4-S15 | InvokeExternalAgent | `external`           | `tool/`                                                   | `multiagent/external/`                    | ✅ v2.0-d |
| D4-S16 | ConfigureAgents     | `configure`          | `shared/config/multiagent.go`                             | `multiagent/configure/`                   | ✅ v2.0-d |
| —      | Domain Kernel       | `kernel`             | `contracts.go` + `observer/`                              | `multiagent/kernel/`                      | ✅ v2.0-d |

**Hub-Spoke 迁 D7（v2.0-b/c，非 D4 目录）：**

| 组件                   | v1.0 路径                                | v2.0 目标                                     | 状态       |
| -------------------- | -------------------------------------- | ------------------------------------------- | -------- |
| Agent FlowBridge     | `multiagent/delegate/bridge.go`        | `orchestration/hubspoke/agent_bridge.go`    | ✅ v2.0-d |
| Dispatch / fallback  | `multiagent/delegate/service.go`（编排部分） | `orchestration/hubspoke/dispatch.go`        | ✅ v2.0-d |
| SubQuery Flow        | `contextengine/enforce/flow_report_test.go`  | `orchestration/hubspoke/subquery_bridge.go` | ✅ v2.0-d |
| Dispatcher bootstrap | `bootstrap/delegate.go`                | `bootstrap/delegate.go`（重构）                 | ✅ v2.0-d |

**v2.0-b 里程碑：**

- `execute/worker.go` — WorkerExecutor 实现（D4-S14 执行面）
- `execute/contracts.go` — WorkerExecutor 接口 + WorkerRunSpec
- `hubspoke/agent_bridge.go` — Agent FlowBridge（从 D4 迁入）
- `hubspoke/dispatch.go` — SpokeDispatcher（从 D4 delegate/service.go 编排逻辑迁入）
- `hubspoke/doc.go` — 包文档
- `delegatetools/` — 重构使用 Dispatcher.Dispatch()；WorkerRole 常量本地化

### 4.6 D5 Observability

> **v2.1 Terminal (2026-06-19):** 4+1 价值流 S21–S24+S0 号段冻结。物理路径 canonical 化完成。9 bridge 包 Deprecated → Phase B 删除。诊断工具链（doctor/tracker/faultinject）已落地。**Change:** devrix-d5-v2-terminal（DM-20260619-006）。

| S ID   | Scenario   | scenario-slug | 物理路径                                                                                                                              | 状态               |
| ------ | ---------- | ------------- | ------------------------------------------------------------------------------------------------------------------------------------- | ---------------- |
| D5-S21 | Instrument | `instrument`  | `instrument/tracer/` `instrument/metrics/`（含 `genai_tokens.go`）`instrument/logger/`（含 `debugfilter/`）`instrument/telemetry/` | v2.1 TERMINAL |
| D5-S22 | Export     | `export`      | `export/`                                                                                                                             | v2.1 TERMINAL |
| D5-S23 | Diagnose   | `diagnose`    | `diagnose/coverage/` `diagnose/incident/` `diagnose/doctor/` `diagnose/tracker/` `diagnose/faultinject/` + `health.go`        | v2.1 TERMINAL |
| D5-S24 | Configure  | `configure`   | `configure/settings/` `configure/runtime/` + `config.go` `load.go`                                                                   | v2.1 TERMINAL |
| D5-S0  | Facade     | —             | `observability.go` `bridge.go`                                                                                                        | v2.1 TERMINAL |

**关键迁移历史：**

- v2.0 (2026-06-15)：`tracer/` + `metrics/` + `logger/` + `telemetry/` → `instrument/`；`exporter/` → `export/`；`coverage/` + `incident/` → `diagnose/`；`settings/` + `runtime/` → `configure/`（DM-20260615-003）。旧路径保留 9 bridge。
- **v2.1 Terminal (2026-06-19)：** 诊断工具链（doctor/tracker/faultinject/debugfilter）落地；`genai_tokens.go` → `instrument/metrics/`；`llm_log.go` → `diagnose/incident/`；Bridge 删除（Phase B）；S21–S24+S0 号段冻结。

### 4.7 D6 Evolution

> **2026-06-15 SA Refine v1.0**: S4 "Orchestration" → S12 GuardRuntime 消除 D7 命名冲突（DM-20260615-002）。v1.0 仅注册表，v2.0 物理迁移。

| S ID   | Scenario      | scenario-slug | v2.0 目标               | 当前路径                  | 状态            |
| ------ | ------------- | ------------- | --------------------- | --------------------- | ------------- |
| D6-S11 | RunEvaluation | `evaluate`    | `evolution/evaluate/` | `evolution/evaluate/` | v2.0 PHYSICAL |
| D6-S12 | GuardRuntime  | `guard`       | `evolution/guard/`    | `evolution/guard/`    | v2.0 PHYSICAL |
| D6-S13 | TrackVersion  | `version`     | `evolution/version/`  | `evolution/version/`  | PLANNED       |
| D6-S14 | ReloadConfig  | `reload`      | `evolution/reload/`   | `evolution/reload/`   | PLANNED       |

**关键 v2.0 迁移（DM-20260615-003，已完成）：**

- `evolution/orchestration/` → `evolution/guard/`（package guard）— 消除与 D7 的命名冲突
- `evolution/eval/` → `evolution/evaluate/`（package evaluate）— slug 语义化
- 旧路径保留 2 个 bridge.go（Deprecated，v2.1 移除）

---

## 5. 目标目录树（D1 终态示例）

```text
internal/layers/communication/
├── kernel/                      # Card、Builder（原 core/）
├── capture/                     # S13
│   ├── gateway.go               # CommunicationGateway 入口
│   ├── session_store.go
│   ├── dispatch.go
│   ├── permission.go
│   └── signal/                  # turn tracker
├── thinking/                    # S14
├── taskprogress/                # S15
├── conclusion/                  # S16
├── channel/                     # S17
│   ├── adapters/
│   ├── connection/
│   ├── instance/
│   ├── ratelimit/
│   └── renderers/
└── delivery/                    # S18
    └── eventbus/
```

---

## 6. 当前 → 目标迁移（D1）

| 当前路径                       | 目标 scenario-slug                          | 迁移状态                             | 关联 Change       |
| -------------------------- | ----------------------------------------- | -------------------------------- | --------------- |
| `gateway/`                 | `capture/` + 部分 `conclusion/`             | ✅ IMPLEMENTED                    | DM-20260614-006 |
| `present/`                 | `thinking/` `taskprogress/` `conclusion/` | ✅ IMPLEMENTED                    | DM-20260614-006 |
| `signal/`                  | `capture/signal/`                         | ✅ IMPLEMENTED                    | DM-20260614-006 |
| `adapters/` … `renderers/` | `channel/`                                | ✅ IMPLEMENTED                    | DM-20260614-006 |
| `eventbus/`                | `delivery/eventbus/`                      | ✅ IMPLEMENTED                    | DM-20260614-006 |
| `core/`                    | `kernel/`                                 | ✅ IMPLEMENTED                    | DM-20260614-006 |
| `communication/milestone/` | —                                         | ✅ 已迁至 `orchestration/milestone/` | DM-20260614-006 |

**迁移原则：**

1. **新代码** 必须写入 §4 登记的 **目标 scenario-slug**；禁止在新 `gateway/` 下追加 F。
2. **旧路径** 允许 `re-export` / type alias 一个发布周期，PR 标注 `BREAKING` 与迁移表。
3. 每个迁移 PR 更新：`code-layout.md` 迁移状态、`layering.md` Package Map、`d1-communication/d1-domain.md` + `spec.md`。
4. 单 PR 仅迁 **一个 scenario-slug**（或一组强耦合 F），并跑关联 L5/T。

---

## 7. 与 OpenSpec 的对应

| 代码                          | 规格                                                      |
| --------------------------- | ------------------------------------------------------- |
| `internal/layers/{domain}/` | `openspec/specs/{domain}-*/{domain}-domain.md`（领域 SoT）+ `spec.md`（Gherkin） |
| scenario-slug               | `layering.md` S 表 + 域 `a-registry.md` / `f-registry.md` |
| 流程 / 时序 / Runbook（D1）      | `d1-communication/terminal-state-guide.md` · `observability-guide.md` |
| `*_test.go` 内 `// T:`       | `openspec/specs/{domain}/t-registry.md`                 |
| span 名                      | `{domain}/span-registry.md`                             |

验收/归档时 **代码路径与 spec 必须同步更新**（见 `specs/05-delivery-process.md` §6.3、§8.2）。

---

## 8. 变更记录

| 版本        | 日期             | 说明                                                                                                                                                                                                                                                                                                                                |
| --------- | -------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1.0.0     | 2026-06-14     | 初版：D1/D7 scenario-slug 注册表 + D1 迁移矩阵                                                                                                                                                                                                                                                                                              |
| 1.1.0     | 2026-06-14     | D1 物理路径迁移完成（capture/channel/delivery/kernel）                                                                                                                                                                                                                                                                                      |
| 1.3.0     | 2026-06-14     | D2 S15–S20 Canonical；delegate_tools → delegatetools (DM-011)                                                                                                                                                                                                                                                                      |
| 1.4.0     | 2026-06-14     | **D3 S1–S6 Canonical**（DM-20260614-016 / devrix-d3-sa-refine）：5+1 价值流 scenario-slug 注册表（`route` `stream` `protect` `budget` `guard` `configure`）；v1.0 物理路径保留 + v2.0 迁移目标映射；D3-X 跨域锚点声明 `internal/bridges/llm/`；contracts.go 拆分粒度占位                                                                                                |
| **1.5.0** | **2026-06-14** | **D3 S1–S6 v2.0 物理迁移状态**（DM-20260614-019 / devrix-d3-sa-refine-v2.0 ACCEPTED commit d222328）：6 个 slug 全部 ✅ 完成（含 `stream/adapter/` 子目录 + `configure/` 跨包合并 shared/config）；D3-X 跨域锚点 ✅ 不动；contracts.go 145 行 AC-09 达成                                                                                                               |
| **1.6.0** | **2026-06-15** | **D4 S11–S16 + Kernel v2.0-d 物理迁移状态**（DM-20260614-018 / devrix-d4-sa-refine commit 3905c6a）：6 个 slug + kernel 全部 ✅ v2.0-d（`provision/` `run/` `isolate/` `execute/` `external/` `configure/` `kernel/`）；Hub-Spoke v2.0-d（agent_bridge/dispatch/subquery_bridge → `orchestration/hubspoke/`）；execute(9) + hubspoke(23) 测试新增；71 包全绿 |
| **1.7.0** | **2026-06-15** | **D4 v2.0-e re-export 清理**（commit e30fe72）：5 个 re-export shim 删除（`factory/legacy.go` `agent/legacy.go` `sessionview/legacy.go` `observer/noop.go` `tool/legacy.go`）；observer 引用迁移至 kernel；71 包全绿                                                                                                                                  |
| **1.8.0** | **2026-06-15** | **DM-020 v1.0 Registry：** D7-S2-A06/A07 turn/ 目录登记；D2-S16 Legacy Freeze；bootstrap 接线注释（WireContextLLM → TurnOrchestrator）                                                                                                                                                                                                         |
| **1.9.0** | **2026-06-15** | **D5+D6 SA Refine v2.0 物理路径迁移完成**（DM-20260615-003）：D5 4 个 scenario 物理迁移（instrument/export/diagnose/configure）+ D6 2 个 scenario 物理迁移（evaluate/guard）；~106 文件移动 + ~133 import 路径更新；3 个包重命名（eval→evaluate, exporter→export, orchestration→guard）；11 个 bridge.go（Deprecated, v2.1 移除）                                                 |
| **1.10.0** | **2026-06-15** | **DM-20260615-004 D7 Intent 路径正交化文档同步**：layering.md v4.4.0 D7 目录树新增 `coordinator/command_handler.go`（IntentCommand 零 LLM 分发）+ `coordinator/orchestrate_path.go`（IntentOrchestrate 显式调 SynthesizeTaskGraph + WaveScheduler）；两文件位于 `internal/layers/orchestration/coordinator/` 包内，PR #35 引入；§3 目录决策树 §6 D1 终态示例同步不需更新（D7 目录树属于 layering.md 职责） |
| **1.11.0** | **2026-06-16** | **D1 领域文档同步**：§7 OpenSpec 对应表增加 `d1-domain.md` 领域 SoT 与 `terminal-state-guide` / `observability-guide` 指南路径 |
| **1.12.0** | **2026-06-19** | **D5 v2.1 Terminal（DM-20260619-006）**：§4.6 D5 物理路径表更新为 v2.1 TERMINAL；diagnose 子目录补全 doctor/tracker/faultinject；instrument 子目录补全 debugfilter；genai_tokens.go → instrument/metrics/ + llm_log.go → diagnose/incident/；Bridge 删除路线标注 |
| **1.13.0** | **2026-07-01** | **D7 Physical Layout Alignment（DM-20260701-004）PR-1 §4.2 D7 终态化**：**(1) 移除 retired ghost shim 行**（`coordinator/` 🔶 row 删除 — DM-20260625-018 已全删；`hubspoke/` 🔶 row 删除 — DM-20260626-004 6S Package Merge 已迁入 sessionorchestrator/ + executionflow/bridge/；`turn/` 行 D7-S2-A06/A07 当前路径改为 `sessionorchestrator/turn_orchestrator.go` + `llm.go`（子包已合并）；`milestone/` row 删除 — 早期 v2.x 已迁入 `executionflow/workplan/`）；**(2) 登记 4 个新增归属**：`plan/`（S5 sub-registration carve-out，doc-only dual-registration 同时归属 `decisionplanning/`，**0 shim / 0 alias / 0 git mv**）+ `orchtypes/`（Cross-S Kernel，**no Go shim, no re-export, 直接 import**）+ `hardening/`（Cross-cutting Discipline Keeper namespace，**ConflictGuard 实际 owner 是 wavescheduler/**）+ `interfaces/`（TaskContract contracts pure types 包）；**(3) D7-S2-A06/A07 当前路径表 1 行**（统一归属 `sessionorchestrator/`，原 `turn/` 子包已物理合并）；**(4) cumulative version bump**：跳过 v1.12.1（该位预留给 devrix-d7-s-layer-normalization DM-20260701-002/003 — S7+ → historical-s-mapping.md 物理拆分的 code-layout 同步） |
| **1.14.0** | **2026-07-01** | **D7 Physical Layout Alignment（DM-20260701-004）PR-3 §4.2 D7 doc-only 双登记 wording 收敛**：§4.2 D7 `D7-S5 sub` 行 "Plan agent" → **"Plan Generation"** 命名收敛（与 spec.md §S5 carve-out wording 对齐 + D7-S6-A04 PlanGenerate PlanKind/DefaultPlanner 的代码定位一致）；保留 PR-1 引入的 `plan/` doc-only dual-registration 注释，与 a-registry.md `D7-S6-A03/A04` cross-reference + `## D7-S5 plan/ ↔ decisionplanning/ 双登记说明` 段构成三处一致 doc-only 登记；**0 函数签名变化 / 0 物理路径变化**（purely 命名 + wording 收敛）。 |