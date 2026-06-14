---
demand-id: DM-20260614-018
title: D4 Multi-Agent — 执行原语价值流重构与 Hub-Spoke 归位 D7
source: 架构审计（D4 S 切法为技术角色词；Hub-Spoke 编排语义散落在 D4/D2/D7；code-layout 缺 D4 注册）
priority: P0
status: S5_Accepted
review-round: R1
dsaft_domain: multi-agent
created: 2026-06-14
parent: dsaft-refactoring-playbook
related:
  - DM-20260614-009  # D2 SA Refine
  - DM-20260614-008  # D7 SA Refine
  - DM-20260614-016  # D3 SA Refine
  - DM-20260614-011  # delegate_tools → D7 delegatetools
  - DM-20260614-020  # D7 Turn 编排上移（因果链：DM-020 → DM-018，互补资产 + 双重边际化）
bilateral-consensus: ../devrix-d7-turn-orchestration/gaming-analysis-bilateral-consensus.md  # G-01~G-12 全部确认；P1~P3 闭合
---

# D4 Multi-Agent — 执行原语价值流重构与 Hub-Spoke 归位 D7

## 0. Review R1 决议（2026-06-14）

| Decision | Owner 结论 | 影响范围 |
|---------|-----------|---------|
| **D4: Hub-Spoke 归属** | ✅ **D7-1 全归 D7** | D4 删除 Hub-Spoke 编排 S；D7 统一 Dispatch + FlowBridge + WorkPlan + sessionqueue |
| **D5: D2 SubQuery Spoke 面** | ✅ **D2 Flow 发布迁 D7** | `nested/flow_report.go` 等 Spoke→FlowEvent 适配迁 D7；D2-S19 仅保留嵌套 QueryLoop 执行机制 |
| **D6: D4-S14 命名** | ✅ **ExecuteWorker** | 见 proposal §4.5 架构师理由；scenario-slug = `execute` |
| **D7: v2.0 迁移范围** | ✅ **并入本 change v2.0** | 不另开 `devrix-d7-hubspoke-consolidate`；D4 v2.0 slice 含 D7 Hub-Spoke 代码收敛 + D2 flow_report 迁出 |
| **D1: S 切法** | ✅ A: 5+1 价值流 | D4-S11–S16 |
| **D2: S8 Observability** | ✅ 迁 D5 | D4 仅保留 emit hook |
| **D3: 新 S 号段** | ✅ S11–S16 | Legacy S1–S10 冻结 |

**R1 关键澄清：**

| # | 议题 | 决议 |
|---|------|------|
| Q1 | Hub-Spoke 是否折中留 D4 执行编排？ | **否**。全归 D7；D4 只暴露 `WorkerExecutor` 执行契约 |
| Q2 | D2 SubQuery 是否纳入 Hub-Spoke 统一？ | **是**。D2 不再直接 `hub.Publish`；由 D7 SpokeBridge 包装 SubQuery 执行并发布 FlowEvent |
| Q3 | v2.0 是否独立 D7 change？ | **否**。与 D4 物理路径迁移同一 v2.0 交付，按 slice 顺序执行 |

---

## 1. 背景

### 1.1 D4 根本目标（North Star 草案）

**在 D7 给定 Worker 派发参数后，可靠地供给 Agent 实例、执行隔离的子任务循环、合并结果——作为 Delegation Execution Follower，不承担 Hub-Spoke 编排决策与进度读模型聚合。**

用户 / 系统侧可验证承诺：

| # | 承诺 | 验收主体 |
|---|------|---------|
| C1 | **供给**：按配额创建 Agent/Worker，注入协作模式与 sidechain 上下文 | Factory / Provision T |
| C2 | **执行**：Agent 主循环可取消，CRITICAL 工具阻塞等权限 | RunAgentLoop + D1 集成 T |
| C3 | **隔离**：Fork/Delegate 执行不污染父 Session（COW + worktree + overlay） | Isolate P0 T |
| C4 | **执行 Worker**：接到 D7 派发的 WorkerSpec 后 fork→run→join，返回结果 | ExecuteWorker T（**非** Hub-Spoke 路由） |
| C5 | **外化**：CLI/Cursor 外部 Agent Tool 可调用、Session 级隔离 | AgentTool P0 T |
| C6 | **配置**（横切）：`multi_agent.*` 策略可配置 | ConfigureAgents |

**Out of Scope（不归 D4）：**

| 能力 | 归属 |
|------|------|
| Hub-Spoke 路由 / delegate_* 工具注册 | **D7** `delegatetools/` |
| WorkPlan 读模型 / FlowEvent 聚合 | **D7-S4** `flow/` + `workplan/` |
| delegate-progress drain / async 通知策略 | **D7-S4** `sessionqueue/` |
| Wave DAG 调度 / WorkerPool | **D7-S3** |
| SubQuery 嵌套执行机制 | **D2-S19** |
| 权限 UI | **D1** |
| Span/Metric 基础设施 | **D5** |
| 委派质量 Judge | **D6** |

