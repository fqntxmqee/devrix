# D7 Turn 编排上移 — 博弈论深度分析（Claude 独立视角）

**Change ID:** devrix-d7-turn-orchestration  
**Demand ID:** DM-20260614-020  
**作者:** Claude (AI 助手)  
**日期:** 2026-06-14  
**目的:** 从博弈论角度深度分析 D7 Turn Leader 重构，补充现有 `gaming-analysis.md` 未覆盖的维度，供后续 Cursor 对焦讨论。

---

## 0. 前置阅读

- `gaming-analysis.md`（本 change，76 行概览）
- `archive/2026-06-14-devrix-d7-sa-refine/gaming-analysis.md`（D7 S 层重构，三方共识）
- `archive/2026-06-14-devrix-d2-sa-refine/gaming-analysis.md`（D2 Follower 博弈）
- `demand.md` / `design.md` / `proposal.md`（本 change）

**本文档与现有 `gaming-analysis.md` 的关系:** 现有分析聚焦于 Stackelberg 角色重划 + 机制设计表。本文档补充六个更深层的博弈论视角，特别是产权理论、不完全合约、递归博弈和过渡期协调问题。

---

## 1. 核心博弈论判断

### 一句话

**D7 Turn 编排上移不是架构洁癖，而是一场产权革命——把 LLM 调用权从 D2 的"事实占有"变为 D7 的"法律所有"，以此消除当前架构中最危险的廉价对话均衡。**

### 1.1 与现有分析的差异

| 维度 | 现有 gaming-analysis.md | 本文档补充 |
|------|------------------------|-----------|
| 核心框架 | Stackelberg Leader/Follower | **Coase 产权定理 + 不完全合约** |
| 激励分析 | 局部最优 vs 全局最优表 | **跨域谈判博弈 + 威胁点** |
| 机制设计 | Turn 状态机 + import lint | **承诺装置的可信度梯度** |
| 过渡期 | Legacy 双轨 | **混合策略均衡与 Schelling 点** |
| 递归结构 | 未涉及 | **SubQuery/Compress 的嵌套博弈** |
| 信息结构 | 信号信息表 | **信息租金与战略性信息隐瞒** |

---

## 2. 产权视角：LLM 调用权的 Coase 定理分析

### 2.1 问题的产权本质

当前架构的核心问题可以用 **Coase 定理** 重新表述：

> 如果产权明晰且交易成本为零，无论初始产权如何分配，资源最终会流向最高效的使用者。

当前状态：
- **LLM 调用权的 de facto 产权在 D2**（S16 RunQueryLoop 内部直连 D3）
- **LLM 调用权的 de jure 产权理应在 D7**（Orchestration Leader）
- **交易成本不为零**（跨域接口变更、测试迁移、认知负担）

Coase 的洞见是：**当 de facto 和 de jure 产权分离时，高交易成本会锁定低效均衡。** D2 持有 LLM 调用权不是因为它更高效，而是因为"先占"——历史上 QueryLoopExecutor 就是这么写的。

### 2.2 当前低效的证据

| 现象 | 产权解释 |
|------|---------|
| Breaker 状态 D7 不可见 | D2 独占 LLM 通道，隐藏了供应商状态信息 |
| Autocompact 在 D2 内直连 D3 | D2 将 LLM 调用权延伸到压缩领域 |
| SubQuery 内循环不可编排 | 嵌套 LLM 调用形成了 D7 不可进入的"私有领地" |
| FastPath 不可观测 | D7 委托后失去 visibility，等于放弃了编排权 |

### 2.3 产权明晰化的三个层次

| 层次 | 措施 | 博弈效果 |
|------|------|---------|
| **L1: 规格产权** (v1.0) | 在 spec 中明确 D7 是 ILLMGateway 的唯一消费方 | Schelling 点——协调预期 |
| **L2: 代码产权** (v2.0) | import lint 禁止 D2→D3 | 硬约束——改变策略空间 |
| **L3: 运行时产权** (v2.0) | TurnOrchestrator 持有 LLM 调用链 | 可验证——D5 span 锚定 |

**Claude 观点:** L1 不是"仅仅是文档"——在博弈论中，**共同知识（common knowledge）的改变本身就是均衡迁移的第一步。** 一旦所有开发者知道"D7 拥有 LLM 调用权"是共识，原有的 D2→D3 路径就变成了"明知故犯"，这改变了违规的心理成本。

