# D7 Orchestration — 5 节点管道调用链路（MUPS v4.3）

**文档类型:** 运行时序 + 调用链路（pipeline architecture & call-chain reference）
**Domain:** D7 Orchestration
**DSAFT Type:** 核心域
**Version:** 1.0.0
**Status:** Active
**Last Updated:** 2026-06-25
**架构入口:** `openspec/specs/d7-orchestration/spec.md`（DSAFT 规范 SoT）
**领域 SoT:** `openspec/specs/d7-orchestration/d7-domain.md`（North Star / Out of Scope）
**详细设计:** `openspec/specs/d7-orchestration/design.md`（六段式架构设计）
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
4. **§4 调用链路 — OrchestratePath 6 步时序**（MUPS 5 节点管道运行时序）
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

D7 域共 **13 个 S 场景**（S1-S6 + S8-S13，S7 留空 N/A），按"5 节点管道主干 + 横切关注点"分层。

### 2.1 13 个 S 场景清单

| S 场景 | 名称 | 职责 | 状态 | 节点归属 |
|--------|------|------|------|----------|
| D7-S1 | Work Model | Task/Plan 数据模型 + 磁盘持久化 + PlanMode 状态机 | IMPLEMENTED (v1.0) | 基础 |
| D7-S2 | Session Orchestrator | ProcessMessage 入口 + Turn 主循环 + 4 IntentKind 正交分发 | IMPLEMENTED (v1.0) | 基础 |
| D7-S3 | Wave Scheduler | TaskGraph DAG + 5-slot WorkerPool + ConflictGuard + ContextPolicy | IMPLEMENTED (v1.1) | 基础 |
| D7-S4 | Execution Flow | Hub 双通道（WorkPlan + SessionQueue + IM）+ SpokeBridge | IMPLEMENTED (v1.0+v1.1 closure) | 基础 |
| D7-S5 | Decision & Planning | 4 PlanKind + Planner + MatchKind 4 规则 + Plan.Validate PP-1/2/3 | IMPLEMENTED (Phase 2 PR-B1) | **管道主干（Plan）** |
| D7-S6 | Error Aggregation & Metrics | `errors.Join` 聚合 + InterruptMetrics + sandbox cleanup observability + 6 metric 字段 | IMPLEMENTED (DM-20260621-010 + DM-20260622-001) | 横切 |
| **D7-S8** | Observation | 4 类 ObsKind × 2 Category + UncertaintyReport + UncertaintyCoord + 3 Observer 子模块 | IMPLEMENTED (Phase 2 PR-A1 + PR-RF) | **管道主干（Observe）** |
| **D7-S9** | Execute | Artifact 4 类 + SideEffect 5 态 + Channel 4 个 + ChannelRouter | IMPLEMENTED (Phase 3 PR-C1 + PR-C2) | **管道主干（Execute）** |
| **D7-S10** | Verify | VerdictKind 4 态 + AggregateVerdicts + VerdictToExitReason + Evidence + SystemAnomaly + 14 ExitReason | IMPLEMENTED (Phase 4) | **管道主干（Verify）** |
| **D7-S11** | Learn | 5 类 LearningAsset + BayesianUpdate + Wilson 95% CI + AdaptivePrior + Memory 3 通道 + Learner | IMPLEMENTED (Phase 5 PR-E1..E5) | **管道主干（Learn）** |
| **D7-S12** | Observe-Learner Wiring | `buildObserveRequest` 3 层 fail-safe + `IntentClassifier.ClassifyWithPrior` + 4 E2E 集成测试 | IMPLEMENTED (Phase 6 PR-F1/F2/F3) | **闭环（LP-1 wiring）** |
| **D7-S13** | Auto-Close | `processAutoClose` + `synthesizeVerdict` + `TrackMode` 3-tier 解析 + sessionSpan 6 prior attributes | IMPLEMENTED (Phase 7 PR-7.1/7.2/7.3) | **闭环（Runtime LP-1）** |

### 2.2 S 场景关系图

