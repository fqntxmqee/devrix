# Proposal: D4 Multi-Agent S/A 重切 — 执行原语价值流化 + Hub-Spoke 归位 D7

**Change ID:** devrix-d4-sa-refine  
**Demand ID:** DM-20260614-018  
**Status:** S3_Design（design + gaming-analysis + tasks 草稿完成，待 S3-Gate）  
**Phase Scope:** D + S（A/F 编排在 design.md；本文件含 Hub-Spoke 边界讨论）

---

## 1. Background

D4 Multi-Agent 自 V1（2026-06-08）至 V2（2026-06-14）共 38 条 T（全 IMPLEMENTED，P0 19 条），功能完整。但 **D4 是 D1–D7 中尚未做价值流切法的核心域之一**：

| 域 | 价值流 S | 状态 |
|----|---------|------|
| D1 Communication | 6 (S13–S18) | ✅ v2.0 |
| D2 Context Engine | 6 (S15–S20) | ✅ v2.0 |
| D3 LLM Gateway | 6 (S1–S6 价值流) | ✅ v1.0 Registry |
| **D4 Multi-Agent** | **0 / 10** | ❌ **本 change** |
| D7 Orchestration | 5 (S1–S5) | ✅ v1.0 |

D4 当前 10 个 S（Factory / Agent / ForkJoin / Collaboration / Observer / AgentTool / Builtin / Observability / SessionView / Delegate）**全部为技术角色词**，且 `internal/layers/multiagent/` 子目录与之 1:1 绑定。

> 本 change 延续 `dsaft-refactoring-playbook.md` 在 D3 的首案模式，并纳入 Owner 关键输入：**Hub-Spoke 设计应归 D7，而非 D4**。

---

## 2. Problem Statement

### 2.1 价值流承诺无法对应到 S

D4 是**核心域（Delegation Execution Follower）**，对 D7 提供 5 类可验证承诺（见 demand §1.1）。当前 S 按 Go 包切分，导致：

- **隔离承诺被拆碎**：ForkJoin(S3) + SessionView(S9) + WorkerEngine(S2-A03) 实为同一承诺「并行执行不污染父 Session」
- **执行与编排混杂**：Delegate(S10) 同时含 Hub-Spoke 派发、FlowBridge、fork/run/join，与 D7 `delegatetools`/`flow` 重叠
- **观测膨胀**：Observability(S8) 在 D4 内部计数，D5 才是 Span/Metric SoT

### 2.2 Hub-Spoke 双头问题（Owner 议题）

现行代码已呈现 **Hub-Spoke 写侧分散、读侧在 D7** 的结构：

```
Leader (D7 Session Orchestrator)
    │
    ├─ delegatetools.RegisterTools     ← D7：delegate_* 路由 ✅
    │
    ├─ delegate.Service.DelegateOrFallback  ← D4：Spoke 选择 + 执行 ⚠️
    │       ├─ Fork Agent Worker (D4)
    │       └─ fallback SubQuery (D2)
    │
    ├─ FlowBridge → ExecutionFlowHub    ← D4 写侧适配 ⚠️
    │
    └─ workplan.Apply ← ExecutionFlowHub  ← D7 读侧 ✅
```

**同时存在的 Spoke 写侧（Hub-Spoke 应统一编排）：**

| Spoke 类型 | 执行体 | Flow 发布 | 调度入口 |
|-----------|--------|----------|---------|
| Delegate Worker | D4 `delegate.Service` | D4 `FlowBridge` | D7 `delegatetools` |
| SubQuery 嵌套 | D2 `nested.SubQuery` | D2 Flow hook | QueryLoop / D7 Wave |
| Wave SubAgent | D2 Background via `SubAgentRunner` | wave events → Flow | D7-S3 Scheduler |

若 Hub-Spoke「拥有权」留在 D4-S10：

