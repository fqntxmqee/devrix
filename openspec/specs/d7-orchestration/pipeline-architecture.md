# D7 Orchestration — 5 节点管道调用链路（MUPS v4.3）

**文档类型:** 运行时序 + 调用链路（pipeline architecture & call-chain reference）
**Domain:** D7 Orchestration
**DSAFT Type:** 核心域
**Version:** 1.2.1
**Status:** Active
**Last Updated:** 2026-06-26 (主链路 RunSessionTurnLoop + WorkTree + MUPS 对齐 v2.6.0)
**架构入口:** `openspec/specs/d7-orchestration/spec.md`（DSAFT 规范 SoT）
**领域 SoT:** `openspec/specs/d7-orchestration/d7-domain.md`（North Star / Out of Scope）
**详细设计:** `openspec/specs/d7-orchestration/design.md`（六段式架构设计）
**理想态蓝图:** `openspec/specs/d7-orchestration/call-chain-v4.3.md`（端到端 ASCII 全图 + 不变量验证清单）

> **v1.2.0 更新（2026-06-26，6 S 博弈角色对齐 v6.0.0）**：
> - **6 S 精简**：14 S → **6 S + 1 横切**（DM-20260626-001 / devrix-d7-six-s-simplification PR #215）。
> - **D7-S1 WorkModel**：D7-S1 (WorkModel) + D7-S14 (EscapeEngine 入口) + D7-S12 (Observe-Learner 闭环) + D7-S13 (AutoClose) 归并 S2 (State Authority→Mediator+Turn Leader+Error Recovery)
> - **D7-S2 SessionOrchestrator**：含 Mediator + Turn Leader + Error Recovery 三角色；D7-S5 (DecisionPlanning) + D7-S8 (Observe) 归 S5 (Information Producer + Quantizer)
> - **D7-S3 WaveScheduler**：不变（Mechanism Designer）
> - **D7-S4 ExecutionFlow + Verify**：D7-S4 + D7-S10 (Verify) 归 S4 (Costly Signaler + Certifier)
> - **D7-S5 DecisionPlanning + Observe**：D7-S5 + D7-S8 归 S5
> - **D7-S6 MUPS Pipeline**：D7-S9 (Execute) + D7-S11 (Learn) 归 S6 (Pipeline Coordinator + Memory Curator)
> - **Cross-cutting Hardening**：D7-S6 (Error Agg & Metrics) + D7-S7 (Hardening 横切) 改为横切，**不占 S 位**
> - **MUPS 5 节点管道挂载**：Observe+Plan 归 S5，Execute+Learn 归 S6，Verify 归 S4，AutoClose+Resume+Escape入口 归 S2
> - **A 数量**：56 → **49**（S1:4 · S2:7 · S3:4 · S4:9 · S5:8 · S6:15 + Hardening:2）
>
> **v1.1.0 更新（2026-06-25，PR #214 squash-merged）**：
> - **D7-S1 重命名**：Task/Plan 数据模型 → **WorkItem** 数据模型（canonical 单一来源）
> - **§6.3 代码路径**与 `internal/layers/orchestration/` 物理结构 1:1 对齐
> - **§3 IntentCommand 分支**：`/task` CLI 已并入 `/worktree` workmodel 统一接口
> - **新增 §7 清理变更日志**：devrix-d7-dead-files-cleanup (DM-20260625-016) + DM-20260625-013..015 10 项 DM
> - **新增 §8 验证清单**：5/5 节点 entry func / 不可变 / 血缘 / 14 ExitReason / 4 VerdictKind 等强保证

**Change IDs:**
- devrix-d7-mups-v4-phase1-foundation (DM-20260620-001) — Phase 1 Foundation + 3 项不变式
- devrix-d7-mups-v4-phase2-observe-plan (DM-20260623-001) — Phase 2 PR-A1 + PR-RF (D7-S8)
- devrix-d7-mups-v4-phase2-plan (DM-20260623-001-PRB1) — Phase 2 PR-B1 (D7-S5)
- devrix-d7-mups-v4-phase3-execute (DM-20260625-001) — Phase 3 PR-C1 (D7-S9 Artifact 4 类)
- devrix-d7-mups-v4-phase3-channels (DM-20260625-001-PRC2) — Phase 3 PR-C2 (D7-S9 Channel 4 个)
- devrix-d7-mups-v4-phase4-verify-promotion (DM-20260623-002) — Phase 4 (D7-S10)
- devrix-d7-mups-v4-phase5-learn (DM-20260623-003) — Phase 5 (D7-S11)
- devrix-d7-mups-v4-phase6-observe-learner-wiring (DM-20260624-001) — Phase 6 (D7-S12 LP-1 wiring)
- devrix-d7-mups-v4-phase7-verify-auto-close (DM-20260625-001) — Phase 7 (D7-S13 Auto-Close)
- devrix-d7-dead-files-cleanup (DM-20260625-013..016) — 10 项 D7 清理 + 3 commits squash-merged PR #214

> **本文档定位：** D7 编排域运行时序与调用链路的**单一权威图谱**。spec.md 各 Scenario 章节是局部契约，本文件是端到端总图。MUPS v4 全部 7 个 Phase 完成后的全景视图。

---

## 概述

D7 编排域回答 **"做什么、按什么顺序做、谁来做、做得怎么样了、学到了什么"**。MUPS v4.3 落地后，D7 域形成 **5 节点管道**：

```
Observe → Plan → Execute → Verify → Learn
   ↑                                    │
   └────────── LP-1 闭环 (Bayesian) ─────┘
```

外加：
- **LP-2 隔离**：PendingAsset 只入 ScheduledMemory，避免解析失败污染主知识库
- **LP-5 反向追溯**：`Plan.SourceObservationIDs` → `Artifact.SourcePlanID` → `Verdict.SourceArtifactID` → `Asset.SourceSessionIDs`
- **D7-S13 Auto-Close**：channel 关闭时自动 synthesizeVerdict + 异步触发 Learn（运行时 LP-1）

本文档依次给出：
1. **§1 5 节点管道总览**（架构图 + 4 类对应表 + 3 项不变式）
2. **§2 S 场景关系图**（13 个 S 场景 + 横向 / 闭环）
3. **§3 调用链路 — 全局入口 D1→D7 路径**（ProcessMessage 入口 + 4 IntentKind 分流）
4. **§4 调用链路 — RunSessionTurnLoop + ItemPipelineRunner 时序**（MUPS 5 节点管道运行时序）
5. **§5 调用链路 — 5 节点管道闭环可视化**（LP-1/LP-2/LP-5 闭环）
6. **§6 Cross-references**

