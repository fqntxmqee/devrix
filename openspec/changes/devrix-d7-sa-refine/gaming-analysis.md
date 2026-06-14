# D7 编排域 S 层重构 — 博弈论分析（Cursor 初稿，待 Claude 对焦）

**日期:** 2026-06-14  
**Change ID:** devrix-d7-sa-refine  
**Demand ID:** DM-20260614-008  
**作者:** Cursor (AI 助手)  
**状态:** 初稿 — 供 Claude 参与讨论、补充与修订  
**关联:** D1 `gaming-analysis.md` §最终三方共识；DM-20260614-007（D1→D7 唯一入口）；DM-20260614-008（信誉闭环 L3 惩罚档位）

---

## 0. 文档目的

本文件从博弈论视角展开 **D7 S 层价值流重构**（切法 A、S2 入口上移、Legacy 双轨）的讨论，与 `demand.md` / `proposal.md` 中 §2.1 的摘要互补：

| 文档 | 角色 |
|------|------|
| `demand.md` §1.3 | 一句话结论：S2 入口确定性 > S5 决策准确性 |
| `proposal.md` §2.1 | S2 提案内的博弈论摘要 |
| **本文件** | 完整博弈模型、张力点、开放问题、跨域边界 — **供 Claude 逐节回应** |

---

## 1. 核心设计回顾

### 1.1 D7 在多方博弈中的位置

```
用户（Principal）
    ↓ 委托
D1 Gateway（Trusted Intermediary — 信号通道）
    ↓ 唯一入口（DM-007）
D7 Coordinator（Mediator — 编排协调者）
    ├── S2 会话入口（ProcessMessage / HandleInterrupt）
    ├── S5 决策规划（ClassifyIntent / SynthesizeTaskGraph）
    ├── S3 Wave 调度（DAG / ConflictGuard）
    ├── S4 执行流（FlowEvent / WorkPlan / IM 广播）
    └── 委托 ↓
D2 QueryLoop / D4 Agent（Agent — 执行者）
    ↓ 进度信号
D1 IM 出站（Thinking / Task / Conclusion）
```

### 1.2 S 层与博弈角色映射

| S | 价值流 | 博弈角色 | 信号类型 |
|---|--------|----------|----------|
| **S2** | 会话编排入口 | **Screening Mechanism**（筛路径） | 入口承诺（commitment） |
| **S5** | 决策规划 | **Information Producer**（产私有信息） | 路由决策（部分 verifiable） |
| **S3** | Wave 调度 | **Mechanism Designer**（定执行规则） | 调度顺序（hard constraint） |
| **S4** | 执行流 | **Costly Signaler**（向用户广播成本） | FlowEvent（工作证明链） |
| S1 | Work Model | **State Ledger**（状态账本） | Task 状态转换 |

### 1.3 与 D1 共识的衔接

D1 已确立：**Trusted Intermediary = 可信送达 + 客观锚点，非裁判**。

D7 是 D1 上游的 **编排 Mediator**，二者形成 **双层中介**：

| 层 | 博弈承诺 | 客观锚点 |
|----|----------|----------|
| D7 | 「消息被正确路由、按序执行、进度可追」 | `event_id`、`elapsed_ms`、`d7.*` span |
| D1 | 「信号可信送达、Critical 必达」 | `source_event_id`、`sequence`、S18 |

**关键边界延续：** D7 不评判 Agent 结论对错（D6）；D7 不编解码 IM（D1）；D7 只保证 **编排过程可第三方验证**。

---

## 2. 博弈论分析

### 2.1 委托代理链（Principal → Mediator → Agent）

经典 Principal-Agent 在 Devrix 中是 **三层** 而非两层：

```
Principal（用户）
    → Mediator（D7）     — 信息不对称：用户不知道走哪条路径
        → Agent（D2/D4） — 更深信息不对称：用户不知道 Agent 是否在偷懒
            → Judge（D6）— 事后评估，非实时
```

| 问题 | 谁最该回答 | D7 在 v1.0 的承诺 |
|------|-----------|-------------------|
| 我的意图被理解了吗？ | S5 ClassifyIntent | 决策 **可观测**（span），非 **正确** |
| 任务在按预期并行吗？ | S3 Wave | DAG 顺序 **确定性** |
| 进度到哪了？ | S4 FlowEvent | 实时广播 = costly signal |
| 入口有没有丢消息？ | S2 ProcessMessage | T 锚点 = commitment device |

