# 终态任务架构 — 版本演进路线图

**Change ID:** devrix-unified-work-tree
**日期:** 2026-06-18
**版本:** v2 — 经 Codex 审查修正
**状态:** Active — 路线图覆盖 v1.0 → v3.0

---

## 0. 演进哲学

```
终态不是一次交付的，是三个里程碑的收敛：

Milestone A (v1.0–v1.2): 产权集中 — D7 独占 WorkTree，6→1 模型
Milestone B (v1.5–v2.0): 递归求解 — 最小递归 → 完整递归 + 工具统一
Milestone C (v2.1–v3.0): 自演化 — 跨会话 + 学习 + 自适应
```

每个里程碑交付**独立可用的价值**，不依赖后续版本。每个版本都是博弈论均衡的一次逼近。

---

## 1. 版本全景

```
v1.0 ──→ v1.1 ──→ v1.2 ──→ v1.5 ──→ v2.0 ──→ v2.1 ──→ v3.0
产权      写入      执行      最小      完整      跨会话    自演化
集中      统一      观测      递归      递归               任务系统
│         │         │         │         │         │         │
│         │         │ 硬依赖   │ 软依赖   │ 依赖    │         │
│         │         │ DM-011   │ DM-011   │ DM-011  │         │
│         │         │ Phase1   │ Phase1+  │ Phase2+ │         │
└─────────┴─────────┴─────────┴─────────┴─────────┴─────────┘
  独立      独立      独立      独立      独立      独立
  交付      交付      交付      交付      交付      交付
```

### 1.1 每个版本的独立价值

| 版本 | 用户可感知的变化 | 不依赖后续版本的独立价值 |
|------|----------------|----------------------|
| v1.0 | `/task list` 行为不变，底层已是 WorkTree | 产权集中完成；CI 防线就位 |
| v1.1 | `/task list --tree` 看到父子关系；todo_write 自动归到当前 goal 下 | 所有写入路径走 WorkTree |
| v1.2 | 子任务执行进度可追踪（飞书卡片显示 "2/5 完成"） | RunRef 挂接可观测 |
| **v1.5** | **LLM 自动把大任务拆成子任务，逐个执行** | **最小递归可用** |
| v2.0 | 复杂任务全自动递归；只有 4 个 task 工具 | 完整递归 + 深度约束 + Uncertainty Anchor |
| v2.1 | 第二天问"昨天那个任务完成了吗"→ 有答案 | 跨会话只读查询可用 |
| v3.0 | 系统自己学会"这类任务通常拆 3 步" | 任务模板自学习 |

---

## 2. 各版本详细设计

### v1.0 — 产权集中 (Foundation)

**目标:** 在不动任何外部接口的前提下，把 6 套任务模型中的 5 套收敛到 WorkTree 底层。

**交付物:**
- `WorkItem` + `WorkKind` + `ExecPolicy` + `CreateWorkItemInput`
- `WorkTree` CRUD + 树遍历 + `GetReadyItems`
- `DiskWorkItemStore` v2（自动迁移 v1 Task JSON）
- `TaskManager` 委托 WorkTree（`task_create` 等 API 不变）
- **CI static analysis** 检测 D2 直写 `sc.Todos` + 新增 `*Registry`（AC23）

**博弈论意义:** Demsetz 产权分配完成——D7 独占 WorkTree 的所有权，D2/D4 的工具面和执行面不变但底层数据源已切换。这是产权集中的"静默革命"——外部零感知，内部已统一。

**不依赖:** 无外部依赖。无需 DM-011。

---

### v1.1 — 写入统一 (Write Paths)

**目标:** 所有创建任务/子任务的入口都写 WorkTree 的 `parent_id`，任务树结构开始可见。

**交付物:**
- Session 首条消息自动创建 `kind=goal` WorkItem
- `delegate_*` spawn 写 `parent_id` + `kind`
- PlanMode approve 批量创建 `kind=implement` 子项
- `todo_write` → `WorkTree.UpsertChecklist`（`kind=checklist, ephemeral=true`）
- Wave `SynthesizeTaskGraph` → WorkTree 子树
- WaveScheduler 从 WorkTree 读 ready 子项（替代独立 TaskGraph）
- `task_list --format=tree` 树形输出
- **Code Owner Bot** 自动 @ D7 架构师（AC24）

**博弈论意义:** 树结构开始对 LLM 可见——LLM 能在 context 中看到 "当前 task 有 3 个子 task，其中 2 个已完成"。这为 v1.5 的递归求解提供了信息基础。

**不依赖:** v1.0。无需 DM-011。

---

### v1.2 — 执行观测 (Execution Visibility)

**目标:** WorkItem 与 RunRegistry 挂接，子任务执行状态可追踪。

**交付物:**
- spawn 时写 `WorkItem.RunRef`
- RunRegistry terminal → callback 更新 WorkItem status
- terminal bubble notify 父 WorkItem
- `QueryWorkPlan` 树形读模型（含 RunRegistry background 状态）
- `GetByRunRef` 索引
- **季度 Property Rights Audit** 脚本（AC25）

