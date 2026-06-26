# WorkItem × MUPS Pipeline 统一方案设计

**文档类型:** 详细架构设计（目标驱动）
**Domain:** D7 Orchestration（D7-S1 WorkModel + D7-S2 SessionOrchestrator + D7-S5/S6 MUPS）
**Status:** Draft — 方案设计（待 Review）
**Version:** 0.6.0
**Last Updated:** 2026-06-26
**Parent:** `d7-domain.md` · `pipeline-architecture.md` · `task-planning-design.md`
**Related:** `workitem-pipeline-unification-design.md` · `workitem-context-graph-design.md`
**Related Tech Debt:** `openspec/tech-debt/worktree-v2-deferred.md` (TD-WT-01/05/06)

> **Change 意图（工作标题）：** `devrix-d7-workitem-pipeline-unification`
>
> 将 **Turn Loop × WorkTree 递归** 与 **MUPS 五节点管道** 合并为一条纵轴：**每个 WorkItem 跑一轮 typed pipeline，结构化信号驱动是否 spawn 子 WorkItem**。

---

## §0 根本目标（North Star — 不可妥协）

以下 5 条是本方案存在的理由。任何设计决策、PR 取舍、性能优化 **不得削弱** 这些目标；若冲突，以本表为准。

| ID | 根本目标 | 可验证承诺 | 失败即否决 |
|----|----------|------------|------------|
| **G1** | **不确定问题可递归探索** | 给定 open-ended / 多 hypothesis 指令，系统能在 depth/budget 内自动分解子 WorkItem 并并行/串行探索，直到 Verdict 收敛或 Escalate | 只能单轮 OrchestratePath + 无结构化 spawn |
| **G2** | **信号可传递、可审计** | 父节点 spawn 决策 **必须** 由子节点或本节点 `WorkItemPipelineRound`（含 Verdict/PlanKind/ExitReason/LP-5 ID）驱动，禁止纯自然语言总结触发分解 | spawn 由 LLM 自由文本或 `task_write` 隐式触发且无 round 记录 |
| **G3** | **LLM 参与但不独断** | Observe/Plan/Verify/Decompose 提案可经 LLM；**SpawnPolicy 裁决、PP-1/2/3、状态迁移、depth/budget 门控** 必须规则化、可单测 | SpawnPolicy 或 terminal 状态完全由 LLM 一次调用决定 |
| **G4** | **WorkTree 是工作语义唯一 SoT** | 所有任务分解、依赖、进度、spawn 结果持久化为 WorkItem；Wave/TaskGraph 仅为 Execute 投影 | 并行路径与 WorkTree 双轨且 WorkTree 滞后 |
| **G5** | **Turn Loop 是 session 级时钟** | 用户每条指令进入 `RunSessionTurnLoop`：取 focus → 跑 item pipeline → 处理 spawn/await → 直至 session 工作集清空或 Escape | ProcessMessage 直进 OrchestratePath 且 RunTurn/WorkTree resolve 链断开 |

### 非目标（Explicit Non-Goals）

| 非目标 | 说明 |
|--------|------|
| 保证 LLM 结论正确 | D7 负责结构与信号，内容质量归 D6 advisory |
| 替换 `/plan` PlanMode 人工审批流 | PlanAgent 只读探索 + 用户 approve 保持不变 |
| 无限深度探索 | depth=3、max children=7、daily limit、EscapeEngine budget 硬约束保留 |
| 一次 PR 重写 ingress | 分 Phase A→D 渐进，每 Phase 可独立验收 |

---

## §1 问题陈述（现状 vs 目标）

### 1.1 现状（v6.1.0）

```
ProcessMessage
  ├─ buildObserveRequest (session 级)
  ├─ EnsureGoal (根 WorkItem，但与 pipeline 弱绑定)
  └─ IntentFast/Orchestrate → OrchestratePath.Run (session 级)
        ├─ SynthesizeTaskGraph
        ├─ SyncWaveNodes → WorkTree 投影
        └─ processAutoClose → Learn (async, session 级)

RunTurn + ResolveHint + DecomposeChildren 存在，但未成为 ingress 主路径。
WorkItem.Uncertainty 与 MUPS UncertaintyCoord 两套系统未统一。
```

