# Devrix 代码路径布局规范（Domain / Scenario）

**Capability:** architecture-code-layout  
**Status:** Active  
**Version:** 1.4.0
**Last Updated:** 2026-06-14
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

| 层级 | 路径段 | 规则 | 示例 |
|------|--------|------|------|
| L1 领域 | `{domain-slug}/` | 全小写复合词，无下划线；与 `layering.md` D 表一致 | `communication`, `contextengine`, `orchestration` |
| L2 场景 | `{scenario-slug}/` | **Go 合法目录名**（同 §2.2）；在本文 §4 登记，**禁止自造** | `capture`, `thinking`, `taskprogress` |
| L3 活动 | `{activity-slug}/` | **可选**；仅当单 S 下 A 组 >1 且需物理隔离时使用；同样遵守 Go 目录名规则 | `dispatch`, `encode/feishu` |
| 域内核 | `kernel/` | 非 S 的共享模型（Card、Session 值对象等） | `communication/kernel/` |
| 跨域契约 | — | `internal/shared/contracts/` | 禁止域内 duplicated 契约 |

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
2. 属于哪个 L1 领域 D？
   → internal/layers/{domain-slug}/
3. 属于哪个 L2 场景 S？（查 §4 注册表）
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

| S ID | Scenario | scenario-slug | 目标路径 | 职责摘要 |
|------|----------|---------------|----------|----------|
| D1-S13 | CaptureUserIntent | `capture` | `communication/capture/` | 入站、Persist、Dispatch、Permission、Command |
| D1-S14 | PresentThinking | `thinking` | `communication/thinking/` | Thinking 信号映射与 emit |
| D1-S15 | PresentTaskProgress | `taskprogress` | `communication/taskprogress/` | Task/Tool/Worker/Milestone **展示** |
| D1-S16 | DeliverConclusion | `conclusion` | `communication/conclusion/` | Conclusion 流式/终态/摘要 |
| D1-S17 | ConnectChannel | `channel` | `communication/channel/` | IM 适配、连接、实例、限流、Encode |
| D1-S18 | GuaranteeDelivery | `delivery` | `communication/delivery/` | EventBus、Critical、Drain |
| — | Domain Kernel | `kernel` | `communication/kernel/` | Card、平台无关消息模型 |

**横切（暂存，随迁移收敛）：**

| 组件 | 当前路径 | 收敛目标 |
|------|----------|----------|
| Turn tracker / 信号锚点 | `communication/capture/signal/` | ✅ 已收敛 |
| 契约映射 | `shared/contracts/im_outbound_signal.go` | 保持 shared |

### 4.2 D7 Orchestration

> D7 待迁移目录须遵守 §2.2（无下划线）。DSAFT 场景名与 slug 映射示例：`Wave Scheduler` → `wavescheduler/`。

| S ID | Scenario | scenario-slug | 目标路径 | 当前路径（迁移中） |
|------|----------|---------------|----------|-------------------|
| D7-S1 | Work Model | `workmodel` | `orchestration/workmodel/` | ✅ DM-012 |
| D7-S2 | Session Orchestrator | `sessionorchestrator` | `orchestration/sessionorchestrator/` | `orchestration/coordinator/` |
| D7-S3 | Wave Scheduler | `wavescheduler` | `orchestration/wavescheduler/` | `orchestration/wave/` |
| D7-S4 | Execution Flow | `executionflow` | `orchestration/executionflow/` | `flow/`, `workplan/`, `imsink/` |
| D7-S5 | Decision & Planning | `decisionplanning` | `orchestration/decisionplanning/` | `coordinator/classifier*` |
| — | Worker tool policy F | `toolpolicy` | `orchestration/toolpolicy/` | ✅ DM-015 |
| — | Delegate routing F | `delegatetools` | `orchestration/delegatetools/` | ✅ DM-011 |
| — | Session command queue F | `sessionqueue` | `orchestration/sessionqueue/` | ✅ DM-013 |
| — | Milestone DAG | `milestone` | `orchestration/milestone/` | ✅ 已迁入 |

### 4.3 D2 Context Engine

> **Canonical S15–S20**（DM-20260614-009）。Legacy module 路径仍有效；v2.0 按 scenario-slug 收敛。

| S ID | Scenario | scenario-slug | 当前路径 | v2.0 目标 |
|------|----------|---------------|----------|-----------|
| D2-S15 | PrepareExecutionContext | `prepare` | `prepare/memory/` `prepare/compression/` `prepare/prompt/` `prepare/conversation/` | ✅ DM-014 |
| D2-S16 | RunQueryLoop | `query` | `contextengine/query/` | 保持（loop 瘦身） |
| D2-S17 | PersistSessionState | `persist` | `persist/snapshot/`, `persist/transcript/` | ✅ DM-014 |
| D2-S18 | EnforceExecutionPolicy | `policy` | `policy/permission/`, `policy/toolrunner/` | ✅ DM-014 |
| D2-S19 | NestedExecution | `nested` | `nested/subquery.go`, `nested/background.go`, `nested/fork.go` | ✅ DM-014 |
| D2-S20 | LegacyHarnessFallback | `legacyharness` | `harness/` | 保持或 `legacy/` |