---

## §1 5 节点管道总览

### 1.1 架构图

```
                          ┌─────── LP-5 反向追溯 ───────┐
                          ↓                             │
   ┌───────────┐ UncertaintyReport  ┌───────┐ Plan  ┌──────────┐ Artifact  ┌──────────┐ Verdict  ┌────────┐
   │  Observe  │ ─────────────────→ │ Plan  │ ────→ │ Execute  │ ────────→ │  Verify  │ ───────→ │ Learn  │
   │  (D7-S8)  │   4 ObsKind        │(D7-S5)│       │ (D7-S9)  │ 4 Artifact│ (D7-S10) │ 4 Verdict│(D7-S11)│
   └───────────┘   × 2 Category     └───────┘       └──────────┘            └──────────┘           └────────┘
        ↑                                                                                          │
        │ AdaptivePrior 注入 (Beta(5,3) Developer / Beta(8,1) Operator)                            │
        │                                                                                          │
        └─────────────── LP-1 闭环 (BayesianUpdate → ReputationStore → next Inject) ───────────────┘
                                                     ↑
                                             D7-S13 Auto-Close
                                          (processAutoClose 在 channel
                                           关闭时异步触发 synthesizeVerdict
                                           → Learner.Learn → ReputationStore.Update)
```

### 1.2 4 类 Plan ↔ 4 类 Artifact ↔ 4 类 Verdict ↔ 5 类 LearningAsset 对应表

| PlanKind | Channel | ArtifactKind | SideEffect 派生 | Verdict 路由 | LearningClass | Memory 通道 |
|----------|---------|--------------|----------------|--------------|---------------|-------------|
| `CommitmentPlan` | `CommitChannel`（1 步同步） | `ArtifactStateChangeCert` | `SideEffectCommitted` | Pass→SOP / Fail→Conclusion | **LearningSOP (★5)** | SkillMemory |
| `ProtocolPlan` | `ProtocolChannel`（顺序多步 + rollback） | `ArtifactStateChangeCert` | `SideEffectCommitted` | Pass→Protocol / Fail→Conclusion | **LearningProtocol (★4)** | SkillMemory |
| `ScenarioPlan` | `ScenarioChannel`（并行探测 + 多数派投票） | `ArtifactProbeReport` | `SideEffectNone` | Pass→Knowledge / Partial→Knowledge / Fail→Conclusion | **LearningKnowledge (★3)** | SkillMemory |
| `ExplorationPlan` | `ExplorationChannel`（多 agent + 优先级排序） | `ArtifactExperimentData` | `SideEffectCommitted`（per PersistScope） | Pass→Knowledge / Fail→Conclusion | **LearningKnowledge (★3)** | SkillMemory |
| **任意 PlanKind** | 任意 Channel | 任意 ArtifactKind | 任意 | **VerdictIndeterminate**（`verifier_parse_failure`） | **LearningPending (⭐★1)** | **ScheduledMemory** (LP-2 隔离) |
| **任意** | **任意** | 任意 | 任意 | **VerdictFail** | **LearningConclusion (★2)** | **FeedbackMemory** |

### 1.3 3 项强制不变式（Phase 1 Foundation）

| ID | 名称 | 约束 | 失败模式 |
|----|------|------|----------|
| **PP-1** | 强度匹配 | `Plan.Strength ≤ min(Observations.Strength)` | `ErrPlanStrengthMismatch` |
| **PP-2** | 可证伪性 | `Plan.FailureCriteria` 必须可被 Artifact 验证（Verify 节点能判定 Pass/Fail） | `ErrPlanFailureCriteriaMissing` |
| **PP-3** | 爆炸半径 | `Plan.BlastRadius.PersistScope ∈ {Transient, Session, Permanent}`（空值 fail-fast） | `ErrPlanPersistScopeInvalid` (PLAN_PERSIST_8012) |

### 1.4 LP-1 / LP-2 / LP-5 闭环

| LP | 全称 | 入口 | 出口 | 关键字段 |
|----|------|------|------|----------|
| **LP-1** | Learning Pipeline 闭环（Bayesian 信誉累积） | `Learner.Learn(ctx, req)` → `ReputationStore.Update` | 下一轮 `Learner.Inject(ctx, sessionID, trackMode)` → `AdaptivePrior.PriorBeta` | `ReputationEvidence.{Alpha, Beta, VerifierFailureCount, TrackMode, WilsonLower, WilsonUpper}` |
| **LP-2** | 隔离（Pending 不污染主知识库） | Verdict.Indeterminate + `verifier_parse_failure` | `AssetBuilder.BuildPending` → `MVEState` → `ScheduledMemory` | `LearningAsset.Class=LearningPending` + `MVEState{TriggerAt, MaxRetries, LastRetryAt}` |
| **LP-5** | 反向追溯链 | `Plan.SourceObservationIDs` | `Asset.SourceSessionIDs` | 5 节点每个节点都保留上游引用；任意节点可回溯到 Observe 入口 |

---

## §2 S 场景关系图

D7 域共 **6 个 Canonical S + 1 横切（v6.0.0 博弈角色对齐精简）**。MUPS 5 节点管道（Observe/Plan/Execute/Verify/Learn）按角色挂载到 6 S；v5 EscapeEngine 入口归 S2；AutoClose + Resume + Cross-cutting Hardening 独立成段。

### 2.1 6 Canonical S + 1 横切清单（v6.0.0）

