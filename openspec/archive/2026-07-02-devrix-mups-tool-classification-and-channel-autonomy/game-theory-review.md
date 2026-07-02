# Game Theory Review: MUPS 5 节点 × Tool 元数据 Control Plane + ToolChannel 自治

**Change ID:** `devrix-mups-tool-classification-and-channel-autonomy`
**Demand ID:** DM-20260701-007
**Review Type:** 博弈论视角 Review（Cursor 走查用）
**Reviewer:** Codex (MiniMax-M3)
**Review Date:** 2026-07-01
**Source Documents:**
- [demand.md](./demand.md)
- [proposal.md](./proposal.md)
- [design.md](./design.md)
- [specs/tool-spec-v3.md](./specs/tool-spec-v3.md)
- [specs/execute-channels.md](./specs/execute-channels.md)
- [specs/verify-contract.md](./specs/verify-contract.md)

---

## 0. TL;DR

机制设计骨架**正确且完整**：类型揭示 + 路由 + 终止约束 + 契约补全 + 信号分离 + 信息透传，这 6 个原语覆盖了 Akerlof / Hart–Holmström / Hardin 三个经典病根，三方案选 A 是合理判断。

**主要薄弱点不在机制本身，而在三个元层面**：

1. **cheap talk 没解**（`emission_class` 声明无审计回路）
2. **缺均衡概念声明**（不知道系统在收敛到哪种 equilibrium）
3. **Learn 节点没接入**（跨 session 不学习，机制是 static 的）

**如果只能改一个**：在 `design.md §2` 显式声明"目标均衡是 SPE，跨 session 由 Learn 节点做 `emission_class` 贝叶斯先验更新"。

---

## 1. 博弈建模：MUPS 5 节点当作多智能体序列博弈

把 Pipeline 当作一个有信息结构的扩展型博弈，局中人：

| 角色 | 英文 | 行动空间 / 职责 |
|------|------|------------------|
| User | Principal | 想要 deliverable，最大化期望价值 |
| LLM | Agent | `{explore, synthesize, abort, repeat}`，单步内不知道用户真实效用 |
| Tool | Resource Provider | 类型决定"我该被调用几次就该收敛" |
| Execute + 4 ToolChannel | Mechanism Designer | 制定路由 + 终止规则 |
| Verify | Auditor | 末端审计，发布 verdict |
| D1 feishu Render | Renderer | 把 verdict 翻译给用户 |

这是**多阶段 Stackelberg 序列博弈**：Plan 选 PlanKind → Execute 选 Channel → Channel 选 ToolChannel → Tool 产 result → Verify 颁 verdict → D1 渲染。每一步都是前一步的 follower，且每一步都在 commit 一个后续行为。

需求把治本方案的核心赌注放在：**把 4 类正交分解从节点级下沉到 tool metadata 级 + 给每个 tool 自带终止信号**。博弈论语言里就是"在信息结构允许的最早节点做 type revelation"。方向正确，下面逐假设拆开看。

---

## 2. 现状博弈论诊断（RC-1..RC-5 的本质）

当前 MUPS 是**严重失败的机制设计**，至少有 3 个并列病根：

### 2.1 公地悲剧（Tragedy of the Commons，Hardin 1968）

LLM 把 D2 的 8K token 预算当作"公共池塘"。每次 `read_file` 都有**私利**（多看一眼更稳）但**社会成本**（token 累积、挤占 budget、无 deliverable）。LLM 个体理性 = 集体非理性，正是 Hardin 标准情形。9 字段 ToolSpec 完全没有把"社会成本"显性化给 LLM，所以 LLM 没有任何 incentive 收敛。

### 2.2 不完全契约（Incomplete Contract，Hart–Holmström 1987）

现有 Verify 节点只判"deliverable 存在"，是一维契约，留下大量**契约缝隙**给 LLM 投机（探索性 finalText 蒙混过关）。需求 RC-4 指的就是这事。

### 2.3 信息不对称 + 信号失灵（Akerlof 1970 / Spence 1973）