### 1.2 目标态（G1–G5 合一）

```
ProcessMessage
  ├─ buildObserveRequest (session 级 prior)
  ├─ EnsureGoal
  └─ RunSessionTurnLoop
        loop:
          focus ← GetFocus
          round ← RunItemPipeline(focus)    // per-WorkItem 五节点
          apply SpawnPolicy(round)          // 规则裁决
          bubble / await / continue
        until session work set closed or Escape
```

---

## §2 核心抽象

### 2.1 两维状态（不破坏现有 TaskStatus 机）

| 维度 | 字段 | 枚举 | 职责 |
|------|------|------|------|
| 生命周期 | `WorkItem.Status` | pending / in_progress / completed / failed / cancelled | 现有 `status.go` 合法迁移，terminal 仍 Lock |
| Pipeline 相位 | `WorkItem.RoundPhase` | idle / observe / plan / execute / verify / learn / decide / await_child | 五节点 + spawn 裁决 + 等子树 |

**不变式 I1：** `Status=pending` 时 `RoundPhase` 必为 `idle`。
**不变式 I2：** `RoundPhase∈{observe..learn}` 时 `Status=in_progress`。
**不变式 I3：** terminal `Status` 时 `RoundPhase=idle` 且 `Locked=true`。

### 2.2 WorkItemPipelineRound（G2 核心契约）

```go
// 位置建议: workmodel/pipeline_round.go
type WorkItemPipelineRound struct {
    RoundNo         int
    WorkItemID      string
    SessionID       string

    // LP-5 血缘（必填，缺失则 round 无效）
    ObservationIDs  []string
    PlanID          string
    PlanKind        plan.PlanKind
    ArtifactID      string
    VerdictID       string

    // 结构化信号（spawn 唯一输入）
    VerdictKind     verify.VerdictKind
    ExitReason      verify.ExitReason
    LearningClass   learn.LearningClass
    UncertaintyMean float64   // 写入 WorkItem.Uncertainty

    // 裁决输出
    SpawnPolicy     SpawnPolicy
    ChildSpecs      []ChildSpec   // LLM 提案，规则校验通过后填充
    SpawnRationale  string        // 审计用，非决策依据

    StartedAt       time.Time
    CompletedAt     time.Time
}

type SpawnPolicy string

const (
    SpawnNone            SpawnPolicy = "none"
    SpawnDecompose       SpawnPolicy = "decompose"
    SpawnParallelExplore SpawnPolicy = "parallel_explore"
    SpawnAwait           SpawnPolicy = "await"
    SpawnInline          SpawnPolicy = "inline"
    SpawnEscalateHuman   SpawnPolicy = "escalate_human" // TD-WT-05
)
```

**不变式 I4（G2）：** `DecomposeChildren` 仅允许在 `SpawnPolicy==SpawnDecompose` 且 `LastRound` 已持久化后调用。
**不变式 I5（G3）：** `SpawnPolicy` 由 `SpawnPolicyEvaluator(round, treeContext)` 纯函数产出，LLM 不得直接写 `SpawnPolicy` 字段。

### 2.3 Uncertainty 统一（G2 + G3）

```
WorkItem.Uncertainty = clamp01(
    0.35 * (1 - ReputationEvidence.WilsonLower)  // MUPS LP-1
  + 0.25 * historicalUncertainty(childStats)      // 现有 resolve.go
  + 0.25 * (1 - VerdictConfidence)                // Verify Evidence
  + 0.15 * evidenceScore(evidenceCount)           // 现有 uncertainty.go
)
```

阈值：`AdaptiveThreshold.ThresholdFor(userID)` 接入 decompose 门控（闭合 TD-WT-01）。

---

## §3 状态机

### 3.1 WorkItem 组合状态转移

