# Game Theory Review — Layer SubContext (DM-20260627-003)

**Reviewer:** Claude (MiniMax-M3) — 博弈论视角
**Date:** 2026-06-28
**Target:** `devrix-d7-layer-subcontext` (Status: S3_Design 待 Review)
**Source SoT:** `demand.md` / `proposal.md` / `design.md` v0.2.0 / `review-notes.md` / `specs/*/spec_delta.md` / `tasks.md`

---

## 0. 整体评估（先结论）

这是一次**机制设计 (Mechanism Design)** 而非纯工程改动：核心是把 WorkItem 之间的"上下文博弈"从 session 单桶（**公共池塘悲剧 Common Pool**）转为**分层产权 (Property Rights)** + **承诺装置 (Commitment Device)**。设计 §3 §6 的多次"拒绝"决议是博弈论意义上**关键的反激励抑制**（anti-incentive），方向正确。但有 3 处**激励错配**需要重新审视；新增 3 条 OQ-LC-8/9/10 风险未在 design §10 风险 register 中识别。

**综合评分 8.0/10，推荐进入 S3-Gate**，补充 OQ-LC-8/9/10 后可启动 S4。

| 维度 | 评分 | 说明 |
|------|------|------|
| 机制设计完整性 | 8.5/10 | 信号通道 + 反激励抑制 + 边界清晰；scope 漂移、cohort 池通胀未识别 |
| 博弈论自洽性 | 8.0/10 | §6.2 反 Moral Hazard 方向正确；混同均衡 / 承诺松弛风险需补 |
| 与现有组件衔接 | 9.0/10 | 与 #262 Rollup / ContextGraph / context-budget 接缝清晰 |
| 验收可测性 | 9.0/10 | T21-T26 集成测试覆盖 LC1-LC6；R-OBS 表单测可定位 |
| 落地风险控制 | 7.5/10 | flag=off 默认 + 双轨；需警惕路径依赖锁定 |

---

## 1. 博弈论维度拆解

### 1.1 信号博弈 / 分离均衡

| 玩家 | 类型 | 私有信息 |
|------|------|----------|
| **Parent LLM** | Sender | Goal 真实范围；open_questions 是否需要用户介入 |
| **Child LLM** | Receiver + Sender | Execute 实际产物质量；Obs 自我评估 |
| **Observe 规则** | Verifier | 不对称信息下决定 SpawnPolicy |

**核心信号**：`ScopeContract`（design §5.2）。它是一个 **Costly Signal** —— Parent 必须为模糊任务付出"结构化范围"成本，才能获得 decompose 通行证。这是对的。但要警惕**混同均衡 (Pooling Equilibrium)** 风险：

> ⚠️ **Risk P1-A**：如果 `open_questions` 仅作软引导，Parent LLM 在不确定时会**策略性输出空 open_questions**（pooling），骗过 SpawnPolicy 后再让子 WI 兜底。结果：ScopeContract 沦为**走过场**信号，与现网"先 decompose 后发现不对"无异。

**建议**：
- `open_questions` 空 vs 非空需有 **Verifier 校验**（design §5.3 已部分提及）：例如用 Verify 节点检查 Goal directive 是否触及多个 in_scope 域 + repo 实际文件数量 → 触发强制 ObsUncertainty
- 参考 `devrix-d7-mups-v5-escape-engine` 已有的 `LoopBudget.ConsecutiveFails` 模式，给 Goal 加一个 `ScopeConfidence` 指标

### 1.2 道德风险 / 隐藏行动 (Moral Hazard)

**最大激励错配在 §6.2 决议**（design §6.2）：

> Execute **每轮** ReAct 强制 Obs* 标签 → **❌ 拒绝** — 污染 transcript；虚高 ObsFact；违反 G3/LC2/LP-5

这一决议的博弈论解释非常清晰：
- **隐藏行动**：LLM 自报 ObsFact Strength = **不可验证 (non-verifiable) 私有行动**
- **逆向选择**：如果 Strength 决定 SpawnPolicy，LLM 会**策略性虚高 Strength**（廉价话术）→ SpawnDecompose 更激进 → token cost 上升
- **审计失败**：LP-5 `Plan.SourceObservationIDs` 不可追溯"随口说的 ObsFact"（design §6.2 第 4 点 ✅）

✅ **这步反激励抑制方向正确**。但要注意**新激励错配**：