### 1.2 现状问题

| 问题 | 根因 |
|------|------|
| D4-S1–S10 按 **Go 子包** 切 S，非价值流 | S 被目录结构绑架（Playbook 原则 1） |
| D4-S10 命名与实现承载 **Hub-Spoke 编排语义** | 与 D7 `delegatetools`/`flow` 职责重叠；D2 SubQuery 已是另一 Spoke 写侧，但规格未统一 |
| S3+S9+S2-A03 同属「隔离执行」承诺 | 按机制拆成三个 S，错误归因困难 |
| S8 Observability 在 D4 内部 | 与 D5 重复；Playbook 原则 4 跨域应在 D 边界决策 |
| `code-layout.md §4` 无 D4 scenario-slug | 物理路径无注册锚点 |
| 无 `d4-domain.md` / `d4-d7-boundary.md` | D2 已有对称文档，D4 缺失 |

### 1.3 D4 博弈定位

> **D4 = Delegation Execution Follower（Stackelberg Follower）**：D7 Leader 决定「派谁、何时、走哪条 Spoke」；D4 保证 Worker Agent 实例的**机制正确**（创建、隔离、执行、合并）；**不**保证任务结构正确（归 D7）或结论质量（归 D6）。

| D4 承诺 | 博弈含义 |
|---------|----------|
| Fork COW 不污染父 Session | costly signal（隔离才可合并） |
| Worker 不能 delegate_* / Fork | 防 Follower 僭越 Leader |
| PermissionGate 阻塞 CRITICAL | 机制约束，非质量评判 |

---

## 2. 问题陈述（Playbook 四轴 Review）

### 轴 ① DSAFT 分层合规

| DSAFT 原则 | 体检结果 | 证据 |
|----------|---------|------|
| S = 价值流 | ❌ 10/10 技术角色词 | `spec.md` Scenarios 表 |
| scenario-slug 语义化 | ❌ 目录 1:1 绑定 | `factory/` `agent/` `delegate/` … |
| 跨域边界 | ❌ Hub-Spoke 编排语义在 D4-S10 | `delegate/service.go` + `bridge.go` vs D7 `flow/` |
| T 锚点 | ✅ 38 条 IMPLEMENTED | `t-registry.md` |
| 注册表一致 | ⚠️ layering 与 spec 同步，但缺 code-layout §4.4 | — |

**系统性根因：**

1. V1/V2 按包增量加 S，无价值流约束；
2. Hub-Spoke v2 引入时把 **编排面（Service/FlowBridge）** 与 **执行面（Agent Fork/Run）** 同归入 D4-S10；
3. D7 v2.0 已迁 `delegatetools`，但 D4 `delegate/` 仍保留编排语义，形成 **双 Hub** 认知负担。

### 轴 ② 用户动线

D4 的直接「用户」是 **D7**（派发 Worker）与 **D1**（权限注入）。终端用户通过 D7→D1 间接验收。

| 承诺 | 现状 | S 切法问题 |
|------|------|-----------|
| Leader 委派子任务，进度可见 | ✅ FlowEvent→IM 有 P0 T | 写侧分散在 D4 bridge + D2 SubQuery，读侧在 D7，**无统一 Hub-Spoke 规格 ownership** |
| Fork 不污染父 Session | ✅ P0 T | S3+S9 拆分 |
| Worker 执行隔离 | ✅ worktree + sidechain T | 埋在 S10，与 D7 Wave SubAgent 路径未对齐 |

### 轴 ③ 博弈论

| 参与者 | 局部最优 | 全局错配 |
|--------|---------|---------|
| D4 作者 | 改 `delegate/service.go` 一处搞定委派 | Hub-Spoke 路由应归 D7，否则 D2 SubQuery Spoke 需重复编排逻辑 |
| D7 作者 | 在 `delegatetools` 加路由 | 仍须调 D4 Service，边界模糊 |
| D2 作者 | SubQuery 自带 Flow 发布 | 与 D4 FlowBridge 两套写侧，WorkPlan 读侧只在 D7 |

### 轴 ④ OpenSpec 交付

- D4 六件套齐备，但 **无 change 包**、无 Legacy 双轨、无 D4↔D7 边界 SoT；
- Hub-Spoke 讨论需写入 `proposal.md` Decision + 可选 `gaming-analysis.md`；
- v1.0 约束：**T 锚定前不改 Go 代码**。

---

## 3. Hub-Spoke 边界讨论（Owner 议题展开）