```
                    ┌──────────────────────────────────────┐
                    │  pending + idle                       │
                    └───────────────┬──────────────────────┘
                                    │ RunItemPipeline 启动
                                    ▼
                    ┌──────────────────────────────────────┐
                    │  in_progress + observe→learn          │
                    └───────────────┬──────────────────────┘
                                    │ Learn 完成
                                    ▼
                    ┌──────────────────────────────────────┐
                    │  in_progress + decide                 │
                    │  SpawnPolicyEvaluator                 │
                    └─┬─────────┬─────────┬─────────┬──────┘
          SpawnNone   │         │         │         │ SpawnInline
                      ▼         ▼         ▼         ▼
                 completed/  decompose  parallel  再跑一轮
                  failed     │         explore    observe
                             ▼         (ephemeral)
                    in_progress + await_child
                             │
                             │ 子节点 terminal
                             ▼
                    ReevaluateParent → decide 或 completed
```

### 3.2 RunItemPipeline 五节点时序（per WorkItem）

| 相位 | 入口函数（现有 / 新建） | 输入 | 输出写入 |
|------|-------------------------|------|----------|
| observe | `buildObserveRequest` + Observe quantize | `item.Directive`, session prior | `ObservationIDs`, `UncertaintyReport` |
| plan | `DefaultPlanner.Plan` | `UncertaintyReport` | `PlanID`, `PlanKind`, PP-1/2/3 |
| execute | `ChannelRouter.Dispatch` 或 Wave 投影 | `Plan` | `ArtifactID` |
| verify | `Verifier.VerifyWithRetry` | `Plan`, `Artifact` | `VerdictKind`, `ExitReason` |
| learn | `Learner.Learn` | `Verdict` | `LearningClass`, `ReputationStore` |
| decide | **`SpawnPolicyEvaluator`** (新) | `WorkItemPipelineRound` draft | `SpawnPolicy`, `ChildSpecs` |

---

## §4 SpawnPolicyEvaluator（G3 规则表）

**输入：** `round`（learn 完成后 draft）、`treeCtx`（depth, childStats, dailyLimit, threshold）

| 优先级 | 条件 | SpawnPolicy | 目标 |
|--------|------|-------------|------|
| R0 | `treeCtx.RunningChildren > 0` | `SpawnAwait` | G5 避免重复 spawn |
| R1 | `depth >= MaxDecomposeDepth` | `SpawnInline` | 非目标：无限深度 |
| R2 | `dailyLimit exceeded` | `SpawnEscalateHuman` | TD-WT-05 |
| R3 | `VerdictKind==Pass` && `PlanKind∈{Commitment,Protocol}` | `SpawnNone` → completed | G1 收敛 |
| R4 | `VerdictKind==Pass` && exploratory PlanKind | `SpawnNone` → completed | 探索成功 |
| R5 | `VerdictKind==Partial` && `UncertaintyMean > threshold` | `SpawnDecompose` | **G1 核心** |
| R6 | `VerdictKind==Fail` && exploratory | `SpawnDecompose` 或 `SpawnParallelExplore` | G1 换 hypothesis |
| R7 | `VerdictKind==Indeterminate` && parse failure | `SpawnInline`（重试同节点） | LP-2 隔离 |
| R8 | default | `SpawnNone` → failed | 停止扩散 |

**LLM 角色（G3）：** 仅在 R5/R6 下调用 **DecomposeProposer** 产出 `ChildSpec[]`；`checkDecomposeLimits` + DAG 校验后才 `DecomposeChildren`。

---

## §5 Turn Loop 主路径（G5）

### 5.1 RunSessionTurnLoop

```go
// 建议位置: sessionorchestrator/session_turn_loop.go
func (o *SessionOrchestrator) RunSessionTurnLoop(ctx context.Context, sessionID string) (<-chan *EngineEvent, error)
```

**循环不变式 I6：** 每 iteration 最多对一个 focus 执行一次完整 `RunItemPipeline`（decide 结束前不 restart observe）。

**与 EscapeEngine 关系（G3）：** 每 iteration 开头 `EscapeEngine.Evaluate(loopCtx)`；`ForceExit` / `EscalateToHuman` 终止 loop。

### 5.2 ProcessMessage 路由变更（Phase C）

