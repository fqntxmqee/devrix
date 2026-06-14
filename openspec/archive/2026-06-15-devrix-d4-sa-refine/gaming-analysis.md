# D4 Multi-Agent — 博弈论分析（D4↔D7 Hub-Spoke 边界）

**Change ID:** devrix-d4-sa-refine  
**Demand ID:** DM-20260614-018  
**日期:** 2026-06-14  
**状态:** ✅ 双边共识（Claude + Cursor）— 对齐 R1 Owner 决议（D7-1 全归 D7）  
**关联:** `design.md` §12；`proposal.md` §4；DM-20260614-009（D2）；DM-20260614-008（D7）；**DM-20260614-020（D7 Turn 编排上移）**  
**双边共识:** `../devrix-d7-turn-orchestration/gaming-analysis-bilateral-consensus.md`  
**Claude Review:** `gaming-analysis-claude-review.md`  
**Cursor Response:** `gaming-analysis-cursor-response.md`

---

## 0. 因果链前言（双边共识 G-01）

> **D4 SA Refine 不是 D4 的孤立优化，而是 D7 Turn 编排上移（DM-020）的连锁产权转移。**

```
DM-020: D7 Turn 编排上移（D7 获得 LLM 调用权）
    │
    ├─ D7 成为"真正的 Leader"（拥有 LLM 编排的硬权力）
    │
    ├─ LLM 调用权 + Hub-Spoke 编排权 = 互补性资产
    │     D7 调完 LLM 后如需经 D4 派发 Worker → 双重边际化
    │
    └─ DM-018: D4 交出 Hub-Spoke 编排权（本 change）
          ├─ D4-S10 Delegate → D4-S14 ExecuteWorker（执行面）+ D7-S2/S4 Hub-Spoke（编排面）
          ├─ D2 SubQuery Flow 发布同迁 D7（三 Spoke 统一出口）
          └─ 与 DM-009（D2 交出 LLM 调用权）对称——三个 SA Refine 的汇聚点是 D7
```

**一句话：** 这是 Stackelberg 均衡修正——把 de facto 权力收拢到 de jure Leader（D7），Follower（D2/D4）只保留域内执行比较优势。

---

## 文档目的（修订）

| 文档 | 角色 |
|------|------|
| `demand.md` §1.3 | D4 Follower 一句话 + Out of Scope |
| `proposal.md` §3–§4 | S11–S16 + Hub-Spoke R1 |
| **本文件** | D4↔D7↔D2 博弈模型、Hub-Spoke 激励错配、Commitment 装置 |
| `design.md` §12 | 接口契约 + v2.0 迁移 |
| `d7-boundary.md` | 跨域 SoT（v1.0 新建） |

---

## 1. 多方博弈位置

```
用户（Principal）
    ↓
D1 Gateway（Trusted Intermediary — 权限 UI + IM 展示）
    ↓
D7 Coordinator（Orchestration Mediator — Leader + Hub-Spoke SoT）
    ├── S2 DispatchWorker / delegatetools
    ├── S3 WaveScheduler（另一 Spoke 入口）
    ├── S4 ExecutionFlowHub → WorkPlan → D1（Costly Signaler）
    └── 选择 Executor ↓
            ├── D4 Multi-Agent（Agent Execution Follower）
            │     S11 Provision → S12 Run → S13 Isolate → S14 ExecuteWorker
            └── D2 Context Engine（Nested Execution Follower）
                  S19 RunNestedQuery（无 Publish）
        ↓ LLM
D3 LLM Gateway（公共能力，与 Hub-Spoke 正交）
    ↑ 可观测
D5 Span / D6 Judge
```

### 1.1 D4 与 D7 的 Stackelberg 关系

| 阶段 | D7（Leader） | D4（Follower） |
|------|-------------|----------------|
| 先动 | 选 Spoke（D4 Worker / D2 SubQuery）；async/sync；fallback | — |
| 后动 | 注入 WorkerSpec、Leader 引用 | fork→run→join；PermissionGate |
| 观测 | FlowEvent → WorkPlan → D1 | AgentEvent → **D7 Bridge**（非直连 Hub） |
| 不保证 | 任务结构、并行策略 | 结论质量（D6）、编排正确性（D7） |

**核心洞察：** D7 保证「谁来做、进度如何广播」；D4 保证「Worker Agent 实例机制正确」。Hub-Spoke 是 **D7 的子博弈场**，不是 D4 的 Scenario。

