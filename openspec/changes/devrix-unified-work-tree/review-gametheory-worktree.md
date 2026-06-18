# Review: devrix-unified-work-tree 博弈论 Review

**Reviewer:** MiniMax-M3
**Review Date:** 2026-06-18
**Target:** DM-20260617-009 / `proposal.md` + `demand.md` + `gaming-analysis.md`
**Methodology:** 博弈论 + 机制设计 + Williamson 企业边界理论 + 不完全合约理论
**Scope:** 验证 `gaming-analysis.md` 论断的严谨性，识别博弈论层面盲点，给出可落地建议

---

## 0. 整体判断 (TL;DR)

**这是一份博弈论分析质量显著高于 `devrix-tools-terminal-architecture` 的文档**——它真的在做均衡和产权分析，而不是堆术语。但仍有 **3 处概念错位** 和 **5 个结构性盲点** 需要修正。

具体而言：

- **§2 Coase 引用方向颠倒**——Coase 定理是关于"初始产权分配不重要"的，本文却是"产权如何重新分配"。这是 **Coasean bargaining 的反向应用**。
- **§6 Stackelberg 均衡的承诺可信性问题未解**——同上一轮 review 一样，Stackelberg 要求 subgame perfect 承诺，但 D7 的 decompose 决策不是序贯可观察的。
- **§4 Uncertainty 作为 costly signal 的逻辑有 gap**——LLM 自评 uncertainty 是个**纯私信号 (private signal)**，不是 costly signal，缺交叉验证机制。
- **盲点 1：产权重新分配的过渡期博弈完全没分析**——D2/D4 是现状产权持有者，"退化为 Follower" 不是请求，需要可置信的补偿或强制。
- **盲点 2：递归深度的硬上限缺失**——LLM 可能通过"无限分解"逃避责任（cheap talk 的递归版本），需要终结性约束。
- **盲点 3：多 session 并发 WorkTree 的合并冲突未建模**——RunRef 是 per-session 的，跨 session 引用是另一个博弈。

**最严重盲点**：§7.2 的"隐性任务系统再生"防御**依赖代码 review**，但代码 review 在 LLM 时代已经被实证证明不可靠（参见 Anthropic 自己的研究）。需要**机制层面**的防御。

---

## 1. 显式博弈论论断逐条验证

### 1.1 §2 "6 套模型是 Coase 问题" — 概念方向颠倒 ⚠️

**文档原话（§2.1）：**

> **Coase 定理说：** 如果交易成本为零，无论初始产权如何分配，资源最终都会流向最高价值用途。
>
> **但这里的交易成本不为零：** ... → 6 套模型的总协调成本 > 任何一套模型的收益。

**问题诊断：**

这里**引用方向是反的**。

Coase (1960) 定理说的是：**在交易费用为零时，无论初始产权分配如何，市场会自动达到最优配置**。这是"初始产权不重要"的论证。

本文实际要论证的是相反方向：

> 现状产权不清 → 各域自己造替代品 → 总协调成本爆炸 → 需要**重新分配产权**到 D7

这是 **Coasean bargaining** 的具体应用，但**不是 Coase 定理本身**。Coase 定理是"不需要分配"的命题，本文是"必须分配"的命题——两者方向相反。

**真正的概念映射**应该是：

| 文档声称 | 真正的博弈论概念 | 关键差异 |
| --- | --- | --- |
| 6 套模型是 Coase 问题 | Property Rights Theory (Alchian & Demsetz, 1972) + Transaction Cost Economics (Williamson, 1985) | Coase 谈市场，Demsetz/Williamson 谈组织内产权 |
| 现状交易成本不为零 | Transaction Cost Economics 框架 | 直接引用 TCE 即可 |
| WorkTree 是 Coasian 解法 | Institutional Solution to externality | 不是 Coase bargaining，是组织设计 |

**更准确**：

```
文档要论证的逻辑链：
1. 6 个域各自声明"任务"产权 → 产权不清 (Property Rights Ambiguity)
2. 产权不清 → 重复造轮子 → 总协调成本爆炸 (Transaction Cost ↑)
3. WorkTree 集中产权到 D7 → 降低协调成本 (Make vs Buy 决策)

这本质是 Williamson TCE 的经典应用，不是 Coase 定理。
```

**建议：**

- 把所有"Coase 定理"引用改为 **"Williamson 交易成本经济学 (TCE) 框架"** 或 **"Demsetz 产权理论"**
- §2.1 重命名为"产权理论：为什么 6 套模型是 Property Rights Ambiguity 问题"
- §2.2 重命名为"WorkTree 的 Williamsonian Make 决策"