| S 场景 | 名称 | 职责 | 状态 | 博弈角色 |
|--------|------|------|------|----------|
| **D7-S1** | **WorkModel** | **WorkItem 事实与状态机**单一权威 + UncertaintyCoord/ReputationEvidence/AdaptivePrior 状态归属 | **IMPLEMENTED (v1.0+post-cleanup v1.1)** | **State Authority** |
| **D7-S2** | **SessionOrchestrator** | ProcessMessage 入口 + Turn 主循环 + LLM 调用权 + RunTurn resolve/decompose/await + ResumeSession 3 决策路由 + AutoClose 4 规则 + EscapeEngine 调度 | **IMPLEMENTED (v1.0)** | **Mediator + Turn Leader + Error Recovery** |
| **D7-S3** | **WaveScheduler** | TaskGraph DAG + 5-slot WorkerPool + ConflictGuard + ContextPolicy | **IMPLEMENTED (v1.1)** | **Mechanism Designer** |
| **D7-S4** | **ExecutionFlow + Verify** | Hub 双通道（WorkPlan + SessionQueue + IM）+ SpokeBridge + VerdictKind 4 态 + AggregateVerdicts + VerdictToExitReason + Evidence + SystemAnomaly + 14 ExitReason | **IMPLEMENTED (v1.0+v1.1+Phase 4)** | **Costly Signaler + Certifier** |
| **D7-S5** | **DecisionPlanning + Observe** | 4 PlanKind + Planner + MatchKind 4 规则 + Plan.Validate PP-1/2/3 + Observation 4 类 + UncertaintyReport + UncertaintyCoord + 3 Observer 子模块 | **IMPLEMENTED (Phase 2 PR-A1+PR-B1+PR-RF)** | **Information Producer + Quantizer** |
| **D7-S6** | **MUPS Pipeline** | 4 Channel + ChannelRouter + 4 ArtifactKind + 5 LearningClass + 3 通道记忆 + ReputationEvidence Bayesian | **IMPLEMENTED (Phase 3 PR-C1/PR-C2 + Phase 5 PR-E1..E5)** | **Pipeline Coordinator + Memory Curator** |
| **Cross-cutting** | **Hardening** | `errors.Join` 聚合 + InterruptMetrics + sandbox cleanup observability + 6 metric 字段 + CircuitBreaker 监控 + ErrorRecoveryPolicy | **IMPLEMENTED (DM-20260621-010 + DM-20260622-001 + DM-20260626-003)** | **Discipline Keeper**（非 S） |

### 2.2 历史对照（v1.1 13 S → v1.2 6 S + 1 横切）

| v1.1 S | v1.2 归并 | 说明 |
|--------|----------|------|
| D7-S1 Work Model | S1 WorkModel | State Authority |
| D7-S2 Session Orchestrator | S2 SessionOrchestrator | 吸收 D7-S12/S13/S14 入口 |
| D7-S3 Wave Scheduler | S3 WaveScheduler | Mechanism Designer |
| D7-S4 Execution Flow | S4 ExecutionFlow + Verify | 吸收 D7-S10 Verify 角色 |
| D7-S5 Decision & Planning | S5 DecisionPlanning + Observe | 吸收 D7-S8 Observe 角色 |
| D7-S6 Error Agg & Metrics | Cross-cutting Hardening | 横切，**不占 S 位** |
| D7-S8 Observation | S5 DecisionPlanning + Observe | 已并入 S5 |
| D7-S9 Execute | S6 MUPS Pipeline | Pipeline Coordinator |
| D7-S10 Verify | S4 ExecutionFlow + Verify | 已并入 S4 |
| D7-S11 Learn | S6 MUPS Pipeline | Memory Curator |
| D7-S12 Observe-Learner Wiring | S2 SessionOrchestrator | 入口归 S2 |
| D7-S13 Auto-Close | S2 SessionOrchestrator | 入口归 S2 |
| D7-S14 (v5) EscapeEngine | S2 SessionOrchestrator | 入口归 S2（Engine 物理独立） |

### 2.3 S 场景关系图（v6.0.0 6 S + 1 横切）

```
                              D7-S2 SessionOrchestrator (入口 / Turn Leader / Error Recovery)
                                           │
        ┌───────────────┬──────────────────┼──────────────────┬─────────────────┐
        ↓               ↓                  ↓                  ↓                 ↓
   4 IntentKind     ClassifyIntent     ProcessMessage     RunTurnLoop     DispatchWorker
  (Skip/Command/   (D7-S5)             (含 buildObserve  (D3+D2 调用)    (D7-S4 hub)
   Fast/Orches)                          Request 优先)
                                           │
  ┌────────────────────────────────────────┼─────────────────────────────────────────┐
  │            MUPS 5 节点管道 (IntentOrchestrate 路径, v6.0.0 挂载到 6 S)             │
  │                                                                                   │
  │   D7-S5 Observe ─→ D7-S5 Plan ─→ D7-S6 Execute ─→ D7-S4 Verify ─→ D7-S6 Learn  │
  │       ↑            (Info Prod)      (Pipe Coord)    (Certifier)     (Memory)     │
  │       │                                                                       │
  │       │   S2 闭环 wiring (buildObserveRequest 3 层 fail-safe)                   │
  │       └─────────── LP-1 闭环 (AdaptivePrior 注入) ──────────────────────────────┘
  │                                  ↑
  │                       S2 Auto-Close (processAutoClose + synthesizeVerdict)
  │                         channel 关闭时异步触发 Learn
  │                                  ↑
  │                       S2 v5 EscapeEngine (5 层 CircuitBreaker L0..L5)
  │                       S2 ResumeSession (3 决策路由: fall through / ForceExit / AbortWithAudit)
  │
  ├── 基础：D7-S1 WorkModel (**WorkItem 持久化** + 状态机) ── State Authority
  ├── 执行：D7-S3 WaveScheduler (DAG + WorkerPool + ConflictGuard) ─→ 喂给 S6
  ├── 事件：D7-S4 ExecutionFlow (Hub 双通道 + SpokeBridge) ─→ IM 广播 + S4 Verify
  └── 横切：Cross-cutting Hardening (errors.Join + 6 metric + CircuitBreaker monitor)
```

### 2.4 关键关系要点（v6.0.0）

1. **D7-S2 是唯一入口** — 4 IntentKind 决定走哪条路径：
   - `IntentSkip` → close channel（不触发 Auto-Close）
   - `IntentCommand` → CommandHandler（/plan, /worktree, /help, /stop）
   - `IntentFast` | `IntentOrchestrate` → **RunSessionTurnLoop**（WorkTree + 逐 WorkItem MUPS）
2. **5 节点管道挂载 6 S（v6.0.0）**：Observe+Plan 归 S5、Execute+Learn 归 S6、Verify 归 S4；按 LP-5 反向追溯链串联
3. **S2 是闭环 + 错误恢复单点**：含 buildObserveRequest（wiring）+ AutoClose（runtime）+ EscapeEngine + ResumeSession
4. **D7-S1/S3/S4 是 v2.0 已有基础设施**，MUPS 期间未改，被新节点复用
5. **Cross-cutting Hardening 横切所有 S 场景**（错误聚合 + metric + CB 监控），不占 S 位

---