```
                              D7-S2 SessionOrchestrator (入口 / Turn Leader)
                                           │
        ┌───────────────┬──────────────────┼──────────────────┬─────────────────┐
        ↓               ↓                  ↓                  ↓                 ↓
   4 IntentKind     ClassifyIntent     ProcessMessage     RunTurnLoop     DispatchWorker
  (Skip/Command/   (D7-S5)             (含 buildObserve  (D3+D2 调用)    (D7-S4 hub)
   Fast/Orches)                          Request 优先)
                                           │
  ┌────────────────────────────────────────┼─────────────────────────────────────────┐
  │            MUPS 5 节点管道 (IntentOrchestrate 路径)                                │
  │                                                                                   │
  │   D7-S8 Observe ─→ D7-S5 Plan ─→ D7-S9 Execute ─→ D7-S10 Verify ─→ D7-S11 Learn │
  │       ↑                                                                       │
  │       │   D7-S12 Observe-Learner Wiring (buildObserveRequest 3 层 fail-safe)   │
  │       └───────────────── LP-1 闭环 (AdaptivePrior 注入) ────────────────────────┘
  │                                  ↑
  │                       D7-S13 Auto-Close (processAutoClose + synthesizeVerdict)
  │                         channel 关闭时异步触发 Learn
  │
  ├── 基础：D7-S1 WorkModel (Task/Plan 持久化 + 状态机)
  ├── 执行：D7-S3 WaveScheduler (DAG + WorkerPool + ConflictGuard) ─→ 喂给 D7-S9
  ├── 事件：D7-S4 ExecutionFlow (Hub 双通道 + SpokeBridge) ─→ IM 广播
  └── 横切：D7-S6 Error Aggregation & Metrics (errors.Join + 6 metric)
```

### 2.3 关键关系要点

1. **D7-S2 是唯一入口** — 4 IntentKind 决定走哪条路径：
   - `IntentSkip` → close channel（不触发 Auto-Close）
   - `IntentCommand` → CommandHandler（/plan, /task, /help, /stop）
   - `IntentFast` → FastPath → D3 (LLM) + D2 (Prepare/ToolRound/Persist)
   - `IntentOrchestrate` → OrchestratePath → **5 节点管道**
2. **D7-S8 / S5 / S9 / S10 / S11 是 5 节点管道主干**，按 LP-5 反向追溯链串联
3. **D7-S12 是 S11→S8 的回写线**（LP-1 闭环 wiring）
4. **D7-S13 是 S10→S11 的自动触发器**（channel 关闭时 runtime LP-1）
5. **D7-S1/S3/S4 是 v2.0 已有基础设施**，MUPS 期间未改，被新节点复用
6. **D7-S6 横切所有 S 场景**（错误聚合 + metric）

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
          │                     ├─ /plan → PlanCLICommands → PlanMode
          │                     ├─ /task → CLICommands → TaskManager (D7-S1)
          │                     └─ /help, /stop → explicit handlers
          │
          ├─ IntentFast       → FastPath.Run
          │                     └─ TurnOrchestrator → D3 (LLM Gateway)
          │                                       → D2 (Prepare / ToolRound / Persist)
          │
          └─ IntentOrchestrate → OrchestratePath.Run  ← MUPS 5 节点管道主入口
                                │
                                ↓
                            §4 6 步时序
```

---

## §4 调用链路 — OrchestratePath 6 步时序

```
OrchestratePath.Run(ctx, req, report)
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

### 6.3 代码实现位置

| 节点 | 代码位置 |
|------|---------|
| D7-S8 Observe | `internal/layers/orchestration/orchtypes/{observation,uncertainty_report,uncertainty_coord,observe_request,intent_quantizer,anomaly_detector}.go` |
| D7-S5 Plan | `internal/layers/orchestration/decisionplanning/` + `internal/layers/orchestration/workmodel/plan_*.go` |
| D7-S9 Execute | `internal/layers/orchestration/execute/` + `internal/shared/types/execute.go` + `internal/layers/orchestration/orchtypes/artifact_kind_alias.go` |
| D7-S10 Verify | `internal/shared/types/verdict.go` + `internal/layers/orchestration/workmodel/aggregate_verdicts.go` + `internal/layers/orchestration/orchtypes/verifier.go` |
| D7-S11 Learn | `internal/layers/orchestration/learn/{learning_asset,asset_content,reputation_evidence,bayesian_update,adaptive_prior,memory,asset_builder,reputation_store,learner}.go` + `internal/shared/types/learning.go` |
| D7-S12 wiring | `internal/layers/orchestration/sessionorchestrator/orchestrator.go::buildObserveRequest` + `internal/layers/orchestration/orchtypes/{observe_request,errors}.go` |
| D7-S13 Auto-Close | `internal/layers/orchestration/sessionorchestrator/{autoclose,tracing,orchestrator}.go` + `internal/layers/orchestration/learn/{learner,asset_builder}.go` + `internal/layers/orchestration/orchtypes/process.go` |
| 生产 wiring | `internal/bootstrap/wire_coordinator.go::WireD7` + `internal/bootstrap/wire_wave.go` |

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
