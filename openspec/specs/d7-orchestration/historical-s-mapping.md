# D7 Orchestration — Historical S / Contract Mapping

**Status:** Historical reference only — **not** current canonical S layers
**Version:** 1.0.0
**Last Updated:** 2026-07-01 (devrix-d7-historical-s-cleanup DM-20260701-003)
**Parent:** `openspec/specs/d7-orchestration/spec.md`
**Current SoT:** D7 canonical S = **S1-S6** only (see `spec.md` Scenarios table)

> Former MUPS node splits (D7-S7–S14), rollup/subcontext (S15–S16), pessimistic runtime (S18), and TaskContract assets (S20–S21) live here for traceability. **Do not** add these IDs back into current registry headings.

---

## Summary Mapping (current target)

| Historical ID | Current Target | Meaning |
|---------------|----------------|---------|
| D7-S7 | D7-S6-A01 | MUPS 5-node pipeline entry |
| D7-S8 | D7-S5-A06 | Observe + UncertaintyReport |
| D7-S9 | D7-S6-A02 | Execute Artifact / WorkItemExecutor |
| D7-S10 | D7-S6-A03 | Verify Verdict / Deliverable Gate |
| D7-S11 | D7-S6-A04 | Learn Node / Reputation / Memory |
| D7-S12 | D7-S6-A05 | Observe-Learner closed loop |
| D7-S13 | D7-S2-A07 + D7-S6-A03 | AutoClose / session completion |
| D7-S14 | D7-S6-A06 | MUPS v5 EscapeEngine |
| D7-S15 | D7-S1-A07 | WorkItem Rollup |
| D7-S16 | D7-S1-A08 + D7-S5-A07 | Layer SubContext / ScopeContract / StrategicPlanProposer |
| D7-S18 | D7-S6-A07 | Pessimistic + Fallback |
| D7-S20 | Contract → D7-S1/D7-S6 | TaskSpec downlink contract |
| D7-S21 | Contract → D7-S1/D7-S6 | TaskReport uplink contract |

---

### Historical: D7-S7 MUPS 5 节点管道入口（mapped to D7-S6-A01）

> Historical note: 该段保留 v4.3 追溯信息。DM-20260701-002 后，MUPS 5 节点不再作为 current S 层扩张，而是映射到 S5/S6 的 A/F 活动链。
> 博弈角色: Pipeline Coordinator（5 节点 owner + 闭环）
> **与 S1-S6 关系：** S1 是 State Authority，S2-S5 是入口/调度/信号/决策；S7-S14 是 5 节点管道的**纵向自治单元**，彼此通过 DependencyContract 串联（Observe→Plan→Execute→Verify→Learn）。

#### D7-S7 节点间依赖契约

```text
Observe(S8) ── UncertaintyReport ──▶ Plan(S8-PR-B1) ── Plan ──▶ Execute(S9) ── Artifact ──▶ Verify(S10) ── Verdict ──▶ Learn(S11)
   ▲                                                                                                              │
   └──────────────── ReputationEvidence (Bayesian) ←───────────────────────────────────────────────────────────────┘
```

| 节点 | 入口契约 | 出口契约 | 节点间约束 |
|------|---------|---------|----------|
| Observe | SessionID + UserMessage + (可选) AdaptivePrior | UncertaintyReport | 4 类 Observation 必须落 UncertaintyCoord |
| Plan | UncertaintyReport | Plan{ID, Kind, Strength, Steps, FailureCriteria, BlastRadius, SourceObservationIDs} | Plan.SourceObservationIDs 必须可反向追溯 Observation |
| Execute | Plan | Artifact{ID, Kind, Payload, Evidence, SourcePlanID} | Artifact.SourcePlanID 必须可反向追溯 Plan |
| Verify | Artifact + Plan | Verdict{Kind, Evidence, Reason, SourceArtifactID} | Verdict.SourceArtifactID 必须可反向追溯 Artifact |
| Learn | Verdict + Plan + Observation (追溯链) | LearningAsset + ReputationEvidence | ReputationEvidence 必须能注入下一轮 Observe 作先验 |

---

### Historical: D7-S8 Observe 节点（mapped to D7-S5-A06）

> North Star: 把"用户消息 + 历史 + 上下文"结构化为 **4 类 Observation**，量化后产出 **UncertaintyReport**（含 UncertaintyCoord）。
> 博弈角色: Information Quantizer（产结构化观察 + 不确定性度量）
> **核心实体：** `Observation{Kind, Strength, Scope, Stakes, SourceIDs}` + `UncertaintyReport{Observations, Overall: UncertaintyCoord, Anomalies: []Anomaly, QuantizedIntent: IntentPayload}`

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S8-A15** | **ObserveQuantize** | **A-BE** | **session_id, message, history, prior?** | **UncertaintyReport** | **observation.recorded** | **✅** | **`orchestration/observe/observe_node.go::Observe`**（Phase 2 PR-A1 S7_Archived + PR-RF 5 review fix）|
| D7-S8-A15-F | AnomalyDetector | F-BE | observations | []Anomaly | — | ✅ | `orchestration/observe/anomaly.go` (Phase 2 PR-A1) |
| D7-S8-A15-F | IntentQuantizer | F-BE | observations + anomaly | IntentPayload | — | ✅ | `orchestration/observe/intent_quantizer.go` (Phase 2 PR-A1 + WithPrior 变体 Phase 6) |

**4 类 Observation 子类型（按 ★ 等级）：**

| ObsKind | 含义 | 强度范围 | Strength 决定 | 备注 |
|---------|------|---------|-------------|------|
| ObsFact | 已确认事实 | ★★-★★★★ | 由 evidence 数量定 | 不可降级 |
| ObsAnomaly | 异常信号 | ★-★★★ | 由 z-score/pattern 定 | 触发 Plan 升格 |
| ObsSignal | 用户/系统信号 | ★-★★★★ | 由来源权威定 | 命令/状态 |
| ObsUser | 用户意图 | ★-★★★ | 由置信度定 | 走 IntentQuantizer |

---

### Historical: D7-S9 Execute 节点（mapped to D7-S6-A02）

> North Star: 按 Plan 调度执行，产出 4 类 Artifact；4 通道（同步 / 异步 / 试探 / 探索）走 C2/W8 1:1 映射。
> 博弈角色: Mechanism Designer（执行规则 + 副作用边界）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S9-A25** | **ExecuteArtifact** | **A-BE** | **Plan, session_id** | **Artifact** | **artifact.produced** | **✅** | **`orchestration/execute/executor.go::Execute`**（Phase 3 PR-C1 S7_Archived，跨域类型上提 `shared/types.Artifact`）|
| **D7-S9-A26** | **RouteChannel** | **A-BE** | **Plan.Step, Artifact?** | **ChannelKind (sync/async/probe/explore)** | **—** | **✅** | **`orchestration/execute/channel_router.go::RouteChannel`**（Phase 3 PR-C2 S7_Archived，C2/W8 1:1 映射）|

**4 类 Artifact 子类型：**

| ArtifactKind | 含义 | 配套 Channel | Evidence 字段 |
|--------------|------|-------------|--------------|
| StateChangeCert | 状态变更凭证 | sync | BeforeHash, AfterHash, Actor |
| ResponseRecord | 响应记录 | async | StatusCode, Body, LatencyMs |
| ProbeReport | 试探报告 | probe | Hypothesis, Result, Confidence |
| ExperimentData | 探索数据 | explore | Samples, Statistics, AnomalyScore |

**4 Channel 实现（C2/W8）：**

| Channel | ChannelKind | 失败处理 | 重试 |
|---------|-------------|---------|------|
| SyncChannel | sync | fast-fail | 0 |
| AsyncChannel | async | queue+retry | 3 |
| ProbeChannel | probe | backoff+fallback | 2 |
| ExploreChannel | explore | best-effort | 1 |

---

### Historical: D7-S10 Verify 节点（mapped to D7-S6-A03）