D2 截断对 LLM 不透明，相当于"高质量完整 read"和"低质量截断 read"对 LLM 看起来一样，是经典的**信号不可分（pooling）**。LLM 拿到信号后无法做有效 Bayesian 更新 → 继续重读 → 继续被截 → 循环。

**6 个假设本质上是把这 3 个病根都治了**：

- 公地悲剧 → `IterationBound.Bounded(n)` 把"社会成本"显性化
- 不完全契约 → VerifyContract 4 元组扩维
- 信号失灵 → TruncateMarker 强制分离信号

机制设计语言：H1+H2+H3 解决 IC（incentive compatibility），H4 解决 hard budget constraint，H5 解决 contract completeness，H6 解决 information preservation（decentralized Bayesian chain）。

---

## 3. 6 个假设的博弈论解读

### 3.1 H1: `emission_class` + 4 control plane 字段 — 类型揭示

**价值**：正确地把工具从匿名变有类型，让 routing 不再盲匹配。

**风险 — Cheap Talk（Farrell–Rabin 1996）**：
`emission_class` 是工具作者 init-time 声明、runtime 不验证。等价于"label 不收费"。`call_agent` 可以声明成 `Fact` 来规避 Probe 的 `Bounded(n)`。**demand 没规定声明与运行时行为的审计回路**。

**建议**：在 Learn 节点（详见 §4.3）建立 `(declared_class, actual_class)` 漂移表，drift 高的 tool 进 v0 backlog 重标。让 cheap talk 升级为 truthful revelation（给声明加 marginal cost：被 audit 抓到要重标）。

### 3.2 H2: 4 ToolChannel 按 emission_class 路由 — 机制分解

**价值**：把"一锅煮 channel"拆成 4 个按类型定制的机制，是标准的**菜单设计（menu design）**：每个 type 看到自己的菜单。

**风险 — Coalition Drift**：
同一个 tool 在不同上下文里可能跨类（`read_file` 一次性读是 Fact，反复重读就是 Probe）。`OnResult` 需要做"行为重分类"。当前 `design.md` 没显式说明。

**建议**：`OnResult` 后跑一次轻量 classifier（call_count > 3 且 evidence 趋同 → 升级为 Probe），并在 audit log 标记 drift。

### 3.3 H3: register time 挂 LTL-Lite L4-L6 — 可执行承诺

**价值**：L4 Bounded、L5 Quotient、L6 Synthesize 三件套合起来是个**完整契约**：

- L4 防无界
- L5 防震荡
- L6 强制终态

**风险 — L0-L3 兼容**：
`observability/instrument/ltl/` 当前是空目录（`grep "L[0-7]-"` 在该路径下无结果），L4-L6 与现有 L0-L3 安全 invariant 没有交叉验证声明。一个 `Bounded(15)` 是否会跟现有 L3 "no-destructive-on-readonly" 冲突？`demand.md §6` 风险表只提"LTL-Lite 是新概念"但没提与现有 invariant 的兼容性。

**建议**：在 `specs/execute-channels.md` 加一节"Compatibility with L0-L3"，至少列出 3 条 cross-check 规则。

### 3.4 H4: `Bounded(n)` + PromptPressure — 可信威胁 + Schelling 聚点

**价值**：iter=16 注入 PromptPressure = Schelling focal point（共同知识信号），iter=17 拒绝 = 硬执行。这两步配合是 subgame perfect equilibrium 的标配。

**风险 — Bounded Rationality（Simon 1957）**：
PromptPressure 假设 LLM"看见"提示就"理解"并"执行"——但 LLM 是 Simon bounded rational，可能视而不见或执行 2-3 步后才合成。`demand.md` 没给"被忽略后"的兜底数据。

**建议**：跑一组 empirical measure，统计 PromptPressure 注入后 LLM 平均还要几次 tool call 才 synthesize。这个数字会决定 P0-AC-1 的 16 是不是合理阈值。如果发现 16 不够，应该 escalate 到 10/12 而不是放宽。

### 3.5 H5: VerifyContract 4 元组 — 契约补全