**Legacy module 路径（冻结追溯）：**

| Legacy S | scenario-slug | 路径 |
|----------|---------------|------|
| D2-S2 | `compression` | ~~`contextengine/compression/`~~ → `contextengine/prepare/compression/` | ✅ DM-014 |
| D2-S3 | `memory` | ~~`contextengine/memory/`~~ → `contextengine/prepare/memory/` | ✅ DM-014 |
| D2-S11 | `queue` | ~~`contextengine/queue/`~~ → `orchestration/sessionqueue/` (D7-S4) | ✅ DM-013 |
| D2-S12 | `worktree` | `contextengine/worktree/` |

### 4.4 D3 LLM Gateway（canonical S1–S6 / 5+1 价值流）

> **Canonical S1–S6**（DM-20260614-016 / devrix-d3-sa-refine / DM-20260614-019 v2.0）。
> v1.0 注册表重排，0 行为变更（已 ACCEPTED commit 199ad18）。
> **v1.1** 韧性可见性 + D6 3 probe + IAdapter.Protocol() + obs nil fail-fast（已 ACCEPTED commit 3a6970b）。
> **v2.0** 物理路径按 scenario-slug 迁移（DM-20260614-019 / devrix-d3-sa-refine-v2.0 **ACCEPTED** commit d222328）。
> 详见 `openspec/archive/2026-06-14-devrix-d3-sa-refine-v2.0/acceptance-report.md`。

| S ID | Scenario | scenario-slug (v2.0) | v1.0 当前路径 | v2.0 目标 | 状态 |
|------|----------|----------------------|--------------|-----------|------|
| D3-S1 | RouteModel | `route` | `gateway/router.go` (路由解析部分) | `llmgateway/route/` | ✅ 完成 |
| D3-S2 | StreamChat | `stream` | `adapter/` (全部) + `gateway/gateway.go` Stream 主实现 | `llmgateway/stream/`（含 `stream/adapter/` 子目录） | ✅ 完成 |
| D3-S3 | ProtectCall | `protect` | `breaker/` + `retry/` + `gateway/breaker_observer.go` | `llmgateway/protect/`（两机制独立 .go） | ✅ 完成 |
| D3-S4 | BudgetTokens | `budget` | `token/` | `llmgateway/budget/` | ✅ 完成 |
| D3-S5 | GuardContent | `guard` | `safety/` | `llmgateway/guard/` | ✅ 完成 |
| D3-S6 | ConfigureGateway | `configure` | `config/` + `shared/config/llmgateway.go` | `llmgateway/configure/`（合并 shared） | ✅ 完成 |
| — | Domain Kernel | (根 `contracts.go` 拆分后 < 200 行) | `llmgateway/contracts.go` | `llmgateway/contracts.go` (145 行) | ✅ AC-09 达成 |
| D3-X | CROSS 跨域锚点 | (Bridge 不变) | `internal/bridges/llm/` | `internal/bridges/llm/` | ✅ 不动 |

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

| 组件 | 当前路径 | 目标 | 状态 |
|------|----------|------|------|
| ~~delegate_tools~~ | ~~`contextengine/delegate_tools.go`~~ | `orchestration/delegatetools/` | ✅ DM-011 |
| TaskManager | ~~`contextengine/tasks/`~~ | `orchestration/workmodel/` (D7-S1) | ✅ DM-012 |
| queue delegate-progress | ~~`contextengine/queue/`~~ | D7-S4 `sessionqueue/` | ✅ DM-013 |

### 4.5 D4 Multi-Agent（canonical S11–S16 / 5+1 价值流）

> **Canonical S11–S16**（DM-20260614-018 / devrix-d4-sa-refine）。v1.0 物理路径保留；Hub-Spoke 编排代码 v2.0 迁 D7 `hubspoke/`。