> North Star: 验证 Artifact 是否满足 Plan.FailureCriteria + 反向追溯 Observation；产出 4 态 Verdict + 14 ExitReason。
> 博弈角色: Certifier（颁发可信判决）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S10-A32** | **VerifyVerdict** | **A-BE** | **Artifact, Plan** | **Verdict** | **verdict.recorded** | **✅** | **`orchestration/verify/verifier.go::Verify`**（Phase 4 S7_Archived）|
| **D7-S10-A33** | **VerdictToExitReason** | **A-BE** | **Verdict, Plan.ObservationStrength** | **ExitReason (14 态)** | **—** | **✅** | **`orchestration/verify/exit_reason.go::MapToExitReason`**（Phase 4 + 14 ExitReason）|
| **D7-S10-A34** | **ExtractEvidence** | **F-BE** | **Artifact, Verdict** | **Evidence** | **—** | **✅** | **`orchestration/verify/evidence.go::ExtractEvidence`** |
| **D7-S10-A35** | **DetectSystemAnomaly** | **F-BE** | **session history** | **SystemAnomaly?** | **—** | **✅** | **`orchestration/verify/system_anomaly.go::Detect`** |

**4 态 VerdictKind：**

| VerdictKind | 触发条件 | 对应 ExitReason 家族 |
|-------------|---------|---------------------|
| ComplianceVerdict | Plan.FailureCriteria 全满足 | natural / succeeded |
| TimelinessVerdict | 时间窗口满足 | resolved_in_window |
| RootCauseVerdict | 反向追溯到 Observation | natural_with_evidence |
| StatisticalVerdict | 概率阈值满足 | statistically_significant |

**14 ExitReason（8 deterministic + 6 verify-driven）：** 详见 `terminal-state-guide.md` §11。

---

### Historical: D7-S11 Learn 节点（mapped to D7-S6-A04）

> North Star: 把 Verdict + 追溯链沉淀为 **LearningAsset** + **ReputationEvidence**（Bayesian 更新）；下轮 Observe 注入 AdaptivePrior。
> 博弈角色: Memory Curator（记忆资产 + 信誉先验）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S11-A36** | **BuildLearningAsset** | **A-BE** | **Verdict, Plan, Observation (追溯链)** | **LearningAsset** | **asset.stored** | **✅** | **`orchestration/learn/asset_builder.go::Build`**（Phase 5 S7_Archived，4 类 LearningClass）|
| **D7-S11-A37** | **UpdateReputationEvidence** | **A-BE** | **SessionID, Verdict.Kind, Evidence** | **ReputationEvidence** | **reputation.updated** | **✅** | **`orchestration/learn/reputation_store.go::Update`**（Bayesian Beta 更新）|
| **D7-S11-A38** | **BuildAdaptivePrior** | **F-BE** | **ReputationEvidence (历史)** | **AdaptivePrior** | **—** | **✅** | **`orchestration/learn/adaptive_prior.go::Build`**（DefaultDeveloperPrior Beta(5,3) / DefaultOperatorPrior Beta(8,1)）|
| **D7-S11-A39** | **MemoryPersist** | **F-BE** | **LearningAsset** | **—** | **memory.written** | **✅** | **`orchestration/learn/memory.go::Persist`**（3 通道：skill / feedback / scheduled）|
| **D7-S11-A40** | **RunLearner** | **A-BE** | **Verdict + 追溯链** | **[]LearningAsset + ReputationEvidence** | **learner.completed** | **✅** | **`orchestration/learn/learner.go::Learn`** |

**5 类 LearningClass（按 ★）：**

| LearningClass | Kind 字段 | TTL | 注入位置 |
|---------------|-----------|-----|---------|
| SOPAsset | SOP | 90d | skill 通道 → Observe.SkillPrior |
| ProtocolAsset | Protocol | 180d | skill 通道 → Plan.ProtocolHint |
| KnowledgeAsset | Knowledge | 365d | feedback 通道 → Plan.Context |
| ConclusionAsset | Conclusion | 30d | scheduled 通道 → Plan.ConclusionRef |
| ReputationEvidence | (非 asset，meta) | session-bound | 自适应先验 → Observe.Prior |

---

### Historical: D7-S12 Observe-Learner 跨域闭环集成（mapped to D7-S6-A05）

> North Star: 把 Learn 节点的 ReputationEvidence 注入 Observe 节点，形成 **LP-1 Bayesian reputation 闭环**。
> 博弈角色: Closed-Loop Operator（闭环执行）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S12-A41** | **BuildObserveRequest** | **A-BE** | **SessionID, UserMessage, AdaptivePrior?** | **ObserveRequest** | **—** | **✅** | **`orchestration/observe/observe_node.go::ObserveRequest`**（WithPrior 变体，Phase 6）|
| **D7-S12-A42** | **InjectPriorToSession** | **F-BE** | **ObserveRequest** | **ObserveRequest** | **—** | **✅** | **`orchestration/sessionorchestrator/observe_request.go::buildObserveRequest`**（3 层 fail-safe：prior nil → DefaultPrior → Beta(1,1) uniform）|
| **D7-S12-A43** | **E2ECloseLP1** | **A-BE** | **session_id (跨多轮)** | **ReputationEvidence round-trip** | **loop.closed** | **✅** | **`tests/integration/d7/e2e_lp1_closure_test.go`**（Phase 6 E2E 测试；E2E round-trip Reputation 注入下一轮 Observe）|

---

### Historical: D7-S13 Verify 自动闭环（mapped to D7-S2-A07 + D7-S6-A03）

> North Star: ProcessRequest 终态时若 Verifier 未触发，自动调用 synthesizeVerdict + Auto-Close，避免无限 pending。
> 博弈角色: Auto Closer（兜底回收 pending session）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S13-A47** | **ProcessAutoClose** | **A-BE** | **ProcessRequest, session state** | **Verdict + ExitReason** | **session.auto_closed** | **✅** | **`orchestration/verify/auto_close.go::ProcessAutoClose`**（Phase 7 S7_Archived，3 层 fail-safe）|
| **D7-S13-A48** | **TrackMode** | **F-BE** | **session state** | **TrackMode (full/partial/track-only)** | **—** | **✅** | **`orchestration/verify/track_mode.go::ResolveTrackMode`** |
| **D7-S13-A49** | **EmitSessionSpanPrior** | **F-BE** | **session, prior** | **sessionSpan (6 prior attributes)** | **—** | **✅** | **`orchestration/sessionorchestrator/session_span.go::EmitPrior`**（prior.adaptive_kind / prior.beta_alpha / prior.beta_beta / prior.evidence_count / prior.cycle_count / prior.last_update）|

**4 条 Auto-Close 触发规则：** 详见 `terminal-state-guide.md` §12。

---

### Historical: D7-S14 EscapeEngine + ResumeSession（mapped to D7-S6-A06）

> North Star: 当 Observe/Plan/Execute/Verify 任一节点 stall/error 时，触发 **5 层 CircuitBreaker**（L0..L5）；用户 `/resume` 后 **3 决策路由**（A fall through / B user_accept→ForceExit / C user_cancel→AbortWithAudit）。
> 博弈角色: Escape Operator（紧急逃生 + 受控恢复）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S14-A50** | **RunEscapeEngine** | **A-BE** | **session_id, signal** | **EscapeDecision** | **escape.{triggered,lifted}** | **✅** | **`orchestration/escape/engine.go::Run`**（Phase v5 PR-V5.0 S7_Archived，5 层 CircuitBreaker L0..L5）|
| **D7-S14-A51** | **ApplyResumeSession** | **A-BE** | **session_id, user_choice** | **ResumeDecision** | **session.{resumed,force_exited,aborted}** | **✅** | **`orchestration/sessionorchestrator/resume.go::applyResumeSession`**（PR-V5.6 S7_Archived + review fix PR-V5.6-rf，3 层 fail-safe + 3 决策路由）|
| **D7-S14-A52** | **EmitSessionSpanResume** | **F-BE** | **session, decision** | **sessionSpan (3 resume attributes)** | **—** | **✅** | **`orchestration/sessionorchestrator/session_span.go::EmitResume`**（resume.decision / resume.circuit_level / resume.user_choice）|