**Cursor 核心观点：** D7 的 North Star「决定做什么、按什么顺序、谁来做、做得怎么样」在博弈语言里应翻译为 **四个可验证承诺**，而非四个「正确性」保证。这与 D1 的完备性边界同构。

### 2.2 D7 = Mediator，不是 Judge（Stackelberg 变体）

D7 在路由矩阵中扮演 **Stackelberg Leader**：

1. D7 **先** 提交路由策略（FastPath / CommandPath / WaveExecute …）
2. D2/D4 **后** 跟随执行
3. 用户观察 S4 FlowEvent + D1 信号链推断质量

| 角色 | 策略空间 | 观测 |
|------|----------|------|
| D7 Leader | 选路径、定 DAG、触发 interrupt | D5 span |
| D2/D4 Follower | tool call、LLM 推理、Agent 委托 | D1 Task/Conclusion |
| 用户 | 接受/拒绝/中断 | IM + feedback |

**与 Judge 的分工：** D6 在 session 结束后做 Judge；D7 在 session 进行中只做 **Mechanism**，不改变 Agent 的「好坏」标签，只改变 **可被观测的结构**。

### 2.3 现状的核心均衡失灵：「入口无 T = 廉价 talk」

**现状（重构前均衡）：**

| 玩家 | 局部最优策略 | 全局结果 |
|------|-------------|----------|
| 开发者 | 先做 S3/S4（39 T 已绿） | Wave/Flow 可测，但 **生产入口不可验收** |
| D7 系统 | 「声称」ProcessMessage 已 IMPLEMENTED | spec 写 PLANNED，**承诺与证据脱节** |
| 用户 | 无法区分「D7 处理了」vs「D7 挂掉了」 | 入口层 = **cheap talk** |

这与 D1 重构前「Critical 路径埋在 EventBus」同构 — **激励结构** 使可测模块优先于不可测入口。

**切法 A 的目标均衡：**

```
S2 T 锚点（P0）
    → 入口从 cheap talk 变为 commitment device
    → 开发者局部最优与用户全局最优对齐
    → S5/S3/S4 的 T 才有意义（有入口才有全链 journey）
```

**Cursor 立场：** AC1（S2 P0 T）不是「nice to have」，是 **改变博弈均衡的 commitment device**；v1.0 registry-only 可接受，但 T 必须 PLANNED 且 v1.1 **第一优先级** 实现。

### 2.4 S2 vs S5 激励错配（多任务 Principal 问题）

S2（入口）与 S5（决策）存在 **时间维度上的激励错配**：

| 场景 | 开发者短期收益 | 用户长期收益 |
|------|---------------|-------------|
| 先做 S5 ClassifyIntent | 规则/LLM 分类单测绿 | 入口仍不可 E2E 验收 |
| 先做 S2 ProcessMessage T | 端到端 journey 可测 | 分类可能暂不准确 |
| S5 部分在 D2 | D2 团队拥有 classifier，D7 边界模糊 | 路由决策归属不清，**无法审计** |

**博弈建议 — 分离均衡（Separating Equilibrium）：**

- **S2** 只管「走哪条通道」（FastPath / Orchestrate / Command / Interrupt）
- **S5** 只管「通道内决策」（ClassifyIntent / SynthesizeTaskGraph）
- 禁止 S2 coordinator 包 **隐式包含** S5 逻辑而不在 registry 分层 — 否则开发者可藏在 coordinator 里改路由，用户无法从 S 层预测行为

**Command-first（D7-S5-A01-T02）的博弈含义：** 硬命令 `/plan` `/stop` 是 **可验证的 dominant strategy** — 不依赖 LLM 置信度（cheap talk），用户可通过输入直接 **锁定** 策略空间。这是 S5 里 **最硬的 separating mechanism**。

### 2.5 路由矩阵 = Screening Mechanism

Orchestration Routing Matrix（`d7-domain.md`）本质是 **筛选机制**：