1. D2 SubQuery 已是 Spoke，但规格写 D4 拥有 Hub-Spoke → **ownership 不一致**；
2. 新增 Spoke 类型需绕 D4 → **域边界扭曲**；
3. D7 已有 `delegatetools` + `flow` + `sessionqueue`，与 D4 `Service` 形成 **编排双头**。

### 2.3 跨域边界缺失

| # | 问题 |
|---|------|
| P1 | 无 `d4-d7-boundary.md`（D2 已有对称文档） |
| P2 | `cross-domain-boundaries.md` 仅 3 行 D3↔D4，无 Hub-Spoke 详表 |
| P3 | `code-layout.md` 无 D4 scenario-slug 注册（§4.4 缺失） |
| P4 | D4-S10 T（12 条 P0）跨 D4/D7/D2 分布，无统一 Canonical 归属 |

---

## 3. Proposed Solution

### 3.1 D 层（微调职责声明）

**D4 Multi-Agent** 保持核心域，职责收窄为：

> **Agent 执行原语域** — 供给实例、运行循环、隔离合并、外化子进程 Agent；**不**拥有 Hub-Spoke 编排、WorkPlan 读模型、delegate_* 路由。

**D7 Orchestration** 扩展 Hub-Spoke SoT（规格层 v1.0，代码 v2.0）：

> **Hub-Spoke 编排域** — Leader 路由、Spoke 派发矩阵、FlowEvent 聚合、进度广播；**编排** D2/D4 执行 Follower。

### 3.2 S 层 — D4 Canonical（5+1 价值流）

```
D4（Multi-Agent / Delegation Execution Follower）
├── D4-S11 ProvisionAgent       # C1：创建、配额、协作模式 prompt
├── D4-S12 RunAgentLoop         # C2：生命周期、PermissionGate、状态机
├── D4-S13 IsolateAndMerge      # C3：Fork/Join + SessionView COW + WorkerEngine overlay
├── D4-S14 ExecuteWorker        # C4：执行 Worker 实例（fork→run→join），不含 Hub-Spoke 路由
├── D4-S15 InvokeExternalAgent  # C5：CLI/Cursor Agent Tool + stream-json
└── D4-S16 ConfigureAgents      # C6：multi_agent 配置（横切）
```

**关键变更：原 D4-S10 Delegate 不再作为 D4 Scenario**

- Hub-Spoke **路由/派发/FlowBridge/异步策略** → 归入 **D7**（见 §3.4）
- D4 仅保留 **ExecuteWorker** 执行面（`forkWorker` + `Run` + `Join` 机制）

### 3.3 S ↔ 承诺 + Legacy 双轨

| 新 S | Scenario | 承诺 | 旧 S（冻结追溯） |
|------|----------|------|-----------------|
| D4-S11 | ProvisionAgent | C1 | S1 Factory + S4 Collaboration + S7 Builtin（注册面） |
| D4-S12 | RunAgentLoop | C2 | S2 Agent（lifecycle, perm_gate, state） |
| D4-S13 | IsolateAndMerge | C3 | S3 ForkJoin + S9 SessionView + S2-A03 WorkerEngine |
| D4-S14 | ExecuteWorker | C4 | S10 Delegate（**仅** fork/run/join/worktree 执行面） |
| D4-S15 | InvokeExternalAgent | C5 | S6 AgentTool |
| D4-S16 | ConfigureAgents | C6 | shared/config/multiagent.go |
| — | **Kernel** | 契约/观察者 | S5 Observer → `multiagent/kernel/` |
| — | **迁出 D5** | 可观测 | S8 Observability |

### 3.4 Hub-Spoke 全归 D7 — 规格层映射（R1: D7-1）

扩展现有 D7 Canonical S（不新增 S6）：

