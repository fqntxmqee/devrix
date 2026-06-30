# Demand: MUPS+WorkTree 传播闭环修复

- **Demand ID:** DM-20260701-001
- **Change ID:** devrix-mups-propagation-convergence
- **Priority:** P0
- **Domain:** D7 Orchestration
- **Status:** S2 Clarified
- **Source:** 2026-07-01 D7 MUPS+WorkTree 设计逻辑审查（上下传播 / 向上反馈 / 不确定性发散收敛 / LLM 交互验收标准）

---

## 1. 背景

D7 MUPS（Observe→Plan→Execute→Verify→Learn）叠加 WorkTree（Goal→decompose→child→rollup）构成"发散-收敛"自治闭环。本需求是对该闭环的**算法语义层**审查结论，覆盖两组问题：

- **A 组（闭环传播缺陷，6 条）**：向下传播 / 向上反馈 / 不确定性数值闭环在代码中的逻辑断裂点。
- **B 组（LLM 交互清晰度缺陷，6 条）**：MUPS 5 节点与大模型交互时，"期望的发散范围"与"收敛的验收标准"是否清晰传达给 LLM。

本需求与 DM-20260630-013（D2+D7 工程实现硬化：silent swallow / 并发 / 安全 / 压缩闭环）**不重叠**——后者是实现层缺陷，本需求是 MUPS 算法语义缺陷。

闭环结构本身**清晰且边界严谨**（typed ChildDownlink/RollupReport、唯一 SpawnPolicy 裁决入口、深度/子节点/24h 限流多重发散边界），问题集中在**收敛侧的数值表达、终止保证，以及发散/验收标准对 LLM 的可见性**。

---

## 2. A 组：闭环传播缺陷

### RH-MUPS-01 不确定性单调比率器破坏收敛信号 〔Critical〕

- **现象**：`item.Uncertainty` 在 WorkItem 生命周期内单调不减；即使所有 child pass、rollup 成功，parent 存储的不确定性仍停在历史最大值。
- **代码**：`sessionorchestrator/item_pipeline.go:366-368`
  ```go
  if item.Uncertainty > uncertaintyMean {
      uncertaintyMean = item.Uncertainty   // ratchet：只升不降
  }
  ```
  随后 `ApplyPipelineRound` 执行 `item.Uncertainty = round.UncertaintyMean`。
- **根因**：比率器（max）本意是"不确定性不应因单轮乐观而虚降"，但它使收敛在数值层面**永久不可见**。任何读 `item.Uncertainty` 判断"是否已收敛 / 是否仍需发散"的下游（含 `ComputeUncertainty` 的 `llmClaim` 回灌、阈值门）都会得到偏高读数。
- **验证**：已确认两处写入；ratchet 路径无下降分支。
- **影响**：收敛达成但系统认为仍高度不确定 → 可能触发不必要的二次发散 / escalate。

### RH-MUPS-02 `item.Uncertainty` 双写路径语义冲突 〔Critical〕

- **现象**：同一字段两个 writer 方向相反，最终值取决于时序。
- **代码**：
  - `item_pipeline.go` → `ApplyPipelineRound`（比率向上，永不降）
  - `workmodel/resolve.go:42-43` → `reevaluateParentAfterChild`：
    ```go
    u := ComputeUncertainty(parent, stats, parent.Uncertainty, 0)
    _ = tm.Tree().SetUncertainty(sessionID, parent.ID, u)   // child 全 pass 时可降（如 0.8→0.16）
    ```
- **根因**：WorkTree 结构信号（`ComputeUncertainty`）与 MUPS 信誉信号（`ComputeUnifiedUncertainty` 含 Wilson）两套不确定性来源在同一字段交汇，且更新策略（max vs replace）不一致，存在重复计数与覆盖竞争。
- **验证**：已确认两条写入路径与相反方向。
- **影响**：收敛信号不确定；与 RH-MUPS-01 互相打架（一条想降、一条强抬）。

### RH-MUPS-03 Rollup 无独立重试上限 → 收敛断裂点 〔Warning（偏高）〕