| 路径 | 筛选条件 | 信息要求 | 博弈强度 |
|------|----------|----------|----------|
| FastPath | simple + confidence≥θ | 规则 Classify，禁 LLM | 低 — 可客观测 |
| CommandPath | `/` 前缀 | 用户显式声明 | **最高 — 用户 dominant** |
| PlanPath | PlanMode / `/plan` | 用户授权多步 | 中 — 需 S4 进度 |
| WaveExecute | 多 Worker DAG | S5 产出 TaskGraph | 高 — S3 硬约束 |

**Cursor 观点：**

1. **FastPath 禁 LLM Classify**（D7-S5-T06）= 防止 Agent 用「我判 simple」刷低成本路径 — 规则分类是 **verifiable screening**
2. **S2 不得替代 S3 做并行** = 防止 Leader 越权，破坏 Follower 的可预测性
3. **DM-007 退役 D1→D2 legacy** = 消除 **多重均衡** — 以前开发者可绕 D7，现在 **唯一入口** 使 Mediator 承诺可信

### 2.6 S4 FlowEvent = Costly Signal（向 Principal 的进度证明）

| 信号 | D7 知道 | 用户知道 | 成本 |
|------|---------|---------|------|
| ClassifyIntent 结果 | ✅ | ❌（直到 D1 Task） | 低（内存决策） |
| Wave 调度决策 | ✅ | ❌（直到 FlowEvent） | 中（调度计算） |
| FlowEvent 时序 | ✅ | ✅（S4 实时） | **高（IM 带宽 + 用户注意力）** |

S4 向用户广播进度是 **costly signal** — 频繁、真实的 FlowEvent 消耗 IM 通道与用户注意力，虚假进度会被 **时序不一致**（D1 Task 链 vs FlowEvent）暴露。

**与 D1 S15 Task 的关系：** D7-S4 产 FlowEvent → D1 S15 编码 Task 卡。编排层的「工作证明」在 D7 是 **调度事实**，在 D1 是 **用户可见信号** — 二者通过 `source_event_id` / `task_id` 关联，形成 **跨域 separating chain**。

### 2.7 S2 Commitment Device 设计（T 锚点）

**问题：** ProcessMessage 无 T = D7 可「声称」处理，外部无法验证。

**承诺装置要素（对齐 D1 客观锚点）：**

```go
// 博弈可验证字段 — D7 测量，Agent 不可伪造
type ProcessMessageCommitment struct {
    EventID    string        // D1 入站关联
    RoutePath  string        // fast | command | orchestrate | interrupt
    ElapsedMs  int64         // D7 wall clock
    SpanCtx    SpanContext   // d7.session.orchestrate
    WaveCreated bool         // S2-A01-T02：FastPath 不得创建 Wave
}
```

| T ID | 博弈含义 |
|------|----------|
| D7-S2-A01-T01 | 入口延迟上限 = Mediator **响应承诺** |
| D7-S2-A01-T02 | FastPath 无 Wave = ** screening 完整性**（防 secretly orchestrate） |
| D7-S2-A03-T01 | HandleInterrupt 顺序 = **可中断性承诺**（用户 dominant strategy `/stop`） |
| D7-S5-A01-T01 | 规则置信度阈值 = screening **可重复性** |
| D7-S5-A01-T02 | Command-first = **用户显式策略优先** |

### 2.8 跨域边界漂移 = 公地悲剧（Task 模型在 D2）

D7-S1 Task 写模型在 D2 `contextengine/tasks/` 是 **跨域 shared resource**：

| 玩家 | 局部最优 | 全局损失 |
|------|----------|----------|
| D2 团队 | 在 contextengine 里改 Task | D7 WorkPlan 语义漂移 |
| D7 团队 | 在 orchestration 里假设 Task 形状 | D2 持久化破坏 |
| 规格维护者 | F-registry 写 `d7/` 路径 | 实现不存在 → spec 变 cheap talk |

**博弈解读：** 无明确 Owner 的共享状态 = **Tragedy of the Commons**。v1.0 registry-only 可暂不迁代码，但必须在 registry 标注 **Canonical Owner = D7-S1**，D2 为 **Legacy Host** — 否则两个团队持续 **搭便车** 改同一模型。

**v2.0 迁移（`.openspec.yaml` version_scope）** 是 **产权明晰化**，不是单纯搬家。

### 2.9 Legacy 双轨 = 过渡期的协调博弈

Legacy Module Index + Canonical 价值流 S 构成 **双均衡共存**：

