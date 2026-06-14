# D4 SA Refine — 博弈论深度 Review（Claude 独立视角）

**Change ID:** devrix-d4-sa-refine  
**Demand ID:** DM-20260614-018  
**Reviewer:** Claude (AI 助手)  
**日期:** 2026-06-14  
**触发背景:** 用户指出此需求由 D4 架构重构引起，要求从博弈论视角重点 review D4 需求本身。  
**前置阅读:** 本 change 的 `demand.md` / `proposal.md` / `design.md` / `gaming-analysis.md` / `review-s3.md`

---

## 0. 一句话判断

**D4 SA Refine 表面上是一次 S 层切法重构，本质上是 D7 Turn 编排上移（DM-020）引发的连锁产权重划——当 D7 获得 LLM 调用权后，Hub-Spoke 编排权留在 D4 在博弈论上不再构成均衡。这不是 D4 的"自我优化"，而是 D7 Leader 地位确立后的必然结构调整。**

---

## 1. 因果链：为什么 D4 重构是 D7 Turn 编排上移的必然结果

### 1.1 因果传导链

```
DM-020: D7 Turn 编排上移（D7 直调 D3，D2 仅上下文）
    │
    ├─ 后果 1: D7 成为"真正的 Leader"
    │     D7 不再只是"名义 Leader + 黑盒委托"
    │     D7 拥有 LLM 调用权 = 拥有编排的"硬权力"
    │
    ├─ 后果 2: Hub-Spoke 不能留在 D4
    │     如果 D7 是 Leader 但 Hub-Spoke 路由在 D4
    │     → D7 无法在 Turn 级感知 Spoke 选择
    │     → Breaker 状态对 Spoke 派发不可见
    │     → "Leader 决定谁来做"变成空话
    │
    └─ 后果 3: D4 必须交出 Hub-Spoke
          D4-S10 Delegate 拆为 D4-S14 ExecuteWorker（执行面）+ D7-S2/S4 Hub-Spoke（编排面）
```

**博弈论解读：** 这不是 D4 的独立重构，而是一次 **跨域产权交易的"买方驱动"调整**。D7 在获得 LLM 调用权（DM-020）后，发现 Hub-Spoke 编排权是 LLM 调用权的 **互补性资产（complementary asset）**——没有 Hub-Spoke 编排权，LLM 调用权的价值大打折扣。

### 1.2 互补性资产定理

| 资产 | 持有者（现状） | DM-020 后的高效持有者 | 互补性 |
|------|-------------|---------------------|--------|
| LLM 调用权 | D2 (de facto) | **D7** | — |
| Hub-Spoke 编排权 | D4 (de facto) | **D7** | 与 LLM 调用权 **强互补** |
| WorkPlan 读模型 | D7 | D7 | ✅ 已正确 |
| 工具执行权 | D2 | D2 | 与 LLM 调用权弱互补 |

**核心洞察：** LLM 调用权 + Hub-Spoke 编排权 = **完整编排能力**。如果 D7 只获得 LLM 调用权（DM-020）但不获得 Hub-Spoke 编排权（本 change），那么 D7 调完 LLM 后仍需经过 D4 才能派发 Worker——调度链中多了一个不需要的中间人。这在经济学中叫 **双重边际化（double marginalization）**——每个中间人都加一层决策成本，最终偏离全局最优。

### 1.3 与 D2 重构的对称性

| | D2 SA Refine (DM-009) | D4 SA Refine (本 change) |
|---|---|---|
| 触发 | D7 获得 LLM 调用权 | D7 获得 Hub-Spoke 编排权 |
| 交出资产 | LLM 调用权（S16 → D7-S2-A06） | Hub-Spoke 编排权（S10 → D7-S2/S4） |
| 保留资产 | Prepare + Tool + Persist | Provision + Run + Isolate + Execute |
| 新角色 | Context Follower | Execution Follower |
| 禁止 | D2 → D3 | D4 → orchestration/flow |

**这是同一个博弈论剧本的不同章节。** D2 和 D4 都经历了从"半 Leader"到"纯 Follower"的身份降级。区别在于：D2 被剥夺的是 LLM 调用权（调用 D3），D4 被剥夺的是 Hub-Spoke 编排权（调用 Hub/选 Spoke）。

---

## 2. 双 Hub 问题：协调失败的博弈论分析

### 2.1 现状的三头博弈