**5 层 CircuitBreaker：**

| Level | 触发条件 | 行为 |
|-------|---------|------|
| L0 | 正常 | observe → plan → execute → verify → learn |
| L1 | 单节点 1 次 error | retry once |
| L2 | 单节点 3 次 error | 切换 fallback path |
| L3 | 跨节点 2 次 stall | 缩窄 plan 范围 |
| L4 | 跨节点 5 次 stall | pause + ask user |
| L5 | 跨节点 10 次 stall | hard escape → abort + audit |

---

## Statistics

| Scenarios | Activities | Implemented | Partial | Planned |
|-----------|------------|-------------|---------|---------|
| **6 Canonical (S1–S6) + 1 横切 (Hardening) + v7.0 TaskContract (S20/S21)** | **55**（49 + 6） | **55** | **0** | **0** |
| + Legacy 追溯段（已并入 Canonical） | +0 | — | — | — |

> **v1.0 + v1.1 closure (2026-06-15):** All S-layer activities are now IMPLEMENTED. v2.0-c/f slices (A06/A07 T 层) are still PLANNED at the T level (no test fixtures in `turn/orchestrator_test.go`); the A 层 activities themselves are wired and active in `bootstrap/wire_coordinator.go`.
>
> **v4.3 MUPS closure (2026-06-25):** Canonical S 扩展至 14 个（S1-S14），MUPS 5 节点管道（Observe/Plan/Execute/Verify/Learn）+ 跨域集成 + Verify Auto-Close + EscapeEngine 全部 IMPLEMENTED。56 A 活动覆盖：S1(6) + S2(7) + S3(4) + S4(5) + S5(4) + S6(1) + S7(1,5 节点门面) + S8(1,Observe) + S9(2,Execute) + S10(4,Verify) + S11(5,Learn) + S12(3,Observe-Learner 闭环) + S13(3,Verify 自动闭环) + S14(3,EscapeEngine)。
>
> **v6.0.0 6 S closure (DM-20260626-001，2026-06-26):** Canonical S **14 → 6 + 1 横切**，A 活动 **56 → 49**。博弈角色对齐：State Authority / Mediator+Turn Leader+Error Recovery / Mechanism Designer / Costly Signaler+Certifier / Information Producer+Quantizer / Pipeline Coordinator+Memory Curator / 横切 Discipline Keeper。MUPS 5 节点管道挂载：Observe+Plan 归 S5，Execute+Learn 归 S6，Verify 归 S4，ResumeSession+EscapeEngine 入口 归 S2，AutoClose 归 S2。7 个 Legacy A 全部并入 Canonical S。详见 §v6.0.0 6 S 精简映射。

---

## v6.0.0 6 S 精简映射（DM-20260626-001）

> 14 S → 6 S + 1 横切后，A 活动重映射如下。**所有原有 A 活动均保留并归入新 S**，只是 S 编号变化；7 个 Legacy A 全部并入 Canonical S（不再保留独立 Legacy 段）。

### 14 S → 6 S 映射表

| 原 S | 原 A 数 | 新 S | 博弈角色 | 新 A 编号（示例） | 变化 |
|------|--------|------|----------|------------------|------|
| S1 Work Model | 6 | **S1 WorkModel** | State Authority | S1-A01..A04 | A01-A03 保留；A04-A06（PlanMode/PlanAgent）下放到 S1-A04 |
| S2 Session Orchestrator | 7 | **S2 SessionOrchestrator** | Mediator + Turn Leader + Error Recovery | S2-A01..A07 | A01-A04/A06/A07 保留；CommandHandler 升级为 A03 |
| S3 Wave Scheduler | 4 | **S3 WaveScheduler** | Mechanism Designer | S3-A01..A04 | 不变 |
| S4 Execution Flow | 5 | **S4 ExecutionFlow + Verify** | Costly Signaler + Certifier | S4-A01..A09 | A01-A05 保留；A06-A09 来自原 S10（Verify）|
| S5 Decision & Planning | 4 | **S5 DecisionPlanning + Observe** | Information Producer + Quantizer | S5-A01..A08 | A01-A04 保留；A05-A08 来自原 S8（Observe/Plan）|
| S6 Observability & Hardening | 1 | **Cross-cutting: Hardening** | Discipline Keeper | Hardening-A01..A02 | 不占 S 位；A14 拆为 2 A |
| S7 MUPS Pipeline | 1 | **（并入 S2 + S4 + S5 + S6）** | — | — | S7 角色拆分到 6 S（Pipeline Coord 归 S6）|
| S8 Observe | 1 | **S5** | Information Quantizer | S5-A06（ObserveQuantize）+ S5-A07（PlanGenerate）| 节点归属调整 |
| S9 Execute | 2 | **S6 MUPS Pipeline** | Pipeline Coordinator | S6-A01（ExecuteArtifact）+ S6-A02（RouteChannel）| 节点归属调整 |
| S10 Verify | 4 | **S4** | Certifier | S4-A06..A09 | 节点归属调整 |
| S11 Learn | 5 | **S6** | Memory Curator | S6-A03..A07（LearningAsset / Reputation / Memory）| 节点归属调整 |
| S12 Observe-Learner 闭环 | 3 | **S2 + S5** | Closed-Loop Operator | S2-A07（buildObserveRequest）+ S5-A08（PriorLoad）| 拆分 |
| S13 Verify 自动闭环 | 3 | **S2 + S4** | Auto Closer | S2-A07（AutoClose）+ S4-A08（sessionSpan Prior）| 拆分 |
| S14 EscapeEngine | 3 | **S2** | Escape Operator | S2-A08（EscapeDispatch）+ S2-A09（ResumeSession）| 节点入口归 S2；Engine 物理独立 |
| **总计** | **49** | **6 S + 1 横切** | — | **49** | -7（去 Legacy 段重复 + S7 拆分精简）|

### 6 S 完整 A 清单（49 A）

#### D7-S1 WorkModel — State Authority（4 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| S1-A01 | CreateWorkPlan | D7-S1-A01 | `sessionorchestrator/workmodel.go` + `workmodel/plan_mode.go` |
| S1-A02 | ManageWorkItem | D7-S1-A02 | `workmodel/{work_tree,task_manager}.go` |
| S1-A03 | QueryWorkPlan | D7-S1-A03 | `executionflow/hub/hub.go` Snapshot |
| S1-A04 | ExecutePlanAgent | D7-S1-A06（原 PlanMode A04-A05 已并入 S2 CommandHandler）| `workmodel/plan_agent.go` |

#### D7-S2 SessionOrchestrator — Mediator + Turn Leader + Error Recovery（7 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| S2-A01 | ProcessMessage | D7-S2-A01 | `sessionorchestrator/orchestrator.go` |
| S2-A02 | HandleInterrupt | D7-S2-A03 | `sessionorchestrator/interrupt.go` |
| S2-A03 | CommandHandler | D7-S2-A04-LEGACY | `sessionorchestrator/command_handler.go` |
| S2-A04 | DispatchWorker | D7-S2-A04 | `sessionorchestrator/dispatch.go` |
| S2-A05 | RunTurnLoop | D7-S2-A06 | `turn/orchestrator.go` |
| S2-A06 | InvokeLLM | D7-S2-A07 | `turn/llm.go` |
| S2-A07 | AutoClose + Resume + Escape + PriorBuild | D7-S13-A47 + D7-S14-A50/A51 + D7-S12-A42 | `sessionorchestrator/{autoclose,resume,observe_request}.go` + `escape/engine.go`（**入口归 S2，Engine 物理独立**）|

#### D7-S3 WaveScheduler — Mechanism Designer（4 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| S3-A01 | ScheduleWave | D7-S3-A01 | `wavescheduler/scheduler.go` |
| S3-A02 | ResolveWorkerContext | D7-S3-A02 | `wavescheduler/context.go` |
| S3-A03 | GuardConflict | D7-S3-A03 | `wavescheduler/conflict.go` |
| S3-A04 | HardenScheduler | D7-S3-A04 | `wavescheduler/scheduler.go::markWaveDone` |