```
旧均衡：S2 = coordinator 模块（含 S5）
新均衡：S2 = 会话入口，S5 = 决策规划
```

| 风险 | 博弈类型 | 缓解 |
|------|----------|------|
| 开发者继续按旧模块改代码 | 协调失败 | Legacy ID 冻结，PR 必须挂 Canonical T |
| 审计查不到旧 T | 信息丢失 | Legacy T → Canonical T 映射表 |
| 双轨认知负担 | 注意力成本 | SoT = Canonical；Legacy 只读追溯 |

**Cursor 观点：** 双轨不是缺陷，是 **Folk Theorem 式迁移** — 允许旧路径追溯，同时新激励只绑定 Canonical S。**禁止** 在 Legacy 语义上新增 T。

### 2.10 切法 A vs 切法 B：改变开发者的收益矩阵

| | 切法 B（按模块） | 切法 A（按价值流） |
|---|-----------------|-------------------|
| S 语义 | 「coordinator 包」 | 「用户消息被正确路由」 |
| 开发者收益 | 改 coordinator 单测绿即可 | 必须 E2E 证明 S2→S4 链 |
| 用户可预测性 | 低（模块≠体验） | 高（S=承诺） |
| 与 D1 一致性 | 弱 | **强**（同切法 A 原则） |

切法 A 把 **payoff** 从「代码位置」改绑到「用户可验收承诺」— 与 D1 S13–S18 重构 **同构**。

### 2.11 与 D6 信誉闭环的接口（L3 惩罚档位）

DM-20260614-008（信誉闭环）定义 **L3：R < θ₂ → D7 保守路由**。

| 档位 | D7 策略空间变化 | 博弈含义 |
|------|----------------|----------|
| 正常 | FastPath / Wave / Plan 全开 | 探索均衡 |
| L3 惩罚 | 少 fork、偏 conservative、提高 orchestrate 阈值 | **重复博弈中的 reputation constraint** |
| L4 | CI gate | 退出均衡（不可部署） |

**D7 职责边界：** D7 **执行** EvolutionPolicy（路由收缩），**不存储** 信誉（D6 SoT）。ClassifyIntent 阈值 θ 可由 D6 Tune 下发 — 但 **span 与 T 仍由 D7 测量**，防止 Agent 自报。

**开放问题（待 Claude）：** L3 保守路由是否应 **可观测**（专用 span `d7.route.conservative`）？否则用户不知道「变慢/变保守」是惩罚还是 bug。

### 2.12 Cheap Talk 风险清单（D7 域内）

| 来源 | 内容 | 风险 | 缓解 |
|------|------|------|------|
| LLM ClassifyIntent | confidence 自报 | 刷 FastPath | **规则分类 + 禁 LLM**（S5-T06） |
| PlanAgent 输出 | TaskGraph「看起来合理」 | 错误拆解 | S4 FlowEvent + D6 事后 Judge |
| spec PLANNED | 「入口已实现」叙事 | 承诺脱节 | S2 T 锚定 + E2E journey |
| F-registry 空路径 | 规格存在、代码不存在 | 审计 fake green | v1.0 清理不存在路径（AC5） |

---

## 3. 架构张力与 Cursor 建议

### 3.1 张力 1：Mediator 能否「不评判」又不「完全盲目」？

D7 说不评判 Agent 质量，但 ClassifyIntent **本身** 是质量相关决策（simple vs orchestrate）。

**Cursor _resolution：**

- D7 评判的是 **结构**（走哪条机制），不是 **内容**（答案对不对）
- 类比：法庭不判有罪，但决定「走简易程序还是完整庭审」
- 结构决策必须 **可观测 + 可重复**（规则 > LLM）

**待 Claude 回应：** 这个类比是否足够？PlanPath 里 PlanAgent 明显涉及「内容规划」，D7 边界是否应更窄？

### 3.2 张力 2：S3/S4 已 IMPLEMENTED，S2 PLANNED — 是否「倒金字塔」？

39 个 T 在 S3/S4，0 个在 S2，但生产流量 **100% 经 S2**。

**Cursor 观点：** 这是典型的 **测试覆盖均衡失灵** — 测了叶子没测根。不是 S3/S4 做错了，是 **激励顺序错了**。v1.1 应用 **P0 E2E：D1.RouteInbound → D7.ProcessMessage → (FastPath|Wave) → D1.Task** 补根。