| S ID | Scenario | scenario-slug (v2.0) | v1.0 当前路径 | v2.0 目标 | 状态 |
|------|----------|----------------------|--------------|-----------|------|
| D4-S11 | ProvisionAgent | `provision` | `factory/` + `collaboration/` + `builtin/` | `multiagent/provision/` | ⬜ v2.0-d |
| D4-S12 | RunAgentLoop | `run` | `agent/`（lifecycle, state, perm_gate） | `multiagent/run/` | ⬜ v2.0-d |
| D4-S13 | IsolateAndMerge | `isolate` | `agent/forkjoin.go` + `sessionview/` + `worker_engine.go` | `multiagent/isolate/` | ⬜ v2.0-d |
| D4-S14 | ExecuteWorker | `execute` | `delegate/service.go` | `multiagent/execute/` | ⬜ v2.0-d |
| D4-S15 | InvokeExternalAgent | `external` | `tool/` | `multiagent/external/` | ⬜ v2.0-d |
| D4-S16 | ConfigureAgents | `configure` | `shared/config/multiagent.go` | `multiagent/configure/` | ⬜ v2.0-d |
| — | Domain Kernel | `kernel` | `contracts.go` + `observer/` | `multiagent/kernel/` | ⬜ v2.0-d |

**Hub-Spoke 迁 D7（v2.0-b/c，非 D4 目录）：**

| 组件 | v1.0 路径 | v2.0 目标 |
|------|----------|-----------|
| Agent FlowBridge | `multiagent/delegate/bridge.go` | `orchestration/hubspoke/agent_bridge.go` |
| Dispatch / fallback | `multiagent/delegate/service.go`（编排部分） | `orchestration/hubspoke/dispatch.go` |
| SubQuery Flow | `contextengine/nested/flow_report.go` | `orchestration/hubspoke/subquery_bridge.go` |

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

| 当前路径 | 目标 scenario-slug | 迁移状态 | 关联 Change |
|----------|-------------------|----------|-------------|
| `gateway/` | `capture/` + 部分 `conclusion/` | ✅ IMPLEMENTED | DM-20260614-006 |
| `present/` | `thinking/` `taskprogress/` `conclusion/` | ✅ IMPLEMENTED | DM-20260614-006 |
| `signal/` | `capture/signal/` | ✅ IMPLEMENTED | DM-20260614-006 |
| `adapters/` … `renderers/` | `channel/` | ✅ IMPLEMENTED | DM-20260614-006 |
| `eventbus/` | `delivery/eventbus/` | ✅ IMPLEMENTED | DM-20260614-006 |
| `core/` | `kernel/` | ✅ IMPLEMENTED | DM-20260614-006 |
| `communication/milestone/` | — | ✅ 已迁至 `orchestration/milestone/` | DM-20260614-006 |

**迁移原则：**

1. **新代码** 必须写入 §4 登记的 **目标 scenario-slug**；禁止在新 `gateway/` 下追加 F。
2. **旧路径** 允许 `re-export` / type alias 一个发布周期，PR 标注 `BREAKING` 与迁移表。
3. 每个迁移 PR 更新：`code-layout.md` 迁移状态、`layering.md` Package Map、`d1-communication/spec.md`。
4. 单 PR 仅迁 **一个 scenario-slug**（或一组强耦合 F），并跑关联 L5/T。

---

## 7. 与 OpenSpec 的对应

| 代码 | 规格 |
|------|------|
| `internal/layers/{domain}/` | `openspec/specs/{domain}-*/spec.md` |
| scenario-slug | `layering.md` S 表 + 域 `a-registry.md` / `f-registry.md` |
| `*_test.go` 内 `// T:` | `openspec/specs/{domain}/t-registry.md` |
| span 名 | `{domain}/span-registry.md` |

验收/归档时 **代码路径与 spec 必须同步更新**（见 `specs/05-delivery-process.md` §6.3、§8.2）。

---

## 8. 变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0.0 | 2026-06-14 | 初版：D1/D7 scenario-slug 注册表 + D1 迁移矩阵 |
| 1.1.0 | 2026-06-14 | D1 物理路径迁移完成（capture/channel/delivery/kernel） |
| 1.3.0 | 2026-06-14 | D2 S15–S20 Canonical；delegate_tools → delegatetools (DM-011) |
| 1.4.0 | 2026-06-14 | **D3 S1–S6 Canonical**（DM-20260614-016 / devrix-d3-sa-refine）：5+1 价值流 scenario-slug 注册表（`route` `stream` `protect` `budget` `guard` `configure`）；v1.0 物理路径保留 + v2.0 迁移目标映射；D3-X 跨域锚点声明 `internal/bridges/llm/`；contracts.go 拆分粒度占位 |
| **1.5.0** | **2026-06-14** | **D3 S1–S6 v2.0 物理迁移状态**（DM-20260614-019 / devrix-d3-sa-refine-v2.0 ACCEPTED commit d222328）：6 个 slug 全部 ✅ 完成（含 `stream/adapter/` 子目录 + `configure/` 跨包合并 shared/config）；D3-X 跨域锚点 ✅ 不动；contracts.go 145 行 AC-09 达成 |