**博弈论意义:** What/How 的 Williamsonian 边界正式运营——WorkItem.RunRef 是外键关系契约，RunRegistry 不拥有任务语义但提供执行可观测性。

**硬依赖:** DM-011 Phase 1 (Register + SetTerminal + GetByRunRef)。

---

### v1.5 — 最小递归 (Minimal Recursion MVP) ⭐ 新增

**为什么需要这个版本:** v2.0 的"完整递归引擎"改动量太大（递归引擎 + uncertainty anchor + tool rename + 删 legacy，~500+ 行）。如果 v2.0 延迟，v1.2 的 WorkTree 只是一棵"静态树"——LLM 能看到父子关系但不会主动分解任务。v1.5 用一个最小改动让树"活起来"。

**目标:** RunTurn 能做**单层**递归——LLM 看到 focus WorkItem，决定要不要 decompose，spawn 子任务，等子任务完成，然后把结果聚合回 focus。

**交付物:**
- `RunTurn.resolve(focus)` hook：单层 focus → decompose? → spawn → await children → re-resolve parent
- 基础 uncertainty：仅 LLM claim（无 anchor，anchor 等 v2.0）
- 单层分解：decompose 只创建一层子任务，子任务不再递归
- `task_await` 基础实现（基于 RunRegistry terminal callback）
- `work_tree.max_decompose_depth = 1`（v1.5 硬编码为 1）

**明确不做:**
- 多层递归（等 v2.0）
- Uncertainty Anchor（LLM claim 直接使用，v2.0 再加 anchor）
- Tool rename（保留 8 个工具名，v2.0 统一）
- 深度/宽度约束（v1.5 只有 1 层，不需要约束）
- 删 legacy（v2.0 再删）

**用户可感知:**
- 之前：用户说"实现暗色模式"→ LLM 只能自己一步步做，或手动 `/task create`
- v1.5：LLM 自动创建 3 个子任务（explore 主题现状 → implement 暗色变量 → verify 视觉一致性），逐个 spawn，等全部完成后汇报

**博弈论意义:** 递归求解的 MVP——验证"LLM decompose → spawn → await → parent continue"这个循环本身是否成立。v1.5 用最简单的形式（单层、无 anchor）把这个循环跑通，v2.0 再加复杂度（多层、anchor、约束）。

**软依赖:** DM-011 Phase 1 (Register + SetTerminal)。如果 DM-011 只交付 Phase 1，v1.5 就能交付。

---

### v2.0 — 完整递归 (Full Recursion)

**目标:** 多层递归 + Uncertainty Anchor + 工具统一 + 清理 legacy。

**交付物:**
- 多层递归 decompose（depth ≤ 3）
- `GetFocus` + kind/uncertainty 优先级调度
- **Uncertainty Anchor 机制**（AC27）——historical failure + structural complexity + evidence 锚定
- 递归深度/宽度约束（AC20: depth ≤ 3, AC21: fallback inline, AC22: daily limit 5）
- Tool 面统一：`task_write/spawn/await/list`（4 个工具替代 8+ 旧工具）
- Alias 期：旧工具名保留为 thin wrapper + deprecation warn
- 删除 legacy：flat Task 直写 / wave.TaskNode 持久化 / sc.Todos 权威

**博弈论意义:** 完整的 Harsanyi 层级博弈均衡——LLM 在多层递归中每一层都面临 uncertainty 评估，Uncertainty Anchor 防止 cheap talk，深度约束防止递归放大，4 工具面是 Commitment Device。产权过渡完成——D2/D4 的 legacy task 模型被正式删除。

**依赖:** DM-011 Phase 2+ (output delta stream)。v1.5 已验证核心循环。

---

### v2.1 — 跨会话 (Cross-Session)

**目标:** 任务树的生命周期超越单个 Session。

**交付物:**
- 跨 Session WorkItem 只读查询：`QueryWorkPlan(historical_session_id)`
- 历史 Session 的 WorkItem 默认 lock（immutable）
- 跨 Session mutable 引用协议：propose-modify → 新 Session → arbitration
- DM-011 RunRegistry terminal 状态 = lock 信号
- 飞书卡片展示："上次你在这个项目有 3 个 task，其中 2 个完成，1 个未完成——要继续吗？"

**用户可感知:**
- 第二天打开 Devrix 问"昨天那个重构完成了吗？"→ 有答案
- "继续昨天的第三个任务"→ 系统创建新 Session，继承历史 WorkItem 的上下文

**博弈论意义:** 任务产权从"per-session 私有"扩展到"跨 session 持久"——D7 不仅是单个会话的编排者，也是跨时间的任务记忆持有者。

**依赖:** DM-011 完整交付。

---

### v3.0 — 自演化 (Self-Evolving Task System)

**目标:** WorkTree 的结构和使用模式通过历史数据自我优化。

**交付物:**