> ⚠️ **Risk P1-B**：一刀切禁止后，LLM 失去"显式不确定"的输出通道，会**改用自然语言软抱怨**（"我不太确定"、"似乎"、"可能"），结果 Observe 规则更难识别 → ObsUncertainty 漏报 → SpawnPolicy 失效

**建议**：
- 软引导 `<open_questions>`（design §6.4 ✅）作为**代理信号**是必要的
- 但 Observe 规则（R-OBS-1..7 表）需增加 **自然语言不确定性词 fuzzy match** 作为兜底（Phase 2 不在编码范围，但 R-OBS 表应预留扩展点）
- 给 `<open_questions>` 设**最低条数门槛**（如 ≥1 才能触发 R-OBS-1），避免"敷衍式 1 条"占位

### 1.3 承诺装置 / 时间不一致 (Commitment Device)

`ChildDownlink`（design §4.1）的 `ScopeIn` / `ScopeOut` 是**强承诺**：父在 Spawn 时绑定子 WI 的合法修改边界。

| 博弈阶段 | 玩家 | 承诺 |
|----------|------|------|
| t0 | Parent 写 ChildDownlink.ScopeIn | 单边承诺 |
| t1 | Child Execute | 在 ScopeIn 内行动 |
| t2 | Child terminal → Bubble | 必须符合 ExpectedReturn |

**激励分析**：这是**有限承诺 (Limited Commitment)** —— Parent 单边设 ScopeIn 但 Child 可以**事后偏离**（Modify file outside ScopeIn）。如果没有 Verify 阶段的 scope 越界检测，承诺装置失效。

**建议**：
- tasks §P1-T3 ScopeContract（design §5）已含 `Verify` 对照，但需要**显式 Verify hook** 在 Child terminal 时检查"动过的文件是否在 ScopeIn 内"
- 这与 `#262 Rollup Phase 1` 的 `buildRollupDirective` 验证结构天然契合，可作为同源实现
- 否则会出现**软承诺 (cheap talk)**：ScopeIn 沦为装饰，子 WI 实际无约束

### 1.4 串通 / 同层协调 (Collusion / Peer Coordination)

design §3.3 通道 B `PeerStatusSignal`（Phase 2）允许并行 explore 的 sibling 在 terminal 后**互相看 verdict + summary**。

**博弈论关注**：这是**信息结构 (Information Structure)** 变化 —— 同层从"完全隔离 (No Communication)"变成"terminal 后单向通报"。

**典型反模式**（值得提前警示）：
- **串通定价**：同层 sibling 在 PeerStatus 看到彼此 verdict 后，倾向于**对齐 verdict**（避免被标 outlier），损害独立判断
- **诱因效应 (Bandwagon Effect)**：后续 terminal 的 sibling 看前面 verdict → 倾向跟随
- **社会比较 (Social Comparison)**：弱 sibling 看到强 sibling 后**降低 effort**（Panda 困境）

✅ **设计已部分抑制**：`terminal-only`、`policy flag 默认 OFF`、`summary ≤ 240 chars`。**但**：
- 默认 OFF 是对的，但 ON 时应加**顺序随机化**而非按 wi_id 排序（design §3.3 + 风险 R6）
- summary 应**截断到 verdict 类型 + 1 句事实**，**不允许**含 sibling verdict 的同义改写

### 1.5 公共池塘 / 上下文通胀 (Common Pool → Inflation)

session transcript 是经典**公共池塘**：
- 每个 WI 写入都消耗 token budget
- 个体 WI 不承担全部成本（budget 是 session 级别）
- 结果：**过度使用**（每个 WI 想 append 越多越好）

design §2.2 的 `budget(Ln) = min(MaxContext × 0.5^n, floor_n)` 是 **Hardin 解决方案**（私有化 + 衰减配额），方向 ✅。但要警惕：

> ⚠️ **Risk P1-C**：衰减配额 + 同层 cohort 共享 ScopeContract = **新公共池塘**（cohort meta 也会膨胀）。需要给 cohort 也设 budget cap（design §3.3 `cohort:<parent>/signals.jsonl` append-only 无 cap）。

**建议**：加一个 `cohort_signal_budget_max` 配置（默认 8KB），触发后**降级**为 CB3 truncate 而非 fail-fast。

---

## 2. 对 §5 开放问题的反馈