| Hub-Spoke 能力 | D7 S | 现行代码 | v2.0 目标（并入本 change） |
|---------------|------|---------|-------------------------|
| delegate_* 路由 | D7-S2 | `orchestration/delegatetools/` | 保持 |
| **Spoke 派发矩阵**（D4 / D2 / fallback） | D7-S2 `DispatchWorker` | `multiagent/delegate/service.go` | `orchestration/hubspoke/dispatch.go` |
| **D4 FlowBridge** | D7-S4 | `multiagent/delegate/bridge.go` | `orchestration/hubspoke/agent_bridge.go` |
| **D2 SubQuery Flow 发布** | D7-S4 | `contextengine/nested/flow_report.go` | `orchestration/hubspoke/subquery_bridge.go` |
| WorkPlan 读模型 | D7-S4 | `orchestration/workplan/` | 保持 |
| delegate-progress drain | D7-S4 | `orchestration/sessionqueue/` | 保持 |
| async 通知策略 | D7-S4 | `delegate/service.go` + sessionqueue | `orchestration/hubspoke/async.go` |
| Wave SubAgent 调度 | D7-S3 | `wave/runners/subagent.go` | 保持（经 D7 Dispatch 统一入口） |

**D4 仅保留执行契约：**

```go
// design.md 正式定义
type WorkerExecutor interface {
    ExecuteSync(ctx context.Context, leader Agent, spec WorkerRunSpec) (WorkerResult, error)
    ExecuteAsync(ctx context.Context, leader Agent, spec WorkerRunSpec) (workerID string, error)
}

// D2 仅保留嵌套执行契约（无 FlowHub 字段）
type NestedExecutor interface {
    RunNestedQuery(ctx context.Context, params NestedQueryParams) (NestedResult, error)
}
```

D7 `SpokeDispatcher` 统一：

1. 解析入口（delegate_* / Wave / 未来扩展）；
2. 选择 Spoke：`D4Worker` | `D2SubQuery` | `D2Background`；
3. 绑定对应 Bridge（agent_bridge / subquery_bridge）；
4. 调用 Executor，**唯一** `hub.Publish` 出口在 D7。

### 3.5 scenario-slug 注册表（`code-layout.md §4.4` 草案）

| S ID | scenario-slug | v2.0 目标 | 当前路径 |
|------|---------------|----------|---------|
| D4-S11 | `provision` | `multiagent/provision/` | `factory/` + `collaboration/` + `builtin/` |
| D4-S12 | `run` | `multiagent/run/` | `agent/`（lifecycle, state, perm_gate） |
| D4-S13 | `isolate` | `multiagent/isolate/` | `agent/forkjoin.go` + `sessionview/` + `worker_engine.go` |
| D4-S14 | `execute` | `multiagent/execute/` | `delegate/service.go`（瘦身後） |
| D4-S15 | `external` | `multiagent/external/` | `tool/` |
| D4-S16 | `configure` | `multiagent/configure/` | `shared/config/multiagent.go` |
| — | `kernel` | `multiagent/kernel/` | `contracts.go` + `observer/` |

**迁出清单（v2.0 并入本 change）：**

| 现行路径 | 目标 |
|---------|------|
| `multiagent/delegate/bridge.go` | `orchestration/hubspoke/agent_bridge.go` |
| `multiagent/delegate/service.go` 编排部分 | `orchestration/hubspoke/dispatch.go` |
| `contextengine/nested/flow_report.go` | `orchestration/hubspoke/subquery_bridge.go` |
| `multiagent/delegate/service.go` 执行部分 | `multiagent/execute/worker.go` |

### 3.6 T 层迁移策略

38 条 IMPLEMENTED T **不改测试代码**。`t-registry.md` 增 `canonical_s` 列：

| 类型 | 处理方式 | 示例 |
|------|---------|------|
| 纯 D4 执行 T | canonical → S11–S15 | `D4-S2-A01-T01` → `D4-S12-A01-T01` |
| Hub-Spoke 跨域 T | canonical → **D7**，D4 列 legacy | `D4-S10-A02-T09` → `D7-S4-A0x-T09` |
| CROSS T | 保持 D4-S0 或迁 CROSS 段 | race / E2E |

**Hub-Spoke 相关 T 重归属（规格层）：**