#### D7-S4 ExecutionFlow + Verify — Costly Signaler + Certifier（9 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| S4-A01 | PublishFlowEvent | D7-S4-A01 | `executionflow/hub/hub.go` |
| S4-A02 | SnapshotWorkPlan | D7-S4-A02 | `executionflow/hub/hub.go` |
| S4-A03 | NotifyGateway | D7-S4-A03 | `executionflow/imsink/gateway.go` |
| S4-A04 | BridgeAgentSpoke | D7-S4-A04 | `executionflow/bridge/agent_bridge.go` |
| S4-A05 | BridgeSubQuerySpoke | D7-S4-A05 | `executionflow/bridge/subquery_bridge.go` |
| S4-A06 | VerifyVerdict | D7-S10-A32 | `verify/verifier.go` |
| S4-A07 | VerdictToExitReason | D7-S10-A33 | `verify/exit_reason.go` |
| S4-A08 | AggregateVerdicts | D7-S10-A34 | `verify/aggregate.go` |
| S4-A09 | DetectSystemAnomaly | D7-S10-A35 | `verify/system_anomaly.go` |

#### D7-S5 DecisionPlanning + Observe — Information Producer + Quantizer（8 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| S5-A01 | ClassifyIntent | D7-S5-A01 | `decisionplanning/classifier.go` |
| S5-A02 | SynthesizeTaskGraph | D7-S5-A02 | `decisionplanning/decomposer.go` |
| S5-A03 | SelectExecutor | D7-S5-A03 | `decisionplanning/executor.go` |
| S5-A04 | EvaluateIntent | D7-S2-A02 | `decisionplanning/classifier.go`（从 S2 移入）|
| S5-A05 | TailShadowClassify | D7-S5-A05 | `decisionplanning/shadow_classifier.go` |
| S5-A06 | ObserveQuantize | D7-S8-A15 | `observe/observe_node.go::Observe` |
| S5-A07 | PlanGenerate | D7-S8-A22 | `observe/plan/planner.go::Plan` |
| S5-A08 | PriorLoad | D7-S12-A41 | `observe/observe_node.go::ObserveRequest`（WithPrior 变体）|

#### D7-S6 MUPS Pipeline — Pipeline Coordinator + Memory Curator（15 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| S6-A01 | ExecuteArtifact | D7-S9-A25 | `execute/executor.go::Execute` |
| S6-A02 | RouteChannel | D7-S9-A26 | `execute/channel_router.go::RouteChannel` |
| S6-A03 | ChannelDispatch | （新 P0 span）⭐ | `mups/channel/router.go::Select`（v6.0.0 新增）|
| S6-A04 | ToolCall | （隐含在 S6-A01）| `execute/tool_call.go`（独立抽出）|
| S6-A05 | RetryPolicy | （隐含在 S6-A01）| `execute/retry.go`（独立抽出）|
| S6-A06 | BuildLearningAsset | D7-S11-A36 | `learn/asset_builder.go::Build` |
| S6-A07 | UpdateReputationEvidence | D7-S11-A37 | `learn/reputation_store.go::Update` |
| S6-A08 | BuildAdaptivePrior | D7-S11-A38 | `learn/adaptive_prior.go::Build` |
| S6-A09 | MemoryPersist | D7-S11-A39（新 P0 span）⭐ | `learn/memory.go::Persist`（3 通道统一）|
| S6-A10 | RunLearner | D7-S11-A40 | `learn/learner.go::Learn` |
| S6-A11 | FeedbackPersist | （从 MemoryPersist 拆出）| `learn/feedback.go` |
| S6-A12 | ScheduledPersist | （从 MemoryPersist 拆出）| `learn/scheduled.go` |
| S6-A13 | CrossSessionLearning | （隐含在 S6-A10）| `learn/cross_session.go`（独立抽出）|
| S6-A14 | ObserveLearnerLoop | D7-S12-A43 | `learn/observer_loop.go::Loop`（E2E LP-1）|
| S6-A15 | AutoClose | D7-S13-A47 | `verify/auto_close.go::ProcessAutoClose`（S6 治理下的兜底）|

#### Cross-cutting Hardening — Discipline Keeper（2 A）

| 新 A ID | Name | 原 ID | Code |
|---------|------|-------|------|
| Hardening-A01 | HardenMetricsAndConcurrency | D7-S6-A14 | `hardening/metrics.go` + `hardening/concurrency.go`（**拆 2 A**）|
| Hardening-A02 | HardenCircuitBreakerMonitor | D7-S6-A14（部分）| `hardening/circuit_breaker.go`（**从 escape 拆分**）|

### 14 S → 6 S 合并依据（v6.0.0）

| 14 S 冗余类型 | 案例 | 6 S 解决方案 |
|---------------|------|--------------|
| 角色重合 | S4 + S9 都叫 "Costly Signaler" | S4 吸收 S9 验证角色（Verify 节点）；S9 Execute 改归 S6 Pipeline |
| 代码同址 | S7 = S2 自身；S13 = S2 内部文件；S12 散落在 S2 内 | S7/S12/S13 全部并入 S2；Engine 物理独立但入口归 S2 |
| 跨切不该独立成 S | S6 Hardening 是观测基础设施 | Hardening 改为 cross-cutting（不占 S 位）|
| 粒度过细 | S5 决策 + S8 Observe Quantize 都属"信息生产+量化" | S5 + S8 合并为 S5（Information Producer + Quantizer）|

---

## D7-S18: Pessimistic Commit + Rule-based Fallback（v7.0 PR-B, DM-20260629-008）✅ IMPLEMENTED（6/7 T 点）

> North Star: 资源耗尽 / 5 层 CB L1 触发 / 连续 3 轮 INDETERMINATE / Verifier 空证 PASS / 人工 abort 5 类触发条件下，**Pessimistic Commit** 给出 MVPArtifact (best-effort 输出 + 风险警告) 替代完全无产出的失败。**Rule-based Fallback** 在 VERDICT 连续 INDETERMINATE 时按 4 候选规则打分选最优。Feature Flag `D7_PESSIMISTIC_COMMIT_ENABLED` 默认 disabled, PR-B 0 行为变更.
>
> 博弈角色: Defensive Runtime Keeper (L3 防御运行时层, 4-Layer × 3-Phase 框架的 L3)
>
> **物理包:**
> - `internal/layers/orchestration/interfaces/{contracts.go,fallback_policy.go,convergence_budget.go}` (3 NEW, pure types)
> - `internal/layers/orchestration/escape/fallback.go` (NEW, PessimisticCommitGuard 默认实现, ~310 LOC)
> - `internal/layers/orchestration/escape/engine.go` (+NotifyPessimistic + SetPessimisticGuard)
> - `internal/layers/orchestration/mups/execute/channel.go` (+ChannelRouter.SetPessimisticGuard + ApplyPessimisticCommit)
> - `internal/bootstrap/pessimistic_guard_wire.go` (NEW, env helper + factory)
>
> **5 类触发条件 (PR-B design.md §3.2):**
> 1. 资源耗尽 (tokens_remaining ≤ reserve) → Pessimistic
> 2. EscapeForceExit / CB L1+ → Pessimistic
> 3. ≥ 3 轮 INDETERMINATE → Rule-based
> 4. Verifier "空证 PASS" → Pessimistic
> 5. 人工 abort (IM 通道关闭) → Abort (向后兼容)
>
> **4 ORCH_* SentinelError 码 (7110-7113):**
> - `ORCH_PESSIMISTIC_TRIGGERED_7110` — pessimistic commit triggered
> - `ORCH_PESSIMISTIC_MVP_EMPTY_7111` — mvp artifact output is empty
> - `ORCH_FALLBACK_RULE_INVALID_7112` — fallback rule not recognized
> - `ORCH_FALLBACK_ABORT_TIMEOUT_7113` — fallback abort timeout
>
> **0 行为变更承诺:** Feature Flag 默认 disabled, 全部 PessimisticCommitGuard 方法都是 nil/disabled 守门 no-op. PR-A 已 S7_Archived 的 11/11 T 点完全兼容.