---

## 3. 不完全合约视角：为什么需要 CompressHint 灰区

### 3.1 合约的不完全性

D7↔D2 的接口契约本质上是 **不完全合约（Incomplete Contract）**：

| 可合约化 | 不可合约化 |
|---------|-----------|
| Prepare 输入/输出类型 | token 预算精确超限的"正确"处理方式 |
| ToolRound 执行权限门禁 | 什么情况下应拒绝 tool call 是"过度"还是"合理" |
| Persist 持久化完成 | 持久化前是否要做额外的上下文清洗 |

**Autocompact 灰区是不完全合约的典型案例：** D2 检测 token 超限（可合约化），但"是否真的需要摘要"和"摘要到什么程度"依赖于模型、消息结构、用户意图等不可完全枚举的状态。

### 3.2 产权分配作为不完全合约的补充

当合约无法穷举所有情况时，**产权归属决定剩余控制权（residual control rights）**：

| 设计选择 | 产权含义 |
|---------|---------|
| CompressHint 由 D2 发出 | D2 拥有"请求压缩"的提议权 |
| 实际 LLM 调用由 D7 执行 | D7 拥有"是否以及如何压缩"的决定权 |
| D2 MergeSummary 合并结果 | D2 拥有"如何整合压缩结果"的执行权 |

这是一个精妙的 **权力分立** 设计：
- D2 不能自己执行压缩（防止"为了省 token 而过度压缩"的局部最优）
- D7 不能自己判断何时需要压缩（因为没有 token 计数的私有信息）
- 两者互相制衡，形成 **制度化的不信任**

### 3.3 与公司治理的类比

| 公司治理 | D7 Turn 编排 |
|---------|-------------|
| 董事会（D7） | 批准重大决策（是否调 LLM） |
| 管理层（D2） | 提出方案（CompressHint）、执行日常运营（Prepare/Tool/Persist） |
| 审计委员会（D6） | 事后评估决策质量 |
| 股东（用户） | 观察 output 决定是否"退出"（/stop） |

**关键原则：** 经营权与所有权分离。D2 经营 LLM 相关资源（token 预算、上下文组装），但 LLM 调用的所有权归 D7。

---

## 4. 递归博弈：SubQuery 嵌套 Turn 的深层结构

### 4.1 嵌套 Stackelberg 博弈

SubQuery 包装为 D7 Turn 创建了 **递归 Stackelberg 结构**：

```
Level 0: D7.RunTurn(scope=main)
  ├── D2.Prepare
  ├── D3.StreamChat → tool_calls 中包含 SubQuery
  ├── D7.RunTurn(scope=subquery)    ← 递归！
  │     ├── D2.Prepare (nested scope)
  │     ├── D3.StreamChat (read-only tools)
  │     └── D2.Persist
  ├── D2.ExecuteToolRound (使用 SubQuery 结果)
  └── D2.Persist
```

**博弈论含义：** 每一层嵌套都是同一个 Leader-Follower 博弈的 **子博弈（subgame）**。子博弈完美均衡要求：在每一个嵌套层，Leader 的策略都是最优的，不论外层如何。

### 4.2 递归的承诺保真度

| 层级 | 承诺 | 保真度风险 |
|------|------|-----------|
| 主 Turn | D7 编排全链 | D2-S19 可能绕过 D7 自己调 D3 |
| SubQuery Turn | D7 包装同一 RunTurn | 嵌套深度过大导致取消传播失效 |
| Compress Turn | D7 调 D3 摘要 | Autocompact 回退到 D2 直连 |

**Claude 观点:** 递归结构的力量在于——如果 `RunTurn(scope=subquery)` 和 `RunTurn(scope=main)` 是**同一代码路径**，那么任何在主 Turn 上的机制改进（Breaker 感知、取消传播、span 锚定）**自动** 应用到所有嵌套层。这是在代码层实现 "子博弈完美" 的关键。

### 4.3 递归的深度与终止

递归博弈需要一个 **终止条件**：

```text
终止条件：tool_calls 中没有嵌套 SubQuery → 不需要更深层的 RunTurn
安全阀：MaxDepth = 3（主 + SubQuery + SubSubQuery）
```