- **现象**：rollup 轮对所有非 Pass 裁决恒 `SpawnInline`，且 apply 处强制 `status=InProgress`，唯一边界是 session loop max=16；耗尽后 `focus!=nil` 但循环静默 `break`，parent 既非 Completed 也未走 human gate。
- **代码**：`spawn_policy.go:53/70/87`（rollup→inline）+ `item_pipeline.go:454-456`（强制 InProgress）+ `session_turn_loop.go:104/136`（loop max + break）。无 `RollupRetries` 计数器（已 grep 确认）。
- **根因**：发散侧有对称的终止保证（depth/daily-limit → `SpawnEscalateHuman`），收敛侧**缺少对应的"放弃→升级人工"出口**。
- **影响**：rollup verify 持续不过时，session 静默结束、目标未收敛、用户无显式失败信号。这是闭环里唯一真正的死路。

### RH-MUPS-04 best_effort 把"全失败"判为可收敛 → 失败被洗成成功 〔Warning〕

- **现象**：`Completed+Failed == Total` 即触发 rollup，`Failed==Total` 也满足；rollup 合成"全失败摘要"，`verifyRollupArtifact` 仅校验长度/关键词/exit_code 可能 Pass → parent 标 `Completed`。
- **代码**：`rollup_gate.go:56-57`（best_effort gate）+ `rollup_verify.go`（不感知子裁决聚合）。
- **根因**：rollup gate 与 rollup verify 都不聚合"子裁决成功率"——收敛只看合成产物形态，不看子结果质量。
- **影响**：全失败任务可能对外呈现为成功，掩盖真实失败。

### RH-MUPS-05 发散未真正缩小问题范围（scope 不细分）〔Info / 设计层〕

- **现象**：`ChildSpec.ScopeIn` 为空时，child 继承 parent **全量** InScope，ScopeContract 不做子集细分。
- **代码**：`workmodel/child_downlink.go:26-33`（fallback 继承父全量 scope）。
- **根因**：decompose 仅靠 LLM 的 `Directive` 文本区分子任务，"问题空间"未被机械切小。
- **影响**：发散可能产出 N 个与父等宽的子任务，不确定性不必然下降——分治价值被削弱，收敛被迫全靠 Verify 兜底。

### RH-MUPS-06 child 不确定性退化为低置信 fact，不上浮为不确定性信号 〔Info〕

- **现象**：子的高不确定性被转成 `strength=1-UncertaintyMean` 的 **ObsFact**（CatBusiness），而非 `ObsUncertainty`。
- **代码**：`sessionorchestrator/item_observe.go:282-300`（`observationsFromChildStructuredBubbles`）。
- **根因**：向上反馈把"子任务自己也没把握"压成弱事实，父在 rollup Observe 阶段看不到"子的不确定性"维度。
- **影响**：父的发散/收敛判断失真——子的不确定性不会驱动父继续发散。

---

## 3. B 组：MUPS 5 节点与 LLM 交互的发散/收敛清晰度缺陷

> 结论先行：**发散范围**对 LLM 是"盲"的（不告知树状态），**收敛验收标准**对生产者不可见（验收 bar 藏在确定性正则里、失败 Reason 不回灌）。

### RH-MUPS-07 Plan 发散提案对树状态盲目 → 盲发散 〔Critical〕

- **现象**：`StrategicPlanProposer` 让 LLM 提案 `execution_mode/scope_in/child_specs`，但 user prompt 只含 `work_item_id/directive/observation_ids/observation_summary`。
- **代码**：`strategic_plan_proposer.go:141-152`（`buildStrategicPlanUserPrompt`）。LLM **不知道**：当前 depth / max depth、已有兄弟子任务数 / `DefaultMaxChildren` 剩余配额、24h decompose 剩余配额、父 ScopeContract（无法做子集划分）。
- **根因**：发散的硬约束在代码层（`CapChildSpecs` / depth / daily-limit）事后截断或降级，LLM 提案时对这些边界无感。
- **影响**：LLM 提案"想分 N 个"被截成 2 个、或"想分"被静默降级为 inline，LLM 全程不知情 → 发散范围不可控、不可解释。

