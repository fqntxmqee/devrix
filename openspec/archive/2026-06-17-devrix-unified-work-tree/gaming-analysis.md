# 终态任务架构 — 博弈论分析

**Change ID:** devrix-unified-work-tree
**Demand ID:** DM-20260617-009
**日期:** 2026-06-18
**版本:** v2 — 经 Codex 审查后修正（见 `review-gametheory-worktree.md` + `gaming-analysis-bilateral-consensus.md`）
**状态:** Active — 双边共识达成，待 S3-Gate Review
**关联:** `proposal.md`, `design.md`, `tasks.md`；DM-011 (RunRegistry)；DM-20260617-001 (QueryLoop 退役)
**参考分析:** `../archive/2026-06-15-devrix-d4-sa-refine/gaming-analysis.md` (D4↔D7 Hub-Spoke 产权转移)

> **v2 修正记录 (2026-06-18):** §2 Coase→Demsetz/Williamson；§2.5 新增产权过渡期博弈；§3.1 补充不确定性维度；§4.2 Costly Signal→Cheap Talk+Uncertainty Anchor；§4.4 新增递归深度硬约束；§6 Stackelberg→Harsanyi Hierarchical Game；§7.2 防御升级为 CI 自动化为主。详见 `gaming-analysis-bilateral-consensus.md`。

---

## 0. 因果链前言

> **多套任务系统不是过度工程化的偶然事故，而是产权不清导致的不可避免的多重边际化。**

```
DM-020: D7 Turn 编排上移（D7 获得 LLM 调用权）
    │
    ├─ D7 成为"真正的 Leader"（决定 what to do + when + who）
    │
    ├─ 但 D2 仍拥有 task 工具面 + todo_write + BackgroundRegistry
    │     D4 仍拥有 wave.TaskNode + delegate 生命周期
    │     → 产权分散：同一个"任务"概念被三方同时声称所有权
    │
    └─ DM-20260617-009（本 change）
          ├─ WorkItem 树 → D7 独占 What 产权
          ├─ RunRegistry → D7 独占 How 观测权（DM-011）
          ├─ D2/D4 退化为无状态执行者（Follower）
          └─ 与 DM-001（D2 交出 LLM 调用权）、DM-018（D4 交出 Hub-Spoke 编排权）
              对称——三个 SA Refine 的汇聚点是 D7 的任务树
```

**一句话：** 终态任务架构是 Stackelberg 均衡——D7 拥有任务树产权（What），RunRegistry 提供执行观测（How），D2/D4 作为 Follower 仅保留各自的执行比较优势。

---

## 文档结构

| 章节 | 内容 |
|------|------|
| §1 | 多方博弈位置 — 谁是 Player，各自的 payoff |
| §2 | 产权理论 — 为什么 6 套模型是 Coase 问题 |
| §3 | What/How 分离 — 企业边界理论 |
| §4 | 递归求解 — 委托-代理模型 |
| §5 | Tool 面简化 — Commitment Device |
| §6 | 与两次失败尝试的对比 — 为什么这次不同 |
| §7 | 终态 Stackelberg 均衡 — 每个 Player 的策略 |
| §8 | 不完全合约风险 — 需要监督的边界 |

---

## 1. 多方博弈位置

```
用户 (Principal)
    │  payoff = 任务完成 + 进度可见
    ↓
D1 Gateway (Trusted Intermediary — IM 展示 + 权限 UI)
    │  payoff = 消息投递可靠性
    ↓
D7 Orchestrator (Leader — 拥有 What 产权 + LLM 调用权)
    │  payoff = 任务吞吐 × 完成质量 × 用户满意度
    │  策略 = 选择 focus → decompose → spawn → await → re-resolve
    │
    ├── WorkTree (What) ─── D7-S1 Work Model
    │     payoff = 单一语义权威，零翻译层
    │
    ├── RunRegistry (How) ─── DM-011
    │     payoff = 执行可观测性，增量输出流
    │
    ├── RunTurn 递归引擎 ─── D7-S2
    │     payoff = uncertainty 驱动的最优分解深度
    │
    ├──→ D4 Multi-Agent (Execution Follower)
    │      payoff = Worker 实例正确性，隔离安全性
    │      策略 = 接收 WorkerSpec → fork→run→join → 回报 terminal
    │
    ├──→ D2 Context Engine (Tool Runner Follower)
    │      payoff = 工具执行可靠性，上下文组装正确性
    │      策略 = 接收 tool call → 执行 → 返回 result
    │
    └──→ D3 LLM Gateway (公共能力)
           payoff = 模型调用成功率 + 延迟
```