### D7-S18-A11 Pessimistic Commit（5 类触发 + MVPArtifact）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S18-A11** | **EvaluatePessimistic** | **A-BE** | **spec, report, budget** | **(ok, blockedReason, err)** | **report.{FallbackUsed, MVPArtifact}** | **✅** | **`orchestration/interfaces/contracts.go::PessimisticCommitGuard.Evaluate` + `orchestration/escape/fallback.go::DefaultPessimisticCommitGuard.Evaluate`** (5 类触发: resource_exhausted / cb_l1 / indeterminate_3x / empty_evidence / manual_abort) |

### D7-S18-A12 Rule-based Fallback（4 候选规则）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S18-A12** | **ResolveRuleFallback** | **F-BE** | **report** | **(policy, ruleName)** | **—** | **✅** | **`orchestration/escape/fallback.go::DefaultPessimisticCommitGuard.ResolveFallback` + `orchestration/interfaces/fallback_policy.go::ParseFallbackRuleName`** (4 候选: most_tests_passed / compiled_clean / min_cost / min_uncertainty, default min_uncertainty) |

---

## Contract Mapping: TaskContract 统一（former D7-S20/S21, non-current S）

> North Star: **TaskSpec（下行契约） + TaskReport（上行契约）** 是 D7 跨节点（Plan / Execute / Verify / Learn）通讯的**统一结构化载体**——v7.0 PR-A 用纯类型包 `interfaces` 替代散落的 wire 数据。
>
> 博弈角色: Contract Owner（跨节点契约锚点 + 不可变 + 字段语义）
>
> **设计：** 4-Layer × 3-Phase 框架（本 Change 仅完成 L1 接口层 + L2 字段语义层 + L4 spec 同步；L3 防御运行时层 留给 PR-B + PR-C）。
>
> **物理包：** `internal/layers/orchestration/interfaces/`（7 NEW: doc.go + errors.go + task_spec.go + task_report.go + task_spec_test.go + task_report_test.go + taskcontract_test.go）。**pure types 原则：0 import D7 任何子包**，仅依赖 `internal/shared/errors/`（SentinelError）。
>
> **Additive 嵌入：** `ChannelRequest.Spec` + `LearnRequest.Report` 是不破坏老路径的可选指针，老代码可继续工作。

### D7-S20-A01 TaskSpec 下行契约

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S20-A01** | **BuildTaskSpec** | **A-BE** | **session_id, plan, channel, work_item** | **TaskSpec** | **task_spec.created** | **✅** | **`orchestration/interfaces/task_spec.go::NewTaskSpec` + `task_spec.go::Validate`（3 创建点统一：Plan 节点入口 / Channel.Execute 入口 / WorkItem 节点入口）** |

### D7-S20-A02 TaskReport 上行契约

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S20-A02** | **BuildTaskReport** | **A-BE** | **session_id, channel, verdict, trace_id** | **TaskReport** | **task_report.created** | **✅** | **`orchestration/interfaces/task_report.go::NewTaskReport` + `task_report.go::With*` (WithVerdict/WithDissent/WithBlockage/WithResource 不可变 builder)** |

### D7-S20-A03 TaskContract 治理横切（PR-A spec 同步）

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S20-A03** | **SyncTaskContractSpec** | **A-BE** | **interfaces API** | **spec.md + d7-domain + a/f/t/span-registry 增量** | **—** | **✅** | **`openspec/specs/d7-orchestration/spec.md` v7.0 ADDED 3 Requirement + 12 Gherkin Scenarios** |

### D7-S21-A01 Dissent 沉淀

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S21-A01** | **RecordDissent** | **A-BE** | **TaskReport + Dissent 候选** | **TaskReport (AppendDissent immutable)** | **dissent.recorded** | **✅** | **`orchestration/interfaces/task_report.go::AppendDissent`（top-3 截断 + summary hash + Learn 沉淀到 feedback 通道）** |

### D7-S21-A02 Blockage 分类

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S21-A02** | **ClassifyBlockage** | **A-BE** | **TaskSpec + 失败原因** | **TaskSpec (WithBlockage immutable)** | **blockage.recorded** | **✅** | **`orchestration/interfaces/task_spec.go::WithBlockage`（3 类 kind: permission/resource/contract）** |

### D7-S21-A03 Resource 抽取

| A ID | Name | Type | Input | Output | State Change | Status | Code Location |
|------|------|------|-------|--------|--------------|--------|---------------|
| **D7-S21-A03** | **ExtractResource** | **A-BE** | **ExecutionResult + 上下文** | **TaskReport (WithResource immutable)** | **resource.recorded** | **✅** | **`orchestration/interfaces/task_report.go::WithResource`（token / elapsed_ms / step_count 三件套）** |

### TaskContract Dissent 治理

- **Top-N silent truncation：** `AppendDissent(d Dissent)` 若已有 ≥ 3 Dissent 则 silently 不追加（避免日志/反馈通道被淹没）。
- **Summary hash：** Dissent.Summary 计算 `fnv64a(Summary string)` 8 hex 前缀，`Learn` 节点去重。
- **Learn 沉淀：** Dissent 通过 `LearnRequest.Report.Dissents` 走 `mups/learn/asset/` 现有 feedback 通道（**老路径完全不变**，仅可选追加）。

### TaskContract Blockage 分类（3 类 kind）

| Kind | 含义 | retryable | example |
|------|------|-----------|---------|
| `permission` | 权限不足（403 / IAM deny） | false | WorkItem.Directive 越权 |
| `resource` | 资源耗尽（OOM / disk full / quota） | true | sandbox OOM killed |
| `contract` | 契约违例（不满足 Plan.FailureCriteria） | true | Artifact evidence 缺失 |

### TaskContract Resource 三件套

| 字段 | 类型 | 取值范围 | 用途 |
|------|------|---------|------|
| `TokenUsed` | int | ≥ 0 | LLM token 消耗（嵌入 ReputationStore Bayesian 更新） |
| `ElapsedMs` | int64 | ≥ 0 | wall-clock 时延（嵌入 5 节点管道 P99 监控） |
| `StepCount` | int | ≥ 0 | ReAct iter 次数（嵌入 `D7_SubTurn_Iteration` span） |

### ORCH_* SentinelError（PR-A 5 个，7100-7104 范围）

| 常量 | Code | 触发 | 含义 |
|------|------|------|------|
| `ErrORCHTaskSpecEmpty` | 7100 | `NewTaskSpec("", ...)` | session_id 为空 |
| `ErrORCHTaskSpecChannelUnknown` | 7101 | `Channel.Kind == "" 或未知` | channel kind 不在 sync/async/probe/explore 4 选 1 |
| `ErrORCHTaskReportEmpty` | 7102 | `NewTaskReport("", ...)` | session_id 为空 |
| `ErrORCHTaskReportVerdictEmpty` | 7103 | `Verdict == "" 或未知` | verdict kind 不在 4 VerdictKind |
| `ErrORCHTaskContractTraceInvalid` | 7104 | `TraceID == "" 或格式 ≠ ts_<8 hex>` | trace_id 格式校验 |

---

### v7.0 PR-A 实现统计

| 阶段 | IMPLEMENTED | PARTIAL | PLANNED |
|------|-------------|---------|---------|
| L1 接口层（TaskSpec struct + TaskReport struct + 3 创建点） | **3/3 T** | 0 | 0 |
| L2 字段语义层（Dissent + Blockage + Resource） | **3/3 T** | 0 | 0 |
| L3 防御运行时层（Pessimistic Commit + Hard Evidence + CoW + Rule-based Fallback + Similarity Check） | 0/0 T（本 PR 不涵盖） | — | 留给 PR-B + PR-C |
| L4 治理横切层（spec sync + Coverage + Perf + Security + Cross-Domain Boundary + Error Code + Convergence Span + AdaptiveThreshold + Layout Guard） | **2/9 T**（spec + 各 registry 同步） | 0 | 7 T 留给 PR-B + PR-C |
| **PR-A Total** | **9/11 P0 T** | **0** | **2（spec 同步）** |

---



---

## Historical F Layer Index (former S7–S21)