## §3 调用链路 — 全局入口 D1→D7 路径

```
D1 Gateway.RouteInbound(msg)
    │
    ↓
D7-S2 SessionOrchestrator.ProcessMessage(ctx, req)
    │  ← 生产 wiring: bootstrap/wire_coordinator.go::WireD7
    │
    ├─[1] buildObserveRequest(ctx, req)                    ← D7-S12 + D7-S13
    │     │  3 层 fail-safe:
    │     │    L1 nil learner        → DefaultDeveloperPrior Beta(5,3)
    │     │    L2 Inject error       → DefaultDeveloperPrior Beta(5,3)
    │     │    L3 正常                → Learner.Inject 拉回 ReputationStore 中的 α/β
    │     │  + TrackMode 3-tier 解析:
    │     │    req.TrackMode="operator"  → DefaultOperatorPrior Beta(8,1) Mean=0.889
    │     │    req.TrackMode="developer" → DefaultDeveloperPrior Beta(5,3) Mean=0.625
    │     │    req.TrackMode="" / 未知    → 兜底 Developer + slog.Warn
    │     ↓
    │   ObserveRequest{SessionID, Message, Prior: AdaptivePrior, TrackMode}
    │     │
    │     ↓
    │   Orchtypes.QuantizeWithPrior / DetectWithPrior / ClassifyWithPrior
    │     ↓
    │   UncertaintyReport{
    │     Observations: [4 ObsKind × 2 Category],
    │     Overall: UncertaintyCoord{Alpha, Beta, Mean, VerifierVerdict},
    │     Anomalies: [Anomaly{Type, Severity, Evidence}],
    │     QuantizedIntent: IntentPayload
    │   }
    │
    ├─[2] ClassifyIntent(report)                            ← D7-S5
    │     │
    │     ├─ RuleClassifier.ClassifyWithPrior(rule, prior)  ← 规则优先
    │     │   → IntentKind ∈ {IntentSkip, IntentCommand, IntentFast, IntentOrchestrate}
    │     │
    │     └─ ShadowClassifier.ClassifyWithPrior(fallback)   ← LLM tail-only
    │
    └─[3] switch intent.Kind
          │
          ├─ IntentSkip       → close channel (no Learn)
          │                     ↑ D7-S13 processAutoClose 不触发
          │
          ├─ IntentCommand    → CommandHandler.Handle
          │                     ├─ /plan → PlanCLICommands → PlanMode (D7-S5 PlanAgent 只读探索)
          │                     ├─ /worktree → CLICommands → TaskManager.Tree() (D7-S1)
          │                     └─ /help, /stop → explicit handlers
          │
          ├─ IntentFast       ─┐
          │                     ├─ RunSessionTurnLoop  ← 主链路（WorkTree + MUPS）
          └─ IntentOrchestrate ─┘
                                │
                                ↓
                            §4 RunSessionTurnLoop 时序
```

> **v2.6.0 主链路（DM-20260629-001）**：`FastPath` / `OrchestratePath` 已退役。
> `IntentFast` 与 `IntentOrchestrate` 仅为分类标签，二者均走
> `RunSessionTurnLoop` → `GetPipelineFocus` → `ItemPipelineRunner.Run`（MUPS）。
> `DefaultOrchestrator.RunTurn` 仅用于 sub-agent / PreparedTurn，不是飞书用户消息主路径。

---

## §4 调用链路 — RunSessionTurnLoop + ItemPipelineRunner 时序

```
RunSessionTurnLoop(ctx, req, intent)
    │  ← bootstrap: WithItemPipelineRunner + WithTaskManager
    │
    ├─[0] EnsureGoal(sessionID, directive)     ← WorkTree 种子（ProcessMessage 已做则跳过）
    │
    └─ loop (max 16 rounds):
          GetPipelineFocus(sessionID)           ← WorkTree 选焦点 WorkItem
          │
          ItemPipelineRunner.Run(ctx, sessionID, focus, userID)  ← MUPS 主入口
            │
            ├─[A] Observe 阶段 (D7-S8)
            │     buildObserveRequest / UncertaintyReport
            │
            ├─[B] Plan 阶段 (D7-S5)
            │     Planner.Plan(ctx, report) → Plan{Kind, Steps, FailureCriteria, ...}
            │
            ├─[C] Execute 阶段 (D7-S9)
            │     WorkItemExecutor.ExecuteWorkItem (per-WorkItem ReAct)
            │       → D3 LLM stream + D2 Prepare/ToolRound/Persist
            │
            ├─[D] Verify 阶段 (D7-S10)
            │     artifact → Verdict (Pass/Fail/Indeterminate)
            │
            └─[E] Learn 阶段 (D7-S11)
                  Learner.Learn → ReputationStore (LP-1)
            │
            Decide spawn/rollup → WorkTree 状态迁移 → 下一 focus 或 break
```

> 下列为 **ItemPipelineRunner.Run 内部** 各阶段细节（原 §4 OrchestratePath 6 步时序，语义不变，入口已迁移）：