### 1.2 三 Spoke 写侧的统一均衡（R1 动机）

| Spoke | 执行 Follower | 现行 Flow 发布 | R1 目标 |
|-------|--------------|---------------|---------|
| Delegate Worker | D4 ExecuteWorker | D4 `FlowBridge` ❌ | D7 `agent_bridge` |
| SubQuery 嵌套 | D2 RunNestedQuery | D2 `flow_report` ❌ | D7 `subquery_bridge` |
| Wave SubAgent | D2 Background | wave → Flow | D7 Dispatch 统一入口 |

若 Hub-Spoke 留在 D4，D2/D7 Wave 路径成为 **规则外玩家** — 无法达成子博弈纳什均衡（各自优化本地 Publish 逻辑）。

---

## 2. 委托代理链

```
Principal（用户）
    → Mediator（D7）       — 不知走 delegate 还是 Wave 还是 SubQuery fallback
        → Follower（D4/D2）— 不知进度如何呈现给用户
            → Worker 实例   — 不知是否被 Leader 正确隔离
                → Judge（D6） — 事后评估委派质量
```

| 问题 | 谁最该回答 | D4 v1.0 承诺 |
|------|-----------|-------------|
| 谁来做子任务？ | D7-S2 DispatchWorker | **Out of Scope** |
| Worker 隔离对吗？ | D4-S13 | COW + worktree T |
| 进度对用户可见？ | D7-S4 FlowEvent | **Out of Scope**（D4 不 Publish） |
| Worker 结果合并？ | D4-S14 ExecuteWorker | Join T |
| 结论好不好？ | D6 | **Out of Scope** |

---

## 3. 现状均衡失灵

### 3.1 S 被 module 绑架

| 玩家 | 局部最优 | 全局结果 |
|------|----------|----------|
| D4 开发者 | 新能力加 `delegate/service.go` | S10 膨胀为 Hub-Spoke + 执行混合体 |
| D7 开发者 | 只改 `delegatetools` 路由 | 派发矩阵仍在 D4，边界模糊 |
| 注册表维护者 | 一包一 S | 10 个技术 S，无法回答 C1–C5 承诺 |

**DSAFT 修正：** Canonical S11–S16 按 **供给→运行→隔离→执行→外化** 切；S1–S10 冻结；S10 编排语义 **迁 D7**。

### 3.2 Follower 做 Leader 的事（Hub-Spoke 双头）

| 代码 | 实际行为 | 博弈角色错配 | 目标 |
|------|----------|-------------|------|
| `delegate/service.go` DelegateOrFallback | 选 D4 vs D2 Spoke | Leader 路由 | D7-S2-A04 |
| `delegate/bridge.go` | hub.Publish | Costly Signaler | D7-S4-A04 |
| `nested/flow_report.go` | SubQuery hub.Publish | 第二套 Signaler | D7-S4-A05 |
| `delegatetools/` | 工具入口 | Leader 路由 ✅ | 保持 D7 |
| `flow/workplan` | 读模型 | Hub 聚合 ✅ | 保持 D7 |

**Owner R1 闭合：** 不是「折中留执行编排」，而是 **全部 Publish 与 Dispatch 归 D7**。

### 3.3 「有 T 无 Canonical Hub-Spoke」= 规格债务

D4-S10 有 12 条 T，其中 5 条实测跨 D7/D2（FlowEvent、delegate-progress、IM）。Canonical 归属分散 → SRE 故障定界时不知查 D4 还是 D7。

**修正：** Hub-Spoke 相关 T canonical → D7-S2/S4；D4 仅保留执行面 T。

---

## 4. D4 Canonical S 的博弈角色

| S | 价值流 | 博弈角色 | Commitment 类型 |
|---|--------|----------|----------------|
| S11 ProvisionAgent | 供给 | 资源门槛（配额） | 硬限制 MaxTotalAgents |
| S12 RunAgentLoop | 执行 | 机制正确性 | 状态机 + PermissionGate |
| S13 IsolateAndMerge | 隔离 | 防污染 | COW costly signal |
| S14 ExecuteWorker | Worker 执行 | Follower 承诺 | fork→run→join 原子性 |
| S15 InvokeExternalAgent | 外化 | 进程隔离 | Session 级 subprocess |
| S16 ConfigureAgents | 配置 | 可替换策略 | 配置热更新（未来） |

**不在 D4 S 层：** Hub-Spoke、WorkPlan、delegate_* 路由、FlowEvent schema。