| 旧 T ID | 建议 Canonical | 理由 |
|---------|---------------|------|
| D4-S10-A01-T01~T07 | D4-S14（执行面） | fork/run/join/worktree |
| D4-S10-A02-T08~T11 | **D7-S4** | FlowEvent / delegate-progress / IM |
| D4-S10-A01-T07 (SubQuery fallback) | **D7-S2** | 派发矩阵决策 |

---

## 4. Review R1 决议与讨论

### 4.1 Decision D4：Hub-Spoke — ✅ D7-1 全归 D7

Owner 否决折中方案。Hub-Spoke **编排语义、Spoke 桥接、Flow 发布** 全部归 D7；D4/D2 仅保留无 Hub 依赖的纯执行 Follower。

| 域 | v2.0 后职责 |
|----|------------|
| **D7** | Dispatch、SpokeBridge（agent + subquery）、WorkPlan、sessionqueue、delegatetools |
| **D4** | Provision / RunAgentLoop / Isolate / **ExecuteWorker** / External / Configure |
| **D2** | Prepare / QueryLoop / Persist / Policy / **RunNestedQuery**（无 Publish） |

### 4.2 Decision D5：D2 SubQuery Flow — ✅ 迁 D7

`nested/flow_report.go` 的 `publishSubQueryFlow` 与 `subQueryFlowEmit` 是 Hub-Spoke **写侧适配**，与 D4 `FlowBridge` 同质，应统一在 D7 `hubspoke/`。

迁后 D2 `SubQueryParams` **删除 `FlowHub` 字段**；D7 `SubQueryBridge` 包装执行并发布 FlowEvent。

### 4.3 Decision D7：v2.0 范围 — ✅ 并入本 change

不另开 `devrix-d7-hubspoke-consolidate`。v2.0 slice 顺序（design.md tasks 细化）：

```
v2.0-a  D7 hubspoke/ 骨架 + 统一 SpokeBridge 接口
v2.0-b  迁 D4 bridge + dispatch；删 D4 Hub 依赖
v2.0-c  迁 D2 flow_report；D2 SubQuery 去 FlowHub
v2.0-d  D4 物理路径 scenario-slug 迁移
v2.0-e  关联 T 全绿 + 1 周期 re-export 清理
```

**依赖**：v2.0-a 必须先于 b/c；D4 物理迁移 d 可与 b/c 并行（不同包）。

### 4.4 Decision D6：D4-S14 命名 — ✅ ExecuteWorker

| 候选 | 优点 | 缺点 |
|------|------|------|
| **ExecuteWorker** ✅ | 与 D2 `RunQueryLoop` 同型（动词+名词）；不泄漏 D7「委派」词汇；slug `execute` 简洁 | 略泛（但 D4 域上下文足够消歧） |
| RunDelegatedWorker | 强调 Leader→Follower 关系 | **委派语义应留在 D7**（R1 D7-1）；D4 S 层重复 Hub-Spoke 词汇；slug 过长 |

**架构师理由（推荐 ExecuteWorker）：**

1. **R1 D7-1 一致性**：Hub-Spoke 全归 D7 后，「Delegated」是 D7 `DispatchWorker` 的词汇，不应下沉到 D4 S 名。
2. **对称命名**：D4-S12 `RunAgentLoop`（根 Agent 主循环）vs D4-S14 `ExecuteWorker`（Worker 实例执行）— 二者都是「执行」，区别在 D7 派发上下文，不在 D4 S 名。
3. **Playbook 原则 1**：S 回答可验证承诺 — D4 的承诺是「给定 WorkerSpec，我能 fork→run→join 并返回结果」，不是「我处理委派决策」。
4. **Follower 纯度**：D2 叫 `RunNestedQuery` 而非 `RunDelegatedSubQuery`；D4 应同样避免 Leader 词汇污染。

### 4.5 D7-1 终态调用链