### RH-MUPS-08 prompt "最多 2 项" 与代码 cap 不对齐 + 战术硬编码 〔Warning〕

- **现象**：prompt 硬写"decompose 最多 2 项 / react_iters_hint 1-5"，但真正生效的是 `CapChildSpecs`+`DefaultMaxChildren`、`DefaultWorkItemMaxIters`。两套上限分散，魔法数字不一致。
- **代码**：`strategic_plan_proposer.go:16-33`（appendix 写死 "max 2"）vs `decompose.go`/`spawn_apply.go` 的 cap 常量。
- **根因**：违反 `03-conventions` D7 反战术硬编码——发散数量/迭代上限应来自统一配置注入 prompt，而非 prompt 与代码各写一份。
- **影响**：上限调整需改两处且易漂移；LLM 收到的约束可能与实际不符。

### RH-MUPS-09 `scope_in` 无细分语义指引 〔Warning〕

- **现象**：prompt 让 LLM 给 `scope_in:["path/"]`，但未约束"必须是父 scope 的真子集 / 子任务间互不重叠 / 合并覆盖父全集"。
- **代码**：`strategic_plan_proposer.go:16-33` + `mapRawChildSpecs`（直接透传 ScopeIn，无校验）。
- **根因**：与 RH-MUPS-05 同源——发散的"范围划分"在 prompt 层和代码层均无契约。
- **影响**：子任务范围可能重叠/越界/留白，发散无法保证"覆盖且不重复"。

### RH-MUPS-10 Verify 纯确定性正则，验收 bar 对生产者不可见 + 失败 Reason 不回灌 → 盲重试 〔Critical〕

- **现象**：Verify 节点**无 LLM**，全靠确定性规则：
  - leaf：`verifyP0P1FileLine` 检 `fileLineCitationRE` + `p0p1SeverityRE`（summary 含 P0/P1 + file:line）。
  - rollup：`verifyRollupArtifact` 检 summary ≥500 runes + 含 "p0/p1" + 不含 denylist。
  - Execute 侧仅经 `AppendDeliverableExecuteHint` 注入一个 schema **tag**（机器标记），**不告知**"≥500 字 / 必须 file:line 引用 / 禁止 planning 元话术"等具体验收要点。
- **代码**：`deliverable_verify.go:48-85` + `rollup_verify.go:22-68` + `deliverable_execute.go:12-21`。失败后 inline 重试，但 verify 的 `Reason` **不回写**进下一轮 execute prompt。
- **根因**：验收标准（acceptance criteria）与生产提示（execute hint）不对称——bar 藏在 verify 正则里，producer 看不到、失败后也得不到反馈。
- **影响**：LLM 盲目重试，收敛靠运气；max_iters 耗尽 → Partial/InProgress（叠加 RH-MUPS-03 死路）。

### RH-MUPS-11 DeliverableSchema 由 LLM 自选可绕过收敛门 〔Warning〕

- **现象**：`StrategicPlanProposal.DeliverableSchema` 会覆盖 `InferDeliverableSchema` 的 directive 关键词推断；LLM 若选 `not_applicable`，`VerifyDeliverable` 直接 NotApplicable，Verify 退化为只看 exit_code。
- **代码**：`item_pipeline.go:191-193`（strategic 覆盖 inferred）+ `deliverable_verify.go:28-30`（NotApplicable 短路）。
- **根因**：让被验收方自选验收标准，存在"自降门槛"逃逸路径。
- **影响**：收敛实质验收门可被 LLM 主动绕过。

### RH-MUPS-12 rollup denylist 中文硬编码战术 + 覆盖脆弱 〔Info〕

- **现象**：`rollupPlanningDenylist = ["parallel explore","我将要","我将","todo_write"]` 写死在 Go 源码。
- **代码**：`rollup_verify.go:14-19`。
- **根因**：违反 `03-conventions` D7 反战术硬编码（自然语言 marker 应集中到 i18n / `materialize/format_hints.go`）；4 项中英混杂，覆盖不全、易绕过。
- **影响**：收敛验收的"反元话术"判定脆弱且不可维护。