```
ItemPipelineRunner.Run(ctx, sessionID, item, userID)   [legacy alias: OrchestratePath.Run — 已删]
    │
    ├─[A] Observe 阶段 (D7-S8)  [Phase 2 PR-A1]
    │     UncertaintyReport ─→ 喂给 Planner
    │     (本步在 buildObserveRequest 已完成，此处为消费)
    │
    ├─[B] Plan 阶段 (D7-S5)
    │     Planner.Plan(ctx, report)
    │       │
    │       ├─ MatchKind(report) ─→ 4 规则分类
    │       │   - 全部 ObsFact + Strength≥0.9          → CommitmentPlan
    │       │   - 含 ObsSignal + 需多步副作用            → ProtocolPlan
    │       │   - 含 ObsDeviation + 需试探               → ScenarioPlan
    │       │   - 含 ObsUncertainty + 需探索              → ExplorationPlan
    │       │
    │       ├─ PP-1 校验: StrengthMatch(plan, report)  → plan.Strength ≤ min(obs.Strength)
    │       ├─ PP-2 校验: FailureCriteria 必填          → 可被 Artifact 验证
    │       ├─ PP-3 校验: BlastRadius{PersistScope}    → Transient/Session/Permanent
    │       │
    │       ↓
    │     Plan{
    │       ID: "plan_<uuid[:8]>_<sha256[:8]>",
    │       Kind: PlanKind,
    │       Strength: float64,
    │       Steps: [Step{ToolName, ToolArgs, IdempotencyKey, EstimatedTokens}],
    │       FailureCriteria: VerifierChecklist,
    │       BlastRadius: BlastRadius{PersistScope, ...},
    │       SourceObservationIDs: [obsID...]   ← LP-5 血缘起点
    │     }
    │
    ├─[C] Execute 阶段 (D7-S9)  [Phase 3 PR-C1 + PR-C2]
    │     ChannelRouter.Dispatch(plan, req)
    │       │
    │       ├─ PlanKind=CommitmentPlan → CommitChannel
    │       │    └─ 1 步同步 + IdempotencyKey 强制 + timeout=5s
    │       │      SideEffect 派生:
    │       │        - exitCode == 0       → SideEffectCommitted
    │       │        - ctx.DeadlineExceeded → SideEffectInflight（PR-C3 → StrategyAskNow）
    │       │        - 其他错误             → SideEffectUnknown
    │       │
    │       ├─ PlanKind=ProtocolPlan → ProtocolChannel
    │       │    └─ 顺序多步 + reverse-order rollback
    │       │      (context.Background + timeout; first non-nil error wins)
    │       │
    │       ├─ PlanKind=ScenarioPlan → ScenarioChannel
    │       │    └─ 并行探测 + 多数派投票 + PersistScope 派生 SideEffectStatus
    │       │
    │       └─ PlanKind=ExplorationPlan → ExplorationChannel
    │            └─ 多 agent 并行 + sync.WaitGroup
    │              排序：(success 优先) → (duration 升序) → (EstimatedTokens 升序)
    │              + mostInformativeError() 报错（最长 error 优先）
    │
    │     ↓
    │   Artifact{
    │     ID: taskID,
    │     Kind ∈ {ArtifactStateChangeCert, ArtifactResponseRecord,
    │             ArtifactProbeReport, ArtifactExperimentData},
    │     SideEffectStatus ∈ {SideEffectNone, SideEffectUnknown, SideEffectInflight,
    │                         SideEffectCommitted, SideEffectRolledBack},
    │     SourcePlanID,                          ← LP-5 中段
    │     AnomaliesCount, Summary, Error, Evidence, ...
    │   }
    │
    ├─[D] Verify 阶段 (D7-S10)  [Phase 4]
    │     Verifier.VerifyWithRetry(ctx, plan, artifact)  ← 3 次兜底
    │       │
    │       ├─ EvidenceExtractor.Extract(artifact) → Evidence{5 fields: Source, Raw, Kind, Confidence, Hash}
    │       ├─ LLM judge: Pass / Partial / Fail / Indeterminate
    │       │   - parse failure → VerdictIndeterminate + IndeterminateReason="verifier_parse_failure"
    │       │     ⚠️ G8-1 修复：不污染 Bayesian α/β（仅 VerifierFailureCount++）
    │       │
    │       ├─ SystemAnomalyAggregator.RecordCatSystem(anomalies) → 阈值触发覆盖
    │       │   (CatSystem 异常连续 ≥ 阈值时强制 VerdictFail)
    │       │
    │       ↓
    │     Verdict{
    │       ID, Kind ∈ {Pass, Partial, Fail, Indeterminate},
    │       SourceArtifactID,                  ← LP-5 后段
    │       Evidence, IndeterminateReason, ...
    │     }
    │       ↓
    │     VerdictToExitReason(verdict) → ExitReason ∈ 14 枚举
    │       - Pass            → "natural"
    │       - Partial         → "natural_with_caveat"
    │       - Fail            → "unresolved"
    │       - Indeterminate   → "verifier_abstain" (G8-1) 或 "abstain" (其他)
    │
    ├─[E] Learn 阶段 (D7-S11)  [Phase 5]
    │     Learner.Learn(ctx, req)  ← 同步调用
    │       │
    │       ├─ LP-3 顺序：先 ReputationStore.Update (commit Bayesian) → 再 Memory.Store
    │       │
    │       ├─ BayesianUpdate(verdict, prior, trackMode):
    │       │   - Pass              → α++
    │       │   - Partial           → α += 0.5
    │       │   - Fail              → β++
    │       │   - Indeterminate + "verifier_parse_failure" → VerifierFailureCount++ only
    │       │   - Wilson Score 95% CI 计算 (z=1.96, z²=3.8416, 4n² 项完整)
    │       │
    │       ├─ 4 Verdict → 5 LearningClass 路由
    │       │   - Pass + CommitmentPlan  → LearningSOP (★5)         → SkillMemory
    │       │   - Pass + ProtocolPlan    → LearningProtocol (★4)    → SkillMemory
    │       │   - Pass + Scenario/Exploration → LearningKnowledge (★3) → SkillMemory
    │       │   - Fail (任意)            → LearningConclusion (★2)  → FeedbackMemory
    │       │   - Indeterminate (任意)   → LearningPending (⭐★1)    → ScheduledMemory (LP-2 隔离)
    │       │
    │       ├─ AssetBuilder.Build(class, content) → LearningAsset
    │       │   + MVEState (Pending 专属): TriggerAt, MaxRetries=3, LastRetryAt
    │       │   + SourceSessionIDs (LP-5 终点)
    │       │
    │       └─ Memory.Store(asset)
    │           + ScheduledMemory.MaxRetries=3 + TriggerAt 兜底
    │
    └─[F] Auto-Close (D7-S13)  [Phase 7]  ⚠️ 异步触发，不阻塞 caller
          │
          ↓
        processAutoClose(ctx, sessionID, ch, learner)
          │ 包装 channel，在 channel 关闭后:
          │
          ├─ synthesizeVerdict(lastEvent):  ← 4 规则
          │   - Type=complete         → VerdictPass (ExitReason="natural")
          │   - Type=error            → VerdictFail (Reason=Content, ExitReason="unresolved")
          │   - Type=tombstone        → VerdictIndeterminate (IndeterminateReason="interrupt")
          │   - 其他 Type              → nil (跳过)
          │   - SourceID 格式: `autoclose:{sessionID}:{nanosecond}`
          │
          ├─ 3 层 fail-safe:
          │   L1 nil learner         → log + skip
          │   L2 Learn error         → log + skip
          │   L3 channel cancel      → log + skip
          │
          └─ go func() {
                ctx, cancel := context.WithTimeout(context.Background(), 10s)
                defer cancel()
                learner.Learn(ctx, req)  ← 异步写入 ReputationStore
              }()

          ↓
        endSpanWithOnce(span, sync.Once 保护)  ← 防 race / double-close
```