**博弈含义：** 没有深度限制的递归 = Agent 可以无限嵌套 LLM 调用 = **策略空间爆炸**。D7 的深度限制是一个 **机制约束**（mechanism constraint），类似于博弈树搜索的深度限制。

---

## 5. 信息租金与战略性隐瞒

### 5.1 D2 的信息租金

在现行架构中，D2 持有三种 **信息租金（information rent）**：

| 信息 | 租金来源 | D7 的盲区 |
|------|---------|----------|
| LLM 调用事实 | 黑盒 QueryLoopExecutor | 不知道 D3 被调用了多少次 |
| Breaker 状态 | 只有 D2 看到 D3 的错误 | 无法做 provider fallback 决策 |
| Model tier 选择 | D2 内部 RouteModel | 无法统一调度 tier 预算 |

**信息租金 = 权力。** D2 开发者（无意中）对这些信息享有的独占访问权，使得 D2 在实际运行中拥有了超越其 Follower 名义的实质性权力。

### 5.2 目标态的信息对称化

| 信息 | v2.0 谁先知道 | 对称化的博弈效果 |
|------|-------------|----------------|
| LLM 调用 | **D7**（TurnOrchestrator 发起） | D7 可以 limit、cancel、measure |
| Breaker 状态 | **D7**（InvokeLLM 返回） | D7 可以 route around 或 graceful degrade |
| Model tier | **D7**（RouteModel 在 D7→D3 前） | D7 可以统一 tier 预算管理 |
| 工具策略 | D2（不变） | D2 保留其核心比较优势 |

**这是 Stackelberg 的精髓：** Leader 先动 = Leader 先知道。当 D7 先于 D2 知道 LLM 调用的结果时，它就真正拥有了"先动优势"（first-mover advantage）。

### 5.3 逆向选择问题

但信息对称化有一个代价：D7 现在承担了更多的 **信息处理负担**。如果 D7 不能有效利用这些信息（例如 Breaker 状态来了但 D7 不做路由调整），那么信息对称化反而增加了系统的复杂度而没有产生价值。

**缓解：** D7-S2-A07 InvokeLLM 必须真正实现 Breaker→路由的闭环，否则就是"为对称而对称"。

---

## 6. 过渡期的混合策略均衡

### 6.1 双轨并存的博弈

v1.0 Registry + v2.0 Structure 创建了一个过渡期，其中存在 **两个可能的均衡**：

| 路径 | 均衡 A：Legacy | 均衡 B：Canonical |
|------|---------------|-------------------|
| 开发者行为 | 继续在 D2-S16 改代码 | 在新接口上开发 |
| 新功能 LLM 调用 | 走 Legacy QueryLoopExecutor | 走 TurnOrchestrator |
| 规范遵从 | "S16 Legacy 仍可用" = 灰色地带 | D2→D3 禁止 = 明亮线 |

### 6.2 混合策略的风险

过渡期的核心风险是 **混合策略均衡（mixed strategy equilibrium）** 而非 **分离均衡（separating equilibrium）**：

- 开发者在 Legacy 和 Canonical 之间随机选择（取决于哪个更方便）
- 代码库同时存在两条 LLM 调用路径
- D5 span 无法区分 "Legacy 路径" 和 "Canonical 路径"
- 新开发者倾向于 copy-paste 已有代码（Legacy 更多 → 惯性）

### 6.3 如何让 Canonical 成为焦点均衡

| 手段 | 机制 | 时间 |
|------|------|------|
| import lint D2→D3 | 硬约束 | v2.0-d |
| Legacy adapter deprecation log | 软约束（每次调用打印 warning） | v2.0-f |
| PR 模板检查 | 社会约束（reviewer 拒绝 D2→D3） | v1.0 即可开始 |
| D5 `d7.turn.canonical` span | 可观测性约束（看板展示双轨比例） | v1.1 |

**Claude 建议:** 在 v1.0 已有的 `gaming-analysis.md` 中增加对混合策略风险的分析，并明确 v2.0 的 **去均衡化（disequilibrium）策略**——即让 Legacy 路径不再是可行策略的步骤。

### 6.4 Schelling 点：为什么 v1.0 零代码变更是对的

Schelling 点的核心洞见：**在无法直接沟通或强制执行的博弈中，行为人会趋向于"自然突出"的焦点。**

v1.0 registry-only 策略的 Schelling 属性：