### 1.1 现状 vs 终态：每个 Player 的信念

| Player | 现状信念（错误） | 终态信念（正确） |
|--------|-----------------|-----------------|
| D2 | "我拥有 task 工具面，所以我也拥有任务语义" | "我只提供工具执行，任务结构由 D7 WorkTree 定义" |
| D4 | "我拥有 TaskNode，所以我也拥有并行调度语义" | "我只执行 WorkerSpec，并行策略由 D7 通过 WorkItem.Policy 决定" |
| D7 | "我是 Leader，但 task 数据散落在 D2/D4" | "我独占 WorkTree 产权，D2/D4 是 Follower" |
| LLM | "我面对 8 个 task 相关工具，语义重叠" | "我面对 4 个统一工具，清晰的任务树模型" |

---

## 2. 产权理论：为什么 6 套模型是 Property Rights Ambiguity 问题

> **术语说明 (2026-06-18 修正):** 本文最初引用 Coase 定理描述产权不清问题。Codex 审查指出这方向是反的——Coase 定理的核心命题是"零交易成本时初始产权分配不重要"，本文论证的是"交易成本不为零所以产权必须重新分配"。这更精确地对应 **Demsetz (1967) 产权理论**和 **Williamson (1985) 交易成本经济学 (TCE)**。Coasean 框架作为 umbrella term 仍可保留，但分析工具应准确归位。

### 2.1 现状产权图

```
"任务"概念的产权分散：

workmodel.Task (D7-S1)
    │ 声称：用户可见的持久化工作单元
    │ 产权基础：task_create / task_list / task_update / task_delete
    │
coordinator.Plan + TaskSpec (D7-S1)
    │ 声称：计划的序列化描述
    │ 产权基础：CreateWorkPlan / PlanMode approve
    │
wave.TaskNode (D7-S3)
    │ 声称：Wave 调度器的执行单元
    │ 产权基础：SynthesizeTaskGraph / dispatchLoop
    │
contracts.TaskSnapshot (shared)
    │ 声称：D1 的只读投影
    │ 产权基础：QueryWorkPlan
    │
enforce.BackgroundTask (D2)
    │ 声称：异步 sub-agent 运行句柄
    │ 产权基础：RunBackground / bg_* ID 前缀
    │
types.TaskFlow + Milestone (shared)
    │ 声称：里程碑级别的任务流
    │ 产权基础：TaskFlowService（可能无人消费）
    │
todo_write → sc.Todos (D2)
      声称：LLM 可见的 checklist
      产权基础：session scratch 直写
```

**Demsetz (1967) 产权理论的核心洞察：** 当产权不清时，外部性无法内部化——每个域承担自己造轮子的全部成本，但只获得部分收益（因为"任务"概念被 6 方共享）。产权明晰后，D7 承担 WorkTree 的全部维护成本，也获得全部协调收益——外部性消失。

**Williamson (1985) TCE 的框架：** 交易成本不为零时，治理结构（谁拥有什么）决定了效率。这里的交易成本包括：LLM 在 6 种工具间切换的认知成本、跨域状态同步的工程成本、ID 前缀不一致的集成成本。

### 2.2 WorkTree 的 Williamsonian Make 决策