> Full F registration preserved below for T-point traceability. Current runtime paths: see `f-registry.md` §Current Path Correction.

## D7-S8-A15 ObserveQuantize ✅ (Phase 2 PR-A1 + PR-RF, DM-20260623-001)

> Observe 节点 F 层登记：4 类 Observation 量化 + UncertaintyReport 产出 + UncertaintyCoord 不确定性度量。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S8-A15-F01** | **ClassifyObsKind** | **F-BE** | **raw signal** | **ObsKind (fact/anomaly/signal/user)** | **✅** | **`orchestration/observe/observation.go::ClassifyObsKind`** |
| **D7-S8-A15-F02** | **ScoreObsStrength** | **F-BE** | **observation + evidence** | **strength (★-★★★★)** | **✅** | **`orchestration/observe/observation.go::ScoreStrength`** |
| **D7-S8-A15-F03** | **DetectAnomaly** | **F-BE** | **[]observation** | **[]Anomaly** | **✅** | **`orchestration/observe/anomaly.go::Detect`**（PR-A1）|
| **D7-S8-A15-F04** | **QuantizeIntent** | **F-BE** | **[]observation + anomalies** | **IntentPayload** | **✅** | **`orchestration/observe/intent_quantizer.go::Quantize`**（PR-A1 + WithPrior 变体 Phase 6）|
| **D7-S8-A15-F05** | **BuildUncertaintyCoord** | **F-BE** | **[]observation** | **UncertaintyCoord** | **✅** | **`orchestration/observe/uncertainty_coord.go::Build`**（PR-A1）|
| **D7-S8-A15-F06** | **BuildUncertaintyReport** | **F-BE** | **observations, coord, anomalies, intent** | **UncertaintyReport** | **✅** | **`orchestration/observe/uncertainty_report.go::Build`**（PR-A1）|

---

## D7-S9-A25/A26 Execute ✅ (Phase 3 PR-C1 + PR-C2, DM-20260625-001)

> Execute 节点 F 层登记：4 类 Artifact 数据契约 + 4 Channel 路由 + C2/W8 1:1 映射。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S9-A25-F01** | **BuildArtifact** | **F-BE** | **Plan, ExecutionResult** | **Artifact (4 子类多态)** | **✅** | **`orchestration/execute/artifact.go::Build`**（PR-C1，跨域类型上提 `shared/types.Artifact`）|
| **D7-S9-A25-F02** | **ResolveArtifactKind** | **F-BE** | **Plan.Step.Kind** | **ArtifactKind (state_change/response/probe/experiment)** | **✅** | **`orchestration/execute/artifact.go::ResolveKind`** |
| **D7-S9-A25-F03** | **ExtractEvidence** | **F-BE** | **ExecutionResult** | **Evidence (与 Plan.FailureCriteria 对齐)** | **✅** | **`orchestration/execute/evidence.go::Extract`** |
| **D7-S9-A26-F01** | **RouteChannelKind** | **F-BE** | **Plan.Step** | **ChannelKind (sync/async/probe/explore)** | **✅** | **`orchestration/execute/channel_router.go::RouteChannel`**（PR-C2）|
| **D7-S9-A26-F02** | **DispatchSync** | **F-BE** | **Plan.Step** | **ExecutionResult (synchronous)** | **✅** | **`orchestration/execute/channel_sync.go::Dispatch`** |
| **D7-S9-A26-F03** | **DispatchAsync** | **F-BE** | **Plan.Step** | **<-chan ExecutionResult** | **✅** | **`orchestration/execute/channel_async.go::Dispatch`**（3 retry + queue）|
| **D7-S9-A26-F04** | **DispatchProbe** | **F-BE** | **Plan.Step** | **ExecutionResult (probe)** | **✅** | **`orchestration/execute/channel_probe.go::Dispatch`**（2 retry + backoff）|
| **D7-S9-A26-F05** | **DispatchExplore** | **F-BE** | **Plan.Step** | **ExecutionResult (explore)** | **✅** | **`orchestration/execute/channel_explore.go::Dispatch`**（1 retry best-effort）|

---

## D7-S10-A32..A35 Verify ✅ (Phase 4, DM-20260623-002)

> Verify 节点 F 层登记：4 态 Verdict + 14 ExitReason + Evidence + SystemAnomaly。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S10-A32-F01** | **ExtractVerdict** | **F-BE** | **Artifact + Plan** | **VerdictKind (compliance/timeliness/root_cause/statistical)** | **✅** | **`orchestration/verify/verifier.go::extractVerdict`** |
| **D7-S10-A32-F02** | **AggregateVerdicts** | **F-BE** | **[]Verdict** | **Verdict (聚合)** | **✅** | **`orchestration/verify/verdict.go::AggregateVerdicts`** |
| **D7-S10-A32-F03** | **VerifyWithRetry** | **F-BE** | **Artifact + Plan** | **Verdict (3 次重试兜底)** | **✅** | **`orchestration/verify/verifier.go::VerifyWithRetry`** |
| **D7-S10-A33-F01** | **MapVerdictToExitReason** | **F-BE** | **Verdict + Plan.ObservationStrength** | **ExitReason (14 态)** | **✅** | **`orchestration/verify/exit_reason.go::MapToExitReason`** |
| **D7-S10-A33-F02** | **IsDeterministicReason** | **F-BE** | **ExitReason** | **bool (8 deterministic vs 6 verify-driven)** | **✅** | **`orchestration/verify/exit_reason.go::IsDeterministic`** |
| **D7-S10-A34-F01** | **ExtractEvidence** | **F-BE** | **Artifact + Verdict** | **Evidence (含 SourceObservationIDs 追溯链)** | **✅** | **`orchestration/verify/evidence.go::ExtractEvidence`** |
| **D7-S10-A34-F02** | **ValidateEvidenceCompleteness** | **F-BE** | **Evidence + Plan.FailureCriteria** | **bool** | **✅** | **`orchestration/verify/evidence.go::ValidateCompleteness`** |
| **D7-S10-A35-F01** | **DetectSystemAnomaly** | **F-BE** | **session history** | **SystemAnomaly?** | **✅** | **`orchestration/verify/system_anomaly.go::Detect`** |
| **D7-S10-A35-F02** | **ClassifyAnomalySeverity** | **F-BE** | **SystemAnomaly** | **Severity (warn/error/critical)** | **✅** | **`orchestration/verify/system_anomaly.go::ClassifySeverity`** |

---

## D7-S11-A36..A40 Learn ✅ (Phase 5, DM-20260623-003)