```
D1 → D7.ProcessMessage
        └─ D7-S2 DispatchWorker
                ├─ delegatetools (Leader tool 入口)
                ├─ SpokeDispatcher
                │     ├─ D4: WorkerExecutor.Execute  ─┐
                │     └─ D2: NestedExecutor.Run       ─┤ 纯执行，无 Publish
                └─ hubspoke.SpokeBridge.Publish ◀──────┘ 唯一 Flow 出口
                        └─ D7-S4 ExecutionFlowHub → WorkPlan → D1 IM
```

---

## 5. Success Metrics

| 指标 | 基线 | v1.0 目标 | v2.0 目标 |
|------|------|----------|----------|
| D4 价值流 S 数 | 0/10 | 5+1 | 5+1 |
| Hub-Spoke 规格 SoT | 分散 D4/D7 | **D7 单点 SoT** | 代码对齐 |
| scenario-slug 语义化 | 0/10 | 6/6 + kernel | 物理 1:1 |
| P0 T 全绿 | 19 | 19（保持） | 19（保持） |
| T 总数 | 38 | 38 + canonical 列 | 38 |
| D4 含 Hub-Spoke 编排 S 数 | 1 (S10) | **0** | 0 |
| cross-domain 文档 | 缺 d4-d7 | ✅ 双向引用 | 持续 |

---

## 6. Implementation Plan（Phase 概要，不估时）

### Phase A — S1→S2 澄清

- `demand.md`（本包）
- `proposal.md`（本文件）
- Owner R1：Decision D4 Hub-Spoke 决议

### Phase B — v1.0 Registry

- D4 `spec.md` / 4 注册表 Canonical 重排
- `d4-domain.md` + `d4-d7-boundary.md`
- `layering.md` §D4 双轨 + §D7 Hub-Spoke 扩展
- `code-layout.md §4.4`
- `cross-domain-boundaries.md` §D4↔D7

### Phase C — S3 design + S3-Gate

- `design.md`：A/F 编排 + Decision 表 + Grill Review
- `gaming-analysis.md`（推荐）
- Gherkin sad path（Worker 不能 delegate、Hub-Spoke 双头防护）

### Phase D — v1.0 验收

- 38 T 追溯表 100% 覆盖
- acceptance-report（v1.0）
- **零 Go 变更**

### Phase E — v2.0（并入本 change，非独立 D7 change）

- v2.0-a~e slice（见 §4.3）
- D7 `hubspoke/` + D4 `execute/` + D2 `nested/` 去 FlowHub
- D4 scenario-slug 物理迁移
- 关联 T 全绿 + re-export 1 周期

---

## 7. Out of Scope（本 change v1.0）

- Go 代码移动
- 修改已有 `// T:` 测试注释
- D6 probe 实现（v1.1）

**v2.0 范围（已纳入本 change，非 Out of Scope）：**

- D7 `hubspoke/` 收敛（含 D2 flow_report + D4 bridge/dispatch）
- D4 物理路径迁移
- D2 `d7-boundary.md` 补丁（SubQuery Flow 迁出声明）

---

## 8. R1 决议记录（2026-06-14）

| # | 议题 | Owner 结论 |
|---|------|-----------|
| 1 | Hub-Spoke 归属 | **D7-1 全归 D7** |
| 2 | D2 SubQuery Spoke 面 | **迁 D7**（`flow_report.go`） |
| 3 | D4-S14 命名 | **ExecuteWorker** |
| 4 | v2.0 迁移 | **并入本 change**，slice a→e |

---

## 9. 相关文档

| 文档 | 用途 |
|------|------|
| `docs/methodology/dsaft-refactoring-playbook.md` | 方法论 SoT |
| `openspec/changes/devrix-d3-sa-refine/proposal.md` | 同型样板 |
| `openspec/specs/d2-context-engine/d7-boundary.md` | 边界文档样板 |
| `openspec/specs/d7-orchestration/design.md` §Hub-Spoke 读模型 | 现行 Hub 设计 |
| `internal/layers/orchestration/delegatetools/` | 已迁 D7 的路由面 |