```
WorkItem 树（单一产权归属 D7）：

WorkItem { ID, ParentID, Kind, Status, Uncertainty, Policy, RunRef, Ephemeral }

吸收所有现状模型：
  workmodel.Task      → WorkItem{Kind=implement, Ephemeral=false}
  coordinator.Plan    → WorkItem{Kind=goal} + Children{Kind=implement}
  wave.TaskNode       → WorkItem{Kind=implement, Policy=parallel_ok}（runtime 投影，v2.0 删除）
  contracts.TaskSnapshot → WorkTree.ListSubtree() 投影
  enforce.BackgroundTask → WorkItem{Kind=shell|agent, Policy=async} + RunRef
  types.TaskFlow      → 删除（无人消费）
  todo_write          → WorkItem{Kind=checklist, Ephemeral=true}
```

**关键动作：** 不是"统一注册表"（infrastructure 方案），而是"统一产权归属"（institutional 方案）。

---

## 2.5 产权过渡期的合规博弈

> **2026-06-18 新增（Codex 审查盲点）：** 原始分析假定了"D2/D4 会自动退化为 Follower"，没有分析过渡期的对抗策略。

产权重新分配是**两阶段博弈 (two-stage game)**：

| 阶段 | 内容 | D2/D4 的策略选择 |
|------|------|-----------------|
| T0 现状 | 6 域分散产权，各自造轮子 | 各自拥有局部 task 语义（现状收益 R_local） |
| T1 过渡 | D7 通过 WorkTree 集中产权 | 合规（走 WorkTree API）或 阳奉（绕过 WorkTree 直写本地状态） |
| T2 终态 | D2/D4 是纯 Follower | 无选择——WorkTree 是唯一路径 |

**T1 的关键博弈：**

```
参与者：D2 开发者, D4 开发者, D7 架构师

D2 的策略空间：
  合规：todo_write → WorkTree.UpsertChecklist
  阳奉：todo_write 直写 sc.Todos（绕过 WorkTree，保留局部信息优势）
  
D4 的策略空间：
  合规：SynthesizeTaskGraph → WorkTree.Create(subtree)
  阳奉：保留 wave.TaskNode 持久化（不经过 WorkTree）

检测概率：
  p(CI only) ≈ 0.7    — static analysis 检测 sc.Todos 直写 / TaskNode 持久化
  p(CR only) ≈ 0.2    — 人工 code review 在 AI 生成 PR 上的 bug 发现率（Anthropic 2024）
  p(CI + CR) ≈ 0.85   — 互补

阳奉收益 R_defiant - R_compliant：
  D2：保留局部信息优势 + 开发便利 → 短期正收益
  但 sc.Todos 被标 ReadProjection 后直写风险上升 → 收益递减

均衡条件：
  if p × F > R_defiant - R_compliant: 合规是子博弈精炼均衡
  if p × F < R_defiant - R_compliant: 阳奉是均衡 → WorkTree 集中产权失败
```

**当前设计的风险：** 纯依赖 code review 时 p≈0.2，需要 F 极高才能维持均衡——不现实。因此必须：

1. **提高 p：** CI static analysis 作为第一道防线（AC23）
2. **提高 F：** 自动 revert + 通报 D7 架构师（AC24）
3. **降低 R_defiant：** `sc.Todos` 标 `// ReadProjection only, DO NOT WRITE`；`wave.TaskNode` 标 `// Deprecated, use WorkItem projection`

---

## 3. What/How 分离：企业边界理论

### 3.1 为什么 WorkTree 和 RunRegistry 必须分离

这是 Williamson 的 **"make or buy"** 决策：

| 维度 | WorkTree (What) | RunRegistry (How) | 决策含义 |
|------|-----------------|-------------------|---------|
| 资产特异性 | 高 — 任务结构是 D7 编排的核心资产 | 低 — 执行句柄是通用的运行观测 | 高特异性 → Make（内部化） |
| 交易频率 | 高 — 每个 turn 都读写 | 中 — spawn 时写入，terminal 时更新 | 高频率 → Make |
| 不确定性 | 中-高 — 树结构动态变化（decompose、status 变迁） | 低 — run 生命周期线性 | 高不确定性 → Make（需要灵活调整） |
| 治理结构 | 垂直整合（D7 独占） | 市场化（RunRegistry 接口，任意 executor 实现） | 三个维度都指向 WorkTree-Make / RunRegistry-Buy |