**3.0.1 自适应 Uncertainty 阈值:**
- 不再使用全局固定阈值（0.3/0.7）
- 根据用户历史行为学习：这个用户偏好激进（少分解）还是稳健（多分解）
- 根据项目类型学习：前端项目的 explore 和 implement 的 uncertainty 分布不同

**3.0.2 WorkItem Kind 自动检测:**
- LLM 遇到无法归类的新场景 → 提议新 Kind（附 evidence）
- D7 S3-Gate review 审批或拒绝
- 审批通过 → Kind 枚举扩展 + WorkTree 支持
- 避免 Kind 枚举"封闭僵化"（太封闭无法适应新场景）和"无限膨胀"（太开放失去 Commitment Device 效果）

**3.0.3 跨项目任务模板:**
- 高频子任务树自动提取为模板：`{goal: "添加暗色模式"} → {explore: "现有主题", implement: "CSS 变量", verify: "视觉检查"}`
- 同类项目的 WorkTree 模式交叉学习
- 新项目启动时自动建议任务模板

**3.0.4 WorkTree 结构自优化:**
- 分析历史 decompose 的 terminal 结果
- 发现"每次 decompose 成 3 个并行 + 1 个串行 terminal 率最高"→ 自动调整此场景的 decompose 策略
- 发现"某个 Kind 的 uncertainty 与 historical failure 的相关性很低"→ 调整 anchor 公式的权重

**用户可感知:**
- 第一次在项目里添加暗色模式：系统建议"这类任务通常分 3 步"，用户确认
- 第五次：系统直接按最优结构 decompose，用户只需要 approve terminal 结果
- "你怎么知道要这么拆？" "在这个项目和你类似的项目里，这个结构 terminal 率最高"

**博弈论意义:** 任务系统从一个"静态的机制设计"演化为"学习的机制设计"——D7 不仅是当前的 Leader，也是历史的 Learner。Uncertainty Anchor 不再依赖固定的 historical failure rate，而是 Bayesian 更新的后验分布。**递归求解从 separating equilibrium 进化为 adaptive equilibrium。**

**依赖:** v2.1 完整交付 + 足够的历史数据（至少跨 10 个 Session 的 WorkTree 数据）。

---

## 3. 版本依赖链

```
                    DM-011 Phase 1         DM-011 Phase 2+
                         │                      │
v1.0 ─→ v1.1 ─→ v1.2 ──→ v1.5 ──→ v2.0 ──→ v2.1 ──→ v3.0
  │       │       │        │        │        │        │
  │       │       │        │        │        │        │
  └───────┴───────┴────────┴────────┴────────┴────────┘
                    每版独立交付价值

关键依赖决策：
  - v1.5 只软依赖 DM-011 Phase 1（最少接口）
  - v2.0 需要 DM-011 Phase 2+（output delta stream）
  - v3.0 需要 10+ Session 历史数据积累
```

---

## 4. 风险与缓解（更新）

| 风险 | 影响版本 | 缓解 |
|------|---------|------|
| DM-011 只交付 Phase 1 | v2.0+ 延迟 | v1.5 已经可用——"最小递归"是独立价值 |
| v1.5 单层递归体验不够好 | 用户不满意 | 明确预期：v1.5 是 MVP，v2.0 才有完整能力 |
| v3.0 数据不足 | 学习效果差 | v3.0 的"模板建议"可降级为手动 `/plan` |
| 版本太多导致用户困惑 | 采用率低 | 每个版本 1 个 PR；用户只感知到 3 个 milestone |
| DM-011 完全失败 | 整个递归线 | v1.0-v1.1 的 WorkTree 本身就有价值（统一真相源）；备选：Legacy BackgroundRegistry 提供基础观测 |

---

## 5. 每个 Milestone 的博弈论进展

```
Milestone A (v1.0–v1.2): 产权集中
  博弈论: Demsetz 产权分配 + Williamson make-or-buy
  均衡: 产权清晰后，D2/D4 没有动机再创造新 task 模型
  验证: CI static analysis 持续 0 告警 30 天

Milestone B (v1.5–v2.0): 递归求解
  博弈论: Cheap Talk → Anchor + Bayesian Persuasion
  均衡: LLM 在每层递归中提供 evidence-backed uncertainty
  验证:AC27 集成测试（空 evidence → 回退 anchor）

Milestone C (v2.1–v3.0): 自演化
  博弈论: 静态机制设计 → 学习的机制设计（Adaptive Equilibrium）
  均衡: D7 从历史数据学习最优 decompose 策略
  验证: 同场景 terminal 率在 10 个 Session 后显著提升
```

---

## 6. 与现有文档的关系

| 文档 | 更新内容 |
|------|---------|
| `design.md` | 引用本路线图；§10 风险更新 |
| `tasks.md` | 新增 v1.5 / v2.1 / v3.0 Phase |
| `.openspec.yaml` | version_scope 扩展到 v3.0 |
| `gaming-analysis.md` | §8 补充完整演进路线引用 |