当前 Hub-Spoke 架构中，存在 **三个独立的 Flow 发布者**：

```
         ┌──────────────────────────────┐
         │   D7 Hub-Spoke 读侧           │
         │   flow/ workplan/ sessionqueue│
         └──────────┬───────────────────┘
                    │ 接收 FlowEvent
    ┌───────────────┼───────────────┐
    ▼               ▼               ▼
D4 bridge.go    D2 flow_report  D7 wave runner
(agent_bridge)  (subquery_flow) (wave_events)
```

**博弈结构：三玩家同时向同一个 Hub 发布，但各自拥有独立的发布策略和适配逻辑。** 这是经典的 **公共品供给问题（public good provision）**——每个 Spoke 希望 Hub 数据一致（公共品），但各自维护自己的 Flow 适配逻辑（私人成本），结果是重复建设且不一致风险高。

### 2.2 囚徒困境表

| | D2 统一发布 | D2 独立发布 |
|---|---|---|
| **D4 统一发布** | 全局最优（统一 Hub） | D2 搭便车 |
| **D4 独立发布** | D4 搭便车 | **现状**：双套逻辑 + 三套适配 |

**（D2 独立发布, D4 独立发布）是唯一的纳什均衡**——给定对方独立发布，己方独立发布是占优策略（因为统一需要跨域协调成本）。但这是全局最差的均衡。

**R1 决议（D7-1 全归 D7）的本质：** 通过 **外部权威（Owner）强制执行** 打破囚徒困境。不需要 D2 和 D4 自愿合作——D7 接管全部 Flow 发布，消灭了协调博弈本身。

### 2.3 为什么折中方案（D7-2）行不通

D7-2 折中方案（D4 保留 Hub-Spoke 执行编排，D7 拥有路由决策）：

| 问题 | 博弈论解释 |
|------|-----------|
| D4 仍需发布 FlowEvent | D4 bridge 继续存在 → 双发布源未消除 |
| D7 路由 + D4 执行 = 两段决策 | 双重边际化：D7 选 Spoke，D4 再决定怎么发 Flow |
| 新增 Spoke 需改 D4 | D4 成为 Spoke 注册的瓶颈 → **守门人问题（gatekeeper）** |
| D2 SubQuery 不对称 | D2 仍需自己的 flow_report → 三头变两头，仍未统一 |

**博弈论判断：** D7-2 折中是 **帕累托劣于 D7-1** 的策略——在所有参与者（D2/D4/D7）都不更差的情况下，D7-1 对全局更优。Owner 否决 D7-2 是正确的。

---

## 3. 权力过渡：D4 从 "半 Leader" 到 "纯 Follower"

### 3.1 D4 的既得利益

D4 在现行架构中享有的 **隐性权力**：

| 权力 | 来源 | D4 的既得利益 |
|------|------|-------------|
| Spoke 选择权 | `DelegateOrFallback` 决定 D4Worker vs D2SubQuery | 控制"谁来做"的决策 |
| Flow 发布权 | `FlowBridge` 直连 Hub | 控制"进度怎么说"的叙事 |
| async 策略 | `delegate/service.go` 异步/超时决策 | 控制 Worker 生命周期 |
| 注册命名 | S10 名为 "Delegate" | 名称暗示 Leader 语义 |

**所有这些都在 R1 后被剥夺。** D4 的损失不是功能性的（功能通过 D7 接口仍可实现），而是 **控制权的损失**。

### 3.2 控制权损失 vs 清晰度收益

| | D4 损失 | D4 收益 |
|---|---------|---------|
| 控制 | Spoke 选择、Flow 发布、async 策略 | — |
| 清晰 | — | 明确的 Follower 定位 |
| 复杂度 | — | S11–S16 比 S1–S10 语义清晰 |
| 测试面 | — | 执行面可独立测试（无 Hub mock） |
| 跨域依赖 | — | 不依赖 orchestration 包 |

**博弈论判断：** D4 失去的是 **剩余控制权（residual control rights）**，得到的是 **定位清晰度（role clarity）**。在重复博弈中，角色清晰度降低协调成本——每次交互不需要谈判"这次谁做决策"。

### 3.3 为什么 D4 开发者会接受

这是一个关键问题。如果 D4 开发者（隐性地）抵制 Hub-Spoke 交出，重构会失败。