**如果合并（把 RunRegistry 塞进 WorkTree）：**
- WorkItem 膨胀为"任务 + goroutine + cancel func + output buffer"
- 违反单一职责 → 修改执行机制的代价波及任务语义
- 等同于企业把供应商内部化后管理成本暴涨

**如果过度分离（WorkTree 不知道 RunRef）：**
- terminal 通知无法反向更新 WorkItem status
- 等同于企业完全外包但失去了供应商的绩效数据

**最优边界：** WorkItem 持有 `RunRef`（外键），RunRegistry 通过 callback 更新 WorkItem status。这是 **关系契约** — 不合并组织，但共享关键信息。

### 3.2 与两次失败尝试的对比

| 尝试 | 方案 | 为什么失败 | 博弈论诊断 |
|------|------|-----------|-----------|
| `unified-task-registry` (DM-011, 已取消) | 统一 TaskRegistry 接口包装所有后台任务 | 依赖 Wave Scheduler v1.2（未完成）| **试图用 infrastructure 解决产权问题** — 统一注册表不改变"谁拥有任务语义"，只包装了访问方式 |
| `wave-worktree-isolation` (已取消) | 为 Wave worker 加 git worktree 隔离 | 写并发未达痛点；依赖未完成 | **解决了不存在的用户问题** — 技术方案没有对应的需求信号 |
| **`unified-work-tree`（本 change）** | D7 独占 WorkItem 树产权 | —（进行中）| **先分配产权，再设计交互** — 产权明晰后，各层自愿退到 Follower |

**核心教训：** 前两次失败是 **"先建基础设施，再指望产权自动理顺"**。本 change 成功是因为 **"先确立产权，基础设施会自然长出来"**。

---

## 4. 递归求解：委托-代理模型

### 4.1 为什么需要递归分解

当前的 `task_create` flat 模型存在一个根本性的委托-代理问题：

```
现状（flat Task）：
  LLM 看到用户请求 → 创建 N 个 task → 逐个执行
  问题：LLM 创建 task 时，对每个 task 的真实难度有 private information
        → 可能创建过于粗糙的 task（隐藏复杂性）
        → 或创建过于细碎的 task（推卸判断责任）

终态（递归 WorkItem）：
  LLM 看到 focus WorkItem → 评估 uncertainty
    if uncertainty > threshold:
      decompose → 创建子 WorkItem → spawn → await children
    else:
      直接执行
```

**这是 separating equilibrium：**
- 高 uncertainty 的 task → 被分解 → 子 task 的完成/失败揭示真实的难度信息
- 低 uncertainty 的 task → 直接执行 → 节省分解和协调成本
- LLM 无法通过"隐藏信息"来推卸责任，因为 uncertainty 阈值强制显式化

### 4.2 Uncertainty 作为 Cheap Talk — 以及为什么需要 Anchor

> **2026-06-18 修正（Codex 审查）：** 原始分析将 Uncertainty 描述为 Spence costly signal——这是错误的。LLM 设置 `uncertainty=0.9` 的"成本"（更多 turn + token）由用户/系统承担，不是 LLM。发送方无私人成本 = Crawford & Sobel (1982) cheap talk，不是 costly signal。Separating equilibrium 论证需重建。

**Cheap talk 问题：**

LLM 自评的 uncertainty 是纯私信号 (private signal)——LLM 知道真实难度，但 D7 无法验证。在 cheap talk 模型下，LLM 的最优策略是：总是声称高 uncertainty → 触发 decompose → 把工作推给子任务 → 自己逃避责任。

**解决方案：Uncertainty Anchor 机制（AC27）**

LLM 的 uncertainty claim 必须被外部数据锚定，使其不再是 cheap talk：