---

## 5. Hub-Spoke 作为 D7 机制设计

### 5.1 子博弈结构

```
子博弈场：Hub-Spoke（D7-S2 + D7-S4）
    Leader：D7 DispatchWorker
    Spokes：{ D4Worker, D2SubQuery, D2Background, WaveRunner... }
    读模型：WorkPlan（D7-S4）
    信号通道：FlowEvent → D1 IM（Costly Signaler）
```

### 5.2 激励相容条件

| 条件 | 设计响应 |
|------|----------|
| Leader 不能伪造 Worker 进度 | FlowEvent 仅 D7 Bridge 发布；Worker 不可直写 Hub |
| Worker 不能僭越 Leader | Worker 禁 delegate_* / Fork（D4-S14 T） |
| Spoke 不可见其他 Spoke 全历史 | ContextPolicy + SessionView COW |
| 用户可见进度可信 | WorkPlan 仅反映 FlowEvent，禁 synthetic progress（对齐 D7 T 候选） |

### 5.3 与 D2 Follower 对称

| 对称轴 | D2 | D4 |
|--------|----|----|
| Follower 承诺 | QueryLoop 机制正确 | Agent/Worker 机制正确 |
| 不拥有 | 编排、Task 写模型 | Hub-Spoke、Flow Publish |
| 嵌套执行 | S19 RunNestedQuery | S14 ExecuteWorker（D7 派发） |
| LLM 消费 | 直接 Process | 经 D2 IEngine.Process |

---

## 6. Commitment 装置（T 锚点）

| 承诺 | P0 T | 博弈含义 |
|------|------|----------|
| COW 不污染父 Session | D4-S13-A01-T05 | 合并前隔离 = costly signal |
| Worker 不能 delegate | D4-S14-A01-T03 | 防 Follower 变 Leader |
| Join dedup | D4-S13-A02-T07 | 防重复信号污染 Leader 上下文 |
| PermissionGate 阻塞 | D4-S12-A02-T02 | 机制约束，非质量评判 |
| FlowEvent→IM（迁 D7 canonical） | D4-S10-A02-T09 → D7 | 进度透明 = Costly Signaler |
| SubQuery fallback 路由（迁 D7） | D4-S10-A01-T07 → D7 | Leader 显式选 Spoke |

---

## 7. 错配修正对照（R1 前后）

| 错配模式 | 修正前 | R1 修正后 |
|----------|--------|-----------|
| S 被包绑架 | 10 技术 S | S11–S16 价值流 |
| Hub-Spoke 双头 | D4 Service + D7 delegatetools | D7 Dispatch 唯一 |
| 双 FlowBridge | D4 bridge + D2 flow_report | D7 hubspoke 统一 |
| D4 名含 Delegated | S10 Delegate | S14 ExecuteWorker |
| v2.0 分裂交付 | 独立 D7 change | 并入 slice a–e |

---

## 8. 开放问题（S3-Gate 前闭合）

| # | 问题 | 建议 | 状态 |
|---|------|------|------|
| OQ1 | Builtin RunExplore 是否仍留 D4？ | 保留为 D4 执行能力；**触发路由**归 D7 fallback | 待 design 细化 |
| OQ2 | Wave SubAgentRunner 是否经 DispatchWorker 统一？ | v2.0-e 收敛；v1.0 登记 | 待 |
| OQ3 | D4 `builtin/` 与 D2 SubQuery fallback 重复？ | D7 派发矩阵显式优先级：D4 enabled → D4；else D2 | R1 隐含接受 |
| OQ4 | metric `agent.fork.*` 归 D5 后 dashboard 迁移？ | v1.1 登记，不改 metric 名字 | 待 |

---

## 9. 与 D2/D7 分析文档交叉引用

| 主题 | 参见 |
|------|------|
| D2 Thin + Follower | `devrix-d2-sa-refine/gaming-analysis.md` §1.1 |
| D7 Leader + Costly Signaler | `devrix-d7-sa-refine/gaming-analysis.md` §10 |
| delegate_tools 已迁 D7 | DM-20260614-011 |
| D2 flow_report 迁 D7 | 本文件 §3.2 + design §12.4 slice c |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：Hub-Spoke 全归 D7 + 三 Spoke 统一 + R1 对称分析 |
| 0.2 | 2026-06-15 | 双边共识落盘：因果链 G-01 前言 + 状态更新 ✅ 双边共识 |