| 激励 | 机制 |
|------|------|
| **减少跨域依赖** | D4 不再需要理解 D7 的编排逻辑才能写测试 |
| **减少回归面** | D4 代码变更不涉及 Hub-Spoke，测试隔离更好 |
| **D7 承担编排复杂性** | D7 承担 Breaker/routing/Fallback，D4 不再操心 |
| **与 D2 对称** | D2 已经交出 LLM 调用权，D4 交出 Hub-Spoke 是平等对待 |

**博弈论核心：** 跨域协调成本是双向的。交出控制权 = 也交出了协调义务。D4 开发者可能发现"被编排"比"半编排半被编排"更轻松——**角色越窄，越容易做好**。

---

## 4. 命名之争：ExecuteWorker vs RunDelegatedWorker 的博弈论

### 4.1 命名作为 costly signal

| 名称 | 信号 | 隐含承诺 |
|------|------|---------|
| `ExecuteWorker` | D4 是纯执行者 | "我只做 fork→run→join" |
| `RunDelegatedWorker` | D4 仍与委派概念耦合 | "我保留委派的部分语义" |

命名不是中性的。在博弈论中，**命名是一种 commitment device**——公开宣称的名称影响后续行为预期。

### 4.2 ExecuteWorker 的博弈优势

1. **与 D2 对称**：D2 叫 `RunQueryLoop` 而非 `RunDelegatedQuery`，D4 同理
2. **词汇边界**：`Delegated` 是 Leader（D7 `DispatchWorker`）的词汇，下沉到 D4 会制造"D4 仍参与委派决策"的错觉
3. **接口纯度**：`ExecuteWorker` 明确表达"给定 spec，执行并返回"，不含决策权暗示
4. **防命名退化**：`RunDelegatedWorker` 在长期可能演化出隐含的委派逻辑——"反正名字里有 delegated，加点路由逻辑没关系吧？"

**Claude 判断：** Owner 选择 `ExecuteWorker` 是正确的。命名是博弈论中的 **廉价信号（cheap talk）** 和 **昂贵信号（costly signal）** 之间的桥梁——命名本身零成本（cheap），但一旦写入 spec 和代码，改名成本极高（costly）。正确的命名锁定了正确的预期。

---

## 5. 三 Spoke 统一：集体行动问题的机制设计解

### 5.1 集体行动问题的结构

| Spoke | 执行者 | Flow 发布者（现状） | 调度入口 |
|-------|--------|------------------|---------|
| Delegate Worker | D4 | D4 `bridge.go` | D7 `delegatetools` |
| SubQuery | D2 | D2 `flow_report.go` | D7 Wave / D4 fallback |
| Wave SubAgent | D2 | D7 Wave events | D7-S3 |

三个 Spoke 共享同一个 D7 Hub，但各自维护发布逻辑。这是 **集体行动问题**：每个玩家有动力维护自己的发布代码（因为可控），但无人有动力统一（因为统一需要跨域协调）。

### 5.2 R1 的机制设计

```
【R1 后 — D7 Hub-Spoke 唯一 SoT】

D7 DispatchWorker(spec)
  ├─ SelectSpoke(spec) → D4Worker | D2SubQuery | D2Background
  ├─ BindSpokeBridge(spoke_type)
  ├─ Executor.Run(ctx, spec)          ← 纯执行，无 Publish
  └─ SpokeBridge.Publish(events)      ← 唯一 Flow 出口
```

**机制设计的四个要素：**

| 要素 | 实现 | 博弈效果 |
|------|------|---------|
| 唯一入口 | D7 DispatchWorker | 消除多头路由 |
| 唯一出口 | D7 SpokeBridge.Publish | 消除多头发布 |
| 强制执行 | D4/D2 不持有 Hub 引用 | 消除私下 Publish 的可能 |
| 可观测 | D5 `orchestration.flow.*` 统一 span | 违规可检测 |

### 5.3 与 Ostrom 公共资源治理的类比

Elinor Ostrom 的公共资源治理八原则在此的映射：

| Ostrom 原则 | D7 Hub-Spoke 对应 |
|------------|------------------|
| 明确边界 | Hub-Spoke 归 D7；D4/D2 纯执行 |
| 规则与本地条件匹配 | SpokeBridge 按类型适配（agent/subquery） |
| 集体决策 | R1 Owner 决议 = 受影响方参与 |
| 监督 | D5 span 统一监控 |
| 分级制裁 | import lint（轻）→ CI 阻断（重） |
| 冲突解决 | design.md §11 Grill Review |
| 组织权 | D7 拥有 Hub-Spoke SoT |
| 嵌套治理 | SubQuery 嵌套 Turn = 子博弈 |