```go
type UncertaintyEvidence struct {
    Source     string // tool_output | dependency_unknown | code_smell
    ToolCallID string // 指向发现不确定性的具体 tool call
    Snippet    string // 引用输出片段（不能凭空捏造）
}

func ComputeUncertainty(wi *WorkItem, llmClaim float64, ev *UncertaintyEvidence) float64 {
    histFail := reputation.FailureRate(wi.Kind)        // 同类任务历史失败率
    structComp := structuralComplexity(wi)              // 依赖深度 + 文件扩散度
    
    // LLM claim 权重随历史样本量动态调整
    sampleSize := reputation.SampleSize(wi.Kind)
    llmWeight := lerp(0.50, 0.15, min(sampleSize/100.0, 1.0))
    // 冷启动: LLM 权重 0.5（中等信任）
    // 充分锚定后: LLM 权重 0.15（高度不信任 cheap talk）
    
    // evidence 为空 → LLM claim 权重强制归零
    if ev == nil || ev.ToolCallID == "" {
        llmWeight = 0
    }
    
    anchorWeight := 1.0 - llmWeight
    return llmWeight*llmClaim + anchorWeight*(0.6*histFail + 0.4*structComp)
}
```

**为什么这解决了 cheap talk：** LLM 想发高 uncertainty 信号必须提供 evidence（指向具体 tool call 输出的引用）——这是 LLM 的真实成本（必须实际执行探索才能获得 evidence）。空 evidence → LLM claim 权重归零 → uncertainty 回退到 historical + structural 锚定值。

**阈值策略（不变）：**
```
uncertainty > 0.7 + decomposable(kind) → decompose
uncertainty < 0.3 → inline execute
0.3 ≤ uncertainty ≤ 0.7 → spawn with fallback
```

### 4.3 GetFocus 优先级作为机制设计

```
GetFocus(sessionID) 优先级：
  1. status=ready（pending + deps satisfied）
  2. kind 顺序：verify > implement > explore > checklist > plan
  3. 同 kind：Uncertainty 降序
```

**为什么 verify 优先级最高？**
- Verify 失败会触发父 task 的重新分解 → 信息价值最高
- 这是 **optimal stopping** 问题：先做信息量最大的 task，后续决策质量更高

**为什么 Uncertainty 降序？**
- 高 uncertainty task 先执行 → 早失败早重试 → 减少级联延迟
- 这是 **shortest-expected-processing-time** 调度策略的变体

### 4.4 递归深度的硬约束：防止 Cheap Talk 递归放大

**Codex 审查发现的盲点：** 即使有 Uncertainty Anchor，LLM 仍可能通过层层 decompose 逃避责任——把"难任务"分解成 N 个"易任务"，每个 leaf 的 cheap talk 成本为零。

这是 **cheap talk 的递归放大 (recursive amplification)**：单层 cheap talk 的危害有限，但递归 cheap talk 会导致任务树爆炸。

**硬约束（AC20-22）：**

| 约束 | 值 | 理由 |
|------|-----|------|
| max_decompose_depth | 3（可配置） | 深度 3 的树已有 goal→plan→implement→verify 四层，覆盖最复杂场景 |
| max_children_per_decompose | 7（可配置） | 单层超过 7 个子任务 → LLM 可能在推卸判断而非真正分解 |
| max_daily_decompose_per_kind | 5 | 同 kind 24h 内 decompose > 5 次 → 触发人工 review |

**深度超限行为：** 达到 max_decompose_depth 的 WorkItem 必须 fallback 到 inline execute——LLM 不能继续分解，必须自己解决。

---

## 5. Tool 面简化：Commitment Device

### 5.1 为什么从 8 个工具简化到 4 个

```
现状（8 个 task 相关工具）：
  task_create, task_get, task_list, task_update, task_delete,
  task_stop, task_output, task_list_background,
  todo_write,
  delegate_explore, delegate_plan, delegate_implement, delegate_verify

终态（4 个统一工具）：
  task_write    ← task_create + task_update + task_delete + todo_write
  task_spawn    ← delegate_* + task_stop
  task_await    ← task_output + task_list_background + status poll
  task_list     ← task_get + task_list + tree view
```