> Learn 节点 F 层登记：4 类 LearningAsset + ReputationEvidence Bayesian 更新 + AdaptivePrior + Memory 3 通道。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S11-A36-F01** | **BuildAssetContent** | **F-BE** | **Verdict + Plan + Observation (追溯链)** | **AssetContent** | **✅** | **`orchestration/learn/asset_builder.go::BuildContent`** |
| **D7-S11-A36-F02** | **ClassifyLearningClass** | **F-BE** | **Verdict.Kind** | **LearningClass (SOP/Protocol/Knowledge/Conclusion)** | **✅** | **`orchestration/learn/asset_builder.go::ClassifyClass`** |
| **D7-S11-A36-F03** | **AssignAssetTTL** | **F-BE** | **LearningClass** | **TTL (90d/180d/365d/30d)** | **✅** | **`orchestration/learn/asset_builder.go::AssignTTL`** |
| **D7-S11-A37-F01** | **BayesianUpdate** | **F-BE** | **prior Beta(α,β) + Verdict.Kind** | **posterior Beta(α',β')** | **✅** | **`orchestration/learn/reputation_store.go::BayesianUpdate`**（PR-v4.5 合并原 bayesian_update.go）|
| **D7-S11-A37-F02** | **StoreReputationEvidence** | **F-BE** | **SessionID + Verdict + Evidence** | **ReputationEvidence** | **✅** | **`orchestration/learn/reputation_store.go::Store`** |
| **D7-S11-A37-F03** | **LoadReputationHistory** | **F-BE** | **SessionID** | **[]ReputationEvidence** | **✅** | **`orchestration/learn/reputation_store.go::LoadHistory`** |
| **D7-S11-A38-F01** | **BuildAdaptivePrior** | **F-BE** | **[]ReputationEvidence** | **AdaptivePrior** | **✅** | **`orchestration/learn/adaptive_prior.go::Build`** |
| **D7-S11-A38-F02** | **DefaultDeveloperPrior** | **F-BE** | **—** | **AdaptivePrior Beta(5,3)** | **✅** | **`orchestration/learn/adaptive_prior.go::DefaultDeveloperPrior`** |
| **D7-S11-A38-F03** | **DefaultOperatorPrior** | **F-BE** | **—** | **AdaptivePrior Beta(8,1)** | **✅** | **`orchestration/learn/adaptive_prior.go::DefaultOperatorPrior`** |
| **D7-S11-A39-F01** | **PersistToSkillChannel** | **F-BE** | **LearningAsset** | **—** | **✅** | **`orchestration/learn/memory.go::PersistSkill`** |
| **D7-S11-A39-F02** | **PersistToFeedbackChannel** | **F-BE** | **LearningAsset** | **—** | **✅** | **`orchestration/learn/memory.go::PersistFeedback`** |
| **D7-S11-A39-F03** | **PersistToScheduledChannel** | **F-BE** | **LearningAsset** | **—** | **✅** | **`orchestration/learn/memory.go::PersistScheduled`** |
| **D7-S11-A40-F01** | **RunLearner** | **F-BE** | **Verdict + 追溯链** | **[]LearningAsset + ReputationEvidence** | **✅** | **`orchestration/learn/learner.go::Learn`** |
| **D7-S11-A40-F02** | **DispatchToMemoryChannel** | **F-BE** | **LearningAsset** | **ChannelKind (skill/feedback/scheduled)** | **✅** | **`orchestration/learn/learner.go::DispatchChannel`** |

---

## D7-S12-A41..A43 Observe-Learner 跨域闭环 ✅ (Phase 6, DM-20260624-001)

> Observe-Learner 跨域闭环 F 层登记：WithPrior 变体 + 3 层 fail-safe + E2E round-trip。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S12-A41-F01** | **BuildObserveRequestWithPrior** | **F-BE** | **SessionID + UserMessage + AdaptivePrior** | **ObserveRequest (含 prior)** | **✅** | **`orchestration/observe/observe_node.go::ObserveRequestWithPrior`**（Phase 6 WithPrior 变体）|
| **D7-S12-A41-F02** | **InjectPriorToIntentQuantizer** | **F-BE** | **ObserveRequest** | **IntentPayload (prior-weighted)** | **✅** | **`orchestration/observe/intent_quantizer.go::QuantizeWithPrior`** |
| **D7-S12-A42-F01** | **ResolvePriorLayer1** | **F-BE** | **SessionID** | **AdaptivePrior (历史)** | **✅** | **`orchestration/sessionorchestrator/observe_request.go::resolvePrior` — Layer 1: 从 ReputationStore Load** |
| **D7-S12-A42-F02** | **ResolvePriorLayer2** | **F-BE** | **AdaptivePrior?** | **AdaptivePrior** | **✅** | **`orchestration/sessionorchestrator/observe_request.go::resolvePrior` — Layer 2: nil → DefaultDeveloperPrior Beta(5,3)** |
| **D7-S12-A42-F03** | **ResolvePriorLayer3** | **F-BE** | **AdaptivePrior?** | **AdaptivePrior** | **✅** | **`orchestration/sessionorchestrator/observe_request.go::resolvePrior` — Layer 3: 仍 nil → Beta(1,1) uniform** |
| **D7-S12-A43-F01** | **E2ECloseLP1RoundTrip** | **F-BE** | **session_id (跨多轮)** | **ReputationEvidence round-trip** | **✅** | **`tests/integration/d7/e2e_lp1_closure_test.go`**（Phase 6 E2E）|

---

## D7-S13-A47..A49 Verify Auto-Close ✅ (Phase 7, DM-20260625-001)

> Verify 自动闭环 F 层登记：processAutoClose + TrackMode + sessionSpan 6 prior attributes。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S13-A47-F01** | **ProcessAutoClose** | **F-BE** | **ProcessRequest + session state** | **Verdict + ExitReason** | **✅** | **`orchestration/verify/auto_close.go::ProcessAutoClose`**（3 层 fail-safe）|
| **D7-S13-A47-F02** | **SynthesizeVerdict** | **F-BE** | **session state + Plan** | **Verdict (default compliance)** | **✅** | **`orchestration/verify/auto_close.go::SynthesizeVerdict`** |
| **D7-S13-A48-F01** | **ResolveTrackMode** | **F-BE** | **session state** | **TrackMode (full/partial/track-only)** | **✅** | **`orchestration/verify/track_mode.go::ResolveTrackMode`** |
| **D7-S13-A48-F02** | **ShouldAutoClose** | **F-BE** | **session state + last activity** | **bool** | **✅** | **`orchestration/verify/auto_close.go::ShouldAutoClose`**（4 条触发规则）|
| **D7-S13-A49-F01** | **EmitSessionSpanPrior** | **F-BE** | **session + prior** | **sessionSpan (6 prior attributes)** | **✅** | **`orchestration/sessionorchestrator/session_span.go::EmitPrior`**（prior.adaptive_kind / beta_alpha / beta_beta / evidence_count / cycle_count / last_update）|

---

## D7-S14-A50..A52 EscapeEngine + ResumeSession ✅ (Phase v5, DM-20260625-003 + DM-20260625-004)

> EscapeEngine + ResumeSession F 层登记：5 层 CircuitBreaker + 3 决策路由 + sessionSpan 3 resume attributes。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S14-A50-F01** | **TriggerEscape** | **F-BE** | **session_id + signal** | **EscapeTrigger** | **✅** | **`orchestration/escape/engine.go::Trigger`** |
| **D7-S14-A50-F02** | **ResolveCircuitLevel** | **F-BE** | **signal + history** | **CircuitLevel (L0..L5)** | **✅** | **`orchestration/escape/circuit_breaker.go::ResolveLevel`** |
| **D7-S14-A50-F03** | **ApplyCircuitBreaker** | **F-BE** | **CircuitLevel + plan** | **plan (modified)** | **✅** | **`orchestration/escape/circuit_breaker.go::Apply`** |
| **D7-S14-A50-F04** | **LiftEscape** | **F-BE** | **session_id** | **—** | **✅** | **`orchestration/escape/engine.go::Lift`** |
| **D7-S14-A51-F01** | **ApplyResumeSessionLayer1** | **F-BE** | **session_id + user_choice** | **ResumeDecision** | **✅** | **`orchestration/sessionorchestrator/resume.go::applyResumeSession` Layer 1** |
| **D7-S14-A51-F02** | **ApplyResumeSessionLayer2** | **F-BE** | **session_id + prior state** | **ResumeDecision** | **✅** | **`orchestration/sessionorchestrator/resume.go::applyResumeSession` Layer 2 (fall through 兜底)** |
| **D7-S14-A51-F03** | **ApplyResumeSessionLayer3** | **F-BE** | **session_id** | **AbortDecision (with audit)** | **✅** | **`orchestration/sessionorchestrator/resume.go::applyResumeSession` Layer 3 (AbortWithAudit)** |
| **D7-S14-A51-F04** | **RouteResumeDecision** | **F-BE** | **user_choice** | **DecisionKind (A fall through / B user_accept→ForceExit / C user_cancel→Abort)** | **✅** | **`orchestration/sessionorchestrator/resume.go::routeResumeDecision`** |
| **D7-S14-A52-F01** | **EmitSessionSpanResume** | **F-BE** | **session + decision** | **sessionSpan (3 resume attributes)** | **✅** | **`orchestration/sessionorchestrator/session_span.go::EmitResume`**（resume.decision / circuit_level / user_choice）|

---

## D7-S18: Pessimistic Commit + Rule-based Fallback F 层 ✅ (PR-B, DM-20260629-008)