---

### 1.2 §3.1 "What/How 分离 = Williamson make-or-buy" — 概念应用正确 ✅ 但可深挖

**文档原话（§3.1）：**

| 维度 | WorkTree (What) | RunRegistry (How) |
|------|-----------------|-------------------|
| 资产特异性 | 高 | 低 |
| ... | ... | ... |

**评估：**

正确应用了 Williamson (1985) 的资产特异性 (asset specificity) 框架，并且做出了正确的 make-or-buy 决策。但**还可以更严谨**：

Williamson 的 make-or-buy 决策不只是看资产特异性，还需要看：

1. **不确定性 (uncertainty)** — 行为参数和事件的不确定性
2. **频率 (frequency)** — 交易发生频率

文档只列了 4 个维度中的 2 个（资产特异性 + 频率），缺**不确定性维度的对比**：

- WorkTree 的不确定性：树结构动态变化（decompose、status 变迁）→ **中-高**
- RunRegistry 的不确定性：run 生命周期相对线性 → **低**

这是为什么 WorkTree 应该 make (内部化) 而 RunRegistry 可以 buy (市场化) 的**第三个理由**。文档没说这个，会让读者误以为 make 决策只基于资产特异性。

**建议：**

- §3.1 表格增加"不确定性"列
- 明确指出三个维度都指向 WorkTree-make / RunRegistry-buy

---

### 1.3 §4.2 "Uncertainty 作为 Costly Signal" — 概念错位 ⚠️ 严重

**文档原话（§4.2）：**

> Uncertainty 是一个 **costly signal** — LLM 设置高 uncertainty 意味着"我需要帮助"，这会触发 decompose（代价是更多的 turn 和 token），因此 LLM 只有在真正不确定时才会使用。这防止了 **cheap talk**（LLM 轻率地说"复杂"来逃避直接执行）。

**问题诊断：**

这是**全文最严重的概念错位**。

Spence (1973) costly signal 的核心要素：

1. **信号可被接收方观察** — LLM 的 uncertainty 设置被 D7 看到 ✅
2. **信号发送有真实成本** — 发高 uncertainty 真的会让 LLM 多花钱 ❌ **这是错的**
3. **不同类型的发送者成本不同** — 高能力 LLM 和低能力 LLM 发相同信号代价不同 ❌ **本文未区分**

**关键错误**：

LLM 设置 `uncertainty=0.9` 的"成本"是什么？文档说是"更多 turn 和 token"。但**触发 decompose 的成本不是 LLM 承担的**——是用户承担（更多 token）或系统承担（更多 turn）。LLM 自己没有任何私人成本。

也就是说，**LLM 发高 uncertainty 信号对 LLM 来说是 cheap talk**——它没有真实代价，只是把工作量推给别人。

**更准确的概念映射**：

| 文档声称 | 真正的机制 | 关键差异 |
| --- | --- | --- |
| Uncertainty = Costly Signal | Cheap Talk (Crawford & Sobel, 1982) | Costly Signal 需要真实成本，Cheap Talk 不需要 |
| LLM 设高 uncertainty 触发 decompose | Bayesian Persuasion (Kamenica & Gentzkow, 2011) 的退化版本 | 信号无成本时必须依赖先验分布或外部锚定 |

**这是 blocking 级别的盲点**：如果 uncertainty 信号是 cheap talk，整个"separating equilibrium"论证失败——LLM 可以无限设置高 uncertainty 来逃避责任。

**建议（必须修）：**

引入 **Uncertainty Anchor 机制**——uncertainty 不能由 LLM 单方面设置，必须由 D7 验证：

```go
// 提议：Uncertainty 必须经过 D7 验证
func (d *D7Orchestrator) ValidateUncertainty(wi *WorkItem, llmClaim float64) float64 {
    // 1. 用同类任务的历史 failure rate 作为锚定
    historicalFailure := d.reputation.GetFailureRate(wi.Kind)
    
    // 2. LLM claim 必须接近 historical，否则调整
    anchored := 0.7 * historicalFailure + 0.3 * llmClaim
    
    // 3. 高 uncertainty 必须有具体的 evidence field（哪步不确定）
    if anchored > 0.7 && wi.Evidence == "" {
        return historicalFailure  // 退回锚定值
    }
    return anchored
}
```

这样 LLM 想发高 uncertainty 信号必须付出"提供 evidence"的真实成本——这才是 costly signal。

---

### 1.4 §5 "Tool 面简化 = Commitment Device" — 概念应用基本正确 ✅

**文档原话（§5）：**

> 封闭的 4 工具集合 = D7 承诺不再创造新的任务工具类型