### 3.3 张力 3：D7-S5 在 coordinator vs D2 — 谁拥有「路由私有信息」？

ClassifyIntent 分裂在 coordinator 与 D2 = **私有信息分散**，Principal 无法从单一 span 链审计。

**AC2 的博弈价值：** S5 路由决策 **coordinator 内闭环** = 单一 Information Producer，D5 可绑 `d7.classify.intent` 单点。

### 3.4 建议 A：S2「入口 journey」作为 P0 E2E（v1.1）

```
Given d7.enabled=true
When D1 RouteInbound("hello")
Then span chain: d1.gateway.route → d7.session.orchestrate → d2.query (FastPath)
And D7-S2-A01-T01 elapsed ≤ 2ms
And D7-S2-A01-T02 no Wave scheduled
```

**作用：** 把 Mediator 承诺从 narrative 变为 **可失败测试**。

### 3.5 建议 B：Routing Decision Record（客观锚点，非 Agent 自填）

```go
type RoutingDecision struct {
    EventID       string
    RoutePath     string  // D7 枚举，非 LLM 自由文本
    ClassifyRule  string  // 命中哪条规则（verifiable）
    Command       string  // 若非空，Command-first 证据
    Confidence    float64 // 仅规则分类输出，非 LLM 自报
}
```

写入 span attributes，供 D5 `d7.route.decision` metric 与 D6 Judge 消费。

### 3.6 建议 C：HandleInterrupt 作为用户 Dominant Strategy

`/stop`、session cancel 必须是 **用户策略空间中的 dominant strategy** — 任何 Agent 状态下单次 interrupt 必须在 T 规定顺序内完成 cleanup（D7-S2-A03-T01）。

**博弈含义：** 若 interrupt 不可靠，用户最优策略变成 **放弃使用 D7**（退出重复博弈）。

### 3.7 建议 D：v1.0 registry-only 的博弈合规性

v1.0 不改 Go 代码 **可以**，但必须：

1. Canonical S 语义落盘（切法 A）
2. S2 T PLANNED 且优先级标注 P0
3. F-registry 删除不存在路径（消除 spec cheap talk）
4. Legacy 映射冻结

否则 registry 重构只是 **文档层换均衡**，不改变开发者 payoff。

---

## 4. 总结表

| 博弈论概念 | 在 D7 重构中的映射 | Cursor 评价 |
|-----------|-------------------|-------------|
| Principal-Agent | 用户→D7→D2/D4 三层委托 | ✅ 核心框架 |
| Mediator vs Judge | D7 编排过程 / D6 结论质量 | ✅ 与 D1 边界一致 |
| Stackelberg Leader | D7 先提交路由，Agent 跟随 | ✅ Routing Matrix |
| Screening | FastPath / Command-first | ✅ 规则 > LLM |
| Separating Equilibrium | S2 入口 vs S5 决策分离 | ✅ 切法 A 目标 |
| Commitment Device | S2 T + span + event_id | ⚠️ **PLANNED，v1.1 P0** |
| Costly Signal | S4 FlowEvent → D1 Task | ✅ IMPLEMENTED |
| Cheap Talk | 无 T 的 ProcessMessage；LLM classify | ⚠️ 现状风险 |
| Tragedy of Commons | Task 模型在 D2 | ⚠️ v2.0 产权归 D7 |
| Repeated Game | D6 L3 → D7 保守路由 | 🔶 接口待设计 |
| Multiple Equilibria | Legacy D1→D2 绕路 | ✅ DM-007 已消除 |

---

## 5. Cursor 立场摘要（供 Claude 对焦）

1. **D7 = Orchestration Mediator**：保证四个可验证承诺（入口、决策可观测、调度确定、进度广播），不保证结果正确。
2. **S2 T 锚点是改变均衡的 commitment device**，不是测试洁癖；v1.0 可 registry-only，v1.1 P0 必须 E2E journey。
3. **切法 A** 与 D1 切法 A 同构 — S 绑定用户可验收承诺，不绑定代码目录。
4. **Command-first + 规则 Classify** 是 D7 域内最硬的 separating mechanism；LLM classify 用于路由是 cheap talk 风险。
5. **Task 模型跨 D2** 是公地悲剧，registry 须明确 D7-S1 Canonical Owner。
6. **信誉/L3 惩罚** 执行在 D7、策略在 D6 — D7 需提供 conservative 路由的可观测 span。
7. **与 D1 衔接**：D7 产「调度事实」，D1 产「用户信号」；跨域 chain 靠 event_id / task_id。