> **L3 防御运行时层:** PR-B 落地 PessimisticCommitGuard interface + 5 类触发条件 (resource_exhausted / cb_l1 / indeterminate_3x / empty_evidence / manual_abort) + 3 FallbackPolicy 路径 (Pessimistic / RuleBased / Abort) + 4 候选规则 (most_tests_passed / compiled_clean / min_cost / min_uncertainty, default min_uncertainty). Feature Flag `D7_PESSIMISTIC_COMMIT_ENABLED` 默认 disabled, 0 行为变更.

### D7-S18-A11 Pessimistic Commit

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S18-A11-F01** | **EvaluatePessimistic** | **F-BE** | **spec, report, budget** | **(ok bool, blockedReason string, err error)** | **✅** | **`orchestration/interfaces/contracts.go::PessimisticCommitGuard.Evaluate` + `orchestration/escape/fallback.go::DefaultPessimisticCommitGuard.Evaluate`** (5 类触发检测) |
| **D7-S18-A11-F02** | **ResolveFallback** | **F-BE** | **report** | **(policy FallbackPolicy, ruleName string)** | **✅** | **`orchestration/escape/fallback.go::DefaultPessimisticCommitGuard.ResolveFallback`** (3 路径: Pessimistic/RuleBased/Abort, Blockage.Source=policy_override 解析) |
| **D7-S18-A11-F03** | **BuildMVPArtifact** | **F-BE** | **report, blockedReason** | **MVPArtifact** | **✅** | **`orchestration/escape/fallback.go::DefaultPessimisticCommitGuard.BuildMVPArtifact`** (Output/RiskWarnings/Trigger/ChainHash FNV-1a) |
| **D7-S18-A11-F04** | **NotifyPessimisticHook** | **F-BE** | **engine, spec, report** | **(*TaskReport, error)** | **✅** | **`orchestration/escape/engine.go::EscapeEngine.NotifyPessimistic`** (5 层 fail-safe: nil guard / nil report / Evaluate error → fall-open / blocked → MVPArtifact 注入) |
| **D7-S18-A11-F05** | **CheckResourceExhausted** | **F-BE** | **used, budget, reserve** | **bool** | **✅** | **`orchestration/interfaces/convergence_budget.go::RemainingBelowReserve`** (资源耗尽触发检测, reserve 默认 10% budget) |

### D7-S18-A12 Rule-based Fallback

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S18-A12-F01** | **ParseFallbackRuleName** | **F-BE** | **string** | **(name, recognized)** | **✅** | **`orchestration/interfaces/fallback_policy.go::ParseFallbackRuleName`** (空 → 默认 / 4 候选 round-trip / 未知 → 默认 + recognized=false) |
| **D7-S18-A12-F02** | **ValidateFallbackPolicy** | **F-BE** | **FallbackPolicy** | **bool** | **✅** | **`orchestration/interfaces/fallback_policy.go::FallbackPolicy.Valid + ValidNonLegacy`** (3 态 / 2 non-legacy) |

---

## D7-S20-A01 TaskSpec 下行契约 F 层 ✅ (PR-A, DM-20260629-007)

> **物理位置：** `orchestration/interfaces/task_spec.go`。pure types 原则（0 import D7 子包）。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S20-A01-F01** | **NewTaskSpec** | **F-BE** | **session_id, plan, channel, work_item, trace_id** | **TaskSpec** | **✅** | **`orchestration/interfaces/task_spec.go::NewTaskSpec`（fail-fast session_id + TraceID format `ts_<8 hex>`）** |
| **D7-S20-A01-F02** | **ValidateTaskSpec** | **F-BE** | **TaskSpec** | **error** | **✅** | **`orchestration/interfaces/task_spec.go::Validate`**（happy path + empty session_id + channel unknown + trace_id 格式校验）|
| **D7-S20-A01-F03** | **WithTaskSpecFields** | **F-BE** | **TaskSpec, Plan/Channel/WorkItem** | **TaskSpec (immutable)** | **✅** | **`orchestration/interfaces/task_spec.go::WithPlan + WithChannel + WithWorkItem`（3 不可变 builder，浅拷贝 `c := *s` 返回新副本）**|

## D7-S20-A02 TaskReport 上行契约 F 层 ✅ (PR-A, DM-20260629-007)

> **物理位置：** `orchestration/interfaces/task_report.go`。pure types 原则。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S20-A02-F01** | **NewTaskReport** | **F-BE** | **session_id, channel, verdict, trace_id** | **TaskReport** | **✅** | **`orchestration/interfaces/task_report.go::NewTaskSpec`（fail-fast session_id + trace_id 格式）** |
| **D7-S20-A02-F02** | **ValidateTaskReport** | **F-BE** | **TaskReport** | **error** | **✅** | **`orchestration/interfaces/task_report.go::Validate`** |
| **D7-S20-A02-F03** | **WithTaskReportFields** | **F-BE** | **TaskReport, Verdict/Resource/Blockage** | **TaskReport (immutable)** | **✅** | **`orchestration/interfaces/task_report.go::WithVerdict + WithResource + WithBlockage`（3 不可变 builder）** |
| **D7-S20-A02-F04** | **AppendDissent** | **F-BE** | **TaskReport, Dissent** | **TaskReport (immutable)** | **✅** | **`orchestration/interfaces/task_report.go::AppendDissent`（top-3 截断 + summary hash 懒计算）**|

## D7-S21-A01/A02/A03 字段语义 F 层 ✅ (PR-A, DM-20260629-007)

> **物理位置：** `orchestration/interfaces/task_report.go` + `task_spec.go` + 内部 helpers。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S21-A01-F01** | **HashDissentSummary** | **F-BE** | **summary string** | **hash 8 hex prefix** | **✅** | **`orchestration/interfaces/task_report.go::hashSummary`（fnv64a → fmt.Sprintf("%08x", h)[:8]）** |
| **D7-S21-A01-F02** | **TopNTuncateDissent** | **F-BE** | **[]Dissent, n int** | **[]Dissent (≤ n)** | **✅** | **`orchestration/interfaces/task_report.go::AppendDissent` 内嵌（默认 n=3，silent truncate 不警告）** |
| **D7-S21-A02-F01** | **ClassifyBlockageKind** | **F-BE** | **failure error + Plan context** | **BlockageKind (permission/resource/contract)** | **✅** | **`orchestration/interfaces/task_spec.go::WithBlockage` 内嵌分类器（403/IAM deny → permission；OOM/disk/quota → resource；其他 → contract）** |
| **D7-S21-A03-F01** | **ExtractResource** | **F-BE** | **ExecutionResult + 上下文 (token accounting + Start/End time + ReAct iter count)** | **Resource (token/time/step)** | **✅** | **`orchestration/interfaces/task_report.go::WithResource` 内嵌抽取器（直接读 execution metadata）** |

## D7-S22 TaskContract PR-B 通讯契约预留位 ✅ (DESIGN ONLY, DM-20260629-006)

> **物理位置：** `orchestration/interfaces/contracts.go`（PLANNED，留给 PR-B Pessimistic Commit / PR-C CoW VersionChain）
>
> **PR-A 不实现**，仅登记 F 层接口签名作为契约锚点；PR-B + PR-C 在不破坏 `interfaces` 包 pure types 原则下扩展。

| F ID | Name | Type | Input | Output | Status | Code Location |
|------|------|------|-------|--------|--------|---------------|
| **D7-S22-F01** | **PessimisticCommitGuard** | **F-BE** | **TaskSpec + TaskReport (diff)** | **ok/blocked** | **⬜ PLANNED (PR-B)** | **`orchestration/interfaces/contracts.go::PessimisticCommitGuard`（防 false success commit，先 mark pessimistic state 再 verify）** |
| **D7-S22-F02** | **CoWVersionChain** | **F-BE** | **TaskSpec vN** | **TaskSpec vN+1 (with prev_version_id)** | **⬜ PLANNED (PR-C)** | **`orchestration/interfaces/contracts.go::CoWVersionChain`（每次 spec 变 → 生成新 version_id，引用前驱用于反向追溯）** |

---