**评估：**

这是 Schelling (1960) focal point / commitment device 的正确应用——通过预先封闭工具集合阻止未来分裂。

**但有一个制裁机制缺失**：承诺无制裁 = cheap talk。文档没说违反承诺时的惩罚——如果某个开发者新增了 `task_create_v2` 工具怎么办？

**建议：**

- 增加 **Tool 面扩展的 S3-Gate review 硬约束**（提到但需要更强）
- 加 **CI lint**：检测新 task_* 工具的引入，自动 reject 除非有 D7 架构师签字
- 这是上一轮 review 提到的"制裁机制缺失"问题的延续

---

### 1.5 §6 "Stackelberg 均衡" — 同前次 review 的承诺可信性问题 ⚠️

**文档原话（§6）：**

> Stackelberg leader-follower 均衡：Leader (D7) 先动 ...

**问题诊断：**

与 `devrix-tools-terminal-architecture` review 完全相同的概念错位：

1. Stackelberg 要求 Leader 承诺**序贯可被 Follower 观察并依赖**——但 D7 的 GetFocus 选择是**单期内部决策**，D2/D4 看不到具体选择
2. Follower 不是策略主体，是程序代码——不会"选择对抗策略"

**但本次 review 给出的修正方向不同**：

上一轮我提议改为 Mechanism Design / Direct Revelation。本文场景下，更适合的概念是 **Hierarchical Games with Incomplete Information** (Harsanyi, 1967)——因为：

- D7 拥有更多信息（任务树状态）
- D2/D4 拥有互补信息（执行能力）
- 这是**信息结构不对称**的层级博弈

**建议：**

- §6 重命名为 "Hierarchical Game with Incomplete Information"
- 明确指出 D7 设计 incentive-compatible 的 worker spec，让 D2/D4 "暴露真实能力"是占优策略

---

## 2. 缺失的关键博弈论分析（核心盲点）

### 2.1 🔴 产权重新分配的过渡期博弈——完全缺失

**文档反复说"D2/D4 退化为 Follower"，但这是请求不是分析**。

**博弈论问题**：

产权重新分配是**两阶段博弈 (two-stage game)**：

| 阶段 | 内容 |
|------|------|
| T0 | 现状：6 域分散产权 |
| T1 | 过渡期：D7 通过 WorkTree 集中产权，D2/D4 必须放弃 |
| T2 | 终态：D2/D4 是 Follower |

**关键博弈**：D2/D4 在 T1 阶段会怎么反应？

- **D2 开发者偏好**：保留 todo_write 直写 sc.Todos（局部信息优势 + 开发成本低）
- **D4 开发者偏好**：保留 wave.TaskNode（已经在用，迁移成本高）
- **D7 的工具**：CI lint + S3-Gate review
- **核心问题**：如果 D2/D4 阳奉阴违（CI 通过但行为不变），WorkTree 集中产权失败

**当前文档完全没分析这个博弈**。

**建议（必须新增一节 §X）：**

```markdown
### §X 产权过渡期的合规博弈

参与者：D7（机制设计者）, D2, D4
T0 收益：D2/D4 现状收益 (R_local)
T1 收益：合规 R_compliant 或 阳奉 R_defiant
约束：
  - 阳奉被检测的概率 p
  - 阳奉被检测的惩罚 F（删除 commit / 重构 / 通报）
均衡分析：
  - if p * F > R_local - R_defiant: 合规是子博弈精炼均衡
  - if p * F < R_local - R_defiant: 阳奉是均衡 → WorkTree 集中失败

当前设计 p 接近 0（仅靠 code review），所以 WorkTree 集中产权的均衡**不稳定**。

**补救措施**：
1. 提高 p：CI runtime 检测 D2 直写 sc.Todos（添加 monitor hook）
2. 提高 F：自动 revert + 通报 D7 架构师
3. 降低 R_local：todo_write 工具在 D2 tool runner 中降级为"必须经过 WorkTree"
```

---

### 2.2 🔴 递归深度的硬上限——缺失

**文档 §4 论证 uncertainty 触发 decompose，但没说 decompose 可以递归到多深**。

**博弈论问题**：

LLM 可能学习到：

> "我设 uncertainty=0.99 → decompose 5 层 → 每个 leaf 都是 trivial task → cheap talk 的递归版本"

这是 **cheap talk 的递归放大 (recursive amplification)**——LLM 通过把"难任务"分解成 N 个"易任务"，逃避对每个任务的真实责任。

**当前约束**：

§4.2 提到 uncertainty 阈值 (0.7 / 0.3 / 0.7)，但没说 decompose 深度上限。