| OQ | Cursor 默认 | 博弈论评估 | Claude 建议 |
|----|-------------|----------|------------|
| **OQ-LC-1** ScopeContract 持久化 | WorkItem 字段 + LastRound 镜像 | ✅ WorkItem 字段是**显式承诺**，比 LastRound 更可追溯（commitment cost 更高 = 信号更可信）。但 WorkItem 是 hot data，**写放大**风险：每次 Goal Plan 都要 update | **A: WorkItem 字段**（审计/可追溯性 > 写放大成本），拆解后 scope readonly |
| **OQ-LC-2** Materialize 实现深度 | 轻量 path | ✅ 避免**复杂机制 = 难以审计**。轻量 path 让 R-OBS 规则可以单测验证，不被 PrepareOrchestrator 副作用污染 | **A: 轻量 path**，L0 Goal 可选全量；明确 Compress 复用边界（避免 PrepareOrchestrator 隐式调用） |
| **OQ-LC-3** CG2 修订版本策略 | design 0.3.0→0.4.0 | ✅ 是。但仅 bump design 不够 —— `workitem-context-graph-design.md` 的 §F3 测试语义需要 **MODIFIED Scenario** 显式说明"cohort domain 共享"（spec_delta §MODIFIED CG2 Scenario 已做 ✅） | **0.4.0 + ADR**（记录为何从"完全隔离"→"transcript 隔离 + cohort 域共享"） |
| **OQ-LC-4** ChildDownlink vs DecomposeProposer timeline | Phase 1 规则 + Phase 2 LLM 填充 | ⚠️ **激励错配风险**：Phase 1 规则模板会让 Parent LLM **预期** Phase 2 LLM 会填充 ExpectedReturn → Phase 1 不认真写 ExpectedReturn。建议 Phase 1 ExpectedReturn 也**规则推断 + 强制非空** | Phase 1 规则模板（OK），但 ExpectedReturn 字段**强制非空**（空值 = 阻断 decompose） |
| **OQ-LC-5** PeerStatus 默认策略 | opt-in policy flag | ✅ opt-in 正确。但**建议加 cohort_size 阈值**：cohort < 3 不暴露（噪声 > 信号），≥3 才暴露 | **opt-in + cohort_size ≥ 3 才暴露** |
| **OQ-LC-6** Execute 每轮 Obs  taxonomy | 禁止 | ✅ 方向正确（详见 §1.2）。补充：`<open_questions>` 必须**最低条数门槛 ≥1** 才能触发 R-OBS-1 | **禁止 + 软引导块保底 + ≥1 门槛** |
| **OQ-LC-7** Phase 2 LLM ObservationProposer | 独立 change | ✅ 符合机制设计原则：**LLM 提案 + 规则裁决**是 G3 范式。但 LLM 输入**明确不含 wi 全文**（design §6.6 ✅），否则重新陷入 Moral Hazard | **独立 change，提案不直接进 ObsFact，必须规则校验** |

---

## 3. 未被设计充分识别的博弈风险（OQ-LC-8/9/10）

### 3.1 OQ-LC-8: 子 WI 对父 Goal ScopeContract 的**策略性放松**（Scope 漂移防御）

**博弈场景**：
- Parent 写 ScopeIn 严格 → Child 觉得"任务约束太死"
- Child terminal 时 Bubble 里**主动建议扩大 Scope** → 触发 Parent 下一次 Plan 扩大 ScopeIn
- 结果：scope 逐层**漂移扩大**（类比软件熵增 / scope creep）

**建议**：Goal ScopeContract 加 **`scope_expansion_max_ratio`** 配置（如 1.5x），超限后强制 SpawnPolicy = SpawnInline 而非继续 decompose。验证通过 ScopeIn 文件数量 delta 计算。

### 3.2 OQ-LC-9: 同层 cohort 的**信号池预算**

design §3.3 的 `cohort:<parent>/signals.jsonl` 是 append-only 但无 cap。PeerStatus + ScopeContract cohort meta 在长任务下会膨胀，形成新的公共池塘。

**建议**：加一个 `cohort_signal_budget_max` 配置（默认 8KB），触发后**降级**为 CB3 truncate 而非 fail-fast。降级次数进 metrics 用于诊断 cohort 健康度。

### 3.3 OQ-LC-10: Feature Flag 的**路径依赖锁定**

`FeatureLayerSubContextEnabled` 默认 false 是对的，但要警惕：**flag=off 路径不被使用 → 维护成本隐性上升 → flag 永远不开**。这是经典的**技术债务博弈**：短期"安全"（保留旧路径），长期"锁定"（双轨维护成本 > 一次切换成本）。