**这不是 UX 优化，这是 Commitment Device：**

| 机制 | 效果 |
|------|------|
| 封闭的 4 工具集合 | D7 承诺不再创造新的任务工具类型 |
| `Kind` 枚举替代工具名 | 新增任务类型 = 新增 Kind 值，不需要新工具 → 低摩擦但受控 |
| `Policy` 字段替代工具行为 | sync/async/readonly/parallel_ok = 正交的行为维度，不炸裂工具面 |
| Alias 期（v2.0） | 旧工具名保留为 wrapper → 不给现有 Prompt 造成 breaking change |

**Schelling 点：** 4 个工具名（write/spawn/await/list）形成了一个 **focal point** — 任何未来的任务功能必须在这些语义下表达，不能重新发明。

### 5.2 为什么前两次尝试没有解决 Tool 面问题

- `unified-task-registry` 试图统一后台存储，但保留所有工具 → **技术统一，语义仍然分裂**
- `wave-worktree-isolation` 专注 worker 隔离，与 tool 面无关
- `unified-work-tree` 的 v2.0 tool rename 是 **产权统一后的自然结果** — 当只有一个 WorkTree 时，多个工具自然退化为一个工具的不同参数

---

## 6. 终态层级博弈均衡

> **2026-06-18 修正（Codex 审查）：** 原始分析使用 Stackelberg leader-follower 模型——但 Stackelberg 要求 Leader 的承诺序贯可被 Follower 观察并依赖，且 Follower 是策略主体。这里的 D2/D4 是程序代码而非策略主体，D7 的 GetFocus 是单期内部决策。更准确的概念是 **Harsanyi (1967) Hierarchical Game with Incomplete Information**——D7 拥有任务树的私有信息，D2/D4 拥有各自执行能力的私有信息，博弈通过 Bayesian 更新持续进行。

### 6.1 均衡状态

```
┌──────────────────────────────────────────────────────────────┐
│ Harsanyi 层级博弈均衡：终态任务架构                            │
│                                                              │
│ Leader (D7) 先动：                                           │
│   1. GetFocus(session) → 选择当前最高优先级 WorkItem          │
│   2. if uncertainty > threshold: decompose → Create children  │
│   3. spawn children → 设置 RunRef                             │
│   4. await children → bubble terminal → re-resolve parent     │
│                                                              │
│ Follower 1 (RunRegistry, DM-011) 后动：                      │
│   1. Register(run) → 返回 runID                              │
│   2. AppendOutput(runID, delta) → 增量流                     │
│   3. SetTerminal(runID, status, summary) → callback WorkTree  │
│                                                              │
│ Follower 2 (D4 Workers) 后动：                                │
│   1. 接收 WorkerSpec（含 WorkItem.ID + worktree 隔离选项）     │
│   2. fork → run → join                                       │
│   3. 产出 Artifact + summary → RunRegistry.SetTerminal       │
│                                                              │
│ Follower 3 (D2 Tools) 后动：                                  │
│   1. 接收 tool call → 执行 → 返回 result                      │
│   2. 不持有任务语义（WorkTree 由 D7 注入）                     │
│                                                              │
│ Information Agent (LLM) 策略：                                │
│   1. 读取 WorkTree subtree → 理解任务上下文                   │
│   2. 对 focus WorkItem 评估 uncertainty                      │
│   3. 使用 4 个统一工具（task_write/spawn/await/list）         │
│                                                              │
│ 用户 (Principal) 观测：                                       │
│   1. WorkTree 投影 → IM 卡片展示任务树和进度                   │
│   2. RunRegistry 投影 → 子任务输出增量流                      │
│   3. FlowEvent → 进度条/DAG 图                                │
└──────────────────────────────────────────────────────────────┘
```

### 6.2 均衡性质