1. **零 Go 变更** = 无回归风险 = 任何人可以无恐惧地接受
2. **规格先行** = 新开发者从文档中学到的就是 Canonical 架构
3. **T 映射完成** = 给已有测试赋予了新的 Canonical 意义
4. **边界文档** = 明确了"什么是违规"的共同知识

**这是最小阻力的共识锚点。** 一旦规格建立，后续的代码迁移就从"是否应该做"变为"什么时候做"——这是从 **价值判断** 到 **执行计划** 的关键转化。

---

## 7. 跨域谈判博弈：D2/D3/D7 的威胁点

### 7.1 谈判结构

D7 Turn 编排上移本质上是一次 **跨域谈判**。三个域各有其威胁点（threat point）——谈判破裂时的 fallback：

| 域 | 威胁点 | 谈判筹码 |
|----|--------|---------|
| D2 | 维持 S16 现状，继续隐式调 D3 | 38+ T 测试点、S15-S17 的复杂性使得外部人难以替代 |
| D3 | 继续服务 D2，D7 通过 D2 间接消费 | LLM 能力是硬需求，无论谁调用 |
| D7 | 维持"名义 Leader"现状 | DM-007 已确保唯一入口，D2 最终需要 D7 调度 |

### 7.2 谈判结果分析

**Nash 谈判解（Nash Bargaining Solution）** 最大化 (D2_gain - D2_threat) × (D7_gain - D7_threat)：

| 方案 | D2 收益 | D7 收益 | Nash 积 | 是否采纳 |
|------|---------|---------|---------|---------|
| 现状（D2 持有 LLM） | 高（不用改） | 低（盲 Leader） | 低 | ❌ |
| D7 接管（D2 完全不调 D3） | 低（大量重构） | 高 | 中 | ⚠️ 太极端 |
| **D7 接管 LLM + D2 保留工具面** | 中（拆接口） | 高 | **最高** | ✅ 当前设计 |

当前设计（R1 决议）正好落在 Nash 谈判解上：
- D2 失去 LLM 调用权（成本），但获得清晰的 Follower 定位（收益——不再承担编排负担）
- D7 获得 LLM 编排权（收益），但承担 Breaker 处理等复杂性（成本）
- D3 获得更清晰的消费者（D7），减少隐式依赖

### 7.3 为什么不是 "D7 全接管"

极端方案"D7 也接管工具执行"会破坏 D2 的比较优势：

- D2 拥有 Permission Gate（S18）——这是工具安全的核心
- D2 拥有 Session State（S17）——持久化需要上下文信息
- D2 拥有 Token 管理（S15）——预算控制需要历史信息

**分工原则：** D7 拥有 **跨域编排权**（何时调 LLM），D2 拥有 **域内执行权**（如何准备上下文、如何执行工具、如何持久化）。这是 **比较优势** 在架构层的体现。

---

## 8. 机制设计的博弈稳定性

### 8.1 机制约束 vs 行为建议

现有 `gaming-analysis.md` §3 列出了 5 种机制设计。从博弈稳定性角度，它们有不同的 **强制力梯度**：

| 机制 | 类型 | 强制力 | 可绕过性 |
|------|------|--------|---------|
| import lint | **硬规则** | 最高 | 零（CI 阻断） |
| Turn 状态机 | 运行时契约 | 高 | 零（编译器+类型系统） |
| Prepare 无 LLM | 设计约束 | 中 | 低（需显式注入 LLM 接口才能违反） |
| SubQuery 嵌套 Turn | 架构契约 | 中 | 中（D2-S19 可退化直连 D3） |
| Legacy 双轨 + Archive | 社会约束 | 低 | 高（依赖开发者自律） |

**Claude 观点:** 目前的设计过度依赖"社会约束"（Legacy 双轨、Archive 追溯）而缺乏足够的"硬规则"。建议：

1. **v2.0-d 的 import lint 应设为最高优先级**——它是整个机制设计中最强的 commitment device
2. **v1.0 registry 应同时登记 lint 规则**（即使 CI 上不执行），作为预期管理
3. **Prepare 接口应为 `ContextPreparer` 而非 `Engine`**——窄接口本身就是反 D2→D3 的机制约束

### 8.2 时间不一致性问题