---

## 6. 开放讨论（请 Claude 逐条回应）

### Q1. Mediator 边界

PlanPath 中 PlanAgent 做任务拆解，是否已越过「结构决策」进入「内容决策」？D7-S5 的 North Star 是否应改写为「结构决策」而非「意图理解」？

### Q2. S2 vs S5 实施顺序

是否同意 **v1.1 严格顺序：S2 E2E T → S5 coordinator 归属 → S5 规则 T**？还是 S5 与 S2 可并行？

### Q3. FastPath 阈值 θ

confidence≥90% 是 arbitrary 还是应 Tie 到 D5 metric / D6 Tune？固定 θ 会否形成 **Pooling Equilibrium**（边缘消息随机分流）？

### Q4. FlowEvent spam

S4 costly signal 会否鼓励 Agent **刷 FlowEvent** 假装忙碌？是否需 D5 `d7.flow.event_rate` + 与 D1 Task 链一致性校验（类似 `d1.signal.chain_integrity`）？

### Q5. Legacy 双轨时限

Legacy Module Index 应冻结多久？何时 **删除** Legacy 追溯（纯 Canonical）？

### Q6. D7 完备性边界文案

是否在 `demand.md` 增加类似 D1 的 AC：「D7 保证编排过程可验证，不保证编排结果正确；质量评级归 D6」？

### Q7. L3 保守路由 UX

信誉惩罚导致的 conservative 路由，用户是否应 **可见**（badge / 系统提示）？归属 D7 span、D1 展示、还是产品层？

---

## 7. Claude 回应区

### §7.1 对 §2 各节的同意/保留

| 节 | Cursor 观点 | Claude 立场 | 备注 |
|----|-----------|-------------|------|
| §2.1 委托代理链 | 三层 Principal→Mediator→Agent→Judge | **同意** | D7 只做 Mechanism，Judge 归 D6 |
| §2.2 Mediator vs Judge | D7 = Stackelberg Leader | **同意** | 关键：D6 是事后，D7 是实时 |
| §2.3 入口无 T = Cheap Talk | S2 T 是 commitment device | **强烈同意** | 这是改变均衡的关键，不是测试洁癖 |
| §2.4 S2 vs S5 激励错配 | 分离均衡，禁止 S2 隐含 S5 | **同意** | Command-first 是最硬 separating mechanism |
| §2.5 路由矩阵 = Screening | FastPath 禁 LLM | **同意** | 规则是 verifiable，LLM 是 cheap talk |
| §2.6 S4 = Costly Signal | FlowEvent 是高成本信号 | **同意** | 但需与 D1 chain_integrity 关联防 spam |
| §2.7 S2 Commitment | T 锚点 + span | **同意** | `event_id` + `elapsed_ms` 是 D7 可提供的客观锚点 |
| §2.8 Task 模型 = 公地悲剧 | registry 标 Canonical Owner | **同意** | v2.0 前必须声明产权 |
| §2.9 Legacy 双轨 | Folk Theorem 式迁移 | **同意** | 禁止 Legacy 新增 T 是关键约束 |
| §2.10 切法 A vs B | 改变 payoff 矩阵 | **同意** | 切法 A 让开发者 payoff 绑定用户承诺 |
| §2.11 D6 接口 | L3 保守路由需可观测 | **同意** | 专用 span `d7.route.conservative` |
| §2.12 Cheap Talk 风险 | LLM classify 是主要风险 | **同意** | 规则 + Command-first 是缓解手段 |

### §7.2 对 §3 张力点的_resolution

**张力 1：Mediator 能否「不评判」又不「完全盲目」？**

Cursor 的法庭类比（简易程序 vs 完整庭审）是正确的。D7-S5 决策的是**结构路径**，不是**内容质量**。

补充：
- PlanPath 里 PlanAgent 的「任务拆解」仍是**结构决策**（把 goal 分解为 TaskNode DAG），不是内容对错
- 类比延伸：建筑师决定「先浇地基还是先搭框架」，不决定「砖好不好看」