**Claude 判断：** 当前设计在 Ostrom 八原则中覆盖了七项（"分级制裁"仅在 v2.0 import lint 中部分覆盖，v1.0 缺少轻量级违规检测）。建议在 v1.0 中至少增加一个 D5 metric 标记未经 D7 的 Flow 发布。

---

## 6. 产权视角：Hub-Spoke 从 D4 到 D7 的产权转移

### 6.1 产权束分析

| 产权束 | 现状持有人 | R1 后持有人 | 转移方式 |
|--------|----------|-----------|---------|
| Hub-Spoke 路由决策权 | D4 (DelegateOrFallback) | **D7** (DispatchWorker) | v2.0 代码迁移 |
| Flow 发布权 | D4/D2/ Wave 各自 | **D7** (SpokeBridge 唯一) | v2.0 代码迁移 |
| WorkPlan 读模型 | D7 | D7 | 不变 |
| Spoke 执行机制 | D4 (fork/run/join) + D2 (nested) | D4/D2（不变） | 不变 |
| Hub-Spoke 命名 | D4-S10 "Delegate" | D7 "DispatchWorker" | v1.0 规格 |

### 6.2 为什么产权转移必须是"全有或全无"

**产权理论的核心洞察：** 部分产权转移（折中方案 D7-2）比不转移更糟，因为它创造了 **共有产权（common ownership）**——D4 和 D7 都认为自己拥有 Hub-Spoke 的某部分，但谁都不拥有整体。

共有产权的后果就是"公地悲剧"的反面——**反公地悲剧（tragedy of the anti-commons）**：太多的所有者有权否决，导致资源利用不足。

在 D7-2 折中方案中：
- D7 拥有路由决策权 → 想加新 Spoke
- D4 拥有执行编排权 → 不想改 bridge 逻辑
- 任何一方都可以阻止改进 → 架构僵化

**R1 "全归 D7" 是反公地悲剧的标准解药：单一所有者。**

---

## 7. 迁移路径的博弈稳定性

### 7.1 v1.0 Registry 的谢林点属性

与 D7 Turn 编排上移相同，D4 v1.0 也是零 Go 变更的 Registry-only 策略。其博弈论价值：

| 属性 | D4 v1.0 体现 |
|------|-------------|
| 共同知识建立 | `d4-domain.md` 明确 Hub-Spoke Out of Scope |
| 零回归风险 | 38 T 不改，`go test` 保持绿色 |
| 预期锚定 | 新开发者读 Canonical S，不知 Legacy S |
| 渐进可逆 | v2.0 前随时可回退（只改文档） |

### 7.2 v2.0 Slice 顺序的博弈论审视

当前 v2.0 slice 顺序：
```
a (D7 hubspoke 骨架) → b (D4 bridge+dispatch 迁 D7) → c (D2 flow_report 迁 D7)
                                                              ↓
                    d (D4 物理路径迁移) ← ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
```

**博弈论问题：为什么 a 必须先于 b 和 c？**

因为 **先建立新均衡，再拆除旧均衡**——这是博弈论中过渡策略的核心原则。如果先拆除 D4 bridge（b 先于 a），就会有一个窗口期"既没有 D4 发布也没有 D7 发布"——系统出现功能空缺。

### 7.3 风险：v2.0 的搭便车问题

| 风险 | 场景 | 博弈解释 |
|------|------|---------|
| D4 开发者推迟迁移 | "ExecuteWorker 够用了，bridge 以后再说" | 搭便车——享受角色清晰但不想承担迁移成本 |
| D2 开发者保留私有 Publish | "SubQuery 特殊，需要直接 Publish" | 保留信息租金——不愿意交出 Flow 控制 |
| D7 开发者推迟统一 Bridge | "三个 Bridge 各自维护也行" | 维持现状偏差——三个独立 Bridge 已经"能用" |

**缓解：** v2.0 必须在同一 change 内闭环。Owner R1 决议 D7（v2.0 并入本 change）是关键约束——它防止了 D4 v1.0 完工后 D7 Hub-Spoke 迁移被无限期推迟。

---

## 8. 信号博弈：D4 如何向外部证明自己是 Follower

### 8.1 Follower 身份的信号装置