**价值**：Hart–Holmström 框架下，从 1 维扩到 4 维是把"投机空间"压到很小的标准动作。

**风险 — Burden of Proof 未分配**：
4 元组列出来了，但没说"谁举证"。一个 Probe 工具能不能用 `source_uncertainty=High` 来合理化 `evidence=0`？

**建议 — 按 emission_class 显式定义举证规则**：

| EmissionClass | 举证要求 |
|---------------|----------|
| Fact | deliverable text 自证 |
| Action | state change evidence 必传 |
| Probe | source_quality 必填 |
| Experiment | result reproducibility 必传 |

### 3.6 H6: `meta["verify_exit_reason"]` 7 跳透传 — 分布式 Bayesian 链

**价值**：把 Reason 当 typed payload 端到端传递是去中心化 Bayesian 更新的标准实现。

**风险 — D1 render 是否只做字符串显示**：
如果 D1 把 `deliverable_missing` 当字符串贴上去而不让 LLM 看见 / 不让下个 session 看见，那 Reason 就**对 LLM 策略不可学习**，只是用户可读。

**建议**：D1 render 把 reason 同时塞进 session summary 写回 Learn 节点，让跨 session 的 LLM 调用能基于历史 verdict 调整 emission_class 标注。

---

## 4. 跨切面盲点

### 4.1 命名 Schelling 冲突（low severity, high 长期成本）

现有 `internal/layers/orchestration/mups/execute/channel.go:69` 的 `Channel` interface 还在，新 `ToolChannel` 紧挨着同一个 package。`grep "type Channel"` 即可看出是两个同义不同物的抽象。

博弈论里这是 coordination game 的 **focal point 错位**——下一次有人来加代码时极易混淆。

**建议**：合并前先把旧 `Channel` 改名 `PlanChannel` 或 `ExecutionStrategy`，把"Channel"这个名字让给新抽象。

### 4.2 缺 Equilibrium Concept 声明

6 个假设铺了 6 个原语，但**没声明目标均衡是什么**：

- subgame perfect equilibrium？
- Bayesian Nash？
- correlated equilibrium？
- evolutionary stable strategy？

需求 P3 说"工具层强收敛"其实指的是 SPE，但没显式说。

**建议**：在 `design.md §2` 加一段 "Equilibrium Concept"：

> 系统设计为：每次 tool call 在 Emit 后处于 subgame perfect equilibrium，下一 iter 没有 dominant 策略偏离。跨 session 通过 Learn 节点做 emission_class 贝叶斯先验更新。

### 4.3 跨 session 学习缺位（Learn 节点空转）

MUPS 第 5 节点是 Learn，但本 change 完全不碰。`(declared_emission_class, actual_behavior_via_evidence)` 配对是天然的 Bayesian 先验更新信号——但没人在做这事。

**建议 — Phase B/C 加一个轻量 audit 表**：

```
tool_name -> declared_class -> actual_class_drift_rate -> v0_backlog_flag
```

drift 高的 tool 进 backlog 重标。这把 H1 的 cheap talk 升级为 truthful revelation。

### 4.4 Phase B 缺 Shadow Mode

P0-AC 顺序是 A → B → C。Phase B（4 ToolChannel + LTL-Lite）一旦 enforce 就是硬切，没有渐进。

**建议**：Phase B 跑 1 周 shadow mode（log-only，新旧 channel 并行跑、不强制 enforce），量 false positive 率后再切。

博弈论里这是 **correlated equilibrium 替代 Nash 的迁移路径**，比"一次性 commit"稳得多。

### 4.5 Filter v2 的 4 维可能过度

从 2 维（agent+risk）到 4 维（+emission_class+task_kind+workspace）是**状态空间爆炸**。每个新维都是新的"type"要验证。

**建议**：先跑 1 个 sprint，统计 4 维里每维的 hit rate——如果 workspace 维 0 命中就该 deferred。

### 4.6 工具元数据迁移工作量被低估