---

## §5 调用链路 — 5 节点管道闭环可视化

```
   ┌─────────────────── D7-S8 Observe ─────────────────────┐
   │  UncertaintyReport{                                   │
   │    Observations: [4 ObsKind × 2 Category],            │
   │    Overall: UncertaintyCoord,                         │
   │    Anomalies: [Anomaly{...}],                         │
   │    QuantizedIntent: IntentPayload                     │
   │  }                                                    │
   │  Prior: AdaptivePrior{                                │
   │    Alpha, Beta, Mean, TrackMode,                      │
   │    ClassifierSource, InjectedAt                       │
   │  }                                                    │
   └─────────────────────────┬─────────────────────────────┘
                             ↓
   ┌─────────────────── D7-S5 Plan ────────────────────────┐
   │  Plan{                                                │
   │    ID, Kind, Strength,                                │
   │    Steps: [Step{ToolName, IdempotencyKey}],           │
   │    FailureCriteria, BlastRadius,                      │
   │    SourceObservationIDs   ← LP-5 血缘起点             │
   │  }                                                    │
   └─────────────────────────┬─────────────────────────────┘
                             ↓
   ┌─────────────────── D7-S9 Execute ─────────────────────┐
   │  Artifact{                                            │
   │    ID, Kind ∈ {StateChangeCert, ResponseRecord,       │
   │               ProbeReport, ExperimentData},           │
   │    SideEffectStatus ∈ {None, Unknown, Inflight,       │
   │                       Committed, RolledBack},         │
   │    Evidence,                                          │
   │    SourcePlanID   ← LP-5 中段                         │
   │  }                                                    │
   └─────────────────────────┬─────────────────────────────┘
                             ↓
   ┌─────────────────── D7-S10 Verify ─────────────────────┐
   │  Verdict{                                             │
   │    ID, Kind ∈ {Pass, Partial, Fail, Indeterminate},  │
   │    SourceArtifactID  ← LP-5 后段                     │
   │    Evidence, IndeterminateReason, ...                 │
   │  }                                                    │
   │  ExitReason ∈ 14 枚举                                 │
   └─────────────────────────┬─────────────────────────────┘
                             ↓
   ┌─────────────────── D7-S11 Learn ──────────────────────┐
   │  ReputationStore.Update(α/β++ via BayesianUpdate)    │
   │  Memory.Store(LearningAsset → 3 通道之一)             │
   │  + ScheduledTick (PendingAsset 重试 MaxRetries=3)     │
   └─────────────────────────┬─────────────────────────────┘
                             ↓
                  D7-S12 buildObserveRequest
                  (Learner.Inject 拉回 α/β)
                  (3 层 fail-safe + TrackMode 3-tier 解析)
                             ↓
   ┌─────────────────── D7-S8 Observe (下轮) ──────────────┐
   │  Prior 已含上一轮 Bayesian 累积                       │
   │  ↑ LP-1 闭环成立                                     │
   │                                                      │
   │  sessionSpan 6 prior attributes:                     │
   │    learn.prior.alpha / beta / mean                   │
   │    learn.prior.track_mode / classifier_source        │
   │    learn.prior.injected_at                           │
   │      = "phase6_lp1" (真实注入)                       │
   │      = "cold_start_failsafe" (兜底)                  │
   └──────────────────────────────────────────────────────┘

   ╔══════════════════ D7-S13 Auto-Close (runtime LP-1) ══════════════════╗
   ║                                                                       ║
   ║   channel 关闭 (IntentSkip / 早退 / 中断)                             ║
   ║      ↓                                                                ║
   ║   processAutoClose(sessionID, ch, learner)                            ║
   ║      ↓                                                                ║
   ║   synthesizeVerdict(lastEvent) ← 4 规则                              ║
   ║      ↓                                                                ║
   ║   go func() { learner.Learn(ctx, req) } ← 异步 + 10s timeout         ║
   ║      ↓                                                                ║
   ║   写入 ReputationStore → 下轮 Observe 注入                           ║
   ║                                                                       ║
   ║   3 层 fail-safe:                                                     ║
   ║     L1 nil learner     → log + skip                                   ║
   ║     L2 Learn error     → log + skip                                   ║
   ║     L3 channel cancel  → log + skip                                   ║
   ║                                                                       ║
   ║   ⚠️ IntentSkip 路径不调用 processAutoClose                            ║
   ╚═══════════════════════════════════════════════════════════════════════╝
```

### 5.1 LP-1 闭环示意（贝叶斯信誉累积）

```
Round 1 (Cold Start):
  ReputationStore.Get("sess_1") → nil
  buildObserveRequest → DefaultDeveloperPrior Beta(5,3) Mean=0.625
  ↓ Observe → Plan → Execute → Verify → Learn
  Verdict = Pass → BayesianUpdate → α=6, β=3
  ReputationStore.Alpha=1, Beta=0 (after LP-3 write)

Round 2 (有先验):
  ReputationStore.Get("sess_1") → ReputationEvidence{Alpha=1, Beta=0}
  buildObserveRequest → BuildAdaptivePrior(rep, "developer")
                       = Beta(5+1, 3+0) = Beta(6,3) Mean=0.667
  ↓ Observe → Plan → Execute → Verify → Learn
  Verdict = Pass → α=7, β=3

Round 3 (累积):
  Beta(7,3) Mean=0.700
  ...

⭐ G8-1 修复 (verifier_parse_failure):
  Indeterminate + "verifier_parse_failure" → VerifierFailureCount++ only
  α/β 保持不变，下一轮仍按真实先验继续累积
  （避免解析失败污染 Bayesian 信誉）
```

### 5.2 LP-2 隔离示意

```
Verifier 返回 parse failure
  ↓
Verdict{Indeterminate, IndeterminateReason="verifier_parse_failure"}
  ↓
BayesianUpdate: VerifierFailureCount++, α/β 不变
  ↓
LearningClass 路由 → LearningPending (⭐★1)
  ↓
AssetBuilder.BuildPending(content) → MVEState{TriggerAt, MaxRetries=3}
  ↓
Memory.Store(asset) → ScheduledMemory ONLY
  (SkillMemory / FeedbackMemory 收不到，不污染主知识库)
  ↓
ScheduledMemory.ListDue(now) → 深拷贝 ScheduledRetry envelope
ScheduledMemory.Delete + Re-Store (避免共享指针)
  ↓
PendingAsset 最多重试 3 次，超出后降级为 ConclusionAsset
```