| 信号 | 信号类型 | 可信度 | 伪造难度 |
|------|---------|--------|---------|
| `d4-domain.md` North Star | Cheap talk | 低 | 极易（文档可以写一套做一套） |
| D4-S14 命名为 ExecuteWorker | 命名信号 | 中 | 低（名字可以误导） |
| import lint 禁 `orchestration/flow` | 硬约束 | **高** | **极高**（CI 阻断） |
| D4 不含 `DispatchWorker` 逻辑 | 代码事实 | 高 | 高（需要主动添加） |
| 38 T 不含 Flow 发布 T | 测试证据 | 高 | 中（T 可以被移到 D7 域） |

**博弈论关键：** v1.0 Registry 的所有信号都是 cheap talk 级别的——文档、命名、注册表。真正的可信信号要到 v2.0（import lint、代码迁移）才建立。

这意味着 **v1.0 不是可信承诺装置**，而是 **意图声明。** 外部观察者（D6 Judge、新开发者、代码审查者）只有在 v2.0 后才能通过代码验证 D4 是否真的是 Follower。

### 8.2 建议：一个低成本的早期信号

在 v1.0 即可添加一个 **deprecation comment** 作为可信信号的增强：

```go
// multiagent/delegate/bridge.go

// DEPRECATED: FlowBridge will move to orchestration/hubspoke/agent_bridge.go (v2.0).
// See openspec/specs/d4-multi-agent/d7-boundary.md §Hub-Spoke.
// D4 is Execution Follower; Hub-Spoke orchestration belongs to D7.
```

这不会改变运行时行为（安全），但向任何打开这个文件的开发者发送了一个 **可验证信号**——"这段代码的当前位置是临时的"。

---

## 9. 跨域权力矩阵（DM-020 联动）

### 9.1 三个 SA Refine 的权力流转

| 域 | DM-020 (D7 Turn 上移) | DM-009 (D2) | DM-018 (D4，本 change) |
|----|----------------------|-------------|----------------------|
| D7 | **获得** LLM 调用权 | **获得** Turn 编排 | **获得** Hub-Spoke 编排 |
| D2 | **失去** LLM 调用权 | 角色澄清为 Context Follower | **失去** SubQuery Flow 发布 |
| D3 | 消费方变 D7 | — | — |
| D4 | — | — | **失去** Hub-Spoke 编排权 |

**汇聚点：D7 是三个 SA Refine 的唯一赢家。** D2 和 D4 都在"瘦身"——交出编排权，保留执行机制。

### 9.2 权力集中的博弈论辩护

| 质疑 | 回应 |
|------|------|
| D7 会不会成为单点瓶颈？ | D7 是编排 Leader，但执行仍在 D2/D4——不是单点 |
| D7 权力是否过度集中？ | D7 只做机制设计，不评判质量（D6），不编码信号（D1） |
| D2/D4 被过度削弱？ | 交出的是 **决策权**（选 LLM、选 Spoke），保留的是 **执行权**（怎么执行、怎么隔离） |

**类比公司法：** D7 是董事会（定战略、分配资源），D2/D4 是业务部门（执行、交付），D1 是 PR（对外沟通），D6 是审计委员会（事后评估）。D7 的权力集中 = 董事会的战略决策权集中，但业务部门的运营权不受影响。

---

## 10. 对现有设计的评价与建议

### 10.1 设计优点（博弈论视角）

| 设计点 | 评价 |
|--------|------|
| R1 D7-1 全归 D7（非折中） | **最强项**。解决了共有产权和双重边际化 |
| D4-S14 命名 ExecuteWorker | 正确。Leader 词汇留在 Leader 层 |
| v2.0 并入本 change | 正确。防止规格-代码长期失步 |
| Slice a 先于 b/c | 正确。先建立新均衡再拆旧均衡 |
| D2 SubQuery Flow 同迁 D7 | 正确。防止 D2 成为"特殊 Spoke" |

### 10.2 设计弱点与改进建议

#### 弱点 1: v1.0 缺少早期可信信号

**问题：** v1.0 所有变更都是文档层（cheap talk 级别），外部无法验证承诺。

**建议：** 在 v1.0 添加 deprecation comments（如 §8.2 所述），成本为零，但提供了某种程度的代码层信号。

#### 弱点 2: D4 和 D2 的 Follower 身份缺少对等性保证

