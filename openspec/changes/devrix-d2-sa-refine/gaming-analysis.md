# D2 Context Engine — 博弈论分析（D2↔D7 边界）

**Change ID:** devrix-d2-sa-refine  
**Demand ID:** DM-20260614-009  
**日期:** 2026-06-14  
**状态:** ✅ 与 DM-20260614-008（D7）对齐 — 可支撑 S3 Design / S5 验收  
**关联:** `openspec/specs/d2-context-engine/d7-boundary.md`；DM-20260614-007（D1→D7 ingress）；DM-20260614-008（D7 Leader）

---

## 0. 文档目的

| 文档 | 角色 |
|------|------|
| `demand.md` §1.3 | D2 Follower 一句话 + Out of Scope |
| `proposal.md` §2–§3 | North Star + S15–S20 切法 |
| **本文件** | D2↔D7 完整博弈模型、错配根因、Commitment、开放问题收敛 |
| `design.md` §12 | 接口契约 + 编排序 + 迁移表 |
| `d7-boundary.md` | 跨域 SoT（双向引用） |

---

## 1. 多方博弈位置（含 D7）

```
用户（Principal）
    ↓ 委托
D1 Gateway（Trusted Intermediary — 信号通道）
    ↓ 唯一入口（DM-007）
D7 Coordinator（Orchestration Mediator — Leader）
    ├── S2 ProcessMessage（筛路径）
    ├── S5 ClassifyIntent / TaskGraph（结构决策）
    ├── S3 WaveScheduler（并行机制）
    ├── S4 ExecutionFlow（Costly Signaler → D1）
    └── 调用 ↓
D2 ContextEngine（Execution Follower）
    ├── S15 PrepareExecutionContext
    ├── S16 RunQueryLoop
    ├── S17 PersistSessionState
    └── S18/S19 约束与嵌套执行
        ↓
D3 LLM / Tools / D4 Agent
    ↑ 可观测
D5 Span / D6 Judge（事后）
```

### 1.1 D2 与 D7 的 Stackelberg 关系

| 阶段 | D7（Leader） | D2（Follower） |
|------|-------------|----------------|
| 先动 | 选 FastPath / Wave / SerialExplore；选 Executor（D2 vs D4） | — |
| 后动 | 注入 `QueryRequest`、Hooks、可选 Queue | 在参数下跑 Prepare→Loop→Persist |
| 观测 | `d7.*` span、FlowEvent | `d2.query.*` span、EngineEvent |
| 不保证 | 结论质量（D6） | 任务结构正确（D7-S5） |

**核心洞察：** D7 保证「编排过程可验证」；D2 保证「单 session 执行机制可验证」。二者不可互换。

### 1.2 与 D7 DM-008 的对称原则

| 域 | 对称原则 | 含义 |
|----|----------|------|
| D7 | S2 入口确定性 > S5 决策准确性 | 先有可测入口，再优化分类 |
| D2 | S16 执行机制确定性 > S10 功能堆叠 | 先 Thin Loop + Persist，再迁 TaskTools |

---

## 2. 委托代理链（Principal → Mediator → Follower → Judge）

```
Principal（用户）
    → Mediator（D7）     — 用户不知走 FastPath 还是 Wave
        → Follower（D2） — 用户不知 tool 顺序是否在 LLM 层被篡改
            → Agent（D4）— 委托执行
                → Judge（D6）— 事后评估
```

| 问题 | 谁最该回答 | D2 在 v1.0 的承诺 |
|------|-----------|-------------------|
| 我的消息进系统了吗？ | D1 + D7-S2 | D2 不面对 ingress |
| 走哪条编排路径？ | D7-S5 | **Out of Scope** |
| 工具回合顺序对吗？ | D2-S16 | tool_result 有序 T |
| 「完成」可信吗？ | D2-S17 | deferred complete T |
| 结论好不好？ | D6 | **Out of Scope** |

---

## 3. 现状均衡失灵（重构动机）

### 3.1 S 被 module 绑架

| 玩家 | 局部最优 | 全局结果 |
|------|----------|----------|
| 开发者 | 新能力放进现有包（tasks/、delegate_tools） | D2 膨胀为「小 D7」 |
| 注册表维护者 | 一包一 S，编号沿用 | 无法回答 turn 生命周期 |
| D7 演进 | 改 D2 里的 Task/Plan | D2 T 回归面爆炸 |

**DSAFT 修正：** Canonical S15–S20 按 **Prepare → Execute → Persist** 切；Legacy S1–S14 冻结。

### 3.2 Follower 做 Leader 的事（跨域漂移）

| 代码 | 实际行为 | 博弈角色错配 | 目标 |
|------|----------|-------------|------|
| `tasks/task_manager.go` | Task CRUD 写模型 | Leader 状态 | D7-S1 |
| `tasks/plan_mode.go` | 结构写限制 | Leader 策略 | D7-S5 |
| `delegate_tools.go` | 路由到 D4 | Leader 路由 | D7 F |
| `queue/` delegate-progress | Flow 聚合 drain | Costly signal 中枢 | D7-S4 |
| `query/loop.go` Hooks+Queue | 编排回调注入点 | 应 D7 注入、D2 只执行 | design §12 |

### 3.3 「有实现无 Canonical」= cheap talk

D2 有大量 Legacy T（S2–S10），但无 Canonical S → 无法回答「Follower 生命周期哪一步失败」。与 D7 重构前「S2 无 T」同构。

**修正：** Canonical T 映射表（design §6）；v1.0 不改测试注释。

---

## 4. D2 Canonical S 的博弈角色