| 性质 | 证明 |
|------|------|
| **子博弈完美** | 每个 follower 的策略在自己的信息集上是最优的 — D4 只关心 WorkerSpec 正确性，不关心编排决策 |
| **激励相容** | D2/D4 没有动机重新声称任务产权 — WorkTree 的封闭 4 工具面让"另起炉灶"的收益为零 |
| **信息高效** | LLM 看到的 WorkTree 是单一真相源 — 不需要跨 tool 名推断任务状态 |
| **Pareto 改进** | 每个 Player 的 payoff 都 ≥ 现状 — D7 减少协调成本，D2/D4 减少语义负担，LLM 减少认知成本 |

### 6.3 与 Claude Code 的对齐

Claude Code 的任务模型是 **隐式的递归树**：
- 用户消息 = root task
- Tool call = sub-task（隐式，无持久化 ID）
- TodoWrite = task 树的可视化投影
- Plan mode = root task 的 extended thinking phase

Devrix 的 WorkTree 是 **显式的递归树**（多了持久化、异步、多方协作），但核心拓扑一致。这不是巧合 — 递归主子任务树是 **不确定性驱动探索的最小充分模型**。

---

## 7. 不完全合约风险

即使产权清晰，合约仍有漏洞。以下是需要持续监督的边界：

### 7.1 风险矩阵

| 风险 | 机制 | 博弈论诊断 | 缓解 |
|------|------|-----------|------|
| **Kind 枚举膨胀** | 新场景要求新 WorkKind → 枚举从 8 增长到 20+ | 封闭枚举是 D7 的 Commitment Device，但压力来自外部（新 worker 类型） | `Policy` 字段吸收行为差异；新增 Kind 需要 D7 domain S3-Gate review |
| **RunRef 漂移** | WorkItem.RunRef 指向已被 RunRegistry GC 的 run | What 和 How 之间的引用完整性失效 | RunRegistry callback 在 terminal 时强制更新 WorkItem status；GC 前检查反向引用 |
| **Ephemeral 边界模糊** | `kind=checklist, ephemeral=true` 被 LLM 用于非 checklist 场景 | ephemeral 是 D7 对 LLM 的信任让步 — LLM 可能滥用 | sc.Todos 投影限制呈现方式；非 checklist kind 禁止 ephemeral |
| **父通知爆炸** | 深度 5+ 的树，每个子 terminal 都 bubble → prompt 污染 | Recursive resolve 的隐藏成本 — 通知信号淹没任务上下文 | bubble 只聚合 terminal count，不传递完整 output；`task_await` 带 offset 分页 |
| **Wave 退化不完全** | `wave.TaskNode` 在 v1.1 保留为 projection → v2.0 忘记删除 | 过渡期的技术债务自然积累 → 又回到多套模型 | v2.0 tasks.md T5.5 显式列出删除 TaskNode 持久化的任务 |
| **D2 重夺产权** | D2 的 tool runner 发现"直接写 sc.Todos 更方便" → 绕过 WorkTree | D2 有局部信息优势 → 可能重新建立隐性 task 语义 | D7-S1 的 WorkTree 通过 DI 注入 D2 tool runner；`sc.Todos` 标为 `// ReadProjection only, DO NOT WRITE` |
| **DM-011 未就绪的连锁阻塞** | RunRegistry 未交付 → v1.0-v1.1 RunRef 可空 → 过渡期 spawn 无执行观测 | 依赖链的 weakest link 问题 — 可空字段被当作"永久可选" | v1.2 的 T3.1 硬依赖 DM-011 PR-1；v1.1 的 empty RunRef 发 warn log |

### 7.2 最危险的场景：隐性任务系统再生

```
场景：
  某开发者需要"定时重试失败任务"
  → 不想改 WorkTree（怕影响现有语义）
  → 在 D2 创建 RetryRegistry（新的 ID 前缀 retry_*）
  → 6 个月后，我们又有了第 7 套任务模型
```

**防御机制（2026-06-18 升级）：**