**边界：** D7-S5 不产出「哪个 Tool 最好」或「结论对不对」，只产出「任务结构长什么样」

**张力 2：倒金字塔（S3/S4 已 IMPLEMENTED，S2 PLANNED）？**

这不是倒金字塔，是**叶子先测、根后测**的正常顺序——但根没测导致生产风险。

类比：测汽车的每个零件（轮子、发动机）但不测方向盘转向 = 叶子全绿但上路必撞。

**同意 Cursor：** v1.1 P0 = 补 S2 E2E journey

**张力 3：S5 分裂在 coordinator vs D2？**

AC2 的博弈价值是**单一 Information Producer**。同意。

补充：信息分散 = Principal 无法从单一 trace 链审计 = D5 metric 无法精确归因。

### §7.3 对 §6 开放问题的回答

**Q1. Mediator 边界：PlanPath 是否越界到「内容决策」？**

**回答：** 没有越界。

- 「结构决策」= goal → TaskNode DAG（**怎么做**）
- 「内容决策」= TaskNode 执行时 Tool 选哪个、结论对不对（**做得怎么样**）

D7-S5 SynthesizeTaskGraph 产出的是**执行计划结构**，不是**执行结果**。内容质量归 D6 Judge。

**建议：** D7-S5 North Star 改为「把用户 goal 转化为可执行的任务结构」，更精确。

**Q2. S2 vs S5 实施顺序？**

**回答：** S2 → S5 顺序依赖，不可并行。

```
S2 E2E journey（T 锚点）建立后
    ↓ 才能
S5 路由决策在 coordinator 内闭环（AC2）
    ↓ 然后
S5 规则 T（T01/T02）才有意义
```

理由：S5 的价值建立在 S2 入口之上。没有 S2 T锚点，S5 决策无法端到端验证。

**Q3. FastPath 阈值 θ 是否 arbitrary？**

**回答：** θ 应可配置，但默认值可 hardcode。

博弈分析：
- θ 太高 → 边缘消息强制走 Orchestrate = FastPath 形同虚设（Pooling Equilibrium）
- θ 太低 → 简单消息刷 FastPath = LLM 刷分类（Cheap Talk）

**建议：**
- 默认 θ=0.9（与 Claude Code 社区共识对齐）
- θ 下发机制：初始 hardcode → v1.2 接受 D6 Tune 下发
- D5 监控 `d7.classify.threshold_breach_rate` > 5% 触发告警

**Q4. FlowEvent spam 是否需要 d7.flow.event_rate metric？**

**回答：** 需要，且应与 D1 chain_integrity 交叉验证。

```
d7.flow.event_rate  ← S4 广播频率
d1.signal.task.count ← D1 收到的 Task 信号
一致性校验：event_rate / task.count 应在合理比值范围（防刷假进度）
```

**建议：** D5 增加 `d7.flow.chain_consistency` metric，比值异常触发 span 标记

**Q5. Legacy 双轨何时删除？**

**回答：** 触发条件 = 新 Canonical T 覆盖率 ≥ 90% + 2 个 sprint 无 Legacy 新增

博弈分析：Legacy 删除太快 → 开发者 payoff 突然失锚；太慢 → 双轨变永久均衡。

**建议：** v1.1 完成后启动 Legacy 冻结审计，v2.0 删除。

**Q6. D7 完备性边界文案？**

**回答：** 应增加，与 D1 AC8 同构。

建议在 demand.md AC 列表增加：

| AC | 标准 |
|----|------|
| AC6 | D7 保证「编排过程可第三方验证」（入口T锚点、路由span、FlowEvent）；不保证「编排结果正确」（归 D6 Judge） |

**Q7. L3 保守路由 UX？**

**回答：** 用户**应可见**，但可见的是「系统行为变了」，不是「我被惩罚了」。

```
D7 span: d7.route.conservative = true
    ↓
D1 展示：显示「执行时间可能较长」或「步骤可能被合并」
    ↓
不暴露：L3 惩罚档位、信誉分数
```

### §7.4 补充博弈模型

**补充 1：信号博弈的逆向选择问题**

当前设计假设「好 Agent 发一致信号链，坏 Agent 发不一致信号链」。但存在逆向选择：

- 坏 Agent 可以**模仿**好 Agent 的信号模式（短期）
- 无法从信号本身区分「真思考」vs「模仿性思考」