**建议**：Phase 1 验收后立刻**给一个真实工作流强制 flag=on**（如所有 depth≥2 WorkTree），避免 dead code。可设 `flag_migration_deadline`（如验收后 30 天）。

---

## 4. 子 WI 公平感知 (Peer Fairness Perception) — 软风险

design §3.3 通道 B 的 PeerStatus 让 sibling 看到彼此结果。如果 A 完成时间远短于 B，B 会觉得"被相对剥夺"（Relative Deprivation）。这不影响系统正确性但影响**用户感知**（Panda 困境）。

**建议**：Phase 2 CLI（`/task context show --wi=<id>`）同时显示 cohort peer progress（不只 verdict），降低"未知焦虑"。这是用户感知层面而非机制层面，但应登记。

---

## 5. 决议建议汇总（写回 review-notes §5）

| # | 项 | 博弈论建议 |
|---|----|----------|
| OQ-LC-1 | ScopeContract 持久化 | **WorkItem 字段**（写放大 < 审计收益）；拆解后 readonly |
| OQ-LC-2 | Materialize 深度 | **轻量 path** + Compress 复用边界明确 |
| OQ-LC-3 | CG2 版本 | **0.4.0 + ADR** |
| OQ-LC-4 | ChildDownlink 时间线 | Phase 1 **ExpectedReturn 强制非空**（空 = 阻断） |
| OQ-LC-5 | PeerStatus | **opt-in + cohort_size ≥ 3** |
| OQ-LC-6 | Execute Obs 标签 | **禁止** + `<open_questions>` ≥1 门槛 |
| OQ-LC-7 | ObservationProposer | **独立 change**，LLM 提案 → 规则裁决 |
| **OQ-LC-8** | Scope 漂移防御（新增） | Goal `scope_expansion_max_ratio`（1.5x），超限强制 SpawnInline |
| **OQ-LC-9** | cohort 信号池预算（新增） | `cohort_signal_budget_max`（8KB），CB3 truncate 降级 |
| **OQ-LC-10** | flag=on 强制路径（新增） | Phase 1 验收 30 天后 depth≥2 WorkTree 强制 flag=on |

---

## 6. design §10 风险 register 补充建议

| ID | 风险 | 概率 | 影响 | 缓解 |
|----|------|------|------|------|
| R11 | ScopeContract pooling（空 open_questions 走场） | 中 | 高 | Verifier 校验 + ScopeConfidence 指标 |
| R12 | LLM 用自然语言软抱怨代替 `<open_questions>` | 中 | 中 | 自然语言 fuzzy match 兜底（Phase 2） |
| R13 | ChildDownlink cheap talk（scope 越界无校验） | 中 | 高 | Verify hook 检查 file diff vs ScopeIn |
| R14 | PeerStatus 串通定价 / 从众效应 | 低 | 中 | 顺序随机化 + summary 截断规则 |
| R15 | cohort signals.jsonl 公共池塘膨胀 | 中 | 中 | `cohort_signal_budget_max` 降级 |
| R16 | Feature flag 永久锁定 | 中 | 中 | `flag_migration_deadline` 30 天 |
| R17 | Scope 漂移扩大（递归 spawn 提议） | 中 | 高 | `scope_expansion_max_ratio` + SpawnInline |

---

## 7. 修订路径建议（不动 design.md 主线）

1. `review-notes.md` §5：本文件引用 + 7 个 OQ 的 1-2 行结论
2. `design.md` §10 风险 register：追加 R11-R17
3. `design.md` §12 开放问题：追加 OQ-LC-8/9/10
4. `tasks.md`：Phase 1 增加 T13f（Verify hook 检查 scope 越界）、T20d（cohort_signal_budget_max 默认）
5. ADR 新增 `adr-001-cg2-cohort-domain-sharing.md` 解释从"完全隔离"→"transcript 隔离 + cohort 域共享"的博弈论依据

**S4 实现前必做**：OQ-LC-1/2/3 决议；OQ-LC-6 软引导门槛。
**S4 实现后**：OQ-LC-8/9 在 Phase 2 落地；OQ-LC-10 在 Phase 1 验收后 30 天启动。

---

## 8. 一句话总结

**整体机制设计方向正确，§6.2 反 Moral Hazard 是本 change 最有价值的决议；新增 3 条 OQ-LC-8/9/10 补足 scope 漂移 / 池通胀 / 路径锁定 3 个隐含博弈风险，建议 S4 前补 design §10/§12，否则 Phase 1 会出现 "flag 永久 off + scope 漂移扩大" 的次优均衡。**