| IntentKind | 现状 v6.1.0 | 目标态 |
|------------|-------------|--------|
| IntentSkip | close | 不变 |
| IntentCommand | CommandHandler | 不变 |
| IntentFast / IntentOrchestrate | OrchestratePath.Run | **RunSessionTurnLoop** |
| delegate_wave 工具 | OrchestratePath | 保留 OrchestratePath 为工具后端（非 ingress 主路径） |

---

## §6 设计决策与根本目标追溯

| 决策 | 选项 | 选定 | 追溯目标 |
|------|------|------|----------|
| D1 Pipeline 粒度 | session vs WorkItem | **WorkItem** | G1, G2 |
| D2 Spawn 触发 | LLM task_write vs 规则 | **规则 SpawnPolicyEvaluator** | G2, G3 |
| D3 ParallelExplore 持久化 | 建子 WorkItem vs ephemeral | **Ephemeral**（结果写回父 LastRound） | G4 简洁；跨 Turn 追踪才 Decompose |
| D4 Execute 引擎 | Wave-only vs ChannelRouter | **ChannelRouter 优先**；多 Step DAG 才投影 Wave | G4 投影不升格 SoT |
| D5 根 WorkItem 策略 | 单 session 单根 vs 多根 | **单 session 单根**（EnsureGoal 现有语义） | G5 与 focus 一致 |
| D6 Uncertainty | 双系统 vs 统一公式 | **统一公式** §2.3 | G2 |
| D7 ingress | 一步到位 vs 分 Phase | **Phase A→D** | 非目标：一次重写 |

---

## §7 与现有代码映射

| 组件 | 文件 | 变更类型 |
|------|------|----------|
| WorkItem 扩展 | `workmodel/workitem.go` | ADD RoundPhase, LastRound |
| SpawnPolicyEvaluator | `workmodel/spawn_policy.go` | NEW |
| RunItemPipeline | `sessionorchestrator/item_pipeline.go` | NEW（从 OrchestratePath 抽） |
| RunSessionTurnLoop | `sessionorchestrator/session_turn_loop.go` | NEW |
| ResolveHint | `workmodel/resolve_hint.go` | MODIFY 读 LastRound |
| ReevaluateParent | `workmodel/resolve.go` | MODIFY 读子 LastRound.Verdict |
| ProcessMessage | `sessionorchestrator/orchestrator.go` | MODIFY Phase C 路由 |
| OrchestratePath | `sessionorchestrator/orchestrate_path.go` | KEEP 工具路径 |
| Disk schema | `workitem_store.go` | v3  migration |

---

## §8 分 Phase 交付（每 Phase 绑定目标）

### Phase A — 契约 + 规则（不动 ingress）

**交付：** `WorkItemPipelineRound`, `SpawnPolicy`, `SpawnPolicyEvaluator`, 单测覆盖 R0–R8
**验收目标：** G2, G3（规则可测）
**风险：** 低

### Phase B — RunItemPipeline 单节点闭环

**交付：** 单个 WorkItem pending→completed 跑通五节点；LP-5 字段齐全
**验收目标：** G2（信号链），G4（WorkItem 写 LastRound）
**风险：** 中（OrchestratePath 拆分）

### Phase C — Turn Loop 接管 ingress

**交付：** ProcessMessage → RunSessionTurnLoop；递归 spawn 集成测试
**验收目标：** G1, G5
**风险：** 高（行为变更，需 feature flag）

### Phase D — 护栏闭合

**交付：** TD-WT-01/05/06；Escape budget；ParallelExplore ephemeral
**验收目标：** G3（人机门控），非目标（无限深度）
**风险：** 中

---

## §9 验收场景（Gherkin 摘要）