### 5.3 LP-5 反向追溯链

```
Asset.SourceSessionIDs = ["sess_1"]
  ↑ AssetBuilder 构建时填充
LearningAsset
  ↑ Verdict.SourceArtifactID 决定 class + content
Verdict
  ↑ Artifact.SourcePlanID 决定 FailureCriteria 验证
Artifact
  ↑ Plan.SourceObservationIDs 决定 Observation 列表
Plan
  ↑ UncertaintyReport.Observations 决定 PlanKind
UncertaintyReport
  ↑ buildObserveRequest 用 sessionID 关联 ProcessRequest
ProcessRequest

→ 任意节点可向上回溯到 Observe 入口
→ Jaeger UI 通过 sessionSpan.traceID 自然支持跨节点追踪
```

---

## §6 Cross-references

### 6.1 同目录引用

- `d7-domain.md` — 领域 SoT（North Star / Out of Scope / 文档索引）
- `spec.md` — DSAFT 规范 SoT（Scenarios / Requirements）— **建议在 spec.md 第 113-144 行的顶层 flow 区块加链接指向本文档**
- `design.md` — 六段式详细架构设计
- `terminal-state-guide.md` — 终态流程指南（IntentKind 四链 / A→F 编排树 / 跨域时序）
- `observability-guide.md` — 可观测性指南（Span↔T、Trace 树、P0 Runbook）
- `span-registry.md` — Span 注册表（含本文档提到的 sessionSpan 6 prior attributes）
- `t-registry.md` — T 层测试点注册表（180 IMPLEMENTED / 147 P0）

### 6.2 跨 Change 引用

- **Phase 1 Foundation** — `openspec/archive/2026-06-20-devrix-context-budget-and-isolation-phase-b/` + `2026-06-20-devrix-context-budget-phase-c-nested/`
- **Phase 2 PR-A1+PR-RF** — `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase2-observe-plan/` (D7-S8 Observe)
- **Phase 2 PR-B1** — `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase2-plan/` (D7-S5 Plan)
- **Phase 3 PR-C1** — `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase3-execute/` (D7-S9 Artifact)
- **Phase 3 PR-C2** — `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase3-channels/` (D7-S9 Channel)
- **Phase 4 Verify** — `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase4-verify-promotion/` (D7-S10)
- **Phase 5 Learn** — `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase5-learn/` (D7-S11)
- **Phase 6 Observe-Learner Wiring** — `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase6-observe-learner-wiring/` (D7-S12)
- **Phase 7 Auto-Close** — `openspec/archive/2026-06-25-devrix-d7-mups-v4-phase7-verify-auto-close/` (D7-S13)

### 6.3 代码实现位置（v4.3 post-cleanup, PR #214）

| 节点 | 代码位置 |
|------|---------|
| D7-S8 Observe | `internal/layers/orchestration/orchtypes/{observation,uncertainty_report,uncertainty_coord,observe_request,intent_quantizer,anomaly_detector,system_anomaly_wiring}.go` |
| D7-S5 Plan | `internal/layers/orchestration/plan/{plan,plan_struct,planner,blast_radius,errors}.go` + `internal/layers/orchestration/workmodel/{plan_agent,plan_mode}.go`（PlanAgent 仅 /plan 命令入口） |
| D7-S9 Execute | `internal/layers/orchestration/mups/execute/{channel,channel_commit,channel_protocol,channel_scenario,channel_exploration,errors}.go` + `internal/shared/types/execute.go` + `internal/layers/orchestration/orchtypes/artifact_kind_alias.go` |
| D7-S10 Verify | `internal/shared/types/verdict.go` + `internal/layers/orchestration/workmodel/{aggregate_verdicts,verify_with_retry,evidence,evidence_extractor,system_anomaly}.go` + `internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason.go` |
| D7-S11 Learn | `internal/layers/orchestration/mups/learn/{learning_asset,asset_content,asset_builder,reputation_evidence,adaptive_prior,memory,reputation_store,learner}.go` + `internal/shared/types/learning.go` |
| D7-S12 wiring | `internal/layers/orchestration/sessionorchestrator/orchestrator.go::buildObserveRequest` + `internal/layers/orchestration/orchtypes/{observe_request,errors}.go` |
| D7-S13 Auto-Close | `internal/layers/orchestration/sessionorchestrator/{autoclose,tracing,orchestrator}.go` + `internal/layers/orchestration/mups/learn/{learner,asset_builder}.go` + `internal/layers/orchestration/orchtypes/process.go` |
| Turn Loop | `internal/layers/orchestration/sessionorchestrator/{orchestrator,exit_reason,verdict_to_exit_reason,recovery,subturn,tool_stream,compression_summarizer,llm,focus_hint,tracing,contracts}.go` |
| WorkTree | `internal/layers/orchestration/workmodel/{work_tree,workitem,workitem_store,task_manager,task_manager_metrics,run_spawn,plan_agent,plan_mode,tool_suite,cli_commands}.go` |
| Escape | `internal/layers/orchestration/escape/` + `internal/layers/orchestration/sessionorchestrator/escape_wiring.go` |
| 生产 wiring | `internal/bootstrap/wire_coordinator.go::WireD7` + `internal/bootstrap/wire_wave.go` |

**清理变更（v4.3 post-cleanup, PR #214 squash-merged 2026-06-24）**：

| 删除项 | 替代方案 | 备注 |
|--------|---------|------|
| `workmodel/task_store.go` | `workmodel/workitem_store.go`（DiskWorkItemStore 直接） | Task flat-view 整层删除 |
| `workmodel/workitem.go` 内 `ToTask/FromTask` conversion | 无（pure WorkItem 模型） | conversion helper 0 残留 |
| `workitem_store.taskStoreAdapter` | `WorkItemStore` interface 直接实现 | adapter 模式去除 |
| `orchtypes/uncertainty_coord.go::FromVerifier` 字符串版 | `FromVerifierTyped` typed enum | shim 去除 |
| `coordinator/` `hubspoke/` type-alias shim | 已并入 `sessionorchestrator/` `orchtypes/` | 历史过渡层去除 |
| `turn/orchestrator.go` 内 ExitReason 枚举（70 行） | `turn/exit_reason.go` 89 行独立 | 文件按职责拆分 |
| `turn/loop_legacy_test.go` | `turn/runturn_main_path_test.go` | 测试名去"legacy"误导 |