`demand.md` 里 PR-A "含 19 工具默认 metadata 迁移"是正确决定（不留 Phase E 尾巴），但 19 个工具每个 `emission_class` 是 4 选 1 + `IterationBound` 3 选 1 + `ConvergenceContract` 4 选 1，理论组合 48 个，单工具标注是 cognitive load 很高。

**建议**：加一个半自动分类器——基于现有 Channel history，把过去 30 天 read_file 类的 tool 按"是否在迭代中被反复重读"自动归类 Probe / Fact，给作者做"建议 + 人工 override"。

---

## 5. 优先级建议（按 ROI 排序）

| 序 | 动作 | 价值 | 成本 | 关联假设 |
|----|------|------|------|----------|
| 1 | `grep -L "EmissionClass:"` 做 19 工具硬 gate（已规划 P0-AC-6） | 高 | 低 | H1 |
| 2 | 旧 `Channel` interface 改名为 `PlanChannel`，合并前消歧 | 中 | 低 | §4.1 |
| 3 | Phase B shadow mode 1 周 + PromptPressure 命中率测量 | 高 | 中 | H4 |
| 4 | VerifyContract 4 元组的 burden of proof 显式规则（按 emission_class 切） | 高 | 低 | H5 |
| 5 | `design.md §2` 补 "Equilibrium Concept" 段 | 中 | 低 | §4.2 |
| 6 | `tool -> declared_class -> actual_class_drift_rate` audit 表 | 中 | 中 | H1 + §4.3 |
| 7 | 半自动 emission_class 分类器 | 中 | 高 | §4.6 |
| 8 | Filter v2 第 4 维（workspace）跑 1 sprint 再决定 | 低 | 低 | §4.5 |

---

## 6. 与上游 DM 的关系（从博弈论视角重画）

```
DM-005 (Verify synthesize enforce) ---+
DM-006 (D2 budget profile)          ---+--- 治标 -- 都是症状处理
DM-012 (deliverable convergence)    ---+
                                      |
                                      v (本 change 治本)
DM-007 Tool metadata control plane + ToolChannel 自治
                                      |
                                      +---  Phase B/C 加 audit 表 (建议 #6)
                                      +---  Phase B 跑 shadow mode (建议 #3)
                                      |
                                      v (后续可选)
Phase E   19 工具值分批迁移到 R4 strict + Learn 节点接入贝叶斯更新
```

---

## 7. Open Questions 给 Cursor

1. `Channel` interface 改名是否会影响调用方测试 / 文档 / 历史 change 引用？
2. shadow mode 跑 1 周的 false positive 阈值是多少算"可切"？
3. PromptPressure 注入后 LLM 平均几次 tool call 才 synthesize 的 empirical baseline 现在有数据吗？
4. Learn 节点目前是否已经有结构（哪怕是 placeholder）？如果有，本 change 是否要在 design 里加一段"Learn 节点接入边界"？
5. `demand.md §6` 风险表里"现有 LTL-Lite L0-L3 安全 invariant 改造（OOS-5）"被列为 OOS，但 L4-L6 与 L0-L3 的兼容性是不是该在 S3-Gate 单独过一次 cross-check？
6. 19 工具 metadata 迁移如果某工具作者不主动标，PR-A 的 fallback 是什么（默认 Action + OpenEnded，是不是太宽松，治本力度被稀释）？

---

## 8. 参考引用

- Hardin, G. (1968). "The Tragedy of the Commons". *Science*.
- Akerlof, G. A. (1970). "The Market for Lemons". *QJE*.
- Spence, M. (1973). "Job Market Signaling". *QJE*.
- Farrell, J. & Rabin, M. (1996). "Cheap Talk". *JEP*.
- Hart, O. & Holmström, B. (1987). "The Theory of Contracts". *Advances in Economic Theory*.
- Simon, H. A. (1957). *Administrative Behavior*.
- Schelling, T. C. (1960). *The Strategy of Conflict* (focal point).

---

## 更新历史

- 2026-07-01：v1 创建 (Codex 博弈论视角 review，对应 demand.md v1)