```gherkin
Scenario: 不确定问题递归探索直至收敛 (G1)
  Given 用户指令 "比较三种缓存方案并给出推荐"
  And 根 WorkItem uncertainty 高
  When RunSessionTurnLoop 执行
  Then 根节点 Verdict Partial 触发 SpawnDecompose
  And 创建 ≥2 子 WorkItem
  And 子节点均 RunItemPipeline
  And 父节点在子节点 Pass 后 Verdict Pass → completed

Scenario: spawn 必须有 LastRound 依据 (G2)
  Given 任意 DecomposeChildren 调用
  Then 父 WorkItem.LastRound.SpawnPolicy == decompose
  And LastRound 含 PlanID + VerdictID + ObservationIDs

Scenario: LLM 不可直接设 SpawnPolicy (G3)
  Given DecomposeProposer 返回任意 JSON
  When SpawnPolicyEvaluator 运行
  Then SpawnPolicy 仅由 R0–R8 决定

Scenario: Turn Loop 驱动 focus 切换 (G5)
  Given 3 个 pending 子 WorkItem
  When GetFocus 连续调用
  Then focus 按 kindFocusPriority + uncertainty tiebreak 切换
  And 直至 session 无 open work
```

---

## §10 开放问题（需 Review 拍板）

| OQ | 问题 | 建议默认 | 影响目标 |
|----|------|----------|----------|
| OQ-1 | Phase C WorkItem Pipeline ingress | **默认开启**（`FeatureWorkItemPipelineEnabled` 恒为 true） | — |
| OQ-2 | Partial 时 decompose 子节点数量上限 | **min(7, proposer N)**（`CapChildSpecs`） | G1 vs 成本 |
| OQ-3 | 子 WorkItem Kind 映射 | hypothesis→`explore`, 实施→`implement` | G4 |
| OQ-4 | Indeterminate 同节点最大重试 | **3**（`DefaultMaxIndeterminateRetries`） | G3 |

**Phase A 状态（2026-06-26）：** `SpawnPolicyEvaluator` + `WorkItemPipelineRound` + 单测 R0–R8 IMPLEMENTED in `workmodel/`.

**Phase B 状态（2026-06-26）：** `ItemPipelineRunner.Run` + `ApplyPipelineRound` + sessionorchestrator 单测 IMPLEMENTED.

**Phase C 状态（2026-06-26）：** `RunSessionTurnLoop` + `ApplySpawnPolicy` + `ProcessMessage` WorkItem Pipeline ingress **默认开启** + 递归 decompose 单测 IMPLEMENTED.

**Phase D 状态（2026-06-26）：** Bootstrap `WireItemPipeline` + `ItemToolRunner` 生产接线；TD-WT-01 `AdaptiveThreshold` 接入 `DefaultTreeEvalContext`；TD-WT-05 `SpawnEscalateHuman` → verify 子 WorkItem；TD-WT-06 父节点 re-eval 互斥；`RunParallelExplore` ephemeral 写回 `LastRound.SpawnRationale`。

**Phase E 状态（2026-06-26）：** `ProcessRequest.UserID` + baggage `user.id` 接入 per-user 阈值；`/task review approve` 人机门控闭环；`human_review` 事件 + Turn Loop 暂停；`SessionOrchestrator.TaskManager()` + `D7StackOptions.WorkItemPipeline` 集成测试脚手架。

**后续（ContextGraph）：** 见 `workitem-context-graph-design.md` v0.1.1（Review 通过；F1 契约已落地，F2–F6 待实现）。

---

## §11 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 0.1.0 | 2026-06-26 | 初稿：G1–G5 根本目标 + 两维状态机 + SpawnPolicy + Phase A–D |
| 0.2.0 | 2026-06-26 | OQ-1..4 决议；Phase A 落地（spawn_policy.go + pipeline_round.go + 单测） |
| 0.3.0 | 2026-06-26 | Phase B：`ItemPipelineRunner` per-WorkItem 五节点闭环 + LP-5 单测 |
| 0.4.0 | 2026-06-26 | Phase C：`RunSessionTurnLoop` + spawn apply + ingress feature flag |
| 0.5.0 | 2026-06-26 | Phase D：bootstrap ToolRunner + TD-WT-01/05/06 + ParallelExplore ephemeral |
| 0.6.0 | 2026-06-26 | Phase E：UserID 阈值 + human review CLI/事件 + 集成测试脚手架 |