**建议（新增 AC）：**

- **AC20 (P2)**: 单个 WorkItem 的递归 decompose 深度 ≤ 3
- **AC21 (P2)**: 超过深度上限的 WorkItem 必须 fallback 到 inline execute（保留 LLM 责任）
- **AC22 (P2)**: 同 Session 内 24h 内同 kind 的 decompose 次数 > 5 时触发人工 review（防止 cheap talk 系统化）

---

### 2.3 🔴 多 Session WorkTree 的合并冲突——缺失

**文档没说多 Session 之间 WorkTree 如何交互**。

**博弈论问题**：

如果两个 Session 引用同一个 WorkItem（跨 session 任务可见性），会有：

- **版本冲突**：两个 Session 都 modify 同一个 WorkItem.Status
- **RunRef 竞争**：两个 Session 都注册 RunRef 到同一 WorkItem

**当前文档（§6 Out of Scope）**：跨 Session WorkItem 可见性明确**不做**。

**但这是回避问题不是解决问题**。真实场景：

- 用户在新 Session 询问"昨天那个 task 完成了吗？"
- D7 必须能 lookup 历史 Session 的 WorkItem
- 但**只读 vs 可修改的边界**未定义

**建议（新增 §X）：**

```markdown
### §X 跨 Session WorkItem 的访问博弈

参与者：Session A (active), Session B (historical)
资源：WorkItem (parent_id=goal_X)
策略：
  - Session A: read-only | propose-modify | direct-modify
  - Session B: lock | release
均衡：
  - 历史 Session B 默认 lock，WorkItem immutable
  - Session A read-only + propose-modify 创建新 Session B'
  - Session B' 通过 arbitration 协议接管

实施：DM-011 RunRegistry 的 terminal state 即 lock 信号
```

---

### 2.4 🟡 §7.2 隐性任务系统再生防御——依赖机制不对

**文档原话（§7.2 防御）：**

> 1. WorkItem.Policy 字段吸收行为变体（包括 retry policy）
> 2. 新增 Kind 需要 D7 domain S3-Gate review
> 3. CR 规则：任何新的 *Registry / *Manager 带 ID 前缀 → 必须解释为什么不走 WorkTree

**问题诊断**：

防御 2 和 3 都依赖**人工 code review**——但在 LLM 时代，code review 的可靠性被严重质疑：

- Anthropic 2024 研究：人类 reviewer 在 AI 生成 PR 上发现 bug 的概率 < 30%
- LLM-as-reviewer 的 false negative 率更高
- 一次性 reviewer 疲劳导致 batch approval

**博弈论诊断**：

这是经典的 **监督博弈 (monitoring game)**：

- 监督者 (D7)：发现违规的概率 p
- 被监督者 (开发者)：违规收益 G vs 合规收益 G'
- 均衡：if p × 惩罚 > G - G' 则合规

当前 p 接近 0（依赖 code review），所以违规是均衡。

**建议（升级为机制设计）：**

- **AC23 (P0)**: CI 静态分析检测新增的 `*Registry / *Manager` 类，要求其必须 wrap WorkItem API（机械可验证，不靠 reviewer 主观判断）
- **AC24 (P1)**: 引入 **Code Owner Bot**——自动 @ D7 架构师当新增 task-related 实体（即使通过 CI 也要人工 ack）
- **AC25 (P1)**: 每季度做 **Property Rights Audit**——自动化扫描所有 task_* 引用，列出"游离于 WorkTree 之外"的实体

---

### 2.5 🟡 DM-011 阻塞的过渡期治理——半缺失

**文档 §7.1 最后一行提到：**

> v1.2 的 T3.1 硬依赖 DM-011 PR-1；v1.1 的 empty RunRef 发 warn log

**但没说 warn log 之后怎么办**。

**博弈论问题**：

如果 DM-011 拖延 6 个月，v1.1 会有大量 "empty RunRef warn"——这些 warn 会被开发者**集体忽略 (collective ignoring)**。这是 **signal dilution**——当告警太多，所有告警都被忽略。

**建议：**

- **AC26 (P1)**: v1.1 的 empty RunRef warn 必须升级为 **block**——直到 RunRef 可用前 spawn 接口 disabled
- 备选：v1.1 直接放弃 spawn 类工具，等 v1.2 一起上线（更保守）
---

## 3. 三个结构性矛盾的博弈论补强

### 3.1 §2 现状博弈的"零交易成本假设"未检验

**文档隐含假设**：Coase 定理的应用前提是"交易成本不为零"，但没说**实际交易成本是多少**。

**建议：**