**时间不一致性（time inconsistency）：** 即使 D2 开发者今天同意"不调 D3"，未来在面对 deadline 压力时，他们可能选择"先绕过 D7 直接调 D3 把功能上线，回头再修"。

这是所有架构治理的核心挑战。缓解：

| 手段 | 效果 |
|------|------|
| import lint（硬阻断） | 完全消除时间不一致性 |
| PR review 检查 | 部分消除（reviewer 可能松懈） |
| D5 span 监控 | 事后发现（已造成 debt） |
| 文化/文档 | 弱（依赖记忆） |

**建议：** v2.0 import lint 不应被视为"可选优化"，而是 **改变博弈均衡的必要条件**。没有硬阻断，时间不一致性会导致架构退化。

---

## 9. 信号博弈：谁在发信号，谁在说谎

### 9.1 信号发送者地图

| 信号 | 发送者 | 接收者 | 真实性风险 |
|------|--------|--------|-----------|
| CompressHint | D2 | D7 | D2 可能过度请求压缩以降低自身负载 |
| EngineEvent (text/thinking) | D3→D7→D1 | 用户 | D3 幻觉（非 D7 问题） |
| ToolRound 执行结果 | D2 | D7→D3 | D2 可能篡改 tool 结果 |
| Turn 完成 | D2-Persist | D7 | D2 可能过早宣称完成 |
| Breaker open | D3 | D7 | D3 可能误报（不常见） |

### 9.2 跨信号验证

**Claude 的核心担忧：** D7 在获得 LLM 编排权后，会收到大量信号，但这些信号的 **真实性** 依赖发信号者的诚实。D7 需要跨信号验证机制：

```text
验证链示例：
  D3 返回 tool_calls → D2 执行 → D2 返回 tool_result
    验证：tool_result 是否真的执行了 tool_calls 要求的操作？
    D7 层面：可对比 tool_calls.arguments 与 tool_result 的一致性
    D6 层面：事后审计（更强但更慢）

  D2 请求 CompressHint → D7 调 D3 摘要 → D2 MergeSummary
    验证：D2 是否真的"需要"压缩（还是为了简化自身逻辑）？
    D7 层面：token_count_before vs token_budget 的客观比较
```

**建议：** 在 D7-S2-A06 RunTurnLoop 中增加 **信号一致性校验点**（signal consistency checkpoint），至少在 span 层面标记异常：

- `d7.turn.compress_requested` AND `token_usage < 80% budget` → flag "可能的过度压缩请求"
- `d7.turn.tool_round_count > N` AND `no tool_calls in final` → flag "可能的工具循环浪费"

---

## 10. 对现有设计的具体建议

### 建议 1: 明确"LLM 调用权"的契约语言

当前 `proposal.md` 和 `design.md` 的措辞集中在接口定义和调用链，缺乏对 **产权** 的语言。

建议在 `d7-domain.md` North Star 中增加：

> **LLM 调用权归 D7。** D7 是唯一有权决定何时、以何种参数调用 D3 的域。D2 拥有"请求 LLM 结果"的权利（通过 CompressHint），但不拥有"执行 LLM 调用"的权利。

这看似语义差异，实际上改变了开发者对"D2 能否调 D3"的默认认知——从"默认可以，除非 lint 拦住"变为"默认不可，必须有 D7 授权"。

### 建议 2: 增加"信号真实性"校验层

在 `design.md` §3 Turn 状态机中，增加一个隐式步骤：

```text
PREPARE → [VALIDATE_SIGNALS] → ROUTE+LLM → ...
              ↑
        跨信号一致性校验（仅 span 标记，不阻断）
```

这不是额外的代码层，而是在 D5 span 中预先注册信号校验的锚点。

### 建议 3: import lint 的博弈文本

`import lint` 不应只在 `design.md` 中一句话带过，应有自己的 rationale：

```text
D2-THIN-T01（import lint）的博弈含义：

这是整个 D7 Turn 编排机制中强制力最高的 commitment device。
没有它，D2→D3 禁令在时间不一致性压力下必然退化。
它的存在不是为了防止"故意违规"，而是为了防止"不经意间的架构退化"——
开发者因 deadline 压力走捷径时，CI 阻断提供了"停下来重新思考"的硬边界。
```

### 建议 4: 过渡期混合策略风险登记

在现有 `gaming-analysis.md` 中增加 §6 "过渡期混合策略风险"，或在 `design.md` §8 v2.0 迁移 Slice 中增加博弈论注释。