### 3.1 现行三路 Spoke 写侧（代码事实）

```
                    ┌─────────────────────────────────────┐
                    │  D7 Hub-Spoke（读侧 + 部分写侧路由）   │
                    │  delegatetools / flow / workplan     │
                    │  sessionqueue / wave scheduler       │
                    └──────────────┬──────────────────────┘
                                   │ dispatch
           ┌───────────────────────┼───────────────────────┐
           ▼                       ▼                       ▼
    D4 delegate.Service     D2 nested.SubQuery      D7 wave.SubAgentRunner
    (Fork Agent Worker)     (嵌套 QueryLoop)         (调度 D2 Background)
           │                       │                       │
           └──────── FlowEvent Publish ────────────────────┘
                                   ▼
                         D7 ExecutionFlowHub → WorkPlan → D1 IM
```

| 组件 | 现行位置 | 博弈角色 |
|------|---------|---------|
| `delegate_explore/plan/implement` 工具 | D7 `delegatetools/` | Leader 路由决策 ✅ |
| `DelegateOrFallback` 派发矩阵 | D4 `delegate/service.go` | **应在 D7**（选 Spoke：D4 Worker vs D2 SubQuery） |
| `FlowBridge` 事件映射 | D4 `delegate/bridge.go` | **应在 D7-S4**（统一 Spoke→FlowEvent 适配） |
| `forkWorker` + `Join` | D4 `delegate/service.go` | 留 D4（执行原语） |
| SubQuery Flow 发布 | D2-S19 | 写侧 Spoke，经 D7 Hub 聚合 |
| WorkPlan 读模型 | D7 `workplan/` | D7 SoT ✅ |

### 3.2 Owner 观点的架构含义

> Hub-Spoke 是 **跨执行后端的编排模式**，不是「多 Agent 域专属」。任何需要 Leader→Worker→进度广播的路径（D4 Agent Worker、D2 SubQuery、未来 Wave 外部 Runner）都应接入 **同一 D7 Hub**。

若 Hub-Spoke 留在 D4-S10：

- D2 SubQuery 已是 Spoke，但规格上 D4「拥有」Hub-Spoke 名称 → **ownership 谎言**；
- 新增 Spoke（如外部 `cursor`/`claude_code` Wave Runner）需绕 D4 → **域边界扭曲**；
- D7 `delegatetools` 已存在，与 D4 `Service` 形成 **编排双头**。

### 3.3 R1 决议：D7-1 全归 D7（编排 + Spoke 桥接 + D2 Flow 发布）

| 层 | D7（Hub-Spoke 唯一 SoT） | D4（Execution Follower） | D2（Nested Follower） |
|----|--------------------------|--------------------------|----------------------|
| **S** | D7-S2 `DispatchWorker` + D7-S4 `AggregateFlow` + SpokeBridge | D4-S14 `ExecuteWorker` | D2-S19 仅 `RunNestedQuery` |
| **A** | 路由、Spoke 选择、async、**统一 FlowBridge**、SubQuery Flow 包装 | fork/run/join/worktree | 嵌套 QueryLoop 机制 |
| **F** | `hubspoke/dispatch`、`executionflow/bridge`、`delegatetools` | `execute/worker.go` | `nested/subquery.go`（无 Publish） |
| **契约** | `SpokeDispatcher.Dispatch(spec)` | `WorkerExecutor.Execute` | `NestedExecutor.Run` |

**v1.0**：规格重划 + Legacy 双轨 + 跨域边界文档，**零 Go 变更**。  
**v2.0（并入本 change，按 slice 顺序）**：

| Slice | 迁出 | 迁入 D7 |
|-------|------|---------|
| v2.0-a | D4 `delegate/bridge.go`、Dispatch 矩阵 | `orchestration/hubspoke/` |
| v2.0-b | D2 `nested/flow_report.go` | `orchestration/hubspoke/subquery_bridge.go` |
| v2.0-c | D4 `delegate/service.go` 编排逻辑 | 瘦身为 `multiagent/execute/` |
| v2.0-d | D4 scenario-slug 物理路径 | `provision/run/isolate/execute/external/` |

### 3.4 D2 SubQuery 迁 D7（R1 Q2 闭合）

Owner 确认：原「D3 调 SubAgent」为 **D2 SubQuery** 笔误；且 **D2 的 Hub-Spoke 写侧面也应迁 D7**。

现行 D2 越界代码（v2.0-b 迁出清单）：

| 路径 | 行为 | v2.0 目标 |
|------|------|----------|
| `contextengine/nested/flow_report.go` | `publishSubQueryFlow` / `subQueryFlowEmit` | `orchestration/hubspoke/subquery_bridge.go` |
| `SubQueryParams.FlowHub` | SubQuery 直连 Hub | D7 注入 SpokeBridge，D2 不持有 Hub 引用 |