| S | 价值流 | 博弈角色 | Commitment 类型 |
|---|--------|----------|----------------|
| S15 | PrepareExecutionContext | Pre-play setup | 上下文合法才可进入 Loop |
| S16 | RunQueryLoop | **Follower 核心机制** | turn 有序、可 cancel |
| S17 | PersistSessionState | Costly completion | durable 后才 complete |
| S18 | EnforceExecutionPolicy | Mechanism constraint | 权限先于 execute |
| S19 | NestedExecution | Nested sub-game | depth / read-only 边界 |
| S20 | LegacyHarnessFallback | Legacy equilibrium | 仅显式配置 |

**横切：** Token 计数 → S15 F；Mock → Legacy S14。

---

## 5. Commitment Devices（T 锚点）

| 承诺 | Canonical T | 机制 | 防什么 |
|------|---------------|------|--------|
| 工具顺序 | S16-A01-T01 | tool_result 与 tool_use 配对 |  silent reorder |
| 完成可信 | S17-A01-T01 | snapshot 后 emit complete | 假完成 |
| Plan 写拒绝 | S18-A01-T01 | plan mode path gate | LLM 自报合规 |
| Bash 沙箱 | S18-A02-T01 | workdir confine | 逃逸执行 |
| Explore 只读 | S19-A01-T01 | write tools excluded | 嵌套越权 |
| D2 Thin | S16-A01-T03 (v1.1) | query 无 D4 import | Follower 变 Router |

---

## 6. D7↔D2 接口的博弈含义

### 6.1 运行时链（现行）

```text
D1.RouteInbound
  → D7.Entry.ProcessMessage
      → SessionOrchestrator（Classify / FastPath / Wave）
      → d2Executor.RunQueryLoop(req)
          → D2.IEngine.Process(session, message)
              → S15 → S16 → S17
      → D7-S4 PublishFlowEvent → D1
```

**Bootstrap 事实：** `wire_coordinator.go` 中 `d2Executor` 适配 `contracts.IEngine`，D2 不感知 D7 类型。

### 6.2 依赖方向（不可反转）

```text
D7 ──depends on──► contracts.IEngine, QueryLoopExecutor
D2 ──NOT import──► orchestration.*
```

若 D2 import D7 → Follower 绑定 Leader 实现 → 无法独立测试 Follower 机制。

### 6.3 LoopHooks / SessionQueue

| 组件 | v1.0 物理位置 | 规格归属 | 博弈含义 |
|------|--------------|----------|----------|
| LoopHooks | D2 struct 字段 | D7 注入策略 | Leader 回调，Follower 执行 |
| SessionQueue | D2 `queue/` | Canonical → D7-S4 | Mediator 进度管道 |

v1.0：**规格收口**；v2.0：**代码迁移**。

---

## 7. Out of Scope 表（博弈 — 为何不能留 D2）

| 迁出 | 为何不能留 | 留在 D2 的均衡后果 |
|------|-----------|-------------------|
| Task 写模型 | Leader ledger | D7 改 Task 必跑 D2 全量 T |
| PlanMode | 结构决策 | Plan 与 Permission 混在 Follower |
| delegate_tools | 路由 = Leader | D2 import D4 编排面 |
| delegate-progress | Costly signal 聚合 | D2 承担 Mediator 广播 |
| ClassifyIntent | Information Producer | 双写分类逻辑 |

---

## 8. 开放问题与收敛（Grill Review）

| # | 问题 | 选项 | **收敛** |
|---|------|------|----------|
| Q1 | S11 Queue Canonical？ | D2 / D7-S4 | **D7-S4**；Legacy S11 追溯 |
| Q2 | worktree 归 S18 还是 S19？ | S18 / S19 | **S18**（执行策略/隔离） |
| Q3 | TaskTools 何时迁？ | v1.0 / v2.0 | **v2.0**；v1.0 Out of Scope |
| Q4 | v1.0 改 loop.go？ | 是 / 否 | **否**；v1.1 import 测试 |
| Q5 | D2 是否保证结论质量？ | 是 / 否 | **否** → D6 |
| Q6 | FastPath 时 D7 多薄？ | 零逻辑 / 最小 Classify | **最小 Classify**（D7-S2）；执行仍 D2-S16 |

---

## 9. 分阶段均衡（v1.0 / v1.1 / v2.0）

| 版本 | 均衡目标 | D2 状态 | D7 联动 |
|------|----------|---------|---------|
| v1.0 Registry | 规格闭合 Leader/Follower | S15–S20 + Legacy 双轨 | DM-008 已归档 |
| v1.1 Traceability | Follower 可独立验证 | Span + S16-T03 | D7-S2 T 全绿 |
| v2.0 Structure | 代码 ≡ 规格 | tasks/delegate 迁出 | D7 v2.0 Task 归位 |

---

## 10. 三方共识摘要

| 决策 | 选择 |
|------|------|
| D2 角色 | Execution Follower |
| D7 角色 | Orchestration Mediator / Leader |
| ingress | D1 → D7 only（DM-007） |
| S 切法 | 切法 A，S15–S20 |
| Legacy | S1–S14 冻结 |
| 跨域 | v1.0 登记；v2.0 迁移 |
| v1.0 | Registry only，无 Go 变更 |

---

## 11. 与 D7 gaming-analysis 的交叉引用

| D7 概念 | D2 对应 |
|---------|---------|
| S2 Screening | D2 不 Screening；只接收已筛路径 |
| S4 Costly Signaler | D2-S17 complete；FlowEvent 归 D7-S4 |
| S5 Information Producer | **Out of Scope** |
| anti-fabrication T03 | D2 不伪造 Task 进度；只 emit EngineEvent |
| Worker = D2/D4 | D2-S16 / D4 delegate，非 D7-S3 |

**双层中介：** D7 编排可验证 + D1 信号必达；D2 在两者之下，只做执行机制。