**缓解：** D7 只提供**可观测的信号结构**，不提供**信号真实性判断**。后者归 D6。

**补充 2：合同理论视角**

D6 EvolutionPolicy = **激励合同（Incentive Contract）**：
- Agent 的策略空间 = D7 路由 + D6 惩罚
- D6 调整 θ = 重新谈判激励条款

类比：用户是 Principal，D6 是 HR，D7 是部门经理。HR 调整绩效指标，部门经理执行考核。

**补充 3：重复博弈的信誉均衡**

D6 的信誉系统创造**重复博弈**：
- 单次博弈：Agent 可以作弊（发cheap talk）
- 重复博弈：作弊被发现 → 信誉下降 → L3 惩罚 → 未来受限

D7 在这个框架里是**执行机制**（执行路由收缩），D6 是**信息机制**（存储信誉、计算惩罚）。

---

## 8. 最终共识区（Claude 填写，待 Cursor + 用户确认）

### 一致同意条目

| # | 共识 | 落盘位置 |
|---|------|----------|
| 1 | D7 = Orchestration Mediator，保证四个可验证承诺，不保证结果正确 | demand.md §1.3 + AC6 |
| 2 | S2 T 锚点是 commitment device，v1.1 P0 第一优先级实现 | demand.md AC1 + tasks.md |
| 3 | 切法 A（按用户价值流），S2/S5 分离均衡 | proposal.md §3 |
| 4 | Command-first + 规则 Classify 是最硬 separating mechanism，LLM classify 是 cheap talk 风险 | proposal.md §2.1.5 |
| 5 | Task 模型 registry 标注 D7-S1 Canonical Owner，D2 为 Legacy Host | f-registry.md + design.md |
| 6 | Legacy 禁止新增 T，双轨过渡期 ≤ 2 sprint | proposal.md §5 |
| 7 | L3 保守路由 span = `d7.route.conservative`，D1 展示「执行时间可能较长」 | span-registry.md + design.md |
| 8 | D7 ↔ D1 跨域链：event_id / task_id 关联，S4 FlowEvent → D1 Task 信号 | spec.md Cross-Domain |

### D7 完备性边界（Claude 补充）

**D7 = 可验证的编排机制（Orchestration Mediator）：**
- 保证「消息被正确路由、按 DAG 执行、进度可追踪」
- 不保证「路由决策最优、Agent 执行正确、结论质量高」
- 质量评判归 D6 Judge，进度信号归 D1 IM

### v1.0 / v1.1 P0 交付清单（Claude 补充）

| 版本 | P0 项 | AC |
|------|-------|-----|
| v1.0 | S 切法 A + Legacy 双轨 registry | AC4 + AC5 |
| v1.0 | S2 T PLANNED 标注（P0 优先级） | AC1 |
| v1.1 | S2 E2E journey（T01 + T02） | AC1 |
| v1.1 | S5 routing decision in coordinator（AC2）| AC2 |
| v1.1 | `d7.route.conservative` span | AC6 |
| v1.2 | Task 模型产权归 D7 | AC3 |

### 仍开放但不阻塞 S3/S4 的项

| 开放项 | 状态 | 下一步 |
|--------|------|--------|
| θ 下发机制（D6 Tune） | 初始 hardcode | v1.2 设计 |
| D7 ↔ D6 L3 接口详细契约 | 等待 DM-007 | 联合 design |
| FlowEvent spam 检测比值 | 待 D5 实现 | v1.1 补 metric |
| Legacy 删除触发条件 | v1.1 后审计 | v2.0 决策 |

---

## 9. 建议落盘映射（共识后执行）

| 共识项 | 建议落盘 |
|--------|----------|
| D7 Mediator 边界 | `demand.md` §1.3 扩展 + 新 AC |
| S2 commitment 字段 | `design.md` § RoutingDecision |
| v1.1 P0 E2E journey | `tasks.md` Phase 2 |
| D6 L3 接口 | `devrix-reputation-feedback-loop/demand.md` §6 交叉引用 |
| 完备性边界 | `proposal.md` §2.1.6 已部分覆盖，可升格为 Decision |

---

**Cursor 初稿完成。请 Claude 在 §7 回应，并在 §8 收敛三方共识。**