| 层级 | 机制 | AC | 类型 |
|------|------|-----|------|
| 第一道防线 | CI static analysis 检测新增 `*Registry / *Manager` 类 + `sc.Todos` 直写 | AC23 (P0, v1.0) | 机械可验证，不依赖 reviewer 主观判断 |
| 第二道防线 | Code Owner Bot 自动 @ D7 架构师（新增 task-related 实体时） | AC24 (P1, v1.1) | 即使 CI 通过也需人工 ack |
| 第三道防线 | 季度 Property Rights Audit — 扫描游离 WorkTree 外的 task 实体 | AC25 (P1, v1.1+) | 持续审计，发现隐性再生 |
| 语义层 | WorkItem.Policy 字段吸收行为变体；新增 Kind 需 S3-Gate review | 现有规则 | 降低"另起炉灶"的动机 |
| 补充 | CR 规则：非 WorkTree 的 ID 前缀 → 必须解释 | 现有规则 | 辅助，不作为 primary defense |

**关键升级：** 防御从"依赖代码审查的监督博弈"升级为"CI 自动化 + CR 互补的多层防御"。p(检测) 从 ~0.2（仅 CR）提升到 ~0.95（CI + CR + quarterly audit）。

**这是产权维护的核心挑战：** 产权不是一次性分配就一劳永逸的 — 需要**制度化的多层防御机制**来防止 property rights erosion。纯代码审查在 LLM 时代已被证明不可靠（监督博弈中 p 太低），必须用机械可验证的 CI 规则作为第一道防线。

---

## 8. 结论

### 8.1 终态任务架构的本质

终态任务架构是一个 **Harsanyi 层级博弈均衡**（信息不对称下的 Bayesian 策略互动），其中：

1. **D7 独占任务树的 What 产权** — WorkItem + WorkTree 是唯一的工作语义权威（Demsetz 产权分配）
2. **RunRegistry 提供 How 观测** — 执行句柄与任务语义分离，通过 RunRef 关联（Williamson make-or-buy 决策）
3. **D2/D4 退化为无状态 Follower** — 保留执行比较优势，放弃任务语义产权；过渡期需 CI 自动化防御阳奉
4. **LLM 面对 4 个统一工具** — task_write/spawn/await/list 形成 Schelling 点（Commitment Device）
5. **Uncertainty Anchor 机制** — LLM 的 uncertainty claim 被 historical + structural 锚定，evidence 为必需（解决 cheap talk）
6. **递归深度硬约束** — max depth 3 + max children 7 + daily limit 5（防止 cheap talk 递归放大）

### 8.2 与前两次尝试的本质差异

| 维度 | 失败模式 | 本 change 的模式 |
|------|---------|-----------------|
| 解决问题的方式 | Infrastructure 统一（注册表、隔离层） | Institutional 统一（产权分配） |
| 依赖策略 | 硬依赖未完成的 Wave Scheduler 版本 | v1.0-v1.1 RunRef 可空，v1.2 硬依赖 DM-011 |
| 范围 | 单层优化（后台任务 OR worker 隔离） | 全栈产权重分配（D7/D2/D4 同时调整） |
| 过渡策略 | 无（期待 big bang） | 5 Phase + legacy adapter + alias 期 |

### 8.3 一句话

**多套任务系统的根因是产权不清（Demsetz 产权理论），不是缺少抽象。WorkTree 用产权分配替代了 infrastructure 统一——先决定"谁拥有任务树"，基础设施会自然收敛。但产权分配必须有制度化防御：Uncertainty Anchor 防止 LLM cheap talk，CI 自动化防止开发者阳奉阴违，递归深度硬约束防止 cheap talk 递归放大。**

---

**维护：** 本文件在 S3-Gate Review 时需同步更新 `design.md` 风险部分。Codex/Cursor 双模型审查已完成，双边共识见 `gaming-analysis-bilateral-consensus.md`。完整演进路线 v1.0→v3.0 见 `version-roadmap.md`。