迁后调用链：

```
D7 DispatchWorker
    ├─ Spoke=D4Worker → WorkerExecutor.Execute → (内部 D2 QueryLoop)
    └─ Spoke=D2SubQuery → SubQueryBridge.Run → nested.RunNestedQuery (纯执行)
            └─ SpokeBridge.Publish(FlowEvent)  ← 统一 D7 出口
```

D2-S19 Canonical 承诺收窄为：**在给定 depth/overlay 下完成嵌套 QueryLoop**；Flow 发布不再是 D2 职责。

---

## 4. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | D4 Canonical S = **S11–S16**（5+1）；旧 S1–S10 Legacy 冻结 | P0 |
| AC2 | North Star + Out of Scope + **Hub-Spoke 归 D7 声明** 在 proposal/design 显式写出 | P0 |
| AC3 | Decision D4（Hub-Spoke 归属）经 Owner R1 确认并写入 design.md | P0 |
| AC4 | 每个 Canonical S ≥1 Gherkin Scenario | P0 |
| AC5 | 38 条 IMPLEMENTED T 通过 canonical→legacy 列追溯；v1.0 不改 `// T:` 注释 | P0 |
| AC6 | `d4-domain.md` + `d4-d7-boundary.md` 创建并与 D7 `d7-domain.md` 双向引用 | P0 |
| AC7 | `code-layout.md §4.4` 补 D4 scenario-slug | P1 |
| AC8 | S3-Gate Approved；**v1.0 无 Go 代码变更** | P0 |
| AC9 | Hub-Spoke v2.0 迁移清单登记（含 D4 bridge/dispatch + D2 flow_report → D7 hubspoke） | P0 |
| AC10 | v2.0 并入本 change，不另开 D7 change；slice 顺序在 design.md tasks 定义 | P0 |

### 分阶段终态

| 版本 | 范围 | 风险 |
|------|------|------|
| v1.0 Registry | S11–S16 + Legacy 双轨 + Hub-Spoke 边界规格 + Gherkin | 低 |
| v1.1 Traceability | Span 归 D5；D6 增 3 probe；Hub-Spoke 写侧 span 统一 `orchestration.flow.*` | 中 |
| v2.0 Structure | D4 scenario-slug 物理迁移 + Hub-Spoke 编排代码迁 D7 | 高 |

---

## 5. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | `dsaft-refactoring-playbook.md`、`DM-20260614-009` D2 边界、`DM-20260614-008` D7 Leader 模型 |
| 依赖 | `DM-20260614-011` delegatetools 已迁 D7（Hub-Spoke 路由前置） |
| 约束 | 新 S 号段 **D4-S11–S16**；旧 S1–S10 不重定义语义 |
| 约束 | v1.0 registry-only；Hub-Spoke 代码迁移放 v2.0 |
| 约束 | 38 条 T 全绿方可启动 v2.0 物理迁移 |

---

## 6. 变更范围

### 新增（v1.0）

- `openspec/changes/devrix-d4-sa-refine/{proposal,design,gaming-analysis}.md`
- `openspec/specs/d4-multi-agent/d4-domain.md`
- `openspec/specs/d4-multi-agent/d7-boundary.md`
- `code-layout.md §4.4` D4 scenario-slug 表

### 修改（v1.0）

- `openspec/specs/d4-multi-agent/{spec,a,f,t}-registry.md` Canonical 列 + Legacy Archive
- `openspec/specs/architecture/layering.md` §D4 双轨表
- `cross-domain-boundaries.md` 扩展 D4↔D7 Hub-Spoke 节

### 不变更（v1.0）

- Go 代码与测试注释
- D7 `delegatetools` 运行时行为
- 已 IMPLEMENTED T 的测试实现

---

## 7. 风险评估

| 风险 | 缓解 |
|------|------|
| Hub-Spoke 迁 D7 与 D4 重构耦合过紧 | v1.0 仅规格边界；v2.0 分独立 slice |
| D4-S10 大量 T 映射到 Hub-Spoke | Legacy 双轨：T 归属改为 D7 Canonical，D4 列 legacy |
| Owner 对 D3/D2 措辞歧义 | R1 澄清记录写入 demand §3.4 |

## 8. 关联需求

| Demand ID | 标题 | 关系 |
|-----------|------|------|
| DM-20260614-009 | D2 SA Refine | Follower 模型；SubQuery Spoke |
| DM-20260614-008 | D7 SA Refine | Leader 模型；ExecutionFlowHub |
| DM-20260614-011 | delegate_tools → D7 | Hub-Spoke 路由已起步 |
| DM-20260614-016 | D3 SA Refine | 样板方法论 |
