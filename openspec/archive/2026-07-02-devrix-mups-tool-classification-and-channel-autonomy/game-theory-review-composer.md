# Game Theory Review (Composer): MUPS Tool Control Plane + ToolChannel 自治

**Change ID:** `devrix-mups-tool-classification-and-channel-autonomy`
**Demand ID:** DM-20260701-007
**Review Type:** 博弈论视角 Review（Composer / Cursor，供 Claude 对照评估）
**Review Date:** 2026-07-01
**Peer Review:** [game-theory-review.md](./game-theory-review.md)（Codex / MiniMax-M3，v1）
**Source Documents:** demand.md · proposal.md · design.md · specs/*.md

---

## 0. Executive Summary（给 Claude 的一页结论）

| 维度 | 判断 |
|------|------|
| 机制骨架 | **同意 Codex**：类型揭示 → 菜单路由 → 可执行承诺 → 契约补全 → 信号分离 → 透传，六原语覆盖 RC-1..RC-5 三大经典失败模式 |
| 目标均衡 | **补充**：应显式声明为 **有限 horizon 子博弈完美均衡（SPE）+ 跨 session 重复博弈中的 reputation 均衡**；单 session 内 SPE，跨 session 靠 Learn ReputationStore |
| 最大未解风险 | **Principal 多目标冲突**（User 要 review、D2 要 budget、D7 要 deliverable）未在 demand 中形式化；Verify 只审计 deliverable，不审计「探索是否浪费」 |
| 与 Codex 分歧 | Shadow mode **建议保留**但应定义为 **cheap talk 校准期**；`Channel`→`PlanChannel` 改名 **P0 而非中优先级**（focal point 冲突在 nested mechanism 下会放大） |
| 对 Open Q 的回答 | 见 §8；Learn 节点 **已有** `mups/learn/` + ReputationStore，本 change 应写清「接入边界」而非留 Phase E |

**若只改一处设计文档**：在 `design.md §2` 增加 **Equilibrium Concept + Multi-Principal Payoff** 两节（见 §4.2、§5.3）。

---

## 1. 与 Codex Review 的关系

Codex v1 已高质量覆盖：

- Stackelberg 序列博弈建模（§1）
- Hardin / Hart–Holmström / Akerlof 三病根映射（§2）
- H1–H6 逐假设解读 + cheap talk / burden of proof / Learn 空转（§3–§4）
- ROI 优先级表（§5）

**本文不做重复展开**，仅在以下方向 **加深或修正**：

1. **嵌套机制设计**（PlanKind Channel × ToolChannel 双层）
2. **多 Principal 效用冲突**（User / D2 / D7 / D1 四方）
3. **有限 vs 无限 horizon**（单 session SPE vs 跨 session repeated game）
4. **默认 metadata 的 pooling 均衡**（PR-A 退路风险）
5. **对 Codex §7 Open Questions 的可验证回答**

---

## 2. 博弈结构：嵌套 Stackelberg + 隐藏行动

### 2.1 双层 Mechanism Designer

Proposal 已澄清两套 Channel：

| 层 | Designer | 路由键 | 决策 |
|----|----------|--------|------|
| L1 | PlanKind `Channel` | `plan.PlanKind` | commit / protocol / scenario / exploration **执行策略** |
| L2 | `ToolChannel` | `tool.EmissionClass` | Fact / Action / Probe / Experiment **终止不变量** |

博弈论上这是 **嵌套 Stackelberg**：Nature 先抽 task_kind → Plan 选 PlanKind → Execute 选 PlanChannel → 每个 tool call 上 ToolChannel 选终止规则 → LLM 作为 follower 选 explore/synthesize。

**关键推论**：若 L1 选 `ExplorationPlan` 而 L2 对 `read_file` 标 `EC_Fact` 且 `OpenEnded`，则 **L2 的 Bounded(n) 可被 L1 战略性地「架空」**。Demand 假设 H2 只保证「按 emission_class 路由」，未约束 **PlanKind 与 EmissionClass 的交叉一致性**。

**建议（新增 REQ）**：

> 当 `PlanKind == ExplorationPlan` 且 `task_kind == review` 时，Filter v2 必须 **禁止** 将 Probe 类工具的 `IterationBound` 降为 `OpenEnded`；或 PlanChannel 必须在 routing 前 **override** 为 task_kind bound。

否则存在 **multi-dimensional deviation**：LLM 通过 Plan 层探索策略 + Tool 层 Fact 标签，绕过 Probe 的 `Bounded(15)`。

### 2.2 隐藏行动与道德风险（Moral Hazard）

User（Principal）观测到的只有：

- 最终 deliverable / 红卡
- token 账单（间接）

LLM（Agent）在 Execute 阶段的「再读一次文件」对 User **不可观测**，对 D2 截断逻辑 **部分可观测**，对 Verify **事后可观测**（tool call 日志）。

这是标准 **hidden action + limited liability**：

- Agent 不承担 token 社会成本（Hardin，Codex §2.1）
- Verify 仅在终局审计，**中间无 short-run IC 约束**（除非 ToolChannel 每步 enforce）

DM-007 的 ToolChannel + Bounded(n) 本质是把 **hidden action 转成每步可合约化的 sufficient statistic**（call count + same-query repetition）。方向正确。

**补充风险**：`PromptPressure` 是 **cheap talk from mechanism to LLM**（Farrell–Rabin），不是 hard transfer。iter=16 的 `SynthesizeNowSignal` 才是 **commitment device**。P0-AC-1 只测 hard stop，**应额外测** soft warning@5 被 LLM 忽略后的期望 iter 分布（Codex §3.4 已提，此处强调为 **必要 T 点**）。

---

## 3. 对 RC-1..RC-5 的博弈论重述（增量）

| RC | Codex 框架 | Composer 增量 |
|----|------------|---------------|
| RC-1 | 无 termination signal | **Pooling equilibrium**：所有 tool 看起来都可无限调用；v3 metadata 是 **separating equilibrium**  attempt |
| RC-2 | Akerlof 信号失灵 | 截断 = **信号破坏**（destroyed signal）；TruncateMarker = **恢复 cheap talk 的可观测性**，使 LLM 能更新 belief on `complete=false` |
| RC-3 | Execute 不分类 | **Uniform pricing** 导致 adverse selection：Probe 工具被当作 Fact 定价（无 bound） |
| RC-4 | Verify 一维契约 | **Incomplete contract**；4 元组 = **多维 screening**；缺 burden of proof 则 Partial 成为 **risk-neutral LLM 的 optimal gamble** |
| RC-5 | Reason 丢失 | **信息链断裂** = 重复博弈中无法 punishment；下轮 Observe 无法 conditioning on `deliverable_missing` |

---

## 4. 六假设：同意、修正、反对

### 4.1 同意 Codex 的核心判断

- **H1 cheap talk**：init-time `EmissionClass` 无 runtime audit → 需 Learn drift 表（Codex §3.1、§4.3）
- **H3 L4–L6 vs L0–L3**：需 cross-check（Codex §3.3）
- **H5 burden of proof**：按 EmissionClass 分配举证（Codex §3.5 表）
- **H6 Reason 需回流 Learn**：否则只对 User 可读，不对 Agent 可学习（Codex §3.6）

### 4.2 修正：目标均衡应写两句，不是一句

Codex 建议 SPE + Learn 贝叶斯更新。Composer 补充 **分 horizon**：

**单 session（有限步 T）**：

> 在 ToolChannel 给定 `Bounded(n)` 与 hard reject 下，Execute 子博弈的目标均衡是 **SPE**：在 iter ≥ n 的子博弈中，LLM 的 dominant 策略是 synthesize（因 further tool call 被 mechanism reject，payoff 为 −∞ 或固定 fail）。

**跨 session（重复博弈，折扣因子 δ）**：

> User 以 verdict + reason 作为 **public signal**；Learn 节点维护 `ReputationEvidence(β)`，下轮 Observe 的 `AdaptivePrior` 调整 emission_class 信任度。目标为 **reputation equilibrium**：declared_class 与 actual drift 高者，下轮 Filter 收紧或 bound 降低。

**design.md 应显式写**：本 change **Phase A–D 只保证单 session SPE 的必要条件**；reputation 均衡需 **Phase E+ / Learn 接入**（见 §6）。

### 4.3 反对/降级：Filter v2 第四维 `workspace`

Codex §4.5 建议 defer workspace 维。Composer **强烈同意**：

- 每增一维 = 新 **type space** × **菜单大小** 指数扩张
- 在 19 工具、4 emission_class 尚未稳定前加 workspace = **过早 mechanism complexity**

**建议**：Phase D 只 ship `agent + emission_class + task_kind` 三维；workspace 写入 OOS 或 feature flag off。

### 4.4 升级优先级：`PlanChannel` 改名

Codex 排 #2（中）。Composer 升为 **P0 门禁**（与 P0-AC-6 并列）：

- 嵌套机制下，两个 `Channel` 接口同包 = **focal point 灾难**（Schelling）
- 实现 PR-B 前未完成 rename → code review 必然混淆 PlanKind 策略与 EmissionClass 终止

**最小改法**：`type Channel interface` → `type PlanChannel interface` + type alias `Channel = PlanChannel` 一 release，再删 alias。

---

## 5. 三方案 A/B/C 的博弈论再评估

| 方案 | 均衡类型 | Composer 评语 |
|------|----------|---------------|
| A 治本 | Separating + SPE（若 PR-A 默认正确） | **选 A 正确**；但 PR-A 默认 `Action+OpenEnded` 会维持 **pooling**，削弱分离 |
| B 半治本 | 部分 separating | 长期留在 **mixed strategy**（部分 tool bound、部分没有） |
| C 治标 | 仅移动 Verify 阈值 | **belief 不更新**，LLM 仍可在 Execute 子博弈 deviate；Verify 再严 = 终局 penalty 无 intermediate IC |

**对 PR-A fallback 的明确立场**（回应 Codex Open Q #6）：

- 若某 surface 文件缺 `EmissionClass`，**禁止** 静默默认 `Action+OpenEnded`
- CI gate 应 **fail build**，而非 runtime 宽松默认
- `read_file` / `grep` / `glob` 在 PR-A 必须 **显式** `EC_Probe + Bounded(15)`，否则 sess_1782885908460 类均衡 **重现**

---

## 6. Learn 节点：不是空转，但未接入本 change

代码现状（2026-07-01）：

- `internal/layers/orchestration/mups/learn/`：`DefaultLearner`、`ReputationStore`、`AdaptivePrior`
- `item_pipeline_test.go` 已 wire `learn.NewDefaultLearner`
- `tracing.go` 已注入 `learn.prior.*` span attributes

**结论**：Learn **不是 placeholder**；缺的是 **(declared_emission_class, observed_call_pattern) → ReputationEvidence** 的映射。

**建议写入 design.md §Learn Boundary**：

| 信号 | 来源 | Learn 写入 | 下轮 Observe 消费 |
|------|------|------------|-------------------|
| `verify_exit_reason=deliverable_missing` | Verify | `ReputationEvidence` ↓ | `AdaptivePrior` 更保守 |
| `probe_iter_exceeded` | ProbeToolChannel | tool-level drift flag | Filter 收紧该 tool |
| `(declared EC, actual same-query count)` | Execute audit | drift rate | v0 backlog 重标 |

本 change **In Scope 最小增量**：Phase C 把 `verify_exit_reason` 写入 Learn 的 feedback 通道（已有 `FeedbackMemory`）；**Out of Scope**：完整 drift 分类器。

---

## 7. Shadow Mode：同意，但定义为「机制 cheap talk 校准」

Codex §4.4 建议 Phase B shadow 1 周。Composer 同意，并形式化：

- **Shadow period**：ToolChannel 并行计算「若 enforce 会否 reject」，**不 block**，只 log `would_reject=true`
- **切换条件**：`would_reject` 与人工标注不一致率 < 5%，且 P0 review 任务 false positive < 2/10 session
- 博弈论：从 **correlated equilibrium**（log 公开）过渡到 **SPE**（hard enforce），降低 **off-equilibrium belief 震荡**

---

## 8. 对 Codex Open Questions 的回答

| # | 问题 | Composer 回答 |
|---|------|---------------|
| 1 | `Channel` 改名影响面？ | `mups/execute/` 内 4 实现 + tests + bootstrap wire；**type alias 可 1-release 兼容**；historical change 文档不需改 ID |
| 2 | Shadow false positive 阈值？ | 建议 **<5% would_reject 与产品预期不符** + **0 次** review 任务误杀 deliverable 已有 case |
| 3 | PromptPressure baseline 数据？ | **当前无**；P0-AC-1 前应先跑 10 session baseline，记录 soft@5 后平均剩余 iter |
| 4 | Learn 结构？ | **已有** `mups/learn/`；本 change 应在 design 写 **Learn 接入边界**（§6 表） |
| 5 | L4–L6 vs L0–L3 cross-check？ | **应在 S3-Gate 单独 T 点**；例如：`Bounded` 不得 override readonly-destructive guard |
| 6 | PR-A fallback 太宽松？ | **是**；默认 `Action+OpenEnded` 稀释治本；应 **grep gate fail**，非 silent default |

---

## 9. 优先级建议（Composer ROI，与 Codex §5 对照）

| 序 | 动作 | vs Codex | 理由 |
|----|------|----------|------|
| 1 | P0-AC-6 grep gate + **禁止 silent default** | 一致 | pooling 均衡根源 |
| 2 | **`PlanChannel` rename** | **升级 P0** | 嵌套机制 focal point |
| 3 | PlanKind × EmissionClass **交叉一致性规则** | **新增** | 防 L1 架空 L2 |
| 4 | VerifyContract burden of proof 表 | 一致 | 防 Partial gamble |
| 5 | design.md Equilibrium + Multi-Principal 两节 | 扩展 Codex #5 | 文档元层 |
| 6 | Shadow mode + PromptPressure baseline | 一致 | 迁移路径 |
| 7 | Learn feedback 写入 `verify_exit_reason` | 扩展 Codex #6 | 重复博弈起点 |
| 8 | workspace Filter 维 defer | 一致 | 降 mechanism 维度 |

---

## 10. 给 Claude 的评估提示（Meta）

请 Claude 重点评估以下 **Composer 与 Codex 的差异**是否成立：

1. **PlanKind × ToolChannel 交叉 deviation** 是否是真实攻击面，还是 over-engineering？
2. **`PlanChannel` rename 是否应升为 P0**，还是 type alias 足够？
3. **PR-A silent default vs grep fail** 哪个更符合「治本叙事」？
4. **单 session SPE vs 跨 session reputation** 双均衡声明是否应进 demand P0 验收？
5. **Learn 最小接入**（仅 `verify_exit_reason` → FeedbackMemory）是否应纳入 DM-007 In Scope？

---

## 11. 参考（补充 Codex §8）

- Myerson, R. B. (1991). *Game Theory: Analysis of Conflict* — mechanism design baseline
- Holmström, B. (1979). "Moral Hazard and Observability" — hidden action
- Fudenberg & Tirole (1991) — repeated games / reputation
- Bolton & Dewatripont (2005) — contract theory multi-dimensional screening

---

## 更新历史

- 2026-07-01：v1 创建（Composer 博弈论 review，对照 Codex game-theory-review.md v1，供 Claude 评估）