---

## 4. 问题汇总

| ID | 组 | 严重度 | 一句话 | 主要代码位 |
|----|----|--------|--------|-----------|
| RH-MUPS-01 | A | Critical | 不确定性单调比率器使收敛不可见 | item_pipeline.go:366 |
| RH-MUPS-02 | A | Critical | item.Uncertainty 双写路径冲突 | resolve.go:42 / item_pipeline.go |
| RH-MUPS-03 | A | Warning+ | Rollup 无重试上限/无升级出口 | spawn_policy.go / session_turn_loop.go:136 |
| RH-MUPS-04 | A | Warning | best_effort 把全失败洗成成功 | rollup_gate.go:56 / rollup_verify.go |
| RH-MUPS-05 | A | Info | 发散不细分 scope，分治失效 | child_downlink.go:26 |
| RH-MUPS-06 | A | Info | 子不确定性退化为弱 fact | item_observe.go:282 |
| RH-MUPS-07 | B | Critical | Plan 发散提案对树状态盲目 | strategic_plan_proposer.go:141 |
| RH-MUPS-08 | B | Warning | prompt 上限与代码 cap 不对齐+硬编码 | strategic_plan_proposer.go:16 |
| RH-MUPS-09 | B | Warning | scope_in 无细分语义指引 | strategic_plan_proposer.go:16 |
| RH-MUPS-10 | B | Critical | 验收 bar 对生产者不可见+失败不回灌 | deliverable_verify.go / rollup_verify.go |
| RH-MUPS-11 | B | Warning | schema 自选可绕过收敛门 | item_pipeline.go:191 |
| RH-MUPS-12 | B | Info | rollup denylist 硬编码战术 | rollup_verify.go:14 |

---

## 5. 验收口径（L5 草案，Given-When-Then）

- **L5-MUPS-01**：GIVEN parent 所有 child 终态为 Completed，WHEN reevaluate + rollup 成功，THEN `item.Uncertainty` SHALL 反映下降（≤ 单轮 historical），不被 ratchet 钉死。
- **L5-MUPS-02**：GIVEN `item.Uncertainty` 由 pipeline 与 reevaluate 先后写入，THEN 二者 SHALL 经唯一 reconcile 函数合并，结果与写入顺序无关。
- **L5-MUPS-03**：GIVEN rollup 连续 N 轮非 Pass（N=配置上限），THEN SHALL 转 `SpawnEscalateHuman` 或显式 `Failed`，不得静默 break。
- **L5-MUPS-04**：GIVEN `Failed==Total`，WHEN rollup verify，THEN parent SHALL NOT 标为 Completed（至少 Partial/Failed + 显式说明）。
- **L5-MUPS-07**：GIVEN Plan 发散提案，THEN user prompt SHALL 含 depth/max_depth、剩余 children 配额、父 scope；LLM 提案超额时 SHALL 收到结构化 reject 反馈。
- **L5-MUPS-10**：GIVEN deliverable schema 适用，THEN execute prompt SHALL 含可读验收要点；WHEN verify Partial，THEN 失败 Reason SHALL 回灌下一轮 execute prompt。
- **L5-MUPS-11**：GIVEN directive 关键词推断出 schema，THEN LLM 的 `not_applicable` SHALL NOT 覆盖推断结果（只能收紧不能放宽）。

---

## 6. 范围与非目标

- **In scope**：D7 workmodel + sessionorchestrator 内 MUPS 算法语义；prompt 契约（发散范围 / 验收标准注入与回灌）；不确定性数值闭环。
- **Out of scope**：D2/D3/D4 实现；DM-20260630-013 已覆盖的工程实现缺陷；WaveScheduler DAG 调度。
- **非目标**：不改 MUPS 5 节点顺序、不改 Intent 四链分发、不引入新的发散策略类别。