### 6.4 E2E 集成测试

- `tests/integration/d7/learn_observe_closure_test.go` — LP-1 闭环 4 scenarios（Phase 6 E2E）
  - `TestIntegration_LearnPass_AlphaAccumulates`
  - `TestIntegration_IndeterminateParseFailure_NoBayesianPollution`（G8-1 闭环）
  - `TestIntegration_PendingAsset_ScheduledMemoryOnly`（LP-2 隔离）
  - `TestIntegration_FiveNodePipeline_End2End`
- `tests/integration/d7/d7_wave_real_test.go` — WaveScheduler 3 任务 DAG 真实派发
- `tests/integration/d7/d7_orthogonal_dispatch_test.go` — 4 IntentKind 正交分发
- `tests/integration/d7/d7_loop_first_test.go` — LoopFirst 入口
- `tests/integration/d7/d7_llm_decomposer_test.go` — LLM Decomposer

### 6.5 外部引用（域外）

- **D1 Communication** — 拥有 ingress（`ProcessMessage`），接收 D7 的 EngineEvent / Flow 展示
- **D2 Context Engine** — D7 RunTurn / Prepare 编排，瘦身后仅保留 Follower 拆面
- **D3 LLM Gateway** — D7 直调（`IGateway`），用于 IntentClassifier / Verifier / LLM Decomposer
- **D4 Multi-Agent** — Delegate RunAgent 由 D7 编排；Agent 生命周期归 D4
- **D5 Observability** — Span / Trace / Metric 接收端；D7-S6 横切 + D7-S13 sessionSpan 6 prior attributes
- **D6 Evolution** — `ValidateOrchestration` advisory 入口

---

## §7 清理变更日志（v4.3 post-cleanup, PR #214 squash-merged 2026-06-24T22:37:02Z）

**DM 序列**: devrix-d7-dead-files-cleanup (DM-20260625-013..016) + 10 项过渡清理 DM

### 7.1 3 commits squash-merged

| Commit | DM | 摘要 | 行数 |
|--------|------|------|------|
| `5c09aef` | DM-20260625-014 | typed-enum FromVerifierTyped + ordinal ClassToStrength + v1.0 history cleanup | +X/-Y |
| `3f29b5a` | DM-20260625-015 | **delete Task flat-view + TaskStore**, collapse to WorkTree (19 files) | +X/-Y |
| `7d32f4b` | DM-20260625-016 | split ExitReason enum (`turn/exit_reason.go`) + rename `loop_legacy_test` | +X/-Y |

**净变化**: 24 files, +498/-751 = **-253 行**

### 7.2 后续 10 项过渡清理 DM（已分别 PR 合入 master）

| DM | 主题 | 影响范围 |
|------|------|----------|
| DM-20260625-007 | drop 3 orphan code paths from pre-MUPS-v4 era | chore(d7) |
| DM-20260625-008 | drop milestone service + TaskController | chore(d6/d7) |
| DM-20260625-009 | drop dead `turn_adapter` + `unified_tools` | chore(d7) |
| DM-20260625-010 | migrate `coordinator.*` + `hubspoke.*` to source pkgs | refactor(d7) |
| DM-20260625-011 | retire `hubspoke/` + drop 4 dead code paths | refactor(d7) |
| DM-20260625-012 | split `runLoop` god function into 7 helpers (D3) | refactor(d7) |
| DM-20260625-013 | DI-migrate process singletons + drop dead code | refactor(d7) |

### 7.3 关键不变量

清理后所有不变式依然成立：

- **WorkItem 单一来源**：TaskManager 只是 facade，`Tree()` 是唯一访问路径
- **5/5 节点不可变契约**：Verdict/Plan/Artifact/LearningAsset 全部 `With*` 副本
- **LP-5 血缘回溯链**：Plan→Obs, Artifact→Plan, Verdict→Artifact, Asset→Session 完整
- **类型强一致**：14 ExitReason + 4 VerdictKind + 5 LearningClass + 4 Channel 全 typed enum
- **跨域低耦合**：`shared/types` 上提打破潜在 import cycle

---

## §8 验证清单（v4.3 post-cleanup 强保证）

| 项 | 状态 | 证据 |
|----|------|------|
| 5/5 节点唯一 entry func | ✅ | Observe=buildObserveRequest, Plan=DefaultPlanner.Plan, Execute=ChannelRouter.Route, Verify=VerifyWithRetry, Learn=DefaultLearner.Learn |
| 跨节点值对象不可变 | ✅ | `With*` 系列 (Verdict.WithKind/WithSourceID, Plan.With*, LearningAsset.WithSourceVerdictIDs/WithTraceID) |
| 血缘回溯 5 节点全覆盖 | ✅ | SourceObservationIDs / SourcePlanID / SourceArtifactID / SourceVerdictIDs / SourceSessionIDs |
| 14 ExitReason enum 独立 | ✅ | `turn/exit_reason.go` 89 行（从 orchestrator.go 抽出） |
| 4 VerdictKind typed enum | ✅ | `shared/types/verdict.go` + String/Parse/Marshal/Unmarshal |
| 5 LearningClass typed enum | ✅ | `shared/types/learning.go` + ordinal ClassToStrength 公式 |
| 4 Channel 1:1 绑定 | ✅ | `execute/channel.go` ChannelRegistry + ChannelRouter |
| 3 层 fail-safe 5/5 | ✅ | buildObserveRequest / VerifyWithRetry / synthesizeVerdict / applyResumeSession |
| WorkTree 唯一根 | ✅ | `TaskManager.Tree()` facade，无 Task flat-view |
| 0 字符串 shim 残留 | ✅ | FromVerifierTyped typed enum 取代 FromVerifier |
| 0 type-alias shim | ✅ | coordinator/hubspoke 已并入 sessionorchestrator/orchtypes |
| 0 Task conversion helper | ✅ | `workitem.go` 内无 `ToTask/FromTask` 等 helper |
| 0 taskStoreAdapter | ✅ | `WorkItemStore` interface 直接实现 |
| go vet `./...` | ✅ | 全包 clean |
| `go test -race ./internal/layers/orchestration/...` | ✅ | 22/22 packages PASS |
| `scripts/verify-archive.sh` | ✅ | 12/12 PASS（escape v5 + 之前 11 项归档） |