### 建议 5: D7-S2-A06 RunTurnLoop 的 "领导力" 指标

建议为 D7 的 Turn Leader 角色定义可量化的"领导力"指标：

| Metric | 含义 | 计算 |
|--------|------|------|
| `d7.turn.orchestration_ratio` | D7 实际编排的 Turn 占比 | D7 发起的 LLM 调用 / (D7 + D2 legacy) |
| `d7.turn.legacy_drift` | Legacy 路径增长速率 | 新 Legacy T / 总 T |
| `d7.turn.commitment_gap` | 规格承诺 vs 代码现实的差距 | PLANNED T 数 / IMPLEMENTED T 数 |

这些 metric 将架构治理从"信念"变为"可观测事实"。

---

## 11. 总结：我的立场

### 11.1 与现有共识的一致点

1. **完全同意 Stackelberg Leader/Follower 框架**——D7 Turn 编排上移是必要的均衡修正
2. **完全同意 v1.0 零代码变更策略**——这是 Schelling 点的经典应用
3. **完全同意 D2→D3 禁令**——这是产权明晰化的核心
4. **完全同意 D7-S2-A06/A07 作为 Canonical Activity**——Turn 循环归 Leader

### 11.2 我额外强调的点

1. **产权视角比 Stackelberg 视角更根本**——Stackelberg 描述了博弈结构，产权解释了为什么结构会退化
2. **import lint 是机制设计中最重要的单点**——它是从软约束到硬约束的质变，不应被视为 v2.0 的可选项
3. **递归嵌套 Turn 是架构中最精巧的设计**——子博弈完美均衡的代码化，值得在 design.md 中展开论证
4. **Autocompact 灰区是不完全合约的经典案例**——权力分立设计是正确的，但不是无代价的（增加了协调面）
5. **过渡期的混合策略风险需要显式管理**——否则 v1.0 Registry 可能成为"永久的双轨均衡"
6. **信号博弈中的真实性校验需提前规划**——D7 成为信号集中点后，信号污染的风险也随之集中

### 11.3 一个担忧

我的主要担忧：**D7 在获得 LLM 编排权的同时，也承担了编排的责任和复杂性。** 如果 D7-S2-A07 InvokeLLM 实现草率——例如只在 happy path 调 D3, 不处理 Breaker、不 fallback、不重试——那么重构后的系统可能比现状更脆弱。

**现状的"丑陋但能用"** vs **重构后的"漂亮但脆弱"** 是一个真实的风险。缓解方式是确保 D7-S2-A07 的 P0 T 覆盖 Breaker 和错误路径。

---

## 12. 开放问题（请 Cursor 回应）

### Q1. 递归嵌套的深度

SubQuery Turn 递归的 MaxDepth 应设为多少？设为 2（只允许一层嵌套）是否足够？我的建议是 3（主 + SubQuery + SubSubQuery），与 D4-S19 已有的深度限制对齐。

### Q2. D7 的"领导力赤字"

D7-S2-A06 RunTurnLoop 是纯新增代码（v2.0 slice a-c），目前 D7 没有持有任何 LLM 调用逻辑。如何确保 D7 团队（或 AI 编码）第一次实现 LLM 编排时质量不低于 D2 已经打磨了 38+ T 的现有实现？

### Q3. Autocompact 的产权边界

CompressHint 的设计中，D2 提议、D7 执行——但如果 D7 **拒绝**压缩请求（例如 D3 目前负载太高），D2 应如何降级？返回错误？使用更激进的 truncation？这需要在接口契约中明确定义。

### Q4. 与 D6 信誉闭环的衔接

L3 惩罚档位定义"D7 保守路由"，但现在 D7 刚刚获得 LLM 编排权。保守路由的 **具体参数化**（少 fork 到多少？阈值从多少调到多少？）应在 D7 Turn 重构的哪个阶段定义？

### Q5. 过渡期的"反事实"度量

我们如何知道 v1.0 Registry 规格真的改变了开发者行为？还是仅仅在文档层换了标签？是否需要一个"反事实"度量——例如跟踪 PR 中 D2→D3 的新增 import 频率——来验证规格的效果？

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：产权、不完全合约、递归博弈、过渡期协调、信号博弈、机制设计 |