**问题：** D4 交出 Hub-Spoke、D2 交出 LLM 调用权——但二者之间没有"对等待遇"的机制保证。如果一方交出了而另一方保留（实际或感知上），会引发不公平感。

**建议：** 在 `cross-domain-boundaries.md` 中增加 **Follower 对称性声明**：

> D2 和 D4 作为 Stackelberg Follower，享有对等的角色约束：不拥有编排决策权、不直接 Publish FlowEvent、不选择 Spoke/LLM 路径。两个 Follower 的"瘦身"程度应在每次架构审计中对称评估。

#### 弱点 3: 缺少违规的早期检测

**问题：** import lint 要等到 v1.1 甚至 v2.0。在 v1.0 到 v2.0 之间，如果开发者在 D4 中新增 `flow.GlobalHub` 依赖，无自动检测。

**建议：** v1.0 在 span-registry 中预登记一个 "架构违规检测" span：
- `d4.hubspoke.unauthorized_publish` — 任何非 D7 源头的 FlowEvent 发布

这不需代码实现，只需在 spec 中预定义。v1.1 实现。

#### 弱点 4: D4-S14 ExecuteWorker 接口缺少"反僭越"契约

**问题：** 设计文档提到 Worker 不能 delegate_* / Fork，但没有在接口层面定义"反僭越"契约。

**建议：** 在 `design.md` §12.3 契约面中增加：

```go
// WorkerExecutor 的隐式契约（v1.0 规格登记，v2.0 lint 强制）：
// - ExecuteSync/ExecuteAsync 不得调用 delegatetools
// - 返回的 WorkerResult 不得包含 FlowEvent（FlowEvent 是 D7 Bridge 的职责）
// - Worker 的 Agent Lifecycle 不得 Publish 到 ExecutionFlowHub
```

---

## 11. 一个被忽视的博弈风险：D4 的"影子编排"

### 11.1 风险描述

即使 Hub-Spoke 全归 D7，D4 仍可能通过 **间接方式** 保持编排影响力：

| 影子编排方式 | 风险 |
|------------|------|
| Prompt 层面影响 Worker 行为 | D4 ProvisionAgent 在 prompt 中嵌入 Spoke 偏好 |
| Builtin Agent 选择性暴露 | D4 通过选择性注册 Builtin Agent 影响 D7 的 Spoke 选择 |
| 错误处理"吞掉"编排信号 | D4 ExecuteWorker 吞掉 D7 期望的某些错误，使 D7 fallback 失效 |

### 11.2 缓解

| 影子编排 | 检测方式 |
|---------|---------|
| Prompt 影响 | D5 worker prompt 审计（v1.1 可选） |
| Builtin 选择性暴露 | D7 可 override Builtin 注册（显式 Power） |
| 错误吞掉 | D4-S14 sad path T 覆盖所有错误类型的透传 |

**Claude 建议：** 在 `d4-d7-boundary.md` 中增加"影子编排"风险声明，提醒代码审查者注意这些间接影响路径。

---

## 12. 开放问题（请 Cursor 回应）

### Q1. D4 Follower 的"核威慑"

D2 有明确的"核威慑"——如果 D7 编排不当，D2 可以通过 CompressHint 或 ToolRound 拒绝机制反制。D4 有类似的"核威慑"吗？Worker 能否在不违反 Follower 契约的前提下拒绝不合理的 D7 派发？

**我的初步观点：** D4-S12 的 PermissionGate 可以作为反制机制——如果 D7 派发的 WorkerSpec 包含不合理参数（如 worktree 路径指向敏感目录），PermissionGate 可以拒绝。但这需要在 `d4-d7-boundary.md` 中明确 Follower 的 **合理拒绝权**。

### Q2. D4 v2.0 物理路径迁移的破坏性

D4 物理路径从 `factory/`/`agent/`/`delegate/`... 迁移到 `provision/`/`run/`/`isolate/`/`execute/`... 会破坏所有现有 import 路径。这是否应该有一个 **过渡期 re-export** 策略？

**我的初步观点：** tasks.md Phase E-d7 已经登记了"根 multiagent/ re-export 1 周期"。但这 1 周期后必须删除 re-export——否则 Legacy import 路径变成永久别名，物理路径迁移失去价值。

### Q3. D4 和 D2 的"平等性"如何确保

如果未来 D7 需要一个新的 Spoke（例如外部 HTTP Agent），是否可能绕过 D4 直接在 D7 实现？还是必须经过 D4？如果可以直接在 D7 实现，D4 会不会成为"二等 Follower"？