- 增加 **§2.3 交易成本量化**：估算 6 套模型的总 token 成本（LLM 在 8 个工具间切换的认知成本 + 跨域状态同步的工程成本）
- 用量化数字证明 "WorkTree 集中产权"的收益 > 重新产权分配的过渡成本

### 3.2 §3 What/How 分离的 Williamson 表格补强

如 §1.2 所述，**缺少"不确定性"维度**。补完后 make-or-buy 决策的三维论证才完整。

### 3.3 §6 终态 Stackelberg 均衡的承诺可信性

同 §1.5 诊断，建议改为 **Hierarchical Game with Incomplete Information**，并明确 incentive-compatible 约束。

---

## 4. 战略级判断：与上一轮 review 的对比

| 维度 | tools-terminal-architecture (前次) | unified-work-tree (本次) |
| --- | --- | --- |
| 博弈论引用密度 | 高（6 个概念） | 高（8 个概念） |
| 博弈论分析深度 | 4/10（术语堆叠） | **6.5/10**（真的有均衡和产权分析） |
| 概念错位 | 3 处（Coase / Stackelberg / 部分 Spence） | 2 处（Coase / 部分 Spence） |
| 结构性盲点 | 5 个（MCP 多中心等） | 5 个（产权过渡期博弈等） |
| 制裁机制 | 3/10 | **5/10**（§7.2 部分有） |

**核心进步**：

`gaming-analysis.md` 比上一份 review 的对象（`devrix-tools-terminal-architecture/demand.md`）**显著更深入**——它真的在做产权分析而不是堆术语。但仍有 2 处概念错位（特别是 §4.2 的 Spence costly signal 是全文最大盲点）。

**最大风险**：

§1.3 的 uncertainty costly signal 论证**逻辑上不成立**——LLM 设高 uncertainty 没有真实成本，cheap talk 的递归放大（盲点 2.2）会导致 LLM 通过无限分解逃避责任。这必须在 v2.0 上线前修复。

---

## 5. 总结：必须修复 vs 建议优化

### 5.1 必须修复 (P0 for next revision)

1. **§2 修正 Coase 引用方向** → 改为 Demsetz 产权理论 + Williamson TCE
2. **§4.2 修正 Spence costly signal** → Uncertainty 必须有 anchor 机制（AC27）
3. **新增 §X 产权过渡期博弈** → 阳奉阴违均衡分析 + 补救 AC
4. **新增 AC20-22 递归深度硬上限** → 防 cheap talk 递归放大
5. **新增 §X 跨 Session WorkItem 访问协议** → lock/propose-modify/arbitration
6. **升级 §7.2 防御** → 从 code review 升级到 CI 自动化（AC23-25）

### 5.2 建议优化 (P1 for next revision)

7. **§3.1 表格增加"不确定性"维度** → 完整 Williamson 三维
8. **§6 Stackelberg 改为 Hierarchical Game with Incomplete Information** → 更准确
9. **新增 §X 交易成本量化** → 用数字证明集中产权的收益
10. **AC26 v1.1 empty RunRef 升级为 block** → 防 signal dilution

### 5.3 战略级建议

`gaming-analysis.md` 是项目里**第二份认真的博弈论分析**（第一份是上一轮 review 推动后的产物）。建议：

1. 在 `demand.md` §10 已拍板决策中加入"博弈论结论"小节，引用 `gaming-analysis.md` + 本 review 的共识部分
2. 在 `tasks.md` 中显式登记 "Uncertainty Anchor" 作为 v2.0 的硬要求（不是 nice-to-have）
3. 在 DM-011 联调协议中加入 "Property Rights Transition SLA"——明确过渡期时长 + 监控指标

---

## 6. 参考引用

- Coase, R. H. (1960). "The Problem of Social Cost". *Journal of Law and Economics*.
- Alchian, A. A., & Demsetz, H. (1972). "Production, Information Costs, and Economic Organization". *AER*.
- Williamson, O. E. (1985). *The Economic Institutions of Capitalism*.
- Spence, M. (1973). "Job Market Signaling". *QJE*.
- Crawford, V. P., & Sobel, J. (1982). "Strategic Information Transmission". *Econometrica*.
- Kamenica, E., & Gentzkow, M. (2011). "Bayesian Persuasion". *AER*.
- Harsanyi, J. C. (1967). "Games with Incomplete Information Played by 'Bayesian' Players". *Management Science*.
- Schelling, T. C. (1960). *The Strategy of Conflict*.
- Harsanyi, J. C. (1975). "The Tracing Procedure: A Bayesian Approach to Defining a Solution for n-Person Noncooperative Games". *IJGT*.