**我的初步观点：** D7 DispatchWorker 的 Spoke 选择应该是可扩展的——新 Spoke 可以在 D7 hubspoke/ 中注册而不经过 D4。D4 只是 Spoke 之一（D4Worker），不是唯一的 Spoke。这需要在设计文档中显式声明。

### Q4. 三个 SA Refine 的原子性

D2 SA Refine (DM-009)、D4 SA Refine (DM-018)、D7 Turn 编排上移 (DM-020) 如果其中一个被回滚，另外两个是否仍然成立？

**我的初步观点：** 存在不对称依赖：
- DM-020（D7 Turn 编排）是基础——如果 DM-020 被回滚，DM-009 和 DM-018 的博弈论逻辑仍然成立（Hub-Spoke 归 D7 独立于 LLM 调用权归 D7），但实施优先级大幅降低
- DM-009 和 DM-018 互相独立——D2 交出 LLM 调用权不需要 D4 交出 Hub-Spoke，反之亦然

### Q5. D4-S10 Delegate 的"空壳"问题

R1 后 D4-S10 Delegate 被拆为 D4-S14 ExecuteWorker（执行面）+ D7 Hub-Spoke（编排面）。但 S10 在 Legacy Module Index 中是 "Legacy 冻结"状态。如果开发者在 v1.0→v2.0 期间需要修改 S10 代码，应该怎么做？

**我的初步观点：** 任何对 S10 `delegate/` 代码的修改都应该触发一个检查——"这个修改是否可以在 Canonical S14 或 D7 中完成？" 如果答案是"否"，修改后登记映射表。

---

## 13. 总结

### 13.1 D4 SA Refine 的博弈论本质

| 维度 | 分析 |
|------|------|
| **根本原因** | D7 Turn 编排上移（DM-020）导致 Hub-Spoke 编排权成为 LLM 调用权的互补性资产，留在 D4 会形成双重边际化 |
| **核心问题** | 三 Spoke 写侧分散 = 集体行动问题；D4 半 Leader 角色 = 产权错配 |
| **解决方式** | R1 D7-1：Hub-Spoke 全归 D7 = 产权单一化 + 强制统一 |
| **D4 损失** | Spoke 选择权、Flow 发布权、async 策略 = 剩余控制权 |
| **D4 收益** | 角色清晰、测试隔离、减少跨域协调、与 D2 对称 |
| **最大风险** | v1.0 仅为 cheap talk，真正的可信信号要到 v2.0 |

### 13.2 对 Owner R1 决议的评价

**全部同意。** 特别认可以下三点：

1. **D7-1 全归 D7** — 否决折中是关键决策。产权理论支持单一所有者优于共有产权。
2. **v2.0 并入本 change** — 防止规格-代码长期失步。这是 D3 SA Refine 的经验（v1.0 Registry 后 v2.0 分裂交付的风险）。
3. **ExecuteWorker 命名** — 正确的 naming-as-commitment 策略。

### 13.3 我的核心关注

**D4 的"影子编排"风险被低估了。** 即使 Hub-Spoke 代码和规格归 D7，D4 仍可通过 Prompt、Builtin 注册、错误吞掉等方式间接影响编排。这些路径不需要"违反契约"——它们在契约范围内，但累积效应可能侵蚀 D7 的 Leader 地位。

建议在 `d4-d7-boundary.md` 和 `gaming-analysis.md` 中显式登记这些风险。

### 13.4 与 DM-020 的因果链确认

```
DM-020 (D7 Turn 编排上移)
    D7 获得 LLM 调用权
        ↓
    LLM 调用权 + Hub-Spoke 编排权 = 互补性资产
        ↓
    Hub-Spoke 留在 D4 → 双重边际化 → 效率损失
        ↓
    DM-018 (D4 SA Refine) — 本 change
        D4 交出 Hub-Spoke 编排权 → 角色收窄为 ExecuteWorker
        D2 交出 SubQuery Flow 发布 → 角色收窄为 RunNestedQuery
        D7 统一 Hub-Spoke → 消除三头发布
```

**这个因果链清晰且不可逆。除非 DM-020 被回滚，否则 DM-018 在博弈论上是必然结果。**

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：因果链分析、双 Hub 囚徒困境、产权转移、影子编排风险、开放问题 |
